package data

// deepseek_search_api.go — DeepSeek 联网搜索与网页抓取 API 封装。
//
// 文档: https://api-docs.deepseek.com/zh-cn/guides/search
//   - Search: POST https://api.deepseek.com/v1/tools/search —— 按关键词搜索网页，返回结构化结果
//   - Fetch:  POST https://api.deepseek.com/v1/tools/fetch  —— 抓取单个网页，返回 Markdown 正文
//
// 鉴权: 与 DeepSeek API 相同的 Bearer Token（设置页 deepSeekSearchApiKey）。
// 计费: 按成功请求次数计费（无结果/失败不计费），因此层内不做额外节流，
//       由 Agent 工具层的参数约束（默认条数、need_content 默认 false）控制成本。
//
// 重试策略: 仅对 HTTP 408/429/5xx 重试（官方指引），其余 4xx 直接报错不重试。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"go-stock/backend/logger"
)

const (
	deepSeekSearchAPI = "https://api.deepseek.com/v1/tools/search"
	deepSeekFetchAPI  = "https://api.deepseek.com/v1/tools/fetch"

	deepSeekSearchMaxQueryLen = 128 // query 最大 Unicode 字符数
	deepSeekSearchMaxCount    = 50 // count 上限
	deepSeekFetchMaxURLLen    = 8192
)

// DeepSeekSearchAPI DeepSeek 联网搜索/网页抓取客户端。
type DeepSeekSearchAPI struct {
	client *resty.Client
	config *SettingConfig
}

// NewDeepSeekSearchAPI 构造客户端。每次调用都重新读取设置，Key 配置后无需重启即可生效。
func NewDeepSeekSearchAPI() *DeepSeekSearchAPI {
	// Search/Fetch 均为轻量只读请求，超时比 AI 调用短
	httpClient := &http.Client{
		Transport: SharedHTTPClient.GetClient().Transport,
		Timeout:   60 * time.Second,
	}
	client := resty.NewWithClient(httpClient).
		SetRetryCount(2).
		SetRetryWaitTime(2 * time.Second).
		// 仅对 408(读超时)/429(限速)/5xx 重试；400/401/402/403/404 重试无意义
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			return r.StatusCode() == http.StatusRequestTimeout ||
				r.StatusCode() == http.StatusTooManyRequests ||
				r.StatusCode() >= 500
		})
	return &DeepSeekSearchAPI{client: client, config: GetSettingConfig()}
}

// getApiKey 返回配置的 DeepSeek Search API Key。
func (api *DeepSeekSearchAPI) getApiKey() string {
	if api.config != nil && api.config.Settings != nil {
		return strings.TrimSpace(api.config.Settings.DeepSeekSearchApiKey)
	}
	return ""
}

// deepSeekAPIError OpenAI 兼容的错误响应体。
type deepSeekAPIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// DeepSeekSearchResult Search 接口返回的单条结果。
type DeepSeekSearchResult struct {
	RerankScore float64 `json:"rerank_score"` // 排序分数 0~3，>=1.5 为优质结果
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet"`  // 短摘要（100~200 字符），适合前端展示
	Summary     string  `json:"summary"`  // 长摘要（600~800 字符），适合模型阅读
	Content     string  `json:"content"` // 网页正文，need_content=false 时恒为空
	SiteName    string  `json:"site_name"`
	SiteIcon    string  `json:"site_icon"`
	Date        string  `json:"date"`     // RFC 3339 (UTC)，未知为空
	Authority   int     `json:"authority"` // 权威性 0~4，-1 未知
}

// DeepSeekSearchResponse Search 接口响应。
type deepSeekSearchResponse struct {
	Results []DeepSeekSearchResult `json:"results"`
}

// DeepSeekFetchResult Fetch 接口响应。
type DeepSeekFetchResult struct {
	Title   string `json:"title"`
	Content string `json:"content"` // Markdown 正文
}

// parseError 把非 200 响应解析为可读错误信息。
func parseDeepSeekError(resp *resty.Response) error {
	var errBody deepSeekAPIError
	if json.Unmarshal(resp.Body(), &errBody) == nil && errBody.Error.Message != "" {
		return fmt.Errorf("DeepSeek API 错误(HTTP %d): %s", resp.StatusCode(), errBody.Error.Message)
	}
	return fmt.Errorf("DeepSeek API 请求失败(HTTP %d): %s", resp.StatusCode(), string(resp.Body()))
}

// Search 执行联网搜索。
//
// 参数:
//   - query: 搜索词，1~128 Unicode 字符；支持 site:/-site:/-/"..."/lang:/after:/before: 搜索语法
//   - count: 返回条数 1~50，0 取默认值（由服务端决定，当前为 10）
//   - needContent: 是否返回网页正文（按次计费下仍建议按需开启，正文会显著增大返回体积）
//   - oriQuestion: 触发搜索的用户原始问题（可选，帮助服务端理解意图）
func (api *DeepSeekSearchAPI) Search(query string, count int, needContent bool, oriQuestion string) ([]DeepSeekSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("搜索词不能为空")
	}
	if len([]rune(query)) > deepSeekSearchMaxQueryLen {
		return nil, fmt.Errorf("搜索词不能超过 %d 个字符", deepSeekSearchMaxQueryLen)
	}
	if api.getApiKey() == "" {
		return nil, fmt.Errorf("DeepSeek 联网搜索未配置，请在设置中填写 DeepSeek API Key（deepSeekSearchApiKey）")
	}
	if count < 0 {
		count = 0
	}
	if count > deepSeekSearchMaxCount {
		count = deepSeekSearchMaxCount
	}

	body := map[string]any{
		"query":        query,
		"count":        count,
		"need_content": needContent,
	}
	if q := strings.TrimSpace(oriQuestion); q != "" {
		body["ori_question"] = q
	}

	logger.SugaredLogger.Infof("DeepSeek Search: query=%q count=%d need_content=%v", query, count, needContent)

	resp, err := api.client.R().
		SetHeader("Authorization", "Bearer "+api.getApiKey()).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(deepSeekSearchAPI)
	if err != nil {
		return nil, fmt.Errorf("联网搜索请求失败: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, parseDeepSeekError(resp)
	}

	var out deepSeekSearchResponse
	if err := json.Unmarshal(resp.Body(), &out); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %v", err)
	}
	return out.Results, nil
}

// Fetch 抓取单个网页并返回 Markdown 正文。
func (api *DeepSeekSearchAPI) Fetch(rawURL string) (*DeepSeekFetchResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("网页地址不能为空")
	}
	if len(rawURL) > deepSeekFetchMaxURLLen {
		return nil, fmt.Errorf("网页地址过长（超过 %d 字符）", deepSeekFetchMaxURLLen)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("无效的网页地址: %s", rawURL)
	}
	if u.User != nil {
		return nil, fmt.Errorf("网页地址不能携带用户名或密码")
	}
	if api.getApiKey() == "" {
		return nil, fmt.Errorf("DeepSeek 联网搜索未配置，请在设置中填写 DeepSeek API Key（deepSeekSearchApiKey）")
	}

	logger.SugaredLogger.Infof("DeepSeek Fetch: url=%s", rawURL)

	resp, err := api.client.R().
		SetHeader("Authorization", "Bearer "+api.getApiKey()).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"url": rawURL}).
		Post(deepSeekFetchAPI)
	if err != nil {
		return nil, fmt.Errorf("网页抓取请求失败: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, parseDeepSeekError(resp)
	}

	var out DeepSeekFetchResult
	if err := json.Unmarshal(resp.Body(), &out); err != nil {
		return nil, fmt.Errorf("解析网页内容失败: %v", err)
	}
	return &out, nil
}
