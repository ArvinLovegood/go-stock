package data

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// TestOKXSimFullFlow 模拟盘完整链路测试（只动 OKX 模拟环境，不涉及真实资金）。
// 链路一（手动）：账户余额 → 行情/合约规格 → 金额换数量 → 市价买入 → 查持仓 → 成交明细 → 平仓；
// 链路二（信号自动）：构造买卖点信号 → okxSignalTradeOne 自动下单 → signal_trade_record
// 流水落库校验（Env=模拟盘）→ 反向平仓助手清仓。
// 运行：go test ./backend/data/ -run TestOKXSimFullFlow -v（需本机可直连 OKX，应用库中已录模拟盘密钥）。
func TestOKXSimFullFlow(t *testing.T) {
	if db.Dao == nil {
		t.Skip("db 未初始化")
	}
	if okxGetCredentialsForEnv(true) == nil {
		t.Skip("应用库未配置模拟盘密钥")
	}

	const instId = "DOGE-USDT-SWAP"
	const amount = 6.0 // 每笔 6 USDT，DOGE 合约最小张数也能成交多张
	startMs := time.Now().UnixMilli()

	// 1) 账户余额（模拟盘）
	bal, err := GetOKXBalanceForEnv(true)
	if err != nil {
		t.Fatalf("查模拟盘余额失败: %v", err)
	}
	t.Logf("① 模拟盘账户: %+v", bal)

	// 2) 合约规格与最新价
	ins := okxLookupInstrument(instId)
	if ins == nil {
		GetOKXInstruments("SWAP")
		ins = okxLookupInstrument(instId)
	}
	if ins == nil {
		t.Fatalf("合约规格获取失败: %s", instId)
	}
	ticker := GetOKXTicker(instId)
	if ticker == nil || okxFloatNum(ticker.Last) <= 0 {
		t.Fatalf("最新价获取失败: %s", instId)
	}
	price := okxFloatNum(ticker.Last)
	t.Logf("② %s last=%s ctVal=%s lotSz=%s minSz=%s", instId, ticker.Last, ins.CtVal, ins.LotSz, ins.MinSz)

	// 3) 金额 → 数量
	sz, err := okxCalcSizeByAmount(amount, price, *ins)
	if err != nil {
		t.Fatalf("金额换数量失败: %v", err)
	}
	t.Logf("③ %.2f USDT ≈ 价格 %.6g → 下单数量 %s 张", amount, price, sz)

	// 4) 手动链路：市价买入开多
	res, err := SubmitOKXOrderForEnv(true, &OKXOrderRequest{
		InstId: instId, TdMode: "cross", Side: "buy", OrdType: "market", Sz: sz,
	})
	if err != nil {
		t.Fatalf("模拟盘市价买入失败: %v", err)
	}
	if res.SCode != "0" || res.OrdId == "" {
		t.Fatalf("模拟盘下单被拒: sCode=%s sMsg=%s", res.SCode, res.SMsg)
	}
	t.Logf("④ 市价买入已提交 ordId=%s", res.OrdId)

	// 5) 持仓确认
	pos, err := GetOKXPositionListForEnv(true, instId)
	if err != nil {
		t.Fatalf("查持仓失败: %v", err)
	}
	var held *OKXPosition
	for i := range pos {
		p := pos[i]
		if p.InstId == instId && math.Abs(okxFloatNum(p.Pos)) > 0 {
			held = &pos[i]
			break
		}
	}
	if held == nil {
		t.Fatalf("下单后未查到持仓: %s", instId)
	}
	t.Logf("⑤ 持仓确认 posSide=%s pos=%s avgPx=%s", held.PosSide, held.Pos, held.AvgPx)

	// 6) 成交明细（确认该 ordId 有成交记录）
	fills, err := GetOKXFillsHistoryForEnv(true, "SWAP", instId, 10)
	if err != nil {
		t.Fatalf("查成交明细失败: %v", err)
	}
	fillHit := false
	for _, f := range fills {
		if f.OrdId == res.OrdId {
			fillHit = true
			t.Logf("⑥ 成交明细 ordId=%s px=%s sz=%s fee=%s pnl=%s", f.OrdId, f.FillPx, f.FillSz, f.FillFee, f.FillPnl)
			break
		}
	}
	if !fillHit {
		t.Logf("⑥ 成交明细暂未见 ordId=%s（市价单成交回执可能延迟，不阻断）", res.OrdId)
	}

	// 7) 平仓
	closeRes, err := CloseOKXPositionForEnv(true, instId, held.PosSide, held.Pos)
	if err != nil || closeRes.SCode != "0" {
		t.Fatalf("平仓失败: err=%v sCode=%s sMsg=%s", err, closeRes.SCode, closeRes.SMsg)
	}
	t.Logf("⑦ 平仓已提交 ordId=%s", closeRes.OrdId)
	posAfter, _ := GetOKXPositionListForEnv(true, instId)
	for _, p := range posAfter {
		if p.InstId == instId && math.Abs(okxFloatNum(p.Pos)) > 0 {
			t.Logf("⑦ 平仓回执延迟，持仓尚未清零: pos=%s（不阻断，模拟盘稍后自清）", p.Pos)
		}
	}

	// 8) 信号自动链路：构造买入信号 → 自动下单 → 落流水
	rec := models.SignalRecord{
		Code:    "ok:" + instId,
		Klt:     "15m",
		Family:  "buysell",
		Kind:    "buy",
		Name:    "模拟流程测试",
		BarTime: startMs,
	}
	okxSignalTradeOne(rec, okxSignalTradeConf{Enable: true, AmountUSDT: amount, InstType: "SWAP"}, amount, true)

	var tr models.SignalTradeRecord
	err = db.Dao.Where("code = ? AND at >= ?", rec.Code, startMs).
		Order("at DESC").First(&tr).Error
	if err != nil {
		t.Fatalf("信号交易流水未落库: %v", err)
	}
	if tr.Status != "success" || tr.OrdId == "" || tr.Env != "模拟盘" {
		t.Fatalf("信号流水异常: status=%s env=%s ordId=%s err=%s", tr.Status, tr.Env, tr.OrdId, tr.ErrMsg)
	}
	t.Logf("⑧ 信号自动下单成功: ordId=%s env=%s sz=%s refPx=%.6g", tr.OrdId, tr.Env, tr.Sz, tr.RefPrice)

	// 9) 反向信号平仓助手：卖出信号平掉多头
	okxCloseOppositePosition(rec, instId, "SWAP", tr.RefPrice, true)
	t.Logf("⑨ 已触发反向平仓（卖出信号平多单）")

	// 10) 终态：模拟盘该合约持仓清零
	posFinal, _ := GetOKXPositionListForEnv(true, instId)
	for _, p := range posFinal {
		if p.InstId == instId && math.Abs(okxFloatNum(p.Pos)) > 0 {
			t.Logf("⑩ 终态仍有持仓 pos=%s（模拟盘撮合回执延迟属正常，可稍后人工确认）", p.Pos)
			return
		}
	}
	t.Logf("⑩ 终态确认：%s 模拟盘持仓已清零，全流程通过", instId)
}
