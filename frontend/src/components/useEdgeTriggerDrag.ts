import { onBeforeUnmount, onMounted, ref, type CSSProperties } from 'vue'

/** 悬浮圆球直径（需与各组件 .edge-trigger-floating 的宽高保持一致） */
const FLOAT_SIZE = 48
/** 吸附阈值：松手时距左/右边缘小于该值则贴边吸附 */
const SNAP = 28
/** 位移超过该像素才判定为拖动（区分普通点击） */
const MOVE_THRESHOLD = 5

export interface EdgeTriggerDragOptions {
  /** localStorage 键：两组件各存一份，互不影响 */
  storageKey: string
  /** 默认贴边侧（原型竖条在右边缘） */
  defaultSide?: 'left' | 'right'
  /** 默认停靠中心 y = innerHeight * ratio + offsetPx，用于还原原 CSS 的 top:50% / calc(50% + Npx) */
  defaultTopRatio?: number
  defaultTopOffset?: number
}

/**
 * 边缘触发条（AI助手 / 信号监控）通用「拖拽 + 吸附」逻辑。
 *
 * 交互口径：
 *  - 按住左键即可任意拖动（位移超过阈值判定为拖动，否则视为普通点击）；
 *  - 拖动中始终以悬浮圆球形态跟随指针（球心对准指针）；
 *  - 松手时距左/右边缘足够近 → 吸附该边缘并恢复「贴边竖条」原型；
 *    否则保持悬浮圆球停在原地；
 *  - 位置（贴边侧/悬浮坐标）持久化到 localStorage，重启后恢复；
 *    窗口尺寸变化时自动收拢回可视区。
 *
 * 为什么只吸附左右两边缘：原型是竖排半圆条，天然适合左右贴边；上/下边缘
 * 不吸附（保持圆球），避免为横条形态再引入一套旋转变形样式。
 */
export function useEdgeTriggerDrag(options: EdgeTriggerDragOptions) {
  const { storageKey, defaultSide = 'right', defaultTopRatio = 0.5, defaultTopOffset = 0 } = options

  /** true = 悬浮圆球形态；false = 贴边竖条原型 */
  const floating = ref(false)
  /** 拖动进行中（视觉态：跟手、禁过渡） */
  const dragging = ref(false)
  /** 本次按下后发生了真实拖动：组件 click 回调据此抑制面板开关 */
  const moved = ref(false)
  /** 当前停靠位置：float=悬浮 / left|right=贴边（用于贴左时的圆角镜像） */
  const side = ref<'float' | 'left' | 'right'>(defaultSide)
  /** 动态定位样式（inline 覆盖 .edge-trigger 默认的 top/right/transform） */
  const style = ref<CSSProperties>({})

  let dockY = 0 // 贴边态：元素中心 y
  let floatX = 0 // 悬浮态：元素左上角
  let floatY = 0

  let pointerId = -1
  let startX = 0
  let startY = 0
  let el: HTMLElement | null = null

  const clamp = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v))

  function applyStyle() {
    if (side.value === 'float') {
      floating.value = true
      style.value = { left: `${floatX}px`, top: `${floatY}px`, right: 'auto', transform: 'none' }
    } else {
      floating.value = false
      // 贴边态沿用原 CSS 的「中心 y + translateY(-50%)」，无需感知元素高度
      const pos: CSSProperties = { top: `${dockY}px`, transform: 'translateY(-50%)' }
      if (side.value === 'left') style.value = { ...pos, left: '0px', right: 'auto' }
      else style.value = { ...pos, left: 'auto', right: '0px' }
    }
  }

  function clampToViewport() {
    dockY = clamp(dockY, 60, Math.max(60, window.innerHeight - 60))
    floatX = clamp(floatX, 0, Math.max(0, window.innerWidth - FLOAT_SIZE))
    floatY = clamp(floatY, 0, Math.max(0, window.innerHeight - FLOAT_SIZE))
  }

  function restore() {
    let restored = false
    try {
      const raw = localStorage.getItem(storageKey)
      if (raw) {
        const s = JSON.parse(raw)
        if (s && (s.side === 'left' || s.side === 'right' || s.side === 'float')) {
          side.value = s.side
          dockY = Number(s.dockY) || 0
          floatX = Number(s.floatX) || 0
          floatY = Number(s.floatY) || 0
          restored = true
        }
      }
    } catch { /* 存储损坏时回退默认位置 */ }
    if (!restored) {
      side.value = defaultSide
      dockY = Math.round(window.innerHeight * defaultTopRatio + defaultTopOffset)
    }
    clampToViewport()
    applyStyle()
  }

  function save() {
    try {
      localStorage.setItem(storageKey, JSON.stringify({ side: side.value, dockY: Math.round(dockY), floatX: Math.round(floatX), floatY: Math.round(floatY) }))
    } catch { /* 忽略存储失败 */ }
  }

  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0) return
    el = e.currentTarget as HTMLElement
    pointerId = e.pointerId
    startX = e.clientX
    startY = e.clientY
    moved.value = false
    // 指针捕获：拖出元素范围后仍能持续收到 move/up
    el.setPointerCapture(pointerId)
    el.addEventListener('pointermove', onPointerMove)
    el.addEventListener('pointerup', onPointerUp)
    el.addEventListener('pointercancel', onPointerUp)
  }

  function onPointerMove(e: PointerEvent) {
    if (e.pointerId !== pointerId) return
    if (!moved.value) {
      if (Math.hypot(e.clientX - startX, e.clientY - startY) < MOVE_THRESHOLD) return
      moved.value = true
      dragging.value = true
    }
    // 拖动中统一以圆球形态跟手（球心对准指针）
    side.value = 'float'
    floatX = clamp(e.clientX - FLOAT_SIZE / 2, 0, Math.max(0, window.innerWidth - FLOAT_SIZE))
    floatY = clamp(e.clientY - FLOAT_SIZE / 2, 0, Math.max(0, window.innerHeight - FLOAT_SIZE))
    applyStyle()
  }

  function onPointerUp(e: PointerEvent) {
    if (e.pointerId !== pointerId) return
    pointerId = -1
    if (el) {
      el.removeEventListener('pointermove', onPointerMove)
      el.removeEventListener('pointerup', onPointerUp)
      el.removeEventListener('pointercancel', onPointerUp)
    }
    dragging.value = false
    if (!moved.value) return // 纯点击：不落位，交给组件 click 回调
    // 吸附判定：距左/右边缘足够近 → 贴边并恢复竖条原型
    if (floatX <= SNAP) side.value = 'left'
    else if (floatX >= window.innerWidth - FLOAT_SIZE - SNAP) side.value = 'right'
    if (side.value !== 'float') {
      dockY = clamp(floatY + FLOAT_SIZE / 2, 60, Math.max(60, window.innerHeight - 60))
    }
    applyStyle()
    save()
  }

  function onResize() {
    clampToViewport()
    applyStyle()
    save()
  }

  onMounted(() => {
    restore()
    window.addEventListener('resize', onResize)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('resize', onResize)
    // 拖动中组件被卸载的兜底清理
    if (el && pointerId !== -1) {
      el.removeEventListener('pointermove', onPointerMove)
      el.removeEventListener('pointerup', onPointerUp)
      el.removeEventListener('pointercancel', onPointerUp)
    }
    pointerId = -1
    el = null
  })

  return { floating, dragging, moved, side, style, onPointerDown }
}
