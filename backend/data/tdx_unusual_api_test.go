package data

import (
	"os"
	"testing"

	"github.com/bensema/gotdx/types"
)

func TestTdxUnusualLive(t *testing.T) {
	if os.Getenv("TEST_TDX_UNUSUAL_LIVE") != "1" {
		t.Skip("set TEST_TDX_UNUSUAL_LIVE=1 to probe the TDX server")
	}
	for _, market := range []uint8{uint8(types.MarketSH), uint8(types.MarketSZ)} {
		for _, start := range []uint32{0, 600, 10000} {
			items, err := NewTdxKLineApi().GetStockUnusual(market, start, 600)
			if err != nil {
				t.Errorf("market %d start %d: %v", market, start, err)
				continue
			}
			if len(items) == 0 {
				t.Logf("market=%d start=%d count=0", market, start)
			} else {
				t.Logf("market=%d start=%d count=%d first=%+v last=%+v", market, start, len(items), items[0], items[len(items)-1])
			}
		}
	}
}
