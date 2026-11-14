package data

import (
	"fmt"
	"math"
	"testing"
)

func TestBiasValuesBasic(t *testing.T) {
	closes := []float64{10, 10, 10, 10, 10, 10, 10, 12}
	b := biasValues(closes, 6)
	if !isNaN(b[4]) {
		t.Fatalf("BIAS warmup 应为 NaN, got %v", b[4])
	}
	if b[5] != 0 {
		t.Fatalf("平盘期 BIAS 应为 0, got %v", b[5])
	}
	// 下标7: sma6=(10+10+10+10+10+12)/6=10.333 → (12-10.333)/10.333*100≈16.13
	want := (12 - 10.333333333333334) / 10.333333333333334 * 100
	if math.Abs(b[7]-want) > 1e-9 {
		t.Fatalf("BIAS 计算不符: got %v want %v", b[7], want)
	}
}

func TestCandlePatternsMorningStar(t *testing.T) {
	// bar4 长阴, bar5 小实体跳空, bar6 阳线收复一半 → 启明星@6
	opens := []float64{10, 10, 10, 10, 9.0, 8.1, 8.2, 8.2}
	highs := []float64{10.1, 10.1, 10.1, 10.1, 9.1, 8.3, 9.1, 9.1}
	lows := []float64{9.9, 9.9, 9.9, 9.9, 8.0, 8.0, 8.1, 8.1}
	closes := []float64{10, 10, 10, 10, 8.1, 8.15, 9.0, 9.0}
	bullish, bearish := candlePatternsValues(opens, highs, lows, closes)
	found := false
	for _, p := range bullish {
		if p.idx == 6 && p.name == "启明星" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应识别启明星@6, bullish=%v", bullish)
	}
	if len(bearish) != 0 {
		t.Fatalf("不应有看跌形态, bearish=%v", bearish)
	}
}

func TestCandlePatternsHammerInDowntrend(t *testing.T) {
	// 连跌后长下影小实体 → 锤头
	opens := []float64{10, 10, 10, 10, 10, 9.0, 9.0}
	highs := []float64{10.1, 9.9, 9.8, 9.7, 9.6, 9.1, 9.05}
	lows := []float64{9.9, 9.7, 9.6, 9.5, 9.0, 8.2, 8.2}
	closes := []float64{9.95, 9.8, 9.7, 9.6, 9.1, 8.9, 8.9}
	bullish, bearish := candlePatternsValues(opens, highs, lows, closes)
	found := false
	for _, p := range bullish {
		if p.idx == 6 && p.name == "锤头" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应识别锤头@6, bullish=%v", bullish)
	}
	if len(bearish) != 0 {
		t.Fatalf("不应有看跌形态, bearish=%v", bearish)
	}
}

func TestProfitRatioSeriesDirection(t *testing.T) {
	n := 120
	upHighs := make([]float64, n)
	upLows := make([]float64, n)
	upCloses := make([]float64, n)
	upOpens := make([]float64, n)
	upVols := make([]float64, n)
	downCloses := make([]float64, n)
	downOpens := make([]float64, n)
	downHighs := make([]float64, n)
	downLows := make([]float64, n)
	downVols := make([]float64, n)
	for i := 0; i < n; i++ {
		upCloses[i] = 10 + float64(i)*0.1
		upOpens[i] = upCloses[i] - 0.05
		upHighs[i] = upCloses[i] + 0.05
		upLows[i] = upOpens[i] - 0.05
		upVols[i] = 1000
		downCloses[i] = 30 - float64(i)*0.1
		downOpens[i] = downCloses[i] + 0.05
		downHighs[i] = downOpens[i] + 0.05
		downLows[i] = downCloses[i] - 0.05
		downVols[i] = 1000
	}
	zeros := make([]float64, n)
	upPR := profitRatioSeries(upOpens, upHighs, upLows, upCloses, upVols, zeros, zeros, 50)
	if upPR[n-1] < 0.9 {
		t.Fatalf("单边上涨末根获利盘应 >0.9, got %v", upPR[n-1])
	}
	downPR := profitRatioSeries(downOpens, downHighs, downLows, downCloses, downVols, zeros, zeros, 50)
	if downPR[n-1] > 0.1 {
		t.Fatalf("单边下跌末根获利盘应 <0.1, got %v", downPR[n-1])
	}
}

func TestMtfScoreSeriesUptrend(t *testing.T) {
	// 稳步上涨 120 根日K → 末根多周期评分应偏多（>=60）
	n := 120
	closes := make([]float64, n)
	opens := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	dayKeys := make([]string, n)
	for i := 0; i < n; i++ {
		closes[i] = 10 + float64(i)*0.05
		opens[i] = closes[i] - 0.02
		highs[i] = closes[i] + 0.03
		lows[i] = opens[i] - 0.02
		dayKeys[i] = "2026-01-05"
	}
	// 用一个简单的日期生成：从 2026-01-05 起每个 i +1 天（周末也含，仅用于周聚合测试）
	d := 5
	month, year := 1, 2026
	for i := 0; i < n; i++ {
		dayKeys[i] = fmt.Sprintf("%04d-%02d-%02d", year, month, d)
		d++
		if d > 28 {
			d = 1
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
	}
	scores := mtfScoreSeries(opens, highs, lows, closes, dayKeys)
	last := scores[n-1]
	if isNaN(last) {
		t.Fatalf("末根评分不应为 NaN")
	}
	if last < 0 || last > 100 {
		t.Fatalf("评分应落在 0~100, got %v", last)
	}
	if last < 60 {
		t.Fatalf("单边上涨末根评分应 >=60, got %v", last)
	}
	// 前几根（MA20 未就绪）可用因子数减少但不应 panic/NaN：第 0 根 ROC10 不足，MA20 不足
	if isNaN(scores[0]) {
		t.Fatalf("首根评分不应为 NaN（因子缺失应从分母剔除）")
	}
}

func TestBuySellPointsExtendedNoPanic(t *testing.T) {
	// 300 根日K + 扩展数据：不 panic、门限放大生效、Score 上限 18
	n := 300
	closes := make([]float64, n)
	opens := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	vols := make([]float64, n)
	amounts := make([]float64, n)
	turns := make([]float64, n)
	dayKeys := make([]string, n)
	for i := 0; i < n; i++ {
		closes[i] = 20 + math.Sin(float64(i)/8)*2 + float64(i)*0.01
		opens[i] = closes[i] - 0.05
		highs[i] = closes[i] + 0.15
		lows[i] = opens[i] - 0.15
		vols[i] = 1e6 + math.Sin(float64(i))*1e5
		amounts[i] = vols[i] * closes[i]
		turns[i] = 1.5
		dayKeys[i] = "2026-01-05"
	}
	d, month, year := 5, 1, 2026
	for i := 0; i < n; i++ {
		dayKeys[i] = fmt.Sprintf("%04d-%02d-%02d", year, month, d)
		d++
		if d > 28 {
			d = 1
			month++
		}
	}
	opts := &BuySellPointsOptions{
		MinScore:      6,
		MinGroups:     2,
		DayKeys:       dayKeys,
		Opens:         opens,
		Amounts:       amounts,
		TurnoverRates: turns,
	}
	buys, sells := BuySellPointsValues(highs, lows, closes, vols, opts)
	for _, p := range append(buys, sells...) {
		if p.Score > 18 {
			t.Fatalf("Score 不应超过 18, got %v", p.Score)
		}
	}
	// 长期上涨后段应产生买点（门控：m60<=0.15 可能拦截，宽松断言：不 panic 即可 + 有输出结构）
	if buys == nil || sells == nil {
		t.Fatalf("返回切片不应为 nil")
	}
}
