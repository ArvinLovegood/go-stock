export function smaValues(closes, period) {
  const out = []
  for (let i = 0; i < closes.length; i++) {
    if (i < period - 1) {
      out.push(null)
      continue
    }
    let s = 0
    for (let j = 0; j < period; j++) s += closes[i - j]
    out.push(s / period)
  }
  return out
}

export function emaFinite(values, period) {
  const out = []
  const k = 2 / (period + 1)
  let ema = null
  for (let i = 0; i < values.length; i++) {
    const v = values[i]
    if (!Number.isFinite(v)) {
      out.push(null)
      continue
    }
    if (ema === null) {
      if (i < period - 1) {
        out.push(null)
        continue
      }
      let s = 0
      let ok = true
      for (let j = i - period + 1; j <= i; j++) {
        if (!Number.isFinite(values[j])) {
          ok = false
          break
        }
        s += values[j]
      }
      if (!ok) {
        out.push(null)
        continue
      }
      ema = s / period
      out.push(ema)
    } else {
      ema = v * k + ema * (1 - k)
      out.push(ema)
    }
  }
  return out
}

export function emaLeadingNull(series, period) {
  const out = series.map(() => null)
  const k = 2 / (period + 1)
  let ema = null
  let sum = 0
  let cnt = 0
  for (let i = 0; i < series.length; i++) {
    const v = series[i]
    if (v == null || !Number.isFinite(v)) {
      out[i] = null
      continue
    }
    if (ema === null) {
      sum += v
      cnt++
      if (cnt < period) {
        out[i] = null
        continue
      }
      if (cnt === period) {
        ema = sum / period
        out[i] = ema
      }
    } else {
      ema = v * k + ema * (1 - k)
      out[i] = ema
    }
  }
  return out
}

export function weightedMaValues(values, period) {
  const out = []
  const denom = period * (period + 1) / 2
  for (let i = 0; i < values.length; i++) {
    if (i < period - 1) { out.push(null); continue }
    let sum = 0
    let ok = true
    for (let j = 0; j < period; j++) {
      const v = values[i - period + 1 + j]
      if (v == null || !Number.isFinite(v)) { ok = false; break }
      sum += v * (j + 1)
    }
    out.push(ok ? sum / denom : null)
  }
  return out
}

export function bollingerBands(closes, period, mult) {
  const mid = smaValues(closes, period)
  const upper = []
  const lower = []
  for (let i = 0; i < closes.length; i++) {
    if (i < period - 1) {
      upper.push(null)
      lower.push(null)
      continue
    }
    const m = mid[i]
    let sumSq = 0
    for (let j = 0; j < period; j++) {
      const d = closes[i - j] - m
      sumSq += d * d
    }
    const std = Math.sqrt(sumSq / period)
    upper.push(m + mult * std)
    lower.push(m - mult * std)
  }
  return { upper, mid, lower }
}

export function obvValues(closes, vols) {
  if (!closes.length) return []
  const out = []
  let obv = vols[0] || 0
  out.push(obv)
  for (let i = 1; i < closes.length; i++) {
    const ch = closes[i] - closes[i - 1]
    if (ch > 0) obv += vols[i] || 0
    else if (ch < 0) obv -= vols[i] || 0
    out.push(obv)
  }
  return out
}

export function macdBundle(closes) {
  const ema12 = emaFinite(closes, 12)
  const ema26 = emaFinite(closes, 26)
  const dif = closes.map((_, i) =>
    ema12[i] != null && ema26[i] != null ? ema12[i] - ema26[i] : null,
  )
  const dea = emaLeadingNull(dif, 9)
  const hist = dif.map((d, i) =>
    d != null && dea[i] != null ? 2 * (d - dea[i]) : null,
  )
  return { dif, dea, hist }
}

export function kdjBundle(highs, lows, closes, n = 9) {
  const len = closes.length
  const rsv = new Array(len).fill(null)
  for (let i = n - 1; i < len; i++) {
    let hn = -Infinity
    let ln = Infinity
    for (let j = 0; j < n; j++) {
      hn = Math.max(hn, highs[i - j])
      ln = Math.min(ln, lows[i - j])
    }
    const c = closes[i]
    rsv[i] = hn === ln ? 50 : ((c - ln) / (hn - ln)) * 100
  }
  const K = new Array(len).fill(null)
  const D = new Array(len).fill(null)
  const J = new Array(len).fill(null)
  let pk = 50
  let pd = 50
  for (let i = 0; i < len; i++) {
    const r = rsv[i]
    if (r == null) continue
    pk = (2 * pk + r) / 3
    pd = (2 * pd + pk) / 3
    K[i] = pk
    D[i] = pd
    J[i] = 3 * pk - 2 * pd
  }
  return { K, D, J }
}

export function rsiBundle(closes, period = 14) {
  const out = new Array(closes.length).fill(null)
  for (let i = period; i < closes.length; i++) {
    let gain = 0
    let loss = 0
    for (let j = 0; j < period; j++) {
      const ch = closes[i - j] - closes[i - j - 1]
      if (ch >= 0) gain += ch
      else loss -= ch
    }
    const ag = gain / period
    const al = loss / period
    out[i] = al === 0 ? 100 : 100 - 100 / (1 + ag / al)
  }
  return out
}

export function atrValues(highs, lows, closes, period = 14) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const tr = new Array(len).fill(null)
  tr[0] = highs[0] - lows[0]
  for (let i = 1; i < len; i++) {
    tr[i] = Math.max(
      highs[i] - lows[i],
      Math.abs(highs[i] - closes[i - 1]),
      Math.abs(lows[i] - closes[i - 1]),
    )
  }
  const out = new Array(len).fill(null)
  let sum = 0
  for (let i = 0; i < period && i < len; i++) {
    sum += tr[i]
  }
  if (len >= period) {
    out[period - 1] = sum / period
    for (let i = period; i < len; i++) {
      out[i] = (out[i - 1] * (period - 1) + tr[i]) / period
    }
  }
  return out
}

export function vwapValues(highs, lows, closes, vols, period = 20) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sumPV = 0
    let sumV = 0
    for (let j = 0; j < period; j++) {
      const tp = (highs[i - j] + lows[i - j] + closes[i - j]) / 3
      sumPV += tp * vols[i - j]
      sumV += vols[i - j]
    }
    out[i] = sumV > 0 ? sumPV / sumV : null
  }
  return out
}

export function mfiValues(highs, lows, closes, vols, period = 14) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const mf = tp.map((t, i) => t * vols[i])
  const out = new Array(len).fill(null)
  for (let i = period; i < len; i++) {
    let posMF = 0
    let negMF = 0
    for (let j = 0; j < period; j++) {
      const idx = i - j
      if (tp[idx] > tp[idx - 1]) posMF += mf[idx]
      else if (tp[idx] < tp[idx - 1]) negMF += mf[idx]
    }
    out[i] = negMF === 0 ? 100 : 100 - 100 / (1 + posMF / negMF)
  }
  return out
}

export function kamaValues(closes, period = 10, fastPeriod = 2, slowPeriod = 30) {
  const len = closes.length
  const out = new Array(len).fill(null)
  if (len < period + 1) return out
  const fastSC = 2 / (fastPeriod + 1)
  const slowSC = 2 / (slowPeriod + 1)
  let kama = closes[period]
  out[period] = kama
  for (let i = period + 1; i < len; i++) {
    const direction = Math.abs(closes[i] - closes[i - period])
    let volatility = 0
    for (let j = 0; j < period; j++) {
      volatility += Math.abs(closes[i - j] - closes[i - j - 1])
    }
    const er = volatility > 0 ? direction / volatility : 0
    const sc = (er * (fastSC - slowSC) + slowSC) ** 2
    kama = kama + sc * (closes[i] - kama)
    out[i] = kama
  }
  return out
}

export function keltnerChannelValues(highs, lows, closes, emaPeriod = 20, atrPeriod = 10, mult = 1.5) {
  const mid = emaFinite(closes, emaPeriod)
  const atr = atrValues(highs, lows, closes, atrPeriod)
  const upper = []
  const lower = []
  for (let i = 0; i < closes.length; i++) {
    if (mid[i] != null && atr[i] != null) {
      upper.push(mid[i] + mult * atr[i])
      lower.push(mid[i] - mult * atr[i])
    } else {
      upper.push(null)
      lower.push(null)
    }
  }
  return { upper, mid, lower }
}

export function supertrendValues(highs, lows, closes, atrPeriod = 10, multiplier = 3) {
  const len = closes.length
  const atr = atrValues(highs, lows, closes, atrPeriod)
  const supertrend = new Array(len).fill(null)
  const direction = new Array(len).fill(0)
  let upperBand = null
  let lowerBand = null
  let prevUpper = null
  let prevLower = null
  let prevDir = 0
  for (let i = 0; i < len; i++) {
    if (atr[i] == null) continue
    const hl2 = (highs[i] + lows[i]) / 2
    let rawUpper = hl2 + multiplier * atr[i]
    let rawLower = hl2 - multiplier * atr[i]
    if (prevUpper != null && rawUpper >= prevUpper && closes[i - 1] <= prevUpper) {
      rawUpper = prevUpper
    }
    if (prevLower != null && rawLower <= prevLower && closes[i - 1] >= prevLower) {
      rawLower = prevLower
    }
    let dir
    if (prevDir === 0) {
      dir = 1
    } else if (prevDir === 1) {
      dir = closes[i] < rawLower ? -1 : 1
    } else {
      dir = closes[i] > rawUpper ? 1 : -1
    }
    upperBand = rawUpper
    lowerBand = rawLower
    supertrend[i] = dir === 1 ? lowerBand : upperBand
    direction[i] = dir
    prevUpper = upperBand
    prevLower = lowerBand
    prevDir = dir
  }
  return { supertrend, direction }
}

export function ichimokuValues(highs, lows, closes, tenkanP = 9, kijunP = 26, senkouBP = 52) {
  const len = closes.length
  function periodHL(h, l, p) {
    const out = new Array(len).fill(null)
    for (let i = p - 1; i < len; i++) {
      let hi = -Infinity
      let lo = Infinity
      for (let j = 0; j < p; j++) {
        hi = Math.max(hi, h[i - j])
        lo = Math.min(lo, l[i - j])
      }
      out[i] = (hi + lo) / 2
    }
    return out
  }
  const tenkan = periodHL(highs, lows, tenkanP)
  const kijun = periodHL(highs, lows, kijunP)
  const senkouB = periodHL(highs, lows, senkouBP)
  const spanA = new Array(len).fill(null)
  const chikou = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (tenkan[i] != null && kijun[i] != null) {
      spanA[i] = (tenkan[i] + kijun[i]) / 2
    }
    if (i + kijunP < len) {
      chikou[i] = closes[i + kijunP]
    }
  }
  return { tenkan, kijun, spanA, senkouB, chikou }
}

export function cciValues(highs, lows, closes, period = 20) {
  const len = closes.length
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sum = 0
    for (let j = 0; j < period; j++) sum += tp[i - j]
    const mean = sum / period
    let meanDev = 0
    for (let j = 0; j < period; j++) meanDev += Math.abs(tp[i - j] - mean)
    meanDev /= period
    out[i] = meanDev > 0 ? (tp[i] - mean) / (0.015 * meanDev) : null
  }
  return out
}

export function ttmSqueezeValues(highs, lows, closes, bollPeriod = 20, bollMult = 2, keltnerPeriod = 20, keltnerAtrPeriod = 10, keltnerMult = 1.5) {
  const boll = bollingerBands(closes, bollPeriod, bollMult)
  const keltner = keltnerChannelValues(highs, lows, closes, keltnerPeriod, keltnerAtrPeriod, keltnerMult)
  const len = closes.length
  const squeeze = new Array(len).fill(false)
  for (let i = 0; i < len; i++) {
    if (boll.lower[i] == null || keltner.lower[i] == null) continue
    squeeze[i] = boll.lower[i] >= keltner.lower[i] && boll.upper[i] <= keltner.upper[i]
  }
  const momentum = new Array(len).fill(null)
  const tp = closes.map((_, i) => (highs[i] + lows[i] + closes[i]) / 3)
  const emaTp = emaFinite(tp, bollPeriod)
  for (let i = 0; i < len; i++) {
    if (emaTp[i] != null) {
      momentum[i] = tp[i] - emaTp[i]
    }
  }
  return { squeeze, momentum }
}

export function sarValues(highs, lows, closes, step = 0.02, maxStep = 0.2) {
  const len = closes.length
  if (len < 2) return { sar: new Array(len).fill(null), direction: new Array(len).fill(0) }
  const sar = new Array(len).fill(null)
  const direction = new Array(len).fill(0)
  let isLong = closes[1] > closes[0]
  let af = step
  let ep = isLong ? highs[1] : lows[1]
  let prevSar = isLong ? lows[0] : highs[0]
  sar[0] = null
  sar[1] = prevSar
  direction[1] = isLong ? 1 : -1
  for (let i = 2; i < len; i++) {
    let curSar = prevSar + af * (ep - prevSar)
    if (isLong) {
      curSar = Math.min(curSar, lows[i - 1], lows[i - 2])
      if (lows[i] < curSar) {
        isLong = false
        curSar = ep
        ep = lows[i]
        af = step
      } else {
        if (highs[i] > ep) {
          ep = highs[i]
          af = Math.min(af + step, maxStep)
        }
      }
    } else {
      curSar = Math.max(curSar, highs[i - 1], highs[i - 2])
      if (highs[i] > curSar) {
        isLong = true
        curSar = ep
        ep = highs[i]
        af = step
      } else {
        if (lows[i] < ep) {
          ep = lows[i]
          af = Math.min(af + step, maxStep)
        }
      }
    }
    sar[i] = curSar
    direction[i] = isLong ? 1 : -1
    prevSar = curSar
  }
  return { sar, direction }
}

export function donchianChannelValues(highs, lows, period = 20) {
  const len = highs.length
  const upper = new Array(len).fill(null)
  const lower = new Array(len).fill(null)
  const mid = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let hi = -Infinity
    let lo = Infinity
    for (let j = 0; j < period; j++) {
      hi = Math.max(hi, highs[i - j])
      lo = Math.min(lo, lows[i - j])
    }
    upper[i] = hi
    lower[i] = lo
    mid[i] = (hi + lo) / 2
  }
  return { upper, mid, lower }
}

export function adxValues(highs, lows, closes, period = 14) {
  const len = closes.length
  if (len < 2) return { adx: new Array(len).fill(null), diP: new Array(len).fill(null), diM: new Array(len).fill(null) }
  const tr = new Array(len).fill(0)
  const plusDM = new Array(len).fill(0)
  const minusDM = new Array(len).fill(0)
  tr[0] = highs[0] - lows[0]
  for (let i = 1; i < len; i++) {
    tr[i] = Math.max(highs[i] - lows[i], Math.abs(highs[i] - closes[i - 1]), Math.abs(lows[i] - closes[i - 1]))
    const upMove = highs[i] - highs[i - 1]
    const downMove = lows[i - 1] - lows[i]
    plusDM[i] = upMove > downMove && upMove > 0 ? upMove : 0
    minusDM[i] = downMove > upMove && downMove > 0 ? downMove : 0
  }
  const smoothTR = new Array(len).fill(null)
  const smoothPDM = new Array(len).fill(null)
  const smoothMDM = new Array(len).fill(null)
  let sTR = 0, sPDM = 0, sMDM = 0
  for (let i = 0; i < period && i < len; i++) {
    sTR += tr[i]; sPDM += plusDM[i]; sMDM += minusDM[i]
  }
  if (len >= period) {
    smoothTR[period - 1] = sTR
    smoothPDM[period - 1] = sPDM
    smoothMDM[period - 1] = sMDM
    for (let i = period; i < len; i++) {
      smoothTR[i] = smoothTR[i - 1] - smoothTR[i - 1] / period + tr[i]
      smoothPDM[i] = smoothPDM[i - 1] - smoothPDM[i - 1] / period + plusDM[i]
      smoothMDM[i] = smoothMDM[i - 1] - smoothMDM[i - 1] / period + minusDM[i]
    }
  }
  const diP = new Array(len).fill(null)
  const diM = new Array(len).fill(null)
  const dx = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (smoothTR[i] != null && smoothTR[i] > 0) {
      diP[i] = 100 * smoothPDM[i] / smoothTR[i]
      diM[i] = 100 * smoothMDM[i] / smoothTR[i]
      const sum = diP[i] + diM[i]
      dx[i] = sum > 0 ? 100 * Math.abs(diP[i] - diM[i]) / sum : 0
    }
  }
  const adx = new Array(len).fill(null)
  if (len >= period * 2 - 1) {
    let sumDx = 0
    for (let i = period - 1; i < period * 2 - 1 && i < len; i++) {
      sumDx += dx[i] || 0
    }
    adx[period * 2 - 2] = sumDx / period
    for (let i = period * 2 - 1; i < len; i++) {
      adx[i] = (adx[i - 1] * (period - 1) + (dx[i] || 0)) / period
    }
  }
  return { adx, diP, diM }
}

export function williamsRValues(highs, lows, closes, period = 14) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let hi = -Infinity
    let lo = Infinity
    for (let j = 0; j < period; j++) {
      hi = Math.max(hi, highs[i - j])
      lo = Math.min(lo, lows[i - j])
    }
    const range = hi - lo
    out[i] = range > 0 ? ((hi - closes[i]) / range) * -100 : null
  }
  return out
}

export function stochRsiValues(closes, rsiPeriod = 14, stochPeriod = 14, kSmooth = 3, dSmooth = 3) {
  const rsi = rsiBundle(closes, rsiPeriod)
  const len = closes.length
  const stochRsi = new Array(len).fill(null)
  for (let i = stochPeriod - 1; i < len; i++) {
    let minRsi = Infinity
    let maxRsi = -Infinity
    let valid = true
    for (let j = 0; j < stochPeriod; j++) {
      if (rsi[i - j] == null) { valid = false; break }
      minRsi = Math.min(minRsi, rsi[i - j])
      maxRsi = Math.max(maxRsi, rsi[i - j])
    }
    if (!valid) continue
    stochRsi[i] = maxRsi !== minRsi ? ((rsi[i] - minRsi) / (maxRsi - minRsi)) * 100 : 0
  }
  const k = new Array(len).fill(null)
  const d = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (stochRsi[i] == null) continue
    let kSum = 0
    let kCnt = 0
    for (let j = 0; j < kSmooth && i - j >= 0; j++) {
      if (stochRsi[i - j] != null) { kSum += stochRsi[i - j]; kCnt++ }
    }
    if (kCnt === kSmooth) k[i] = kSum / kCnt
  }
  for (let i = 0; i < len; i++) {
    if (k[i] == null) continue
    let dSum = 0
    let dCnt = 0
    for (let j = 0; j < dSmooth && i - j >= 0; j++) {
      if (k[i - j] != null) { dSum += k[i - j]; dCnt++ }
    }
    if (dCnt === dSmooth) d[i] = dSum / dCnt
  }
  return { k, d }
}

export function cmfValues(highs, lows, closes, vols, period = 20) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let sumMFV = 0
    let sumVol = 0
    for (let j = 0; j < period; j++) {
      const idx = i - j
      const range = highs[idx] - lows[idx]
      const mfv = range > 0 ? ((closes[idx] - lows[idx]) - (highs[idx] - closes[idx])) / range * vols[idx] : 0
      sumMFV += mfv
      sumVol += vols[idx]
    }
    out[i] = sumVol > 0 ? sumMFV / sumVol : null
  }
  return out
}

export function aroonValues(highs, lows, period = 25) {
  const len = highs.length
  const up = new Array(len).fill(null)
  const down = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let highIdx = 0
    let lowIdx = 0
    for (let j = 1; j < period; j++) {
      if (highs[i - j] > highs[i - highIdx]) highIdx = j
      if (lows[i - j] < lows[i - lowIdx]) lowIdx = j
    }
    up[i] = ((period - 1 - highIdx) / (period - 1)) * 100
    down[i] = ((period - 1 - lowIdx) / (period - 1)) * 100
  }
  return { up, down }
}

export function cmoValues(closes, period = 14) {
  const len = closes.length
  const out = new Array(len).fill(null)
  for (let i = period; i < len; i++) {
    let sumUp = 0
    let sumDown = 0
    for (let j = 0; j < period; j++) {
      const diff = closes[i - j] - closes[i - j - 1]
      if (diff > 0) sumUp += diff
      else sumDown -= diff
    }
    out[i] = sumUp + sumDown > 0 ? ((sumUp - sumDown) / (sumUp + sumDown)) * 100 : 0
  }
  return out
}

export function forceIndexValues(closes, vols, period = 13) {
  const len = closes.length
  if (len < 2) return new Array(len).fill(null)
  const raw = new Array(len).fill(null)
  raw[0] = 0
  for (let i = 1; i < len; i++) {
    raw[i] = (closes[i] - closes[i - 1]) * vols[i]
  }
  const out = emaFinite(raw, period)
  return out
}

export function pivotPointsValues(highs, lows, closes) {
  const len = closes.length
  const pp = new Array(len).fill(null)
  const s1 = new Array(len).fill(null)
  const s2 = new Array(len).fill(null)
  const r1 = new Array(len).fill(null)
  const r2 = new Array(len).fill(null)
  for (let i = 1; i < len; i++) {
    const h = highs[i - 1]
    const l = lows[i - 1]
    const c = closes[i - 1]
    const p = (h + l + c) / 3
    pp[i] = p
    r1[i] = 2 * p - l
    s1[i] = 2 * p - h
    r2[i] = p + (h - l)
    s2[i] = p - (h - l)
  }
  return { pp, s1, s2, r1, r2 }
}

export function demaValues(closes, period = 21) {
  const len = closes.length
  const e1 = emaFinite(closes, period)
  const e1Arr = e1.map(v => v ?? 0)
  const e2 = emaFinite(e1Arr, period)
  const out = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (e1[i] != null && e2[i] != null) {
      out[i] = 2 * e1[i] - e2[i]
    }
  }
  return out
}

export function zigzagValues(highs, lows, closes, threshold = 5) {
  const len = closes.length
  if (len < 3) return { zigzag: new Array(len).fill(null), directions: new Array(len).fill(0) }
  const points = []
  points.push({ idx: 0, price: highs[0], isHigh: true })
  let lastHigh = { idx: 0, price: highs[0] }
  let lastLow = { idx: 0, price: lows[0] }
  let lookingFor = 'high'
  for (let i = 1; i < len; i++) {
    const chgPct = threshold
    if (lookingFor === 'high') {
      if (highs[i] >= lastHigh.price) {
        lastHigh = { idx: i, price: highs[i] }
        if (points.length > 0) points[points.length - 1] = { idx: i, price: highs[i], isHigh: true }
      } else if (lastHigh.price - lows[i] >= lastHigh.price * chgPct / 100) {
        points.push({ idx: lastHigh.idx, price: lastHigh.price, isHigh: true })
        lastLow = { idx: i, price: lows[i] }
        lookingFor = 'low'
      }
    } else {
      if (lows[i] <= lastLow.price) {
        lastLow = { idx: i, price: lows[i] }
        if (points.length > 0) points[points.length - 1] = { idx: i, price: lows[i], isHigh: false }
      } else if (highs[i] - lastLow.price >= lastLow.price * chgPct / 100) {
        points.push({ idx: lastLow.idx, price: lastLow.price, isHigh: false })
        lastHigh = { idx: i, price: highs[i] }
        lookingFor = 'high'
      }
    }
  }
  const zigzag = new Array(len).fill(null)
  const directions = new Array(len).fill(0)
  for (let p = 0; p < points.length; p++) {
    const pt = points[p]
    zigzag[pt.idx] = pt.price
    directions[pt.idx] = pt.isHigh ? 1 : -1
  }
  return { zigzag, directions }
}

/**
 * TD Sequential 神奇九转（DeMark 经典简化实现，A股反转计数）
 * Setup 阶段：连续 9 根，每根收盘 < 4 根前收盘（卖出计数，标记在 K 线上方，红色系）
 * 相反方向为买入计数（标记在 K 线下方，绿色系）。
 * 计数被「与 4 根前比较不成立」打断即清零重数；9 根完成输出完整标记（1-9 全标，9 高亮）。
 * @returns {{ sell: number[], buy: number[] }} 每根 K 的当前计数（0=无，1~9）
 */
export function tdSequentialValues(closes) {
  const len = closes.length
  const sell = new Array(len).fill(0)
  const buy = new Array(len).fill(0)
  if (len < 5) return { sell, buy }
  let sellCnt = 0
  let buyCnt = 0
  for (let i = 4; i < len; i++) {
    const c = closes[i]
    const c4 = closes[i - 4]
    if (c < c4) {
      sellCnt = Math.min(sellCnt + 1, 9)
      buyCnt = 0
    } else if (c > c4) {
      buyCnt = Math.min(buyCnt + 1, 9)
      sellCnt = 0
    } else {
      // 平盘：两计数都中断
      sellCnt = 0
      buyCnt = 0
    }
    sell[i] = sellCnt
    buy[i] = buyCnt
  }
  return { sell, buy }
}

/**
 * BBI 多空指标 = (MA3 + MA6 + MA12 + MA24) / 4（A股本土经典）
 * 收盘上穿 BBI 视为转多、下穿转空；与 BOLL/EMA 同渲染为主图叠加线。
 */
export function bbiValues(closes, p1 = 3, p2 = 6, p3 = 12, p4 = 24) {
  const len = closes.length
  const m1 = smaValues(closes, p1)
  const m2 = smaValues(closes, p2)
  const m3 = smaValues(closes, p3)
  const m4 = smaValues(closes, p4)
  const out = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (m1[i] != null && m2[i] != null && m3[i] != null && m4[i] != null) {
      out[i] = (m1[i] + m2[i] + m3[i] + m4[i]) / 4
    }
  }
  return out
}

/**
 * 涨跌停价位线（A股规则）
 * 以「昨日收盘」为锚：主板 ±10%、创业板/科创板 ±20%、北交所 ±30%、ST ±5%（板块判断在组件层按代码识别）。
 * 锚定规则（兼容日K与分钟K）：按东八区日历日分组，找最后一根K所在交易日之前的最近一个交易日的收盘——
 * 日K即前一根日K收盘，分钟K即前一交易日最后一根收盘（分钟周期不能用前一分钟当锚）。
 * 仅适用于分时与日K周期（涨跌停是日级概念，周/月K下由组件层跳过）。
 * 注意：跌停 = 昨收 × (1 - pct)，不是连乘——跌停后次日仍以新昨收为锚，符合 A 股实际规则。
 * @param {number[]} closes 收盘价序列（与 times 等长）
 * @param {number[]} times unix 秒时间序列（extractOHLCV 的 times，东八区基准）
 * @returns {{ limitUp:number|null, limitDown:number|null, prevClose:number|null }} prevClose=锚定昨收（null=数据不足）
 */
export function limitPriceLines(closes, times, pct = 0.1) {
  const len = closes.length
  if (len === 0 || !times || times.length !== len) return { limitUp: null, limitDown: null, prevClose: null }
  // +8h 再 floor：按东八区日历日分组（数据时间戳均为 +08:00 基准，避免 UTC 日界偏移）
  const dayKey = t => Math.floor((t + 8 * 3600) / 86400)
  const lastDay = dayKey(times[len - 1])
  let anchor = null
  for (let i = len - 1; i >= 0; i--) {
    if (dayKey(times[i]) !== lastDay) {
      anchor = closes[i]
      break
    }
  }
  if (anchor == null || !Number.isFinite(anchor) || anchor <= 0) {
    return { limitUp: null, limitDown: null, prevClose: null }
  }
  return {
    limitUp: roundPrice(anchor * (1 + pct)),
    limitDown: roundPrice(anchor * (1 - pct)),
    prevClose: anchor,
  }
}

/** A股价格 2 位小数四舍五入（交易所真实涨停价即按此规则撮合显示） */
function roundPrice(v) {
  return Math.round(v * 100) / 100
}

/**
 * 涨跌停档位证据（从已加载 K 线数据推断 ±5% ST 档，与名称信号互补）
 *
 * ST 的 ±5% 是交易所硬约束：任一交易日内，价格（含 open/high/low）都不能显著偏离昨收 ±5%。
 * 按东八区交易日分组，对每个交易日计算相对前一交易日收盘的极值：
 *   ext = max((当日最高 − 昨收)/昨收, (昨收 − 当日最低)/昨收)
 * 逐日证据（昨收 ≥ 2 元才采信，规避低价股取整膨胀；开盘偏离昨收 >12% 或 ext >12% 的
 * 交易日视为除权/坏数据直接跳过——涨跌停帽约束含开盘价，主板交易日不可能偏离昨收 12% 以上）：
 * - cap5Hit: 最高/最低恰好触及按昨收四舍五入到分的 5% 帽价 → ST 铁证（ST 股的涨停/跌停价就是这个数）
 * - notStDay: ext ∈ (5.6%, 12%] 或恰好触及 10% 帽价 → 该日绝非 5% 档
 * - recentRegime: 从最近一个交易日往回扫 10 个交易日，第一个「档位指示日」给出当前档位判决
 *   （'st' / 'notSt' / null=近10日无指示）。关键：只认最近——ST 股戴帽前的旧 10% 波动日
 *   留在 K 线窗口里，若拿全历史极值当反证会把名称明明是 ST 的股判回 10%（本函数曾踩此坑）。
 * - days/maxExt/stCaps: 全窗统计（除权日已剔除），供名称缺失时兜底推断。
 * 分时/日K 数据同样适用（分时按日分组后与日K 等价，最后交易日含当日盘中也会入账）；
 * 周月K 因涨跌停为日级概念由组件层跳过。
 * @param {number[]} opens 开盘价序列（与 times 等长，用于剔除除权日；缺省时跳过该项检查）
 */
export function limitBandEvidence(times, highs, lows, closes, opens) {
  const len = closes.length
  const out = { days: 0, maxExt: 0, stCaps: 0, recentRegime: null }
  if (!times || times.length !== len || len === 0) return out
  const hasOpen = !!opens && opens.length === len
  const dayKey = t => Math.floor((t + 8 * 3600) / 86400)
  const cents = v => Math.round(Number(v) * 100)
  const dayRecs = []
  let curDay = dayKey(times[0])
  let curOpen = hasOpen ? opens[0] : null
  let curHigh = highs[0]
  let curLow = lows[0]
  let curClose = closes[0]
  let prevDayClose = null
  const finalizeDay = () => {
    if (prevDayClose == null || prevDayClose < 2) return
    // 开盘大幅偏离昨收=除权/坏数据日，其 ext 是假信号
    if (curOpen != null && Number.isFinite(curOpen) && Math.abs(curOpen - prevDayClose) / prevDayClose > 0.12) return
    const ext = Math.max((curHigh - prevDayClose) / prevDayClose, (prevDayClose - curLow) / prevDayClose)
    if (!Number.isFinite(ext) || ext <= 0) return
    const cap5Up = Math.round(prevDayClose * 1.05 * 100)
    const cap5Down = Math.round(prevDayClose * 0.95 * 100)
    const cap10Up = Math.round(prevDayClose * 1.1 * 100)
    const cap10Down = Math.round(prevDayClose * 0.9 * 100)
    const cap5Hit = cents(curHigh) === cap5Up || cents(curLow) === cap5Down
    dayRecs.push({
      cap5Hit,
      notStDay: !cap5Hit && ((ext > 0.056 && ext <= 0.12) || cents(curHigh) === cap10Up || cents(curLow) === cap10Down),
    })
    out.days++
    if (ext > out.maxExt) out.maxExt = ext
    if (cap5Hit) out.stCaps++
  }
  for (let i = 1; i < len; i++) {
    const dk = dayKey(times[i])
    if (dk === curDay) {
      curHigh = Math.max(curHigh, highs[i])
      curLow = Math.min(curLow, lows[i])
      curClose = closes[i]
      continue
    }
    finalizeDay()
    prevDayClose = curClose
    curDay = dk
    curOpen = hasOpen ? opens[i] : null
    curHigh = highs[i]
    curLow = lows[i]
    curClose = closes[i]
  }
  finalizeDay() // 收尾最后一个交易日（分时场景=今日盘中；此前循环只在日切换时结算，最后一天从未入账）
  // 从最近一日往回扫 10 个交易日，第一个「档位指示日」给出当前档位判决
  for (let i = dayRecs.length - 1, seen = 0; i >= 0 && seen < 10; i--, seen++) {
    if (dayRecs[i].notStDay) { out.recentRegime = 'notSt'; break }
    if (dayRecs[i].cap5Hit) { out.recentRegime = 'st'; break }
  }
  return out
}

/**
 * Weis Wave 威斯波浪（按 ZigZag 波段累积量能）
 * 每个 ZigZag 波段内成交量累加成一根柱：
 * - 上涨波段柱为红（多头推力），下跌波段柱为绿（空头推力）
 * - 柱高 = 波段总成交量（放量推动 vs 缩量调整一目了然）
 * - 波段内逐根累积（非一次到位），柱随波段推进实时生长
 * 配合价格波段：价升量增=健康推动，价升量缩=背离警告。
 * @param {number[]} vols 成交量
 * @param {{zigzag:number[],directions:number[]}} zz zigzagValues 输出
 * @returns {{wave:number[], colors:number[]}} wave=每根K的当前波段累积量（null=无波段），colors=+1/-1 方向
 */
export function weisWaveValues(vols, zz) {
  const len = vols.length
  const wave = new Array(len).fill(null)
  const colors = new Array(len).fill(0)
  if (len === 0 || !zz || !zz.zigzag) return { wave, colors }
  // 找出所有 zigzag 锚点（按索引升序）
  const anchors = []
  for (let i = 0; i < len; i++) {
    if (zz.zigzag[i] != null) anchors.push(i)
  }
  if (anchors.length === 0) return { wave, colors }
  // 波段 = 从上一个锚点到当前锚点（含）；首段从数据起点到第一个锚点
  const segments = []
  let prev = 0
  for (const a of anchors) {
    segments.push({ from: prev, to: a, dir: zz.directions[a] || 0 })
    prev = a
  }
  if (prev < len - 1) {
    // 尾部未完成波段（延续最后一个方向）
    segments.push({ from: prev, to: len - 1, dir: segments.length > 0 ? segments[segments.length - 1].dir * -1 : 0 })
  }
  for (const seg of segments) {
    let acc = 0
    for (let i = seg.from; i <= seg.to; i++) {
      const v = Number.isFinite(vols[i]) ? vols[i] : 0
      acc += v
      wave[i] = acc
      colors[i] = seg.dir
    }
  }
  return { wave, colors }
}

/**
 * 自动背离检测（价格 pivot vs 指标值）
 * 经典 TradingView Divergence Indicator 的通用化实现：
 * - 用左滞后/右滞后 pivot 检测价格摆动高低点（pivotLen 默认 5：两侧各 5 根确认）
 * - 顶背离：价格高点抬高、指标值降低 → 红色标记（Regular Bearish）
 * - 底背离：价格低点降低、指标值抬高 → 绿色标记（Regular Bullish）
 * - 检测窗口：同一方向相邻两个 pivot 之间（间隔不超过 maxRange 根，默认 60，防止跨周期误配）
 * - 返回连线的两个端点（价格坐标 + 指标坐标），由 primitive 在主图画价格连线，副图画指标连线
 * @param {number[]} prices 价格序列（通常 closes）
 * @param {number[]} indicator 指标序列（RSI/MACD DIF 等，与 prices 等长）
 * @param {{pivotLen?:number, maxRange?:number}} opts
 * @returns {{bearish:{i1:number,i2:number,p1:number,p2:number,v1:number,v2:number}[], bullish:同结构[]}}
 */
export function divergenceValues(prices, indicator, { pivotLen = 5, maxRange = 60 } = {}) {
  const len = prices.length
  const bearish = []
  const bullish = []
  if (len < pivotLen * 2 + 2) return { bearish, bullish }

  // pivot 检测：i 为 pivot 需左右各 pivotLen 根都更低（高点）或更高（低点）
  const pivotsHigh = []
  const pivotsLow = []
  for (let i = pivotLen; i < len - pivotLen; i++) {
    if (indicator[i] == null) continue
    let isHigh = true
    let isLow = true
    for (let j = 1; j <= pivotLen; j++) {
      if (prices[i] <= prices[i - j] || prices[i] <= prices[i + j]) isHigh = false
      if (prices[i] >= prices[i - j] || prices[i] >= prices[i + j]) isLow = false
      if (!isHigh && !isLow) break
    }
    if (isHigh) pivotsHigh.push(i)
    if (isLow) pivotsLow.push(i)
  }

  // 顶背离：相邻两个价格高点 pivot，价格抬高 + 指标降低
  for (let a = 0; a < pivotsHigh.length; a++) {
    for (let b = a + 1; b < pivotsHigh.length; b++) {
      const i1 = pivotsHigh[a]
      const i2 = pivotsHigh[b]
      if (i2 - i1 > maxRange) break
      if (indicator[i1] == null || indicator[i2] == null) continue
      if (prices[i2] > prices[i1] && indicator[i2] < indicator[i1]) {
        bearish.push({ i1, i2, p1: prices[i1], p2: prices[i2], v1: indicator[i1], v2: indicator[i2] })
        break // 每个 pivot 只配最近的下一个满足者，避免连环连线
      }
    }
  }
  // 底背离：相邻两个价格低点 pivot，价格降低 + 指标抬高
  for (let a = 0; a < pivotsLow.length; a++) {
    for (let b = a + 1; b < pivotsLow.length; b++) {
      const i1 = pivotsLow[a]
      const i2 = pivotsLow[b]
      if (i2 - i1 > maxRange) break
      if (indicator[i1] == null || indicator[i2] == null) continue
      if (prices[i2] < prices[i1] && indicator[i2] > indicator[i1]) {
        bullish.push({ i1, i2, p1: prices[i1], p2: prices[i2], v1: indicator[i1], v2: indicator[i2] })
        break
      }
    }
  }
  return { bearish, bullish }
}

export function satsValues(highs, lows, closes, vols, {
  atrLen = 14,
  baseMult = 2.0,
  erLen = 20,
  adaptStrength = 0.5,
  atrBaselineLen = 100,
  useAdaptive = true,
  useTqi = true,
  qualityStrength = 0.4,
  qualityCurve = 1.5,
  smoothMult = true,
  useAsymBands = true,
  asymStrength = 0.5,
  useEffAtr = true,
  useCharFlip = true,
  charFlipMinAge = 5,
  charFlipHigh = 0.55,
  charFlipLow = 0.25,
  tqiWeightEr = 0.35,
  tqiWeightVol = 0.20,
  tqiWeightStruct = 0.25,
  tqiWeightMom = 0.20,
  tqiStructLen = 20,
  tqiMomLen = 10,
  volLen = 20,
  multSmoothAlpha = 0.15,
} = {}) {
  const len = closes.length
  const rawAtr = atrValues(highs, lows, closes, atrLen)
  const atrBase = smaValues(rawAtr, atrBaselineLen)
  const outStLine = new Array(len).fill(null)
  const outUpper = new Array(len).fill(null)
  const outLower = new Array(len).fill(null)
  const outDirection = new Array(len).fill(0)
  const outTqi = new Array(len).fill(0)
  let prevLowerBand = null
  let prevUpperBand = null
  let prevDir = 0
  let prevActiveMultSm = null
  let prevPassiveMultSm = null
  let trendStartBar = 0
  const tqiWeightSum = tqiWeightEr + tqiWeightVol + tqiWeightStruct + tqiWeightMom
  const tqiWeightDenom = tqiWeightSum > 0 ? tqiWeightSum : 1
  for (let i = 0; i < len; i++) {
    if (rawAtr[i] == null || atrBase[i] == null) continue
    const atrVal = rawAtr[i]
    const volRatio = atrBase[i] !== 0 ? atrVal / atrBase[i] : 1
    let erValue = 0
    if (i >= erLen) {
      const change = Math.abs(closes[i] - closes[i - erLen])
      let volatility = 0
      for (let j = 0; j < erLen; j++) {
        volatility += Math.abs(closes[i - j] - closes[i - j - 1])
      }
      erValue = volatility !== 0 ? change / volatility : 0
    }
    const effAtr = useEffAtr ? atrVal * (0.5 + 0.5 * erValue) : atrVal
    const tqiEr = Math.max(0, Math.min(1, erValue))
    let tqiVol = 0.5
    if (vols[i] > 0 && i >= volLen) {
      let vMean = 0
      for (let j = 0; j < volLen; j++) vMean += vols[i - j]
      vMean /= volLen
      let vStdSq = 0
      for (let j = 0; j < volLen; j++) {
        const d = vols[i - j] - vMean
        vStdSq += d * d
      }
      const vStd = Math.sqrt(vStdSq / volLen)
      const volZ = vStd !== 0 ? (vols[i] - vMean) / vStd : 0
      const t = Math.max(0, Math.min(1, (volZ - (-1)) / (2 - (-1))))
      tqiVol = t
    } else {
      const t = Math.max(0, Math.min(1, (volRatio - 0.6) / (1.8 - 0.6)))
      tqiVol = t
    }
    let tqiStruct = 0
    if (i >= tqiStructLen) {
      let structHi = -Infinity
      let structLo = Infinity
      for (let j = 0; j < tqiStructLen; j++) {
        structHi = Math.max(structHi, highs[i - j])
        structLo = Math.min(structLo, lows[i - j])
      }
      const structRange = structHi - structLo
      const pricePos = structRange !== 0 ? (closes[i] - structLo) / structRange : 0.5
      tqiStruct = Math.max(0, Math.min(1, Math.abs(pricePos - 0.5) * 2))
    }
    let tqiMom = 0
    if (i >= tqiMomLen) {
      const windowChange = closes[i] - closes[i - tqiMomLen]
      let alignedBars = 0
      for (let j = 0; j < tqiMomLen; j++) {
        const barChange = closes[i - j] - closes[i - j - 1]
        if ((windowChange > 0 && barChange > 0) || (windowChange < 0 && barChange < 0)) {
          alignedBars++
        }
      }
      tqiMom = alignedBars / tqiMomLen
    }
    const tqiRaw = useTqi
      ? (tqiEr * tqiWeightEr + tqiVol * tqiWeightVol + tqiStruct * tqiWeightStruct + tqiMom * tqiWeightMom) / tqiWeightDenom
      : 0.5
    const tqi = Math.max(0, Math.min(1, tqiRaw))
    outTqi[i] = tqi
    const legacyAdaptFactor = useAdaptive ? (1 + adaptStrength * (0.5 - erValue)) : 1
    const qualityDeviation = useTqi ? Math.pow(1 - tqi, qualityCurve) : 0.5
    const tqiMult = 1 - qualityStrength + qualityStrength * (0.6 + 0.8 * qualityDeviation)
    const symMult = baseMult * legacyAdaptFactor * tqiMult
    let activeMultRaw = symMult
    let passiveMultRaw = symMult
    if (useTqi && useAsymBands) {
      const asymTighten = 1 - asymStrength * tqi * 0.3
      const asymWiden = 1 + asymStrength * tqi * 0.4
      activeMultRaw = symMult * asymTighten
      passiveMultRaw = symMult * asymWiden
    }
    const activeMultSm = prevActiveMultSm == null
      ? activeMultRaw
      : (smoothMult ? prevActiveMultSm * (1 - multSmoothAlpha) + activeMultRaw * multSmoothAlpha : activeMultRaw)
    const passiveMultSm = prevPassiveMultSm == null
      ? passiveMultRaw
      : (smoothMult ? prevPassiveMultSm * (1 - multSmoothAlpha) + passiveMultRaw * multSmoothAlpha : passiveMultRaw)
    prevActiveMultSm = activeMultSm
    prevPassiveMultSm = passiveMultSm
    const activeMult = activeMultSm
    const passiveMult = passiveMultSm
    const curPrevDir = prevDir === 0 ? 1 : prevDir
    const lowerMult = curPrevDir === 1 ? activeMult : passiveMult
    const upperMult = curPrevDir === 1 ? passiveMult : activeMult
    const hl2 = (highs[i] + lows[i]) / 2
    const lowerBandRaw = hl2 - lowerMult * effAtr
    const upperBandRaw = hl2 + upperMult * effAtr
    let lowerBand = prevLowerBand == null
      ? lowerBandRaw
      : (closes[i - 1] > prevLowerBand ? Math.max(lowerBandRaw, prevLowerBand) : lowerBandRaw)
    let upperBand = prevUpperBand == null
      ? upperBandRaw
      : (closes[i - 1] < prevUpperBand ? Math.min(upperBandRaw, prevUpperBand) : upperBandRaw)
    const priceFlipUp = prevDir === -1 && prevUpperBand != null && closes[i] > prevUpperBand
    const priceFlipDown = prevDir === 1 && prevLowerBand != null && closes[i] < prevLowerBand
    const trendAge = i - trendStartBar
    const prevTqi = i > 0 ? outTqi[i - 1] : 0.5
    const charFlipCondBase = useCharFlip && useTqi && prevTqi > charFlipHigh && tqi < charFlipLow && trendAge >= charFlipMinAge
    const charFlipDown = charFlipCondBase && curPrevDir === 1 && i > 0 && closes[i] < closes[i - 1]
    const charFlipUp = charFlipCondBase && curPrevDir === -1 && i > 0 && closes[i] > closes[i - 1]
    const finalFlipUp = priceFlipUp || charFlipUp
    const finalFlipDown = priceFlipDown || charFlipDown
    let dir = prevDir === 0 ? 1 : (finalFlipUp ? 1 : (finalFlipDown ? -1 : curPrevDir))
    if (dir !== curPrevDir) trendStartBar = i
    prevLowerBand = lowerBand
    prevUpperBand = upperBand
    prevDir = dir
    outStLine[i] = dir === 1 ? lowerBand : upperBand
    outUpper[i] = upperBand
    outLower[i] = lowerBand
    outDirection[i] = dir
  }
  return { stLine: outStLine, upper: outUpper, lower: outLower, direction: outDirection, tqi: outTqi }
}

export function alligatorValues(highs, lows, closes, jawLen = 13, teethLen = 8, lipsLen = 5, jawOffset = 8, teethOffset = 5, lipsOffset = 3) {
  const len = closes.length
  const jawRaw = smaValues((highs.map((h, i) => (h + lows[i]) / 2)), jawLen)
  const teethRaw = smaValues((highs.map((h, i) => (h + lows[i]) / 2)), teethLen)
  const lipsRaw = smaValues((highs.map((h, i) => (h + lows[i]) / 2)), lipsLen)
  const jaw = new Array(len).fill(null)
  const teeth = new Array(len).fill(null)
  const lips = new Array(len).fill(null)
  for (let i = jawOffset; i < len; i++) {
    if (jawRaw[i - jawOffset] != null) jaw[i] = jawRaw[i - jawOffset]
  }
  for (let i = teethOffset; i < len; i++) {
    if (teethRaw[i - teethOffset] != null) teeth[i] = teethRaw[i - teethOffset]
  }
  for (let i = lipsOffset; i < len; i++) {
    if (lipsRaw[i - lipsOffset] != null) lips[i] = lipsRaw[i - lipsOffset]
  }
  return { jaw, teeth, lips }
}

export function aoValues(highs, lows, fastLen = 5, slowLen = 34) {
  const len = highs.length
  const midprice = highs.map((h, i) => (h + lows[i]) / 2)
  const fastSma = smaValues(midprice, fastLen)
  const slowSma = smaValues(midprice, slowLen)
  const ao = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (fastSma[i] != null && slowSma[i] != null) {
      ao[i] = fastSma[i] - slowSma[i]
    }
  }
  return ao
}

export function hullMaValues(closes, period = 9) {
  const halfLen = Math.floor(period / 2)
  const sqrtLen = Math.floor(Math.sqrt(period))
  const wmaHalf = weightedMaValues(closes, halfLen)
  const wmaFull = weightedMaValues(closes, period)
  const diff = closes.map((_, i) => {
    if (wmaHalf[i] != null && wmaFull[i] != null) return 2 * wmaHalf[i] - wmaFull[i]
    return null
  })
  const wma = weightedMaValues(diff, sqrtLen)
  return wma
}

export function adValues(highs, lows, closes, vols) {
  const len = closes.length
  if (len === 0) return []
  const ad = new Array(len).fill(0)
  for (let i = 0; i < len; i++) {
    const range = highs[i] - lows[i]
    let mfm = 0
    if (range > 0) {
      mfm = ((closes[i] - lows[i]) - (highs[i] - closes[i])) / range
    }
    const mfv = mfm * (vols[i] || 0)
    ad[i] = (i > 0 ? ad[i - 1] : 0) + mfv
  }
  return ad
}

export function trixValues(closes, period = 15) {
  const ema1 = emaFinite(closes, period)
  const ema2 = emaFinite(
    ema1.map(v => v == null ? NaN : v),
    period,
  )
  const ema3 = emaFinite(
    ema2.map(v => v == null ? NaN : v),
    period,
  )
  const trix = new Array(closes.length).fill(null)
  for (let i = 1; i < closes.length; i++) {
    if (ema3[i] != null && ema3[i - 1] != null && ema3[i - 1] !== 0) {
      trix[i] = ((ema3[i] - ema3[i - 1]) / ema3[i - 1]) * 10000
    }
  }
  return trix
}

// TRIX 斜率：TRIX 的一阶差分，衡量三重平滑动量的加速度
export function trixSlopeValues(closes, period = 15) {
  const trix = trixValues(closes, period)
  const slope = new Array(closes.length).fill(null)
  for (let i = 1; i < closes.length; i++) {
    if (trix[i] != null && trix[i - 1] != null) {
      slope[i] = trix[i] - trix[i - 1]
    }
  }
  return slope
}

export function rocValues(closes, period = 12) {
  const len = closes.length
  const roc = new Array(len).fill(null)
  for (let i = period; i < len; i++) {
    if (closes[i - period] !== 0) {
      roc[i] = ((closes[i] - closes[i - period]) / closes[i - period]) * 100
    }
  }
  return roc
}

export function fractalValues(highs, lows, period = 2) {
  const len = highs.length
  const fractalHigh = new Array(len).fill(null)
  const fractalLow = new Array(len).fill(null)
  for (let i = period; i < len - period; i++) {
    let isHigh = true
    let isLow = true
    for (let j = 1; j <= period; j++) {
      if (highs[i] <= highs[i - j] || highs[i] <= highs[i + j]) isHigh = false
      if (lows[i] >= lows[i - j] || lows[i] >= lows[i + j]) isLow = false
    }
    if (isHigh) fractalHigh[i] = highs[i]
    if (isLow) fractalLow[i] = lows[i]
  }
  return { fractalHigh, fractalLow }
}

export function chopValues(highs, lows, closes, period = 14) {
  const len = closes.length
  const chop = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let atrSum = 0
    let ok = true
    for (let j = 0; j < period; j++) {
      const idx = i - j
      let tr
      if (idx - 1 >= 0) {
        tr = Math.max(
          highs[idx] - lows[idx],
          Math.abs(highs[idx] - closes[idx - 1]),
          Math.abs(lows[idx] - closes[idx - 1]),
        )
      } else {
        tr = highs[idx] - lows[idx]
      }
      if (!Number.isFinite(tr)) { ok = false; break }
      atrSum += tr
    }
    if (!ok) continue
    const range = highs[i] - lows[i - period + 1]
    if (range <= 0) continue
    const lowIdx = i - period + 1
    let hi = -Infinity
    let lo = Infinity
    for (let j = lowIdx; j <= i; j++) {
      if (highs[j] > hi) hi = highs[j]
      if (lows[j] < lo) lo = lows[j]
    }
    const trueRange = hi - lo
    if (trueRange <= 0) continue
    chop[i] = 100 * Math.log(atrSum / trueRange) / Math.log(period)
  }
  return chop
}

export function elderRayValues(highs, lows, closes, emaPeriod = 13) {
  const ema = emaFinite(closes, emaPeriod)
  const bullPower = new Array(closes.length).fill(null)
  const bearPower = new Array(closes.length).fill(null)
  for (let i = 0; i < closes.length; i++) {
    if (ema[i] != null) {
      bullPower[i] = highs[i] - ema[i]
      bearPower[i] = lows[i] - ema[i]
    }
  }
  return { bullPower, bearPower }
}

export function chaikinOscValues(highs, lows, closes, vols, fastPeriod = 3, slowPeriod = 10) {
  const ad = adValues(highs, lows, closes, vols)
  const fastEma = emaFinite(ad.map(v => Number.isFinite(v) ? v : NaN), fastPeriod)
  const slowEma = emaFinite(ad.map(v => Number.isFinite(v) ? v : NaN), slowPeriod)
  const co = new Array(closes.length).fill(null)
  for (let i = 0; i < closes.length; i++) {
    if (fastEma[i] != null && slowEma[i] != null) {
      co[i] = fastEma[i] - slowEma[i]
    }
  }
  return co
}

export function vwapBandsValues(highs, lows, closes, vols, period = 20, mult = 2) {
  const len = closes.length
  const vwap = vwapValues(highs, lows, closes, vols, period)
  const upper = new Array(len).fill(null)
  const lower = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    if (vwap[i] == null) continue
    let sumSq = 0
    let cnt = 0
    const start = Math.max(0, i - period + 1)
    for (let j = start; j <= i; j++) {
      const tp = (highs[j] + lows[j] + closes[j]) / 3
      const diff = tp - vwap[i]
      sumSq += diff * diff * (vols[j] || 1)
      cnt += (vols[j] || 1)
    }
    if (cnt > 0) {
      const std = Math.sqrt(sumSq / cnt)
      upper[i] = vwap[i] + mult * std
      lower[i] = vwap[i] - mult * std
    }
  }
  return { vwap, upper, lower }
}

export function massIndexValues(highs, lows, emaPeriod = 9, emaPeriod2 = 9, sumPeriod = 25) {
  const len = highs.length
  const range = new Array(len)
  for (let i = 0; i < len; i++) {
    range[i] = highs[i] - lows[i]
  }
  const singleEma = emaFinite(range, emaPeriod)
  const doubleEma = emaFinite(singleEma.map(v => v == null ? NaN : v), emaPeriod)
  const emaRatio = singleEma.map((v, i) => {
    if (v != null && doubleEma[i] != null && doubleEma[i] !== 0) return v / doubleEma[i]
    return null
  })
  const ratioEma = emaFinite(emaRatio.map(v => v == null ? NaN : v), emaPeriod2)
  const mass = new Array(len).fill(null)
  for (let i = sumPeriod - 1; i < len; i++) {
    let sum = 0
    let ok = true
    for (let j = 0; j < sumPeriod; j++) {
      if (ratioEma[i - j] == null) { ok = false; break }
      sum += ratioEma[i - j]
    }
    if (ok) mass[i] = sum
  }
  return mass
}

export function ulcerIndexValues(closes, period = 14) {
  const len = closes.length
  const ui = new Array(len).fill(null)
  for (let i = period - 1; i < len; i++) {
    let maxClose = -Infinity
    for (let j = 0; j < period; j++) {
      if (closes[i - j] > maxClose) maxClose = closes[i - j]
    }
    let sumSq = 0
    for (let j = 0; j < period; j++) {
      const pctDrawdown = ((closes[i - j] - maxClose) / maxClose) * 100
      sumSq += pctDrawdown * pctDrawdown
    }
    ui[i] = Math.sqrt(sumSq / period)
  }
  return ui
}

export function coppockValues(closes, wmaLen = 10, roc1 = 14, roc2 = 11) {
  const len = closes.length
  const rocA = new Array(len).fill(null)
  const rocB = new Array(len).fill(null)
  for (let i = roc1; i < len; i++) {
    if (closes[i - roc1] !== 0) rocA[i] = ((closes[i] - closes[i - roc1]) / closes[i - roc1]) * 100
  }
  for (let i = roc2; i < len; i++) {
    if (closes[i - roc2] !== 0) rocB[i] = ((closes[i] - closes[i - roc2]) / closes[i - roc2]) * 100
  }
  const sum = closes.map((_, i) => {
    if (rocA[i] != null && rocB[i] != null) return rocA[i] + rocB[i]
    return null
  })
  const coppock = weightedMaValues(sum, wmaLen)
  return coppock
}

export function temaValues(closes, period = 21) {
  const ema1 = emaFinite(closes, period)
  const ema2 = emaFinite(ema1.map(v => v == null ? NaN : v), period)
  const ema3 = emaFinite(ema2.map(v => v == null ? NaN : v), period)
  const tema = new Array(closes.length).fill(null)
  for (let i = 0; i < closes.length; i++) {
    if (ema1[i] != null && ema2[i] != null && ema3[i] != null) {
      tema[i] = ema1[i] + (ema1[i] - ema2[i]) + ((ema1[i] - ema2[i]) - (ema2[i] - ema3[i]))
    }
  }
  return tema
}

// TEMA 斜率组合：返回原始斜率(raw)与 EMA 平滑斜率(smoothed)
// smoothPeriod: 平滑周期，>1 时对原始斜率做 EMA 平滑，<=1 时 smoothed===raw
export function temaSlopeBundle(closes, period = 21, smoothPeriod = 5) {
  const tema = temaValues(closes, period)
  const raw = new Array(closes.length).fill(null)
  for (let i = 1; i < closes.length; i++) {
    if (tema[i] != null && tema[i - 1] != null) {
      raw[i] = tema[i] - tema[i - 1]
    }
  }
  let smoothed = raw
  if (smoothPeriod > 1) {
    smoothed = emaFinite(raw.map(v => v == null ? NaN : v), smoothPeriod)
  }
  return { raw, smoothed }
}

// TEMA 斜率（平滑后）：供信号评估等只需要单条曲线的场景使用
export function temaSlopeValues(closes, period = 21, smoothPeriod = 5) {
  return temaSlopeBundle(closes, period, smoothPeriod).smoothed
}

export function smiValues(highs, lows, closes, kPeriod = 14, dPeriod = 3, emaPeriod = 3) {
  const len = closes.length
  const highest = new Array(len).fill(null)
  const lowest = new Array(len).fill(null)
  for (let i = kPeriod - 1; i < len; i++) {
    let hi = -Infinity
    let lo = Infinity
    for (let j = 0; j < kPeriod; j++) {
      if (highs[i - j] > hi) hi = highs[i - j]
      if (lows[i - j] < lo) lo = lows[i - j]
    }
    highest[i] = hi
    lowest[i] = lo
  }
  const rawSMI = new Array(len).fill(null)
  for (let i = 0; i < len; i++) {
    const range = highest[i] != null && lowest[i] != null ? highest[i] - lowest[i] : null
    if (range != null && range !== 0) {
      rawSMI[i] = 200 * ((closes[i] - (highest[i] + lowest[i]) / 2) / range)
    }
  }
  const smiLine = emaFinite(rawSMI.map(v => v == null ? NaN : v), emaPeriod)
  const signalLine = emaFinite(smiLine.map(v => v == null ? NaN : v), dPeriod)
  return { smi: smiLine, signal: signalLine }
}

export function smcValues(highs, lows, closes, opens, internalLen = 5, swingLen = 50) {
  const len = closes.length
  const swingHighs = new Array(len).fill(null)
  const swingLows = new Array(len).fill(null)
  for (let i = swingLen; i < len - swingLen; i++) {
    let isHigh = true
    let isLow = true
    for (let j = 1; j <= swingLen; j++) {
      if (highs[i] <= highs[i - j] || highs[i] <= highs[i + j]) isHigh = false
      if (lows[i] >= lows[i - j] || lows[i] >= lows[i + j]) isLow = false
      if (!isHigh && !isLow) break
    }
    if (isHigh) swingHighs[i] = highs[i]
    if (isLow) swingLows[i] = lows[i]
  }
  const intHighs = new Array(len).fill(null)
  const intLows = new Array(len).fill(null)
  for (let i = internalLen; i < len - internalLen; i++) {
    let isHigh = true
    let isLow = true
    for (let j = 1; j <= internalLen; j++) {
      if (highs[i] <= highs[i - j] || highs[i] <= highs[i + j]) isHigh = false
      if (lows[i] >= lows[i - j] || lows[i] >= lows[i + j]) isLow = false
      if (!isHigh && !isLow) break
    }
    if (isHigh) intHighs[i] = highs[i]
    if (isLow) intLows[i] = lows[i]
  }
  const bosLines = []
  const chochLines = []
  let lastHighIdx = -1
  let lastLowIdx = -1
  let lastHighVal = -Infinity
  let lastLowVal = Infinity
  let trend = 0
  for (let i = 0; i < len; i++) {
    if (intHighs[i] != null) {
      if (lastHighIdx >= 0 && intHighs[i] > lastHighVal) {
        if (trend === -1) {
          chochLines.push({ time: i, fromIdx: lastHighIdx, fromPrice: lastHighVal, toIdx: i, toPrice: intHighs[i], type: 'choch', bull: true })
          trend = 1
        } else if (trend === 1) {
          bosLines.push({ time: i, fromIdx: lastHighIdx, fromPrice: lastHighVal, toIdx: i, toPrice: intHighs[i], type: 'bos', bull: true })
        }
        if (trend === 0) trend = 1
      }
      lastHighIdx = i
      lastHighVal = intHighs[i]
    }
    if (intLows[i] != null) {
      if (lastLowIdx >= 0 && intLows[i] < lastLowVal) {
        if (trend === 1) {
          chochLines.push({ time: i, fromIdx: lastLowIdx, fromPrice: lastLowVal, toIdx: i, toPrice: intLows[i], type: 'choch', bull: false })
          trend = -1
        } else if (trend === -1) {
          bosLines.push({ time: i, fromIdx: lastLowIdx, fromPrice: lastLowVal, toIdx: i, toPrice: intLows[i], type: 'bos', bull: false })
        }
        if (trend === 0) trend = -1
      }
      lastLowIdx = i
      lastLowVal = intLows[i]
    }
  }
  const swingBosLines = []
  const swingChochLines = []
  let sLastHighIdx = -1
  let sLastLowIdx = -1
  let sLastHighVal = -Infinity
  let sLastLowVal = Infinity
  let sTrend = 0
  for (let i = 0; i < len; i++) {
    if (swingHighs[i] != null) {
      if (sLastHighIdx >= 0 && swingHighs[i] > sLastHighVal) {
        if (sTrend === -1) {
          swingChochLines.push({ time: i, fromIdx: sLastHighIdx, fromPrice: sLastHighVal, toIdx: i, toPrice: swingHighs[i], type: 'choch', bull: true })
          sTrend = 1
        } else if (sTrend === 1) {
          swingBosLines.push({ time: i, fromIdx: sLastHighIdx, fromPrice: sLastHighVal, toIdx: i, toPrice: swingHighs[i], type: 'bos', bull: true })
        }
        if (sTrend === 0) sTrend = 1
      }
      sLastHighIdx = i
      sLastHighVal = swingHighs[i]
    }
    if (swingLows[i] != null) {
      if (sLastLowIdx >= 0 && swingLows[i] < sLastLowVal) {
        if (sTrend === 1) {
          swingChochLines.push({ time: i, fromIdx: sLastLowIdx, fromPrice: sLastLowVal, toIdx: i, toPrice: swingLows[i], type: 'choch', bull: false })
          sTrend = -1
        } else if (sTrend === -1) {
          swingBosLines.push({ time: i, fromIdx: sLastLowIdx, fromPrice: sLastLowVal, toIdx: i, toPrice: swingLows[i], type: 'bos', bull: false })
        }
        if (sTrend === 0) sTrend = -1
      }
      sLastLowIdx = i
      sLastLowVal = swingLows[i]
    }
  }
  const fvgZones = []
  for (let i = 2; i < len; i++) {
    const bullFvgTop = lows[i]
    const bullFvgBot = highs[i - 2]
    if (bullFvgTop > bullFvgBot) {
      fvgZones.push({ startIdx: i - 2, endIdx: i, top: bullFvgTop, bot: bullFvgBot, bull: true, mitigated: false, mitigatedIdx: null })
    }
    const bearFvgBot = highs[i]
    const bearFvgTop = lows[i - 2]
    if (bearFvgBot < bearFvgTop) {
      fvgZones.push({ startIdx: i - 2, endIdx: i, top: bearFvgTop, bot: bearFvgBot, bull: false, mitigated: false, mitigatedIdx: null })
    }
  }
  for (let fi = 0; fi < fvgZones.length; fi++) {
    const fz = fvgZones[fi]
    for (let k = fz.endIdx + 1; k < len; k++) {
      if (fz.bull && lows[k] <= fz.bot) {
        fz.mitigated = true
        fz.mitigatedIdx = k
        break
      }
      if (!fz.bull && highs[k] >= fz.top) {
        fz.mitigated = true
        fz.mitigatedIdx = k
        break
      }
    }
  }
  const atrArr = atrValues(highs, lows, closes, 14)
  const orderBlocks = []
  for (let i = 1; i < len; i++) {
    const isBullOB = closes[i] > highs[i - 1] && closes[i - 1] < opens[i - 1]
    const isBearOB = closes[i] < lows[i - 1] && closes[i - 1] > opens[i - 1]
    if (isBullOB) {
      const obTop = Math.max(opens[i - 1], closes[i - 1])
      const obBot = lows[i - 1]
      const atrVal = atrArr[i] != null ? atrArr[i] : 0
      if (obTop - obBot <= 3 * atrVal || atrVal === 0) {
        orderBlocks.push({ idx: i - 1, top: obTop, bot: obBot, bull: true, mitigated: false, mitigatedIdx: null })
      }
    }
    if (isBearOB) {
      const obTop = highs[i - 1]
      const obBot = Math.min(opens[i - 1], closes[i - 1])
      const atrVal = atrArr[i] != null ? atrArr[i] : 0
      if (obTop - obBot <= 3 * atrVal || atrVal === 0) {
        orderBlocks.push({ idx: i - 1, top: obTop, bot: obBot, bull: false, mitigated: false, mitigatedIdx: null })
      }
    }
  }
  for (let oi = 0; oi < orderBlocks.length; oi++) {
    const ob = orderBlocks[oi]
    for (let k = ob.idx + 2; k < len; k++) {
      if (ob.bull && lows[k] <= ob.bot) {
        ob.mitigated = true
        ob.mitigatedIdx = k
        break
      }
      if (!ob.bull && highs[k] >= ob.top) {
        ob.mitigated = true
        ob.mitigatedIdx = k
        break
      }
    }
  }
  const swingHighPoints = []
  const swingLowPoints = []
  for (let i = 0; i < len; i++) {
    if (swingHighs[i] != null) swingHighPoints.push({ idx: i, price: swingHighs[i] })
    if (swingLows[i] != null) swingLowPoints.push({ idx: i, price: swingLows[i] })
  }
  const intHighPoints = []
  const intLowPoints = []
  for (let i = 0; i < len; i++) {
    if (intHighs[i] != null) intHighPoints.push({ idx: i, price: intHighs[i] })
    if (intLows[i] != null) intLowPoints.push({ idx: i, price: intLows[i] })
  }
  return {
    swingHighs,
    swingLows,
    intHighs,
    intLows,
    bosLines,
    chochLines,
    swingBosLines,
    swingChochLines,
    fvgZones,
    orderBlocks,
    swingHighPoints,
    swingLowPoints,
    intHighPoints,
    intLowPoints,
  }
}
