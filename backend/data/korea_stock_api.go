package data

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/korean"
)

// 韩国市场数据 API（Naver 金融，韩国官方门户）— 韩股数据的兜底/补充源
//
// 接口（均实测验证，2026-08）：
//   - 日K: https://fchart.stock.naver.com/sise.nhn?symbol=005930&timeframe=day&count=250&requestType=0
//     支持 KOSPI 指数（symbol=KOSPI，价格带两位小数）与个股，全历史（1990年起），响应为 XML（EUC-KR 编码）
//   - 分时: 同上 timeframe=minute，仅个股有效（KOSPI 指数返回空 <protocol/>），count 建议传 2500（约一周）
//     item 格式: "YYYYMMDDHHMM|null|null|null|价格|累计成交量"（open/high/low 恒为 null，成交量为累计值需差分）
//   - 日K item 格式: "YYYYMMDD|开|高|低|收|量"（已用 Naver polling 的 ov/hv/lv 字段交叉验证字段顺序）
//
// 注：腾讯 fqkline 对韩股仅返回 1 根K线（day/week/month 均如此，港股正常），故韩股日K统一走 Naver。

// naverSymbolFromStockCode 东财代码 → Naver symbol
// "100.KS11"(韩国KOSPI指数) → "KOSPI"；"177.005930"(三星电子) → "005930"；其他市场代码不支持
func naverSymbolFromStockCode(stockCode string) (string, bool) {
	switch {
	case stockCode == "100.KS11":
		return "KOSPI", true
	case strings.HasPrefix(stockCode, "177.") && len(stockCode) == len("177.000660"):
		symbol := strings.TrimPrefix(stockCode, "177.")
		if _, err := strconv.Atoi(symbol); err == nil {
			return symbol, true
		}
	}
	return "", false
}

// koreaNameBySymbol 已知韩股代码 → 中文显示名（Naver 返回的是韩文名，前端用户读中文）
var koreaNameBySymbol = map[string]string{
	"KOSPI":  "韩国KOSPI",
	"005930": "三星电子",
	"000660": "SK海力士",
}

// naverChartXML fchart 接口返回的 XML 结构
type naverChartXML struct {
	ChartData struct {
		Name  string `xml:"name,attr"`
		Items []struct {
			Data string `xml:"data,attr"`
		} `xml:"item"`
	} `xml:"chartdata"`
}

// naverChartItem 解析后的单条数据（日K/分时通用）
type naverChartItem struct {
	DateTime string // 日K "20260818"，分时 "202608191257"
	Open     string
	High     string
	Low      string
	Close    string
	Volume   string
}

// fetchNaverChart 拉取 fchart XML 并解析（EUC-KR → UTF-8）
func fetchNaverChart(symbol, timeframe string, count int) (*naverChartXML, error) {
	reqURL := fmt.Sprintf("https://fchart.stock.naver.com/sise.nhn?symbol=%s&timeframe=%s&count=%d&requestType=0",
		symbol, timeframe, count)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36").
		SetHeader("Referer", "https://finance.naver.com").
		Get(reqURL)
	if err != nil {
		return nil, err
	}
	body := resp.Body()
	if resp.StatusCode() != 200 {
		preview := body
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %q", resp.StatusCode(), string(preview))
	}

	// 声明为 EUC-KR 编码（名称等韩文字段），解码为 UTF-8 后再解析；纯数字字段不受影响
	decoded, decErr := korean.EUCKR.NewDecoder().Bytes(body)
	if decErr != nil {
		decoded = body
	}

	var doc naverChartXML
	// 声明仍为 encoding="EUC-KR"，需提供 CharsetReader（字节已解码为 UTF-8，透传即可）
	dec := xml.NewDecoder(bytes.NewReader(decoded))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := dec.Decode(&doc); err != nil {
		preview := decoded
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("xml unmarshal error: %w body_prefix=%q", err, string(preview))
	}
	return &doc, nil
}

// parseNaverItems 解析 "日期[时间]|开|高|低|收|量" 格式的 item 数据
func parseNaverItems(doc *naverChartXML) []naverChartItem {
	items := make([]naverChartItem, 0, len(doc.ChartData.Items))
	for _, raw := range doc.ChartData.Items {
		parts := strings.Split(raw.Data, "|")
		if len(parts) < 6 {
			continue
		}
		items = append(items, naverChartItem{
			DateTime: parts[0],
			Open:     parts[1],
			High:     parts[2],
			Low:      parts[3],
			Close:    parts[4],
			Volume:   parts[5],
		})
	}
	return items
}

// GetKoreaDayKLine 获取韩股日K（Naver fchart，支持 KOSPI 指数与韩股个股）
// stockCode 形如 "100.KS11"（韩国KOSPI）、"177.005930"（三星电子）、"177.000660"（SK海力士）
// days 为请求的K线数量（最近 N 个交易日），实际返回可能少于该值
func GetKoreaDayKLine(stockCode string, days int) *[]KLineData {
	empty := &[]KLineData{}
	symbol, ok := naverSymbolFromStockCode(stockCode)
	if !ok {
		logger.SugaredLogger.Errorf("GetKoreaDayKLine: unsupported stock code: %s", stockCode)
		return empty
	}
	if days <= 0 {
		days = 250
	}
	if days > 1000 {
		days = 1000
	}

	doc, err := fetchNaverChart(symbol, "day", days)
	if err != nil {
		logger.SugaredLogger.Errorf("GetKoreaDayKLine(%s) error: %v", stockCode, err)
		return empty
	}
	items := parseNaverItems(doc)
	if len(items) == 0 {
		logger.SugaredLogger.Errorf("GetKoreaDayKLine(%s): empty day chart", stockCode)
		return empty
	}

	k := &[]KLineData{}
	for _, it := range items {
		// DateTime "20260818" → "2026-08-18"
		day := it.DateTime
		if len(day) == 8 {
			day = day[:4] + "-" + day[4:6] + "-" + day[6:8]
		}
		*k = append(*k, KLineData{
			Day:    day,
			Open:   it.Open,
			Close:  it.Close,
			High:   it.High,
			Low:    it.Low,
			Volume: it.Volume,
		})
	}
	return k
}

// GetKoreaMinuteTrend 获取韩股最近交易日分时走势（Naver fchart，仅个股有效，KOSPI 指数不支持返回 nil）
// 用途：东财 trends2 失败时的兜底数据源，返回结构与 GetGlobalIndexTrend 完全一致
// 昨收取自 Naver 日K 的前一根收盘价；均价按成交量加权（VWAP）逐分钟累计计算
func GetKoreaMinuteTrend(stockCode string) *GlobalIndexTrendResult {
	symbol, ok := naverSymbolFromStockCode(stockCode)
	if !ok {
		return nil
	}

	// 1. 日K（取最近3根）：最后一根为最新交易日（盘中为当日实时K线），前一根收盘即昨收
	dayDoc, err := fetchNaverChart(symbol, "day", 3)
	if err != nil {
		logger.SugaredLogger.Warnf("GetKoreaMinuteTrend(%s) fetch day error: %v", stockCode, err)
		return nil
	}
	dayItems := parseNaverItems(dayDoc)
	if len(dayItems) == 0 {
		return nil
	}
	latestDay := dayItems[len(dayItems)-1]
	preClose := 0.0
	if len(dayItems) >= 2 {
		preClose, _ = strconv.ParseFloat(dayItems[len(dayItems)-2].Close, 64)
	}

	// 2. 分钟数据（count=2500 约一周），过滤出最新交易日
	minuteDoc, err := fetchNaverChart(symbol, "minute", 2500)
	if err != nil {
		logger.SugaredLogger.Warnf("GetKoreaMinuteTrend(%s) fetch minute error: %v", stockCode, err)
		return nil
	}
	minuteItems := parseNaverItems(minuteDoc)

	name := koreaNameBySymbol[symbol]
	if name == "" {
		name = dayDoc.ChartData.Name // 未知代码回退用韩文名（已解码 UTF-8）
	}
	result := &GlobalIndexTrendResult{
		Code:     symbol,
		Name:     name,
		PreClose: preClose,
		Date:     latestDay.DateTime[:4] + "-" + latestDay.DateTime[4:6] + "-" + latestDay.DateTime[6:8],
		Items:    make([]GlobalIndexTrendItem, 0, 400),
	}

	// 累计成交量/成交额 → 每分钟成交量、VWAP 均价
	var cumVol, cumAmount float64
	var prevCumVol float64
	for _, it := range minuteItems {
		if !strings.HasPrefix(it.DateTime, latestDay.DateTime) {
			continue
		}
		price, _ := strconv.ParseFloat(it.Close, 64)
		cumVol, _ = strconv.ParseFloat(it.Volume, 64)
		minuteVol := cumVol - prevCumVol
		if minuteVol < 0 {
			minuteVol = 0
		}
		prevCumVol = cumVol
		cumAmount += price * minuteVol
		avg := 0.0
		if cumVol > 0 {
			avg = cumAmount / cumVol
		}
		// DateTime "202608191257" → "2026-08-19 12:57"（与东财 trends2 格式一致）
		ts := it.DateTime
		if len(ts) != 12 {
			continue
		}
		result.Items = append(result.Items, GlobalIndexTrendItem{
			Time:     ts[:4] + "-" + ts[4:6] + "-" + ts[6:8] + " " + ts[8:10] + ":" + ts[10:12],
			Price:    price,
			AvgPrice: avg,
			Volume:   minuteVol,
			Amount:   price * minuteVol,
		})
	}
	if len(result.Items) == 0 {
		return nil
	}
	return result
}
