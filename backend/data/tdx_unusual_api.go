package data

import (
	"fmt"

	"github.com/bensema/gotdx/proto"
)

// GetStockUnusual retrieves one page of the TDX main-market unusual events.
func (t *TdxKLineApi) GetStockUnusual(market uint8, start, count uint32) ([]proto.UnusualData, error) {
	if err := t.ensureClient(); err != nil {
		return nil, fmt.Errorf("初始化通达信异动连接: %w", err)
	}
	t.mu.Lock()
	items, err := t.client.StockUnusual(market, start, count)
	t.mu.Unlock()
	if err == nil {
		return items, nil
	}
	if reconnectErr := t.reconnect(); reconnectErr != nil {
		return nil, fmt.Errorf("重连通达信异动: %w", reconnectErr)
	}
	t.mu.Lock()
	items, err = t.client.StockUnusual(market, start, count)
	t.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("获取通达信异动 market=%d: %w", market, err)
	}
	return items, nil
}
