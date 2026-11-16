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

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	tdxUnusualPageSize = 600
	tdxUnusualMaxPages = 20
	tdxUnusualPreview  = 8
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
	for _, market := range []uint8{uint8(types.MarketSH), uint8(types.MarketSZ)} {
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
	return false
}

func tdxUnusualNotification(items []proto.UnusualData) string {
	var parts []string
	for i, item := range items {
		if i >= tdxUnusualPreview {
			break
		}
		detail := fmt.Sprintf("%s %s %s", item.Time, item.Code, item.Desc)
		if item.Value != "" {
			detail += " " + item.Value
		}
		parts = append(parts, detail)
	}
	text := strings.Join(parts, "；")
	if len(items) > tdxUnusualPreview {
		text += fmt.Sprintf("；另有 %d 条异动", len(items)-tdxUnusualPreview)
	}
	return text
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
	runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    fmt.Sprintf("通达信异动（%d条）", len(items)),
		"isRed":   false,
		"source":  "通达信",
		"content": tdxUnusualNotification(items),
	})
}
