package data

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const yahooFinanceBaseURL = "https://query1.finance.yahoo.com"

var yahooSymbolPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.-]{0,19}$`)

// YahooFinanceAPI uses Yahoo's unofficial public endpoints; availability and
// redistribution rights are subject to Yahoo's terms.
type YahooFinanceAPI struct {
	client  *resty.Client
	baseURL string
}

func NewYahooFinanceAPI() *YahooFinanceAPI {
	return &YahooFinanceAPI{
		client:  CreateGlobalProxiedHTTPClient(12 * time.Second),
		baseURL: yahooFinanceBaseURL,
	}
}

func supportsYahooDailyFallback(code, klt, adjustFlag string) bool {
	flag := strings.ToLower(strings.TrimSpace(adjustFlag))
	return (IsHKStockCode(code) || IsUSStockCode(code)) && klt == "101" &&
		(flag == "" || flag == "none" || flag == "0")
}

func yahooSymbol(code string) (string, error) {
	original := strings.TrimSpace(code)
	code = strings.ToUpper(original)
	switch {
	case strings.HasSuffix(code, ".HK"):
		code = strings.TrimSuffix(code, ".HK")
		return yahooHKSymbol(code)
	case strings.HasSuffix(code, ".US"):
		code = strings.TrimSuffix(code, ".US")
	case strings.HasPrefix(code, "GB_"):
		code = strings.TrimPrefix(code, "GB_")
	case strings.HasPrefix(code, "HK"):
		return yahooHKSymbol(strings.TrimPrefix(code, "HK"))
	case strings.HasPrefix(original, "us") && len(code) > 2:
		code = strings.TrimPrefix(code, "US")
	}
	if !yahooSymbolPattern.MatchString(code) || !strings.ContainsAny(code, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return "", fmt.Errorf("不支持的港美股代码: %q", code)
	}
	return code, nil
}

func yahooHKSymbol(code string) (string, error) {
	if len(code) < 1 || len(code) > 5 {
		return "", fmt.Errorf("无效港股代码: %q", code)
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("无效港股代码: %q", code)
		}
	}
	code = strings.TrimLeft(code, "0")
	if code == "" {
		return "", fmt.Errorf("无效港股代码")
	}
	for len(code) < 4 {
		code = "0" + code
	}
	return code + ".HK", nil
}

func (a *YahooFinanceAPI) get(path string, params map[string]string, result any) error {
	resp, err := a.client.R().SetHeader("User-Agent", "Mozilla/5.0").
		SetQueryParams(params).Get(a.baseURL + path)
	if err != nil {
		return fmt.Errorf("Yahoo Finance 请求失败: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("Yahoo Finance 请求失败: HTTP %d", resp.StatusCode())
	}
	if err := json.Unmarshal(resp.Body(), result); err != nil {
		return fmt.Errorf("Yahoo Finance 响应解析失败: %w", err)
	}
	return nil
}

type yahooChart struct {
	Chart struct {
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
		Result []struct {
			Meta struct {
				ExchangeTimezoneName string `json:"exchangeTimezoneName"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// DailyKLine returns unadjusted daily candles, with an inclusive date cursor.
func (a *YahooFinanceAPI) DailyKLine(code string, limit int, end string) ([]KLineData, error) {
	symbol, err := yahooSymbol(code)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 5000 {
		return nil, fmt.Errorf("K线数量必须在 1 至 5000 之间")
	}
	now := time.Now().UTC()
	cutoff := ""
	endTime := now.Add(24 * time.Hour)
	if strings.TrimSpace(end) != "" {
		cutoff = klineCutoffKey(end)
		if len(cutoff) < 10 {
			return nil, fmt.Errorf("无效的 K 线日期游标: %q", end)
		}
		day, parseErr := time.Parse("2006-01-02", cutoff[:10])
		if parseErr != nil {
			return nil, fmt.Errorf("无效的 K 线日期游标: %q", end)
		}
		// Add two days to cover both time zones and Yahoo's exclusive period2.
		endTime = day.Add(48 * time.Hour)
	}
	startTime := endTime.AddDate(0, 0, -(limit*3 + 30))
	var response yahooChart
	err = a.get("/v8/finance/chart/"+url.PathEscape(symbol), map[string]string{
		"interval": "1d",
		"period1":  strconv.FormatInt(startTime.Unix(), 10),
		"period2":  strconv.FormatInt(endTime.Unix(), 10),
	}, &response)
	if err != nil {
		return nil, err
	}
	if response.Chart.Error != nil {
		return nil, fmt.Errorf("Yahoo Finance 图表错误: %s: %s", response.Chart.Error.Code, response.Chart.Error.Description)
	}
	if len(response.Chart.Result) == 0 || len(response.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 未返回 %s 的日线数据", symbol)
	}
	result := response.Chart.Result[0]
	loc, err := time.LoadLocation(result.Meta.ExchangeTimezoneName)
	if err != nil || result.Meta.ExchangeTimezoneName == "" {
		return nil, fmt.Errorf("Yahoo Finance 交易所时区无效: %q", result.Meta.ExchangeTimezoneName)
	}
	quote := result.Indicators.Quote[0]
	rows := make([]KLineData, 0, len(result.Timestamp))
	var previous float64
	for i, ts := range result.Timestamp {
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) ||
			quote.Open[i] == nil || quote.High[i] == nil || quote.Low[i] == nil ||
			quote.Close[i] == nil || quote.Volume[i] == nil {
			continue
		}
		day := time.Unix(ts, 0).In(loc).Format("2006-01-02")
		if cutoff != "" && klineCutoffKey(day) > cutoff {
			continue
		}
		open, high, low, closePrice, volume := *quote.Open[i], *quote.High[i], *quote.Low[i], *quote.Close[i], *quote.Volume[i]
		if open <= 0 || high <= 0 || low <= 0 || closePrice <= 0 || volume < 0 || high < low {
			continue
		}
		format := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
		row := KLineData{Day: day, Open: format(open), High: format(high), Low: format(low), Close: format(closePrice), Volume: format(volume)}
		if previous > 0 {
			row.ChangeValue = format(closePrice - previous)
			row.ChangePercent = format((closePrice - previous) / previous * 100)
			row.Amplitude = format((high - low) / previous * 100)
		}
		previous = closePrice
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 未返回 %s 的有效日线数据", symbol)
	}
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows, nil
}

type YahooNewsItem struct {
	Title       string   `json:"title"`
	Publisher   string   `json:"publisher"`
	URL         string   `json:"link"`
	PublishedAt int64    `json:"providerPublishTime"`
	Tickers     []string `json:"relatedTickers"`
}

// StockNews returns search results explicitly associated with the requested ticker.
func (a *YahooFinanceAPI) StockNews(code string, limit int) ([]YahooNewsItem, error) {
	symbol, err := yahooSymbol(code)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 20 {
		return nil, fmt.Errorf("新闻数量必须在 1 至 20 之间")
	}
	var response struct {
		News *[]YahooNewsItem `json:"news"`
	}
	if err := a.get("/v1/finance/search", map[string]string{
		"q": symbol, "quotesCount": "0", "newsCount": strconv.Itoa(limit),
	}, &response); err != nil {
		return nil, err
	}
	if response.News == nil {
		return nil, fmt.Errorf("Yahoo Finance 新闻响应缺少 news 字段")
	}
	items := make([]YahooNewsItem, 0, len(*response.News))
	for _, item := range *response.News {
		for _, ticker := range item.Tickers {
			if strings.EqualFold(ticker, symbol) && item.Title != "" && item.URL != "" {
				items = append(items, item)
				break
			}
		}
	}
	return items, nil
}
