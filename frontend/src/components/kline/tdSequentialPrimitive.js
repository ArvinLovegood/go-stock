/**
 * K 线图「TD Sequential 神奇九转」数字标记 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 在主图每根 K 线的上方/下方绘制计数数字（1~9）：
 * - 卖出计数（closes[i] < closes[i-4] 连续）：标在 K 线上方，红色系；数到 9 = 潜在顶部反转
 * - 买入计数（closes[i] > closes[i-4] 连续）：标在 K 线下方，绿色系；数到 9 = 潜在底部反转
 * - 1~8 用小号暗色，9 用大号高亮 + 圆角徽章（与东财/同花顺九转效果一致）
 * - primitive 纯视图对象，计数数组由 Vue 侧注入（getter 带版本缓存）
 */
import { CLR_RISE, CLR_FALL } from './constants'

const FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'
const FONT_SIZE = 10
const FONT_SIZE_9 = 12

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

class TDSequentialRenderer {
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
      for (const m of this._marks) {
        const fontLogical = (m.num === 9 ? FONT_SIZE_9 : FONT_SIZE) * vpr
        const fontStr = `${fontLogical}px ${FONT_FAMILY}`
        ctx.font = fontStr
        const text = String(m.num)
        const tm = ctx.measureText(text)
        const w = tm.width
        const h = fontLogical
        const cx = m.x * hpr
        // 数字左上角：上方标记画在 K 线高点之上，下方标记画在低点之下
        const padY = 5 * vpr
        const by = m.above ? m.y * vpr - padY - h : m.y * vpr + padY
        const px = cx - w / 2

        if (m.num === 9) {
          // 9：高亮圆角徽章（红涨绿跌：卖出计数=红色徽章，买入计数=绿色徽章）
          const padX = 3 * hpr
          const bx = px - padX
          const bw = w + padX * 2
          const bh = h + 2 * vpr
          ctx.fillStyle = m.sell ? hexToRgba(CLR_RISE, 0.85) : hexToRgba(CLR_FALL, 0.85)
          ctx.beginPath()
          const rr = Math.min(3 * Math.min(hpr, vpr), bw / 2, bh / 2)
          ctx.moveTo(bx + rr, by)
          ctx.arcTo(bx + bw, by, bx + bw, by + bh, rr)
          ctx.arcTo(bx + bw, by + bh, bx, by + bh, rr)
          ctx.arcTo(bx, by + bh, bx, by, rr)
          ctx.arcTo(bx, by, bx + bw, by, rr)
          ctx.closePath()
          ctx.fill()
          ctx.fillStyle = '#ffffff'
        } else {
          ctx.fillStyle = m.sell ? hexToRgba(CLR_RISE, 0.65) : hexToRgba(CLR_FALL, 0.65)
        }

        ctx.textBaseline = 'top'
        ctx.textAlign = 'left'
        ctx.fillText(text, px, by)
      }
    })
  }
}

class TDSequentialPaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new TDSequentialRenderer()
  }

  update() {
    const prim = this._primitive
    const series = prim._series
    if (!series) {
      this._renderer.setData([])
      return
    }
    const counts = prim._getCounts ? prim._getCounts() : null
    const bars = prim._getBars ? prim._getBars() : null
    if (!counts || !bars || !bars.times) {
      this._renderer.setData([])
      return
    }

    const ts = prim._chart?.timeScale()
    if (!ts) {
      this._renderer.setData([])
      return
    }

    const marks = []
    const len = bars.times.length
    for (let i = 0; i < len; i++) {
      const s = counts.sell[i] || 0
      const b = counts.buy[i] || 0
      if (s <= 0 && b <= 0) continue
      const time = bars.times[i]
      const x = ts.timeToCoordinate(time)
      if (x == null) continue
      if (s > 0) {
        const y = series.priceToCoordinate(bars.highs[i])
        if (y != null) marks.push({ x, y, num: s, sell: true, above: true })
      }
      if (b > 0) {
        const y = series.priceToCoordinate(bars.lows[i])
        if (y != null) marks.push({ x, y, num: b, sell: false, above: false })
      }
    }
    this._renderer.setData(marks)
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

class TDSequentialPrimitive {
  /**
   * @param {() => {times:number[],highs:number[],lows:number[]}|null} getBars
   * @param {() => {sell:number[],buy:number[]}|null} getCounts
   */
  constructor(getBars, getCounts) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getBars = getBars
    this._getCounts = getCounts
    this._paneView = new TDSequentialPaneView(this)
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

export function createTDSequentialPrimitive(series, getBars, getCounts) {
  const prim = new TDSequentialPrimitive(getBars, getCounts)
  series.attachPrimitive(prim)
  return prim
}

export { TDSequentialPrimitive }
