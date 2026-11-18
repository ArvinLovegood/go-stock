package data

import (
	"math"
	"testing"
)

// 样本 < smiMinSamples 时回退经典阈值 ±40，区域武装 + 双确认状态机与前端 computeSmiSignals 同语义
func TestComputeSmiSignalsArmAndConfirm(t *testing.T) {
	n := 20
	smi := nanSlice(n)
	for i := 0; i < n; i++ {
		smi[i] = 0
	}
	signal := nanSlice(n)
	// 底部武装@10（v<=-40），bar12 脱离阈值且上穿 Signal → 买点@12
	smi[10] = -45
	smi[11] = -42
	smi[12] = -20
	signal[12] = -30
	// 顶部武装@15（v>=40），bar17 脱离阈值且下穿 Signal → 卖点@17
	smi[15] = 45
	smi[17] = 20
	signal[17] = 30

	bottoms, tops := smiZoneThresholds(smi)
	// 20 根 < 120 有效样本 → 全程回退 ±40
	for i := 0; i < n; i++ {
		if bottoms[i] != smiFallbackBottom || tops[i] != smiFallbackTop {
			t.Fatalf("样本不足应回退 ±40, i=%d bottom=%v top=%v", i, bottoms[i], tops[i])
		}
	}
	buys, sells := computeSmiSignals(smi, signal, bottoms, tops, 10)
	if len(buys) != 1 || buys[0] != 12 {
		t.Fatalf("应产生买点@12, buys=%v", buys)
	}
	if len(sells) != 1 || sells[0] != 17 {
		t.Fatalf("应产生卖点@17, sells=%v", sells)
	}
}

// 武装后 confirmBars 根内未确认则作废
func TestComputeSmiSignalsTimeout(t *testing.T) {
	n := 20
	smi := nanSlice(n)
	for i := 0; i < n; i++ {
		smi[i] = 0
	}
	signal := nanSlice(n)
	smi[0] = -45 // 武装@0
	// 后续 10 根一直在阈值上方但 Signal 缺失（NaN），bar11 起超时作废
	smi[1] = -30
	smi[2] = -20
	bottoms, tops := smiZoneThresholds(smi)
	buys, sells := computeSmiSignals(smi, signal, bottoms, tops, 10)
	if len(buys) != 0 || len(sells) != 0 {
		t.Fatalf("超时后不应触发, buys=%v sells=%v", buys, sells)
	}
	// 超时作废后，重新武装需再次进入区域；bar14 再次进入底部，bar16 确认
	for i := 0; i < n; i++ {
		signal[i] = math.NaN()
	}
	signal[16] = -30
	smi[14] = -45
	smi[15] = -42
	smi[16] = -20
	buys, _ = computeSmiSignals(smi, signal, bottoms, tops, 10)
	if len(buys) != 1 || buys[0] != 16 {
		t.Fatalf("重新武装后应触发买点@16, buys=%v", buys)
	}
}

// 滚动窗口 P20/P80 阈值与前端 smiZoneThresholds 同口径
func TestSmiZoneThresholdsAdaptive(t *testing.T) {
	n := 150
	smi := make([]float64, n)
	for i := 0; i < n; i++ {
		smi[i] = float64(i) // 0..149，无 NaN
	}
	bottoms, tops := smiZoneThresholds(smi)
	// 末根窗口覆盖全部 150 个样本（W=300），k=round(0.2*150)=30 → P20=vals[29]=29, P80=vals[150-30]=120
	if bottoms[n-1] != 29 || tops[n-1] != 120 {
		t.Fatalf("滚动分位阈值不符: bottom=%v top=%v, want 29/120", bottoms[n-1], tops[n-1])
	}
	// 早期样本不足时保持回退值
	if bottoms[0] != smiFallbackBottom || tops[0] != smiFallbackTop {
		t.Fatalf("早期应回退 ±40: bottom=%v top=%v", bottoms[0], tops[0])
	}
}
