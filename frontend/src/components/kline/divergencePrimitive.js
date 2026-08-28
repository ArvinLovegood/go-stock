/**
 * K 线图「自动背离 Divergence」标记 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 在主图绘制背离连线与徽章（仿 TradingView Divergence Indicator）：
 * - 顶背离（Bearish）：价格高点抬高 + 指标值降低 → 红色连线 + 「顶背离」红徽章（画在第二个高点上方）
 * - 底背离（Bullish）：价格低点降低 + 指标值抬高 → 绿色连线 + 「底背离」绿徽章（画在第二个低点下方）
 * - 连线连接两个价格 pivot 点；指标侧连线由 Vue 层在对应副图 series 上另行绘制（本 primitive 只管主图）
 * - primitive 纯视图对象，背离检测结果由 Vue 侧注入（getter 带版本缓存）
 */
import { CLR_RISE, CLR_FALL } from './constants'

const FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'
const FONT_SIZE = 11
const LINE_W = 1.5

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

class DivergenceRenderer {
  constructor() {
    this._lines = []
    this._badges = []
  }

  setData({ lines, badges }) {
    this._lines = lines || []
    this._badges = badges || []
  }

  draw(target) {
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio

      // 1. 背离连线（虚线）
      if (this._lines.length > 0) {
        ctx.lineWidth = Math.max(1, Math.round(LINE_W * Math.min(hpr, vpr)))
        for (const ln of this._lines) {
          ctx.strokeStyle = ln.bearish ? hexToRgba(CLR_RISE, 0.9) : hexToRgba(CLR_FALL, 0.9)
          ctx.setLineDash([5 * hpr, 3 * hpr])
          ctx.beginPath()
          ctx.moveTo(ln.x1 * hpr, ln.y1 * vpr)
          ctx.lineTo(ln.x2 * hpr, ln.y2 * vpr)
          ctx.stroke()
        }
        ctx.setLineDash([])
      }

      // 2. 端点圆点 + 徽章
      for (const b of this._badges) {
        ctx.fillStyle = b.bearish ? hexToRgba(CLR_RISE, 0.95) : hexToRgba(CLR_FALL, 0.95)
        ctx.beginPath()
        ctx.arc(b.px * hpr, b.py * vpr, 3 * Math.min(hpr, vpr), 0, Math.PI * 2)
        ctx.fill()
      }
      for (const b of this._badges) {
        if (!b.text) continue
        const fontLogical = FONT_SIZE * vpr
        const fontStr = `${fontLogical}px ${FONT_FAMILY}`
        ctx.font = fontStr
        const tm = ctx.measureText(b.text)
        const w = tm.width
        const h = fontLogical
        // 徽章画在第二个 pivot 点外侧（顶背离在上方，底背离在下方）
        const padY = 10 * vpr
        const by = b.above ? b.py * vpr - padY - h : b.py * vpr + padY
        const bx = b.px * hpr - w / 2
        const padX = 3 * hpr
        const rx = bx - padX
        const rw = w + padX * 2
        const rh = h + 2 * vpr
        ctx.fillStyle = b.bearish ? hexToRgba(CLR_RISE, 0.85) : hexToRgba(CLR_FALL, 0.85)
        ctx.beginPath()
        const rr = Math.min(3 * Math.min(hpr, vpr), rw / 2, rh / 2)
        ctx.moveTo(rx + rr, by)
        ctx.arcTo(rx + rw, by, rx + rw, by + rh, rr)
        ctx.arcTo(rx + rw, by + rh, rx, by + rh, rr)
        ctx.arcTo(rx, by + rh, rx, by, rr)
        ctx.arcTo(rx, by, rx + rw, by, rr)
        ctx.closePath()
        ctx.fill()
        ctx.fillStyle = '#ffffff'
        ctx.textBaseline = 'top'
        ctx.textAlign = 'left'
        ctx.font = fontStr
        ctx.fillText(b.text, bx, by + 1 * vpr)
      }
    })
  }
}

class DivergencePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new DivergenceRenderer()
  }

  update() {
    const prim = this._primitive
    const series = prim._series
    if (!series) {
      this._renderer.setData({ lines: [], badges: [] })
      return
    }
    const data = prim._getData ? prim._getData() : null
    const bars = prim._getBars ? prim._getBars() : null
    if (!data || !bars || !bars.times) {
      this._renderer.setData({ lines: [], badges: [] })
      return
    }
    const ts = prim._chart?.timeScale()
    if (!ts) {
      this._renderer.setData({ lines: [], badges: [] })
      return
    }

    const lines = []
    const dots = []
    const badges = []
    const mk = (d, bearish) => {
      const x1 = ts.timeToCoordinate(bars.times[d.i1])
      const x2 = ts.timeToCoordinate(bars.times[d.i2])
      const y1 = series.priceToCoordinate(d.p1)
      const y2 = series.priceToCoordinate(d.p2)
      if (x1 == null || x2 == null || y1 == null || y2 == null) return
      lines.push({ x1, y1, x2, y2, bearish })
      dots.push({ px: x1, py: y1, bearish })
      dots.push({ px: x2, py: y2, bearish })
      // 徽章只画在第二个端点（背离确认点）
      badges.push({ px: x2, py: y2, bearish, above: bearish, text: bearish ? '顶背离' : '底背离' })
    }
    for (const d of data.bearish || []) mk(d, true)
    for (const d of data.bullish || []) mk(d, false)
    this._renderer.setData({ lines, badges: [...dots, ...badges.filter(b => b.text)] })
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

class DivergencePrimitive {
  /**
   * @param {() => {times:number[]}|null} getBars
   * @param {() => {bearish:[],bullish:[]}|null} getData 背离检测结果（divergenceValues 输出）
   */
  constructor(getBars, getData) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getBars = getBars
    this._getData = getData
    this._paneView = new DivergencePaneView(this)
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

export function createDivergencePrimitive(series, getBars, getData) {
  const prim = new DivergencePrimitive(getBars, getData)
  series.attachPrimitive(prim)
  return prim
}

export { DivergencePrimitive }
