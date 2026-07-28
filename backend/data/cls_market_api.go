package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClsMarketApi 财联社市场数据接口
type ClsMarketApi struct{}

func NewClsMarketApi() *ClsMarketApi {
	return &ClsMarketApi{}
}

// IndexTlineItem 指数分时数据点
type IndexTlineItem struct {
	Date            int     `json:"date"`             // 日期 YYYYMMDD
	Minute          int     `json:"minute"`           // 时间 HHMM
	LastPx          float64 `json:"last_px"`          // 最新价
	Change          float64 `json:"change"`           // 涨跌幅
	ChangeColor     int     `json:"change_color"`     // 0=跌 1=涨
	Amp             float64 `json:"amp"`              // 振幅
	PreclosePx      float64 `json:"preclose_px"`      // 昨收
	OpenPx          float64 `json:"open_px"`          // 开盘
	ChangePx        float64 `json:"change_px"`        // 涨跌点数
	BusinessAmount  int64   `json:"business_amount"`  // 成交量(股)
	BusinessBalance float64 `json:"business_balance"` // 成交额(元)
}

// IndexTlineResult 指数分时数据返回
type IndexTlineResult struct {
	Date             string           `json:"date"`             // 日期 YYYY-MM-DD
	TotalBalance     float64          `json:"totalBalance"`     // 两市成交额(元)
	PrevBalance      float64          `json:"prevBalance"`      // 上一交易日成交额(元)
	BalanceChange    float64          `json:"balanceChange"`    // 成交额变化(元, 正=放量 负=缩量)
	BalanceChangePct float64          `json:"balanceChangePct"` // 成交额变化百分比(%)
	Items            []IndexTlineItem `json:"items"`            // 分时数据
}

// SectorAnchor 板块异动事件
type SectorAnchor struct {
	SymbolCode string `json:"symbol_code"` // 板块代码
	SymbolName string `json:"symbol_name"` // 板块名称
	ArticleID  int    `json:"article_id"`  // 文章ID
	CTime      string `json:"c_time"`      // 异动时间
	Float      string `json:"float"`       // up=涨, down=跌
}

// MarketEmotion 市场情绪数据
type MarketEmotion struct {
	MarketDegree      string `json:"market_degree"`          // 市场热度 0-100
	ShszBalance       string `json:"shsz_balance"`           // 两市成交额
	ShszBalanceChange string `json:"shsz_balance_change_px"` // 成交额变化
	UpRatio           string `json:"up_ratio"`               // 上涨比例
	UpRatioNum        string `json:"up_ratio_num"`           // 上涨家数
	Performance       string `json:"performance"`            // 表现(涨幅)
	UpOpenRatio       string `json:"up_open_ratio"`          // 开板率
	ProfitRatio       string `json:"profit_ratio"`           // 获利率
	UpDownDis         struct {
		SuspendNum int `json:"suspend_num"` // 停牌
		UpNum      int `json:"up_num"`      // 涨停
		DownNum    int `json:"down_num"`    // 跌停
		RiseNum    int `json:"rise_num"`    // 上涨
		FallNum    int `json:"fall_num"`    // 下跌
		FlatNum    int `json:"flat_num"`    // 平
	} `json:"up_down_dis"`
	LimitUpBoard struct {
		Row1 []string `json:"row1"` // ["一板","二板","三板","高度板"]
		Row2 []string `json:"row2"` // 数量
		Row3 []string `json:"row3"` // 连板率
	} `json:"limit_up_board"`
}

// GetMarketEmotion 获取市场情绪数据
func (a *ClsMarketApi) GetMarketEmotion() (*MarketEmotion, error) {
	url := "https://x-quote.cls.cn/v2/quote/a/stock/emotion?app=CailianpressWeb&os=web&sv=8.7.9"
	body, err := a.httpGet(url)
	if err != nil {
		logger.SugaredLogger.Errorf("获取市场情绪数据失败: %v", err)
		return nil, err
	}
	var resp struct {
		Code int           `json:"code"`
		Msg  string        `json:"msg"`
		Data MarketEmotion `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.SugaredLogger.Errorf("解析市场情绪数据失败: %v", err)
		return nil, err
	}
	return &resp.Data, nil
}

// IndexQuoteItem 指数行情
type IndexQuoteItem struct {
	SecuCode string  `json:"secu_code"` // 指数代码
	SecuName string  `json:"secu_name"` // 指数名称
	LastPx   float64 `json:"last_px"`   // 最新价格
	Change   float64 `json:"change"`    // 涨跌幅(小数, 如0.0152=1.52%)
	ChangePx float64 `json:"change_px"` // 涨跌点数
}

// GetIndexQuotes 获取A股主要指数行情（来源于财联社首页API）
func (a *ClsMarketApi) GetIndexQuotes() ([]IndexQuoteItem, error) {
	url := "https://x-quote.cls.cn/quote/index/home?app=CailianpressWeb&os=web&sv=8.7.9"
	body, err := a.httpGet(url)
	if err != nil {
		logger.SugaredLogger.Errorf("获取指数行情失败: %v", err)
		return nil, err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			IndexQuote []IndexQuoteItem `json:"index_quote"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		logger.SugaredLogger.Errorf("解析指数行情失败: %v", err)
		return nil, err
	}
	return resp.Data.IndexQuote, nil
}

// GetIndexTline 获取指数分时数据
// date 格式: "2026-07-22" 或 "20260722"
func (a *ClsMarketApi) GetIndexTline(date string) (*IndexTlineResult, error) {
	dateStr := strings.ReplaceAll(date, "-", "")
	if len(dateStr) != 8 {
		dateStr = time.Now().Format("20060102")
	}

	// 尝试获取指定日期数据，若为空(非交易日)则回溯最近7天
	data, usedDateStr := a.fetchTlineDataWithFallback(dateStr)

	result := &IndexTlineResult{
		Date:  fmt.Sprintf("%s-%s-%s", usedDateStr[:4], usedDateStr[4:6], usedDateStr[6:8]),
		Items: data,
	}
	for _, item := range data {
		result.TotalBalance += item.BusinessBalance
	}

	// 回溯查找上一交易日成交额
	if result.TotalBalance > 0 {
		parsedDate, pErr := time.Parse("20060102", usedDateStr)
		if pErr == nil {
			for i := 1; i <= 7; i++ {
				prevDate := parsedDate.AddDate(0, 0, -i)
				prevDateStr := prevDate.Format("20060102")
				prevBalance, ok := a.fetchTotalBalance(prevDateStr)
				if ok && prevBalance > 0 {
					result.PrevBalance = prevBalance
					result.BalanceChange = result.TotalBalance - prevBalance
					if prevBalance > 0 {
						result.BalanceChangePct = result.BalanceChange / prevBalance * 100
					}
					break
				}
			}
		}
	}

	return result, nil
}

// fetchTlineDataWithFallback 获取 tline 数据，若指定日期为空则回溯最近交易日
func (a *ClsMarketApi) fetchTlineDataWithFallback(dateStr string) ([]IndexTlineItem, string) {
	// 先尝试指定日期
	data := a.fetchTlineData(dateStr)
	if len(data) > 0 {
		return data, dateStr
	}
	// 回溯最近7天
	parsedDate, err := time.Parse("20060102", dateStr)
	if err != nil {
		return nil, dateStr
	}
	for i := 1; i <= 7; i++ {
		prevDate := parsedDate.AddDate(0, 0, -i)
		prevDateStr := prevDate.Format("20060102")
		prevData := a.fetchTlineData(prevDateStr)
		if len(prevData) > 0 {
			return prevData, prevDateStr
		}
	}
	return nil, dateStr
}

// fetchTlineData 获取指定日期的 tline 数据
func (a *ClsMarketApi) fetchTlineData(dateStr string) []IndexTlineItem {
	url := fmt.Sprintf(
		"https://x-quote.cls.cn/quote/index/tline?app=CailianpressWeb&date=%s&os=web&sv=8.7.9",
		dateStr,
	)
	body, err := a.httpGet(url)
	if err != nil {
		return nil
	}
	var resp struct {
		Code int              `json:"code"`
		Data []IndexTlineItem `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	return resp.Data
}

// fetchTotalBalance 获取指定日期的总成交额
func (a *ClsMarketApi) fetchTotalBalance(dateStr string) (float64, bool) {
	data := a.fetchTlineData(dateStr)
	if len(data) == 0 {
		return 0, false
	}
	var total float64
	for _, item := range data {
		total += item.BusinessBalance
	}
	return total, total > 0
}

// GetSectorAnchors 获取板块异动时间点
// date 格式: "2026-07-22" 或 "20260722"
func (a *ClsMarketApi) GetSectorAnchors(date string) ([]SectorAnchor, error) {
	dateStr := strings.ReplaceAll(date, "-", "")
	if len(dateStr) != 8 {
		dateStr = time.Now().Format("20060102")
	}

	// 尝试指定日期，若为空(非交易日)则回溯最近7天
	data := a.fetchSectorAnchors(dateStr)
	if len(data) > 0 {
		return data, nil
	}
	parsedDate, pErr := time.Parse("20060102", dateStr)
	if pErr == nil {
		for i := 1; i <= 7; i++ {
			prevDate := parsedDate.AddDate(0, 0, -i)
			prevDateStr := prevDate.Format("20060102")
			prevData := a.fetchSectorAnchors(prevDateStr)
			if len(prevData) > 0 {
				return prevData, nil
			}
		}
	}
	return nil, nil
}

// fetchSectorAnchors 获取指定日期的板块异动数据
func (a *ClsMarketApi) fetchSectorAnchors(dateStr string) []SectorAnchor {
	dateFormatted := fmt.Sprintf("%s-%s-%s", dateStr[:4], dateStr[4:6], dateStr[6:8])
	url := fmt.Sprintf(
		"https://www.cls.cn/v3/transaction/anchor?app=CailianpressWeb&cdate=%s&os=web&sv=8.7.9",
		dateFormatted,
	)
	body, err := a.httpGet(url)
	if err != nil {
		return nil
	}
	var resp struct {
		Errno int            `json:"errno"`
		Msg   string         `json:"msg"`
		Data  []SectorAnchor `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	return resp.Data
}

func (a *ClsMarketApi) httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", getRandomUA())
	req.Header.Set("Referer", "https://www.cls.cn/finance")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
