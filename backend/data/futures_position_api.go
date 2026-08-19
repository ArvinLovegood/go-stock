package data

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"

	"github.com/tidwall/gjson"
)

// 股指期货（IF/IH/IC/IM）前20会员多空单持仓数据：
// 主源为东方财富数据中心 RPT_FUTU_NET_POSITION（与官网"期指持仓-净仓位指数对比"同源，
// 含多单/空单/净持仓/增减/结算价/现货指数收盘价/基差）；
// 降级源为中金所官网成交持仓排名 CSV（/sj/ccpm/YYYYMM/DD/VAR_1.csv，GBK 编码，
// 含前20会员成交量/持买单量/持卖单量明细），无指数收盘价与基差。
// 持仓数据为 T 日盘后更新（约 17:30-20:00），盘中查询返回上一交易日数据。

// FuturesPositionRow 单个交易日的前20会员持仓汇总（单位：手）
type FuturesPositionRow struct {
	TradeDate     string  `json:"tradeDate" md:"交易日"`
	SettlePrice   float64 `json:"settlePrice" md:"结算价"`
	LongPosition  int64   `json:"longPosition" md:"多单持仓(手)"`
	LongChange    int64   `json:"longChange" md:"多单增减(手)"`
	ShortPosition int64   `json:"shortPosition" md:"空单持仓(手)"`
	ShortChange   int64   `json:"shortChange" md:"空单增减(手)"`
	NetPosition   int64   `json:"netPosition" md:"净持仓(手)"`
	IndexClose    float64 `json:"indexClose" md:"现货指数收盘"`
	IndexChange   float64 `json:"indexChange" md:"指数涨跌"`
	Basis         float64 `json:"basis" md:"基差"`
}

// FuturesMemberRank 前20会员持仓明细（中金所 CSV，单位：手）
type FuturesMemberRank struct {
	Contract      string `json:"contract" md:"合约"`
	Rank          int    `json:"rank" md:"排名"`
	VolName       string `json:"volName" md:"成交量会员"`
	Volume        int64  `json:"volume" md:"成交量"`
	VolChange     int64  `json:"volChange" md:"成交量增减"`
	LongName      string `json:"longName" md:"持买单会员"`
	LongPosition  int64  `json:"longPosition" md:"持买单量"`
	LongChange    int64  `json:"longChange" md:"持买单增减"`
	ShortName     string `json:"shortName" md:"持卖单会员"`
	ShortPosition int64  `json:"shortPosition" md:"持卖单量"`
	ShortChange   int64  `json:"shortChange" md:"持卖单增减"`
}

// FuturesPositionResp 期指持仓趋势响应
type FuturesPositionResp struct {
	Variety      string               `json:"variety"`      // 品种代码：IF/IH/IC/IM
	VarietyName  string               `json:"varietyName"`  // 品种名称：沪深300股指期货...
	ContractCode string               `json:"contractCode"` // 主力合约：IF2609
	IndexCode    string               `json:"indexCode"`    // 对应现货指数代码：sh000300
	Source       string               `json:"source"`       // 数据源：eastmoney / cffex
	Rows         []FuturesPositionRow `json:"rows"`         // 按交易日升序
}

// futuresVarietyMeta 品种元信息
type futuresVarietyMeta struct {
	code string // 品种大写代码
	name string // 品种名称
	// 现货指数代码（新浪/东财通用小写前缀格式）
	indexCode string
}

var futuresVarietyMetas = map[string]futuresVarietyMeta{
	"if": {code: "IF", name: "沪深300股指期货", indexCode: "sh000300"},
	"ih": {code: "IH", name: "上证50股指期货", indexCode: "sh000016"},
	"ic": {code: "IC", name: "中证500股指期货", indexCode: "sh000905"},
	"im": {code: "IM", name: "中证1000股指期货", indexCode: "sh000852"},
}

// NormalizeFuturesVariety 归一化期指品种输入：
// 支持 IF/if/If、IF主力、沪深300、中证1000 等写法，无法识别返回空串。
func NormalizeFuturesVariety(input string) string {
	s := strings.TrimSpace(input)
	s = strings.ToUpper(s)
	// 剥离常见后缀：IF主力 → IF、IF2609 → IF
	for _, suffix := range []string{"主力", "当月", "连续", "主连"} {
		s = strings.TrimSuffix(s, suffix)
	}
	if len(s) > 2 {
		if _, err := strconv.Atoi(s[2:]); err == nil {
			s = s[:2]
		}
	}
	if _, ok := futuresVarietyMetas[strings.ToLower(s)]; ok {
		return strings.ToLower(s)
	}
	// 中文名映射
	switch {
	case strings.Contains(input, "沪深300") || strings.Contains(input, "300"):
		return "if"
	case strings.Contains(input, "上证50") || strings.Contains(input, "50"):
		return "ih"
	case strings.Contains(input, "中证500") || strings.Contains(input, "500"):
		return "ic"
	case strings.Contains(input, "中证1000") || strings.Contains(input, "1000"):
		return "im"
	}
	return ""
}

// FuturesPositionApi 股指期货多空持仓数据 API
type FuturesPositionApi struct {
	mainContractCache map[string]mainContractCacheEntry
	mu                sync.Mutex
}

type mainContractCacheEntry struct {
	code      string
	expiresAt time.Time
}

// NewFuturesPositionApi 创建股指期货持仓 API 实例
func NewFuturesPositionApi() *FuturesPositionApi {
	return &FuturesPositionApi{mainContractCache: map[string]mainContractCacheEntry{}}
}

// GetMainContract 查询品种当前主力合约（如 IF → IF2609）。
// 通过东财 RPT_FUTU_POSITIONCODE 的 IS_MAINCODE=1 标记定位，缓存 1 小时。
func (f *FuturesPositionApi) GetMainContract(variety string) string {
	key := NormalizeFuturesVariety(variety)
	if key == "" {
		return ""
	}
	f.mu.Lock()
	if entry, ok := f.mainContractCache[key]; ok && time.Now().Before(entry.expiresAt) {
		f.mu.Unlock()
		return entry.code
	}
	f.mu.Unlock()

	u := "https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_FUTU_POSITIONCODE&columns=ALL&pageSize=5&pageNumber=1&source=WEB&client=WEB&filter=" +
		url.QueryEscape(fmt.Sprintf("(TRADE_CODE=\"%s\")(IS_MAINCODE=\"1\")", strings.ToUpper(key)))
	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36").
		Get(u)
	if err != nil {
		logger.SugaredLogger.Warnf("GetMainContract %s request error: %v", key, err)
		return ""
	}
	code := gjson.Get(string(resp.Body()), "result.data.0.SECURITY_CODE").String()
	if code == "" {
		return ""
	}
	f.mu.Lock()
	f.mainContractCache[key] = mainContractCacheEntry{code: code, expiresAt: time.Now().Add(time.Hour)}
	f.mu.Unlock()
	return code
}

// GetFuturesPositionTrend 获取期指前20会员多空单持仓趋势（按交易日升序）。
// variety 支持 IF/IH/IC/IM 及中文名；contract 为空时自动定位主力合约；
// days 为取最近交易日数量（默认 60，上限 500）；东财失败时降级走中金所 CSV。
func (f *FuturesPositionApi) GetFuturesPositionTrend(variety, contract string, days int) *FuturesPositionResp {
	key := NormalizeFuturesVariety(variety)
	if key == "" {
		return nil
	}
	meta := futuresVarietyMetas[key]
	if days <= 0 {
		days = 60
	}
	if days > 500 {
		days = 500
	}
	contract = strings.TrimSpace(strings.ToUpper(contract))
	if contract == "" {
		contract = f.GetMainContract(key)
		if contract == "" {
			// 主力合约定位失败：按品种代码 + 当月月份兜底拼接（如 IF2608）
			contract = strings.ToUpper(key) + time.Now().Format("0601")
		}
	}

	if resp := f.fetchEastMoneyTrend(meta, contract, days); resp != nil && len(resp.Rows) > 0 {
		return resp
	}
	logger.SugaredLogger.Warnf("GetFuturesPositionTrend eastmoney failed, fallback to cffex csv, variety=%s", key)
	return f.fetchCffexTrend(meta, contract, days)
}

// fetchEastMoneyTrend 东财数据中心 RPT_FUTU_NET_POSITION
func (f *FuturesPositionApi) fetchEastMoneyTrend(meta futuresVarietyMeta, contract string, days int) *FuturesPositionResp {
	filter := fmt.Sprintf("(SECURITY_CODE=\"%s\")", contract)
	u := "https://datacenter-web.eastmoney.com/api/data/v1/get?reportName=RPT_FUTU_NET_POSITION&columns=ALL" +
		fmt.Sprintf("&pageSize=%d&pageNumber=1&source=WEB&client=WEB&sortColumns=TRADE_DATE&sortTypes=-1&filter=", days) +
		url.QueryEscape(filter)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36").
		Get(u)
	if err != nil {
		logger.SugaredLogger.Warnf("fetchEastMoneyTrend %s request error: %v", contract, err)
		return nil
	}
	body := string(resp.Body())
	if !gjson.Get(body, "success").Bool() {
		logger.SugaredLogger.Warnf("fetchEastMoneyTrend %s resp not success: %s", contract, gjson.Get(body, "message").String())
		return nil
	}
	arr := gjson.Get(body, "result.data").Array()
	if len(arr) == 0 {
		return nil
	}
	rows := make([]FuturesPositionRow, 0, len(arr))
	for _, item := range arr {
		rows = append(rows, FuturesPositionRow{
			TradeDate:     item.Get("TRADE_DATE").String()[:10],
			SettlePrice:   item.Get("SETTLE_PRICE").Float(),
			LongPosition:  item.Get("TOTAL_LONG_POSITION").Int(),
			LongChange:    item.Get("LP_CHANGE_TOTAL").Int(),
			ShortPosition: item.Get("TOTAL_SHORT_POSITION").Int(),
			ShortChange:   item.Get("SP_CHANGE_TOTAL").Int(),
			NetPosition:   item.Get("NET_POSITION").Int(),
			IndexClose:    item.Get("CLOSE_PRICE").Float(),
			IndexChange:   item.Get("CLOSE_PRICE_CHANGE").Float(),
			Basis:         item.Get("BASIS").Float(),
		})
	}
	// 接口按日期降序返回，反转为升序便于前端绘图
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return &FuturesPositionResp{
		Variety:      meta.code,
		VarietyName:  meta.name,
		ContractCode: contract,
		IndexCode:    meta.indexCode,
		Source:       "eastmoney",
		Rows:         rows,
	}
}

// fetchCffexTrend 中金所官网 CSV 降级源：逐日拉取并汇总前20会员多空单。
// 无指数收盘价/基差/结算价字段（置 0），多空增减由相邻交易日差值计算。
func (f *FuturesPositionApi) fetchCffexTrend(meta futuresVarietyMeta, contract string, days int) *FuturesPositionResp {
	// 中金所 CSV 按品种汇总：VAR_1.csv 为该品种全部合约按会员合并排名
	base := strings.ToUpper(meta.code)
	fetched := 0
	iterations := 0
	var rows []FuturesPositionRow
	// 从今天往前找交易日，最多回溯 days*2+10 个自然日（跨周末/节假日）
	for d := time.Now(); fetched < days && iterations <= days*2+10; d = d.AddDate(0, 0, -1) {
		iterations++
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		rank := f.fetchCffexRank(base, d)
		if rank == nil {
			continue
		}
		var long, short int64
		for _, m := range rank {
			long += m.LongPosition
			short += m.ShortPosition
		}
		rows = append(rows, FuturesPositionRow{
			TradeDate:     d.Format("2006-01-02"),
			LongPosition:  long,
			ShortPosition: short,
			NetPosition:   long - short,
		})
		fetched++
	}
	if len(rows) == 0 {
		return nil
	}
	// 反转为升序并计算增减差值
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	for i := 1; i < len(rows); i++ {
		rows[i].LongChange = rows[i].LongPosition - rows[i-1].LongPosition
		rows[i].ShortChange = rows[i].ShortPosition - rows[i-1].ShortPosition
	}
	return &FuturesPositionResp{
		Variety:      meta.code,
		VarietyName:  meta.name,
		ContractCode: contract,
		IndexCode:    meta.indexCode,
		Source:       "cffex",
		Rows:         rows,
	}
}

// GetFuturesMemberRank 获取某交易日的前20会员持仓明细龙虎榜（中金所 CSV）。
// variety 支持 IF/IH/IC/IM；tradeDate 为空取最近一个交易日（自动回溯 15 个自然日）。
func (f *FuturesPositionApi) GetFuturesMemberRank(variety, tradeDate string) []FuturesMemberRank {
	key := NormalizeFuturesVariety(variety)
	if key == "" {
		return nil
	}
	base := futuresVarietyMetas[key].code
	if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(tradeDate), time.Local); err == nil {
		return f.fetchCffexRank(base, t)
	}
	for i := 0; i <= 15; i++ {
		d := time.Now().AddDate(0, 0, -i)
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		if rank := f.fetchCffexRank(base, d); rank != nil {
			return rank
		}
	}
	return nil
}

// fetchCffexRank 拉取并解析中金所单日 CSV（GBK 编码）。
// 路径格式：/sj/ccpm/YYYYMM/DD/VAR_1.csv（YYYY/MM/DD 格式会 302 到 404）。
// 返回 nil 表示该日无数据（节假日/未发布）。
func (f *FuturesPositionApi) fetchCffexRank(varietyUpper string, d time.Time) []FuturesMemberRank {
	u := fmt.Sprintf("http://www.cffex.com.cn/sj/ccpm/%s/%s/%s_1.csv",
		d.Format("200601"), d.Format("02"), varietyUpper)
	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36").
		SetHeader("Referer", "http://www.cffex.com.cn/").
		Get(u)
	if err != nil || resp == nil || resp.StatusCode() != 200 {
		return nil
	}
	// CSV 为 GBK 编码，GB18030 为其超集，直接转 UTF-8
	raw := resp.Body()
	if len(raw) == 0 {
		return nil
	}
	return parseCffexCSV(string(GB18030ToUTF8(raw)))
}

// parseCffexCSV 解析中金所成交持仓排名 CSV：
// 前两行为表头（交易日,合约,排名,成交量排名,,,持买单量排名,,,持卖单量排名,），
// 数据列：日期,合约,名次,会员简称,成交量,增减,会员简称,持买单量,增减,会员简称,持卖单量,增减
func parseCffexCSV(body string) []FuturesMemberRank {
	reader := csv.NewReader(strings.NewReader(body))
	records, err := reader.ReadAll()
	if err != nil || len(records) <= 2 {
		return nil
	}
	ranks := make([]FuturesMemberRank, 0, 20)
	for _, rec := range records[2:] {
		if len(rec) < 12 {
			continue
		}
		rank, err := strconv.Atoi(strings.TrimSpace(rec[2]))
		if err != nil {
			continue
		}
		longPos := parseOrNullInt(nil, rec[7])
		shortPos := parseOrNullInt(nil, rec[10])
		if longPos == 0 && shortPos == 0 {
			continue
		}
		ranks = append(ranks, FuturesMemberRank{
			Contract:      strings.TrimSpace(rec[1]),
			Rank:          rank,
			VolName:       strings.TrimSpace(rec[3]),
			Volume:        parseOrNullInt(nil, rec[4]),
			VolChange:     parseOrNullInt(nil, rec[5]),
			LongName:      strings.TrimSpace(rec[6]),
			LongPosition:  longPos,
			LongChange:    parseOrNullInt(nil, rec[8]),
			ShortName:     strings.TrimSpace(rec[9]),
			ShortPosition: shortPos,
			ShortChange:   parseOrNullInt(nil, rec[11]),
		})
	}
	if len(ranks) == 0 {
		return nil
	}
	return pickMainContractRows(ranks)
}

// pickMainContractRows 品种 CSV 含该品种全部合约（每合约 20 行），
// 按总成交量最大的合约（即主力合约）过滤，只保留主力合约的 20 行。
func pickMainContractRows(ranks []FuturesMemberRank) []FuturesMemberRank {
	volByContract := map[string]int64{}
	for _, r := range ranks {
		volByContract[r.Contract] += r.Volume
	}
	mainContract := ""
	var maxVol int64 = -1
	for c, v := range volByContract {
		if v > maxVol {
			maxVol = v
			mainContract = c
		}
	}
	filtered := make([]FuturesMemberRank, 0, 20)
	for _, r := range ranks {
		if r.Contract == mainContract {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return ranks
	}
	return filtered
}

func parseOrNullInt(err error, s string) int64 {
	if err != nil {
		return 0
	}
	v, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if e != nil {
		return 0
	}
	return v
}

// FuturesPositionBrief 期指持仓概要（供 AI 工具输出）
type FuturesPositionBrief struct {
	Variety       string  `md:"品种"`
	ContractCode  string  `md:"主力合约"`
	TradeDate     string  `md:"数据日期"`
	LongPosition  int64   `md:"多单持仓(手)"`
	LongChange    int64   `md:"多单增减(手)"`
	ShortPosition int64   `md:"空单持仓(手)"`
	ShortChange   int64   `md:"空单增减(手)"`
	NetPosition   int64   `md:"净持仓(手)"`
	IndexClose    float64 `md:"指数收盘"`
	Basis         float64 `md:"基差"`
}

// BuildFuturesPositionBrief 从趋势数据提取最新一期概要
func BuildFuturesPositionBrief(resp *FuturesPositionResp) FuturesPositionBrief {
	brief := FuturesPositionBrief{
		Variety:      resp.Variety,
		ContractCode: resp.ContractCode,
	}
	if len(resp.Rows) == 0 {
		return brief
	}
	last := resp.Rows[len(resp.Rows)-1]
	brief.TradeDate = last.TradeDate
	brief.LongPosition = last.LongPosition
	brief.LongChange = last.LongChange
	brief.ShortPosition = last.ShortPosition
	brief.ShortChange = last.ShortChange
	brief.NetPosition = last.NetPosition
	brief.IndexClose = last.IndexClose
	brief.Basis = last.Basis
	return brief
}
