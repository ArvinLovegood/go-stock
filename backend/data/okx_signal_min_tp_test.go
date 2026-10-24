package data

import (
	"math"
	"testing"
)

// 开单最低止盈收益率估算（okxSignalTpYieldPct）单测。
// 纯函数无网络依赖；配置读取/振幅拉取由调用方（okxSignalTradeOne 门槛）与模拟盘全流程验证覆盖。

func TestOkxSignalTpYieldPctFixed(t *testing.T) {
	// fixed 模式：预期止盈收益率 = 固定止盈收益率本身，与杠杆/振幅无关
	cfg := okxTPSLConf{Mode: "fixed", TpPct: 5}
	if got := okxSignalTpYieldPct(cfg, 0, 0); got != 5 {
		t.Errorf("fixed 模式应返回 TpPct=5，got %v", got)
	}
	// fixed 模式未配置止盈（TpPct=0）→ 0（无法估算，不拦截）
	cfg = okxTPSLConf{Mode: "fixed", TpPct: 0}
	if got := okxSignalTpYieldPct(cfg, 2, 10); got != 0 {
		t.Errorf("fixed 未配置止盈应返回 0，got %v", got)
	}
}

func TestOkxSignalTpYieldPctVol(t *testing.T) {
	cfg := okxTPSLConf{Mode: "vol", TpMult: 1.5}
	// vol 模式：振幅 × 止盈倍数 × 杠杆（保证金口径）
	if got := okxSignalTpYieldPct(cfg, 0.4, 10); math.Abs(got-6) > 1e-9 {
		t.Errorf("vol 模式应为 0.4×1.5×10=6%%，got %v", got)
	}
	// 杠杆未知（<=0）按兜底 10x 估算，与监控兜底口径一致
	if got := okxSignalTpYieldPct(cfg, 0.4, 0); math.Abs(got-6) > 1e-9 {
		t.Errorf("杠杆未知应按 10x 兜底估算得 6%%，got %v", got)
	}
	// 振幅取不到 → 0（无法估算，不拦截）
	if got := okxSignalTpYieldPct(cfg, 0, 10); got != 0 {
		t.Errorf("振幅为 0 应返回 0，got %v", got)
	}
	// 显式关闭止盈（止盈倍数=0）→ 0（无止盈线，不拦截）
	cfg = okxTPSLConf{Mode: "vol", TpMult: 0}
	if got := okxSignalTpYieldPct(cfg, 0.4, 10); got != 0 {
		t.Errorf("止盈倍数为 0 应返回 0，got %v", got)
	}
}
