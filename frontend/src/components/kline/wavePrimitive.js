/**
 * K 线图「波浪理论」Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 支持两种模式：
 * - impulse5（完整5浪）：用户选 p0（浪1起点）+ p5（浪5终点），画完整驱动浪
 * - predict3（3浪预测）：用户选 p0（浪1起点）+ p3（浪3当前位置），画已完成浪1-2-3，
 *   预测浪4-5（P3 之后用虚线+半透明区分「预期投影」）
 *
 * 叠加绘制：斐波那契回调线、趋势通道线、波浪折线、浪号标注（1-5）、浪间比例标注。
 * 支持画多个波浪，ESC 清除，Ctrl+Z 撤销。
 * 结构对称于 measurePrimitive.js（Renderer / PaneView / Primitive 三件套 + 工厂函数）。
 *
 * 5 浪价格比例（两模式统一）：A 由锚点系数推导，P1=P0+A, P2=P0+0.382A, P3=P0+2A, P4=P0+1.382A, P5=P0+2.382A
 *   满足 浪2回撤61.8%、浪3扩展161.8%、浪4回撤38.2%、浪5=浪1
 * 5 浪时间比例：0/0.191/0.309/0.691/0.809/1（浪3 耗时最长 0.382W）
 */
import { CLR_RISE, CLR_FALL } from './constants'

// ===== 模块私有辅助 =====

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

function roundRect(ctx, x, y, w, h, r) {
  const rr = Math.max(0, Math.min(r, w / 2, h / 2))
  ctx.beginPath()
  ctx.moveTo(x + rr, y)
  ctx.arcTo(x + w, y, x + w, y + h, rr)
  ctx.arcTo(x + w, y + h, x, y + h, rr)
  ctx.arcTo(x, y + h, x, y, rr)
  ctx.arcTo(x, y, x + w, y, rr)
  ctx.closePath()
}

// ===== 常量 =====

// 5 浪时间比例（斐波那契）：P0→P5 之间 P1-P4 的时间位置
const WAVE_TIME_RATIOS = [0, 0.191, 0.309, 0.691, 0.809, 1]

// 锚点价格系数：anchorIdx → P_anchor = P0 + coeff·A
// impulse5 锚 P5（coeff=2.382）；predict3 锚 P3（coeff=2.0）
const ANCHOR_PRICE_COEFF = { 5: 2.382, 3: 2.0 }

// 斐波那契回调线水平（0%=P0 价, 100%=锚点价）
const FIB_LEVELS = [
  { v: 0,     label: '0%' },
  { v: 0.236, label: '23.6%' },
  { v: 0.382, label: '38.2%' },
  { v: 0.5,   label: '50%' },
  { v: 0.618, label: '61.8%' },
  { v: 0.786, label: '78.6%' },
  { v: 1,     label: '100%' },
]

// 浪间比例标注（贴各浪段中点）
const WAVE_RATIOS = [
  { seg: [1, 2], text: '回撤 61.8%' },  // 浪2 回撤浪1
  { seg: [2, 3], text: '扩展 161.8%' }, // 浪3 扩展浪1
  { seg: [3, 4], text: '回撤 38.2%' },  // 浪4 回撤浪3
]

const FIB_COLOR = '#9aa0a6'
const RATIO_COLOR = '#c8c8c8'

// ===== 纯计算函数 =====

/**
 * 根据两端点计算理想化 5 浪结构（纯数据域，无坐标）
 * 两种模式价格节点公式相同，仅浪1幅度 A 的除数（锚点系数）不同
 * @param {{time:number,price:number}} p0 起点
 * @param {{time:number,price:number}} pEnd 终点（impulse5 为 p5，predict3 为 p3）
 * @param {'impulse5'|'predict3'} mode 绘制模式
 */
function computeWave(p0, pEnd, mode = 'impulse5') {
  const anchorIdx = mode === 'predict3' ? 3 : 5
  const coeff = ANCHOR_PRICE_COEFF[anchorIdx]
  const D = pEnd.price - p0.price             // 总价差（含方向）
  const A = D / coeff                          // 浪1 幅度
  const up = D >= 0                            // 上涨波浪 vs 下跌波浪
  const color = up ? CLR_RISE : CLR_FALL
  const prices = [
    p0.price,             // P0
    p0.price + A,         // P1 = P0 + A
    p0.price + A * 0.382, // P2 = P0 + 0.382A（浪2 回撤 61.8%）
    p0.price + A * 2.0,   // P3 = P0 + 2.0A（浪3 扩展 161.8%）
    p0.price + A * 1.382, // P4 = P0 + 1.382A（浪4 回撤 38.2%）
    p0.price + A * 2.382, // P5 = P0 + 2.382A
  ]
  return {
    p0, pEnd, prices, color, up, mode, anchorIdx,
    // predict3 模式下第一个预测点索引（P4=4），impulse5 为 null（全部为已发生）
    predictFromIdx: mode === 'predict3' ? 4 : null,
  }
}

function clampNum(min, v, max) {
  return Math.max(min, Math.min(v, max))
}

/**
 * 计算一个波浪的全部渲染数据（媒体坐标）
 * 中间点 P1-P4 默认按 WAVE_TIME_RATIOS 像素插值；
 * 若 shape.overrides[i] 存在（被手动拖过的中间点），用其 price+time 覆盖。
 * 时间跨度 W 由锚点反推：W = (xAnchor−x0) / WAVE_TIME_RATIOS[anchorIdx]
 * @param {object} shape {p0, p5, mode?, overrides?}
 * @param {boolean} isPreview 预览态（忽略 overrides）
 */
function calcWaveRenderData(ts, series, shape, isPreview) {
  const { p0, p5: pEnd, mode = 'impulse5', overrides } = shape
  const x0 = ts.timeToCoordinate(p0.time)
  const xAnchor = ts.timeToCoordinate(pEnd.time)   // impulse5: x5；predict3: x3
  if (x0 == null || xAnchor == null) return null
  // 退化保护：两点时间相同则 W=0，所有点塌缩，跳过绘制
  if (Math.abs(xAnchor - x0) < 1) return null

  const wave = computeWave(p0, pEnd, mode)
  const anchorIdx = wave.anchorIdx
  const anchorRatio = WAVE_TIME_RATIOS[anchorIdx]   // 0.691 或 1
  const W = (xAnchor - x0) / anchorRatio            // 总时间跨度

  // 6 个折点 P0-P5
  // 端点 P0 用 x0，锚点 P_anchor 用 xAnchor；中间点默认按 ratio 插值，overrides 覆盖 price+time
  const points = []
  for (let i = 0; i < 6; i++) {
    let priceI = wave.prices[i]
    let xi
    if (i === 0) {
      xi = x0
    } else if (i === anchorIdx) {
      xi = xAnchor
    } else {
      const ov = !isPreview && overrides ? overrides[i] : null
      if (ov) {
        priceI = ov.price
        // override time 转 x 失败时回退到插值 x 并 clamp 到 [x0, xAnchor]，避免跳变
        const ox = ov.time != null ? ts.timeToCoordinate(ov.time) : null
        xi = ox != null ? ox : clampNum(x0, x0 + W * WAVE_TIME_RATIOS[i], xAnchor)
      } else {
        xi = x0 + W * WAVE_TIME_RATIOS[i]
      }
    }
    const yi = series.priceToCoordinate(priceI)
    if (yi == null) return null
    points.push({ x: xi, y: yi })
  }

  // 斐波那契回调线：范围限定为真实选区 [x0, xAnchor]，基准价为 p0→锚点价
  const fibLines = []
  const fibRangeEnd = wave.prices[anchorIdx]        // = pEnd.price（用户选点价格，真实）
  for (const fl of FIB_LEVELS) {
    const price = p0.price + (fibRangeEnd - p0.price) * fl.v
    const y = series.priceToCoordinate(price)
    if (y == null) continue
    fibLines.push({ y, label: fl.label })
  }

  // 趋势通道：基线 P2-P4，平行线过 P3，延伸到 x5（预测终点，投影语义）
  const x5 = points[5].x
  let channelBase = null
  let channelParallel = null
  if (points[4].x !== points[2].x) {
    const m = (points[4].y - points[2].y) / (points[4].x - points[2].x)
    channelBase = {
      x1: x0, y1: points[2].y + m * (x0 - points[2].x),
      x2: x5, y2: points[2].y + m * (x5 - points[2].x),
    }
    channelParallel = {
      x1: x0, y1: points[3].y + m * (x0 - points[3].x),
      x2: x5, y2: points[3].y + m * (x5 - points[3].x),
    }
  }

  // 浪间比例标注（贴浪段中点；predict3 中预测段标记 predicted）
  const ratios = []
  for (const r of WAVE_RATIOS) {
    const a = points[r.seg[0]]
    const b = points[r.seg[1]]
    ratios.push({
      x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, text: r.text,
      predicted: wave.predictFromIdx != null && r.seg[1] >= wave.predictFromIdx,
    })
  }

  return {
    color: wave.color,
    up: wave.up,
    isPreview,
    startPointOnly: false,
    points,
    x0,
    fibX1: x0,           // fib 线左端
    fibX2: xAnchor,      // fib 线右端（真实选区右端，标签贴此）
    x5,                  // 通道线延伸终点
    fibLines,
    channelBase,
    channelParallel,
    ratios,
    anchorIdx: wave.anchorIdx,
    predictFromIdx: wave.predictFromIdx,   // null 或 4
  }
}

// ===== WaveRenderer（实现 IPrimitivePaneRenderer）=====

class WaveRenderer {
  constructor() {
    this._shapes = []   // 已完成波浪渲染数据
    this._preview = null // 当前预览渲染数据
  }

  setData({ shapes, preview }) {
    this._shapes = shapes || []
    this._preview = preview || null
  }

  draw(target) {
    if (this._shapes.length === 0 && !this._preview) return
    target.useBitmapCoordinateSpace(scope => {
      for (const s of this._shapes) this._drawWave(scope, s, false)
      if (this._preview) this._drawWave(scope, this._preview, true)
    })
  }

  _drawWave(scope, wave, isPreview) {
    // 仅有 p0 无 pEnd：画起点圆点标记
    if (wave.startPointOnly) {
      this._drawPoint(scope, wave.points[0], wave.color)
      return
    }
    const alpha = isPreview ? 0.6 : 1.0
    this._drawFibLines(scope, wave, alpha)
    this._drawChannel(scope, wave, alpha)
    this._drawPolyline(scope, wave, isPreview, alpha)
    this._drawWaveNumbers(scope, wave, alpha)
    this._drawRatios(scope, wave, alpha)
  }

  // 斐波那契回调线（7 条横线 + 百分比标签贴真实右端）
  _drawFibLines(scope, wave, alpha) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    const x1 = Math.round(wave.fibX1 * hpr)
    const x2 = Math.round(wave.fibX2 * hpr)
    ctx.lineWidth = Math.max(1, Math.round(0.8 * Math.min(hpr, vpr)))
    ctx.setLineDash([3 * hpr, 2 * hpr])
    for (const fl of wave.fibLines) {
      const y = Math.round(fl.y * vpr)
      ctx.strokeStyle = hexToRgba(FIB_COLOR, alpha * 0.7)
      ctx.beginPath()
      ctx.moveTo(x1, y)
      ctx.lineTo(x2, y)
      ctx.stroke()
    }
    ctx.setLineDash([])
    // 标签贴真实右端
    for (const fl of wave.fibLines) {
      const y = Math.round(fl.y * vpr)
      this._drawText(ctx, fl.label, x2 + 4 * hpr, y, hpr, vpr,
        hexToRgba(FIB_COLOR, alpha), 'left', 'middle', 10)
    }
  }

  // 趋势通道线（P2-P4 基线 + 过 P3 平行线，虚线；predict3 含预测成分降透明度）
  _drawChannel(scope, wave, alpha) {
    if (!wave.channelBase || !wave.channelParallel) return
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    // predict3 中通道部分基于预测点 P4，整体作为投影处理
    const channelAlpha = wave.predictFromIdx != null ? alpha * 0.3 : alpha * 0.45
    ctx.lineWidth = Math.max(1, Math.round(1 * Math.min(hpr, vpr)))
    ctx.strokeStyle = hexToRgba(wave.color, channelAlpha)
    ctx.setLineDash([5 * hpr, 3 * hpr])
    // 基线 P2-P4
    const b = wave.channelBase
    ctx.beginPath()
    ctx.moveTo(Math.round(b.x1 * hpr), Math.round(b.y1 * vpr))
    ctx.lineTo(Math.round(b.x2 * hpr), Math.round(b.y2 * vpr))
    ctx.stroke()
    // 平行线 过 P3
    const p = wave.channelParallel
    ctx.beginPath()
    ctx.moveTo(Math.round(p.x1 * hpr), Math.round(p.y1 * vpr))
    ctx.lineTo(Math.round(p.x2 * hpr), Math.round(p.y2 * vpr))
    ctx.stroke()
    ctx.setLineDash([])
  }

  // 波浪折线 P0→P1→P2→P3→P4→P5
  // predict3：P0→P[pf-1] 实线（真实走势），P[pf-1]→P5 虚线半透明（预期投影）
  _drawPolyline(scope, wave, isPreview, alpha) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    ctx.lineWidth = Math.max(1.5, Math.round(1.8 * Math.min(hpr, vpr)))
    ctx.lineJoin = 'round'
    ctx.lineCap = 'round'
    const pf = wave.predictFromIdx   // null 或 4

    const drawPath = (start, end) => {
      ctx.beginPath()
      for (let i = start; i <= end; i++) {
        const px = Math.round(wave.points[i].x * hpr)
        const py = Math.round(wave.points[i].y * vpr)
        if (i === start) ctx.moveTo(px, py)
        else ctx.lineTo(px, py)
      }
      ctx.stroke()
    }

    if (pf == null) {
      // impulse5：全实线（预览态仍按原逻辑加预览虚线）
      ctx.strokeStyle = hexToRgba(wave.color, alpha)
      if (isPreview) ctx.setLineDash([6 * hpr, 3 * hpr])
      drawPath(0, 5)
      ctx.setLineDash([])
    } else {
      // predict3：P0→P[pf-1] 实线（真实走势），P[pf-1]→P5 虚线半透明（预测）
      // 实线段即使在预览态也保持实线（这是已发生的真实走势）
      ctx.strokeStyle = hexToRgba(wave.color, alpha)
      drawPath(0, pf - 1)
      ctx.setLineDash([6 * hpr, 3 * hpr])
      ctx.strokeStyle = hexToRgba(wave.color, alpha * 0.55)
      drawPath(pf - 1, 5)
      ctx.setLineDash([])
    }
  }

  // 浪号标注：P0 画小圆点，P1-P5 画填充圆 + 白边 + 数字；预测点降透明度
  _drawWaveNumbers(scope, wave, alpha) {
    const pf = wave.predictFromIdx
    for (let i = 0; i < wave.points.length; i++) {
      const isPred = pf != null && i >= pf   // P4/P5 预测
      const ptAlpha = isPred ? alpha * 0.5 : alpha
      if (i === 0) {
        // P0 起点小圆点
        this._drawPoint(scope, wave.points[i], hexToRgba(wave.color, ptAlpha))
      } else {
        // P1-P5 浪号圆
        this._drawWaveNumber(scope, wave.points[i], String(i), wave.color, ptAlpha)
      }
    }
  }

  _drawPoint(scope, pt, color) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    const cx = Math.round(pt.x * hpr)
    const cy = Math.round(pt.y * vpr)
    const r = Math.max(3, Math.round(4 * Math.min(hpr, vpr)))
    ctx.beginPath()
    ctx.arc(cx, cy, r, 0, Math.PI * 2)
    ctx.fillStyle = color
    ctx.fill()
    ctx.lineWidth = Math.max(1, Math.round(1 * hpr))
    ctx.strokeStyle = '#fff'
    ctx.stroke()
  }

  _drawWaveNumber(scope, pt, num, color, alpha) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    const cx = Math.round(pt.x * hpr)
    const cy = Math.round(pt.y * vpr)
    const r = Math.max(7, Math.round(9 * Math.min(hpr, vpr)))
    // 填充圆
    ctx.beginPath()
    ctx.arc(cx, cy, r, 0, Math.PI * 2)
    ctx.fillStyle = hexToRgba(color, alpha)
    ctx.fill()
    // 白边
    ctx.lineWidth = Math.max(1, Math.round(1.5 * hpr))
    ctx.strokeStyle = '#fff'
    ctx.stroke()
    // 数字
    ctx.fillStyle = '#fff'
    ctx.textBaseline = 'middle'
    ctx.textAlign = 'center'
    ctx.font = `bold ${Math.max(9, Math.round(11 * hpr))}px -apple-system, "Segoe UI", "Microsoft YaHei", sans-serif`
    ctx.fillText(num, cx, cy)
  }

  // 浪间比例标注（贴浪段中点小字 + 半透明背景；预测段降透明度）
  _drawRatios(scope, wave, alpha) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    const fontLogical = 10
    const fontStr = `${fontLogical}px -apple-system, "Segoe UI", "Microsoft YaHei", sans-serif`
    ctx.font = fontStr
    for (const r of wave.ratios) {
      const rAlpha = r.predicted ? alpha * 0.55 : alpha
      const m = ctx.measureText(r.text)
      const padX = 4, padY = 2
      const w = Math.ceil((m.width + padX * 2) * hpr)
      const h = Math.ceil((fontLogical + padY * 2) * vpr)
      const cx = Math.round(r.x * hpr)
      const cy = Math.round(r.y * vpr)
      const bx = cx - Math.round(w / 2)
      const by = cy - Math.round(h / 2)
      // 背景
      ctx.fillStyle = 'rgba(20, 20, 23, 0.78)'
      roundRect(ctx, bx, by, w, h, 3 * Math.min(hpr, vpr))
      ctx.fill()
      // 文本
      ctx.fillStyle = hexToRgba(RATIO_COLOR, rAlpha)
      ctx.textBaseline = 'middle'
      ctx.textAlign = 'center'
      ctx.font = fontStr
      ctx.fillText(r.text, cx, cy)
    }
  }

  _drawText(ctx, text, x, y, hpr, vpr, color, align, baseline, fontLogical) {
    ctx.fillStyle = color
    ctx.textBaseline = baseline
    ctx.textAlign = align
    ctx.font = `${fontLogical}px -apple-system, "Segoe UI", "Microsoft YaHei", sans-serif`
    ctx.fillText(text, x, y)
  }
}

// ===== WavePaneView（实现 IPrimitivePaneView）=====

class WavePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new WaveRenderer()
  }

  update() {
    const prim = this._primitive
    const chart = prim._chart
    const series = prim._series
    if (!chart || !series) {
      prim._lastRenderPoints = []
      this._renderer.setData({ shapes: [], preview: null })
      return
    }
    const ts = chart.timeScale()

    // 已完成波浪（mode 优先取 shape 自带，兼容旧数据）
    const shapes = []
    const renderPts = []
    for (const s of prim._shapes) {
      const rd = calcWaveRenderData(ts, series, s, false)
      if (rd) {
        shapes.push(rd)
        renderPts.push(rd.points)
      } else {
        renderPts.push(null)
      }
    }
    // 缓存渲染点供 primitive.hitTest 复用，避免拖拽命中检测时重复计算
    prim._lastRenderPoints = renderPts

    // 当前预览（mode 取全局当前模式）
    let preview = null
    if (prim._p0) {
      const p0Coord = this._pointCoord(ts, series, prim._p0)
      if (p0Coord) {
        if (prim._p5) {
          preview = calcWaveRenderData(ts, series, { p0: prim._p0, p5: prim._p5, mode: prim._mode }, true)
        } else {
          // 仅有 p0：起点标记（方向未知，默认红色）
          preview = {
            color: CLR_RISE,
            startPointOnly: true,
            points: [p0Coord],
          }
        }
      }
    }

    this._renderer.setData({ shapes, preview })
  }

  _pointCoord(ts, series, pt) {
    const x = ts.timeToCoordinate(pt.time)
    const y = series.priceToCoordinate(pt.price)
    if (x == null || y == null) return null
    return { x, y }
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

// ===== WavePrimitive（实现 ISeriesPrimitive）=====

class WavePrimitive {
  constructor() {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._shapes = []   // 已完成波浪 [{p0, p5, mode}]（仅存端点 + 模式）
    this._p0 = null     // 当前波浪起点
    this._p5 = null     // 当前预览终点（predict3 下实际是 p3）
    this._mode = 'impulse5'  // 当前绘制模式（用于预览）
    this._lastRenderPoints = []  // 缓存最近渲染的各 shape 点坐标（媒体坐标），供 hitTest 复用
    this._paneView = new WavePaneView(this)
  }

  // —— 生命周期 ——
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

  // —— 模式 ——
  setMode(mode) {
    this._mode = mode || 'impulse5'
    this._requestRedraw()
  }

  // —— 当前预览 ——
  setP0(pt) {
    this._p0 = pt
    this._requestRedraw()
  }

  setP5(pt) {
    this._p5 = pt
    this._requestRedraw()
  }

  // —— 已完成波浪管理 ——
  addShape(shape) {
    this._shapes.push(shape)
    this._requestRedraw()
  }

  setShapes(shapes) {
    this._shapes = shapes.slice()
    this._requestRedraw()
  }

  removeLastShape() {
    this._shapes.pop()
    this._requestRedraw()
  }

  getShapeCount() {
    return this._shapes.length
  }

  // —— 清除 ——
  clearCurrent() {
    this._p0 = null
    this._p5 = null
    this._requestRedraw()
  }

  clearAll() {
    this._shapes = []
    this._p0 = null
    this._p5 = null
    this._lastRenderPoints = []
    this._requestRedraw()
  }

  clearP5() {
    this._p5 = null
    this._requestRedraw()
  }

  hasP0() { return !!this._p0 }
  hasP5() { return !!this._p5 }

  // —— 拖拽命中检测 ——
  // 命中阈值（逻辑像素）；略大于浪号圆半径（max 9px），与 long 价格线 12px 风格一致
  static get HIT_PX() { return 10 }

  /**
   * 命中检测：返回最近被命中的点 {shapeIndex, pointIdx} 或 null
   * 复用 _lastRenderPoints 缓存（updateAllViews 后由 PaneView 回写），避免重复坐标换算
   * @param {number} mediaX 鼠标媒体坐标 x
   * @param {number} mediaY 鼠标媒体坐标 y
   */
  hitTest(mediaX, mediaY) {
    const pts = this._lastRenderPoints
    if (!pts || pts.length === 0) return null
    const HIT = WavePrimitive.HIT_PX
    let best = null
    let bestDist = HIT + 1
    for (let si = 0; si < pts.length; si++) {
      const p6 = pts[si]
      if (!p6 || p6.length === 0) continue
      // bbox 包围盒短路：鼠标明显不在该波浪附近则跳过
      let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
      for (const p of p6) {
        if (p.x < minX) minX = p.x
        if (p.x > maxX) maxX = p.x
        if (p.y < minY) minY = p.y
        if (p.y > maxY) maxY = p.y
      }
      if (mediaX < minX - HIT || mediaX > maxX + HIT ||
          mediaY < minY - HIT || mediaY > maxY + HIT) continue
      // 6 点距离比较
      for (let pi = 0; pi < p6.length; pi++) {
        const d = Math.hypot(p6[pi].x - mediaX, p6[pi].y - mediaY)
        if (d <= HIT && d < bestDist) {
          bestDist = d
          best = { shapeIndex: si, pointIdx: pi }
        }
      }
    }
    return best
  }

  /**
   * 更新某个波浪的某个点（拖拽调用）
   * - pointIdx 0 → 改 p0；pointIdx===anchorIdx → 改 p5；拖端点清 overrides（中间点重算斐波那契）
   * - 其他 pointIdx → 写入 shape.overrides[pointIdx]（中间点主观微调）
   */
  updatePoint(shapeIndex, pointIdx, pt) {
    const shape = this._shapes[shapeIndex]
    if (!shape) return
    const mode = shape.mode || 'impulse5'
    const anchorIdx = mode === 'predict3' ? 3 : 5
    if (pointIdx === 0) {
      shape.p0 = pt
      shape.overrides = undefined            // 拖端点 → 清中间点覆盖，重算斐波那契
    } else if (pointIdx === anchorIdx) {
      shape.p5 = pt
      shape.overrides = undefined            // predict3 拖 P3 同样清 overrides[4/5]
    } else {
      shape.overrides ||= {}
      shape.overrides[pointIdx] = pt
    }
    this._requestRedraw()
  }

  _requestRedraw() {
    if (this._requestUpdate) {
      try { this._requestUpdate() } catch (e) { /* ignore */ }
    }
  }
}

/**
 * 创建波浪 primitive 并 attach 到指定 series
 * @param {import('lightweight-charts').ISeriesApi} series
 * @returns {WavePrimitive}
 */
export function createWavePrimitive(series) {
  const prim = new WavePrimitive()
  series.attachPrimitive(prim)
  return prim
}

export { WavePrimitive }
