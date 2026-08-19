package data

import (
	"testing"
)

func TestFuturesPositionApi_GetFuturesPositionTrend(t *testing.T) {
	api := NewFuturesPositionApi()
	main := api.GetMainContract("IF")
	if main == "" {
		t.Log("GetMainContract(IF) returned empty (network may be blocked), skip")
	}
	t.Logf("IF 主力合约: %s", main)

	resp := api.GetFuturesPositionTrend("沪深300", "", 10)
	if resp == nil || len(resp.Rows) == 0 {
		t.Fatalf("GetFuturesPositionTrend(IF) returned empty, resp=%v", resp)
	}
	t.Logf("品种=%s 合约=%s 源=%s 指数=%s 行数=%d", resp.Variety, resp.ContractCode, resp.Source, resp.IndexCode, len(resp.Rows))
	last := resp.Rows[len(resp.Rows)-1]
	t.Logf("最新: %+v", last)
	if last.NetPosition != last.LongPosition-last.ShortPosition {
		t.Fatalf("净持仓校验失败: %d != %d - %d", last.NetPosition, last.LongPosition, last.ShortPosition)
	}
}

func TestFuturesPositionApi_CffexRank(t *testing.T) {
	api := NewFuturesPositionApi()
	ranks := api.GetFuturesMemberRank("IM", "")
	if len(ranks) == 0 {
		t.Fatal("GetFuturesMemberRank(IM) returned empty")
	}
	t.Logf("中金所 IM 会员明细 %d 行, 第一行: %+v", len(ranks), ranks[0])
}
