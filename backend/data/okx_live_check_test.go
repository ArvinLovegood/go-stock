package data

import (
	"os"
	"strings"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// TestOKXLivePositionCheck 只读核对：实盘当前持仓/最近委托/成交明细 ↔ 信号交易流水对照。
// 不含任何下单逻辑。仅显式运行：
// OKX_PROBE_DB=D:\go-stock\data\stock.db go test ./backend/data/ -run TestOKXLivePositionCheck -count=1 -v
func TestOKXLivePositionCheck(t *testing.T) {
	if dbPath := os.Getenv("OKX_PROBE_DB"); strings.TrimSpace(dbPath) != "" {
		db.Init(dbPath)
	} else if db.Dao == nil {
		db.Init("../../data/stock.db")
	}

	// ---- 信号交易流水（实盘环境）----
	var recs []models.SignalTradeRecord
	if err := db.Dao.Where("env = ?", "实盘").Order("at DESC").Limit(20).Find(&recs).Error; err != nil {
		t.Fatalf("读取实盘信号流水失败: %v", err)
	}
	t.Logf("==== 实盘信号流水（最近 %d 条）====", len(recs))
	for _, r := range recs {
		t.Logf("id=%d %s klt=%s %s %s sz=%s 金额=%.2f 信号价=%.6g 单价=%.6g ordId=%s 状态=%s 错误=%q barTime=%d at=%d",
			r.ID, r.Code, r.Klt, r.Side, r.Env, r.Sz, r.AmountUSDT, r.SignalPrice, r.RefPrice, r.OrdId, r.Status, r.ErrMsg, r.BarTime, r.At)
	}

	// ---- 实盘当前持仓 ----
	poss, err := GetOKXPositionListForEnv(false, "")
	if err != nil {
		t.Fatalf("实盘持仓查询失败: %v", err)
	}
	t.Logf("==== 实盘当前持仓（%d 笔）====", len(poss))
	for _, p := range poss {
		t.Logf("%s side=%s 数量=%s 均价=%s 标记价=%s 未实现盈亏=%s 杠杆=%s 保证金=%s mgnMode=%s",
			p.InstId, p.PosSide, p.Pos, p.AvgPx, p.MarkPx, p.Upl, p.Lever, p.Margin, p.MgnMode)
	}

	// ---- 实盘最近委托 ----
	orders, err := GetOKXHistoryOrdersForEnv(false)
	if err != nil {
		t.Fatalf("实盘历史委托查询失败: %v", err)
	}
	t.Logf("==== 实盘最近委托（%d 笔）====", len(orders))
	for i, o := range orders {
		if i >= 15 {
			break
		}
		t.Logf("%s side=%s sz=%s 均价=%s 状态=%s reduceOnly=%v ordId=%s 来源=%s 时间=%s",
			o.InstId, o.Side, o.Sz, o.AvgPx, o.State, o.ReduceOnly, o.OrdId, o.Source, o.UTime)
	}

	// ---- 一致性核对：每笔 success 实盘流水应有对应委托，方向一致 ----
	for _, r := range recs {
		if r.Status != "success" || r.OrdId == "" {
			continue
		}
		found := false
		for _, o := range orders {
			if o.OrdId == r.OrdId {
				found = true
				if o.Side != r.Side {
					t.Errorf("方向不一致! 流水 %s side=%s 但委托 side=%s", r.Code, r.Side, o.Side)
				}
				break
			}
		}
		if !found {
			t.Logf("提示: 流水 ordId=%s（%s %s）不在最近委托列表中（可能过旧被截断）", r.OrdId, r.Code, r.Side)
		}
	}
	t.Log("==== 核对完成 ====")
}
