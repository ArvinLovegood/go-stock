import { nextTick, onBeforeUnmount, ref, type Ref } from 'vue'

/**
 * K 线弹窗的统一尺寸与高度自适应。
 *
 * 全站用模态框承载 StockLightweightKlineChart 的地方都对齐「信号监控」抽屉里的那一个：
 * 弹窗宽度吃满视口 92%，图表高度按「实测卡片高度」反推，一次收敛到刚好铺满而不出滚动条。
 */

/** 弹窗宽度：留出两侧可视边距，避免贴边 */
export const KLINE_MODAL_WIDTH = '92vw'

export const KLINE_MODAL_STYLE = {
  width: KLINE_MODAL_WIDTH,
  maxWidth: KLINE_MODAL_WIDTH,
  boxSizing: 'border-box',
}

export const KLINE_MODAL_CONTENT_STYLE = {
  overflow: 'hidden',
  minWidth: 0,
  boxSizing: 'border-box',
}

/** 弹窗卡片占视口高度的比例 */
const KLINE_CARD_BUDGET_RATIO = 0.94
/** 图表高度下限，防止小窗口下把图压没 */
const KLINE_MIN_CHART_PX = 320
/** 图表外还有可滚动内容时，图表按视口高度取比例值 */
const KLINE_SCROLLABLE_RATIO = 0.55
const KLINE_SCROLLABLE_MAX_PX = 700
const KLINE_SCROLLABLE_MIN_PX = 400

interface KlineModalFitOptions {
  /** 首次渲染的高度基准 */
  initialHeight?: number
  /**
   * 弹窗内除图表外还有其它可滚动内容（卡片/表格）时置 true。
   *
   * 这类弹窗的卡片高度会被内容上限（content-style 的 maxHeight）钉住，不再随图表高度变化，
   * 而下面的 fitKlineChart 是「按实测卡片高度反推」的反馈式收敛，前提正是卡片高度跟随图表——
   * 前提不成立时每轮都会算出同一个正增量，图表无限增高且永不收敛。
   * 故这些弹窗改用按视口比例的固定高度，不参与收敛。
   */
  scrollable?: boolean
}

export function useKlineModalFit(wrapRef: Ref<HTMLElement | null>, options: KlineModalFitOptions = {}) {
  const winHeight = ref(typeof window !== 'undefined' ? window.innerHeight : 800)
  const initialHeight = options.initialHeight ?? 420
  const scrollable = options.scrollable === true

  function ratioHeight() {
    return Math.min(KLINE_SCROLLABLE_MAX_PX, Math.max(KLINE_SCROLLABLE_MIN_PX, Math.round(winHeight.value * KLINE_SCROLLABLE_RATIO)))
  }

  const chartHeight = ref(scrollable ? ratioHeight() : Math.max(initialHeight, winHeight.value - 230))
  let observer: ResizeObserver | null = null

  /**
   * 卡片查找：preset="card" 的模态卡片同时带 n-modal 与 n-card class，
   * 而组件内部可能还有自己的 n-card 包裹层，故优先取 n-modal。
   */
  function cardOf(el: HTMLElement | null): HTMLElement | null {
    if (!el || !el.closest) return null
    return (el.closest('.n-modal') ?? el.closest('.n-card')) as HTMLElement | null
  }

  /**
   * 组件内除图表外还有工具条/图例/提示行等固定开销（且会随数据加载变化），
   * 用固定值估算必然对不上——算小了留白、算大了弹窗出现滚动条。
   * 这里按实测卡片高度反推：卡片比预算高就缩、比预算矮就长。
   */
  function fitKlineChart() {
    const card = cardOf(wrapRef.value)
    if (!card) return
    const budget = Math.round(winHeight.value * KLINE_CARD_BUDGET_RATIO)
    const next = Math.max(KLINE_MIN_CHART_PX, chartHeight.value + (budget - card.offsetHeight))
    // 4px 阈值：避免与 ResizeObserver 互相触发形成抖动
    if (Math.abs(next - chartHeight.value) > 4) chartHeight.value = next
  }

  function onWinResize() {
    winHeight.value = window.innerHeight
    if (scrollable) {
      chartHeight.value = ratioHeight()
      return
    }
    fitKlineChart()
  }

  /** 弹窗打开后（内容已渲染）量一次，并对后续尺寸变化保持跟随 */
  async function attach(attempt = 0) {
    await nextTick()
    window.addEventListener('resize', onWinResize)
    if (scrollable) {
      chartHeight.value = ratioHeight()
      return
    }
    const el = wrapRef.value
    // 模态内容是懒渲染的，偶发一帧内还拿不到元素，重试几帧即可
    if (!el) {
      if (attempt < 5) requestAnimationFrame(() => attach(attempt + 1))
      return
    }
    fitKlineChart()
    if (typeof ResizeObserver === 'undefined') return
    if (observer) observer.disconnect()
    // 组件内的工具条/信号汇总会随数据加载变高，观测后再校正一次
    observer = new ResizeObserver(() => fitKlineChart())
    observer.observe(el)
  }

  function detach() {
    window.removeEventListener('resize', onWinResize)
    if (observer) observer.disconnect()
    observer = null
  }

  onBeforeUnmount(detach)

  return { chartHeight, attach, detach, fitKlineChart }
}