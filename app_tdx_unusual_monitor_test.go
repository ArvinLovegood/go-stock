package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

func TestTdxUnusualMonitorBaselinePagingAndReset(t *testing.T) {
	m := &tdxUnusualMonitor{}
	sh, sz := uint8(types.MarketSH), uint8(types.MarketSZ)
	feeds := map[uint8][]proto.UnusualData{sh: {}, sz: {}}
	for i := 0; i < tdxUnusualPageSize+2; i++ {
		feeds[sh] = append(feeds[sh], proto.UnusualData{
			Index: uint16(i), Market: uint16(sh), Code: "600000", Time: "09:30:00", Desc: "加速拉升",
		})
	}
	fetch := func(market uint8, start, count uint32) ([]proto.UnusualData, error) {
		items := feeds[market]
		if int(start) >= len(items) {
			return nil, nil
		}
		end := min(int(start+count), len(items))
		return items[start:end], nil
	}
	if items, errs := m.collect("2026-11-16", fetch); len(items) != 0 || len(errs) != 0 {
		t.Fatalf("baseline produced events or errors: %v %v", items, errs)
	}
	if got := m.markets[sh].next; got != uint32(tdxUnusualPageSize+2) {
		t.Fatalf("baseline did not paginate: %d", got)
	}
	feeds[sh] = append(feeds[sh], proto.UnusualData{Market: uint16(sh), Code: "600001", Time: "09:31:00", Desc: "加速下跌"})
	feeds[sz] = append(feeds[sz], proto.UnusualData{Market: uint16(sz), Code: "000001", Time: "09:31:01", Desc: "主力买入"})
	items, errs := m.collect("2026-11-16", fetch)
	if len(items) != 2 || len(errs) != 0 {
		t.Fatalf("expected two new events: %v %v", items, errs)
	}
	if items, _ := m.collect("2026-11-16", fetch); len(items) != 0 {
		t.Fatalf("duplicate notifications: %v", items)
	}
	feeds[sh] = feeds[sh][:1]
	if items, errs := m.collect("2026-11-17", fetch); len(items) != 0 || len(errs) != 0 {
		t.Fatalf("new day should establish a baseline: %v %v", items, errs)
	}
	m.reset()
	if items, _ := m.collect("2026-11-17", fetch); len(items) != 0 {
		t.Fatalf("re-enabling should establish a baseline: %v", items)
	}
}

func TestTdxUnusualMonitorFailureAndStockFilter(t *testing.T) {
	m := &tdxUnusualMonitor{}
	sh, sz := uint8(types.MarketSH), uint8(types.MarketSZ)
	fail := true
	fetch := func(market uint8, start, count uint32) ([]proto.UnusualData, error) {
		if market == sz && fail {
			return nil, errors.New("offline")
		}
		if start > 0 {
			return nil, nil
		}
		return []proto.UnusualData{{Market: uint16(market), Code: map[uint8]string{sh: "600000", sz: "000001"}[market], Time: "09:30:00", Desc: "拉升"}}, nil
	}
	items, errs := m.collect("2026-11-16", fetch)
	if len(items) != 0 || len(errs) != 1 || m.markets[sz].initialized {
		t.Fatalf("failed market must not initialize: %v %v", items, errs)
	}
	fail = false
	items, errs = m.collect("2026-11-16", fetch)
	if len(items) != 0 || len(errs) != 0 || !m.markets[sz].initialized {
		t.Fatalf("recovered market must baseline without old alerts: %v %v", items, errs)
	}
	for _, tt := range []struct {
		market uint8
		code   string
		want   bool
	}{
		{sh, "688001", true}, {sz, "301001", true}, {sh, "510300", false},
		{sz, "200001", false}, {sh, "6000XX", false},
	} {
		if got := isTdxAShare(tt.market, tt.code); got != tt.want {
			t.Errorf("%d %s: got %v", tt.market, tt.code, got)
		}
	}
}

func TestTdxUnusualNotificationSummary(t *testing.T) {
	items := make([]proto.UnusualData, 10)
	for i := range items {
		items[i] = proto.UnusualData{Time: "09:30:00", Code: fmt.Sprintf("600%03d", i), Desc: "加速拉升", Value: "1.2%"}
	}

	text := tdxUnusualNotification(items)
	if !strings.Contains(text, "600000") || strings.Contains(text, "600008") || !strings.Contains(text, "另有 2 条异动") {
		t.Fatalf("unexpected summary: %s", text)
	}
}

func TestTdxUnusualMonitorResumesAfterPageError(t *testing.T) {
	m := &tdxUnusualMonitor{}
	sh := uint8(types.MarketSH)
	failed := true
	fetch := func(market uint8, start, count uint32) ([]proto.UnusualData, error) {
		if market != sh {
			return nil, nil
		}
		if start == 0 {
			return make([]proto.UnusualData, count), nil
		}
		if failed {
			return nil, errors.New("temporary error")
		}
		return []proto.UnusualData{{Market: uint16(sh), Code: "600000", Time: "09:30:00", Desc: "拉升"}}, nil
	}
	if _, errs := m.collect("2026-11-16", fetch); len(errs) != 1 || m.markets[sh].next != tdxUnusualPageSize || m.markets[sh].initialized {
		t.Fatalf("partial baseline lost progress: %+v %v", m.markets[sh], errs)
	}
	failed = false
	items, errs := m.collect("2026-11-16", fetch)
	if len(items) != 0 || len(errs) != 0 || !m.markets[sh].initialized || m.markets[sh].next != tdxUnusualPageSize+1 {
		t.Fatalf("recovered baseline incorrect: %+v %v %v", m.markets[sh], items, errs)
	}
}
