/**
 * K 线图「成交量分布 Volume Profile（可见区间）」Primitive —— lightweight-charts v5 自定义叠加层
 *
 * 仿 TradingView VPVR（Volume Profile Visible Range）：
 * - 直方图从主图右侧向左展开，宽度 = 该价格档成交量占比 × 最大宽度
 * - 档内红（收≥开，买方主导）/ 绿（收<开，卖方主导）左右拼接，符合 A 股红涨绿跌
 * - POC（成交量最大的价格档）：整幅宽度橙色虚线 + 右侧价格标签
 * - Value Area（70% 成交集中区）：VAH/VAL 虚线，区外档位自动降透明度
 * - 每根 K 的成交量按 [low, high] 均匀摊入所跨价格档（标准 VP 算法）
 * - 档数随主图高度自适应；随缩放/拖动按可见区间自动重算
 * - primitive 为纯视图对象，OHLCV 数据由 Vue 侧通过 getter 注入（带版本缓存）
 */
import { CLR_RISE, CLR_FALL } from './constants'
import { formatVolumeCn, formatPrice2 } from './format'

// ===== 可调参数 =====
const PROFILE_WIDTH_RATIO = 0.18 // 直方图最大宽度占绘图区宽度比例
const ROW_HEIGHT_PX = 12         // 每个价格档的目标像素高度
const ROWS_MIN = 14              // 档数下限
const ROWS_MAX = 56              // 档数上限
const VALUE_AREA_RATIO = 0.70    // Value Area 成交量占比（TradingView 默认 70%）
const POC_COLOR = '#f59e0b'      // POC / VAH / VAL 线颜色（琥珀色，明暗主题均可读）
const OUTSIDE_VA_ALPHA = 0.42    // Value Area 之外的档位透明度系数

const FONT_FAMILY = '-apple-system, "Segoe UI", "Microsoft YaHei", sans-serif'

function hexToRgba(hex, alpha) {
  let h = String(hex || '').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const r = parseInt(h.slice(0, 2), 16) || 0
  const g = parseInt(h.slice(2, 4), 16) || 0
  const b = parseInt(h.slice(4, 6), 16) || 0
  return `rgba(${r},${g},${b},${alpha})`
}

/**
 * 计算可见区间的成交量分布
 * @param {{highs:number[],lows:number[],opens:number[],closes:number[],vols:number[]}} bars
 * @param {number} i0 起始索引（含）
 * @param {number} i1 结束索引（含）
 * @param {number} rowCount 价格档数
 */
function computeVolumeProfile(bars, i0, i1, rowCount) {
  let minLow = Infinity
  let maxHigh = -Infinity
  let totalVol = 0
  for (let i = i0; i <= i1; i++) {
    const lo = bars.lows[i]
    const hi = bars.highs[i]
    if (!Number.isFinite(lo) || !Number.isFinite(hi)) continue
    if (lo < minLow) minLow = lo
    if (hi > maxHigh) maxHigh = hi
    totalVol += Number.isFinite(bars.vols[i]) ? bars.vols[i] : 0
  }
  if (!Number.isFinite(minLow) || !Number.isFinite(maxHigh) || totalVol <= 0) {
    return null
  }
  if (maxHigh <= minLow) {
    // 全部同一价格（如一字板）：单档处理
    maxHigh = minLow + Math.max(minLow * 1e-4, 1e-6)
  }
  const bucketH = (maxHigh - minLow) / rowCount
  const buy = new Float64Array(rowCount)
  const sell = new Float64Array(rowCount)
  const total = new Float64Array(rowCount)
  for (let i = i0; i <= i1; i++) {
    const lo = bars.lows[i]
    const hi = bars.highs[i]
    if (!Number.isFinite(lo) || !Number.isFinite(hi)) continue
    const v = Number.isFinite(bars.vols[i]) ? bars.vols[i] : 0
    if (v <= 0) continue
    let b0 = Math.floor((lo - minLow) / bucketH)
    let b1 = Math.floor((hi - minLow) / bucketH)
    if (b0 < 0) b0 = 0
    if (b1 > rowCount - 1) b1 = rowCount - 1
    if (b1 < b0) b1 = b0
    const per = v / (b1 - b0 + 1)
    const isBuy = bars.closes[i] >= bars.opens[i]
    for (let b = b0; b <= b1; b++) {
      total[b] += per
      if (isBuy) buy[b] += per
      else sell[b] += per
    }
  }
  let maxVol = 0
  let pocIdx = -1
  for (let b = 0; b < rowCount; b++) {
    if (total[b] > maxVol) {
      maxVol = total[b]
      pocIdx = b
    }
  }
  if (pocIdx < 0 || maxVol <= 0) return null

  // Value Area：从 POC 出发向两侧扩展，每次取相邻较大档，直到累计 ≥ 70% 总量
  const target = totalVol * VALUE_AREA_RATIO
  let vaVol = total[pocIdx]
  let vaLo = pocIdx
  let vaHi = pocIdx
  while (vaVol < target) {
    const upVol = vaLo > 0 ? total[vaLo - 1] : -1
    const dnVol = vaHi < rowCount - 1 ? total[vaHi + 1] : -1
    if (upVol < 0 && dnVol < 0) break
    if (dnVol >= upVol) {
      vaHi++
      vaVol += dnVol
    } else {
      vaLo--
      vaVol += upVol
    }
  }

  return {
    minLow,
    bucketH,
    buy,
    sell,
    total,
    maxVol,
    totalVol,
    pocIdx,
    vaLo,
    vaHi,
    vah: minLow + (vaHi + 1) * bucketH,
    val: minLow + vaLo * bucketH,
    barCount: i1 - i0 + 1,
  }
}

// ===== VolumeProfileRenderer（实现 IPrimitivePaneRenderer）=====

class VolumeProfileRenderer {
  constructor() {
    this._rows = []
    this._meta = null
  }

  setData({ rows, meta }) {
    this._rows = rows || []
    this._meta = meta || null
  }

  draw(target) {
    if (this._rows.length === 0 || !this._meta) return
    const meta = this._meta
    target.useBitmapCoordinateSpace(scope => {
      const ctx = scope.context
      const hpr = scope.horizontalPixelRatio
      const vpr = scope.verticalPixelRatio
      const bitmapW = scope.bitmapSize.width
      const bitmapH = scope.bitmapSize.height
      const maxW = Math.max(24, Math.round(bitmapW * PROFILE_WIDTH_RATIO))
      const rightX = bitmapW

      // 1. 各价格档直方图（从右向左展开；买方红在右侧、卖方绿在左侧拼接）
      for (const row of this._rows) {
        if (!(row.vol > 0)) continue
        const yTop = Math.round(row.yTop * vpr)
        const yBot = Math.round(row.yBot * vpr)
        const h = Math.max(1, yBot - yTop)
        const wTotal = Math.max(1, Math.round((row.vol / meta.maxVol) * maxW))
        const wBuy = Math.max(0, Math.round((row.buy / meta.maxVol) * maxW))
        const alpha = row.inVA ? 1 : OUTSIDE_VA_ALPHA
        ctx.fillStyle = hexToRgba(CLR_FALL, 0.5 * alpha)
        ctx.fillRect(rightX - wTotal, yTop, wTotal, h)
        ctx.fillStyle = hexToRgba(CLR_RISE, 0.5 * alpha)
        ctx.fillRect(rightX - wBuy, yTop, wBuy, h)
      }

      // 2. Value Area 上/下沿虚线（仅覆盖直方图宽度）
      const lineWidth = Math.max(1, Math.round(1 * Math.min(hpr, vpr)))
      ctx.lineWidth = lineWidth
      ctx.strokeStyle = hexToRgba(POC_COLOR, 0.55)
      ctx.setLineDash([4 * hpr, 3 * hpr])
      for (const y of [meta.vahY, meta.valY]) {
        if (y == null) continue
        const py = Math.round(y * vpr)
        ctx.beginPath()
        ctx.moveTo(rightX - maxW, py)
        ctx.lineTo(rightX, py)
        ctx.stroke()
      }

      // 3. POC 整幅宽度虚线 + 价格标签
      if (meta.pocY != null) {
        const py = Math.round(meta.pocY * vpr)
        ctx.strokeStyle = POC_COLOR
        ctx.lineWidth = Math.max(1, Math.round(1.2 * Math.min(hpr, vpr)))
        ctx.setLineDash([6 * hpr, 4 * hpr])
        ctx.beginPath()
        ctx.moveTo(0, py)
        ctx.lineTo(rightX, py)
        ctx.stroke()
        ctx.setLineDash([])
        this._drawTag(ctx, hpr, vpr, bitmapW, bitmapH, `POC ${formatPrice2(meta.pocPrice)}`, POC_COLOR, py, maxW)
      } else {
        ctx.setLineDash([])
      }

      // 4. 右上角信息标签（可见区间根数 + 总量）
      const info = `VPVR ${meta.barCount}根 总量${formatVolumeCn(meta.totalVol)}`
      this._drawTag(ctx, hpr, vpr, bitmapW, bitmapH, info, hexToRgba('#e2e8f0', 0.9), 6, 0, true)
    })
  }

  /** 右侧小标签：深色圆角底 + 彩色文字；y=0 时贴顶（信息标签） */
  _drawTag(ctx, hpr, vpr, bitmapW, bitmapH, text, color, py, maxW, isTop) {
    if (!text) return
    const fontLogical = 10
    const fontStr = `${fontLogical}px ${FONT_FAMILY}`
    ctx.font = fontStr
    const m = ctx.measureText(text)
    const padX = 4, padY = 2
    const w = Math.ceil((m.width + padX * 2) * hpr)
    const h = Math.ceil((fontLogical + padY * 2) * vpr)
    let bx = Math.max(2, bitmapW - Math.round(maxW * 0.5) - w)
    if (isTop) bx = Math.max(2, bitmapW - w - 4 * hpr)
    let by = Math.round(py * vpr) - h
    if (by < 2) by = Math.round(py * vpr) + 2
    if (by + h > bitmapH - 2) by = bitmapH - h - 2
    ctx.fillStyle = 'rgba(20, 20, 23, 0.8)'
    ctx.beginPath()
    const rr = Math.min(3 * Math.min(hpr, vpr), w / 2, h / 2)
    ctx.moveTo(bx + rr, by)
    ctx.arcTo(bx + w, by, bx + w, by + h, rr)
    ctx.arcTo(bx + w, by + h, bx, by + h, rr)
    ctx.arcTo(bx, by + h, bx, by, rr)
    ctx.arcTo(bx, by, bx + w, by, rr)
    ctx.closePath()
    ctx.fill()
    ctx.fillStyle = color
    ctx.textBaseline = 'middle'
    ctx.textAlign = 'left'
    ctx.font = fontStr
    ctx.fillText(text, bx + Math.round(padX * hpr), by + Math.round(h / 2))
  }
}

// ===== VolumeProfilePaneView（实现 IPrimitivePaneView）=====

class VolumeProfilePaneView {
  constructor(primitive) {
    this._primitive = primitive
    this._renderer = new VolumeProfileRenderer()
  }

  update() {
    const prim = this._primitive
    const chart = prim._chart
    const series = prim._series
    if (!chart || !series) {
      this._renderer.setData({ rows: [], meta: null })
      return
    }
    const bars = prim._getBars ? prim._getBars() : null
    if (!bars || !bars.times || bars.times.length === 0) {
      this._renderer.setData({ rows: [], meta: null })
      return
    }
    const ts = chart.timeScale()
    const range = ts.getVisibleLogicalRange()
    if (!range || range.to <= range.from) {
      this._renderer.setData({ rows: [], meta: null })
      return
    }
    const len = bars.times.length
    const i0 = Math.max(0, Math.ceil(range.from))
    const i1 = Math.min(len - 1, Math.floor(range.to))
    if (i1 < i0) {
      this._renderer.setData({ rows: [], meta: null })
      return
    }

    // 档数随主图高度自适应
    let rowCount = ROWS_MIN
    try {
      const paneH = chart.panes()[prim._paneIndex]?.getHeight?.() || 0
      if (paneH > 0) rowCount = Math.max(ROWS_MIN, Math.min(ROWS_MAX, Math.round(paneH / ROW_HEIGHT_PX)))
    } catch { /* ignore */ }

    const prof = computeVolumeProfile(bars, i0, i1, rowCount)
    if (!prof) {
      this._renderer.setData({ rows: [], meta: null })
      return
    }

    // 价格 → 媒体坐标（渲染时再乘像素比）
    const rows = []
    for (let b = 0; b < rowCount; b++) {
      const priceHigh = prof.minLow + (b + 1) * prof.bucketH
      const priceLow = prof.minLow + b * prof.bucketH
      const yTop = series.priceToCoordinate(priceHigh)
      const yBot = series.priceToCoordinate(priceLow)
      if (yTop == null || yBot == null) continue
      rows.push({
        yTop: Math.min(yTop, yBot),
        yBot: Math.max(yTop, yBot),
        vol: prof.total[b],
        buy: prof.buy[b],
        inVA: b >= prof.vaLo && b <= prof.vaHi,
      })
    }
    const pocPrice = prof.minLow + (prof.pocIdx + 0.5) * prof.bucketH
    const pocY = series.priceToCoordinate(pocPrice)
    const vahY = series.priceToCoordinate(prof.vah)
    const valY = series.priceToCoordinate(prof.val)
    this._renderer.setData({
      rows,
      meta: {
        maxVol: prof.maxVol,
        totalVol: prof.totalVol,
        barCount: prof.barCount,
        pocPrice,
        pocY: pocY == null ? null : pocY,
        vahY: vahY == null ? null : vahY,
        valY: valY == null ? null : valY,
      },
    })
  }

  renderer() {
    return this._renderer
  }

  zOrder() {
    return 'top'
  }
}

// ===== VolumeProfilePrimitive（实现 ISeriesPrimitive）=====

class VolumeProfilePrimitive {
  /**
   * @param {() => {times:number[],opens:number[],highs:number[],lows:number[],closes:number[],vols:number[]}|null} getBars
   *        返回当前全部 OHLCV（带版本缓存），primitive 每次 update 按可见区间切片
   */
  constructor(getBars) {
    this._chart = null
    this._series = null
    this._requestUpdate = null
    this._getBars = getBars
    this._paneIndex = 0
    this._paneView = new VolumeProfilePaneView(this)
  }

  // —— 生命周期 ——
  attached(param) {
    this._chart = param.chart
    this._series = param.series
    this._requestUpdate = param.requestUpdate
    this._paneIndex = 0
    try {
      const panes = param.chart.panes()
      for (let p = 0; p < panes.length; p++) {
        if (panes[p].getSeries().includes(param.series)) {
          this._paneIndex = p
          break
        }
      }
    } catch { /* ignore */ }
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

  /** 数据变更后请求重绘（切换股票/周期/新K线到达时由 Vue 侧调用） */
  requestRedraw() {
    if (this._requestUpdate) {
      try { this._requestUpdate() } catch (e) { /* ignore */ }
    }
  }
}

/**
 * 创建成交量分布 primitive 并 attach 到指定 series
 * @param {import('lightweight-charts').ISeriesApi} series
 * @param {() => object|null} getBars OHLCV getter（Vue 侧带缓存）
 * @returns {VolumeProfilePrimitive}
 */
export function createVolumeProfilePrimitive(series, getBars) {
  const prim = new VolumeProfilePrimitive(getBars)
  series.attachPrimitive(prim)
  return prim
}

export { VolumeProfilePrimitive }
