/**
 * SMI「底部/顶部区域」主图叠加 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 思路：SMI(14,3,3) 范围 -100~+100，但不同品种/周期的 SMI 实际分布差异很大
 * （1 分钟横盘合约 SMI 很少触及经典 ±40），固定阈值容易失效。
 * 因此底部/顶部阈值完全由历史数据预测，无任何固定钳制：
 * - 每根 K 线取其「往前滚动窗口」（默认 300 根）内的有效 SMI 值排序，
 *   P20 分位 = 该时刻的底部阈值，P80 分位 = 顶部阈值
 * - 滚动窗口使阈值随行情状态自适应：趋势市分布整体偏移后，阈值跟着漂移；
 *   早期样本不足（<120 个有效值）时用「从开头累积的扩展窗口」，
 *   仍不足才回退经典阈值 ±40（仅冷启动阶段）
 * - 不做「最低 N% 均值」：均值会被 -100 这类极端尾部拉得过深，导致中等的深谷识别不出来
 * - SMI 连续处于阈值之内（≤底部阈值 / ≥顶部阈值）的区间，即在主图上以
 *   半透明竖带标注：绿色 = 底部区域（潜在反弹/低吸区），红色 = 顶部区域（潜在回落/减仓区）
 *
 * 样本不足时回退经典阈值 ±40。进行中的区间（最后一根 K 线仍处于区域内）标为「进行中」，
 * 右缘虚线 + 标签强调当前所处状态。
 *
 * 只画时间窗口、不画方向：底部/顶部区域代表超卖/超买概率，反转需配合 SMI 上穿/下穿 Signal 确认。
 */

const BOTTOM_RGB = '34,197,94'  // 底部区域：绿 #22c55e（潜在低吸）
const TOP_RGB = '239,68,68'     // 顶部区域：红 #ef4444（潜在回落）
const LABEL_FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'

/** 经典阈值：超卖 / 超买，仅作为冷启动（有效样本不足）时的回退值 */
const SMI_FALLBACK_BOTTOM = -40
const SMI_FALLBACK_TOP = 40

/** 自适应参数：滚动窗口 + P20/P80 分位作为底部/顶部阈值，纯历史数据驱动，无固定钳制 */
const SMI_ZONE_WINDOW = 300
const SMI_ZONE_QUANTILE = 0.2
const SMI_MIN_SAMPLES = 120

/**
 * 按历史数据滚动预测底部/顶部区域阈值：
 * 对每根 K 线 i，取窗口 [i-W+1, i] 内的有效 SMI 值排序，
 * P20 分位 = bottoms[i]，P80 分位 = tops[i]；
 * 窗口尚未累积满时即为扩展窗口（从序列开头累积），有效样本仍 < 120 时保持 ±40 回退值。
 * @param {(number|null)[]} smi
 * @returns {{bottoms:number[], tops:number[], adaptive:boolean}}
 */
export function smiZoneThresholds(smi) {
  const n = smi ? smi.length : 0
  const bottoms = new Array(n).fill(SMI_FALLBACK_BOTTOM)
  const tops = new Array(n).fill(SMI_FALLBACK_TOP)
  let adaptive = false
  for (let i = 0; i < n; i++) {
    const start = Math.max(0, i - SMI_ZONE_WINDOW + 1)
    const vals = []
    for (let j = start; j <= i; j++) {
      const v = smi[j]
      if (v != null && Number.isFinite(v)) vals.push(v)
    }
    if (vals.length >= SMI_MIN_SAMPLES) {
      vals.sort((a, b) => a - b)
      const k = Math.max(1, Math.round(SMI_ZONE_QUANTILE * vals.length))
      bottoms[i] = vals[k - 1]               // 滚动 P20
      tops[i] = vals[vals.length - k]        // 滚动 P80
      adaptive = true
    }
  }
  return { bottoms, tops, adaptive }
}

/**
 * 从 SMI 序列中提取底部/顶部区域（连续处于阈值之内的区间）。
 * @param {(number|null)[]} smi
 * @param {number[]} [bottoms] 逐根底部阈值（滚动窗口预测）
 * @param {number[]} [tops] 逐根顶部阈值（滚动窗口预测）
 * @returns {{start:number,end:number,pending:boolean,type:'bottom'|'top'}[]}
 */
export function computeSmiZones(smi, bottoms, tops) {
  const zones = []
  if (!smi || smi.length === 0) return zones
  let startIdx = -1
  let curType = null
  const flush = (endIdx, pending) => {
    if (startIdx >= 0 && endIdx >= startIdx) {
      zones.push({ start: startIdx, end: endIdx, pending, type: curType })
    }
    startIdx = -1
    curType = null
  }
  for (let i = 0; i < smi.length; i++) {
    const v = smi[i]
    if (v == null || !Number.isFinite(v)) {
      // 数据缺失：当前区间直接截断（避免把断层算进区间）
      flush(i - 1, false)
      continue
    }
    const b = bottoms ? bottoms[i] : SMI_FALLBACK_BOTTOM
    const t = tops ? tops[i] : SMI_FALLBACK_TOP
    const type = v <= b ? 'bottom' : (v >= t ? 'top' : null)
    if (startIdx < 0) {
      if (type) {
        startIdx = i
        curType = type
      }
    } else if (type !== curType) {
      flush(i - 1, false)
      if (type) {
        startIdx = i
        curType = type
      }
    }
  }
  // 末段仍处于区域内 → 进行中
  if (startIdx >= 0) {
    flush(smi.length - 1, true)
  }
  return zones
}

/**
 * 预测买卖点：区域结束 + SMI 上/下穿 Signal 双确认。
 * - 买点：SMI 曾进入底部区域（超卖武装），之后第一根「脱离底部阈值 且 SMI > Signal」的 K 线
 * - 卖点：SMI 曾进入顶部区域（超买武装），之后第一根「脱离顶部阈值 且 SMI < Signal」的 K 线
 * - 武装后 confirmBars 根内未确认则作废（防止陈旧信号）；触发后需重新进入区域才会再次武装
 * @param {(number|null)[]} smi
 * @param {(number|null)[]} signal SMI 的 Signal 线
 * @param {number[]} bottoms 逐根底部阈值
 * @param {number[]} tops 逐根顶部阈值
 * @param {number} [confirmBars] 确认超时（根）
 * @returns {{buys:number[], sells:number[]}} 触发买卖点的 K 线下标
 */
export function computeSmiSignals(smi, signal, bottoms, tops, confirmBars = 10) {
  const buys = []
  const sells = []
  if (!smi || smi.length === 0) return { buys, sells }
  let armed = null // {type:'buy'|'sell', idx}
  for (let i = 0; i < smi.length; i++) {
    const v = smi[i]
    const sig = signal ? signal[i] : null
    if (v == null || !Number.isFinite(v)) continue
    const b = bottoms ? bottoms[i] : SMI_FALLBACK_BOTTOM
    const t = tops ? tops[i] : SMI_FALLBACK_TOP
    // 进入区域 → 武装对应方向（互斥：后进入的覆盖先前的）
    if (v <= b) {
      if (!armed || armed.type !== 'buy') armed = { type: 'buy', idx: i }
    } else if (v >= t) {
      if (!armed || armed.type !== 'sell') armed = { type: 'sell', idx: i }
      continue
    }
    if (!armed) continue
    // 超时作废
    if (i - armed.idx > confirmBars) {
      armed = null
      continue
    }
    // 双确认触发（脱离区域 + 穿越 Signal）
    if (sig == null || !Number.isFinite(sig)) continue
    if (armed.type === 'buy') {
      if (v > b && v > sig) {
        buys.push(i)
        armed = null
      }
    } else if (v < t && v < sig) {
      sells.push(i)
      armed = null
    }
  }
  return { buys, sells }
}

class SmiZoneRenderer {
  constructor() {
    this._zones = []
  }

  setData(zones) {
    this._zones = zones || []
  }

  draw(target) {
    if (this._zones.length === 0) return
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio
      const width = scope.bitmapSize.width
      const height = scope.bitmapSize.height

      for (const z of this._zones) {
        const rgb = z.type === 'bottom' ? BOTTOM_RGB : TOP_RGB
        const x1 = Math.max(0, z.x1 * hpr)
        const x2 = Math.min(width, z.x2 * hpr)
        if (x2 <= 0 || x1 >= width || x2 <= x1) continue

        // 半透明竖带：覆盖整个主图高度（这是时间窗口信号，不是价位区间）
        ctx.fillStyle = `rgba(${rgb},${z.pending ? 0.1 : 0.14})`
        ctx.fillRect(x1, 0, x2 - x1, height)

        // 进行中的区间：右缘虚线 + 标签，强调「当前正处于该区域」
        if (z.pending) {
          ctx.strokeStyle = `rgba(${rgb},0.85)`
          ctx.lineWidth = 1 * hpr
          ctx.setLineDash([4 * hpr, 4 * hpr])
          ctx.beginPath()
          ctx.moveTo(x2, 0)
          ctx.lineTo(x2, height)
          ctx.stroke()
          ctx.setLineDash([])

          const fontSize = 10 * vpr
          ctx.font = `${fontSize}px ${LABEL_FONT_FAMILY}`
          const label = z.label || (z.type === 'bottom' ? 'SMI 底部区域' : 'SMI 顶部区域')
          const tw = ctx.measureText(label).width
          const padX = 4 * hpr
          const padY = 4 * vpr
          // 放在区间内右上角，超出区间则靠右缘对齐
          let tx = x2 - tw - padX * 2
          if (tx < x1 + padX) tx = x1 + padX
          ctx.fillStyle = `rgba(${rgb},0.85)`
          const bx = tx - 3 * hpr
          const by = 10 * vpr
          const bw = tw + 6 * hpr
          const bh = fontSize + padY
          const rr = Math.min(3 * Math.min(hpr, vpr), bw / 2, bh / 2)
          ctx.beginPath()
          ctx.moveTo(bx + rr, by)
          ctx.arcTo(bx + bw, by, bx + bw, by + bh, rr)
          ctx.arcTo(bx + bw, by + bh, bx, by + bh, rr)
          ctx.arcTo(bx, by + bh, bx, by, rr)
          ctx.arcTo(bx, by, bx + bw, by, rr)
          ctx.closePath()
          ctx.fill()
          ctx.fillStyle = '#ffffff'
          ctx.textBaseline = 'middle'
          ctx.textAlign = 'left'
          ctx.fillText(label, tx, by + bh / 2)
        }
      }
    })
  }
}

class SmiZonePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new SmiZoneRenderer()
  }

  update() {
    const prim = this._primitive
    const chart = prim._chart
    const shared = prim._ensureShared()
    if (!shared || !chart || !chart.timeScale()) {
      this._renderer.setData([])
      return
    }
    const ts = chart.timeScale()
    const data = shared.data
    const th = shared.th
    const zones = shared.zones

    let barSpacing = 6
    try {
      barSpacing = ts.options().barSpacing || barSpacing
    } catch (e) { /* ignore */ }

    const marks = []
    for (const z of zones) {
      const x1 = ts.timeToCoordinate(data.times[z.start])
      const x2 = ts.timeToCoordinate(data.times[z.end])
      if (x1 == null || x2 == null) continue
      // 区间右缘纳入信号 K 线本身（半个柱宽），只有 1 根时也保证可见
      // 进行中区间标签显示区间末端（当前）的滚动阈值，阈值纯由历史数据预测
      const endBottom = th && th.bottoms ? th.bottoms[z.end] : null
      const endTop = th && th.tops ? th.tops[z.end] : null
      marks.push({
        x1,
        x2: Math.max(x2 + barSpacing / 2, x1 + barSpacing),
        pending: z.pending,
        type: z.type,
        label: z.pending && th && th.adaptive
          ? (z.type === 'bottom'
            ? `SMI 底部区域 ≤${endBottom != null ? endBottom.toFixed(1) : '-40.0'}`
            : `SMI 顶部区域 ≥${endTop != null ? endTop.toFixed(1) : '40.0'}`)
          : undefined,
      })
    }
    this._renderer.setData(marks)
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'bottom'
  }
}

class SmiMarkerRenderer {
  constructor() {
    this._markers = []
  }

  setData(markers) {
    this._markers = markers || []
  }

  draw(target) {
    if (this._markers.length === 0) return
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio

      for (const m of this._markers) {
        const rgb = m.type === 'buy' ? BOTTOM_RGB : TOP_RGB
        const x = m.x * hpr
        const y = m.y * vpr
        const s = 4.5 * vpr // 三角半径
        ctx.beginPath()
        if (m.type === 'buy') {
          // 上三角，位于 K 线最低价下方
          ctx.moveTo(x, y - s)
          ctx.lineTo(x - s, y + s * 0.6)
          ctx.lineTo(x + s, y + s * 0.6)
        } else {
          // 下三角，位于 K 线最高价上方
          ctx.moveTo(x, y + s)
          ctx.lineTo(x - s, y - s * 0.6)
          ctx.lineTo(x + s, y - s * 0.6)
        }
        ctx.closePath()
        ctx.fillStyle = `rgba(${rgb},0.95)`
        ctx.fill()
        ctx.strokeStyle = 'rgba(255,255,255,0.7)'
        ctx.lineWidth = 1 * hpr
        ctx.stroke()

        // B/S 字母徽章
        const fontSize = 9 * vpr
        ctx.font = `bold ${fontSize}px ${LABEL_FONT_FAMILY}`
        ctx.textAlign = 'center'
        ctx.textBaseline = m.type === 'buy' ? 'top' : 'bottom'
        const ly = m.type === 'buy' ? y + s + 2 * vpr : y - s - 2 * vpr
        ctx.fillStyle = `rgba(${rgb},1)`
        ctx.fillText(m.type === 'buy' ? 'B' : 'S', x, ly)
      }
    })
  }
}

class SmiMarkerPaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new SmiMarkerRenderer()
  }

  update() {
    const prim = this._primitive
    const chart = prim._chart
    const series = prim._series
    const shared = prim._ensureShared()
    if (!shared || !chart || !series || !chart.timeScale()) {
      this._renderer.setData([])
      return
    }
    const ts = chart.timeScale()
    const data = shared.data
    const { buys, sells } = shared.signals

    let barSpacing = 6
    try {
      barSpacing = ts.options().barSpacing || barSpacing
    } catch (e) { /* ignore */ }

    const markers = []
    const push = (idx, type) => {
      const x = ts.timeToCoordinate(data.times[idx])
      if (x == null) return
      let y = null
      if (type === 'buy') {
        const low = data.lows ? data.lows[idx] : null
        const py = low != null && Number.isFinite(low) ? series.priceToCoordinate(low) : null
        // 锚定最低价下方；价格不在可视范围或无低点数据时退到面板底部
        y = py != null ? py + barSpacing * 0.8 : null
      } else {
        const high = data.highs ? data.highs[idx] : null
        const py = high != null && Number.isFinite(high) ? series.priceToCoordinate(high) : null
        y = py != null ? py - barSpacing * 0.8 : null
      }
      if (y == null) return
      markers.push({ x, y, type })
    }
    for (const i of buys) push(i, 'buy')
    for (const i of sells) push(i, 'sell')
    this._renderer.setData(markers)
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    // 标记层置于 K 线之上，避免被实体遮挡
    return 'top'
  }
}

class SmiZonePrimitive {
  /**
   * @param {() => {times:number[], smi:(number|null)[], signal:(number|null)[], highs?:number[], lows?:number[]}|null} getData
   */
  constructor(getData) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getData = getData
    // 共享缓存：阈值/区间/信号只在数据变化时重算一次，滚动/缩放每帧的 update() 直接复用
    this._shared = null
    this._bandView = new SmiZonePaneView(this)
    this._markerView = new SmiMarkerPaneView(this)
  }

  /** 数据引用变化时重算阈值/区间/买卖信号，否则直接复用 */
  _ensureShared() {
    const data = this._getData ? this._getData() : null
    if (!data || !data.times || !data.smi) {
      this._shared = null
      return null
    }
    if (!this._shared || this._shared.data !== data) {
      const th = smiZoneThresholds(data.smi)
      const zones = computeSmiZones(data.smi, th.bottoms, th.tops)
      const signals = computeSmiSignals(data.smi, data.signal, th.bottoms, th.tops)
      this._shared = { data, th, zones, signals }
    }
    return this._shared
  }

  attached(param) {
    this._chart = param.chart
    this._series = param.series
    this._requestUpdate = param.requestUpdate
  }

  detached() {
    this._chart = null
    this._series = null
    this._requestUpdate = null
  }

  updateAllViews() {
    this._bandView.update()
    this._markerView.update()
  }

  paneViews() {
    return [this._bandView, this._markerView]
  }

  requestRedraw() {
    if (this._requestUpdate) {
      try { this._requestUpdate() } catch (e) { /* ignore */ }
    }
  }
}

export function createSmiZonePrimitive(series, getData) {
  const prim = new SmiZonePrimitive(getData)
  series.attachPrimitive(prim)
  return prim
}

export { SmiZonePrimitive }
