/**
 * K 线图「画框测量」Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 支持画多个矩形框，每个框显示涨跌幅% / 价差 / K线数 / 成交量。
 * 已完成框实线；当前正在画的预览框虚线 + 起点圆点标记。
 * primitive 为纯视图对象，统计由 Vue 侧计算后注入。
 */
import { CLR_RISE, CLR_FALL } from './constants'
import { formatVolumeCn, formatPctField, formatSigned2 } from './format'

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

function buildLabelLines(stats, p1, p2) {
  if (!stats) return []
  const dir = p2.price >= p1.price ? '↑' : '↓'
  return [
    `${dir} 涨跌 ${formatPctField(stats.pct)}`,
    `价差 ${formatSigned2(stats.diff)}`,
    `K线 ${stats.barCount} 根`,
    `量 ${formatVolumeCn(stats.volSum)}`,
  ]
}

// ===== MeasureRenderer（实现 IPrimitivePaneRenderer）=====

class MeasureRenderer {
  constructor() {
    this._shapes = []     // 已完成框 [{p1:{x,y}, p2:{x,y}, color, lines}]
    this._preview = null  // 预览框 {p1:{x,y}, p2:{x,y}, color, lines, hasP2}
  }

  setData({ shapes, preview }) {
    this._shapes = shapes || []
    this._preview = preview || null
  }

  draw(target) {
    if (this._shapes.length === 0 && !this._preview) return
    target.useBitmapCoordinateSpace(scope => {
      for (const s of this._shapes) this._drawShape(scope, s, false)
      if (this._preview) this._drawPreview(scope, this._preview)
    })
  }

  _drawShape(scope, shape, isPreview) {
    const ctx = scope.context
    const hpr = scope.horizontalPixelRatio
    const vpr = scope.verticalPixelRatio
    const bitmapW = scope.bitmapSize.width
    const bitmapH = scope.bitmapSize.height

    const x1 = Math.round(shape.p1.x * hpr)
    const y1 = Math.round(shape.p1.y * vpr)
    const x2 = Math.round(shape.p2.x * hpr)
    const y2 = Math.round(shape.p2.y * vpr)
    const left = Math.min(x1, x2)
    const top = Math.min(y1, y2)
    const w = Math.max(1, Math.abs(x2 - x1))
    const h = Math.max(1, Math.abs(y2 - y1))

    // 矩形填充
    ctx.fillStyle = hexToRgba(shape.color, isPreview ? 0.10 : 0.18)
    ctx.fillRect(left, top, w, h)

    // 边框（预览框虚线）
    ctx.lineWidth = Math.max(1, Math.round(1.5 * Math.min(hpr, vpr)))
    ctx.strokeStyle = shape.color
    if (isPreview) ctx.setLineDash([5 * hpr, 3 * hpr])
    ctx.strokeRect(left, top, w, h)
    ctx.setLineDash([])

    // 文本标签
    this._drawLabel(ctx, left, top, w, h, hpr, vpr, bitmapW, bitmapH, shape.color, shape.lines)
  }

  _drawPreview(scope, preview) {
    // 只有 p1（无 p2）：画起点圆点标记
    if (!preview.hasP2) {
      this._drawPoint(scope, preview.p1, preview.color)
      return
    }
    // 有 p1 + p2：画虚线预览框
    this._drawShape(scope, preview, true)
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

  _drawLabel(ctx, left, top, w, h, hpr, vpr, bitmapW, bitmapH, color, lines) {
    if (!lines || !lines.length) return
    const fontLogical = 11
    const lineHLogical = fontLogical + 4
    const padXLogical = 6
    const padYLogical = 4
    const colorBarWLogical = 3
    const fontStr = `${fontLogical}px -apple-system, "Segoe UI", "Microsoft YaHei", sans-serif`

    ctx.font = fontStr
    let maxW = 0
    for (const ln of lines) {
      const m = ctx.measureText(ln)
      if (m.width > maxW) maxW = m.width
    }
    const boxW = Math.ceil((maxW + padXLogical * 2 + colorBarWLogical) * hpr)
    const boxH = Math.ceil((lineHLogical * lines.length + padYLogical * 2) * vpr)

    // 标签 x：默认贴矩形右侧；右边界出界则翻到左侧
    const rightEdge = left + w
    let boxX
    if (rightEdge + boxW > bitmapW) {
      boxX = Math.max(2, left - boxW)
    } else {
      boxX = rightEdge
    }
    // 标签 y：贴矩形上沿；出界则下移到底部
    let boxY = top
    if (boxY < 0) boxY = top + h
    if (boxY + boxH > bitmapH) boxY = Math.max(0, bitmapH - boxH)

    // 背景
    ctx.fillStyle = 'rgba(20, 20, 23, 0.82)'
    roundRect(ctx, boxX, boxY, boxW, boxH, 4 * Math.min(hpr, vpr))
    ctx.fill()

    // 左侧色条（标识涨跌色）
    ctx.fillStyle = color
    ctx.fillRect(boxX, boxY, Math.max(2, Math.round(colorBarWLogical * hpr)), boxH)

    // 文本
    ctx.fillStyle = color
    ctx.textBaseline = 'top'
    ctx.textAlign = 'left'
    ctx.font = fontStr
    const textX = boxX + Math.round((padXLogical + colorBarWLogical) * hpr)
    for (let i = 0; i < lines.length; i++) {
      const textY = boxY + Math.round((padYLogical + i * lineHLogical) * vpr)
      ctx.fillText(lines[i], textX, textY)
    }
  }
}

// ===== MeasurePaneView（实现 IPrimitivePaneView）=====

class MeasurePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new MeasureRenderer()
  }

  update() {
    const prim = this._primitive
    const chart = prim._chart
    const series = prim._series
    if (!chart || !series) {
      this._renderer.setData({ shapes: [], preview: null })
      return
    }
    const ts = chart.timeScale()

    // 计算已完成框坐标
    const shapes = []
    for (const s of prim._shapes) {
      const coords = this._calcCoords(ts, series, s.p1, s.p2)
      if (!coords) continue
      shapes.push({
        ...coords,
        color: s.p2.price >= s.p1.price ? CLR_RISE : CLR_FALL,
        lines: buildLabelLines(s.stats, s.p1, s.p2),
      })
    }

    // 计算当前预览框
    let preview = null
    if (prim._p1) {
      const p1Coord = this._pointCoord(ts, series, prim._p1)
      if (p1Coord) {
        if (prim._p2) {
          const p2Coord = this._pointCoord(ts, series, prim._p2)
          if (p2Coord) {
            preview = {
              p1: p1Coord,
              p2: p2Coord,
              color: prim._p2.price >= prim._p1.price ? CLR_RISE : CLR_FALL,
              lines: buildLabelLines(prim._stats, prim._p1, prim._p2),
              hasP2: true,
            }
          }
        } else {
          // 只有 p1：画起点标记
          preview = {
            p1: p1Coord,
            p2: p1Coord,
            color: CLR_RISE,
            lines: [],
            hasP2: false,
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

  _calcCoords(ts, series, p1, p2) {
    const c1 = this._pointCoord(ts, series, p1)
    const c2 = this._pointCoord(ts, series, p2)
    if (!c1 || !c2) return null
    return { p1: c1, p2: c2 }
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

// ===== MeasurePrimitive（实现 ISeriesPrimitive）=====

class MeasurePrimitive {
  constructor() {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._shapes = []     // 已完成框 [{p1, p2, stats}]
    this._p1 = null       // 当前正在画的框起点
    this._p2 = null       // 当前正在画的框预览终点
    this._stats = null
    this._paneView = new MeasurePaneView(this)
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

  // —— 当前预览框 ——
  setP1(pt) {
    this._p1 = pt
    this._requestRedraw()
  }

  setP2(pt) {
    this._p2 = pt
    this._requestRedraw()
  }

  setStats(stats) {
    this._stats = stats
    this._requestRedraw()
  }

  // —— 已完成框管理 ——
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
    this._p1 = null
    this._p2 = null
    this._stats = null
    this._requestRedraw()
  }

  clearAll() {
    this._shapes = []
    this._p1 = null
    this._p2 = null
    this._stats = null
    this._requestRedraw()
  }

  clearP2() {
    this._p2 = null
    this._stats = null
    this._requestRedraw()
  }

  hasP1() { return !!this._p1 }
  hasP2() { return !!this._p2 }

  _requestRedraw() {
    if (this._requestUpdate) {
      try { this._requestUpdate() } catch (e) { /* ignore */ }
    }
  }
}

/**
 * 创建测量 primitive 并 attach 到指定 series
 * @param {import('lightweight-charts').ISeriesApi} series
 * @returns {MeasurePrimitive}
 */
export function createMeasurePrimitive(series) {
  const prim = new MeasurePrimitive()
  series.attachPrimitive(prim)
  return prim
}

export { MeasurePrimitive }
