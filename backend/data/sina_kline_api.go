package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type SinaKLineApi struct {
	client *resty.Client
	config *SettingConfig
}

type SinaKLineItem struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

func NewSinaKLineApi(config *SettingConfig) *SinaKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	return &SinaKLineApi{
		client: client,
		config: config,
	}
}

func sinaScaleFromKlt(klt string) string {
	switch klt {
	case "1":
		return "1"
	case "5":
		return "5"
	case "15":
		return "15"
	case "30":
		return "30"
	case "60":
		return "60"
	case "101":
		return "240"
	case "102":
		return "1200"
	default:
		return ""
	}
}

func sinaSymbolFromStockCode(stockCode string) string {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return "sh" + pureCode
			case "SZ":
				return "sz" + pureCode
			case "BJ":
				return "bj" + pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		return strings.ToLower(code[:2]) + code[2:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6":
			return "sh" + code
		case "0", "3":
			return "sz" + code
		case "8", "9":
			return "bj" + code
		}
	}
	return strings.ToLower(code)
}

func (s *SinaKLineApi) GetKLineData(stockCode, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	scale := sinaScaleFromKlt(klt)
	if scale == "" {
		logger.SugaredLogger.Warnf("SinaKLine: unsupported klt %s", klt)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1023 {
		limit = 1023
	}
	symbol := sinaSymbolFromStockCode(stockCode)
	if symbol == "" {
		logger.SugaredLogger.Errorf("SinaKLine: invalid stock code: %s", stockCode)
		return result
	}

	ts := time.Now().UnixMilli()
	callback := fmt.Sprintf("callback_%d", ts)
	baseURL := "https://quotes.sina.cn/cn/api/jsonp_v2.php/" + callback + "/CN_MarketDataService.getKLineData"
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("scale", scale)
	params.Set("ma", "no")
	params.Set("datalen", fmt.Sprintf("%d", limit))
	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req := s.client.R()
	req.SetHeader("User-Agent", getRandomUA())
	req.SetHeader("Accept", "*/*")
	req.SetHeader("Referer", "https://finance.sina.com.cn")
	req.SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("SinaKLine HTTP error: %v", err)
		return result
	}
	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("SinaKLine HTTP %d", resp.StatusCode())
		return result
	}

	body := string(resp.Body())
	items := s.parseJSONPResponse(body)
	if items == nil {
		return result
	}

	converted := s.convertToKLineData(items, klt)

	if klt == "101" || klt == "102" || klt == "103" {
		converted = s.appendTodayKLine(converted, stockCode, klt)
	}

	return &converted
}

var jsonpArrayStartRe = regexp.MustCompile(`\[\s*\{`)

func (s *SinaKLineApi) parseJSONPResponse(body string) []SinaKLineItem {
	var items []SinaKLineItem
	trimmed := strings.TrimSpace(body)

	startLoc := jsonpArrayStartRe.FindStringIndex(trimmed)
	if len(startLoc) == 2 {
		jsonStart := startLoc[0]
		lastBracket := strings.LastIndex(trimmed, "]")
		if lastBracket < jsonStart {
			logger.SugaredLogger.Errorf("SinaKLine: no closing ] found, body prefix: %s", truncateStr(trimmed, 200))
			return nil
		}
		jsonStr := trimmed[jsonStart : lastBracket+1]
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			logger.SugaredLogger.Errorf("SinaKLine JSONP parse error: %v, jsonStr len=%d, body prefix: %s", err, len(jsonStr), truncateStr(trimmed, 200))
			return nil
		}
		return items
	}

	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		logger.SugaredLogger.Errorf("SinaKLine JSON parse error: %v, body prefix: %s", err, truncateStr(trimmed, 200))
		return nil
	}
	return items
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *SinaKLineApi) appendTodayKLine(data []KLineData, stockCode, klt string) []KLineData {
	if len(data) == 0 {
		return data
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")

	lastDay := data[len(data)-1].Day
	if len(lastDay) >= 10 && lastDay[:10] == todayStr {
		return data
	}

	hour, minute := now.Hour(), now.Minute()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return data
	}
	if hour < 9 || (hour == 9 && minute < 30) {
		return data
	}

	symbol := sinaSymbolFromStockCode(stockCode)
	sinaCode := symbol
	if strings.HasPrefix(sinaCode, "bj") {
		sinaCode = "sb" + sinaCode[2:]
	}

	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", now.UnixMilli(), sinaCode)
	resp, err := s.client.R().
		SetHeader("Host", "hq.sinajs.cn").
		SetHeader("Referer", "https://finance.sina.com.cn/").
		SetHeader("User-Agent", getRandomUA()).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Warnf("SinaKLine appendTodayKLine hq error: %v", err)
		return data
	}

	body := GB18030ToUTF8(resp.Body())
	parts := strings.SplitN(body, "\"", 3)
	if len(parts) < 2 {
		return data
	}
	fields := strings.Split(parts[1], ",")
	if len(fields) < 32 {
		return data
	}

	openPrice := strings.TrimSpace(fields[1])
	prevClose := strings.TrimSpace(fields[2])
	curPrice := strings.TrimSpace(fields[3])
	highPrice := strings.TrimSpace(fields[4])
	lowPrice := strings.TrimSpace(fields[5])
	volume := strings.TrimSpace(fields[8])
	amount := strings.TrimSpace(fields[9])
	tradeDate := strings.TrimSpace(fields[30])

	if tradeDate != todayStr {
		return data
	}

	curPriceF, _ := strconv.ParseFloat(curPrice, 64)
	prevCloseF, _ := strconv.ParseFloat(prevClose, 64)
	openF, _ := strconv.ParseFloat(openPrice, 64)
	highF, _ := strconv.ParseFloat(highPrice, 64)
	lowF, _ := strconv.ParseFloat(lowPrice, 64)
	volF, _ := strconv.ParseFloat(volume, 64)
	amountF, _ := strconv.ParseFloat(amount, 64)

	if curPriceF <= 0 || openF <= 0 {
		return data
	}

	day := todayStr
	if klt == "102" {
		_, isoWeek := now.ISOWeek()
		day = fmt.Sprintf("%s-W%02d", now.Format("2006"), isoWeek)
	} else if klt == "103" {
		day = now.Format("2006-01")
	}

	todayKd := KLineData{
		Day:    day,
		Open:   fmt.Sprintf("%.2f", openF),
		Close:  fmt.Sprintf("%.2f", curPriceF),
		High:   fmt.Sprintf("%.2f", highF),
		Low:    fmt.Sprintf("%.2f", lowF),
		Volume: fmt.Sprintf("%.0f", volF/100),
		Amount: fmt.Sprintf("%.2f", amountF),
	}
	if prevCloseF > 0 {
		todayKd.ChangePercent = fmt.Sprintf("%.2f", (curPriceF-prevCloseF)/prevCloseF*100)
		todayKd.ChangeValue = fmt.Sprintf("%.2f", curPriceF-prevCloseF)
		todayKd.Amplitude = fmt.Sprintf("%.2f", (highF-lowF)/prevCloseF*100)
	}

	if klt == "101" {
		return append(data, todayKd)
	}

	if klt == "102" || klt == "103" {
		lastDayStr := data[len(data)-1].Day
		samePeriod := false
		if klt == "102" {
			samePeriod = lastDayStr == day
		} else if klt == "103" {
			samePeriod = len(lastDayStr) >= 7 && lastDayStr[:7] == day[:7]
		}
		if samePeriod {
			data[len(data)-1] = todayKd
		} else {
			data = append(data, todayKd)
		}
	}

	return data
}

func (s *SinaKLineApi) convertToKLineData(items []SinaKLineItem, klt string) []KLineData {
	result := make([]KLineData, 0, len(items))
	for i, item := range items {
		kd := KLineData{
			Day:    item.Day,
			Open:   safeStr(item.Open),
			Close:  safeStr(item.Close),
			High:   safeStr(item.High),
			Low:    safeStr(item.Low),
			Volume: safeStr(item.Volume),
			Amount: "0",
		}
		if i > 0 {
			prevClose, _ := parseFloatToFloat(items[i-1].Close)
			curClose, _ := parseFloatToFloat(item.Close)
			curHigh, _ := parseFloatToFloat(item.High)
			curLow, _ := parseFloatToFloat(item.Low)
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", curClose-prevClose)
			}
			if prevClose > 0 {
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		result = append(result, kd)
	}
	return result
}

func safeStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "null" {
		return "0"
	}
	return s
}

type TencentKLineApi struct {
	client *resty.Client
	config *SettingConfig
}

type TencentKLineResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data map[string]struct {
		Day      [][]string `json:"day,omitempty"`
		Week     [][]string `json:"week,omitempty"`
		Month    [][]string `json:"month,omitempty"`
		Qfqday   [][]string `json:"qfqday,omitempty"`
		Qfqweek  [][]string `json:"qfqweek,omitempty"`
		Qfqmonth [][]string `json:"qfqmonth,omitempty"`
	} `json:"data"`
}

func NewTencentKLineApi(config *SettingConfig) *TencentKLineApi {
	client := SharedHTTPClient
	client.SetTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	return &TencentKLineApi{
		client: client,
		config: config,
	}
}

func tencentPeriodFromKlt(klt string) string {
	switch klt {
	case "101":
		return "day"
	case "102":
		return "week"
	case "103":
		return "month"
	default:
		return ""
	}
}

func tencentSymbolFromStockCode(stockCode string) string {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return "sh" + pureCode
			case "SZ":
				return "sz" + pureCode
			case "BJ":
				return "bj" + pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		return strings.ToLower(code[:2]) + code[2:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6":
			return "sh" + code
		case "0", "3":
			return "sz" + code
		case "8", "9":
			return "bj" + code
		}
	}
	return strings.ToLower(code)
}

func (t *TencentKLineApi) GetKLineData(stockCode, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	period := tencentPeriodFromKlt(klt)
	if period == "" {
		logger.SugaredLogger.Warnf("TencentKLine: unsupported klt %s (only day/week/month)", klt)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 800 {
		limit = 800
	}
	symbol := tencentSymbolFromStockCode(stockCode)
	if symbol == "" {
		logger.SugaredLogger.Errorf("TencentKLine: invalid stock code: %s", stockCode)
		return result
	}

	varName := fmt.Sprintf("kline_%sqfq", period)
	param := fmt.Sprintf("%s,%s,,,%d,qfq", symbol, period, limit)
	params := url.Values{}
	params.Set("_var", varName)
	params.Set("param", param)
	reqURL := "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?" + params.Encode()

	req := t.client.R()
	req.SetHeader("User-Agent", getRandomUA())
	req.SetHeader("Accept", "*/*")
	req.SetHeader("Referer", "https://gu.qq.com")

	resp, err := req.Get(reqURL)
	if err != nil {
		logger.SugaredLogger.Errorf("TencentKLine HTTP error: %v", err)
		return result
	}
	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("TencentKLine HTTP %d", resp.StatusCode())
		return result
	}

	body := string(resp.Body())
	jsonStr := t.stripVarPrefix(body, varName)
	if jsonStr == "" {
		logger.SugaredLogger.Errorf("TencentKLine: failed to strip var prefix, body prefix: %s", truncateStr(body, 200))
		return result
	}

	var tResp TencentKLineResponse
	if err := json.Unmarshal([]byte(jsonStr), &tResp); err != nil {
		logger.SugaredLogger.Errorf("TencentKLine JSON parse error: %v", err)
		return result
	}
	if tResp.Code != 0 {
		logger.SugaredLogger.Errorf("TencentKLine API error: code=%d msg=%s", tResp.Code, tResp.Msg)
		return result
	}

	return t.extractKLineData(&tResp, symbol, period)
}

func (t *TencentKLineApi) stripVarPrefix(body, varName string) string {
	trimmed := strings.TrimSpace(body)
	prefix := varName + "="
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), ";")
	}
	return trimmed
}

func (t *TencentKLineApi) extractKLineData(tResp *TencentKLineResponse, symbol, period string) *[]KLineData {
	result := &[]KLineData{}
	for _, stockData := range tResp.Data {
		var rows [][]string
		switch period {
		case "day":
			if len(stockData.Qfqday) > 0 {
				rows = stockData.Qfqday
			} else {
				rows = stockData.Day
			}
		case "week":
			if len(stockData.Qfqweek) > 0 {
				rows = stockData.Qfqweek
			} else {
				rows = stockData.Week
			}
		case "month":
			if len(stockData.Qfqmonth) > 0 {
				rows = stockData.Qfqmonth
			} else {
				rows = stockData.Month
			}
		}
		if len(rows) == 0 {
			continue
		}
		converted := t.convertRowsToKLineData(rows)
		return &converted
	}
	return result
}

func (t *TencentKLineApi) convertRowsToKLineData(rows [][]string) []KLineData {
	result := make([]KLineData, 0, len(rows))
	for i, row := range rows {
		if len(row) < 6 {
			continue
		}
		kd := KLineData{
			Day:    row[0],
			Open:   safeStr(row[1]),
			Close:  safeStr(row[2]),
			High:   safeStr(row[3]),
			Low:    safeStr(row[4]),
			Volume: safeStr(row[5]),
			Amount: "0",
		}
		if len(row) >= 7 {
			kd.Amount = safeStr(row[6])
		}
		if i > 0 {
			prevClose, _ := parseFloatToFloat(rows[i-1][2])
			curClose, _ := parseFloatToFloat(row[2])
			curHigh, _ := parseFloatToFloat(row[3])
			curLow, _ := parseFloatToFloat(row[4])
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (curClose-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", curClose-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (curHigh-curLow)/prevClose*100)
			}
		}
		result = append(result, kd)
	}
	return result
}

type KLineSourceResult struct {
	Data   *[]KLineData `json:"data"`
	Source string       `json:"source"`
}

// eastMoneyAdjustFromFlag 将前端复权标识转为东方财富 API 的 fqt 参数值。
// "qfq"→"1"(前复权)、"hfq"→"2"(后复权)、"none"/"0"→"0"(不复权)；
// 空串或无法识别时返回空串，保留 EastMoney API 默认行为（兼容旧调用方）。
func eastMoneyAdjustFromFlag(adjustFlag string) string {
	switch strings.ToLower(strings.TrimSpace(adjustFlag)) {
	case "qfq":
		return "1"
	case "hfq":
		return "2"
	case "none", "0":
		return "0"
	default:
		return ""
	}
}

// FetchKLineWithFallback 按降级链获取 K 线数据：MAC→东方财富→(港美股返回)→新浪→腾讯→通达信。
// adjustFlag 可选，控制复权类型："qfq"前复权、"hfq"后复权、"none"/"0"不复权；
// 未传时各数据源保持原有默认行为（A股 MAC/通达信默认前复权，港股默认不复权，EastMoney 走 API 默认）。
// 注意：新浪/腾讯数据源硬编码前复权作为兜底，adjustFlag 对其无效；港美股 ExKLine2 协议不支持复权。
func FetchKLineWithFallback(stockCode, stockName, klt string, limit int, end string, adjustFlag ...string) *KLineSourceResult {
	flag := adjustFlagFromVariadic(adjustFlag...)

	macResult := fetchFromMACWithTimeout(stockCode, klt, limit, 5*time.Second, flag)
	if macResult != nil && macResult.Data != nil && len(*macResult.Data) > 0 {
		macResult.Source = "tdx-mac"
		fillVolumeRatio(macResult.Data)
		return macResult
	}
	logger.SugaredLogger.Warnf("MAC K线数据为空或超时，尝试东方财富数据源: code=%s klt=%s", stockCode, klt)

	eastMoneyResult := fetchFromEastMoney(stockCode, stockName, klt, limit, end, flag)
	if eastMoneyResult != nil && eastMoneyResult.Data != nil && len(*eastMoneyResult.Data) > 0 {
		eastMoneyResult.Source = "eastmoney"
		fillVolumeRatio(eastMoneyResult.Data)
		return eastMoneyResult
	}

	// 港美股/中证指数/海外指数：MAC失败后仅走东方财富，新浪/腾讯/通达信不支持港美股/.CSI/100.XXX海外指数
	if IsHKStockCode(stockCode) || IsUSStockCode(stockCode) || IsCSIIndexCode(stockCode) || IsGlobalIndexCode(stockCode) {
		if macResult != nil {
			macResult.Source = "tdx-mac-ex"
			return macResult
		}
		return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
	}

	logger.SugaredLogger.Warnf("EastMoney K线数据为空或失败，尝试新浪数据源: code=%s klt=%s", stockCode, klt)

	sinaResult := fetchFromSina(stockCode, klt, limit)
	if sinaResult != nil && sinaResult.Data != nil && len(*sinaResult.Data) > 0 {
		sinaResult.Source = "sina"
		fillVolumeRatio(sinaResult.Data)
		return sinaResult
	}
	logger.SugaredLogger.Warnf("新浪K线数据也为空，尝试腾讯数据源: code=%s klt=%s", stockCode, klt)

	tencentResult := fetchFromTencent(stockCode, klt, limit)
	if tencentResult != nil && tencentResult.Data != nil && len(*tencentResult.Data) > 0 {
		tencentResult.Source = "tencent"
		fillVolumeRatio(tencentResult.Data)
		return tencentResult
	}
	logger.SugaredLogger.Warnf("腾讯K线数据也为空，尝试通达信数据源: code=%s klt=%s", stockCode, klt)

	tdxResult := fetchFromTdx(stockCode, klt, limit, flag)
	if tdxResult != nil && tdxResult.Data != nil && len(*tdxResult.Data) > 0 {
		tdxResult.Source = "tdx"
		fillVolumeRatio(tdxResult.Data)
		return tdxResult
	}
	logger.SugaredLogger.Warnf("通达信K线数据也为空: code=%s klt=%s", stockCode, klt)

	if eastMoneyResult != nil {
		eastMoneyResult.Source = "eastmoney"
		return eastMoneyResult
	}
	return &KLineSourceResult{Data: &[]KLineData{}, Source: ""}
}

// fillVolumeRatio 补算量比：量比 = 当日成交量 / 过去5日平均成交量
func fillVolumeRatio(data *[]KLineData) {
	if data == nil || len(*data) < 6 {
		return
	}
	list := *data
	for i := 5; i < len(list); i++ {
		if list[i].VolumeRatio != "" {
			continue
		}
		curVol, err := strconv.ParseFloat(list[i].Volume, 64)
		if err != nil || curVol <= 0 {
			continue
		}
		var sum float64
		validDays := 0
		for j := i - 5; j < i; j++ {
			v, err := strconv.ParseFloat(list[j].Volume, 64)
			if err == nil && v > 0 {
				sum += v
				validDays++
			}
		}
		if validDays > 0 {
			avg := sum / float64(validDays)
			if avg > 0 {
				list[i].VolumeRatio = fmt.Sprintf("%.2f", curVol/avg)
			}
		}
	}
}

func fetchFromEastMoney(stockCode, stockName, klt string, limit int, end string, adjustFlag string) *KLineSourceResult {
	api := NewEastMoneyKLineApi(GetSettingConfig())
	emAdjust := eastMoneyAdjustFromFlag(adjustFlag)
	var data *[]KLineData
	if strings.TrimSpace(end) == "" {
		data = api.GetKLineDataBefore(stockCode, klt, emAdjust, limit, "20500101")
	} else {
		data = api.GetKLineDataBefore(stockCode, klt, emAdjust, limit, end)
	}
	return &KLineSourceResult{Data: data}
}

func fetchFromSina(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewSinaKLineApi(GetSettingConfig())
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTencent(stockCode, klt string, limit int) *KLineSourceResult {
	api := NewTencentKLineApi(GetSettingConfig())
	data := api.GetKLineData(stockCode, klt, limit)
	return &KLineSourceResult{Data: data}
}

func fetchFromTdx(stockCode, klt string, limit int, adjustFlag string) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetKLineData(stockCode, klt, limit, adjustFlag)
	return &KLineSourceResult{Data: data}
}

func fetchFromMAC(stockCode, klt string, limit int, adjustFlag string) *KLineSourceResult {
	api := NewTdxKLineApi()
	data := api.GetMACKLineData(stockCode, klt, limit, adjustFlag)
	return &KLineSourceResult{Data: data}
}

func fetchFromMACWithTimeout(stockCode, klt string, limit int, timeout time.Duration, adjustFlag string) *KLineSourceResult {
	type result struct {
		rs *KLineSourceResult
		ok bool
	}
	ch := make(chan result, 1)
	go func() {
		rs := fetchFromMAC(stockCode, klt, limit, adjustFlag)
		ch <- result{rs: rs, ok: true}
	}()

	select {
	case r := <-ch:
		return r.rs
	case <-time.After(timeout):
		logger.SugaredLogger.Warnf("fetchFromMAC 超时 (%v)，降级到下一个数据源: code=%s klt=%s", timeout, stockCode, klt)
		return nil
	}
}

func AggregateSinaMonthKLine(dailyData *[]KLineData) *[]KLineData {
	if dailyData == nil || len(*dailyData) == 0 {
		return dailyData
	}
	arr := *dailyData
	grouped := make(map[string][]KLineData)
	var monthKeys []string
	for _, kd := range arr {
		monthKey := extractMonthKey(kd.Day)
		if monthKey == "" {
			continue
		}
		if _, exists := grouped[monthKey]; !exists {
			monthKeys = append(monthKeys, monthKey)
		}
		grouped[monthKey] = append(grouped[monthKey], kd)
	}
	result := make([]KLineData, 0, len(monthKeys))
	for _, mk := range monthKeys {
		group := grouped[mk]
		if len(group) == 0 {
			continue
		}
		first := group[0]
		last := group[len(group)-1]
		highF := -1e18
		lowF := 1e18
		volSum := 0.0
		amtSum := 0.0
		for _, k := range group {
			h, _ := parseFloatToFloat(k.High)
			l, _ := parseFloatToFloat(k.Low)
			v, _ := parseFloatToFloat(k.Volume)
			a, _ := parseFloatToFloat(k.Amount)
			if h > highF {
				highF = h
			}
			if l < lowF {
				lowF = l
			}
			volSum += v
			amtSum += a
		}
		highS := first.High
		lowS := first.Low
		if highF > -1e17 {
			highS = fmt.Sprintf("%.2f", highF)
		}
		if lowF < 1e17 {
			lowS = fmt.Sprintf("%.2f", lowF)
		}
		result = append(result, KLineData{
			Day:           last.Day,
			Open:          first.Open,
			Close:         last.Close,
			High:          highS,
			Low:           lowS,
			Volume:        fmt.Sprintf("%.0f", volSum),
			Amount:        fmt.Sprintf("%.2f", amtSum),
			ChangePercent: last.ChangePercent,
			ChangeValue:   last.ChangeValue,
			Amplitude:     last.Amplitude,
			TurnoverRate:  last.TurnoverRate,
		})
	}
	return &result
}

func extractMonthKey(dayStr string) string {
	s := strings.TrimSpace(dayStr)
	if len(s) >= 7 && s[4] == '-' {
		return s[:7]
	}
	if len(s) >= 6 {
		return s[:4] + "-" + s[4:6]
	}
	if len(s) >= 10 && (strings.Contains(s, "/") || strings.Contains(s, "-")) {
		t, err := time.Parse("2006-01-02", strings.ReplaceAll(s, "/", "-"))
		if err == nil {
			return t.Format("2006-01")
		}
	}
	return ""
}
