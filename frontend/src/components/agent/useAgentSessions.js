// Agent 多会话状态仓库（阶段3 composable 拆分，自 FloatingAgentAssistant.vue 迁出）
// 职责：会话状态隔离（sessionStates）、会话列表/激活/重命名/置顶/删除、历史持久化、
//       消息分组视图模型（messageGroups）与分组展开/折叠面板状态。
// 纯状态管理：不含流式事件处理（见 ./useAgentStream.js）与 UI 交互（留在宿主组件）。
import { reactive, ref, computed, nextTick, onBeforeUnmount } from 'vue'
import { useMessage } from 'naive-ui'
import {
  SaveAiAssistantSession,
  GetAiAssistantSession,
  ListAiAssistantSessions,
  RenameAiAssistantSession,
  SetAiAssistantSessionPinned,
  DeleteAiAssistantSession
} from '../../../wailsjs/go/main/App'

const STORAGE_KEY_ACTIVE_SESSION = 'go-stock-agent-active-session'

/**
 * 多会话状态仓库
 * @param {{ scrollToBottom: (force?: boolean) => void, hooks?: { beforeDeleteSession?: (sid: string) => void } }} deps
 *   scrollToBottom：滚动到底部回调（切换会话/加载历史后由宿主组件提供，避免 composable 依赖 DOM 引用）
 *   hooks.beforeDeleteSession：删除会话前的清理钩子（由 useAgentStream 注入 stopFormatTimer，
 *   会话的流式定时器归属流处理模块，本模块不直接持有）
 */
export function useAgentSessions({ scrollToBottom, hooks = {} }) {
  const message = useMessage()

  // ===== 多会话状态仓库 =====
  // sessionStates: sessionId -> { messages, isStreamLoad, sentFromFloating, isAborted,
  //   expandedGroups:Set, reasoningExpandedMap, formatTimer, title }
  // 通过「可写 computed」把原有的单例状态代理到当前激活会话，使既有代码（xxx.value = ...）
  // 与模板绑定无需大改即可按会话隔离；后台会话的流式写入由 onAgentMessage 直接操作其 state。
  const sessionStates = reactive({})
  // sessionOrder 为当前展示的会话列表：浏览态为本地未落库的新会话 + 后端最近 10 条；
  // 搜索态为后端按关键词匹配的结果（条目上 local=true 表示尚未落库）
  const sessionOrder = ref([]) // [{ sessionId, title, updatedAt, local? }]
  const activeSessionId = ref('')
  const sessionKeyword = ref('')
  let sessionSearchTimer = null

  function ensureSessionState(sid) {
    if (!sid) return null
    if (!sessionStates[sid]) {
      sessionStates[sid] = {
        messages: [],
        isStreamLoad: false,
        sentFromFloating: false,
        isAborted: false,
        expandedGroups: new Set(),
        reasoningExpandedMap: {},
        formatTimer: null,
        title: '',
        draft: '' // 未发送的输入草稿：切换会话时由宿主保存/恢复，避免多会话草稿互相覆盖
      }
    }
    return sessionStates[sid]
  }

  function sessionState(sid = activeSessionId.value) {
    return sessionStates[sid] || null
  }

  const messages = computed({
    get: () => sessionState()?.messages ?? [],
    set: v => { const st = sessionState(); if (st) st.messages = v }
  })
  const isStreamLoad = computed({
    get: () => !!sessionState()?.isStreamLoad,
    set: v => { const st = sessionState(); if (st) st.isStreamLoad = v }
  })
  const sentFromFloating = computed({
    get: () => !!sessionState()?.sentFromFloating,
    set: v => { const st = sessionState(); if (st) st.sentFromFloating = v }
  })
  const isAborted = computed({
    get: () => !!sessionState()?.isAborted,
    set: v => { const st = sessionState(); if (st) st.isAborted = v }
  })
  const expandedGroups = computed({
    get: () => sessionState()?.expandedGroups ?? new Set(),
    set: v => { const st = sessionState(); if (st) st.expandedGroups = v }
  })
  const reasoningExpandedMap = computed({
    get: () => sessionState()?.reasoningExpandedMap ?? {},
    set: v => { const st = sessionState(); if (st) st.reasoningExpandedMap = v }
  })
  const sessionId = computed({
    get: () => activeSessionId.value,
    set: v => { activeSessionId.value = v }
  })

  // 消息分组视图模型：把扁平消息列表按「用户提问 + AI 回答」聚合为问答组，驱动 MessageGroup 渲染
  const messageGroups = computed(() => {
    const groups = []
    let currentGroup = null

    for (let i = 0; i < messages.value.length; i++) {
      const msg = messages.value[i]
      if (msg.role === 'user') {
        if (currentGroup) {
          groups.push(currentGroup)
        }
        currentGroup = {
          id: i,
          userMsg: msg,
          userIndex: i,
          assistantMsg: null,
          assistantIndex: -1
        }
      } else if (msg.role === 'assistant' && currentGroup) {
        currentGroup.assistantMsg = msg
        currentGroup.assistantIndex = i
      }
    }
    if (currentGroup) {
      groups.push(currentGroup)
    }
    return groups
  })

  function isGroupExpanded(groupIndex) {
    return expandedGroups.value.has(groupIndex)
  }

  function toggleGroup(groupIndex) {
    const newSet = new Set(expandedGroups.value)
    if (newSet.has(groupIndex)) {
      newSet.delete(groupIndex)
    } else {
      newSet.add(groupIndex)
    }
    expandedGroups.value = newSet
  }

  function initDefaultExpanded() {
    if (messageGroups.value.length > 0 && expandedGroups.value.size === 0) {
      expandedGroups.value = new Set([messageGroups.value.length - 1])
    }
  }

  function ensureLatestGroupExpanded() {
    if (messageGroups.value.length > 0) {
      const lastIndex = messageGroups.value.length - 1
      const newSet = new Set(expandedGroups.value)
      newSet.add(lastIndex)
      expandedGroups.value = newSet
    }
  }

  function toggleReasoning(index) {
    reasoningExpandedMap.value = {
      ...reasoningExpandedMap.value,
      [index]: !reasoningExpandedMap.value[index]
    }
  }

  // ===== 多会话管理 =====

  const SESSION_WELCOME = '我是 go-stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？'

  function welcomeMessage() {
    return {
      role: 'assistant',
      content: SESSION_WELCOME,
      time: new Date().toLocaleString(),
      modelName: '',
      reasoning: ''
    }
  }

  // 生成会话 ID：时间戳 + 随机后缀，避免同毫秒碰撞
  function newSessionId() {
    return Date.now().toString() + Math.random().toString(36).slice(2, 6)
  }

  // 会话标题：取首条 user 消息前 40 字
  function deriveTitle(st) {
    const firstUser = (st?.messages || []).find(m => m.role === 'user' && m.content)
    if (!firstUser) return ''
    const t = String(firstUser.content).replace(/\s+/g, ' ').trim()
    return t.length > 40 ? t.slice(0, 40) + '…' : t
  }

  // 更新本地会话列表条目的标题/时间（不存在则按需插入，local 标记尚未落库的新会话）
  function upsertSessionOrder(sid, { title, updatedAt, prepend = false } = {}) {
    const idx = sessionOrder.value.findIndex(s => s.sessionId === sid)
    if (idx >= 0) {
      const cur = sessionOrder.value[idx]
      sessionOrder.value[idx] = {
        ...cur,
        sessionId: sid,
        title: title || cur.title,
        updatedAt: updatedAt || cur.updatedAt
      }
    } else if (prepend) {
      sessionOrder.value.unshift({ sessionId: sid, title: title || '', updatedAt: updatedAt || '', local: true })
    } else {
      sessionOrder.value.push({ sessionId: sid, title: title || '', updatedAt: updatedAt || '', local: true })
    }
  }

  // ---- 会话重命名 / 置顶 ----
  const renamingSessionId = ref('')
  const renamingTitle = ref('')

  function startRenameSession(s) {
    renamingSessionId.value = s.sessionId
    renamingTitle.value = s.title || ''
    nextTick(() => {
      const el = document.querySelector('.session-menu-item .n-input__input-el')
      el?.focus()
    })
  }

  // Enter/失焦保存；Esc 走 cancel（renamingSessionId 已清空时直接返回，避免 Enter→blur 双触发）
  async function confirmRenameSession() {
    const sid = renamingSessionId.value
    if (!sid) return
    renamingSessionId.value = ''
    const title = renamingTitle.value.trim()
    try {
      await RenameAiAssistantSession(sid, title)
    } catch (_) {
    }
    if (!title) {
      // 清除自定义标题：回拉列表以恢复正文派生标题
      await refreshSessionList(sessionKeyword.value)
      return
    }
    const item = sessionOrder.value.find(s => s.sessionId === sid)
    if (item) item.title = title
  }

  function cancelRenameSession() {
    renamingSessionId.value = ''
  }

  async function togglePinSession(s) {
    const next = !s.pinned
    try {
      await SetAiAssistantSessionPinned(s.sessionId, next)
    } catch (_) {
      message.error('置顶操作失败')
      return
    }
    s.pinned = next
    // 本地立即重排：置顶在前，其余保持原有更新时间倒序（sort 为稳定排序）
    sessionOrder.value = [...sessionOrder.value].sort((a, b) => (b.pinned === true) - (a.pinned === true))
  }

  // 从后端拉取会话列表：keyword 为空取最近 10 条，非空则按标题与内容搜索（上限 50 条）。
  // 浏览态保留本地尚未落库的新会话，避免刚新建的会话从列表消失。
  async function refreshSessionList(keyword = '') {
    try {
      const kw = keyword.trim()
      const list = await ListAiAssistantSessions(kw, kw ? 50 : 10)
      const remote = Array.isArray(list) ? list : []
      const remoteIds = new Set(remote.map(s => s.sessionId))
      const localOnly = kw ? [] : sessionOrder.value.filter(s => s.local && !remoteIds.has(s.sessionId))
      sessionOrder.value = [
        ...localOnly,
        ...remote.map(s => ({ sessionId: s.sessionId, title: s.title ?? '', updatedAt: s.updatedAt ?? '', pinned: s.pinned === true }))
      ]
    } catch (_) {
    }
  }

  // 搜索框输入：防抖后请求后端，避免逐字触发查询
  function onSessionSearchInput() {
    if (sessionSearchTimer) clearTimeout(sessionSearchTimer)
    sessionSearchTimer = setTimeout(() => {
      sessionSearchTimer = null
      refreshSessionList(sessionKeyword.value)
    }, 250)
  }

  // 把后端返回的消息数组规整为前端消息结构
  function normalizeMessages(list) {
    return list.map(m => ({
      role: m.role ?? '',
      content: m.content ?? '',
      time: m.time ?? '',
      modelName: m.modelName ?? '',
      reasoning: m.reasoning ?? '',
      steps: m.steps ?? [],
      // 恢复历史时工具卡片不再处于运行态（会话已结束），统一收尾为已停止
      toolCards: (Array.isArray(m.toolCards) ? m.toolCards : []).map(c => ({
        ...c,
        status: c.status === 'running' ? 'stopped' : c.status,
        duration: c.duration || ''
      })),
      todos: m.todos ?? [],
      jsonMarkdown: m.jsonMarkdown ?? '',
      images: Array.isArray(m.images) ? m.images : []
    }))
  }

  // 加载指定会话的消息历史（仅在该会话尚未加载时拉取，避免覆盖正在流式写入的后台会话）
  async function loadSessionMessages(sid) {
    if (!sid) return
    const st = ensureSessionState(sid)
    if (st.messages.length > 0) return
    try {
      const resp = await GetAiAssistantSession(sid)
      const list = resp?.messages
      if (Array.isArray(list) && list.length > 0) {
        st.messages = normalizeMessages(list)
      }
    } catch (_) {
    }
  }

  // 切换当前激活会话
  async function setActiveSession(sid) {
    if (!sid) return
    activeSessionId.value = sid
    localStorage.setItem(STORAGE_KEY_ACTIVE_SESSION, sid)
    ensureSessionState(sid)
    await loadSessionMessages(sid)
    nextTick(() => {
      initDefaultExpanded()
      scrollToBottom(true)
    })
  }

  // 删除会话：中断 → 后端删除 → 清理本地 → 重选激活会话
  async function deleteSession(sid) {
    if (!sid) return
    try {
      await DeleteAiAssistantSession(sid)
    } catch (_) {
    }
    hooks.beforeDeleteSession?.(sid)
    delete sessionStates[sid]
    sessionOrder.value = sessionOrder.value.filter(s => s.sessionId !== sid)
    if (activeSessionId.value === sid) {
      const next = sessionOrder.value[0]?.sessionId
      if (next) {
        setActiveSession(next)
      } else {
        createSession()
      }
    }
  }

  // 新建一个空会话并激活
  function createSession() {
    const sid = newSessionId()
    const st = ensureSessionState(sid)
    st.messages = [welcomeMessage()]
    upsertSessionOrder(sid, { prepend: true })
    setActiveSession(sid)
    return sid
  }

  async function loadHistory() {
    await refreshSessionList()
    if (!activeSessionId.value) {
      const remembered = localStorage.getItem(STORAGE_KEY_ACTIVE_SESSION)
      if (remembered) {
        // 上次会话若不在「最近 10 条」内，也仍需恢复它，并临时挂到列表顶部以便可见
        if (!sessionOrder.value.some(s => s.sessionId === remembered)) {
          upsertSessionOrder(remembered, { prepend: true })
        }
        activeSessionId.value = remembered
      } else if (sessionOrder.value.length > 0) {
        activeSessionId.value = sessionOrder.value[0].sessionId
      } else {
        const sid = newSessionId()
        ensureSessionState(sid)
        upsertSessionOrder(sid, { prepend: true })
        activeSessionId.value = sid
      }
    }
    const sid = activeSessionId.value
    const st = ensureSessionState(sid)
    localStorage.setItem(STORAGE_KEY_ACTIVE_SESSION, sid)
    await loadSessionMessages(sid)
    // 无历史内容时给当前会话补欢迎语
    if (st.messages.length === 0) {
      st.messages = [welcomeMessage()]
    }
    // 用已加载的消息补全标题（恢复的旧会话可能还没有列表条目标题）
    upsertSessionOrder(sid, { title: deriveTitle(st) })
    nextTick(() => {
      initDefaultExpanded()
    })
  }

  function saveHistory(sid = activeSessionId.value) {
    const st = sessionStates[sid]
    if (!st || st.messages.length === 0) return
    const list = st.messages.map(m => ({
      role: m.role,
      content: m.content,
      time: m.time ?? '',
      modelName: m.modelName ?? '',
      reasoning: m.reasoning ?? '',
      steps: m.steps ?? [],
      toolCards: m.toolCards ?? [],
      todos: m.todos ?? [],
      jsonMarkdown: m.jsonMarkdown ?? '',
      images: (m.role === 'user' && Array.isArray(m.images)) ? m.images : []
    }))
    SaveAiAssistantSession(sid, list).catch(() => {})
    // 同步本地会话列表的标题与时间
    upsertSessionOrder(sid, { title: deriveTitle(st), updatedAt: new Date().toLocaleString() })
  }

  onBeforeUnmount(() => {
    if (sessionSearchTimer) {
      clearTimeout(sessionSearchTimer)
      sessionSearchTimer = null
    }
  })

  return {
    // 状态
    sessionStates,
    sessionOrder,
    activeSessionId,
    sessionKeyword,
    renamingSessionId,
    renamingTitle,
    // 会话状态读写
    ensureSessionState,
    sessionState,
    messages,
    isStreamLoad,
    sentFromFloating,
    isAborted,
    expandedGroups,
    reasoningExpandedMap,
    sessionId,
    // 分组视图模型与展开状态
    messageGroups,
    isGroupExpanded,
    toggleGroup,
    initDefaultExpanded,
    ensureLatestGroupExpanded,
    toggleReasoning,
    // 会话生命周期
    welcomeMessage,
    newSessionId,
    deriveTitle,
    upsertSessionOrder,
    startRenameSession,
    confirmRenameSession,
    cancelRenameSession,
    togglePinSession,
    refreshSessionList,
    onSessionSearchInput,
    normalizeMessages,
    loadSessionMessages,
    setActiveSession,
    deleteSession,
    createSession,
    loadHistory,
    saveHistory
  }
}
