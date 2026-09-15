package agent

// template_backtest_test.go — 提示词模板回测统计的守护测试（纯函数，不依赖 DB）。

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/models"
)

func TestTemplateScoreBounds(t *testing.T) {
	// 全优：超额胜率 100%、平均超额 +30%、CV=0 → 应接近满分
	hi := templateScore(100, 30, 0)
	// 全劣：超额胜率 0、平均超额 -30%、CV≥1 → 应接近 0 分
	lo := templateScore(0, -30, 2)
	if hi < 95 || hi > 100 {
		t.Errorf("全优场景评分应接近 100，实际 %.1f", hi)
	}
	if lo < 0 || lo > 5 {
		t.Errorf("全劣场景评分应接近 0，实际 %.1f", lo)
	}
	// CV 无效（-1）按 0.5 计 → 稳定分 15
	mid := templateScore(50, 0, -1)
	want := 50.0/100*40 + 30*(math.Tanh(0)+1)/2 + 15 // 20 + 15 + 15 = 50
	if math.Abs(mid-want) > 0.11 {
		t.Errorf("CV 无效场景评分应为 %.1f，实际 %.1f", want, mid)
	}
}

func TestStddevAndMedian(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5}
	if m := median(vals); m != 3 {
		t.Errorf("median = %v, want 3", m)
	}
	if m := median([]float64{1, 2}); m != 1.5 {
		t.Errorf("median = %v, want 1.5", m)
	}
	// 样本标准差（ddof=1）：{1..5} 均值 3，方差 = (4+1+0+1+4)/4 = 2.5 → σ≈1.5811
	if s := stddev(vals, 3); math.Abs(s-math.Sqrt(2.5)) > 1e-9 {
		t.Errorf("stddev = %v, want %v", s, math.Sqrt(2.5))
	}
	if s := stddev([]float64{1}, 1); s != 0 {
		t.Errorf("单样本 stddev 应为 0，实际 %v", s)
	}
}

func TestBuildEquityCurveAndDrawdown(t *testing.T) {
	d1 := time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local)
	d2 := time.Date(2026, 1, 6, 10, 0, 0, 0, time.Local)
	rows := []models.AiRecommendBacktest{
		{PeriodDays: 5, ReturnPct: 10, RecommendTime: d1},
		{PeriodDays: 5, ReturnPct: -6, RecommendTime: d1}, // 同日第二条 → 等权 (10-6)/2=2%
		{PeriodDays: 5, ReturnPct: -20, RecommendTime: d2},
		{PeriodDays: 10, ReturnPct: 100, RecommendTime: d1}, // 非主周期，应被排除
	}
	curve := buildEquityCurve(rows, 5)
	if len(curve) != 2 {
		t.Fatalf("曲线点数 = %d, want 2", len(curve))
	}
	if curve[0].DailyPct != 2 {
		t.Errorf("首日组合收益 = %v, want 2", curve[0].DailyPct)
	}
	wantEq1 := 1.02
	if curve[0].Equity != wantEq1 {
		t.Errorf("首日净值 = %v, want %v", curve[0].Equity, wantEq1)
	}
	wantEq2 := 1.02 * 0.8 // 1.02 * (1-0.2)
	if math.Abs(curve[1].Equity-wantEq2) > 1e-9 {
		t.Errorf("次日净值 = %v, want %v", curve[1].Equity, wantEq2)
	}
	// 最大回撤：峰值 1.02 → 谷 0.816 → 回撤 ≈ -20%
	dd := maxDrawdownPct(curve)
	wantDD := 0.816/1.02 - 1
	if math.Abs(dd-wantDD) > 1e-9 {
		t.Errorf("最大回撤 = %v, want %v", dd, wantDD)
	}
}

func TestDominantPeriod(t *testing.T) {
	p, c := dominantPeriod(map[int]int{5: 3, 10: 3})
	if p != 5 {
		t.Errorf("同样本数应取更小周期，got %d", p)
	}
	p, c = dominantPeriod(map[int]int{5: 2, 10: 5})
	if p != 10 || c != 5 {
		t.Errorf("应取样本最多周期，got (%d,%d)", p, c)
	}
}

func TestMatchPromptTemplateIDNoCache(t *testing.T) {
	// 未初始化 DB 时缓存列表为空，任何提示词都应返回 0（不 panic）
	if id := matchPromptTemplateID("任意内容"); id != 0 {
		t.Errorf("空缓存应返回 0，实际 %d", id)
	}
	if id := matchPromptTemplateID(""); id != 0 {
		t.Errorf("空提示词应返回 0，实际 %d", id)
	}
}
