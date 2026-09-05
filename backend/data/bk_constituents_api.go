package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
)

// @Author spark
// @Date 2026/9/5
// @Desc 东方财富板块/概念成分股查询 API
// -----------------------------------------------------------------------------------

// BKConstituentsApi 板块成分股 API（板块与概念共用东财 BK 代码体系，同一接口）
type BKConstituentsApi struct{}

func NewBKConstituentsApi() *BKConstituentsApi {
	return &BKConstituentsApi{}
}

// bkConstituentsResponse 东财 push2 clist 接口返回结构（字段值为 any：停牌等场景会返回 "-"）
type bkConstituentsResponse struct {
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			F12  string `json:"f12"`  // 代码
			F13  int    `json:"f13"`  // 市场
			F14  string `json:"f14"`  // 名称
			F2   any    `json:"f2"`   // 最新价
			F3   any    `json:"f3"`   // 涨跌幅
			F4   any    `json:"f4"`   // 涨跌额
			F5   any    `json:"f5"`   // 成交量（手）
			F6   any    `json:"f6"`   // 成交额（元）
			F8   any    `json:"f8"`   // 换手率
			F10  any    `json:"f10"`  // 量比
			F20  any    `json:"f20"`  // 总市值
			F21  any    `json:"f21"`  // 流通市值
			F23  any    `json:"f23"`  // 市盈率（动态）
			F62  any    `json:"f62"`  // 主力净流入（元）
			F184 any    `json:"f184"` // 主力净流入占比
		} `json:"diff"`
	} `json:"data"`
}

// bkConstituentsHosts 东财 clist 服务主机（push2 实时优先，部分网络环境下 EOF 时回退 push2delay）
var bkConstituentsHosts = []string{"https://push2.eastmoney.com", "https://push2delay.eastmoney.com"}

// fetchBKConstituentsPage 抓取一页成分股数据，主机列表按序回退
func fetchBKConstituentsPage(bkCode string, page, pageSize int) (*bkConstituentsResponse, error) {
	var lastErr error
	for _, host := range bkConstituentsHosts {
		url := fmt.Sprintf("%s/api/qt/clist/get?pn=%d&pz=%d&po=1&np=1&fltt=2&invt=2&fid=f62&fs=b:%s&fields=f12,f13,f14,f2,f3,f4,f5,f6,f8,f10,f20,f21,f23,f62,f184&_=_%d",
			host, page, pageSize, bkCode, time.Now().UnixMilli())
		resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
			SetHeader("Referer", "https://quote.eastmoney.com/center/boardlist.html").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36").
			Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		var result bkConstituentsResponse
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			lastErr = err
			continue
		}
		return &result, nil
	}
	return nil, lastErr
}

// GetBKConstituentStocks 获取板块/概念的全部成分股实时行情
// bkCode 为东财板块代码（如 BK0475），板块（行业）与概念代码通用
// 默认按主力净流入降序；支持分页抓取全量（东财单页上限约 100 条）
func (b *BKConstituentsApi) GetBKConstituentStocks(bkCode string) []models.BKConstituentStock {
	bkCode = strings.TrimSpace(bkCode)
	if bkCode == "" {
		return []models.BKConstituentStock{}
	}

	const pageSize = 100
	var stocks []models.BKConstituentStock
	total := 0
	for page := 1; ; page++ {
		result, err := fetchBKConstituentsPage(bkCode, page, pageSize)
		if err != nil {
			logger.SugaredLogger.Errorf("GetBKConstituentStocks request error: %v", err)
			break
		}
		total = result.Data.Total
		if len(result.Data.Diff) == 0 {
			break
		}
		for _, item := range result.Data.Diff {
			stocks = append(stocks, models.BKConstituentStock{
				Code:             item.F12,
				Name:             item.F14,
				Price:            toFloatOrZero(item.F2),
				ChangePercent:    toFloatOrZero(item.F3),
				Change:           toFloatOrZero(item.F4),
				Volume:           toFloatOrZero(item.F5),
				DealAmount:       toFloatOrZero(item.F6),
				TurnoverRate:     toFloatOrZero(item.F8),
				VolumeRatio:      toFloatOrZero(item.F10),
				TotalMarketCap:   toFloatOrZero(item.F20),
				FlowMarketCap:    toFloatOrZero(item.F21),
				PERatio:          toFloatOrZero(item.F23),
				MainNetInflow:    toFloatOrZero(item.F62),
				MainNetInflowPct: toFloatOrZero(item.F184),
			})
		}
		// 已取完全部成分股
		if len(stocks) >= total || len(result.Data.Diff) < pageSize {
			break
		}
	}
	if stocks == nil {
		stocks = []models.BKConstituentStock{}
	}
	logger.SugaredLogger.Debugf("GetBKConstituentStocks %s: got %d stocks (total=%d)", bkCode, len(stocks), total)
	return stocks
}

// toFloatOrZero 安全转换东财字段值（停牌时为 "-" 等非法数字）
func toFloatOrZero(v any) float64 {
	if v == nil {
		return 0
	}
	f, _ := convertor.ToFloat(convertor.ToString(v))
	return f
}
