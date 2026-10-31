package data

import (
	"math"
	"testing"
)

// Mass Index 闸门单测：纯函数，无网络依赖。
// 行情拉取与缓存由 okxMassGateState 覆盖，此处只验证算法、分位阈值与状态机。

func TestOkxMassIndexConstantRange(t *testing.T) {
	// 恒定 range=2：单 EMA、双 EMA、比值 EMA 都收敛到常数，25 期求和 = 25
	n := 120
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i := 0; i < n; i++ {
		highs[i] = 10 + float64(i)
		lows[i] = highs[i] - 2
	}
	mass := okxMassIndex(highs, lows)
	// 预热 9+9+9+25-3 = 48 根（0-based 索引 48 起有效）
	if mass[47] != 0 {
		t.Errorf("第 48 根之前应为无效(0)，got %v", mass[47])
	}
	if math.Abs(mass[48]-25) > 1e-9 {
		t.Errorf("恒定振幅的 MI 应为 25，got %v", mass[48])
	}
	if math.Abs(mass[n-1]-25) > 1e-9 {
		t.Errorf("尾部恒定振幅的 MI 应为 25，got %v", mass[n-1])
	}
}

func TestOkxMassAdaptiveThresholds(t *testing.T) {
	// 样本不足 → 回退经典 27/26.5
	mass := make([]float64, okxMassMinSamples-1)
	for i := range mass {
		mass[i] = 25
	}
	if enter, exit := okxMassAdaptiveThresholds(mass); enter != okxMassEnter || exit != okxMassExit {
		t.Errorf("样本不足应回退 27/26.5，got %v/%v", enter, exit)
	}

	// MI 长期偏低（P90=25，低于经典 27）→ 进入线自适应降到 25，离开线 24.5
	mass = make([]float64, 200)
	for i := range mass {
		mass[i] = 25
	}
	if enter, exit := okxMassAdaptiveThresholds(mass); enter != 25 || exit != 24.5 {
		t.Errorf("低 MI 品种应自适应到 25/24.5，got %v/%v", enter, exit)
	}

	// MI 极低（P90=22）→ 进入线钳制到下限 24，离开线 23.5
	mass = make([]float64, 200)
	for i := range mass {
		mass[i] = 22
	}
	if enter, exit := okxMassAdaptiveThresholds(mass); enter != okxMassMinEnter || exit != okxMassMinEnter-okxMassHysteresis {
		t.Errorf("极低 MI 品种应钳制到 24/23.5，got %v/%v", enter, exit)
	}

	// 80% 的 25 + 20% 的 30 → P90 = 30，离开线 29.5
	mass = make([]float64, 200)
	for i := range mass {
		if i < 160 {
			mass[i] = 25
		} else {
			mass[i] = 30
		}
	}
	if enter, exit := okxMassAdaptiveThresholds(mass); math.Abs(enter-30) > 1e-9 || math.Abs(exit-29.5) > 1e-9 {
		t.Errorf("20%% 的样本为 30 时应取 30/29.5，got %v/%v", enter, exit)
	}

	// MI 极端偏高（P90=40）→ 进入线钳制到上限 31，离开线 30.5
	mass = make([]float64, 200)
	for i := range mass {
		mass[i] = 40
	}
	if enter, exit := okxMassAdaptiveThresholds(mass); enter != okxMassMaxEnter || math.Abs(exit-(okxMassMaxEnter-okxMassHysteresis)) > 1e-9 {
		t.Errorf("极端高 MI 品种应钳制到 31/30.5，got %v/%v", enter, exit)
	}
}

func TestOkxMassTailState(t *testing.T) {
	enter, exit := okxMassEnter, okxMassExit
	flat := make([]float64, 62)
	for i := 48; i < len(flat); i++ {
		flat[i] = 25
	}
	if got := okxMassTailState(flat, enter, exit, okxMassCoolBars); got != okxMassNormal {
		t.Fatalf("未出现凸起应为正常，got %d", got)
	}

	// 进入凸起（≥27）：末尾仍在窗口内 → 拦截开仓
	bulge := append(append([]float64{}, flat...), 30)
	if got := okxMassTailState(bulge, enter, exit, okxMassCoolBars); got != okxMassBulge {
		t.Fatalf("末尾仍在凸起应为 bulge，got %d", got)
	}

	// 迟滞：在 26.5~27 之间不算离开窗口
	hold := append(append([]float64{}, bulge...), 26.8, 26.8)
	if got := okxMassTailState(hold, enter, exit, okxMassCoolBars); got != okxMassBulge {
		t.Fatalf("26.5~27 之间应判定仍在窗口，got %d", got)
	}

	// 回落确认（<26.5）当根 → 冷却
	confirm := append(append([]float64{}, hold...), 26)
	if got := okxMassTailState(confirm, enter, exit, okxMassCoolBars); got != okxMassCooldown {
		t.Fatalf("刚确认回落应为 cooldown，got %d", got)
	}

	// 确认后第 1 根仍冷却，第 2 根恢复正常
	oneAfter := append(append([]float64{}, confirm...), 25)
	if got := okxMassTailState(oneAfter, enter, exit, okxMassCoolBars); got != okxMassCooldown {
		t.Fatalf("确认后第 1 根应仍为 cooldown，got %d", got)
	}
	twoAfter := append(append([]float64{}, oneAfter...), 25)
	if got := okxMassTailState(twoAfter, enter, exit, okxMassCoolBars); got != okxMassNormal {
		t.Fatalf("确认后第 2 根应恢复正常，got %d", got)
	}

	// 自适应阈值更高的品种：进入线 30 时 MI=30 刚好触发，进入线 31 时不再触发
	if got := okxMassTailState(bulge, 30, 29.5, okxMassCoolBars); got != okxMassBulge {
		t.Fatalf("进入线 30 时 MI=30 应刚好触发，got %d", got)
	}
	if got := okxMassTailState(bulge, 31, 30.5, okxMassCoolBars); got != okxMassNormal {
		t.Fatalf("进入线 31 时 MI=30 不应触发，got %d", got)
	}

	// 预热不足（全无效）→ 正常，不拦截
	empty := make([]float64, 30)
	if got := okxMassTailState(empty, enter, exit, okxMassCoolBars); got != okxMassNormal {
		t.Fatalf("无效序列应为正常，got %d", got)
	}
}
