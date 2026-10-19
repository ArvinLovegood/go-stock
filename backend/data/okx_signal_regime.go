package data

import (
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 信号自动交易的「环境门控」。
//
// 回测依据（19 个 OKX 永续 / 1 年 5m+15m+1h / 2025-10 ~ 2026-10，含双边手续费 0.05%+滑点 0.02%）：
//   - 5m 裸信号（≥4 路双向翻手）每品种年化 -462%，日均 4.5 笔/品种，胜率 30%；
//   - 15m 同口径 -302%，胜率 35%；
//   - 只提高共振门槛到 6 路 + 持有到反向信号：5m -59%、15m -59%，但 15m 在下跌季度仍大幅回撤；
//   - 再叠加「1H 趋势态」方向门控（EMA50/EMA200 + 收盘位置；当时用 BTC 当代理）：15m 年化 -28%，胜率 38%，
//     是全部组合里回撤最小的一档；1h 上同一门控可做到约打平。
//
// 结论：这套 9 路共振信号在 5m/15m 上没有可提取的择时优势（在场期间的市场漂移并不高于无条件漂移），
// 手续费是主要亏损来源，因此唯一有效的改法是「少下单 + 顺大势 + 限制单日开仓次数」。
// 本文件只提供门控数据，不做任何下单决策，便于单独测试。

// okxRegimeTTL 趋势态缓存时长：1H 级别趋势不需要秒级刷新，5 分钟足够且能省掉每笔信号的行情请求。
const okxRegimeTTL = 5 * time.Minute

// okxRegimeEntry 单个合约的趋势态缓存项。
type okxRegimeEntry struct {
	value  int
	expiry time.Time
}

// okxRegimeCache 按合约缓存趋势态（进程内，重启清零）。
var (
	okxRegimeMu    sync.Mutex
	okxRegimeCache = map[string]okxRegimeEntry{}
)

// okxTrendRegime 返回「该合约自身」1H 的趋势态：
//
//	1  = 多头趋势（EMA50 > EMA200 且收盘价在 EMA200 之上）
//	-1 = 空头趋势（EMA50 < EMA200 且收盘价在 EMA200 之下）
//	0  = 过渡区（均线缠绕或价格在均线与趋势之间，方向不明）
//
// 门控必须用它自己的趋势，不能用 BTC 代理：2026 年对 SOXL / NVDA 等股票类永续的回测里，
// BTC 1H 门控与标的走势并不同步——NVDA 上 41 笔信号被打成 36 笔空单（BTC 走弱只放行做空），
// 而 NVDA 同期是上涨的；换成标的自身趋势后，同一组参数由 -2.3% 转为 +1.1%（SOXL ① 档）。
//
// 取不到数据（新上市合约不足 220 根）时返回 0，调用方按「不拦截」处理
// （行情异常时不应把全部信号静默吃掉）。
func okxTrendRegime(instId string) int {
	if instId == "" {
		return 0
	}
	okxRegimeMu.Lock()
	if e, ok := okxRegimeCache[instId]; ok && time.Now().Before(e.expiry) {
		v := e.value
		okxRegimeMu.Unlock()
		return v
	}
	okxRegimeMu.Unlock()

	v := 0
	candles := GetOKXCandles(instId, "1H", 300)
	if len(candles) >= 220 {
		closes := make([]float64, 0, len(candles))
		for _, c := range candles {
			// 未收盘的那根不能用于判断，否则同分钟反复扫描会出现趋势态抖动
			if c.Confirm && c.Close > 0 {
				closes = append(closes, c.Close)
			}
		}
		if len(closes) >= 220 {
			ema50 := okxEMA(closes, 50)
			ema200 := okxEMA(closes, 200)
			last := closes[len(closes)-1]
			switch {
			case ema50 > ema200 && last > ema200:
				v = 1
			case ema50 < ema200 && last < ema200:
				v = -1
			}
		}
	}

	okxRegimeMu.Lock()
	okxRegimeCache[instId] = okxRegimeEntry{value: v, expiry: time.Now().Add(okxRegimeTTL)}
	okxRegimeMu.Unlock()
	return v
}

// okxEMA 计算指数移动平均的最后一个值（前 period 个值用简单均值播种，避免早期虚高）。
func okxEMA(values []float64, period int) float64 {
	if len(values) == 0 || period <= 0 {
		return 0
	}
	if len(values) < period {
		period = len(values)
	}
	sum := 0.0
	for _, v := range values[:period] {
		sum += v
	}
	ema := sum / float64(period)
	k := 2.0 / float64(period+1)
	for _, v := range values[period:] {
		ema = v*k + ema*(1-k)
	}
	return ema
}

// okxSignalEntriesToday 统计某合约在某环境「今天（东八区自然日）」已成功开仓的笔数。
// 走信号流水表，重启后仍然有效；查询失败返回 -1，调用方按「不拦截」处理。
func okxSignalEntriesToday(simulated bool, instId string) int {
	if db.Dao == nil || instId == "" {
		return -1
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()
	var cnt int64
	err := db.Dao.Model(&models.SignalTradeRecord{}).
		Where("inst_id = ? AND env = ? AND family = ? AND status = ? AND at >= ?",
			instId, okxTradeEnvLabel(simulated), "buysell", "success", dayStart).
		Count(&cnt).Error
	if err != nil {
		logger.SugaredLogger.Warnf("统计当日开仓笔数失败: instId=%s err=%v", instId, err)
		return -1
	}
	return int(cnt)
}
