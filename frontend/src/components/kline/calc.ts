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

// 最小二乘线性回归斜率：估算序列在 [i-period+1, i] 窗口内的局部趋势（单位：数值/根）。
// 相比「一阶差分再 EMA 平滑」更低噪声、更低滞后，作为 TEMA 速度/加速度的底层估计量。
export function linregSlope(values, period) {
  const n = Math.max(2, period)
  const out = new Array(values.length).fill(null)
  if (values.length < n) return out
  const sumX = n * (n - 1) / 2
  const sumXX = n * (n - 1) * (2 * n - 1) / 6
  const denom = n * sumXX - sumX * sumX
  for (let i = n - 1; i < values.length; i++) {
    let sy = 0
    let sxy = 0
    let ok = true
    for (let j = 0; j < n; j++) {
      const v = values[i - n + 1 + j]
      if (v == null || !Number.isFinite(v)) { ok = false; break }
      sy += v
      sxy += v * j
    }
    if (!ok) { out[i] = null; continue }
    out[i] = (n * sxy - sumX * sy) / denom
  }
  return out
}

// TEMA 趋势动量（速度 + 加速度，ATR 标准化）：
// 以 TEMA(period) 为基准序列，用回归斜率估计其一阶速度 vel（再除以 ATR(14)，单位：ATR/根，无量纲），
// 再用速度差分得到二阶加速度 acc（单位：ATR/根²）。
// ATR 标准化让「过零轴」「加速度爆发」等阈值跨价格/周期尺度一致，
// 取代原「一阶差分 + EMA 平滑 + 固定百分比变化率」的尺度敏感方案。
export function temaVelocityBundle(highs, lows, closes, { period = 21, slopePeriod = 3 } = {}) {
  const tema = temaValues(closes, period)
  const atr = atrValues(highs, lows, closes, 14)
  const rawVel = linregSlope(tema, slopePeriod)
  const vel = new Array(closes.length).fill(null)
  for (let i = 0; i < closes.length; i++) {
    if (rawVel[i] != null && atr[i] != null && atr[i] > 0) vel[i] = rawVel[i] / atr[i]
  }
  const acc = new Array(closes.length).fill(null)
  for (let i = 1; i < closes.length; i++) {
    if (vel[i] != null && vel[i - 1] != null) acc[i] = vel[i] - vel[i - 1]
  }
  return { vel, acc }
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

/**
 * 买卖点各路信号采用等权计数制（2026-09 由加权制简化而来）：每路信号命中记 1 分，
 * 综合得分 = 命中信号路数，满分 = 路数 9；命中越多，箭头强度标识越强。
 *
 * 简化依据（箭头级 A/B，50 只 / 32.2 万根 hfq 日线 / 1991-2026，前向 20 根超额、分段基准）：
 * 加权与等权在选择性对齐后几乎等效（历史实测差异 ≤0.06pp、胜率 ≤0.4pp、Jaccard 78~98%）。
 * 等权 3/4/5 路（对应原加权 2/3/4 分档）对比加权基线：卖侧三档全面持平或改善
 * （+1.08→+1.30 / +1.52→+1.55 / +1.67→+1.71%，标准/严格档 Jaccard 97.9%/99.2% 几乎重合），
 * 买侧小幅 −0.06~−0.12pp（灵敏档箭头 −32%，因等权 3 路比加权 2 分更严），滞后与折让不变。
 *
 * 9 路信号构成与取舍教训（加权时代实测、等权制下依然成立——决定「哪些信号入集」的是组内
 * 多样性而非单信号边际强弱）：震荡组 RSI/KDJ/CCI、动量组 MACD/TEMA/TRIX/ADX、量价组 均价线/放量。
 * CCI 单信号边际为负但必须保留（换成同属 EMA 派生的 TRIX 会让卖点标准档 +0.80%→+0.40%）；
 * ADX 来自 Wilder 的 DI 平滑（与 EMA 族不同源），是唯一单信号边际正且传导到箭头级的成员；
 * 同根顶部形态类信号（长上影/破上轨/顶背离等）边际全负，不纳入。
 */

/** 共振强度满分（9 路信号全部命中）；箭头标签按 命中路数/9 折算百分比 */
export const BUY_SELL_MAX_SCORE = 9

/**
 * 「买卖点预测」多指标共振检测（纯因果：第 i 根只用 [0, i] 数据，无未来函数）
 *
 * 买点看「超卖修复 + 动量转强」，卖点看「超买衰竭 + 动量转弱」，各自 9 路信号，按三类分组：
 *  - 震荡组 osc：RSI(14) 上穿 30 / 下穿 70、KDJ(9) 低位金叉(K<35)/高位死叉(K>65)、CCI(20) 回穿 -100/+100
 *  - 动量组 mom：MACD(12,26,9) 金叉/死叉、TEMA(21,5) 斜率转向、TRIX(15) 斜率转向、
 *                ADX(14) 单根斜率上行且 DI+>DI- / 斜率下行且 DI+<DI-
 *  - 量价组 vol：收复/跌破均价线、放量（本根量 > 1.5 × 近 5 根均量）
 *
 * 三类信号同源性强（RSI/KDJ/CCI 都是震荡指标，MACD/TEMA/TRIX 都派生自 EMA），
 * 因此采用「分组门控」而非纯计数：综合得分 >= minScore 且覆盖组数 >= minGroups 才算信号点，
 * 避免「MACD金叉 + TEMA斜率转正」这类同源双响被误判为强共振。ADX 由 Wilder 的 DI 平滑而来，
 * 与 EMA 族不同源，故加入动量组不产生同源冗余（实测箭头级不劣于原 8 路）。
 *
 * 综合得分 = 共振窗口内命中的信号路数（等权计数，每路 1 分，满分 9）。
 * 历史上曾按「各信号出现后 20 根均收益 − 基准」的实测边际定权（0.585~1.051、两侧合计 7.25），
 * 后经箭头级 A/B 验证：选择性对齐后加权与等权几乎等效（差异 ≤0.06pp、胜率 ≤0.4pp），
 * 权重的增益仅在连续刻度（更细的灵敏度步进），故 2026-09 简化为等权计数制。
 * 边际排序时代的关键结论依然约束信号集构成：单信号边际为负的 CCI/KDJ 必须保留——
 * 其价值在组内多样性（箭头级 A/B 已证：把 CCI 换成同属 EMA 派生的 TRIX 会让卖点标准档 +0.80%→+0.40%）。
 *
 * 滚动共振窗口：真实共振是先后到达的（震荡指标先转向、动量随后确认、量能最后放大），
 * 要求三类信号在同一根同时出现会几乎无信号（实测 1440 根真实噪声数据为 0 个）。
 * 故以 window 根滚动窗口累计命中，只要窗口内覆盖 minGroups 个组即视为共振；
 * 窗口只回看过去，不引入未来函数。
 *
 * 关于「贴近波段极值」：曾试过「极值快速通道」（本根创近 N 根新低/新高即放宽门控、把箭头落到
 * 该根），但实测显著降低准确率——创近 20 根新低的那根本质上多处于下跌途中而非波段底部，等于
 * 半山腰接飞刀（标准档胜率 50%→32%，前向 20 根均收益 +0.6%→-0.9%）。波段低点只能由随后的
 * 反转来确认，不做未来函数就必然滞后几根，故此处不做任何提前。
 *
 * 关于「其余指标」：把 calc.ts 内另约 30 个指标（SAR/一目/九转/Aroon/Coppock/溃疡指数/CHOP/
 * Supertrend/Keltner/ADX/ROC…）因果化后逐个实测前向 20 根超额边际，确有若干显著为正
 * （涨停 +0.66、CHOP<38.2且价在MA20上 +0.48、溃疡指数见顶回落 +0.37、九转买入计9 +0.27、
 * SAR翻多 +0.19；卖向 Aroon下穿且>70 +0.56、一目转换下穿基准 +0.49、Coppock下穿0 +0.41），
 * 但把它们作为**新增得分成员**加入信号集，在箭头级 A/B 中一律无增益、只稀释既有共振
 * （标准档买点超额 +0.11→+0.08/+0.04，提高阈值保持同等选择性后仍不占优），故不纳入信号集。
 * 唯一例外是 ADX 的「单根斜率 + DI 方向」（见上方 2026-09 追加说明）：它不是拐点事件，
 * 而是趋势强度的连续状态，箭头级 A/B 三档双向均不劣于原 8 路，故纳入动量组。
 * 真正有效的是把它们揭示的方向用作**顺势门控**（下述 MA20 门控），见实测数据。
 *
 * 顺势门控：买点候选须收在 MA20 上方、卖点候选须收在 MA20 下方（日K类周期）。
 * 实测（48 只 / 23.6 万根 / 2001-2026，前向 20 根超额、分段基准）标准档：
 * 买点 4097→3440 根、胜率 53.9%→54.9%、超额 +0.11%→+0.37%；卖点 4541→3690 根、
 * 48.4%→49.2%、+0.09%→+0.35%；灵敏/标准/严格三档双向全部改善，MA20 与 EMA21 效果相当，
 * 分时（5 分钟K）实测无增益，故 intraday 时不启用。
 *
 * 波动放大门控（仅卖点、仅日K类周期）：当日 ATR14 高于「近 250 根 ATR 均值」的 1.3 倍时不出卖点。
 * 实测（50 只 / 24.9 万根 / 2001-2026，前向 20 根超额、分段基准）标准档卖点：
 * 3787→2661 根、胜率 55.2%→56.4%、超额 +0.18%→+0.74%，且由「段1 正 / 段2 负」转为两段同号为正
 * （段1 +0.46%→+1.29%、段2 -0.07%→+0.29%）；灵敏（+0.39%→+0.86%）、严格（+0.02%→+0.58%）同向改善。
 * 阈值 1.3 是唯一未在样本内挑选的稳健取值：放宽到 1.4/1.5 改善减半、收紧到 1.2 只是多砍 8% 信号。
 * 正反两向样本外检验（用一段选规则、另一段验证）均通过：段1 选出「剔除波动放大」→ 段2 超额
 * -0.07%→+0.23%；段2 选出同一规则 → 段1 +0.46%→+1.56%。
 * 原因：波动放大阶段由情绪/趋势主导，均值回归型超买回落（CCI/RSI/KDJ 回穿）会踏空在趋势中继上。
 * 2026-09 复测（20 只 / 2.9 万根、前向 20 根）买点侧同样受害：ATR 比值 >1.3 的买点胜率仅 15%~33%，
 * 故买点质量门控（见下）同步纳入「波动平稳」条件（与卖点共用 1.3 阈值）。
 *
 * 状态门控（仅日K/周K等非分时周期）：用 CHOP(14) 区分「趋势 / 过渡 / 震荡」状态。
 * 实测（50 只 / 23.7 万根 / 2001-2026，前向 20 根超额、分段基准）A股日线由动量延续主导：
 * 买点在趋势市（CHOP<45）三档超额 +0.85%/+0.97%/+0.64%（无门控仅 +0.23%/+0.30%/+0.32%），
 * 而震荡市（CHOP>61.8）买点为 +0.00%/+0.07%/-0.59% —— 「震荡市低吸」在日线上更差，故买点只留趋势市；
 * 卖点在趋势市（CHOP<38.2）+1.33%/+1.69%/+1.50%、震荡市（CHOP>61.8）+1.14%/+1.40%/+1.38%，
 * 而过渡区（38.2~61.8）仅 +0.90%，故卖点排除过渡区、取 CHOP 两侧极值。
 * 把基线阈值提到相同箭头数后超额仅 +0.33%~0.36%，说明增益来自状态识别而非「变严格」；
 * 三档双向两段同号为正，CHOP 窗口不足（值 null）时不拦截。分时周期未验证，不启用。
 *
 * 买点质量门控（仅日K类周期，2026-09 调优）：「杠铃」双分支，动量或反转其一成立——
 *   动量分支 = 波动平稳（atrRatio<1.3，与卖点同阈值）&& 60 日涨幅 ≤15%（不追过度延伸）
 *              && RSI14 ≥50（动量确认）&& 收盘不低于 MA250（年线上方才顺势做多）；
 *   反转分支 = 收盘较 MA250 贴水 ≥5%（深跌后已收复 MA20 的企稳反弹，吃超跌修复）。
 * 实测（20 只 / 2.86 万根、前向 20 根；前 ~3.4 年训练段选参，后一年为样本外验证段）：
 *   训练段买胜率 49.0%→60.8%（n 255→130）、均收益 +0.92%→+2.73%，前后两半 58.6%/63.3% 稳定；
 *   验证段（弱市年，全池基准涨占比 42.4%）胜率 41.5%→47.6%、均收益 -0.61%→+0.95%；
 *   上一验证段（牛市年）胜率 56.7%→80.0%、超额 +1.51%→+6.96pp；leave-one-out 无单股依赖。
 *   注意：纯动量分支（无反转分支）在弱市验证段胜率仅 36%——反转分支是弱市对冲，勿删。
 *   各条件窗口不足（null）时该条件放行（与 MA20/CHOP 惯例一致）；分钟周期不启用。
 *
 * 卖点确认门控（仅日K类周期，2026-09 调优）：当根收盘须低于前根（阴跌确认）且 RSI14 ≤50
 * （须处弱势区），避免在强势整理中过早离场。实测同上：训练段卖胜率 55.3%→58.7%（n 161→126）、
 * 前后两半 58.2%/58.9%；验证段 71.9%→74.1%。RSI 窗口不足时放行。
 *
 * 箭头落点：固定在信号首次确认的那根 K 线（簇起点），价格取该根的 low（买）/ high（卖）。
 * 不做极值回填——回填会把箭头画到信号出现之前的几根 K 线上。
 *
 * 跨方向不设约束：买卖点各自独立聚类（同类 minGap 合并），不要求「卖点高于前一个买点、
 * 买点低于前一个卖点」，也不限制买卖点之间的间隔。原先的交替校验会把「下跌中跌破上一个买点」
 * 这类真实信号丢掉（该卖点价格低于前一个买点），故已移除。
 *
 * @param {number[]} highs 最高价序列
 * @param {number[]} lows 最低价序列
 * @param {number[]} closes 收盘价序列
 * @param {number[]} vols 成交量序列
 * @param {{minScore?:number,minGroups?:number,minGap?:number,window?:number,sellWindow?:number,dayKeys?:string[]|null,intraday?:boolean}} [opts]
 *   minScore 命中最少信号路数（等权计数，每路 1 分、满分 9，三档 3/4/5）；minGroups 需覆盖的信号组数（默认 2；取 3 会强制
 *   卖点依赖卖向边际为负的「放量」信号，实测更差，勿改回 3）；minGap 同类聚类间距；
 *   window 买点共振累计窗口根数；sellWindow 卖点共振累计窗口根数（更短=确认更快，默认 2）；
 *   dayKeys 与序列等长的交易日键（分钟周期传入以启用当日累计均价线）；intraday 是否分钟周期
 * @returns {{buys:Array,sells:Array}} 每项 { i, price, score, reasons: string[] }
 */
export function buySellPointsValues(highs, lows, closes, vols, {
  minScore = 4,
  minGroups = 2,
  minGap = 5,
  window = 4,
  sellWindow = 2,
  dayKeys = null,
  intraday = false,
} = {}) {
  const len = closes.length
  const empty = { buys: [], sells: [] }
  if (len < 30) return empty

  const rsi = rsiBundle(closes, 14)
  const { K, D } = kdjBundle(highs, lows, closes, 9)
  const { dif, dea } = macdBundle(closes)
  // TEMA 通道改用「ATR 标准化的回归斜率速度」（temaVelocityBundle）：比原「EMA(5) 平滑一阶差分」
  // 更低噪声、更低滞后、尺度无关，零轴穿越更干净（仍与 TRIX/MACD 同属动量组，不新增路数）。
  const { vel: temaVel } = temaVelocityBundle(highs, lows, closes, { period: 21, slopePeriod: 3 })
  const trixSlope = trixSlopeValues(closes, 15)
  const cci = cciValues(highs, lows, closes, 20)
  const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
  // 分钟周期用「当日累计均价线」（分时本义），日K类周期沿用滚动 20 根 VWAP
  const vwap = (intraday && Array.isArray(dayKeys) && dayKeys.length === len)
    ? cumulativeVwapByDay(highs, lows, closes, vols, dayKeys)
    : vwapValues(highs, lows, closes, vols, 20)
  const volMa = smaValues(vols, 5)
  const crossUp = (arr, i, lv) => arr[i] != null && arr[i - 1] != null && arr[i] > lv && arr[i - 1] <= lv
  const crossDown = (arr, i, lv) => arr[i] != null && arr[i - 1] != null && arr[i] < lv && arr[i - 1] >= lv
  const slopeUp = (i) => temaVel[i] != null && temaVel[i - 1] != null && temaVel[i] > 0 && temaVel[i - 1] <= 0
  const slopeDown = (i) => temaVel[i] != null && temaVel[i - 1] != null && temaVel[i] < 0 && temaVel[i - 1] >= 0
  const trixUp = (i) => trixSlope[i] != null && trixSlope[i - 1] != null && trixSlope[i] > 0 && trixSlope[i - 1] <= 0
  const trixDown = (i) => trixSlope[i] != null && trixSlope[i - 1] != null && trixSlope[i] < 0 && trixSlope[i - 1] >= 0
  const adxUpBar = (i) => adx[i] != null && adx[i - 1] != null && adx[i] > adx[i - 1]
  const adxDownBar = (i) => adx[i] != null && adx[i - 1] != null && adx[i] < adx[i - 1]

  // 逐根采集命中：组别 0=震荡(osc) 1=动量(mom) 2=量价(vol)，等权计数（每路 1 分）
  const buyHits = new Array(len)
  const sellHits = new Array(len)
  for (let i = 1; i < len; i++) {
    const volOk = volMa[i] != null && volMa[i] > 0 && vols[i] > volMa[i] * 1.5

    const b = []
    if (crossUp(rsi, i, 30)) b.push([0, 'RSI上穿30', 1])
    if (K[i] != null && D[i] != null && K[i - 1] != null && D[i - 1] != null && K[i] > D[i] && K[i - 1] <= D[i - 1] && K[i] < 35) b.push([0, 'KDJ低位金叉', 1])
    if (crossUp(cci, i, -100)) b.push([0, 'CCI回穿-100', 1])
    if (dif[i] != null && dea[i] != null && dif[i - 1] != null && dea[i - 1] != null && dif[i] > dea[i] && dif[i - 1] <= dea[i - 1]) {
      b.push([1, dif[i] < 0 ? 'MACD零轴下金叉' : 'MACD金叉', 1])
    }
    if (slopeUp(i)) b.push([1, 'TEMA斜率转正', 1])
    if (trixUp(i)) b.push([1, 'TRIX斜率转正', 1])
    if (adxUpBar(i) && diP[i] > diM[i]) b.push([1, 'ADX转强(DI+>DI-)', 1])
    if (vwap[i] != null && vwap[i - 1] != null && closes[i] > vwap[i] && closes[i - 1] <= vwap[i - 1]) b.push([2, intraday ? '收复均价线' : '收复VWAP', 1])
    if (volOk) b.push([2, '放量', 1])
    buyHits[i] = b

    const s = []
    if (crossDown(rsi, i, 70)) s.push([0, 'RSI下穿70', 1])
    if (K[i] != null && D[i] != null && K[i - 1] != null && D[i - 1] != null && K[i] < D[i] && K[i - 1] >= D[i - 1] && K[i] > 65) s.push([0, 'KDJ高位死叉', 1])
    if (crossDown(cci, i, 100)) s.push([0, 'CCI回穿+100', 1])
    if (dif[i] != null && dea[i] != null && dif[i - 1] != null && dea[i - 1] != null && dif[i] < dea[i] && dif[i - 1] >= dea[i - 1]) {
      s.push([1, dif[i] > 0 ? 'MACD零轴上死叉' : 'MACD死叉', 1])
    }
    if (slopeDown(i)) s.push([1, 'TEMA斜率转负', 1])
    if (trixDown(i)) s.push([1, 'TRIX斜率转负', 1])
    if (adxDownBar(i) && diP[i] < diM[i]) s.push([1, 'ADX转弱(DI+<DI-)', 1])
    if (vwap[i] != null && vwap[i - 1] != null && closes[i] < vwap[i] && closes[i - 1] >= vwap[i - 1]) s.push([2, intraday ? '跌破均价线' : '跌破VWAP', 1])
    if (volOk) s.push([2, '放量', 1])
    sellHits[i] = s
  }

  // 顺势门控（仅日K类周期）：买点须收在 MA20 上方、卖点须收在 MA20 下方。
  // 实测（48 只 / 23.6 万根，前向 20 根超额、分段基准）三档双向均改善，故对日K类周期启用；
  // 分钟周期（intraday）实测无增益（箭头数腰斩、质量持平或略差），不启用。
  const ma20 = intraday ? null : smaValues(closes, 20)
  const trendOk = (i, buy) => !ma20 || ma20[i] == null
    || (buy ? closes[i] > ma20[i] : closes[i] < ma20[i])

  // 波动放大门控（仅卖点、仅日K类周期）：当日 ATR14 >= 近 250 根 ATR 均值 × 1.3 时不出卖点。
  // 基准波动只用「过去 250 根（不含当根）」，故无未来函数；窗口不足 250 根时不拦截。
  // 实测见函数头注释：三档卖点超额均改善，且正反两向样本外检验通过；买点侧实测不受影响，不设门控。
  const ATR_VOL_RATIO = 1.3
  const atr14 = intraday ? null : atrValues(highs, lows, closes, 14)
  let atrRatio = null
  if (atr14) {
    atrRatio = new Array(len).fill(null)
    let sum = 0
    let cnt = 0
    for (let i = 0; i < len; i++) {
      if (i > 0 && atr14[i - 1] != null) { sum += atr14[i - 1]; cnt++ }
      const drop = i - 1 - 250
      if (drop >= 0 && atr14[drop] != null) { sum -= atr14[drop]; cnt-- }
      if (cnt >= 250 && sum > 0 && atr14[i] != null) atrRatio[i] = atr14[i] / (sum / cnt)
    }
  }
  const calmOk = (i) => !atrRatio || atrRatio[i] == null || atrRatio[i] < ATR_VOL_RATIO

  // 状态门控（仅日K/周K等非分时周期）：用 CHOP(14) 区分「趋势 / 过渡 / 震荡」状态。
  // 实测（50 只 / 23.7 万根，前向 20 根超额、分段基准）A股日线是动量延续主导，而非均值回归：
  //   买点在趋势市（CHOP<45）超额 +0.85%/+0.97%/+0.64%（灵敏/标准/严格），无门控仅 +0.23%/+0.30%/+0.32%；
  //   震荡市买点（CHOP>61.8）为 +0.00%/+0.07%/-0.59% —— 「震荡市低吸」在日线上反而更差，故买点只留趋势市。
  //   卖点在趋势市（CHOP<38.2）+1.33%/+1.69%/+1.50%、震荡市（CHOP>61.8）+1.14%/+1.40%/+1.38%，
  //   而过渡区（38.2~61.8）仅 +0.90%，故卖点排除过渡区、取 CHOP 两侧极值。
  // 把基线阈值提到相同箭头数后超额仅 +0.33%~0.36%，说明增益来自状态识别而非「变严格」。
  // 三档双向两段同号为正；CHOP 窗口不足时（值为 null）不拦截。
  const CHOP_TREND = 45
  const CHOP_OSC = 61.8
  const chop14 = intraday ? null : chopValues(highs, lows, closes, 14)
  const regimeBuyOk = (i) => !chop14 || chop14[i] == null || chop14[i] < CHOP_TREND
  const regimeSellOk = (i) => !chop14 || chop14[i] == null
    || chop14[i] < 38.2 || chop14[i] > CHOP_OSC

  // 买点质量门控（仅日K类周期）：「杠铃」双分支（动量 OR 反转），实测与设计说明见函数头注释。
  // 各条件窗口不足（null）时该条件放行；分钟周期不启用（与 MA20/CHOP/ATR 门控一致）。
  const BUY_MOMO60_MAX = 0.15
  const BUY_RSI_MIN = 50
  const BUY_DMA250_MIN = 0
  const BUY_DMA250_MAX = -0.05
  const ma250 = intraday ? null : smaValues(closes, 250)
  const buyQualityOk = (i) => {
    if (intraday) return true
    const m60 = i >= 60 && closes[i - 60] > 0 ? closes[i] / closes[i - 60] - 1 : null
    const d250 = ma250 && ma250[i] != null && ma250[i] > 0 ? closes[i] / ma250[i] - 1 : null
    const momentumOk = calmOk(i)
      && (m60 == null || m60 <= BUY_MOMO60_MAX)
      && (rsi[i] == null || rsi[i] >= BUY_RSI_MIN)
      && (d250 == null || d250 >= BUY_DMA250_MIN)
    const contrarianOk = d250 != null && d250 <= BUY_DMA250_MAX
    return momentumOk || contrarianOk
  }

  // 卖点确认门控（仅日K类周期）：当根收盘须低于前根（阴跌确认）且 RSI14 ≤50（弱势区），
  // 避免在强势整理中过早离场；RSI 窗口不足（null）时放行。实测见函数头注释。
  const SELL_RSI_MAX = 50
  const sellConfirmOk = (i) => intraday
    || (i > 0 && closes[i] < closes[i - 1] && (rsi[i] == null || rsi[i] <= SELL_RSI_MAX))

  // 滚动窗口累计命中 → 共振候选（窗口只回看过去）
  // 卖点用更短的共振窗口（默认 2 根）：卖点信号全是「转折确认型」，窗口越长确认越晚。
  // 实测（50 只 / 23.7 万根 / 2001-2026，前向 20 根超额、分段基准）卖点窗口由 4 收窄到 2：
  //   标准档 超额 +1.42%→+1.53%（段2 +1.08%→+1.51%）、箭头相对局部顶点滞后 6.03→5.45 根、
  //   峰值折让 5.42%→4.72%；灵敏档 +1.15%→+1.09%（两段各 +1.09%）；严格档 +1.39%→+1.76%。
  //   代价是箭头数减少（标准档 3331→1401），可用灵敏度档位补回。买点窗口实测收窄到 2 根无改善，保持 4 根。
  const w = Math.max(1, window)
  const wSell = Math.max(1, sellWindow)
  const candBuys = []
  const candSells = []
  for (let i = Math.max(w, wSell); i < len; i++) {
    const b = collectWindowHits(buyHits, i, w)
    if (b.score >= minScore && b.groups >= minGroups && trendOk(i, true) && regimeBuyOk(i) && buyQualityOk(i)) candBuys.push({ i, ...b })
    const s = collectWindowHits(sellHits, i, wSell)
    if (s.score >= minScore && s.groups >= minGroups && trendOk(i, false) && calmOk(i) && regimeSellOk(i) && sellConfirmOk(i)) candSells.push({ i, ...s })
  }

  // 聚类（同类信号在 minGap 内合并为一簇）
  const clusters = []
  buildClusters(candBuys, true, minGap, clusters)
  buildClusters(candSells, false, minGap, clusters)
  clusters.sort((a, b) => a.start - b.start)

  // 落点固定在信号首次确认的那根 K 线（簇起点），不向前回填极值：
  // 信号在最新一根 K 线上确认时，箭头必须就画在该跟上，否则会出现
  // 「新信号出现、箭头却标在之前几根 K 线」的错位。
  // 不做跨方向的交替校验：卖点低于前一个买点、或买卖点相邻时同样保留，
  // 否则「下跌中跌破了上一个买点」这类真实信号会被丢掉。
  const picked = clusters.map(cl => ({
    i: cl.start,
    price: cl.buy ? lows[cl.start] : highs[cl.start],
    score: cl.score,
    reasons: cl.reasons,
    buy: cl.buy,
  }))

  return {
    buys: picked.filter(p => p.buy).map(({ buy, ...rest }) => rest),
    sells: picked.filter(p => !p.buy).map(({ buy, ...rest }) => rest),
  }
}

/** 累计窗口内的命中：按信号名去重（避免同源信号在窗口内重复计分），返回命中路数（等权得分）、覆盖组数与理由 */
function collectWindowHits(hitsByBar, i, w) {
  const labels = new Map()
  for (let k = Math.max(0, i - w + 1); k <= i; k++) {
    const hits = hitsByBar[k]
    if (!hits) continue
    for (const h of hits) if (!labels.has(h[1])) labels.set(h[1], h)
  }
  const groups = new Set()
  let score = 0
  for (const h of labels.values()) {
    groups.add(h[0])
    score += h[2]
  }
  return {
    score: Math.round(score * 100) / 100,
    groups: groups.size,
    reasons: Array.from(labels.keys()),
  }
}

/** 时序聚类：minGap 根以内的同类候选归为一簇，评分与理由取簇起点（即箭头所在那根 K 线） */
function buildClusters(cands, buy, minGap, out) {
  let cur = null
  for (const c of cands) {
    if (!cur || c.i - cur.end > minGap) {
      if (cur) out.push(cur)
      cur = { buy, start: c.i, end: c.i, score: c.score, reasons: c.reasons }
    } else {
      cur.end = c.i
    }
  }
  if (cur) out.push(cur)
}

/**
 * 「TEMA 转折」独立买卖点系统（与 9 路共振体系 buySellPointsValues 完全独立、互不影响）。
 *
 * 观测序列改为 TEMA(21) 的「速度」vel 与「加速度」acc（见 temaVelocityBundle）：
 * 以 TEMA 为基准、用最小二乘回归斜率估计一阶速度（并 ATR 标准化为无量纲），再差分得到加速度。
 * 相比旧的「一阶差分 + EMA(5) 平滑 + 固定百分比变化率」，回归斜率更低噪声、更低滞后，
 * ATR 标准化则让过零轴 / 爆发阈值跨价格与周期尺度一致，无需再靠价格量级的地板 hack。
 *
 * 输出三类标记（方向语义与渲染沿用原状）：
 * - 预警（kind='pred'）：加速度穿越零轴而速度尚未换向 —— 买向：vel<0 且 acc 上穿 0（速度触底企稳）；
 *   卖向：vel>0 且 acc 下穿 0（镜像）。仅作提前关注提示。
 * - 确认（kind='conf'）：速度穿越零轴（vel 由 ≤0 转 >0 为买确认、由 ≥0 转 <0 为卖确认）。
 * - 急速（kind='impulse'）：确认点当根（及之前）已出现爆发式换向——速度换向前后的加速度显著超过
 *   近期 |acc| 基线（IMPULSE_K 倍），表示「趋势换向带爆发力」。买向爆发偏强、卖向爆发偏警示，
 *   方向不对称结论与渲染（买「T强」/ 卖「T急」）沿用旧版。
 *
 * 门控与密度控制沿用旧经验结论：
 * - minGap 默认 20：同向事件聚簇，每簇保留首个预警与首个「通过门控」的确认。
 * - MA20 顺势门控（仅日K类周期、仅确认点，含 impulse）：买确认须收在 MA20 上、卖确认须收在 MA20 下。
 * - 不做跨方向交替约束。
 * 无未来函数：全部条件只使用截至当根 i 的数据（vel/acc 的 i-1、i 及 MA20[i]、截至 i-1 的滚动 |acc| 均值）。
 *
 * @param {number[]} highs 最高价序列
 * @param {number[]} lows 最低价序列
 * @param {number[]} closes 收盘价序列
 * @param {{period?:number,slopePeriod?:number,minGap?:number,intraday?:boolean}} [opts]
 * @returns {{buys:Array,sells:Array}} 每项 { i, price, kind }，kind: 'pred'（预警）| 'conf'（确认·温和）| 'impulse'（急速穿越·警示）
 */
export function temaTurnPointsValues(highs, lows, closes, {
  period = 21,
  slopePeriod = 3,
  minGap = 20,
  intraday = false,
} = {}) {
  const len = closes.length
  const empty = { buys: [], sells: [] }
  if (len < 30) return empty

  const { vel, acc } = temaVelocityBundle(highs, lows, closes, { period, slopePeriod })

  // MA20 顺势门控（仅日K类周期、仅确认点）
  const ma20 = intraday ? null : smaValues(closes, 20)
  const trendOk = (i, buy) => !ma20 || ma20[i] == null
    || (buy ? closes[i] > ma20[i] : closes[i] < ma20[i])

  // 爆发式换向判别（确认点分级用）：换向瞬间的加速度（已 ATR 标准化）显著超过
  // 近期 |acc| 基线（近 100 根、至少 50 根起算），即「趋势换向带爆发力」；尺度无关。
  const IMPULSE_K = 2.5
  const rollAbsAcc = new Array(len).fill(null)
  {
    let sum = 0
    let cnt = 0
    const dq = []
    for (let i = 0; i < len; i++) {
      const v = acc[i]
      if (v != null) { dq.push(Math.abs(v)); sum += Math.abs(v); cnt++ }
      if (dq.length > 100) { sum -= dq.shift(); cnt-- }
      if (cnt >= 50) rollAbsAcc[i] = sum / cnt
    }
  }
  const impulseChg = (i, buy) => {
    const a = acc[i]
    const rm = rollAbsAcc[i - 1]
    if (a == null || rm == null || rm <= 0) return false
    return buy ? a > IMPULSE_K * rm : a < -IMPULSE_K * rm
  }

  // 逐根采集事件：预警=加速度越零轴（速度触底/见顶，尚未换向）；确认=速度过零轴
  const candBuys = []
  const candSells = []
  for (let i = 3; i < len; i++) {
    const v0 = vel[i]
    const v1 = vel[i - 1]
    const a0 = acc[i]
    const a1 = acc[i - 1]
    if (v0 != null && a0 != null && a1 != null && v0 < 0 && a0 > 0 && a1 <= 0) candBuys.push({ i, kind: 'pred' })
    if (v0 != null && a0 != null && a1 != null && v0 > 0 && a0 < 0 && a1 >= 0) candSells.push({ i, kind: 'pred' })
    if (v0 != null && v1 != null && v0 > 0 && v1 <= 0) candBuys.push({ i, kind: 'conf' })
    if (v0 != null && v1 != null && v0 < 0 && v1 >= 0) candSells.push({ i, kind: 'conf' })
  }

  // 聚类：同向事件在 minGap 根内归为一簇，每簇保留首个预警与首个「通过门控」的确认；
  // 确认点分级（impulse）只由「确认点当根及之前」的事件决定（含当根 acc）——确认点之后
  // 的簇内事件不得改变其分级，否则构成未来函数
  const pick = (cands, buy) => {
    const out = []
    let cur = null
    for (const c of cands) {
      if (!cur || c.i - cur.end > minGap) {
        if (cur) out.push(cur)
        cur = { start: c.i, end: c.i, pred: null, conf: null, strong: false, confStrong: false }
      } else {
        cur.end = c.i
      }
      if (c.kind === 'pred' && cur.pred == null) cur.pred = c.i
      if (impulseChg(c.i, buy)) cur.strong = true
      if (c.kind === 'conf' && cur.conf == null && trendOk(c.i, buy)) {
        cur.conf = c.i
        cur.confStrong = cur.strong
      }
    }
    if (cur) out.push(cur)
    return out
  }

  const buys = []
  const sells = []
  for (const cl of pick(candBuys, true)) {
    const pts = []
    if (cl.pred != null) pts.push({ i: cl.pred, price: lows[cl.pred], kind: 'pred' })
    if (cl.conf != null) {
      pts.push({ i: cl.conf, price: lows[cl.conf], kind: cl.confStrong ? 'impulse' : 'conf' })
    }
    pts.sort((a, b) => a.i - b.i)
    buys.push(...pts)
  }
  for (const cl of pick(candSells, false)) {
    const pts = []
    if (cl.pred != null) pts.push({ i: cl.pred, price: highs[cl.pred], kind: 'pred' })
    if (cl.conf != null) {
      pts.push({ i: cl.conf, price: highs[cl.conf], kind: cl.confStrong ? 'impulse' : 'conf' })
    }
    pts.sort((a, b) => a.i - b.i)
    sells.push(...pts)
  }

  return { buys, sells }
}

/**
 * 当日累计均价线（分时 VWAP）：按交易日分组，日内累计 典型价×量 / 累计量
 * 开盘前 warmup 根不输出（此时均价≈现价，穿越无意义）
 */
function cumulativeVwapByDay(highs, lows, closes, vols, dayKeys, warmup = 5) {
  const len = closes.length
  const out = new Array(len).fill(null)
  let curDay = null
  let cumPV = 0
  let cumV = 0
  let cnt = 0
  for (let i = 0; i < len; i++) {
    if (dayKeys[i] !== curDay) {
      curDay = dayKeys[i]
      cumPV = 0
      cumV = 0
      cnt = 0
    }
    const tp = (highs[i] + lows[i] + closes[i]) / 3
    cumPV += tp * vols[i]
    cumV += vols[i]
    cnt++
    out[i] = cnt >= warmup && cumV > 0 ? cumPV / cumV : null
  }
  return out
}
