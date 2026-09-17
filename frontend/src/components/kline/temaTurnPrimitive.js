/**
 * K 线图「TEMA 斜率转折」标记 Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 与 9 路共振体系（buySellPrimitive.js）完全独立、可同时开启：
 * - 预警（pred）：空心三角 + 「T预」标签 —— TEMA 斜率拐头但尚未过零轴，负边际信号，仅提示关注、不可单独操作
 * - 温和确认（conf）：实心三角 + 「T确」标签 —— 斜率温和穿越零轴（实测 买 +0.56%/卖 +0.60%），渐进趋势启动
 * - 买向急速穿越（impulse+buy）：实心三角+外圈描边 + 「T强」标签 —— 确认时已见同向急变（变化率超 50% 且有规模），
 *   实测强于温和确认（买 +1.01% vs +0.56%，两段同号）——「买要急」：突破爆发、趋势动能强
 * - 卖向急速穿越（impulse+sell）：半透明三角 + 「T急」标签 —— 急速下穿多为脉冲杀跌、易反弹，
 *   实测弱于温和确认（卖 −0.09% vs +0.60%）——「卖要稳」：警示勿追卖
 * - 买向红（CLR_RISE）、卖向绿（CLR_FALL），与 A 股红涨绿跌一致
 * - 与 9 路箭头错位绘制（GAP 更大），两套体系同时开启时上下错开、不互相遮挡；不加光晕
 * - primitive 纯视图对象，信号点由 Vue 侧注入（getter 带版本缓存）
 */
import { CLR_RISE, CLR_FALL } from './constants'

const FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'
const FONT_SIZE = 10
const HALF_W = 6
const HEIGHT = 10
// 比买卖点箭头（GAP=4）更远离 K 线，两套体系同时开启时上下错开、不互相遮挡
const GAP = 18

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

class TemaTurnRenderer {
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
        const color = m.buy ? CLR_RISE : CLR_FALL
        const x = m.x * hpr
        const y = m.y * vpr
        const halfW = HALF_W * hpr
        const hgt = HEIGHT * vpr
        const gap = GAP * vpr
        // 买向箭头尖在上方（指向最低价），卖向箭头尖在下方（指向最高价）
        const apex = m.buy ? y + gap : y - gap
        const base = m.buy ? apex + hgt : apex - hgt

        ctx.beginPath()
        ctx.moveTo(x, apex)
        ctx.lineTo(x - halfW, base)
        ctx.lineTo(x + halfW, base)
        ctx.closePath()
        if (m.pred) {
          // 预警：空心三角（仅描边）
          ctx.lineWidth = 1.5 * vpr
          ctx.strokeStyle = hexToRgba(color, 0.9)
          ctx.stroke()
        } else if (m.impulse && m.buy) {
          // 买向急速穿越=强信号（T强）：实心三角 + 外圈描边（实测 买+1.01% vs 温和+0.56%，买要急）
          ctx.fillStyle = hexToRgba(color, 0.9)
          ctx.fill()
          ctx.lineWidth = 1.5 * vpr
          ctx.strokeStyle = hexToRgba(color, 1)
          ctx.stroke()
        } else if (m.impulse) {
          // 卖向急速穿越=警示（T急）：半透明三角（实测 卖−0.09% vs 温和+0.60%，卖要稳，脉冲杀跌易反弹）
          ctx.fillStyle = hexToRgba(color, 0.5)
          ctx.fill()
        } else {
          // 温和确认：实心三角
          ctx.fillStyle = hexToRgba(color, 0.9)
          ctx.fill()
        }

        const fontLogical = FONT_SIZE * vpr
        ctx.font = `${fontLogical}px ${FONT_FAMILY}`
        const text = 'T' + (m.pred ? '预' : m.impulse ? (m.buy ? '强' : '急') : '确')
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

class TemaTurnPaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new TemaTurnRenderer()
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
    const collect = (list, buy) => {
      for (const p of list || []) {
        const t = bars.times[p.i]
        if (t == null) continue
        const x = ts.timeToCoordinate(t)
        const y = series.priceToCoordinate(p.price)
        if (x == null || y == null) continue
        marks.push({ x, y, buy, pred: p.kind === 'pred', impulse: p.kind === 'impulse' })
      }
    }
    collect(data.buys, true)
    collect(data.sells, false)

    this._renderer.setData(marks)
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

class TemaTurnPrimitive {
  /**
   * @param {() => {times:number[],highs:number[],lows:number[]}|null} getBars
   * @param {() => {buys:Array,sells:Array}|null} getData TEMA 转折点检测结果，每项 { i, price, kind }，
   *        kind: 'pred'（预警）| 'conf'（温和确认）| 'impulse'（急速穿越·警示）
   */
  constructor(getBars, getData) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getBars = getBars
    this._getData = getData
    this._paneView = new TemaTurnPaneView(this)
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

export function createTemaTurnPrimitive(series, getBars, getData) {
  const prim = new TemaTurnPrimitive(getBars, getData)
  series.attachPrimitive(prim)
  return prim
}

export { TemaTurnPrimitive }
