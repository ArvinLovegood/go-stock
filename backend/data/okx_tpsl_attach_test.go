package data

import (
	"math"
	"testing"
)

// 交易所侧 TPSL 触发价计算（okxComputeTPSLPx / okxAlignPxToTick）单测。
// 纯函数无网络依赖；网络相关部分（振幅/配置/凭证）由模拟盘全流程验证覆盖。

func TestOkxAlignPxToTick(t *testing.T) {
	cases := []struct {
		px, tick, want float64
	}{
		{0.12345, 0.001, 0.123},
		{0.10252, 0.0001, 0.1025},
		{102.36, 0.1, 102.4},
		{100.0, 0.001, 100.0},
		{50.0, 0, 50.0},   // tickSz 无效原样返回
		{-1.0, 0.01, -1.0}, // 非法价格原样返回
	}
	for _, c := range cases {
		got := okxAlignPxToTick(c.px, c.tick)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("okxAlignPxToTick(%v, %v) = %v, want %v", c.px, c.tick, got, c.want)
		}
	}
}

func TestOkxComputeTPSLPxBuy(t *testing.T) {
	// 开多：TP 在上方、SL 在下方；tickSz=0.1
	tp, sl, ok := okxComputeTPSLPx("buy", 100, 2, 1, 0.1)
	if !ok {
		t.Fatal("buy 双边计算应成功")
	}
	if tp != 102.0 {
		t.Errorf("TP = %v, want 102", tp)
	}
	if sl != 99.0 {
		t.Errorf("SL = %v, want 99", sl)
	}
}

func TestOkxComputeTPSLSell(t *testing.T) {
	// 开空：TP 在下方、SL 在上方；tickSz=0.001
	tp, sl, ok := okxComputeTPSLPx("sell", 0.1029, 1.5, 1.0, 0.001)
	if !ok {
		t.Fatal("sell 双边计算应成功")
	}
	if tp != 0.101 {
		t.Errorf("TP = %v, want 0.101 (0.1029×(1-1.5%%)=0.10135 → tick 对齐 0.101)", tp)
	}
	if sl != 0.104 {
		t.Errorf("SL = %v, want 0.104 (0.1029×1.01=0.103929 → tick 对齐 0.104)", sl)
	}
}

func TestOkxComputeTPSLPxOneSide(t *testing.T) {
	// 只止盈（slPct=0）：SL 返回 0，仍算成功
	tp, sl, ok := okxComputeTPSLPx("buy", 100, 2, 0, 0.01)
	if !ok || tp != 102.0 || sl != 0 {
		t.Errorf("单边止盈: tp=%v sl=%v ok=%v", tp, sl, ok)
	}
	// 只止损（tpPct=0）
	tp, sl, ok = okxComputeTPSLPx("buy", 100, 0, 1, 0.01)
	if !ok || tp != 0 || sl != 99.0 {
		t.Errorf("单边止损: tp=%v sl=%v ok=%v", tp, sl, ok)
	}
}

func TestOkxComputeTPSLPxInvalid(t *testing.T) {
	// tick 对齐后贴回参考价（阈值过小）→ 非法
	if _, _, ok := okxComputeTPSLPx("buy", 100, 0.0001, 1, 0.5); ok {
		t.Error("TP tick 对齐后贴回参考价应判非法")
	}
	// SL tick 对齐后贴回参考价 → 非法
	if _, _, ok := okxComputeTPSLPx("buy", 100, 2, 0.0001, 0.5); ok {
		t.Error("SL tick 对齐后贴回参考价应判非法")
	}
	// 参考价非法
	if _, _, ok := okxComputeTPSLPx("buy", 0, 2, 1, 0.01); ok {
		t.Error("参考价 <=0 应判非法")
	}
}
