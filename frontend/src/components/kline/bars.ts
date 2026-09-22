/**
 * 数据源原始 K 线行 → 指标计算用的数值序列。
 *
 * 抽成公共模块的原因：图表组件与后台信号监控引擎必须用**完全相同**的取数口径，
 * 否则同一只票、同一根 K 线在图上与面板上的买卖点会对不上（验收硬指标）。
 */
import { parseNumStr } from './format'
import { sortKey, toChartTime, extractYmdDatePart } from './time'

/** 原始行形如 { day, open, high, low, close, volume, amplitude }（东财口径） */
export function extractOHLCV(rows) {
  const sorted = [...(rows || [])].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const times = []
  const opens = []
  const closes = []
  const highs = []
  const lows = []
  const vols = []
  const amplitudes = []
  const days = []
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    times.push(t)
    opens.push(o)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    vols.push(Number.isFinite(v) ? v : 0)
    days.push(extractYmdDatePart(r.day))
    const rawAmp = parseNumStr(r.amplitude)
    amplitudes.push(Number.isFinite(rawAmp) ? rawAmp : (o > 0 ? (h - l) / o * 100 : NaN))
  }
  return { times, opens, closes, highs, lows, vols, amplitudes, days }
}