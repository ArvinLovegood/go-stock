package data

import (
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
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

// macExMarketFromStockCode 将港美股/中证指数代码转为扩展行情的 category 值和纯代码。
// 港股：主板 category=31，创业板 category=48（代码 08 开头为创业板）。
// 美股：category=74。
// 中证指数（.CSI 后缀，如 930599.CSI）：category=62（ExCategoryCSIIndex），
//
//	用于 930XXX/000XXX 等中证指数公司发布且无沪/深市镜像代码的指数（如中证高端装备制造 930599）。
//	注意：000300.SH/000852.SH/000510.SH 等有沪市镜像代码的指数仍走 tdxMarketFromStockCode + MAC 主客户端。
//
// A股代码返回 ok=false，应使用 tdxMarketFromStockCode + MAC 客户端。
func macExMarketFromStockCode(stockCode string) (category uint8, code string, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(stockCode))
	if strings.Contains(upper, ".") {
		parts := strings.Split(upper, ".")
		if len(parts) == 2 {
			switch parts[1] {
			case "HK":
				return hkCategoryFromCode(parts[0]), parts[0], true
			case "US":
				return uint8(types.ExCategoryUSStock), parts[0], true
			case "CSI":
				return uint8(types.ExCategoryCSIIndex), parts[0], true
			}
		}
	}
	if strings.HasPrefix(upper, "HK") {
		c := upper[2:]
		return hkCategoryFromCode(c), c, true
	}
	if strings.HasPrefix(upper, "US") {
		return uint8(types.ExCategoryUSStock), upper[2:], true
	}
	if strings.HasPrefix(upper, "GB_") {
		return uint8(types.ExCategoryUSStock), upper[3:], true
	}
	return 0, "", false
}

// hkCategoryFromCode 根据港股代码判断板块分类：08 开头为创业板(48)，其余为主板(31)
func hkCategoryFromCode(code string) uint8 {
	if len(code) >= 2 && code[:2] == "08" {
		return 48 // 香港创业板
	}
	return 31 // 香港主板
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

// convertMACAuctionData 将 gotdx 的 MAC 竞价数据（港美股，proto.MACAuctionItem）转换为统一的 TdxCallAuctionData。
// 与 A股的 proto.AuctionData 差异：Unmatched 为 int32（带符号），用 %d 即可正确格式化。
func convertMACAuctionData(list []proto.MACAuctionItem) []TdxCallAuctionData {
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

// GetMACCallAuction 通过 MAC 主客户端（gotdx.NewMAC，端口7709）获取港美股集合竞价明细。
// MACAuction 走 MAC 主行情协议（0x123D），用 market+code 寻址，必须用 macClient，不能用 macExClient。
func (t *TdxKLineApi) GetMACCallAuction(stockCode string, start uint32, count uint32) *[]TdxCallAuctionData {
	result := &[]TdxCallAuctionData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}
	if count <= 0 {
		count = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.macMu.Lock()
	list, err := t.macClient.MACAuction(market, code, start, count)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACAuction error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		list, err = t.macClient.MACAuction(market, code, start, count)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACAuction retry error: %v", err)
			return result
		}
	}

	converted := convertMACAuctionData(list)
	return &converted
}

// GetCallAuctionAuto 统一集合竞价调度入口：港股/美股走 MAC 主客户端的 MACAuction，A股走主行情客户端的 StockAuction。
func (t *TdxKLineApi) GetCallAuctionAuto(stockCode string, start uint32, count uint32) *[]TdxCallAuctionData {
	market, _ := tdxMarketFromStockCode(stockCode)
	// 港股(MarketHK)/美股(MarketUSA) 走 MAC 主客户端的 MACAuction
	if market == uint8(types.MarketHK) || market == uint8(types.MarketUSA) {
		return t.GetMACCallAuction(stockCode, start, count)
	}
	// A股(SH/SZ/BJ) 走主行情客户端的 StockAuction
	return t.GetCallAuction(stockCode, start, count)
}

// tdxAdjustFromFlag 将前端传入的复权标识字符串映射为 gotdx 的复权常量。
// adjustFlag 取值："qfq"→前复权(AdjustQFQ)、"hfq"→后复权(AdjustHFQ)、"none"/"0"→不复权(AdjustNone)。
// 当 adjustFlag 为空或无法识别时，返回 legacyDefault，保持各调用方原有硬编码默认行为。
func tdxAdjustFromFlag(adjustFlag string, legacyDefault uint16) uint16 {
	switch strings.ToLower(strings.TrimSpace(adjustFlag)) {
	case "qfq":
		return types.AdjustQFQ
	case "hfq":
		return types.AdjustHFQ
	case "none", "0":
		return types.AdjustNone
	default:
		return legacyDefault
	}
}

// adjustFlagFromVariadic 从 variadic 参数中提取第一个复权标识，未提供时返回空串。
func adjustFlagFromVariadic(adjustFlag ...string) string {
	if len(adjustFlag) > 0 {
		return adjustFlag[0]
	}
	return ""
}

func (t *TdxKLineApi) GetKLineData(stockCode string, klt string, limit int, adjustFlag ...string) *[]KLineData {
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

	adjust := tdxAdjustFromFlag(adjustFlagFromVariadic(adjustFlag...), types.AdjustQFQ)

	t.mu.Lock()
	bars, err := t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, adjust)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockKLine error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		bars, err = t.client.StockKLine(uint16(klineType), market, code, 0, uint16(fetchCount), 0, adjust)
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
// A股使用 MAC 主客户端（MACSymbolBars），港美股/中证指数使用 MAC Ex 扩展行情客户端（ExKLine2）
// adjustFlag 可选，控制复权类型："qfq"前复权(默认A股)、"hfq"后复权、"none"/"0"不复权(默认港股)；
// 港美股/中证指数 ExKLine2 协议不支持复权参数，adjustFlag 对其无效；东方财富降级源支持复权。
func (t *TdxKLineApi) GetMACKLineData(stockCode string, klt string, limit int, adjustFlag ...string) *[]KLineData {
	if limit <= 0 {
		limit = 500
	}

	// 海外指数（100.XXX，如 100.DJIA 道琼斯/100.SPX 标普500/100.NDX 纳斯达克/100.HSI 恒生）：
	// MAC 主客户端不识别此类代码（tdxMarketFromStockCode 会落入 default 返回 MarketSH，
	// MACSymbolBars 把 "100.DJIA" 当沪市代码查询返回错误非空数据），直接返回空让回退链走东方财富。
	// 东方财富 secid=100.DJIA 等即为有效格式（convertStockCode 原样返回）。
	if IsGlobalIndexCode(stockCode) {
		return &[]KLineData{}
	}

	flag := adjustFlagFromVariadic(adjustFlag...)

	// 判断是否港美股/中证指数（.CSI 后缀）
	if exMarket, exCode, ok := macExMarketFromStockCode(stockCode); ok {
		// 港美股/中证指数统一走 MAC Ex 扩展行情（ExKLine2，港股主板=31/创业板=48/美股=74/中证指数=62）。
		// 注意：MAC 主客户端（MACSymbolBars）不支持港美股 market=3/4，会忽略 market 参数，
		// 把 5 位港股代码当 A 股 6 位代码处理（如 02202→002202.SZ 金风科技），返回错误的非空数据，
		// 因此港美股不再尝试 MAC 主源，直接走 ExKLine2（ExKLine2 协议不支持复权参数，忽略 adjustFlag）。
		// 中证指数（930XXX 等）无沪/深市镜像代码，MAC 主客户端同样无法寻址，必须走 ExKLine2 + category=62。
		return t.getMACExKLineData(exMarket, exCode, klt, limit)
	}

	// A股走 MAC 客户端，默认前复权
	aAdjust := tdxAdjustFromFlag(flag, types.AdjustQFQ)
	return t.getMACMainKLineDataEx(stockCode, klt, limit, aAdjust)
}

// getMACMainKLineDataEx A股走 MAC 主客户端，adjust 指定复权类型（默认前复权 AdjustQFQ）
func (t *TdxKLineApi) getMACMainKLineDataEx(stockCode string, klt string, limit int, adjust uint16) *[]KLineData {
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
	bars, err := t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, adjust)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACSymbolBars error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		bars, err = t.macClient.MACSymbolBars(market, code, uint16(klineType), 1, 0, fetchCount, adjust)
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

	// 港美股通过扩展行情接口 ExKLine2 获取，不复权
	t.macExMu.Lock()
	items, err := t.macExClient.ExKLine2(market, code, uint16(klineType), 0, fetchCount, 1)
	t.macExMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACEx ExKLine2 error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMACEx(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMACEx error: %v", reconnectErr)
			return result
		}
		t.macExMu.Lock()
		items, err = t.macExClient.ExKLine2(market, code, uint16(klineType), 0, fetchCount, 1)
		t.macExMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACEx ExKLine2 retry error: %v", err)
			return result
		}
	}

	if len(items) == 0 {
		return result
	}

	converted := convertExKLineItem(items)

	if aggN > 1 {
		converted = *AggregateKLineEveryN(&converted, aggN)
	}

	return &converted
}

func convertMACSymbolBar(list []proto.MACSymbolBar) []KLineData {
	result := make([]KLineData, 0, len(list))
	for i, bar := range list {
		day := formatMACDateTime(bar.DateTime.Format("2006-01-02 15:04:05"))
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

// convertExKLineItem 将扩展行情 ExKLine2 返回的 ExKLineItem 列表转为 KLineData
// 用于港股（cat=31/48）和美股（cat=74）K 线
func convertExKLineItem(list []proto.ExKLineItem) []KLineData {
	result := make([]KLineData, 0, len(list))
	for i, item := range list {
		day := formatMACDateTime(item.DateTime.Format("2006-01-02 15:04:05"))
		kd := KLineData{
			Day:    day,
			Open:   fmt.Sprintf("%.2f", item.Open),
			Close:  fmt.Sprintf("%.2f", item.Close),
			High:   fmt.Sprintf("%.2f", item.High),
			Low:    fmt.Sprintf("%.2f", item.Low),
			Volume: fmt.Sprintf("%d", item.Vol),
			Amount: fmt.Sprintf("%.2f", item.Amount),
		}
		if i > 0 {
			prevClose := list[i-1].Close
			if prevClose > 0 {
				kd.ChangePercent = fmt.Sprintf("%.2f", (item.Close-prevClose)/prevClose*100)
				kd.ChangeValue = fmt.Sprintf("%.2f", item.Close-prevClose)
				kd.Amplitude = fmt.Sprintf("%.2f", (item.High-item.Low)/prevClose*100)
			}
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

// MACCapitalFlowData 通达信MAC资金流向数据（个股，单位：元）
type MACCapitalFlowData struct {
	StockCode        string  `md:"股票代码"`
	TodayMainIn      float64 `md:"今日主力流入"`
	TodayMainOut     float64 `md:"今日主力流出"`
	TodayMainNetIn   float64 `md:"今日主力净流入"`
	TodayRetailIn    float64 `md:"今日散户流入"`
	TodayRetailOut   float64 `md:"今日散户流出"`
	TodayRetailNetIn float64 `md:"今日散户净流入"`
	FiveDayMainBuy   float64 `md:"5日主力买入"`
	FiveDayMainSell  float64 `md:"5日主力卖出"`
	FiveDayMainNetIn float64 `md:"5日主力净流入"`
	FiveDaySuperNet  float64 `md:"5日超大单净流入"`
	FiveDayLargeNet  float64 `md:"5日大单净流入"`
	FiveDayMediumNet float64 `md:"5日中单净流入"`
	FiveDaySmallNet  float64 `md:"5日小单净流入"`
}

// GetMACCapitalFlow 通过通达信MAC接口获取个股资金流向数据，
// 包括今日主力/散户流入流出及净流入、5日主力买卖净额与超大/大/中/小单净流入。
// 主要支持 A 股；港美股 MAC 主客户端不一定支持，失败时返回 nil。
func (t *TdxKLineApi) GetMACCapitalFlow(stockCode string) *MACCapitalFlowData {
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return nil
	}

	market, code := tdxMarketFromStockCode(stockCode)

	t.macMu.Lock()
	reply, err := t.macClient.MACCapitalFlow(market, code)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACCapitalFlow error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return nil
		}
		t.macMu.Lock()
		reply, err = t.macClient.MACCapitalFlow(market, code)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACCapitalFlow retry error: %v", err)
			return nil
		}
	}

	if reply == nil {
		return nil
	}
	return &MACCapitalFlowData{
		StockCode:        stockCode,
		TodayMainIn:      reply.TodayMainIn,
		TodayMainOut:     reply.TodayMainOut,
		TodayMainNetIn:   reply.TodayMainNetIn,
		TodayRetailIn:    reply.TodayRetailIn,
		TodayRetailOut:   reply.TodayRetailOut,
		TodayRetailNetIn: reply.TodayRetailNetIn,
		FiveDayMainBuy:   reply.FiveDayMainBuy,
		FiveDayMainSell:  reply.FiveDayMainSell,
		FiveDayMainNetIn: reply.FiveDayMainNetIn,
		FiveDaySuperNet:  reply.FiveDaySuperNet,
		FiveDayLargeNet:  reply.FiveDayLargeNet,
		FiveDayMediumNet: reply.FiveDayMediumNet,
		FiveDaySmallNet:  reply.FiveDaySmallNet,
	}
}

// TdxStockBasic 通达信返回的股票基础信息（代码+名称+昨收+小数位+量单位）
type TdxStockBasic struct {
	StockCode    string  // 带市场前缀的小写代码，如 sh600519 / sz000001 / bj430047
	Code         string  // 不带前缀的纯代码，如 600519
	Name         string  // 证券名称
	Market       string  // 市场标识：SH/SZ/BJ
	PreClose     float64 // 昨收价
	DecimalPoint int8    // 价格小数位数
	VolUnit      uint16  // 成交量单位
}

// GetAllStockList 通过通达信标准行情接口拉取沪深京全市场股票代码+名称列表。
// 即时性高（新股上市当天即可见），不会被封 IP。仅覆盖 A 股，不含港美股。
// 返回结果按市场顺序：深圳 -> 上海 -> 北京，已用 types.IsStock 过滤掉指数/债券等非股票标的，场内 ETF 由 IsOnExchangeFund 放行。
func (t *TdxKLineApi) GetAllStockList() *[]TdxStockBasic {
	result := &[]TdxStockBasic{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}

	// 沪深京三个市场，顺序与 gotdx examples/stock_all 一致
	markets := []types.Market{types.MarketSZ, types.MarketSH, types.MarketBJ}
	for _, market := range markets {
		if err := t.fetchStockListByMarket(market, result); err != nil {
			logger.SugaredLogger.Warnf("TdxKLine GetAllStockList market %s error: %v, retry once...", market.String(), err)
			// 单市场失败不影响其它市场，重连后重试一次
			if reconnectErr := t.reconnect(); reconnectErr != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
				continue
			}
			if retryErr := t.fetchStockListByMarket(market, result); retryErr != nil {
				logger.SugaredLogger.Errorf("TdxKLine GetAllStockList market %s retry error: %v", market.String(), retryErr)
			}
		}
	}
	return result
}

// fetchStockListByMarket 拉取单个市场的全部证券列表，过滤出股票与场内 ETF 后追加到 result
func (t *TdxKLineApi) fetchStockListByMarket(market types.Market, result *[]TdxStockBasic) error {
	t.mu.Lock()
	items, err := t.client.StockAll(market.Uint8())
	t.mu.Unlock()
	if err != nil {
		return err
	}

	marketStr := market.String()
	for _, item := range items {
		// 用 types.IsStock 过滤指数/债券等非股票标的；场内 ETF 另由 IsOnExchangeFund 放行（要求 代码.SH/SZ/BJ 格式）
		symbol := fmt.Sprintf("%s.%s", item.Code, marketStr)
		if !types.IsStock(symbol) && !IsOnExchangeFund(item.Code) {
			continue
		}
		*result = append(*result, TdxStockBasic{
			StockCode:    strings.ToLower(marketStr) + item.Code,
			Code:         item.Code,
			Name:         item.Name,
			Market:       marketStr,
			PreClose:     item.PreClose,
			DecimalPoint: item.DecimalPoint,
			VolUnit:      item.VolUnit,
		})
	}
	return nil
}

// SyncStockBasicToDB 将通达信拉取的沪深京全市场股票列表同步（upsert）到 StockBasic 表。
// 以 Symbol（纯代码）为去重键：已存在则更新名称/市场，不存在则新增。
// 即时性高，适合作为 A 股基础信息的主数据源，定期调用以同步新上市/改名/退市。
func (t *TdxKLineApi) SyncStockBasicToDB() (added, updated int, err error) {
	list := t.GetAllStockList()
	if list == nil || len(*list) == 0 {
		return 0, 0, fmt.Errorf("通达信未返回股票列表数据")
	}

	for _, item := range *list {
		tsCode := fmt.Sprintf("%s.%s", item.Code, item.Market) // 如 600519.SH
		stockInfo := &StockBasic{
			Symbol: item.Code,
			TsCode: tsCode,
			Name:   item.Name,
			Market: item.Market,
		}
		// 以 Symbol 查询是否已存在
		exist := &StockBasic{}
		db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).First(exist)
		if exist.ID == 0 {
			if e := db.Dao.Model(&StockBasic{}).Create(stockInfo); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncStockBasicToDB create %s error: %v", tsCode, e.Error)
				continue
			}
			added++
		} else {
			// 已存在则更新名称与市场（不覆盖 Tushare/东财已补充的行业/地区等字段）
			if e := db.Dao.Model(&StockBasic{}).Where("symbol = ?", stockInfo.Symbol).Updates(map[string]any{
				"ts_code": tsCode,
				"name":    item.Name,
				"market":  item.Market,
			}); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncStockBasicToDB update %s error: %v", tsCode, e.Error)
				continue
			}
			updated++
		}
	}

	logger.SugaredLogger.Infof("SyncStockBasicToDB 完成：新增 %d 条，更新 %d 条", added, updated)
	return added, updated, nil
}

// 扩展行情分类常量（由探测测试实测得到）
const (
	exCategoryHKMain = 31 // 香港主板
	exCategoryHKGem  = 48 // 香港创业板
	exCategoryUS     = 74 // 美国股票
)

// TdxExStockBasic 扩展行情返回的股票基础信息（港股/美股）
type TdxExStockBasic struct {
	Code     string // 代码：港股为 5 位数字（00001），美股为字母代码（AAPL）
	Name     string // 名称
	Category uint8  // 扩展行情分类
	Market   string // 市场标识：HK / US
}

// GetHKUSStockList 通过通达信扩展行情接口拉取港股+美股代码+名称列表。
// 即时性高（新股上市当天即可见），不会被封 IP。覆盖港股主板/创业板、美股。
func (t *TdxKLineApi) GetHKUSStockList() (*[]TdxExStockBasic, *[]TdxExStockBasic, error) {
	hkList := &[]TdxExStockBasic{}
	usList := &[]TdxExStockBasic{}
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return hkList, usList, err
	}

	total, err := t.fetchExCountWithRetry()
	if err != nil {
		return hkList, usList, err
	}

	pageSize := uint16(1000)
	for start := uint32(0); start < total; start += uint32(pageSize) {
		remain := total - start
		size := pageSize
		if remain < uint32(pageSize) {
			size = uint16(remain)
		}
		items, err := t.fetchExListWithRetry(start, size)
		if err != nil {
			logger.SugaredLogger.Warnf("GetHKUSStockList ExList start=%d error: %v", start, err)
			continue
		}
		for _, item := range items {
			switch item.Category {
			case exCategoryHKMain, exCategoryHKGem:
				*hkList = append(*hkList, TdxExStockBasic{
					Code:     item.Code,
					Name:     item.Name,
					Category: item.Category,
					Market:   "HK",
				})
			case exCategoryUS:
				*usList = append(*usList, TdxExStockBasic{
					Code:     item.Code,
					Name:     item.Name,
					Category: item.Category,
					Market:   "US",
				})
			}
		}
		if len(items) == 0 {
			break
		}
	}
	return hkList, usList, nil
}

func (t *TdxKLineApi) fetchExCountWithRetry() (uint32, error) {
	t.macExMu.Lock()
	total, err := t.macExClient.ExCount()
	t.macExMu.Unlock()
	if err != nil {
		logger.SugaredLogger.Warnf("ExCount error: %v, retry...", err)
		if e := t.reconnectMACEx(); e != nil {
			return 0, e
		}
		t.macExMu.Lock()
		total, err = t.macExClient.ExCount()
		t.macExMu.Unlock()
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func (t *TdxKLineApi) fetchExListWithRetry(start uint32, count uint16) ([]proto.ExListItem, error) {
	t.macExMu.Lock()
	items, err := t.macExClient.ExList(start, count)
	t.macExMu.Unlock()
	if err != nil {
		logger.SugaredLogger.Warnf("ExList start=%d error: %v, retry...", start, err)
		if e := t.reconnectMACEx(); e != nil {
			return nil, e
		}
		t.macExMu.Lock()
		items, err = t.macExClient.ExList(start, count)
		t.macExMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

// SyncHKUSStockBasicToDB 将通达信扩展行情拉取的港股/美股列表同步（upsert）到对应表。
// 港股写入 stock_base_info_hk（Code 格式 00001.HK），美股写入 stock_base_info_us。
// 以 Code 为去重键：已存在则更新名称，不存在则新增。
func (t *TdxKLineApi) SyncHKUSStockBasicToDB() (hkAdded, hkUpdated, usAdded, usUpdated int, err error) {
	hkList, usList, err := t.GetHKUSStockList()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// 港股：Code 补 .HK 与 DCToTsCode 风格一致
	for _, item := range *hkList {
		code := fmt.Sprintf("%05s.HK", item.Code) // 如 00001.HK
		stockInfo := &models.StockInfoHK{
			Code: code,
			Name: item.Name,
		}
		exist := &models.StockInfoHK{}
		db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", code).First(exist)
		if exist.ID == 0 {
			if e := db.Dao.Model(&models.StockInfoHK{}).Create(stockInfo); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB create HK %s error: %v", code, e.Error)
				continue
			}
			hkAdded++
		} else {
			if e := db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", code).Updates(map[string]any{
				"name": item.Name,
			}); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB update HK %s error: %v", code, e.Error)
				continue
			}
			hkUpdated++
		}
	}

	// 美股：Code 直接使用字母代码
	for _, item := range *usList {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		stockInfo := &models.StockInfoUS{
			Code: code,
			Name: item.Name,
		}
		exist := &models.StockInfoUS{}
		db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", code).First(exist)
		if exist.ID == 0 {
			if e := db.Dao.Model(&models.StockInfoUS{}).Create(stockInfo); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB create US %s error: %v", code, e.Error)
				continue
			}
			usAdded++
		} else {
			if e := db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", code).Updates(map[string]any{
				"name": item.Name,
			}); e.Error != nil {
				logger.SugaredLogger.Warnf("SyncHKUSStockBasicToDB update US %s error: %v", code, e.Error)
				continue
			}
			usUpdated++
		}
	}

	logger.SugaredLogger.Infof("SyncHKUSStockBasicToDB 完成：港股新增 %d 更新 %d，美股新增 %d 更新 %d",
		hkAdded, hkUpdated, usAdded, usUpdated)
	return hkAdded, hkUpdated, usAdded, usUpdated, nil
}

// === 分时成交数据（gotdx 集成） ===
// A股走标准协议 StockTransaction / GetMinuteTimeData（gotdx.New 主客户端）
// 港美股走 MAC 协议 MACTransactions / MACTickCharts（gotdx.NewMAC 主客户端）
// 调度方式与 GetCallAuctionAuto 一致：按 tdxMarketFromStockCode 返回的 market 分流。

// TdxMinuteTimeData 分时图数据点
type TdxMinuteTimeData struct {
	Time  string  `json:"time"`  // "HH:MM"（A股按交易时间轴生成）或 "HH:MM:SS"（港美股 MAC）
	Price float64 `json:"price"` // 当前价
	Avg   float64 `json:"avg"`   // 均价
	Vol   int     `json:"vol"`   // 成交量
}

// TdxMinuteTimeDataBundle 分时图数据包（含当日行情概览，供前端绘制分时图）
type TdxMinuteTimeDataBundle struct {
	StockCode string              `json:"stockCode"`
	Date      string              `json:"date"`     // "2006-01-02"
	PreClose  float64             `json:"preClose"` // 昨收
	Open      float64             `json:"open"`     // 今开
	High      float64             `json:"high"`
	Low       float64             `json:"low"`
	Close     float64             `json:"close"`
	Vol       uint32              `json:"vol"`    // 总成交量
	Amount    float64             `json:"amount"` // 总成交额
	Items     []TdxMinuteTimeData `json:"items"`  // 分时点
}

// TdxTransactionData 分笔成交明细
type TdxTransactionData struct {
	Time      string  `json:"time"`      // "HH:MM"（A股）或 "HH:MM:SS"（港美股）
	Price     float64 `json:"price"`     // 成交价
	Vol       int64   `json:"vol"`       // 成交量（股）
	Num       int     `json:"num"`       // 笔数（A股为委托笔数 Num，港美股为 TradeCount）
	BuyOrSell int     `json:"buyOrSell"` // 0=买, 1=卖, 2=中性
	Action    string  `json:"action"`    // "BUY"/"SELL"/"NEUTRAL"
}

// GetMinuteTimeDataAuto 统一分时图调度：A股走标准协议，港美股走 MAC MACTickCharts。
func (t *TdxKLineApi) GetMinuteTimeDataAuto(stockCode string) *TdxMinuteTimeDataBundle {
	market, _ := tdxMarketFromStockCode(stockCode)
	if market == uint8(types.MarketHK) || market == uint8(types.MarketUSA) {
		return t.GetMACMinuteTimeData(stockCode)
	}
	return t.GetStockMinuteTimeData(stockCode)
}

// GetTransactionDataAuto 统一分笔成交调度：A股走标准协议，港美股走 MAC MACTransactions。
func (t *TdxKLineApi) GetTransactionDataAuto(stockCode string, start uint32, count uint32) *[]TdxTransactionData {
	market, _ := tdxMarketFromStockCode(stockCode)
	if market == uint8(types.MarketHK) || market == uint8(types.MarketUSA) {
		return t.GetMACTransaction(stockCode, start, count)
	}
	return t.GetStockTransaction(stockCode, start, count)
}

// GetStockMinuteTimeData 通过标准协议获取 A 股当日分时图（gotdx.GetMinuteTimeData）。
// 标准协议返回的 MinuteTimeData 不含时间字段，按 A 股交易时间轴（09:30-11:30 + 13:00-15:00）生成。
func (t *TdxKLineApi) GetStockMinuteTimeData(stockCode string) *TdxMinuteTimeDataBundle {
	result := &TdxMinuteTimeDataBundle{StockCode: stockCode}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	reply, err := t.client.GetMinuteTimeData(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine GetMinuteTimeData error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		reply, err = t.client.GetMinuteTimeData(market, code)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine GetMinuteTimeData retry error: %v", err)
			return result
		}
	}

	if reply == nil || len(reply.List) == 0 {
		return result
	}

	timeSlots := buildAShareMinuteTimeSlots(len(reply.List))
	result.Items = make([]TdxMinuteTimeData, 0, len(reply.List))
	for i, item := range reply.List {
		ts := ""
		if i < len(timeSlots) {
			ts = timeSlots[i]
		}
		result.Items = append(result.Items, TdxMinuteTimeData{
			Time:  ts,
			Price: item.Price,
			Avg:   item.Avg,
			Vol:   item.Vol,
		})
		// 第一个点作为今开
		if i == 0 {
			result.Open = item.Price
		}
		// 累计最高最低
		if item.Price > 0 {
			if result.High == 0 || item.Price > result.High {
				result.High = item.Price
			}
			if result.Low == 0 || item.Price < result.Low {
				result.Low = item.Price
			}
			result.Close = item.Price
		}
		result.Vol += uint32(item.Vol)
	}
	// 标准协议不返回昨收/日期，用当日时间填充
	result.Date = time.Now().Format("2006-01-02")
	return result
}

// buildAShareMinuteTimeSlots 生成 A 股分时时间轴（上午 09:30-11:30 = 120 分钟 + 下午 13:00-15:00 = 120 分钟）
func buildAShareMinuteTimeSlots(n int) []string {
	if n <= 0 {
		return nil
	}
	slots := make([]string, 0, n)
	// 上午 09:30 - 11:30（120 分钟，含 09:30 不含 11:30）
	for i := 0; i < 120 && len(slots) < n; i++ {
		total := 9*60 + 30 + i
		slots = append(slots, fmt.Sprintf("%02d:%02d", total/60, total%60))
	}
	// 下午 13:00 - 15:00（120 分钟，含 13:00 不含 15:00）
	for i := 0; i < 120 && len(slots) < n; i++ {
		total := 13*60 + i
		slots = append(slots, fmt.Sprintf("%02d:%02d", total/60, total%60))
	}
	return slots
}

// GetStockTransaction 通过标准协议获取 A 股当日分笔成交明细（gotdx.StockTransaction）。
// start 为起始偏移，count 为请求条数（最大 500，单次返回不超过 500 条）。
func (t *TdxKLineApi) GetStockTransaction(stockCode string, start uint32, count uint32) *[]TdxTransactionData {
	result := &[]TdxTransactionData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	if count <= 0 {
		count = 500
	}
	if count > 500 {
		count = 500
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockTransaction(market, code, uint16(start), uint16(count))
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockTransaction error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		list, err = t.client.StockTransaction(market, code, uint16(start), uint16(count))
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockTransaction retry error: %v", err)
			return result
		}
	}

	converted := convertTransactionData(list)
	return &converted
}

func convertTransactionData(list []proto.TransactionData) []TdxTransactionData {
	result := make([]TdxTransactionData, 0, len(list))
	for _, item := range list {
		result = append(result, TdxTransactionData{
			Time:      item.Time,
			Price:     item.Price,
			Vol:       int64(item.Vol),
			Num:       item.Num,
			BuyOrSell: item.BuyOrSell,
			Action:    item.Action,
		})
	}
	return result
}

// GetMACMinuteTimeData 通过 MAC 协议获取港美股当日分时图（gotdx.MACTickCharts）。
// MACTickCharts 返回多日分时 + 当日行情概览（PreClose/Open/High/Low/Close/Vol/Amount）。
// 取 Charts 的最后一天作为当日分时数据。
func (t *TdxKLineApi) GetMACMinuteTimeData(stockCode string) *TdxMinuteTimeDataBundle {
	result := &TdxMinuteTimeDataBundle{StockCode: stockCode}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	// days=1 表示查询当日分时
	t.macMu.Lock()
	reply, err := t.macClient.MACTickCharts(market, code, 0, 1)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACTickCharts error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		reply, err = t.macClient.MACTickCharts(market, code, 0, 1)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACTickCharts retry error: %v", err)
			return result
		}
	}

	if reply == nil || len(reply.Charts) == 0 {
		return result
	}

	// 取最后一天（当日）分时
	lastDay := reply.Charts[len(reply.Charts)-1]
	result.Date = lastDay.Date
	result.PreClose = lastDay.PreClose
	result.Open = reply.Open
	result.High = reply.High
	result.Low = reply.Low
	result.Close = reply.Close
	result.Vol = reply.Vol
	result.Amount = reply.Amount

	result.Items = make([]TdxMinuteTimeData, 0, len(lastDay.Ticks))
	for _, tick := range lastDay.Ticks {
		result.Items = append(result.Items, TdxMinuteTimeData{
			Time:  tick.Time,
			Price: tick.Price,
			Avg:   tick.Avg,
			Vol:   int(tick.Vol),
		})
	}
	return result
}

// GetHistoryMinuteTimeDataAuto 拉取历史日期的分时图数据。
// A 股走标准协议 StockHistoryTickChart（用 buildAShareMinuteTimeSlots 生成时间轴），
// 港美股走扩展行情 ExTickChart（date>0 时返回历史分时，自带 Time 字段）。
// tradeDate 格式："2006-01-02"（如 "2026-07-17"），内部转为 YYYYMMDD 传给 gotdx。
func (t *TdxKLineApi) GetHistoryMinuteTimeDataAuto(stockCode, tradeDate string) *TdxMinuteTimeDataBundle {
	result := &TdxMinuteTimeDataBundle{StockCode: stockCode, Date: tradeDate}
	// 日期格式转换："2006-01-02" → "20060102"（uint32）
	parsed, err := time.ParseInLocation("2006-01-02", tradeDate, time.Local)
	if err != nil {
		logger.SugaredLogger.Errorf("TdxKLine parse tradeDate %s error: %v", tradeDate, err)
		return result
	}
	dateUint := uint32(parsed.Year()*10000 + int(parsed.Month())*100 + parsed.Day())

	// 港美股走扩展行情 ExTickChart（category+code 寻址，date>0 表示历史）
	if _, _, ok := macExMarketFromStockCode(stockCode); ok {
		return t.GetExHistoryMinuteTimeData(stockCode, dateUint, tradeDate)
	}
	// A 股走标准协议 StockHistoryTickChart（market+code 寻址）
	return t.GetStockHistoryMinuteTimeData(stockCode, dateUint, tradeDate)
}

// GetStockHistoryMinuteTimeData A 股走标准协议，调用 gotdx StockHistoryTickChart 拉取历史分时图。
// HistoryMinuteTimeData 与 MinuteTimeData 结构相同（无 Time 字段），用 buildAShareMinuteTimeSlots 生成时间轴。
func (t *TdxKLineApi) GetStockHistoryMinuteTimeData(stockCode string, dateUint uint32, tradeDate string) *TdxMinuteTimeDataBundle {
	result := &TdxMinuteTimeDataBundle{StockCode: stockCode, Date: tradeDate}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return result
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockHistoryTickChart(dateUint, market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockHistoryTickChart error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return result
		}
		t.mu.Lock()
		list, err = t.client.StockHistoryTickChart(dateUint, market, code)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockHistoryTickChart retry error: %v", err)
			return result
		}
	}

	if len(list) == 0 {
		return result
	}

	timeSlots := buildAShareMinuteTimeSlots(len(list))
	result.Items = make([]TdxMinuteTimeData, 0, len(list))
	for i, item := range list {
		ts := ""
		if i < len(timeSlots) {
			ts = timeSlots[i]
		}
		result.Items = append(result.Items, TdxMinuteTimeData{
			Time:  ts,
			Price: item.Price,
			Avg:   item.Avg,
			Vol:   item.Vol,
		})
		if i == 0 {
			result.Open = item.Price
		}
		if item.Price > 0 {
			if result.High == 0 || item.Price > result.High {
				result.High = item.Price
			}
			if result.Low == 0 || item.Price < result.Low {
				result.Low = item.Price
			}
			result.Close = item.Price
		}
		result.Vol += uint32(item.Vol)
	}
	return result
}

// GetExHistoryMinuteTimeData 港美股走扩展行情，调用 gotdx ExTickChart 拉取历史分时图。
// ExTickChartData 自带 Time 字符串字段（HH:MM:SS），无需手动生成时间轴。
// 注意：ExTickChart 返回的 Price/Avg 已是真实价格（无需除以 1000）。
func (t *TdxKLineApi) GetExHistoryMinuteTimeData(stockCode string, dateUint uint32, tradeDate string) *TdxMinuteTimeDataBundle {
	result := &TdxMinuteTimeDataBundle{StockCode: stockCode, Date: tradeDate}
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return result
	}
	category, exCode, ok := macExMarketFromStockCode(stockCode)
	if !ok {
		logger.SugaredLogger.Warnf("TdxKLine GetExHistoryMinuteTimeData: not a Ex code: %s", stockCode)
		return result
	}

	t.macExMu.Lock()
	list, err := t.macExClient.ExTickChart(category, exCode, dateUint)
	t.macExMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine ExTickChart error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMACEx(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMACEx error: %v", reconnectErr)
			return result
		}
		t.macExMu.Lock()
		list, err = t.macExClient.ExTickChart(category, exCode, dateUint)
		t.macExMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine ExTickChart retry error: %v", err)
			return result
		}
	}

	if len(list) == 0 {
		return result
	}

	result.Items = make([]TdxMinuteTimeData, 0, len(list))
	for i, item := range list {
		result.Items = append(result.Items, TdxMinuteTimeData{
			Time:  item.Time,
			Price: item.Price,
			Avg:   item.Avg,
			Vol:   item.Vol,
		})
		if i == 0 {
			result.Open = item.Price
		}
		if item.Price > 0 {
			if result.High == 0 || item.Price > result.High {
				result.High = item.Price
			}
			if result.Low == 0 || item.Price < result.Low {
				result.Low = item.Price
			}
			result.Close = item.Price
		}
		result.Vol += uint32(item.Vol)
	}
	return result
}

// GetMACTransaction 通过 MAC 协议获取港美股当日分笔成交明细（gotdx.MACTransactions）。
// start 为起始偏移，count 为请求条数（gotdx 内部已分页，单次最多返回 count 条，最大 1000）。
func (t *TdxKLineApi) GetMACTransaction(stockCode string, start uint32, count uint32) *[]TdxTransactionData {
	result := &[]TdxTransactionData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return result
	}
	if count <= 0 {
		count = 500
	}
	if count > 1000 {
		count = 1000
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.macMu.Lock()
	list, err := t.macClient.MACTransactions(market, code, start, count)
	t.macMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine MACTransactions error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
			return result
		}
		t.macMu.Lock()
		list, err = t.macClient.MACTransactions(market, code, start, count)
		t.macMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine MACTransactions retry error: %v", err)
			return result
		}
	}

	converted := convertMACTransactionData(list)
	return &converted
}

func convertMACTransactionData(list []proto.MACTransactionItem) []TdxTransactionData {
	result := make([]TdxTransactionData, 0, len(list))
	for _, item := range list {
		result = append(result, TdxTransactionData{
			Time:      item.Time,
			Price:     item.Price,
			Vol:       int64(item.Vol),
			Num:       int(item.TradeCount),
			BuyOrSell: int(item.BuyOrSell),
			Action:    macActionFromCode(int(item.BuyOrSell)),
		})
	}
	return result
}

// GetAllTransactionDataAuto 循环分页拉取当日全量分笔成交明细。
// A 股走标准协议 StockFullTransaction（gotdx 内部自动循环 count=600），
// 港美股走 MAC MACTransactions 手动循环 count=1000。
// 返回顺序统一为「从早到晚」（gotdx 原始返回为「最新→最旧」，已自动反转）。
// 默认开启数据库缓存（5 分钟 TTL），skipCache=true 时强制走 gotdx 拉取并刷新缓存。
func (t *TdxKLineApi) GetAllTransactionDataAuto(stockCode string, skipCache bool) *[]TdxTransactionData {
	today := time.Now().Format("2006-01-02")

	// 1. 缓存命中判断（未跳过缓存时）
	if !skipCache {
		meta, err := db.GetStockTransactionCacheMeta(stockCode, today)
		if err == nil && meta != nil && !db.IsTransactionCacheExpired(meta) {
			cached, cacheErr := db.GetStockTransactionCache(stockCode, today)
			if cacheErr == nil && len(cached) > 0 {
				result := make([]TdxTransactionData, 0, len(cached))
				for _, c := range cached {
					result = append(result, TdxTransactionData{
						Time:      c.TradeTime,
						Price:     c.Price,
						Vol:       c.Vol,
						Num:       c.Num,
						BuyOrSell: c.BuyOrSell,
						Action:    c.Action,
					})
				}
				logger.SugaredLogger.Infof("TdxKLine transaction cache hit: %s %s, count=%d", stockCode, today, len(result))
				return &result
			}
		}
	}

	// 2. 缓存未命中或过期，从 gotdx 拉取全量
	market, _ := tdxMarketFromStockCode(stockCode)
	var fetched *[]TdxTransactionData
	if market == uint8(types.MarketHK) || market == uint8(types.MarketUSA) {
		fetched = t.GetMACAllTransaction(stockCode)
	} else {
		fetched = t.GetStockAllTransaction(stockCode)
	}

	// 3. 异步写入缓存（不阻塞返回，失败仅记录日志）
	if fetched != nil && len(*fetched) > 0 {
		items := make([]models.StockTransactionCache, 0, len(*fetched))
		for i, item := range *fetched {
			items = append(items, models.StockTransactionCache{
				StockCode: stockCode,
				TradeDate: today,
				TradeTime: item.Time,
				Seq:       i,
				Price:     item.Price,
				Vol:       item.Vol,
				Num:       item.Num,
				BuyOrSell: item.BuyOrSell,
				Action:    item.Action,
			})
		}
		go func() {
			if err := db.SaveStockTransactionCache(stockCode, today, items); err != nil {
				logger.SugaredLogger.Warnf("TdxKLine save transaction cache error: %v", err)
			}
		}()
	}

	return fetched
}

// GetStockAllTransaction A 股走标准协议，调用 gotdx StockFullTransaction 一次性拉全量分笔成交。
func (t *TdxKLineApi) GetStockAllTransaction(stockCode string) *[]TdxTransactionData {
	empty := &[]TdxTransactionData{}
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return empty
	}
	market, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockFullTransaction(market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockFullTransaction error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return empty
		}
		t.mu.Lock()
		list, err = t.client.StockFullTransaction(market, code)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockFullTransaction retry error: %v", err)
			return empty
		}
	}

	converted := convertTransactionData(list)
	// StockFullTransaction 返回顺序为「最新→最旧」，反转为「从早到晚」
	reverseTdxTransactionData(converted)
	return &converted
}

// GetMACAllTransaction 港美股走 MAC，循环分页拉取全量分笔成交。
// 单次 count=1000，start 递增直到返回少于 count。安全上限 50000 笔。
func (t *TdxKLineApi) GetMACAllTransaction(stockCode string) *[]TdxTransactionData {
	empty := &[]TdxTransactionData{}
	if err := t.ensureMACClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACClient error: %v", err)
		return empty
	}
	market, code := tdxMarketFromStockCode(stockCode)

	const batchSize uint32 = 1000
	const maxTotal = 50000
	var all []TdxTransactionData
	var start uint32 = 0

	for {
		t.macMu.Lock()
		list, err := t.macClient.MACTransactions(market, code, start, batchSize)
		t.macMu.Unlock()

		if err != nil {
			logger.SugaredLogger.Warnf("TdxKLine MACTransactions(start=%d) error: %v, reconnecting...", start, err)
			if reconnectErr := t.reconnectMAC(); reconnectErr != nil {
				logger.SugaredLogger.Errorf("TdxKLine reconnectMAC error: %v", reconnectErr)
				break
			}
			t.macMu.Lock()
			list, err = t.macClient.MACTransactions(market, code, start, batchSize)
			t.macMu.Unlock()
			if err != nil {
				logger.SugaredLogger.Errorf("TdxKLine MACTransactions retry error: %v", err)
				break
			}
		}

		if len(list) == 0 {
			break
		}
		all = append(all, convertMACTransactionData(list)...)
		if len(list) < int(batchSize) {
			break // 已拉完
		}
		start += batchSize
		if len(all) >= maxTotal {
			logger.SugaredLogger.Warnf("TdxKLine MACTransactions hit maxTotal %d, truncating", maxTotal)
			break
		}
	}

	// MAC 返回顺序为「最新→最旧」，反转为「从早到晚」
	reverseTdxTransactionData(all)
	return &all
}

// reverseTdxTransactionData 原地反转切片顺序。
func reverseTdxTransactionData(list []TdxTransactionData) {
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}

// actionStringToBuyOrSell 将 gotdx 返回的 Action 字符串（BUY/SELL/NEUTRAL）转为数值方向。
// 0=买 1=卖 2=中性，未知默认为中性。
func actionStringToBuyOrSell(action string) int {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "BUY":
		return 0
	case "SELL":
		return 1
	default:
		return 2
	}
}

// GetHistoryTransactionDataAuto 拉取历史日期的全量分笔成交明细（带买卖方向）。
// A 股走标准协议 StockHistoryFullTransactionWithTrans，港美股走扩展行情 ExHistoryTransaction。
// tradeDate 格式："2006-01-02"（如 "2026-07-17"），内部转为 YYYYMMDD 传给 gotdx。
// 返回顺序统一为「从早到晚」。默认走数据库缓存（5 分钟 TTL），skipCache=true 强制刷新。
func (t *TdxKLineApi) GetHistoryTransactionDataAuto(stockCode, tradeDate string, skipCache bool) *[]TdxTransactionData {
	empty := &[]TdxTransactionData{}
	// 缓存命中判断
	if !skipCache {
		meta, err := db.GetStockTransactionCacheMeta(stockCode, tradeDate)
		if err == nil && meta != nil && !db.IsTransactionCacheExpired(meta) {
			cached, cacheErr := db.GetStockTransactionCache(stockCode, tradeDate)
			if cacheErr == nil && len(cached) > 0 {
				result := make([]TdxTransactionData, 0, len(cached))
				for _, c := range cached {
					result = append(result, TdxTransactionData{
						Time:      c.TradeTime,
						Price:     c.Price,
						Vol:       c.Vol,
						Num:       c.Num,
						BuyOrSell: c.BuyOrSell,
						Action:    c.Action,
					})
				}
				logger.SugaredLogger.Infof("TdxKLine history transaction cache hit: %s %s, count=%d", stockCode, tradeDate, len(result))
				return &result
			}
		}
	}

	// 日期格式转换："2006-01-02" → "20060102"（uint32）
	parsed, err := time.ParseInLocation("2006-01-02", tradeDate, time.Local)
	if err != nil {
		logger.SugaredLogger.Errorf("TdxKLine parse tradeDate %s error: %v", tradeDate, err)
		return empty
	}
	dateUint := uint32(parsed.Year()*10000 + int(parsed.Month())*100 + parsed.Day())

	market, _ := tdxMarketFromStockCode(stockCode)
	var fetched []TdxTransactionData

	// 港美股走扩展行情 ExHistoryTransaction（category+code 寻址）
	if _, code, ok := macExMarketFromStockCode(stockCode); ok {
		fetched = t.GetExHistoryTransaction(stockCode, code, dateUint)
	} else {
		// A 股走标准协议 StockHistoryFullTransactionWithTrans（market+code 寻址）
		fetched = t.GetStockHistoryTransactionWithTrans(market, stockCode, dateUint)
	}

	if len(fetched) == 0 {
		return empty
	}

	// 反转为「从早到晚」
	reverseTdxTransactionData(fetched)

	// 异步写入缓存
	items := make([]models.StockTransactionCache, 0, len(fetched))
	for i, item := range fetched {
		items = append(items, models.StockTransactionCache{
			StockCode: stockCode,
			TradeDate: tradeDate,
			TradeTime: item.Time,
			Seq:       i,
			Price:     item.Price,
			Vol:       item.Vol,
			Num:       item.Num,
			BuyOrSell: item.BuyOrSell,
			Action:    item.Action,
		})
	}
	go func() {
		if err := db.SaveStockTransactionCache(stockCode, tradeDate, items); err != nil {
			logger.SugaredLogger.Warnf("TdxKLine save history transaction cache error: %v", err)
		}
	}()

	return &fetched
}

// GetStockHistoryTransactionWithTrans A 股走标准协议，调用 gotdx StockHistoryFullTransactionWithTrans 拉取历史全量分笔成交（带方向）。
func (t *TdxKLineApi) GetStockHistoryTransactionWithTrans(market uint8, stockCode string, dateUint uint32) []TdxTransactionData {
	if err := t.ensureClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureClient error: %v", err)
		return nil
	}
	_, code := tdxMarketFromStockCode(stockCode)

	t.mu.Lock()
	list, err := t.client.StockHistoryFullTransactionWithTrans(dateUint, market, code)
	t.mu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine StockHistoryFullTransactionWithTrans error: %v, reconnecting...", err)
		if reconnectErr := t.reconnect(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnect error: %v", reconnectErr)
			return nil
		}
		t.mu.Lock()
		list, err = t.client.StockHistoryFullTransactionWithTrans(dateUint, market, code)
		t.mu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine StockHistoryFullTransactionWithTrans retry error: %v", err)
			return nil
		}
	}

	converted := convertHistoryTransactionWithTrans(list)
	// gotdx 原始返回顺序为「最新→最旧」，统一反转为「从早到晚」以匹配时间轴
	reverseTdxTransactionData(converted)
	return converted
}

// GetExHistoryTransaction 港美股走扩展行情，调用 gotdx ExHistoryTransaction 拉取历史分笔成交。
// 注意：ExHistoryTransaction 协议内部已自动循环拉取全量，无需手动分页。
func (t *TdxKLineApi) GetExHistoryTransaction(stockCode, exCode string, dateUint uint32) []TdxTransactionData {
	if err := t.ensureMACExClient(); err != nil {
		logger.SugaredLogger.Errorf("TdxKLine ensureMACExClient error: %v", err)
		return nil
	}
	category, _, ok := macExMarketFromStockCode(stockCode)
	if !ok {
		logger.SugaredLogger.Warnf("TdxKLine GetExHistoryTransaction: not a Ex code: %s", stockCode)
		return nil
	}

	t.macExMu.Lock()
	list, err := t.macExClient.ExHistoryTransaction(dateUint, category, exCode)
	t.macExMu.Unlock()

	if err != nil {
		logger.SugaredLogger.Warnf("TdxKLine ExHistoryTransaction error: %v, reconnecting...", err)
		if reconnectErr := t.reconnectMACEx(); reconnectErr != nil {
			logger.SugaredLogger.Errorf("TdxKLine reconnectMACEx error: %v", reconnectErr)
			return nil
		}
		t.macExMu.Lock()
		list, err = t.macExClient.ExHistoryTransaction(dateUint, category, exCode)
		t.macExMu.Unlock()
		if err != nil {
			logger.SugaredLogger.Errorf("TdxKLine ExHistoryTransaction retry error: %v", err)
			return nil
		}
	}

	converted := convertExHistoryTransaction(list)
	// gotdx 原始返回顺序为「最新→最旧」，统一反转为「从早到晚」以匹配时间轴
	reverseTdxTransactionData(converted)
	return converted
}

// convertHistoryTransactionWithTrans 转换 A 股历史分笔成交数据。
// HistoryTransactionDataWithTrans 只有 Action 字符串字段，需映射为 BuyOrSell 数值。
func convertHistoryTransactionWithTrans(list []proto.HistoryTransactionDataWithTrans) []TdxTransactionData {
	result := make([]TdxTransactionData, 0, len(list))
	for _, item := range list {
		timeStr := item.Time.Format("15:04:05")
		result = append(result, TdxTransactionData{
			Time:      timeStr,
			Price:     item.Price,
			Vol:       int64(item.Vol),
			Num:       item.Num,
			BuyOrSell: actionStringToBuyOrSell(item.Action),
			Action:    item.Action,
		})
	}
	return result
}

// convertExHistoryTransaction 转换港美股扩展行情历史分笔成交数据。
// ExHistoryTransactionItem.Price 是 uint32 原始值，需除以 1000 转为实际价格（与 ExKLine2 同源单位）。
// ExHistoryTransactionItem.Time 已是 "HH:MM:SS" 格式字符串。
func convertExHistoryTransaction(list []proto.ExHistoryTransactionItem) []TdxTransactionData {
	result := make([]TdxTransactionData, 0, len(list))
	for _, item := range list {
		result = append(result, TdxTransactionData{
			Time:      item.Time,
			Price:     float64(item.Price) / 1000.0,
			Vol:       int64(item.Vol),
			Num:       0, // ExHistoryTransactionItem 无 Num 字段
			BuyOrSell: actionStringToBuyOrSell(item.Action),
			Action:    item.Action,
		})
	}
	return result
}

// macActionFromCode 将 MAC 协议的买卖方向代码转为可读字符串（0=BUY, 1=SELL, 2=NEUTRAL）
func macActionFromCode(code int) string {
	switch code {
	case 0:
		return "BUY"
	case 1:
		return "SELL"
	case 2:
		return "NEUTRAL"
	default:
		return fmt.Sprintf("%d", code)
	}
}
