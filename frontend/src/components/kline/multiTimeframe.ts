/**
 * 多周期共振评分（Multi-Timeframe Score）：
 * 将当前周期的 K 线聚合到上一级周期（日K→周K、周K→月K），在两个周期上各评估 6 个因子，
 * 每个因子命中计 1 分，周期得分 = 命中数 / 可用因子数 × 100；总分 = 当前周期 50% + 上一级周期 50%。
 *
 * 因子（等权，多空对称）：
 *   1. 收盘价在 MA20 上方（趋势位置）
 *   2. MA5 > MA20（均线多头排列）
 *   3. MACD DIF > DEA（动量方向）
 *   4. RSI(14) > 50（相对强弱）
 *   5. KDJ K > D（摆动方向）
 *   6. ROC(10) > 0（短期动量）
 *
 * 防未来数据：上一级周期因子只使用「已完成」的周期K线 —— 日K在第 N 周内只看得到
 * 第 N-1 周及更早的周线因子（周线在周末才收盘）；周K同理只看已完成的月线。
 * 因子窗口不足（null）时该因子从分母剔除；两侧都不可用时评分为 null。
 */
import { smaValues, macdBundle, rsiBundle, kdjBundle, rocValues } from './calc'

/** 提取 'YYYY-MM-DD'（兼容 '2026/11/14 15:00' 与 '2026-11-14' 两种格式） */
function ymdOf(day) {
  const s = String(day || '')
  const m = s.match(/(\d{4})[-/](\d{2})[-/](\d{2})/)
  if (!m) return null
  return `${m[1]}-${m[2]}-${m[3]}`
}

/** 周键：所在周的周一日期（UTC 计算，避免时区漂移） */
function weekKeyOf(ymd) {
  const [y, m, d] = ymd.split('-').map(Number)
  const dt = new Date(Date.UTC(y, m - 1, d))
  const wd = dt.getUTCDay() || 7 // 周一=1 … 周日=7
  dt.setUTCDate(dt.getUTCDate() - (wd - 1))
  return `${dt.getUTCFullYear()}-${String(dt.getUTCMonth() + 1).padStart(2, '0')}-${String(dt.getUTCDate()).padStart(2, '0')}`
}

/** 月键：'YYYY-MM' */
function monthKeyOf(ymd) {
  return ymd.slice(0, 7)
}

/**
 * 聚合到上一级周期：按键分组，组内取首开盘/最高/最低/末收盘，成交量求和。
 * @param {Array<{day,open,high,low,close,volume}>} rows 原始 K 线行（任意顺序，内部按 day 升序）
 * @param {'weekly'|'monthly'} level 聚合级别
 * @returns {{keys:string[], endKeys:string[], opens:number[], highs:number[], lows:number[], closes:number[]}}
 *   keys 为组键（周一日期 / 'YYYY-MM'），endKeys 为组内最后一根的键（用于顺序比较）
 */
export function resampleToHigherTf(rows, level) {
  const keyOf = level === 'monthly' ? monthKeyOf : weekKeyOf
  const sorted = [...(rows || [])]
    .map((r) => ({ r, ymd: ymdOf(r.day) }))
    .filter((x) => x.ymd)
    .sort((a, b) => (a.ymd < b.ymd ? -1 : a.ymd > b.ymd ? 1 : 0))
  const keys = []
  const endKeys = []
  const opens = []
  const highs = []
  const lows = []
  const closes = []
  let cur = null
  for (const { r, ymd } of sorted) {
    const k = keyOf(ymd)
    const o = Number(r.open), h = Number(r.high), l = Number(r.low), c = Number(r.close)
    if (![o, h, l, c].every(Number.isFinite)) continue
    if (!cur || cur.key !== k) {
      cur = { key: k, end: ymd, o, h, l, c }
      keys.push(k)
      endKeys.push(ymd)
      opens.push(o)
      highs.push(h)
      lows.push(l)
      closes.push(c)
    } else {
      cur.end = ymd
      cur.h = Math.max(cur.h, h)
      cur.l = Math.min(cur.l, l)
      cur.c = c
      endKeys[endKeys.length - 1] = ymd
      highs[highs.length - 1] = cur.h
      lows[lows.length - 1] = cur.l
      closes[closes.length - 1] = cur.c
    }
  }
  return { keys, endKeys, opens, highs, lows, closes }
}

/**
 * 在一组 OHLC 上评估 6 因子，返回每根的 {hits, available}（hits=命中数，available=可用因子数）。
 */
function factorHitsAt(opens, highs, lows, closes) {
  const n = closes.length
  const ma5 = smaValues(closes, 5)
  const ma20 = smaValues(closes, 20)
  const { dif, dea } = macdBundle(closes)
  const rsi14 = rsiBundle(closes, 14)
  const { K, D } = kdjBundle(highs, lows, closes, 9)
  const roc10 = rocValues(closes, 10)
  const out = new Array(n)
  for (let i = 0; i < n; i++) {
    let hits = 0
    let available = 0
    const add = (ok) => {
      available++
      if (ok) hits++
    }
    const c = closes[i]
    if (c != null && ma20[i] != null) add(c > ma20[i])
    if (ma5[i] != null && ma20[i] != null) add(ma5[i] > ma20[i])
    if (dif[i] != null && dea[i] != null) add(dif[i] > dea[i])
    if (rsi14[i] != null) add(rsi14[i] > 50)
    if (K[i] != null && D[i] != null) add(K[i] > D[i])
    if (roc10[i] != null) add(roc10[i] > 0)
    out[i] = available > 0 ? { hits, available } : null
  }
  return out
}

/**
 * 多周期共振评分主入口。
 * @param {Array<{day,open,high,low,close}>} rows 当前周期原始 K 线行
 * @param {'weekly'|'monthly'} level 聚合级别（日K→'weekly'，周K→'monthly'）
 * @returns {{scores:(number|null)[], baseScores:(number|null)[], higherScores:(number|null)[]}}
 *   与 rows 等长（按 day 升序对齐；乱序输入会先排序，返回序列即升序结果）。
 *   scores 0~100：>=60 多周期共振偏多，<=40 共振偏空，40~60 分歧区。
 */
export function mtfScoreBundle(rows, level = 'weekly') {
  const n = rows?.length || 0
  const empty = { scores: new Array(n).fill(null), baseScores: new Array(n).fill(null), higherScores: new Array(n).fill(null) }
  if (!n) return empty

  const keyOf = level === 'monthly' ? monthKeyOf : weekKeyOf
  const sorted = [...rows]
    .map((r) => ({ r, ymd: ymdOf(r.day) }))
    .filter((x) => x.ymd
      && [x.r.open, x.r.high, x.r.low, x.r.close].every((v) => Number.isFinite(Number(v))))
    .sort((a, b) => (a.ymd < b.ymd ? -1 : a.ymd > b.ymd ? 1 : 0))

  // 当前周期因子（直接在原始序列上算）
  const baseO = sorted.map((x) => Number(x.r.open))
  if (baseO.length < 2) return empty
  const baseH = sorted.map((x) => Number(x.r.high))
  const baseL = sorted.map((x) => Number(x.r.low))
  const baseC = sorted.map((x) => Number(x.r.close))
  const baseHits = factorHitsAt(baseO, baseH, baseL, baseC)

  // 上一级周期因子
  const { keys, closes: hC, highs: hH, lows: hL, opens: hO } = resampleToHigherTf(rows, level)
  const higherHits = hC.length >= 2 ? factorHitsAt(hO, hH, hL, hC) : []

  const scores = new Array(sorted.length).fill(null)
  const baseScores = new Array(sorted.length).fill(null)
  const higherScores = new Array(sorted.length).fill(null)

  // 指针：对每根基础 K 线，找键值严格小于其所在键的最后一个已完成上级周期
  let j = -1
  for (let i = 0; i < sorted.length; i++) {
    const ymd = sorted[i].ymd
    const k = keyOf(ymd)
    while (j + 1 < keys.length && keys[j + 1] < k) j++
    const bh = baseHits[i]
    if (bh && bh.available > 0) baseScores[i] = (bh.hits / bh.available) * 100

    let hs = null
    // j 已指向 keys[j] < k 的最后一个已完成周期（防御性再校验一次，保证严格小于当前键）
    let jUse = j
    if (jUse >= 0 && keys[jUse] >= k) jUse--
    while (jUse >= 0 && !(higherHits[jUse] && higherHits[jUse].available > 0)) jUse--
    if (jUse >= 0) {
      const hh = higherHits[jUse]
      hs = (hh.hits / hh.available) * 100
    }
    higherScores[i] = hs

    if (baseScores[i] != null && hs != null) scores[i] = baseScores[i] * 0.5 + hs * 0.5
    else if (baseScores[i] != null) scores[i] = baseScores[i]
    else if (hs != null) scores[i] = hs
  }
  return { scores, baseScores, higherScores }
}
