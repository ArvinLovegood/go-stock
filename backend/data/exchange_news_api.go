package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"

	"github.com/go-resty/resty/v2"
)

// 交易所公告/资讯聚合：币安官方公告（CMS）+ 币安广场新闻 + OKX 官方公告 + 欧易星球快讯。
//
// 数据源均公开、无需 API Key：
//   - OKX（官方 v5 公共接口）:
//     GET https://www.okx.com/api/v5/support/announcements?page=N[&annType=<分类>]
//   - 币安（官网公告中心同款接口，未官方文档化，存在变动风险）:
//     GET https://www.binance.com/bapi/composite/v1/public/cms/article/list/query
//     ?type=1&pageNo=N&pageSize=20[&catalogId=<分类>]
//     必须走 GET（POST 会被 WAF 403）；返回无 url 字段，需按 code 构造跳转链接。
//   - 币安广场新闻（square/news/all 页面同款接口，未官方文档化，存在变动风险）:
//     GET https://www.binance.com/bapi/composite/v4/friendly/pgc/feed/news/list?strategy=10
//     固定返回约 40 条最新文章、无分页参数；内容为英文原文（页面中文翻译依赖前端登录态）。
//
// 网络策略：复用合约客户端的双通道（直连优先，失败自动降级设置中的「合约代理」），
// 币安 CMS/广场与 fapi 行情共用 binanceChannel 通道状态。

const binanceCmsBase = "https://www.binance.com"

// binanceCatalogNames 币安公告分类（catalogId → 中文名，实测有效）。
var binanceCatalogNames = map[int]string{
	48:  "新币上线",
	49:  "最新新闻",
	93:  "最新活动",
	161: "下架",
	51:  "API 更新",
	128: "空投",
	157: "维护更新",
}

// okxAnnTypeNames OKX 公告分类（annType → 中文名，实测有效值）。
var okxAnnTypeNames = map[string]string{
	"announcements-new-listings":                             "新币上线",
	"announcements-trading-updates":                          "交易更新",
	"announcements-api":                                      "API 公告",
	"announcements-web3":                                     "Web3",
	"announcements-deposit-withdrawal-suspension-resumption": "充提暂停/恢复",
	"announcements-p2p-trading":                              "P2P 交易",
	"announcements-others":                                   "其他",
}

// ExchangeAnnouncement 统一公告条目。
type ExchangeAnnouncement struct {
	Source      string `json:"source"`      // binance / binancenews / okx / okxorbit
	ID          string `json:"id"`          // 原始 ID（币安公告 article id，广场新闻为帖子 id，OKX 为空）
	Title       string `json:"title"`       // 标题
	URL         string `json:"url"`         // 公告跳转链接（币安公告按 code 构造，其余为原始链接）
	Catalog     string `json:"catalog"`     // 分类展示名（快讯为来源媒体名）
	Summary     string `json:"summary"`     // 内容摘要（快讯类有）
	PublishedAt int64  `json:"publishedAt"` // 毫秒时间戳
}

// FetchExchangeAnnouncements 拉取交易所公告。
// source: all/binance/binancenews/okx/okxorbit；catalog: 币安为分类ID（如 48 新币上线），OKX 为 annType
// （如 announcements-new-listings），空为全部；page 从 1 开始。
// 各源独立容错：单源失败不影响其他源，失败时返回空列表并记日志。
func FetchExchangeAnnouncements(source, catalog string, page int) []ExchangeAnnouncement {
	if page < 1 {
		page = 1
	}
	source = strings.ToLower(strings.TrimSpace(source))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	switch source {
	case "binance":
		return fetchBinanceAnnouncements(ctx, catalog, page)
	case "binancenews":
		return fetchBinanceSquareNews(ctx)
	case "okx":
		return fetchOKXAnnouncements(ctx, catalog, page)
	case "okxorbit":
		return fetchOKXOrbitNews(ctx)
	default:
		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			out []ExchangeAnnouncement
		)
		appendFn := func(list []ExchangeAnnouncement) {
			mu.Lock()
			out = append(out, list...)
			mu.Unlock()
		}
		wg.Add(4)
		go func() {
			defer wg.Done()
			appendFn(fetchBinanceAnnouncements(ctx, "", page))
		}()
		go func() {
			defer wg.Done()
			appendFn(fetchBinanceSquareNews(ctx))
		}()
		go func() {
			defer wg.Done()
			appendFn(fetchOKXAnnouncements(ctx, "", page))
		}()
		go func() {
			defer wg.Done()
			appendFn(fetchOKXOrbitNews(ctx))
		}()
		wg.Wait()
		sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt > out[j].PublishedAt })
		return out
	}
}

// ===== 币安 CMS 公告 =====

type binanceCmsResp struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Data    struct {
		Catalogs []struct {
			CatalogId   int    `json:"catalogId"`
			CatalogName string `json:"catalogName"`
			Articles    []struct {
				ID          int64  `json:"id"`
				Code        string `json:"code"`
				Title       string `json:"title"`
				ReleaseDate int64  `json:"releaseDate"` // 毫秒
			} `json:"articles"`
		} `json:"catalogs"`
	} `json:"data"`
}

func fetchBinanceAnnouncements(ctx context.Context, catalogId string, page int) []ExchangeAnnouncement {
	params := url.Values{}
	params.Set("type", "1")
	params.Set("pageNo", strconv.Itoa(page))
	params.Set("pageSize", "20")
	if strings.TrimSpace(catalogId) != "" {
		params.Set("catalogId", strings.TrimSpace(catalogId))
	}
	body, err := doBinanceCmsRequest(ctx, "/bapi/composite/v1/public/cms/article/list/query", params)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安公告失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	var resp binanceCmsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.SugaredLogger.Warnf("解析币安公告失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	out := make([]ExchangeAnnouncement, 0, 32)
	for _, c := range resp.Data.Catalogs {
		name := c.CatalogName
		if zh, ok := binanceCatalogNames[c.CatalogId]; ok {
			name = zh
		}
		for _, a := range c.Articles {
			link := ""
			if a.Code != "" {
				link = binanceCmsBase + "/en/support/announcement/" + a.Code
			}
			out = append(out, ExchangeAnnouncement{
				Source:      "binance",
				ID:          strconv.FormatInt(a.ID, 10),
				Title:       strings.TrimSpace(a.Title),
				URL:         link,
				Catalog:     name,
				PublishedAt: a.ReleaseDate,
			})
		}
	}
	return out
}

// binanceCmsDoOnce 币安 CMS（www.binance.com）单次 GET。
func binanceCmsDoOnce(ctx context.Context, cli *resty.Client, path string, params url.Values) ([]byte, error) {
	if cli == nil {
		return nil, errors.New("币安 HTTP 客户端不可用")
	}
	req := cli.R().SetContext(ctx).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	if len(params) > 0 {
		req.SetQueryParamsFromValues(params)
	}
	resp, err := req.Get(binanceCmsBase + path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, &binanceHTTPError{statusCode: resp.StatusCode(), body: truncateForLog(resp.String())}
	}
	return resp.Body(), nil
}

// doBinanceCmsRequest 公告接口双通道：直连优先，失败走「合约代理」；
// 代理优先期间仍失败时兜底回试直连。与 fapi 行情共用 binanceChannel 通道状态。
func doBinanceCmsRequest(ctx context.Context, path string, params url.Values) ([]byte, error) {
	attemptDirectFirst := !binancePreferProxy()
	var lastErr error

	if attemptDirectFirst {
		body, err := binanceCmsDoOnce(ctx, binanceDirectClient(), path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isBinanceChannelFailure(err) {
			return nil, err
		}
		binanceMarkDirectFailed()
		logger.SugaredLogger.Warnf("币安 CMS 直连失败，尝试代理通道: path=%s err=%v", path, err)
	}

	if proxyCli := binanceProxyClient(); proxyCli != nil {
		body, err := binanceCmsDoOnce(ctx, proxyCli, path, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}

	if !attemptDirectFirst {
		body, err := binanceCmsDoOnce(ctx, binanceDirectClient(), path, params)
		if err == nil {
			binanceMarkDirectOK()
			return body, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("币安公告请求失败：直连不可达且未配置「合约代理」")
	}
	return nil, lastErr
}

// ===== OKX v5 公告 =====

type okxAnnResp struct {
	Code string `json:"code"`
	Data []struct {
		Details []struct {
			AnnType string `json:"annType"`
			Title   string `json:"title"`
			URL     string `json:"url"`
			PTime   string `json:"pTime"` // 毫秒时间戳（字符串）
		} `json:"details"`
		TotalPage int `json:"totalPage"`
	} `json:"data"`
}

func fetchOKXAnnouncements(ctx context.Context, annType string, page int) []ExchangeAnnouncement {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	if t := strings.TrimSpace(annType); t != "" {
		params.Set("annType", t)
	}
	body, err := doOKXRequest(ctx, http.MethodGet, "/api/v5/support/announcements", params, nil, false)
	if err != nil {
		logger.SugaredLogger.Warnf("获取 OKX 公告失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	var resp okxAnnResp
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.SugaredLogger.Warnf("解析 OKX 公告失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	out := make([]ExchangeAnnouncement, 0, 32)
	for _, g := range resp.Data {
		for _, d := range g.Details {
			ts, _ := strconv.ParseInt(strings.TrimSpace(d.PTime), 10, 64)
			name := d.AnnType
			if zh, ok := okxAnnTypeNames[d.AnnType]; ok {
				name = zh
			}
			out = append(out, ExchangeAnnouncement{
				Source:      "okx",
				Title:       strings.TrimSpace(d.Title),
				URL:         strings.TrimSpace(d.URL),
				Catalog:     name,
				PublishedAt: ts,
			})
		}
	}
	return out
}

// ===== OKX 欧易星球快讯（orbit/news，聚合 Odaily/PANews/ChainCatcher 等媒体） =====
//
// 数据来源：https://www.okx.com/zh-hans/orbit/news 页面 SSR 内嵌 JSON
// （<script id="appState" type="application/json"> → appContext.initialProps.orbit.feedNewsData）。
// 页面翻页走 priapi newsFlash 接口，但该接口对非浏览器环境返回空（有风控指纹校验），
// 因此只取 SSR 首屏（约 10 条，轮询即可持续获得最新快讯）。

const okxOrbitNewsURL = "https://www.okx.com/zh-hans/orbit/news"

// okxAppStateScript SSR 数据 script 标签前缀。
const okxAppStateScript = `<script data-id="__app_data_for_ssr__" type="application/json" id="appState">`

type okxOrbitState struct {
	AppContext struct {
		InitialProps struct {
			Orbit struct {
				FeedNewsData struct {
					ContentDataList []struct {
						ContentID         string `json:"contentId"`
						Title             string `json:"title"`
						TranslatedTitle   string `json:"translatedTitle"`
						Content           string `json:"content"`
						TranslatedContent string `json:"translatedContent"`
						PublishTime       string `json:"publishTime"` // 毫秒时间戳（字符串）
						ShareURL          string `json:"shareUrl"`
						SourcePlatform    string `json:"sourcePlatform"`
						Author            struct {
							NickName string `json:"nickName"`
						} `json:"author"`
					} `json:"contentDataList"`
				} `json:"feedNewsData"`
			} `json:"orbit"`
		} `json:"initialProps"`
	} `json:"appContext"`
}

// okxPageGet 双通道获取 OKX 页面 HTML（复用 OKX 合约客户端的直连/代理通道状态）。
func okxPageGet(ctx context.Context, pageURL string) ([]byte, error) {
	doOnce := func(cli *resty.Client) ([]byte, error) {
		if cli == nil {
			return nil, errors.New("OKX HTTP 客户端不可用")
		}
		resp, err := cli.R().SetContext(ctx).
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36").
			SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
			Get(pageURL)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, &okxHTTPError{statusCode: resp.StatusCode(), body: truncateForLog(resp.String())}
		}
		return resp.Body(), nil
	}

	attemptDirectFirst := !okxPreferProxy()
	var lastErr error
	if attemptDirectFirst {
		body, err := doOnce(okxDirectClient())
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isOKXChannelFailure(err) {
			return nil, err
		}
		okxMarkDirectFailed()
	}
	if proxyCli := okxProxyClient(); proxyCli != nil {
		body, err := doOnce(proxyCli)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}
	if !attemptDirectFirst {
		body, err := doOnce(okxDirectClient())
		if err == nil {
			okxMarkDirectOK()
			return body, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("OKX 页面请求失败：直连不可达且未配置「合约代理」")
	}
	return nil, lastErr
}

// extractSSRState 从页面 HTML 中提取 id="appState" 的 SSR JSON。
func extractSSRState(html []byte) ([]byte, error) {
	start := bytes.Index(html, []byte(okxAppStateScript))
	if start < 0 {
		return nil, errors.New("未找到 OKX SSR 数据节点（appState）")
	}
	start += len(okxAppStateScript)
	end := bytes.Index(html[start:], []byte("</script>"))
	if end < 0 {
		return nil, errors.New("OKX SSR 数据节点未闭合")
	}
	return html[start : start+end], nil
}

// fetchOKXOrbitNews 抓取 OKX 欧易星球快讯（中文优先标题/内容）。
func fetchOKXOrbitNews(ctx context.Context) []ExchangeAnnouncement {
	html, err := okxPageGet(ctx, okxOrbitNewsURL)
	if err != nil {
		logger.SugaredLogger.Warnf("获取 OKX 快讯页面失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	stateJSON, err := extractSSRState(html)
	if err != nil {
		logger.SugaredLogger.Warnf("提取 OKX 快讯 SSR 数据失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	var state okxOrbitState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		logger.SugaredLogger.Warnf("解析 OKX 快讯 SSR 数据失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	out := make([]ExchangeAnnouncement, 0, len(state.AppContext.InitialProps.Orbit.FeedNewsData.ContentDataList))
	for _, n := range state.AppContext.InitialProps.Orbit.FeedNewsData.ContentDataList {
		title := strings.TrimSpace(n.TranslatedTitle)
		if title == "" {
			title = strings.TrimSpace(n.Title)
		}
		summary := strings.TrimSpace(n.TranslatedContent)
		if summary == "" {
			summary = strings.TrimSpace(n.Content)
		}
		// 正文按 rune 截断，避免前端 tooltip 过长
		if r := []rune(summary); len(r) > 200 {
			summary = string(r[:200]) + "…"
		}
		ts, _ := strconv.ParseInt(strings.TrimSpace(n.PublishTime), 10, 64)
		source := strings.TrimSpace(n.Author.NickName)
		if source == "" {
			source = n.SourcePlatform
		}
		out = append(out, ExchangeAnnouncement{
			Source:      "okxorbit",
			ID:          strings.TrimSpace(n.ContentID),
			Title:       title,
			URL:         strings.TrimSpace(n.ShareURL),
			Catalog:     source,
			Summary:     summary,
			PublishedAt: ts,
		})
	}
	return out
}

// ===== 币安广场新闻（square/news/all，聚合 Binance News 及合作媒体文章） =====
//
// 数据来源：GET /bapi/composite/v4/friendly/pgc/feed/news/list?strategy=10
// 固定返回约 40 条最新文章，无分页参数（轮询即可获得最新内容）；
// 内容为英文原文（zh-CN 页面的中文翻译依赖前端登录态，接口侧无法直接获取）。

type binanceSquareResp struct {
	Code string `json:"code"`
	Data struct {
		Vos []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			SubTitle   string `json:"subTitle"` // 正文摘要
			AuthorName string `json:"authorName"`
			WebLink    string `json:"webLink"`
			Date       int64  `json:"date"` // Unix 秒级时间戳
		} `json:"vos"`
	} `json:"data"`
}

// fetchBinanceSquareNews 抓取币安广场新闻文章列表。
func fetchBinanceSquareNews(ctx context.Context) []ExchangeAnnouncement {
	params := url.Values{}
	params.Set("strategy", "10")
	body, err := doBinanceCmsRequest(ctx, "/bapi/composite/v4/friendly/pgc/feed/news/list", params)
	if err != nil {
		logger.SugaredLogger.Warnf("获取币安广场新闻失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	var resp binanceSquareResp
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.SugaredLogger.Warnf("解析币安广场新闻失败: %v", err)
		return []ExchangeAnnouncement{}
	}
	out := make([]ExchangeAnnouncement, 0, len(resp.Data.Vos))
	for _, n := range resp.Data.Vos {
		summary := strings.TrimSpace(n.SubTitle)
		// 正文按 rune 截断，避免前端 tooltip 过长
		if r := []rune(summary); len(r) > 200 {
			summary = string(r[:200]) + "…"
		}
		out = append(out, ExchangeAnnouncement{
			Source:      "binancenews",
			ID:          strings.TrimSpace(n.ID),
			Title:       strings.TrimSpace(n.Title),
			URL:         strings.TrimSpace(n.WebLink),
			Catalog:     strings.TrimSpace(n.AuthorName),
			Summary:     summary,
			PublishedAt: n.Date * 1000, // 秒 → 毫秒
		})
	}
	return out
}
