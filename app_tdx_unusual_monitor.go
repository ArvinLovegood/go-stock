package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm/clause"
)

const (
	tdxUnusualPageSize = 600
	tdxUnusualMaxPages = 20
	tdxUnusualPreview  = 8
	// 通知展示全部条目（前端限高滚动折叠），名称解析的实时行情批取单次上限
	tdxUnusualNameFetchLimit = 60
)

type unusualCursor struct {
	next        uint32
	initialized bool
}

type tdxUnusualMonitor struct {
	mu                sync.Mutex
	generation        atomic.Uint64
	appliedGeneration uint64
	date              string
	markets           map[uint8]*unusualCursor
}

type unusualFetcher func(market uint8, start, count uint32) ([]proto.UnusualData, error)

func (m *tdxUnusualMonitor) reset() {
	m.generation.Add(1)
}

// collect runs with m.mu held; the first complete scan establishes a baseline.
func (m *tdxUnusualMonitor) collect(date string, fetch unusualFetcher) ([]proto.UnusualData, []error) {
	generation := m.generation.Load()
	if m.date != date || m.appliedGeneration != generation {
		m.date = date
		m.appliedGeneration = generation
		m.markets = make(map[uint8]*unusualCursor)
	}
	var fresh []proto.UnusualData
	var errs []error
	for _, market := range []uint8{uint8(types.MarketSH), uint8(types.MarketSZ), uint8(types.MarketBJ)} {
		state := m.markets[market]
		if state == nil {
			state = &unusualCursor{}
			m.markets[market] = state
		}
		for page := 0; page < tdxUnusualMaxPages; page++ {
			items, err := fetch(market, state.next, tdxUnusualPageSize)
			if err != nil {
				errs = append(errs, fmt.Errorf("market=%d offset=%d: %w", market, state.next, err))
				break
			}
			if state.initialized {
				for _, item := range items {
					if isTdxAShare(market, item.Code) && item.Market == uint16(market) &&
						item.Desc != "" && item.Time != "" {
						fresh = append(fresh, item)
					}
				}
			}
			state.next += uint32(len(items))
			if len(items) < tdxUnusualPageSize {
				state.initialized = true
				break
			}
			if page == tdxUnusualMaxPages-1 {
				errs = append(errs, fmt.Errorf("market=%d 异动分页达到本轮上限，后续继续读取", market))
			}
		}
	}
	return fresh, errs
}

func isTdxAShare(market uint8, code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	if market == uint8(types.MarketSH) {
		return strings.HasPrefix(code, "600") || strings.HasPrefix(code, "601") ||
			strings.HasPrefix(code, "603") || strings.HasPrefix(code, "605") ||
			strings.HasPrefix(code, "688") || strings.HasPrefix(code, "689")
	}
	if market == uint8(types.MarketSZ) {
		return strings.HasPrefix(code, "000") || strings.HasPrefix(code, "001") ||
			strings.HasPrefix(code, "002") || strings.HasPrefix(code, "003") ||
			strings.HasPrefix(code, "300") || strings.HasPrefix(code, "301")
	}
	// 北交所：43xxxx（转板/老三板）、82/83/87/88xxxx、920xxx
	if market == uint8(types.MarketBJ) {
		return strings.HasPrefix(code, "43") || strings.HasPrefix(code, "82") ||
			strings.HasPrefix(code, "83") || strings.HasPrefix(code, "87") ||
			strings.HasPrefix(code, "88") || strings.HasPrefix(code, "92")
	}
	return false
}

// tdxUnusualPrefixedCode 将异动事件的 6 位代码转成带市场前缀的行情代码（sh600519 / sz000001 / bj920098），
// 与 StockInfo 缓存表及行情批取接口的代码格式保持一致。
func tdxUnusualPrefixedCode(market uint16, code string) string {
	switch uint8(market) {
	case uint8(types.MarketSH):
		return "sh" + code
	case uint8(types.MarketBJ):
		return "bj" + code
	default:
		return "sz" + code
	}
}

// tdxUnusualBareCode 去掉行情代码的市场前缀，还原 6 位代码。
func tdxUnusualBareCode(code string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(code, "sh"), "sz"), "bj")
}

// tdxUnusualStockNames 解析通知条目的股票名称：
// 优先查本地 StockInfo 缓存表（行情批取会持续写入），未命中的再批量拉一次实时行情补齐。
// 名称缺失（网络失败等）不阻塞通知，回退为只展示代码。实时行情批取单次上限 60 个代码，
// 超出部分留空（本地缓存命中不受限）。
func tdxUnusualStockNames(items []proto.UnusualData) map[string]string {
	if len(items) == 0 {
		return nil
	}
	codes := make([]string, 0, len(items))
	for _, item := range items {
		codes = append(codes, tdxUnusualPrefixedCode(item.Market, item.Code))
	}

	names := make(map[string]string)
	var cached []data.StockInfo
	if err := db.Dao.Select("code", "name").Where("code IN ?", codes).Find(&cached).Error; err == nil {
		for _, si := range cached {
			if si.Name != "" {
				names[tdxUnusualBareCode(si.Code)] = si.Name
			}
		}
	}

	missing := make([]string, 0, len(codes))
	for _, code := range codes {
		if names[tdxUnusualBareCode(code)] == "" && !slice.Contain(missing, code) {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		if len(missing) > tdxUnusualNameFetchLimit {
			missing = missing[:tdxUnusualNameFetchLimit]
		}
		infos, err := data.NewStockDataApi().GetStockCodeRealTimeData(missing...)
		if err != nil {
			logger.SugaredLogger.Warnf("通达信异动通知: 批量获取股票名称失败: %v", err)
		}
		for _, si := range *infos {
			if si.Name != "" {
				names[tdxUnusualBareCode(si.Code)] = si.Name
			}
		}
	}
	return names
}

// saveTdxUnusualEvents 将本轮新采集的异动事件落库（tdx_unusual_events 表），
// 按唯一键（日期+时间+市场+代码+类型）去重，名称缺失时留空。失败只记日志，不影响通知。
func saveTdxUnusualEvents(items []proto.UnusualData, names map[string]string) {
	if len(items) == 0 {
		return
	}
	date := time.Now().In(ShanghaiTimezone).Format("2006-01-02")
	events := make([]models.TdxUnusualEvent, 0, len(items))
	for _, item := range items {
		events = append(events, models.TdxUnusualEvent{
			ChangeDate: date,
			ChangeTime: item.Time,
			Market:     int(item.Market),
			StockCode:  item.Code,
			StockName:  names[item.Code],
			TypeName:   item.Desc,
			Value:      item.Value,
		})
	}
	if err := db.Dao.Clauses(clause.OnConflict{DoNothing: true}).Create(&events).Error; err != nil {
		logger.SugaredLogger.Warnf("通达信异动事件落库失败: %v", err)
	}
}

// tdxUnusualNotification 通知文本：每条异动一行（时间 代码 名称 类型 数值），一目了然；
// 名称缺失时自动省略。前端以 white-space:pre-line 渲染并限高滚动，超长自动折叠。
func tdxUnusualNotification(items []proto.UnusualData, names map[string]string) string {
	var lines []string
	for _, item := range items {
		fields := []string{item.Time, item.Code}
		if name := names[item.Code]; name != "" {
			fields = append(fields, name)
		}
		fields = append(fields, item.Desc)
		if item.Value != "" {
			fields = append(fields, item.Value)
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	return strings.Join(lines, "\n")
}

// tdxUnusualNotificationLines 通知逐行内容 + 多空倾向（1=利好 -1=利空 0=中性），
// 供前端按行着色：利好红字、利空绿字。
func tdxUnusualNotificationLines(items []proto.UnusualData, names map[string]string) []map[string]any {
	lines := make([]map[string]any, 0, len(items))
	for _, item := range items {
		fields := []string{item.Time, item.Code}
		if name := names[item.Code]; name != "" {
			fields = append(fields, name)
		}
		fields = append(fields, item.Desc)
		if item.Value != "" {
			fields = append(fields, item.Value)
		}
		lines = append(lines, map[string]any{
			"text":  strings.Join(fields, " "),
			"trend": data.TdxUnusualTrend(item.Desc),
		})
	}
	return lines
}

func (a *App) pollTdxUnusual() {
	m := &a.tdxUnusualMonitor
	if !m.mu.TryLock() {
		return
	}
	defer m.mu.Unlock()
	var settings data.Settings
	if err := db.Dao.Select("tdx_unusual_notify").First(&settings).Error; err != nil {
		logger.SugaredLogger.Errorf("读取通达信异动通知设置失败: %v", err)
		return
	}
	if !settings.TdxUnusualNotify {
		m.date, m.markets = "", nil
		return
	}
	now := time.Now().In(ShanghaiTimezone)
	if !isTradingTime(now) {
		return
	}
	generation := m.generation.Load()
	items, errs := m.collect(now.Format("2006-01-02"), data.NewTdxKLineApi().GetStockUnusual)
	for _, err := range errs {
		logger.SugaredLogger.Warnf("通达信异动通知: %v", err)
	}
	if len(items) == 0 || a.ctx == nil || generation != m.generation.Load() {
		return
	}
	if err := db.Dao.Select("tdx_unusual_notify").First(&settings).Error; err != nil {
		logger.SugaredLogger.Errorf("读取通达信异动通知设置失败: %v", err)
		return
	}
	if !settings.TdxUnusualNotify {
		m.date, m.markets = "", nil
		return
	}
	names := tdxUnusualStockNames(items)
	saveTdxUnusualEvents(items, names)
	runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    fmt.Sprintf("通达信异动（%d条）", len(items)),
		"isRed":   false,
		"source":  "通达信",
		"content": tdxUnusualNotification(items, names),
		"lines":   tdxUnusualNotificationLines(items, names),
	})
}
