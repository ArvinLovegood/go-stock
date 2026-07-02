/**
 * K 线图「画框测量」Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 在主图上画两点矩形框，实时显示涨跌幅% / 价差 / K线数 / 成交量。
 * primitive 为纯视图对象，不持有 mergedRawRows；统计由 Vue 侧计算后通过 setStats 注入。
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
    this._p1 = null        // { x, y } 逻辑像素坐标
    this._p2 = null
    this._color = CLR_RISE
    this._lines = []
    this._labelAnchor = 'right'
  }

  setData({ p1, p2, color, lines, labelAnchor }) {
    this._p1 = p1
    this._p2 = p2
    this._color = color || CLR_RISE
    this._lines = lines || []
    this._labelAnchor = labelAnchor || 'right'
  }

  draw(target) {
    if (!this._p1 || !this._p2) return
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio
      const bitmapW = scope.bitmapSize.width
      const bitmapH = scope.bitmapSize.height

      const x1 = Math.round(this._p1.x * hpr)
      const y1 = Math.round(this._p1.y * vpr)
      const x2 = Math.round(this._p2.x * hpr)
      const y2 = Math.round(this._p2.y * vpr)
      const left = Math.min(x1, x2)
      const top = Math.min(y1, y2)
      const w = Math.max(1, Math.abs(x2 - x1))
      const h = Math.max(1, Math.abs(y2 - y1))

      // 矩形填充（18% 透明度）
      ctx.fillStyle = hexToRgba(this._color, 0.18)
      ctx.fillRect(left, top, w, h)

      // 边框
      ctx.lineWidth = Math.max(1, Math.round(1.5 * Math.min(hpr, vpr)))
      ctx.strokeStyle = this._color
      ctx.strokeRect(left, top, w, h)

      // 文本标签
      this._drawLabel(ctx, left, top, w, h, hpr, vpr, bitmapW, bitmapH)
    })
  }

  _drawLabel(ctx, left, top, w, h, hpr, vpr, bitmapW, bitmapH) {
    const lines = this._lines
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
    if (this._labelAnchor === 'left' || rightEdge + boxW > bitmapW) {
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
    ctx.fillStyle = this._color
    ctx.fillRect(boxX, boxY, Math.max(2, Math.round(colorBarWLogical * hpr)), boxH)

    // 文本
    ctx.fillStyle = this._color
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
    const p1 = prim._p1
    const p2 = prim._p2
    if (!chart || !series || !p1 || !p2) {
      this._renderer.setData({ p1: null, p2: null, lines: [], color: CLR_RISE })
      return
    }
    const ts = chart.timeScale()
    const x1 = ts.timeToCoordinate(p1.time)
    const x2 = ts.timeToCoordinate(p2.time)
    const y1 = series.priceToCoordinate(p1.price)
    const y2 = series.priceToCoordinate(p2.price)
    // 任一端点滑出可见范围则不绘制（滑回自动恢复，同 priceLine 行为）
    if (x1 == null || x2 == null || y1 == null || y2 == null) {
      this._renderer.setData({ p1: null, p2: null, lines: [], color: CLR_RISE })
      return
    }
    const color = p2.price >= p1.price ? CLR_RISE : CLR_FALL
    const lines = buildLabelLines(prim._stats, p1, p2)
    this._renderer.setData({
      p1: { x: x1, y: y1 },
      p2: { x: x2, y: y2 },
      color,
      lines,
      labelAnchor: 'right',
    })
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
    this._p1 = null
    this._p2 = null
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

  // —— 外部 API ——
  setP1(pt) {
    this._p1 = pt
    this._requestRedraw()
  }

  setP2(pt) {
    this._p2 = pt
    this._requestRedraw()
  }

  setPoints(p1, p2) {
    this._p1 = p1
    this._p2 = p2
    this._requestRedraw()
  }

  setStats(stats) {
    this._stats = stats
    this._requestRedraw()
  }

  clear() {
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
