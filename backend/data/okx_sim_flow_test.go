package data

import (
	"math"
	"os"
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

	// 交易所侧 TPSL 的附加与软件侧监控共用「止盈止损」开关：开关关闭时本就不附加（设计如此）。
	// 为实测附加链路，设 OKX_SIM_TEST_TPSL=1 临时启用（vol 模式，无需依赖信号杠杆换算），
	// 测试结束自动还原原值，不留残留配置。
	if os.Getenv("OKX_SIM_TEST_TPSL") == "1" {
		prev := GetSettingConfig().Settings
		prevEnable, prevMode := prev.OKXTPSLEnable, prev.OKXTPSLMode
		if uerr := db.Dao.Model(&Settings{}).Where("id = ?", prev.ID).
			Updates(map[string]any{"okx_tpsl_enable": true, "okx_tpsl_mode": "vol"}).Error; uerr != nil {
			t.Fatalf("临时启用止盈止损失败: %v", uerr)
		}
		defer func() {
			if rerr := db.Dao.Model(&Settings{}).Where("id = ?", prev.ID).
				Updates(map[string]any{"okx_tpsl_enable": prevEnable, "okx_tpsl_mode": prevMode}).Error; rerr != nil {
				t.Logf("还原止盈止损配置失败（请手动检查设置页）: %v", rerr)
			}
		}()
		t.Logf("已临时启用止盈止损（mode=vol），测试结束还原为 enable=%v mode=%q", prevEnable, prevMode)
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

	// 8.1) 交易所侧附带止盈止损验证：开多后市价单应带上未触发条件单，
	//      且触发价分处参考价两侧（TP 高于、SL 低于）。附加被拒时流水 ErrMsg 会记录降级裸单。
	if tr.ErrMsg != "" {
		t.Logf("⑧.1 流水备注（TPSL 触发价或降级信息）: %s", tr.ErrMsg)
	}
	algos, aerr := GetOKXPendingAlgoOrdersEnv(true, instId)
	if aerr != nil {
		t.Logf("⑧.1 查交易所侧条件单失败（网络问题不阻断）: %v", aerr)
	} else if len(algos) == 0 {
		t.Errorf("⑧.1 开仓后交易所侧无未触发条件单：attachAlgoOrds 未生效或被拒降级，请核对日志")
	} else {
		for _, a := range algos {
			t.Logf("⑧.1 交易所侧条件单 algoId=%s ordType=%s side=%s sz=%s TP=%s SL=%s state=%s",
				a.AlgoId, a.OrdType, a.Side, a.Sz, a.TpTriggerPx, a.SlTriggerPx, a.State)
			tp, sl := okxFloatNum(a.TpTriggerPx), okxFloatNum(a.SlTriggerPx)
			// 开多：TP 应高于、SL 应低于参考价
			if tp > 0 && tp <= tr.RefPrice {
				t.Errorf("⑧.1 开多 TP=%.8g 未高于参考价 %.8g（方向错误）", tp, tr.RefPrice)
			}
			if sl > 0 && sl >= tr.RefPrice {
				t.Errorf("⑧.1 开多 SL=%.8g 未低于参考价 %.8g（方向错误）", sl, tr.RefPrice)
			}
			// 顺带校验触发价无浮点长尾（OKX 51006 价格精度拒单的根因）
			if s := a.TpTriggerPx; len(s) > 12 {
				t.Errorf("⑧.1 TP 触发价疑似浮点长尾: %s", s)
			}
			if s := a.SlTriggerPx; len(s) > 12 {
				t.Errorf("⑧.1 SL 触发价疑似浮点长尾: %s", s)
			}
		}
	}

	// 9) 反向信号平仓助手：卖出信号平掉多头
	// 注意：必须用 Kind=sell 的信号记录——步骤 8 的 rec 是 buy 信号，
	// 用它会走「平空」分支（无空头可平）而什么都不做，测不到反向平仓与孤儿单清理。
	revRec := rec
	revRec.Kind = "sell"
	revRec.Name = "模拟流程测试·反向"
	okxCloseOppositePosition(revRec, instId, "SWAP", tr.RefPrice, true)
	t.Logf("⑨ 已触发反向平仓（卖出信号平多单）")

	// 9.1) 孤儿条件单清理验证：反向平仓后旧仓位已不在，交易所侧条件单必须撤掉，
	//      否则同合约冷却后再开仓会被旧单误触发平掉新仓。撤销是异步的，等几秒再查。
	time.Sleep(4 * time.Second)
	algosAfter, aerr2 := GetOKXPendingAlgoOrdersEnv(true, instId)
	if aerr2 != nil {
		t.Logf("⑨.1 查条件单失败（不阻断）: %v", aerr2)
	} else if len(algosAfter) > 0 {
		t.Errorf("⑨.1 反向平仓后孤儿条件单未清理，仍剩 %d 张（会导致新仓被误平）", len(algosAfter))
		for _, a := range algosAfter {
			t.Logf("⑨.1 残留条件单 algoId=%s TP=%s SL=%s", a.AlgoId, a.TpTriggerPx, a.SlTriggerPx)
		}
	} else {
		t.Logf("⑨.1 孤儿条件单已清理：交易所侧无残留未触发条件单")
	}

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
