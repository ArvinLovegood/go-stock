/**
 * Mass Index「反转窗口」主图叠加 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * Mass Index(9,9,25) 的信号规则：
 * - MI 越过进入线 = 反转凸起（有大波动要来，但不指方向）
 * - MI 回落跌破离开线 = 确认信号，趋势大概率反转
 * 本 primitive 把「凸起开始 → 确认回落」这段区间以琥珀色半透明竖带标注在主图上，
 * 尚未回落的最后一段标为「预警」，已回落的一段标为「反转」。
 *
 * 阈值自适应：MI 是「单 EMA / 二次 EMA」的比值求和，本身无量纲，27/26.5 对多数品种直接可用；
 * 但不同品种 MI 分布不同：山寨币 MI 长期偏高（固定 27 会几乎持续触发），
 * 低波动品种/短周期（如 1 分钟线的横盘合约）MI 长期低于 27（固定 27 一个信号都抓不到）。
 * 因此进入线取当前可见数据 MI 的 90% 分位（钳制 24~31：上限防失效、下限防噪声过敏），
 * 离开线低 0.5；样本不足时回退 27/26.5 ——与后端 OKX 闸门 okxMassGateState 的口径完全一致。
 *
 * 只画时间窗口、不画方向：MI 本身不指示涨跌方向，方向需配合 SAR/Aroon 等确认。
 */

const ZONE_RGB = '245,158,11' // 与子图 Mass 线同色 #f59e0b
const LABEL_FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'

/** 经典阈值：进入凸起 / 确认反转（回落），同时作为样本不足时的回退值 */
const MI_ENTER = 27
const MI_EXIT = 26.5

/** 自适应阈值参数（与后端 okx_mass_gate.go 保持一致） */
const MI_QUANTILE = 0.9
const MI_HYSTERESIS = 0.5
// 进入线钳制区间：上限防极端品种把线抬到永不触发；下限 24 保证安静品种也能出信号，
// 又不至于低到把常态 MI（25 附近）整段都标成窗口
const MI_ENTER_MIN = 24
const MI_ENTER_MAX = 31
const MI_MIN_SAMPLES = 120

/**
 * 按 MI 序列自身分布计算自适应阈值：进入线 = P90（钳制 24~31），离开线 = 进入线 − 0.5。
 * @param {(number|null)[]} mass
 * @returns {{enter:number, exit:number, adaptive:boolean}}
 */
export function massAdaptiveThresholds(mass) {
  const vals = []
  if (mass) {
    for (const v of mass) {
      if (v != null && Number.isFinite(v)) vals.push(v)
    }
  }
  if (vals.length < MI_MIN_SAMPLES) {
    return { enter: MI_ENTER, exit: MI_EXIT, adaptive: false }
  }
  vals.sort((a, b) => a - b)
  let enter = vals[Math.max(0, Math.ceil(MI_QUANTILE * vals.length) - 1)]
  if (enter < MI_ENTER_MIN) enter = MI_ENTER_MIN
  if (enter > MI_ENTER_MAX) enter = MI_ENTER_MAX
  const exit = enter - MI_HYSTERESIS
  return { enter, exit, adaptive: true }
}

/**
 * 从 Mass Index 序列中提取反转窗口。
 * @param {(number|null)[]} mass
 * @param {number} [enter] 进入凸起的阈值（默认 27）
 * @param {number} [exit] 确认反转（回落）的阈值（默认 26.5）
 * @returns {{start:number,end:number,pending:boolean}[]}
 */
export function computeMassIndexZones(mass, enter = MI_ENTER, exit = MI_EXIT) {
  const zones = []
  if (!mass || mass.length === 0) return zones
  let startIdx = -1
  for (let i = 0; i < mass.length; i++) {
    const v = mass[i]
    if (v == null || !Number.isFinite(v)) {
      // 数据缺失：若凸起中途断档，直接结束当前窗口（避免把断层算进区间）
      if (startIdx >= 0 && i - 1 >= startIdx) {
        zones.push({ start: startIdx, end: i - 1, pending: false })
      }
      startIdx = -1
      continue
    }
    if (startIdx < 0) {
      if (v >= enter) startIdx = i
    } else if (v < exit) {
      zones.push({ start: startIdx, end: i, pending: false })
      startIdx = -1
    }
  }
  // 末段仍在进入线以上：凸起已形成但尚未回落 → 反转预警
  if (startIdx >= 0) {
    zones.push({ start: startIdx, end: mass.length - 1, pending: true })
  }
  return zones
}

class MassZoneRenderer {
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
        const x1 = Math.max(0, z.x1 * hpr)
        const x2 = Math.min(width, z.x2 * hpr)
        if (x2 <= 0 || x1 >= width || x2 <= x1) continue

        // 半透明竖带：覆盖整个主图高度（这是时间窗口信号，不是价位区间）
        ctx.fillStyle = `rgba(${ZONE_RGB},${z.pending ? 0.08 : 0.13})`
        ctx.fillRect(x1, 0, x2 - x1, height)

        // 确认根（或预警段右缘）画虚线，强调「信号发生的位置」
        ctx.strokeStyle = `rgba(${ZONE_RGB},${z.pending ? 0.55 : 0.9})`
        ctx.lineWidth = 1 * hpr
        ctx.setLineDash(z.pending ? [3 * hpr, 4 * hpr] : [5 * hpr, 3 * hpr])
        ctx.beginPath()
        ctx.moveTo(x2, 0)
        ctx.lineTo(x2, height)
        ctx.stroke()
        ctx.setLineDash([])

        // 左上角标签
        const fontSize = 10 * vpr
        ctx.font = `${fontSize}px ${LABEL_FONT_FAMILY}`
        ctx.textBaseline = 'top'
        ctx.textAlign = 'left'
        const label = z.label || (z.pending ? 'MASS 反转预警' : 'MASS 反转窗口')
        const tw = ctx.measureText(label).width
        const padX = 4 * hpr
        const padY = 4 * vpr
        let tx = x1 + padX
        if (tx + tw > x2) tx = Math.max(x1 + padX, x2 - tw - padX)
        // 背景胶囊，避免压在 K 线上看不清
        ctx.fillStyle = `rgba(${ZONE_RGB},0.85)`
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
        ctx.fillText(label, tx, by + bh / 2)
      }
    })
  }
}

class MassZonePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new MassZoneRenderer()
    // 窗口与阈值只在数据变化时重算：滚动/缩放每帧都会调用 update()，排序 600 个 MI 值没必要反复做
    this._cachedData = null
    this._cachedZones = []
    this._cachedThresholds = null
  }

  update() {
    const prim = this._primitive
    const series = prim._series
    const chart = prim._chart
    if (!series || !chart) {
      this._renderer.setData([])
      return
    }
    const data = prim._getData ? prim._getData() : null
    if (!data || !data.times || !data.mass) {
      this._renderer.setData([])
      return
    }
    const ts = chart.timeScale()
    if (!ts) {
      this._renderer.setData([])
      return
    }

    if (this._cachedData !== data) {
      this._cachedThresholds = massAdaptiveThresholds(data.mass)
      this._cachedZones = computeMassIndexZones(data.mass, this._cachedThresholds.enter, this._cachedThresholds.exit)
      this._cachedData = data
    }
    const th = this._cachedThresholds
    const zones = this._cachedZones

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
      marks.push({
        x1,
        x2: Math.max(x2 + barSpacing / 2, x1 + barSpacing),
        pending: z.pending,
        // 阈值被自适应调整时在标签上标出当前进入线，避免用户以为窗口仍按 27 起算
        label: th && th.adaptive
          ? `${z.pending ? 'MASS 反转预警' : 'MASS 反转窗口'} ≥${th.enter.toFixed(1)}`
          : (z.pending ? 'MASS 反转预警' : 'MASS 反转窗口'),
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

class MassZonePrimitive {
  /**
   * @param {() => {times:number[], mass:(number|null)[]}|null} getData
   */
  constructor(getData) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getData = getData
    this._paneView = new MassZonePaneView(this)
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
    this._paneView.update()
  }

  paneViews() {
    return [this._paneView]
  }

  requestRedraw() {
    if (this._requestUpdate) {
      try { this._requestUpdate() } catch (e) { /* ignore */ }
    }
  }
}

export function createMassZonePrimitive(series, getData) {
  const prim = new MassZonePrimitive(getData)
  series.attachPrimitive(prim)
  return prim
}

export { MassZonePrimitive }
