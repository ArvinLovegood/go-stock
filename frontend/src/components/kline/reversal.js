const WINDOW = 20
const MAX_WAIT = 10

function emit(events, type, direction, phase, index) {
  events.push({ type, direction, phase, index })
}

function trackPending(events, pending, type, direction, index, close) {
  if (!pending) return null
  if (close < pending.stop || index - pending.index > MAX_WAIT) {
    emit(events, type, direction, 'invalid', index)
    return null
  }
  if (close > pending.trigger) {
    emit(events, type, direction, 'confirm', index)
    return null
  }
  return pending
}

function priceSignals(bars, direction, enabled, events) {
  const { opens, highs, lows, closes, vols } = bars
  const sign = direction === 'bull' ? 1 : -1
  const o = opens.map(v => sign * v)
  const c = closes.map(v => sign * v)
  const h = (sign === 1 ? highs : lows).map(v => sign * v)
  const l = (sign === 1 ? lows : highs).map(v => sign * v)
  let falseBreak = null
  let exhaustion = null
  let double = null
  let firstPivot = null

  for (let i = 0; i < c.length; i++) {
    if (enabled.falseBreak) {
      const hadPending = !!falseBreak
      falseBreak = trackPending(events, falseBreak, 'falseBreak', direction, i, c[i])
      if (!hadPending && !falseBreak && i >= WINDOW) {
        const level = Math.min(...l.slice(i - WINDOW, i))
        if (l[i] < level) {
          if (c[i] > level) {
            emit(events, 'falseBreak', direction, 'confirm', i)
          } else {
            falseBreak = { index: i, trigger: level, stop: l[i] }
            emit(events, 'falseBreak', direction, 'warning', i)
          }
        }
      }
    }

    if (enabled.exhaustion) {
      const hadPending = !!exhaustion
      exhaustion = trackPending(events, exhaustion, 'exhaustion', direction, i, c[i])
      if (!hadPending && !exhaustion && i >= WINDOW && c[i] < c[i - 5]) {
        const previousVolumes = vols.slice(i - WINDOW, i)
        const avg = previousVolumes.reduce((a, b) => a + b, 0) / WINDOW
        const range = h[i] - l[i]
        const body = Math.abs(c[i] - o[i])
        const lowerShadow = Math.min(o[i], c[i]) - l[i]
        const upperShadow = h[i] - Math.max(o[i], c[i])
        const hammer = range > 0 && lowerShadow >= range * 0.5 && upperShadow <= range * 0.25
        const engulfing = c[i - 1] < o[i - 1] && c[i] > o[i] &&
          o[i] <= c[i - 1] && c[i] >= o[i - 1] && body > 0
        if (avg > 0 && Number.isFinite(vols[i]) && vols[i] >= 1.8 * avg && (hammer || engulfing)) {
          exhaustion = { index: i, trigger: h[i], stop: l[i] }
          emit(events, 'exhaustion', direction, 'warning', i)
        }
      }
    }

    if (enabled.double) {
      double = trackPending(events, double, 'double', direction, i, c[i])
      const pivot = i - 2
      if (pivot >= 2 && l[pivot] < l[pivot - 1] && l[pivot] < l[pivot - 2] &&
          l[pivot] < l[pivot + 1] && l[pivot] < l[pivot + 2]) {
        if (firstPivot && !double) {
          const gap = pivot - firstPivot.index
          const scale = Math.abs(firstPivot.price)
          if (gap >= 4 && gap <= 40 && scale > 0 && Math.abs(l[pivot] - firstPivot.price) / scale <= 0.03) {
            const trigger = Math.max(...h.slice(firstPivot.index + 1, pivot))
            const stop = Math.min(firstPivot.price, l[pivot])
            // A neckline crossed before the second pivot was known cannot be signalled retroactively.
            if (c.slice(pivot + 1, i + 1).some(value => value < stop || value > trigger)) {
              firstPivot = { index: pivot, price: l[pivot] }
              continue
            }
            double = { index: i, trigger, stop }
            emit(events, 'double', direction, 'warning', i)
          }
        }
        firstPivot = { index: pivot, price: l[pivot] }
      }
    }
  }
}

function rsiSignals(values, direction, events) {
  let state = null
  const rsi = direction === 'bull' ? values : values.map(v => v == null ? null : 100 - v)
  for (let i = 0; i < rsi.length; i++) {
    const v = rsi[i]
    if (!Number.isFinite(v)) continue
    if (state && i - state.start > 30) {
      if (state.stage === 'confirm') emit(events, 'rsiSwing', direction, 'invalid', i)
      state = null
    }
    if (!state) {
      if (v < 30) state = { stage: 'low', low: v, start: i }
      continue
    }
    if (state.stage === 'low') {
      state.low = Math.min(state.low, v)
      if (v >= 30) {
        state.stage = 'rebound'
        state.peak = v
      }
    } else if (state.stage === 'rebound') {
      if (v >= state.peak) state.peak = v
      else if (v <= state.low) state = null
      else {
        state.stage = 'pullback'
        state.pullback = v
      }
    } else if (state.stage === 'pullback') {
      if (v <= state.low) state = null
      else if (v <= state.pullback) state.pullback = v
      else {
        state.stage = 'confirm'
        emit(events, 'rsiSwing', direction, 'warning', i)
        if (v > state.peak) {
          emit(events, 'rsiSwing', direction, 'confirm', i)
          state = null
        }
      }
    } else if (v <= state.low) {
      emit(events, 'rsiSwing', direction, 'invalid', i)
      state = null
    } else if (v > state.peak) {
      emit(events, 'rsiSwing', direction, 'confirm', i)
      state = null
    }
  }
}

export function reversalSignals(bars, rsi, enabled) {
  const events = []
  for (const direction of ['bull', 'bear']) {
    priceSignals(bars, direction, enabled, events)
    if (enabled.rsiSwing) rsiSignals(rsi, direction, events)
  }
  return events.sort((a, b) => a.index - b.index)
}
