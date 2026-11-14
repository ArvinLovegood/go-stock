function parseFloatPct(s) {
  const v = parseFloat(String(s ?? '').replace(/%/g, '').trim())
  return Number.isFinite(v) ? v / 100 : 0
}

export function chipBarCostCenter(r) {
  const h = Number(r.high)
  const l = Number(r.low)
  const c = Number(r.close)
  const o = Number(r.open)
  const vol = Number(r.volume)
  const amt = Number(r.amount)
  const hlOk = Number.isFinite(h) && Number.isFinite(l) && h > 0 && l > 0 && h >= l
  if (!hlOk) return null
  if (Number.isFinite(amt) && amt > 0 && Number.isFinite(vol) && vol > 0) {
    const vwap = amt / vol
    if (Number.isFinite(vwap) && vwap > 0) return Math.min(h, Math.max(l, vwap))
  }
  if ([h, l, c].every(Number.isFinite) && c > 0) {
    const tp = (h + l + c) / 3
    if (Number.isFinite(tp)) return Math.min(h, Math.max(l, tp))
  }
  if ([h, l, o, c].every(Number.isFinite) && o > 0 && c > 0) {
    const tp = (h + l + o + c) / 4
    if (Number.isFinite(tp)) return Math.min(h, Math.max(l, tp))
  }
  return (h + l) / 2
}

export function addChipVolumeKernel(dist, bins, minP, width, low, high, vol, center) {
  if (vol <= 0 || low <= 0 || high <= 0) return
  let lo = low
  let hi = high
  if (hi < lo) [lo, hi] = [hi, lo]
  const span = hi - lo
  const loIdx = Math.max(0, Math.min(bins - 1, Math.floor((lo - minP) / width)))
  const hiIdx = Math.max(0, Math.min(bins - 1, Math.floor((hi - minP) / width)))
  if (hiIdx < loIdx) return
  if (span < 1e-9 * Math.max(1, hi)) {
    const i = Math.max(0, Math.min(bins - 1, Math.floor(((lo + hi) / 2 - minP) / width)))
    dist[i] += vol
    return
  }
  let m = center
  if (!Number.isFinite(m)) m = (lo + hi) / 2
  m = Math.min(hi, Math.max(lo, m))
  const sigma = Math.max(span * 0.18, hi * 1e-6, 1e-6)
  let wsum = 0
  for (let i = loIdx; i <= hiIdx; i++) {
    const bc = minP + (i + 0.5) * width
    if (bc < lo || bc > hi) continue
    const d = (bc - m) / sigma
    wsum += Math.exp(-0.5 * d * d)
  }
  if (wsum <= 0) {
    const cnt = hiIdx - loIdx + 1
    const add = vol / cnt
    for (let i = loIdx; i <= hiIdx; i++) dist[i] += add
    return
  }
  for (let i = loIdx; i <= hiIdx; i++) {
    const bc = minP + (i + 0.5) * width
    if (bc < lo || bc > hi) continue
    const d = (bc - m) / sigma
    const w = Math.exp(-0.5 * d * d)
    dist[i] += (vol * w) / wsum
  }
}

export function calcChipDistribution(rows, bins) {
  if (!rows?.length || bins <= 0) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  let minP = Infinity, maxP = 0
  for (const r of rows) {
    const lo = Number(r.low) || 0
    const hi = Number(r.high) || 0
    if (lo > 0 && lo < minP) minP = lo
    if (hi > 0 && hi > maxP) maxP = hi
  }
  if (minP <= 0 || maxP <= 0 || maxP < minP) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  if (maxP === minP) maxP = minP * 1.001
  const width = (maxP - minP) / bins
  if (width <= 0) return { items: [], avgCost: 0, profitRatio: 0, current: 0 }
  const dist = new Float64Array(bins)
  for (const r of rows) {
    let turn = parseFloatPct(r.turnoverRate)
    if (turn < 0) turn = 0
    if (turn > 0.98) turn = 0.98
    const remain = 1.0 - turn
    for (let i = 0; i < bins; i++) dist[i] *= remain
    const low = Number(r.low) || 0
    const high = Number(r.high) || 0
    const vol = Number(r.volume) || 0
    if (vol <= 0 || low <= 0 || high <= 0) continue
    const center = chipBarCostCenter(r)
    addChipVolumeKernel(dist, bins, minP, width, low, high, vol, center)
  }
  let sum = 0
  for (let i = 0; i < bins; i++) sum += dist[i]
  const cur = Number(rows[rows.length - 1].close) || Number(rows[rows.length - 1].high) || 0
  const items = []
  let avgCost = 0, profitVol = 0
  for (let i = 0; i < bins; i++) {
    const center = minP + (i + 0.5) * width
    const v = dist[i]
    const ratio = sum > 0 ? v / sum : 0
    items.push({ price: Math.round(center * 10000) / 10000, vol: Math.round(v * 10000) / 10000, ratio: Math.round(ratio * 1e6) / 1e6 })
    avgCost += v * center
    if (center <= cur) profitVol += v
  }
  if (sum > 0) avgCost /= sum
  const profitRatio = sum > 0 ? profitVol / sum : 0
  return { items, avgCost: Math.round(avgCost * 10000) / 10000, profitRatio: Math.round(profitRatio * 1e6) / 1e6, current: Math.round(cur * 10000) / 10000, minPrice: minP, maxPrice: maxP }
}

/**
 * 获利比例历史序列：沿 K 线逐根推进筹码分布（换手衰减 + 当根成交量分布），逐根记录
 * 「当前价以下的筹码占比」（获利盘比例 0~1）。算法与 calcChipDistribution 完全同口径，
 * 区别仅在于逐根快照输出，用于画副图曲线与信号评估。
 * @param {Array<{low,high,close,volume,amount,turnoverRate}>} rows 按 day 升序的原始 K 线行
 * @param {number} bins 价格分档数（默认 50，越小越平滑且越快）
 * @returns {{ratios:number[], avgCosts:number[]}} 与 rows 等长；前几根筹码未成形时为 0
 */
export function profitRatioSeries(rows, bins = 50) {
  const n = rows?.length || 0
  const ratios = new Array(n).fill(0)
  const avgCosts = new Array(n).fill(0)
  if (!n || bins <= 0) return { ratios, avgCosts }
  let minP = Infinity, maxP = 0
  for (const r of rows) {
    const lo = Number(r.low) || 0
    const hi = Number(r.high) || 0
    if (lo > 0 && lo < minP) minP = lo
    if (hi > 0 && hi > maxP) maxP = hi
  }
  if (minP <= 0 || maxP <= 0 || maxP < minP) return { ratios, avgCosts }
  if (maxP === minP) maxP = minP * 1.001
  const width = (maxP - minP) / bins
  if (width <= 0) return { ratios, avgCosts }
  const dist = new Float64Array(bins)
  for (let k = 0; k < n; k++) {
    const r = rows[k]
    let turn = parseFloatPct(r.turnoverRate)
    if (turn < 0) turn = 0
    if (turn > 0.98) turn = 0.98
    const remain = 1.0 - turn
    for (let i = 0; i < bins; i++) dist[i] *= remain
    const low = Number(r.low) || 0
    const high = Number(r.high) || 0
    const vol = Number(r.volume) || 0
    if (vol > 0 && low > 0 && high > 0) {
      addChipVolumeKernel(dist, bins, minP, width, low, high, vol, chipBarCostCenter(r))
    }
    let sum = 0
    for (let i = 0; i < bins; i++) sum += dist[i]
    if (sum <= 0) continue
    const cur = Number(r.close) || Number(r.high) || 0
    let profitVol = 0
    let costAcc = 0
    for (let i = 0; i < bins; i++) {
      const center = minP + (i + 0.5) * width
      costAcc += dist[i] * center
      if (center <= cur) profitVol += dist[i]
    }
    ratios[k] = Math.round((profitVol / sum) * 1e6) / 1e6
    avgCosts[k] = Math.round((costAcc / sum) * 10000) / 10000
  }
  return { ratios, avgCosts }
}
