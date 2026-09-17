/**
 * K 线图「买卖点预测」箭头标记 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 在主图绘制共振信号箭头（红买绿卖，与 A 股红涨绿跌一致）：
 * - 买点：红色实心三角，箭头朝上指向该根 K 线最低价（下方），标注 B + 共振分百分比
 * - 卖点：绿色实心三角，箭头朝下指向该根 K 线最高价（上方），标注 S + 共振分百分比
 * - 最新一个信号额外加一圈光晕，突出「当前最近一次可操作信号」
 * - primitive 纯视图对象，信号点由 Vue 侧注入（getter 带版本缓存）
 */
import { CLR_RISE, CLR_FALL } from './constants'
import { BUY_SELL_MAX_SCORE } from './calc'

const FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'
const FONT_SIZE = 10
const HALF_W = 6
const HEIGHT = 10
const GAP = 4
const HALO_R = 11

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

class BuySellRenderer {
  constructor() {
    this._marks = []
  }

  setData(marks) {
    this._marks = marks || []
  }

  draw(target) {
    if (this._marks.length === 0) return
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio
      const pr = Math.min(hpr, vpr)

      for (const m of this._marks) {
        const color = m.buy ? CLR_RISE : CLR_FALL
        const x = m.x * hpr
        const y = m.y * vpr
        const halfW = HALF_W * hpr
        const hgt = HEIGHT * vpr
        const gap = GAP * vpr
        // 买点箭头尖在上方（指向最低价），卖点箭头尖在下方（指向最高价）
        const apex = m.buy ? y + gap : y - gap
        const base = m.buy ? apex + hgt : apex - hgt

        // 最新信号：外圈光晕
        if (m.latest) {
          ctx.beginPath()
          ctx.arc(x, apex, HALO_R * pr, 0, Math.PI * 2)
          ctx.fillStyle = hexToRgba(color, 0.16)
          ctx.fill()
        }

        ctx.beginPath()
        ctx.moveTo(x, apex)
        ctx.lineTo(x - halfW, base)
        ctx.lineTo(x + halfW, base)
        ctx.closePath()
        ctx.fillStyle = hexToRgba(color, m.latest ? 1 : 0.9)
        ctx.fill()

        const fontLogical = FONT_SIZE * vpr
        ctx.font = `${fontLogical}px ${FONT_FAMILY}`
        const text = (m.buy ? 'B' : 'S') + Math.round((m.score / BUY_SELL_MAX_SCORE) * 100) + '%'
        const w = ctx.measureText(text).width
        const padY = 1 * vpr
        const ty = m.buy ? base + padY : base - padY - fontLogical
        ctx.fillStyle = hexToRgba(color, 0.95)
        ctx.textBaseline = 'top'
        ctx.textAlign = 'left'
        ctx.fillText(text, x - w / 2, ty)
      }
    })
  }
}

class BuySellPaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new BuySellRenderer()
  }

  update() {
    const prim = this._primitive
    const series = prim._series
    if (!series) {
      this._renderer.setData([])
      return
    }
    const data = prim._getData ? prim._getData() : null
    const bars = prim._getBars ? prim._getBars() : null
    if (!data || !bars || !bars.times) {
      this._renderer.setData([])
      return
    }
    const ts = prim._chart?.timeScale()
    if (!ts) {
      this._renderer.setData([])
      return
    }

    const marks = []
    let latestIdx = -1
    const collect = (list, buy) => {
      for (const p of list || []) {
        const t = bars.times[p.i]
        if (t == null) continue
        const x = ts.timeToCoordinate(t)
        const y = series.priceToCoordinate(p.price)
        if (x == null || y == null) continue
        marks.push({ x, y, buy, score: p.score, latest: false, idx: p.i })
        if (p.i > latestIdx) latestIdx = p.i
      }
    }
    collect(data.buys, true)
    collect(data.sells, false)
    for (const m of marks) m.latest = m.idx === latestIdx

    this._renderer.setData(marks)
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

class BuySellPrimitive {
  /**
   * @param {() => {times:number[],highs:number[],lows:number[]}|null} getBars
   * @param {() => {buys:Array,sells:Array}|null} getData 买卖点检测结果
   */
  constructor(getBars, getData) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getBars = getBars
    this._getData = getData
    this._paneView = new BuySellPaneView(this)
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

export function createBuySellPrimitive(series, getBars, getData) {
  const prim = new BuySellPrimitive(getBars, getData)
  series.attachPrimitive(prim)
  return prim
}

export { BuySellPrimitive }