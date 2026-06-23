package data

import (
	"fmt"
	"go-stock/backend/logger"
	"strings"
	"sync"
	"time"

	gotdx "github.com/bensema/gotdx"
	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

type TdxKLineApi struct {
	client      *gotdx.Client
	macClient   *gotdx.Client
	macExClient *gotdx.Client
	mu          sync.Mutex // 保护 client
	macMu       sync.Mutex // 保护 macClient
	macExMu     sync.Mutex // 保护 macExClient
}

var (
	tdxApiInstance *TdxKLineApi
	tdxApiOnce     sync.Once
)

func NewTdxKLineApi() *TdxKLineApi {
	tdxApiOnce.Do(func() {
		tdxApiInstance = &TdxKLineApi{}
	})
	return tdxApiInstance
}

func (t *TdxKLineApi) newClient() *gotdx.Client {
	cfg := GetSettingConfig()
	timeoutSec := cfg.CrawlTimeOut
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.New(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) newMACClient() *gotdx.Client {
	cfg := GetSettingConfig()
	timeoutSec := cfg.CrawlTimeOut
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.NewMAC(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) newMACExClient() *gotdx.Client {
	cfg := GetSettingConfig()
	timeoutSec := cfg.CrawlTimeOut
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return gotdx.NewMACEx(
		gotdx.WithAutoSelectFastest(true),
		gotdx.WithTimeoutSec(int(timeoutSec)),
	)
}

func (t *TdxKLineApi) ensureClient() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client == nil {
		t.client = t.newClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client != nil {
		t.client.Disconnect()
	}
	t.client = t.newClient()
	return nil
}

func (t *TdxKLineApi) ensureMACClient() error {
	t.macMu.Lock()
	defer t.macMu.Unlock()
	if t.macClient == nil {
		t.macClient = t.newMACClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnectMAC() error {
	t.macMu.Lock()
	defer t.macMu.Unlock()
	if t.macClient != nil {
		t.macClient.Disconnect()
	}
	t.macClient = t.newMACClient()
	return nil
}

func (t *TdxKLineApi) ensureMACExClient() error {
	t.macExMu.Lock()
	defer t.macExMu.Unlock()
	if t.macExClient == nil {
		t.macExClient = t.newMACExClient()
	}
	return nil
}

func (t *TdxKLineApi) reconnectMACEx() error {
	t.macExMu.Lock()
	defer t.macExMu.Unlock()
	if t.macExClient != nil {
		t.macExClient.Disconnect()
	}
	t.macExClient = t.newMACExClient()
	return nil
}

func tdxMarketFromStockCode(stockCode string) (uint8, string) {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(code, ".") {
		parts := strings.Split(code, ".")
		if len(parts) == 2 {
			market := parts[1]
			pureCode := parts[0]
			switch market {
			case "SH", "SS":
				return uint8(types.MarketSH), pureCode
			case "SZ":
				return uint8(types.MarketSZ), pureCode
			case "BJ":
				return uint8(types.MarketBJ), pureCode
			case "HK":
				return uint8(types.MarketHK), pureCode
			case "US":
				return uint8(types.MarketUSA), pureCode
			}
		}
	}
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") || strings.HasPrefix(code, "BJ") {
		marketStr := code[:2]
		pureCode := code[2:]
		switch strings.ToUpper(marketStr) {
		case "SH":
			return uint8(types.MarketSH), pureCode
		case "SZ":
			return uint8(types.MarketSZ), pureCode
		case "BJ":
			return uint8(types.MarketBJ), pureCode
		}
	}
	// hk00700 → MarketHK, "00700"
	if strings.HasPrefix(code, "HK") {
		return uint8(types.MarketHK), code[2:]
	}
	// usAAPL → MarketUSA, "AAPL"
	if strings.HasPrefix(code, "US") {
		return uint8(types.MarketUSA), code[2:]
	}
	// gb_AAPL → MarketUSA, "AAPL"
	if strings.HasPrefix(code, "GB_") {
		return uint8(types.MarketUSA), code[3:]
	}
	if len(code) >= 1 {
		first := code[0:1]
		switch first {
		case "6":
			return uint8(types.MarketSH), code
		case "0", "3":
			return uint8(types.MarketSZ), code
		case "8", "9":
			return uint8(types.MarketBJ), code
		}
	}
	return uint8(types.MarketSH), code
}

// TdxMarketFromStockCode 是 tdxMarketFromStockCode 的导出版本，供外部包调用
func TdxMarketFromStockCode(stockCode string) (uint8, string) {
	return tdxMarketFromStockCode(stockCode)
}

// macExMarketFromStockCode 将港美股代码转为 MAC 扩展行情的 market 值和纯代码
// A股代码返回 ok=false，应使用 tdxMarketFromStockCode + MAC 客户端
func macExMarketFromStockCode(stockCode string) (market uint8, code string, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(upper, ".") {
		parts := strings.Split(upper, ".")
		if len(parts) == 2 {
			switch parts[1] {
			case "HK":
				return uint8(types.ExCategoryHKStock), parts[0], true
			case "US":
				return uint8(types.ExCategoryUSStock), parts[0], true
			}
		}
	}
	if strings.HasPrefix(upper, "HK") {
		return uint8(types.ExCategoryHKStock), upper[2:], true
	}
	if strings.HasPrefix(upper, "US") {
		return uint8(types.ExCategoryUSStock), upper[2:], true
	}
	if strings.HasPrefix(upper, "GB_") {
		return uint8(types.ExCategoryUSStock), upper[3:], true
	}
	return 0, "", false
}

type TdxCallAuctionData struct {
	Time      string `json:"time"`
	Price     string `json:"price"`
	Matched   string `json:"matched"`
	Unmatched string `json:"unmatched"`
	Flag      string `json:"flag"`
}

func (t *TdxKLineApi) GetCallAuction(stockCode string, start uint32, count uint32) *[]TdxCallAuctionData {
	result := &[]TdxCallAuctionData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	if count <= 0 {
		count = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockAuction(market, code, start, count)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockAuction error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		list, err = t.client.StockAuction(market, code, start, count)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockAuction retry error: %v", err)
			return result
		}
	}

	converted := convertAuctionData(list)
	return &converted
}

func convertAuctionData(list []proto.AuctionData) []TdxCallAuctionData {
	result := make([]TdxCallAuctionData, 0, len(list))
	for _, item := range list {
		flagStr := "买盘"
		if item.Flag < 0 {
			flagStr = "卖盘"
		}
		result = append(result, TdxCallAuctionData{
			Time:      item.Time,
			Price:     fmt.Sprintf("%.2f", item.Price),
			Matched:   fmt.Sprintf("%d", item.Matched),
			Unmatched: fmt.Sprintf("%d", item.Unmatched),
			Flag:      flagStr,
		})
	}
	return result
}

func (t *TdxKLineApi) GetCallAuctionLatest(stockCode string) *TdxCallAuctionData {
	data := t.GetCallAuction(stockCode, 0, 500)
	if data == nil || len(*data) == 0 {
		return nil
	}
	last := &(*data)[len(*data)-1]
	return last
}

func (t *TdxKLineApi) GetKLineData(stockCode string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	if limit <= 0 {
		limit = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine: unsupported klt %s", klt)
		return result
	}

	fetchCount := limit
	if aggN > 1 {
		fetchCount = limit * aggN
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.mu.Lock()
	bars, err := t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, types.AdjustQFQ)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockKLine error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		bars, err = t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, types.AdjustQFQ)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockKLine retry error: %v", err)
			return result
		}
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertTdxKLine(bars)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

func tdxKLineTypeFromKlt(klt string) int {
	switch klt {
	case "1":
		return 8
	case "5":
		return 0
	case "15":
		return 1
	case "30":
		return 2
	case "60":
		return 3
	case "101":
		return 4
	case "102":
		return 5
	case "103":
		return 6
	case "104":
		return 10
	case "106":
		return 11
	default:
		return -1
	}
}

func tdxAggregationParams(klt string) (srcKlt string, n int) {
	switch klt {
	case "10":
		return "1", 10
	case "120":
		return "60", 2
	case "105":
		return "102", 26
	default:
		return "", 1
	}
}

// GetMACKLineData 通过 MAC 行情接口获取 K 线数据
// A股使用 MAC 客户端，港美股使用 MAC Ex 客户端
// 港股同时在 MAC 和 MAC Ex 上尝试
func (t *TdxKLineApi) GetMACKLineData(stockCode string, klt string, limit int) *[]KLineData {
	if limit <= 0 {
		limit = 500
	}

	// 判断是否港美股
	if exMarket, exCode, ok := macExMarketFromStockCode(stockCode); ok {
		// 港股：先尝试 MAC 主服务器（MarketHK=3），再尝试 MAC Ex（ExCategoryHKStock=71）
		if IsHKStockCode(stockCode) {
			data := t.getMACMainKLineData(uint8(types.MarketHK), exCode, klt, limit)
			if data != nil && len(*data) > 0 {
				return data
			}
		}
		// MAC Ex 扩展行情
		return t.getMACExKLineData(exMarket, exCode, klt, limit)
	}

	// A股走 MAC 客户端
	return t.getMACMainKLineDataEx(stockCode, klt, limit)
}

// getMACMainKLineDataEx A股走 MAC 主客户端
func (t *TdxKLineApi) getMACMainKLineDataEx(stockCode string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine MAC: unsupported klt %s", klt)
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.macMu.Lock()
	bars, err := t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustQFQ)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACSymbolBars error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		bars, err = t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustQFQ)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACSymbolBars retry error: %v", err)
			return result
		}
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertMACSymbolBar(bars)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

// getMACMainKLineData 通过 MAC 主客户端获取K线（指定 market 和 code）
func (t *TdxKLineApi) getMACMainKLineData(market uint8, code string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	t.macMu.Lock()
	bars, err := t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustNone)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Debugf("TdxKLine MAC main MACSymbolBars for HK error: %v", err)
		return result
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertMACSymbolBar(bars)
	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}
	return &converted
}

// getMACExKLineData 通过 MAC 扩展行情接口获取港美股 K 线数据
func (t *TdxKLineApi) getMACExKLineData(market uint8, code string, klt string, limit int) *[]KLineData {
	result := &[]KLineData{}
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return result
	}

	aggSrc, aggN := tdxAggregationParams(klt)
	actualKlt := klt
	if aggSrc != "" {
		actualKlt = aggSrc
	}

	klineType := tdxKLineTypeFromKlt(actualKlt)
	if klineType < 0 {
		logger.SugaredLogger.Warnf("TdxKLine MAC Ex: unsupported klt %s", klt)
		return result
	}

	fetchCount := uint32(limit)
	if aggN > 1 {
		fetchCount = uint32(limit * aggN)
		if fetchCount > 8000 {
			fetchCount = 8000
		}
	}

	// 港美股不复权（扩展行情不支持复权）
	t.macExMu.Lock()
	bars, err := t.macExClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustNone)
	t.macExMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACEx MACSymbolBars error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMACEx(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMACEx error: %v", reconnectErr)
			return result
		}
		t.macExMu.Lock()
		bars, err = t.macExClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, types.AdjustNone)
		t.macExMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACEx MACSymbolBars retry error: %v", err)
			return result
		}
	}

	if len(bars) == 0 {
		return result
	}

	converted := convertMACSymbolBar(bars)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

func convertMACSymbolBar(list []proto.MACSymbolBar) []KLineData {
	result := make([]KLineData, 0, len(list))
	for i, bar := range list {
		day := formatMACDateTime(bar.DateTime)
		kd := KLineData{
			Day:    day,
			Open:   fmt.Sprintf("%.2f", bar.Open),
			Close:  fmt.Sprintf("%.2f", bar.Close),
			High:   fmt.Sprintf("%.2f", bar.High),
			Low:    fmt.Sprintf("%.2f", bar.Low),
			Volume: fmt.Sprintf("%.0f", bar.Vol),
			Amount: fmt.Sprintf("%.2f", bar.Amount),
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (bar.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", bar.Close-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (bar.High-bar.Low)/prevClose*100)
			}
		}
		if bar.Turnover > 0 {
			kd.TurnoverRate = fmt.Sprintf("%.2f", bar.Turnover)
		}
		result = append(result, kd)
	}
	return result
}

// formatMACDateTime 将 MAC 返回的 DateTime 字符串转为统一格式
// MAC DateTime: "2006-01-02 15:04:05" 或 "2006-01-02 00:00:00"
// 分钟线需要时间: "2006-01-02 15:04"
// 日线及以上只需日期: "2006-01-02"
func formatMACDateTime(dt string) string {
	if len(dt) <= 10 {
		return dt
	}
	// 有时间部分，判断是否为 00:00:00（日线及以上）
	timePart := dt[11:]
	if timePart == "00:00:00" {
		return dt[:10]
	}
	// 分钟线：去掉秒，保留 "YYYY-MM-DD HH:MM"
	if len(dt) >= 16 {
		return dt[:16]
	}
	return dt[:10]
}

func convertTdxKLine(list []proto.SecurityBar) []KLineData {
	result := make([]KLineData, 0, len(list))
	for i, bar := range list {
		kd := KLineData{
			Day:    bar.DateTime.Format("2006-01-02 15:04"),
			Open:   fmt.Sprintf("%.2f", bar.Open),
			Close:  fmt.Sprintf("%.2f", bar.Close),
			High:   fmt.Sprintf("%.2f", bar.High),
			Low:    fmt.Sprintf("%.2f", bar.Low),
			Volume: fmt.Sprintf("%.0f", bar.Vol),
			Amount: fmt.Sprintf("%.2f", bar.Amount),
		}
		if bar.RiseRate != 0 {
			kd.ChangePercent = fmt.Sprintf("%.2f", bar.RiseRate)
			kd.ChangeValue = fmt.Sprintf("%.2f", bar.RisePrice)
		} else if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (bar.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", bar.Close-prevClose)
			}
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.Amplitude = fmt.Sprintf("%.2f", (bar.High-bar.Low)/prevClose*100)
			}
		}
		if bar.Turnover > 0 {
			kd.TurnoverRate = fmt.Sprintf("%.2f", bar.Turnover)
		}
		result = append(result, kd)
	}
	return result
}

type TdxCompanyInfoSection struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type TdxFinanceInfo struct {
	Market              uint8   `json:"market"`
	Code                string  `json:"code"`
	FloatShares         float64 `json:"floatShares"`
	TotalShares         float64 `json:"totalShares"`
	EPS                 float64 `json:"eps"`
	TotalAssets         float64 `json:"totalAssets"`
	CurrentAssets       float64 `json:"currentAssets"`
	FixedAssets         float64 `json:"fixedAssets"`
	IntangibleAssets    float64 `json:"intangibleAssets"`
	ShareholderCount    float64 `json:"shareholderCount"`
	CurrentLiabilities  float64 `json:"currentLiabilities"`
	LongTermLiabilities float64 `json:"longTermLiabilities"`
	CapitalReserve      float64 `json:"capitalReserve"`
	TotalEquity         float64 `json:"totalEquity"`
	OperatingRevenue    float64 `json:"operatingRevenue"`
	OperatingCost       float64 `json:"operatingCost"`
	AccountsReceivable  float64 `json:"accountsReceivable"`
	OperatingProfit     float64 `json:"operatingProfit"`
	InvestmentIncome    float64 `json:"investmentIncome"`
	NetCashFlow         float64 `json:"netCashFlow"`
	Inventory           float64 `json:"inventory"`
	TotalProfit         float64 `json:"totalProfit"`
	AfterTaxProfit      float64 `json:"afterTaxProfit"`
	NetProfit           float64 `json:"netProfit"`
	UndistributedProfit float64 `json:"undistributedProfit"`
	NetAssetsPerShare   float64 `json:"netAssetsPerShare"`
	IPODate             string  `json:"ipoDate"`
	UpdatedDate         string  `json:"updatedDate"`
}

type TdxXDXRItem struct {
	Date            string   `json:"date"`
	Category        uint8    `json:"category"`
	Name            string   `json:"name"`
	Fenhong         *float64 `json:"fenhong"`
	Peigujia        *float64 `json:"peigujia"`
	Songzhuangu     *float64 `json:"songzhuangu"`
	Peigu           *float64 `json:"peigu"`
	Suogu           *float64 `json:"suogu"`
	PreFloatShares  *float64 `json:"preFloatShares"`
	PreTotalShares  *float64 `json:"preTotalShares"`
	PostFloatShares *float64 `json:"postFloatShares"`
	PostTotalShares *float64 `json:"postTotalShares"`
}

type TdxCompanyInfoBundle struct {
	Sections []TdxCompanyInfoSection `json:"sections"`
	XDXR     []TdxXDXRItem           `json:"xdxr"`
	Finance  *TdxFinanceInfo         `json:"finance"`
}

func (t *TdxKLineApi) GetF10Data(stockCode string) *TdxCompanyInfoBundle {
	result := &TdxCompanyInfoBundle{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	bundle, err := t.client.StockF10(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockF10 error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		bundle, err = t.client.StockF10(market, code)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockF10 retry error: %v", err)
			return result
		}
	}

	if bundle == nil {
		return result
	}

	result.Sections = make([]TdxCompanyInfoSection, 0, len(bundle.Sections))
	for _, s := range bundle.Sections {
		result.Sections = append(result.Sections, TdxCompanyInfoSection{
			Name:    s.Name,
			Content: s.Content,
		})
	}

	result.XDXR = make([]TdxXDXRItem, 0, len(bundle.XDXR))
	for _, x := range bundle.XDXR {
		item := TdxXDXRItem{
			Date:     x.Date.Format("2006-01-02"),
			Category: x.Category,
			Name:     x.Name,
		}
		if x.Fenhong != nil {
			v := float64(*x.Fenhong)
			item.Fenhong = &v
		}
		if x.Peigujia != nil {
			v := float64(*x.Peigujia)
			item.Peigujia = &v
		}
		if x.Songzhuangu != nil {
			v := float64(*x.Songzhuangu)
			item.Songzhuangu = &v
		}
		if x.Peigu != nil {
			v := float64(*x.Peigu)
			item.Peigu = &v
		}
		if x.Suogu != nil {
			v := float64(*x.Suogu)
			item.Suogu = &v
		}
		if x.PreFloatShares != nil {
			v := float64(*x.PreFloatShares)
			item.PreFloatShares = &v
		}
		if x.PreTotalShares != nil {
			v := float64(*x.PreTotalShares)
			item.PreTotalShares = &v
		}
		if x.PostFloatShares != nil {
			v := float64(*x.PostFloatShares)
			item.PostFloatShares = &v
		}
		if x.PostTotalShares != nil {
			v := float64(*x.PostTotalShares)
			item.PostTotalShares = &v
		}
		result.XDXR = append(result.XDXR, item)
	}

	if bundle.Finance != nil {
		f := bundle.Finance
		result.Finance = &TdxFinanceInfo{
			Market:              f.Market,
			Code:                f.Code,
			FloatShares:         float64(f.FloatShares),
			TotalShares:         float64(f.TotalShares),
			EPS:                 float64(f.EPS),
			TotalAssets:         float64(f.TotalAssets),
			CurrentAssets:       float64(f.CurrentAssets),
			FixedAssets:         float64(f.FixedAssets),
			IntangibleAssets:    float64(f.IntangibleAssets),
			ShareholderCount:    float64(f.ShareholderCount),
			CurrentLiabilities:  float64(f.CurrentLiabilities),
			LongTermLiabilities: float64(f.LongTermLiabilities),
			CapitalReserve:      float64(f.CapitalReserve),
			TotalEquity:         float64(f.TotalEquity),
			OperatingRevenue:    float64(f.OperatingRevenue),
			OperatingCost:       float64(f.OperatingCost),
			AccountsReceivable:  float64(f.AccountsReceivable),
			OperatingProfit:     float64(f.OperatingProfit),
			InvestmentIncome:    float64(f.InvestmentIncome),
			NetCashFlow:         float64(f.NetCashFlow),
			Inventory:           float64(f.Inventory),
			TotalProfit:         float64(f.TotalProfit),
			AfterTaxProfit:      float64(f.AfterTaxProfit),
			NetProfit:           float64(f.NetProfit),
			UndistributedProfit: float64(f.UndistributedProfit),
			NetAssetsPerShare:   float64(f.NetAssetsPerShare),
		}
		if f.IPODate > 0 {
			result.Finance.IPODate = tdxDateToString(f.IPODate)
		}
		if f.UpdatedDate > 0 {
			result.Finance.UpdatedDate = tdxDateToString(f.UpdatedDate)
		}
	}

	return result
}

type TdxCompanyCategory struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

func (t *TdxKLineApi) GetF10CategoryList(stockCode string) *[]TdxCompanyCategory {
	result := &[]TdxCompanyCategory{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	if _, err := t.client.Connect(); err != nil {
		t.mu.Unlock()
		logger.SugaredLogger.Warnf("TdxKLine Connect error: %v", err)
		return result
	}
	categories, err := t.client.GetCompanyCategories(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyCategories error: %v", err)
		return result
	}

	if categories == nil || len(categories.Categories) == 0 {
		return result
	}

	items := make([]TdxCompanyCategory, 0, len(categories.Categories))
	for _, c := range categories.Categories {
		items = append(items, TdxCompanyCategory{
			Name:     c.Name,
			Filename: c.Filename,
		})
	}
	return &items
}

func (t *TdxKLineApi) GetF10CategoryContent(stockCode string, categoryName string) *TdxCompanyInfoSection {
	result := &TdxCompanyInfoSection{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	if _, err := t.client.Connect(); err != nil {
		t.mu.Unlock()
		logger.SugaredLogger.Warnf("TdxKLine Connect error: %v", err)
		return result
	}
	categories, err := t.client.GetCompanyCategories(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyCategories error: %v", err)
		return result
	}

	if categories == nil {
		return result
	}

	var target *proto.CompanyCategory
	for i := range categories.Categories {
		if categories.Categories[i].Name == categoryName {
			target = &categories.Categories[i]
			break
		}
	}
	if target == nil {
		logger.SugaredLogger.Warnf("TdxKLine category '%s' not found for %s", categoryName, stockCode)
		return result
	}

	t.mu.Lock()
	content, err := t.client.GetCompanyContent(market, code, target.Filename, target.Start, target.Length)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetCompanyContent error: %v", err)
		return result
	}

	result.Name = target.Name
	result.Content = content.Content
	return result
}

func (t *TdxKLineApi) GetFinanceInfo(stockCode string) *TdxFinanceInfo {
	bundle := t.GetF10Data(stockCode)
	if bundle == nil || bundle.Finance == nil {
		return nil
	}
	return bundle.Finance
}

func (t *TdxKLineApi) GetXDXRInfo(stockCode string) *[]TdxXDXRItem {
	bundle := t.GetF10Data(stockCode)
	if bundle == nil {
		return &[]TdxXDXRItem{}
	}
	return &bundle.XDXR
}

func tdxDateToString(d uint32) string {
	if d == 0 {
		return ""
	}
	year := int(d / 10000)
	month := int((d % 10000) / 100)
	day := int(d % 100)
	if year < 1900 || month < 1 || month > 12 || day < 1 || day > 31 {
		return fmt.Sprintf("%d", d)
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func init() {
	_ = time.DateTime
}

// MACBelongBoardItem 股票所属板块信息
type MACBelongBoardItem struct {
	BoardType      string  `json:"boardType" md:"板块类型"`
	BoardCode      string  `json:"boardCode" md:"板块代码"`
	BoardName      string  `json:"boardName" md:"板块名称"`
	Price          float64 `json:"price" md:"板块价格/指数"`
	PreClose       float64 `json:"preClose" md:"板块昨收"`
	LimitUpCount   float64 `json:"limitUpCount" md:"涨停数"`
	LimitDownCount float64 `json:"limitDownCount" md:"跌停数"`
}

// GetMACSymbolBelongBoard 通过 MAC 行情接口获取股票所属板块信息
func (t *TdxKLineApi) GetMACSymbolBelongBoard(stockCode string) *[]MACBelongBoardItem {
	result := &[]MACBelongBoardItem{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}

	market, code := tdxMarketFromStockCode(stockCode)

	t.macMu.Lock()
	items, err := t.macClient.MACSymbolBelongBoard(code, market)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACSymbolBelongBoard error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		items, err = t.macClient.MACSymbolBelongBoard(code, market)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACSymbolBelongBoard retry error: %v", err)
			return result
		}
	}

	converted := make([]MACBelongBoardItem, 0, len(items))
	for _, item := range items {
		converted = append(converted, MACBelongBoardItem{
			BoardType:      item.BoardType,
			BoardCode:      item.BoardCode,
			BoardName:      item.BoardName,
			Price:          item.Price,
			PreClose:       item.PreClose,
			LimitUpCount:   item.LimitUpCount,
			LimitDownCount: item.LimitDownCount,
		})
	}
	return &converted
}
