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
	sh, sz, bj := uint8(types.MarketSH), uint8(types.MarketSZ), uint8(types.MarketBJ)
	feeds := map[uint8][]proto.UnusualData{sh: {}, sz: {}, bj: {}}
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
	sh, sz, bj := uint8(types.MarketSH), uint8(types.MarketSZ), uint8(types.MarketBJ)
	fail := true
	fetch := func(market uint8, start, count uint32) ([]proto.UnusualData, error) {
		if market == sz && fail {
			return nil, errors.New("offline")
		}
		if market == bj {
			return nil, nil
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
		{bj, "920098", true}, {bj, "833171", true}, {bj, "430047", true},
		{bj, "871981", true}, {bj, "830001", true},
		{bj, "600000", false}, {bj, "301001", false}, {bj, "510300", false},
	} {
		if got := isTdxAShare(tt.market, tt.code); got != tt.want {
			t.Errorf("%d %s: got %v", tt.market, tt.code, got)
		}
	}
}

func TestTdxUnusualPrefixedCode(t *testing.T) {
	cases := []struct {
		market uint16
		code   string
		want   string
	}{
		{uint16(types.MarketSH), "600519", "sh600519"},
		{uint16(types.MarketSZ), "000001", "sz000001"},
		{uint16(types.MarketBJ), "920098", "bj920098"},
	}
	for _, tt := range cases {
		if got := tdxUnusualPrefixedCode(tt.market, tt.code); got != tt.want {
			t.Errorf("market=%d code=%s: got %s want %s", tt.market, tt.code, got, tt.want)
		}
		if bare := tdxUnusualBareCode(tdxUnusualPrefixedCode(tt.market, tt.code)); bare != tt.code {
			t.Errorf("bare roundtrip failed: %s", bare)
		}
	}
}

func TestTdxUnusualNotificationSummary(t *testing.T) {
	items := make([]proto.UnusualData, 10)
	for i := range items {
		items[i] = proto.UnusualData{Time: "09:30:00", Code: fmt.Sprintf("600%03d", i), Desc: "加速拉升", Value: "1.2%"}
	}

	names := map[string]string{"600000": "浦发银行"}
	text := tdxUnusualNotification(items, names)
	// 全量展示（每条一行），不截断、不输出「另有 N 条异动」；缺失名称的条目自动省略名称
	for i := 0; i < 10; i++ {
		code := fmt.Sprintf("600%03d", i)
		if !strings.Contains(text, code) {
			t.Fatalf("missing item %s: %s", code, text)
		}
	}
	if strings.Contains(text, "另有") || strings.Contains(text, "；") ||
		strings.Contains(text, "600001 浦发银行") || !strings.Contains(text, "600000 浦发银行 加速拉升") {
		t.Fatalf("unexpected summary: %s", text)
	}
	if got := tdxUnusualPrefixedCode(uint16(types.MarketSH), "600000"); got != "sh600000" {
		t.Fatalf("unexpected prefixed code: %s", got)
	}
	if got := tdxUnusualPrefixedCode(uint16(types.MarketSZ), "000001"); got != "sz000001" {
		t.Fatalf("unexpected prefixed code: %s", got)
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
