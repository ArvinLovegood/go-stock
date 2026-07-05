/**
 * lightweight-charts-drawing 插件宿主封装
 *
 * 架构澄清（与早期注释不同）：
 *   - DrawingManager 仅管理 drawing 生命周期/选中/拖拽编辑，setActiveTool 只是存字段，
 *     handleClick 在有 activeTool 时【什么都不做】。插件不内置交互式创建。
 *   - 交互创建需自己挂 chart.subscribeClick 收集 anchor，够数后用
 *     getToolRegistry().get(type).factory(id, anchors) 创建 Drawing 再 addDrawing。
 *
 * 本 host 职责：封装 DrawingManager + 工具配置表 + 自实现的 click 收集，
 * 对 Vue 侧提供精简接口。与现有 wave/measure primitive 共存（互斥锁协调）。
 */
import { DrawingManager, getToolRegistry } from 'lightweight-charts-drawing'

/**
 * 首批集成工具配置表
 * @property {string} key      内部标识（Vue 侧 setActiveTool 用）
 * @property {string} label    按钮文案
 * @property {string} type     插件 toolType（registry.get 用，与 Drawing 子类 readonly type 一致）
 * @property {number} anchors  所需锚点数（UI 提示用，与 registry requiredAnchors 一致）
 * @property {string} group    分组（线条/斐波那契/预测仓位）
 */
export const DRAWING_TOOLS = [
  // 线条类（5）
  { key: 'trendline', label: '趋势线', type: 'trend-line', anchors: 2, group: '线条' },
  { key: 'ray', label: '射线', type: 'ray', anchors: 2, group: '线条' },
  { key: 'hline', label: '水平线', type: 'horizontal-line', anchors: 1, group: '线条' },
  { key: 'vline', label: '垂直线', type: 'vertical-line', anchors: 1, group: '线条' },
  { key: 'crossline', label: '十字线', type: 'cross-line', anchors: 1, group: '线条' },
  // 斐波那契类（4）
  { key: 'fib-retr', label: 'Fib回撤', type: 'fib-retracement', anchors: 2, group: '斐波那契' },
  { key: 'fib-ext', label: 'Fib扩展', type: 'fib-extension', anchors: 3, group: '斐波那契' },
  { key: 'fib-chan', label: 'Fib通道', type: 'fib-channel', anchors: 3, group: '斐波那契' },
  { key: 'fib-circ', label: 'Fib圆', type: 'fib-circles', anchors: 2, group: '斐波那契' },
  // 预测仓位类（4）
  { key: 'long-pos', label: '多仓图示', type: 'long-position', anchors: 3, group: '预测仓位' },
  { key: 'short-pos', label: '空仓图示', type: 'short-position', anchors: 3, group: '预测仓位' },
  { key: 'forecast', label: '预测', type: 'forecast', anchors: 2, group: '预测仓位' },
  { key: 'gann-fan', label: '甘恩扇', type: 'gann-fan', anchors: 2, group: '预测仓位' },
]

// key → 配置 映射（快速查找）
const TOOL_BY_KEY = new Map(DRAWING_TOOLS.map(t => [t.key, t]))

/**
 * 创建 drawing host（封装 DrawingManager + 交互式 anchor 收集）
 * @param {{chart: object, series: object, container: HTMLElement}} opts
 * @param {{onProgress?: ({collected:number, required:number}) => void, onComplete?: () => void}} [cbs]
 * @returns {object} host
 */
export function createDrawingHost({ chart, series, container }, cbs = {}) {
  const manager = new DrawingManager()
  manager.attach(chart, series, container)
  const registry = getToolRegistry()

  // 收集事件订阅句柄，detach 时统一清理
  const unsubscribers = []

  let currentKey = null        // 当前工具 key（Vue 侧标识）
  let currentType = null       // 当前工具 type（registry key）
  let collectedAnchors = []    // 正在收集的锚点
  let idCounter = 0

  /**
   * chart.subscribeClick 回调：收集 anchor，够数后 factory 创建 drawing
   * 注意：manager.attach 已自己挂了一个 handleClick（选中用），这里再挂一个收集用，
   *       两个订阅者都会收到 click，互不干扰。
   */
  const clickHandler = (param) => {
    if (!currentType) return
    if (!param || !param.point || param.time === undefined) return
    const entry = registry.get(currentType)
    if (!entry) {
      console.warn('[drawing] tool not in registry:', currentType)
      return
    }
    const price = series.coordinateToPrice(param.point.y)
    if (price == null) return
    collectedAnchors.push({ time: param.time, price })
    const required = entry.requiredAnchors || (TOOL_BY_KEY.get(currentKey)?.anchors ?? 2)
    if (collectedAnchors.length < required) {
      // 还需继续收集
      cbs.onProgress?.({ collected: collectedAnchors.length, required })
    } else {
      // 够数 → 创建 drawing
      idCounter += 1
      const id = `drawing-${Date.now()}-${idCounter}`
      try {
        const drawing = entry.factory(id, collectedAnchors.slice(), entry.defaultStyle, entry.defaultOptions)
        manager.addDrawing(drawing)
      } catch (e) {
        console.error('[drawing] factory/addDrawing failed:', e)
      }
      collectedAnchors = []
      cbs.onComplete?.()
    }
  }
  chart.subscribeClick(clickHandler)

  return {
    /** DrawingManager 实例（供 Vue 侧直接读 getAllDrawings 等） */
    manager,

    /**
     * 设置当前激活工具（开始交互收集 anchor）
     * @param {string|null} toolKey DRAWING_TOOLS.key 或 null（取消）
     */
    setActiveTool(toolKey) {
      if (toolKey == null) {
        this.cancelActive()
        return
      }
      const tool = TOOL_BY_KEY.get(toolKey)
      if (!tool) return
      const entry = registry.get(tool.type)
      if (!entry) {
        console.warn('[drawing] tool not in registry:', tool.type)
        return
      }
      currentKey = toolKey
      currentType = tool.type
      collectedAnchors = []
      // 不调 manager.setActiveTool（插件实现仅存字段，无交互效果，且会干扰选中逻辑）
    },

    /** 取消当前绘制（清空已收集 anchor，不删已完成 drawing） */
    cancelActive() {
      currentKey = null
      currentType = null
      collectedAnchors = []
    },

    /** 撤销最后一个 drawing */
    undoLast() {
      const all = manager.getAllDrawings()
      if (all.length > 0) {
        manager.removeDrawing(all[all.length - 1].id)
      }
    },

    /** 清空所有 drawings */
    clearAll() {
      manager.clearAll()
    },

    /**
     * 订阅事件（包装 manager.on，自动收集 unsubscribe）
     * @returns {() => void} 取消订阅函数
     */
    on(event, cb) {
      const unsub = manager.on(event, cb)
      unsubscribers.push(unsub)
      return unsub
    },

    /** 当前激活工具 key（null=未激活） */
    getActiveToolKey() {
      return currentKey
    },

    /** 已完成 drawing 数 */
    getDrawingsCount() {
      return manager.getAllDrawings().length
    },

    /** 分离 manager（组件卸载/图表重建时调用） */
    detach() {
      try { chart.unsubscribeClick(clickHandler) } catch (e) { /* ignore */ }
      unsubscribers.splice(0).forEach(unsub => {
        try { unsub() } catch (e) { /* ignore */ }
      })
      try { manager.detach() } catch (e) { /* ignore */ }
    },
  }
}
