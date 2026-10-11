// Agent 流式事件处理（阶段3 composable 拆分，自 FloatingAgentAssistant.vue 迁出）
// 职责：onAgentMessage 流式路由（思考区/步骤/工具卡片/任务清单/轮次统计/DONE 收尾）、
//       工具调用卡片状态机、流式期间的定期 markdown 格式化定时器、
//       发送编排（dispatch 落库消息对 + 调后端）、重新生成/编辑重发、中断收尾。
// 会话状态与历史持久化见 ./useAgentSessions.js；UI 提示（toast/分享浮层）留在宿主组件。
import { nextTick } from 'vue'
import { useMessage } from 'naive-ui'
import { ChatWithAgent, AbortChatWithAgentBySession, SaveAIResponseResult, SendDesktopNotification } from '../../../wailsjs/go/main/App'
import { formatMarkdown } from './format'
import { parseTodosSnapshot, extractPlanChecklist, parseStepText } from './streamParse'

// Wails 全局事件名：与 agent-chat.vue 共用（双端按 sessionId 路由互不串扰）
export const AGENT_EVENT = 'agent-message'

/**
 * 流式事件处理
 * @param {{
 *   sessions: ReturnType<typeof useAgentSessions>,   // 会话仓库（见 useAgentSessions.js）
 *   scrollToBottom: (force?: boolean) => void,       // 滚动回调（宿主提供）
 *   config: {                                        // 执行参数 refs（宿主持有，设置面板绑定）
 *     aiConfigId, aiConfigOptions, sysPromptId, selectedSkillDirs,
 *     memoryMode, memoryCount, thinkingMode, agentMode,
 *     inputValue, pendingImages,                     // 发送后需清空的输入状态
 *     panelVisible                                   // 面板开合：关闭时流完成发系统通知
 *   }
 * }} deps
 */
export function useAgentStream({ sessions, scrollToBottom, config }) {
  const message = useMessage()
  const {
    sessionStates,
    activeSessionId,
    sessionId,
    messages,
    isStreamLoad,
    isAborted,
    sentFromFloating,
    reasoningExpandedMap,
    messageGroups,
    ensureLatestGroupExpanded,
    saveHistory
  } = sessions
  const {
    aiConfigId,
    aiConfigOptions,
    sysPromptId,
    selectedSkillDirs,
    memoryMode,
    memoryCount,
    thinkingMode,
    agentMode,
    inputValue,
    pendingImages,
    panelVisible
  } = config

  // 根据 AI 配置 id 解析模型展示名（下拉 label，含 [模型名]/[视觉] 后缀）
  function modelLabelForConfig(configId) {
    const opts = aiConfigOptions.value
    if (!opts?.length) return ''
    const id = configId != null ? Number(configId) : Number(opts[0].value)
    const found = opts.find(o => Number(o.value) === id)
    return found?.label != null ? String(found.label) : ''
  }

  function formatCardDuration(ms) {
    if (ms == null || ms < 0) return ''
    if (ms < 60000) return (ms / 1000).toFixed(1).replace(/\.0$/, '') + 's'
    return Math.floor(ms / 60000) + 'm' + Math.round((ms % 60000) / 1000) + 's'
  }

  // 参数预览：JSON 对象取 key=value 拼接，数组报项数，其余截断原文
  function buildArgsPreview(args) {
    const raw = (args || '').trim()
    if (!raw) return ''
    try {
      const obj = JSON.parse(raw)
      if (Array.isArray(obj)) return `${obj.length} 项`
      if (typeof obj === 'object' && obj !== null) {
        const pairs = Object.entries(obj).map(([k, v]) => {
          let s = typeof v === 'string' ? v : JSON.stringify(v)
          if (s && s.length > 24) s = s.slice(0, 24) + '…'
          return `${k}=${s}`
        })
        const out = pairs.join(' ')
        return out.length > 60 ? out.slice(0, 60) + '…' : out
      }
    } catch {}
    return raw.length > 60 ? raw.slice(0, 60) + '…' : raw
  }

  // ---- 工具调用过程卡片 ----
  // 后端把工具活动编码进 [STEP] 文本流：🔧 调用工具：name(args) / ✅ name 返回结果（N字）/ ⚠️ ...失败。
  // 这里将其解析为结构化卡片并维护状态机：running → success / error / stopped。
  // 另兼容部分路径透传的结构化 tool_calls（含 id，参数可能分片到达）。

  // 首个工具卡片出现时自动展开面板（仅对当前激活会话生效，expandKey 为 't-<消息索引>'）
  function expandToolCards(expandKey) {
    if (!expandKey) return
    reasoningExpandedMap.value = {
      ...reasoningExpandedMap.value,
      [expandKey]: true
    }
  }

  function pushToolCard(item, name, args, expandKey) {
    if (!item.toolCards) item.toolCards = []
    const last = item.toolCards[item.toolCards.length - 1]
    if (last && last.status === 'running' && last.name === name) {
      // 同名且仍在执行的重复预告：去重，参数以更长分片为准
      if ((args || '').length > (last.args || '').length) {
        last.args = args
        last.argsPreview = buildArgsPreview(args)
      }
      return
    }
    item.toolCards.push({ id: '', name, args, argsPreview: buildArgsPreview(args), status: 'running', startTs: Date.now(), duration: '' })
    expandToolCards(expandKey)
  }

  function completeToolCard(item, name, status) {
    const cards = item.toolCards || []
    for (let i = cards.length - 1; i >= 0; i--) {
      if (cards[i].status === 'running' && (!name || cards[i].name === name)) {
        cards[i].status = status
        cards[i].endTs = Date.now()
        cards[i].duration = formatCardDuration(cards[i].endTs - cards[i].startTs)
        if (name) break
      }
    }
  }

  // 停止/结束时收尾：仍在运行的卡片标记为已停止，避免空转 spinner
  function stopRunningToolCards(item) {
    completeToolCard(item, null, 'stopped')
  }

  // 从一段 [STEP] 文本中按出现顺序解析工具事件（一个分片可能同时含完成与新调用）
  function updateToolCardsFromStep(item, text, expandKey) {
    if (!text) return
    if (text.includes('⚠️') && (text.includes('失败') || text.includes('异常'))) {
      completeToolCard(item, null, 'error')
    }
    const events = []
    let m
    const doneRe = /✅\s*([^\s（(]+?)\s*返回结果/g
    while ((m = doneRe.exec(text)) !== null) events.push({ pos: m.index, type: 'done', name: m[1] })
    const callRe = /调用工具[:：]\s*([^\s（(]+?)\s*[（(]/g
    while ((m = callRe.exec(text)) !== null) events.push({ pos: m.index, type: 'call', name: m[1], argsStart: m.index + m[0].length })
    if (!events.length) return
    events.sort((a, b) => a.pos - b.pos)
    for (let i = 0; i < events.length; i++) {
      const ev = events[i]
      if (ev.type === 'call') {
        let end = text.length
        for (let j = i + 1; j < events.length; j++) {
          if (events[j].pos > ev.pos) { end = events[j].pos; break }
        }
        const args = text.slice(ev.argsStart, end).replace(/\s*\n\s*$/, '').replace(/[）)]\s*$/, '')
        pushToolCard(item, ev.name, args, expandKey)
      } else {
        completeToolCard(item, ev.name, 'success')
      }
    }
  }

  // ---- 流式期间的定期 markdown 格式化（后台会话同样受益，定时器挂在会话状态上） ----
  function startFormatTimer(sid = activeSessionId.value) {
    stopFormatTimer(sid)
    const st = sessionStates[sid]
    if (!st) return
    st.formatTimer = setInterval(() => {
      const list = st.messages
      const last = list[list.length - 1]
      if (last && last.role === 'assistant') {
        if (last.rawContent) {
          const fmt = formatMarkdown(last.rawContent)
          last.content = fmt.content
          if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        }
        if (last.rawReasoning) {
          const fmt = formatMarkdown(last.rawReasoning)
          last.reasoning = fmt.content
        }
      }
    }, 1500)
  }

  function stopFormatTimer(sid = activeSessionId.value) {
    const st = sessionStates[sid]
    if (st && st.formatTimer) {
      clearInterval(st.formatTimer)
      st.formatTimer = null
    }
  }

  // 中断核心：收尾工具卡片/思考用时并格式化已积累内容，调后端按会话取消。
  // 返回是否真正中断了流（供宿主决定是否提示「已中断」），UI 提示留在宿主组件。
  function abortStream(sid) {
    const st = sessionStates[sid]
    if (!st || !st.isStreamLoad) return false
    st.isAborted = true
    st.isStreamLoad = false
    stopFormatTimer(sid)
    const list = st.messages
    const last = list[list.length - 1]
    if (last && last.role === 'assistant') {
      stopRunningToolCards(last)
      if (last.thinkingStartTs && !last.thinkingDuration) {
        last.thinkingDuration = Date.now() - last.thinkingStartTs
      }
      if (last.rawContent) {
        const fmt = formatMarkdown(last.rawContent)
        last.content = fmt.content
        if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
      }
      if (last.rawReasoning) {
        const fmt = formatMarkdown(last.rawReasoning)
        last.reasoning = fmt.content
      }
    }
    AbortChatWithAgentBySession(sid)
    return true
  }

  // 发送编排（sendMessage 与重新生成/编辑重发共用）：落库消息对 + 调后端。
  // overrideConfigId 供「换模型重试」一次性指定模型，
  // 不改宿主 aiConfigId（避免触发模型切换副作用：thinkingMode 自动调整/提示/持久化）
  function dispatchAgentMessage(text, images, overrideConfigId) {
    const userMsg = {
      role: 'user',
      content: text,
      time: new Date().toLocaleString(),
      modelName: '',
      reasoning: '',
      steps: []
    }
    if (images.length) userMsg.images = images
    messages.value.push(userMsg)
    const configId = overrideConfigId ?? aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
    const modelName = modelLabelForConfig(configId)
    messages.value.push({
      role: 'assistant',
      content: '',
      rawContent: '',
      time: new Date().toLocaleString(),
      modelName,
      reasoning: '',
      rawReasoning: '',
      steps: [],
      toolCards: [],
      todos: [],
      jsonMarkdown: '',
      stats: null
    })
    inputValue.value = ''
    pendingImages.value = []
    isStreamLoad.value = true
    isAborted.value = false
    sentFromFloating.value = true
    startFormatTimer()
    saveHistory()
    nextTick(() => {
      ensureLatestGroupExpanded()
      const lastGroup = messageGroups.value[messageGroups.value.length - 1]
      if (lastGroup) {
        reasoningExpandedMap.value = {
          ...reasoningExpandedMap.value,
          [lastGroup.assistantIndex]: true,
          ['j-' + lastGroup.assistantIndex]: true
        }
      }
      scrollToBottom(true)
    })
    ChatWithAgent(text, configId, selectedSkillDirs.value.length ? null : sysPromptId.value, memoryMode.value, memoryCount.value, thinkingMode.value, agentMode.value === 'auto' ? '' : agentMode.value, sessionId.value, selectedSkillDirs.value.join(','), images.length ? JSON.stringify(images) : '')
  }

  // 重新生成：撤回该轮回答后按原问题重发（仅最后一轮可用，对齐主流交互）；
  // overrideConfigId 传入时用指定模型重试本轮（换模型重试）
  function regenerateMessage(group, overrideConfigId) {
    if (isStreamLoad.value) return
    const st = sessionStates[activeSessionId.value]
    if (!st) return
    const list = st.messages
    const idx = group.assistantIndex
    const userMsg = list[idx - 1]
    if (!userMsg || userMsg.role !== 'user') {
      message.warning('未找到对应的提问，无法重新生成')
      return
    }
    const question = userMsg.content
    const images = Array.isArray(userMsg.images) ? userMsg.images.slice() : []
    st.messages = list.slice(0, idx)
    saveHistory()
    dispatchAgentMessage(question, images, overrideConfigId)
  }

  // 编辑重发：问题回填输入框并撤回该轮问答，修改后重新发送（仅最后一轮可用）
  function editUserMessage(group) {
    if (isStreamLoad.value) return
    const st = sessionStates[activeSessionId.value]
    if (!st) return
    inputValue.value = group.userMsg.content
    st.messages = st.messages.slice(0, group.userIndex)
    saveHistory()
    message.info('已回填到输入框，修改后重新发送')
  }

  function onAgentMessage(msg) {
    // 按 sessionId 路由：仅处理本组件已登记的会话；空 sessionId（独立 /agent 页）一律忽略，
    // 顺带消除与 agent-chat.vue 共用全局 agent-message 事件的串扰。
    const sid = msg?.sessionId
    if (!sid) return
    const st = sessionStates[sid]
    if (!st || st.isAborted) return
    const list = st.messages

    if (msg.content === 'agent-DONE' || (msg?.response_meta?.finish_reason === 'stop')) {
      st.isStreamLoad = false
      st.sentFromFloating = false
      st.isAborted = false
      stopFormatTimer(sid)
      const last = list[list.length - 1]
      if (last && last.role === 'assistant') {
        stopRunningToolCards(last)
        // 结束时仍无答案内容：思考用时在此定格（兜底纯思考被打断等场景）
        if (last.thinkingStartTs && !last.thinkingDuration) {
          last.thinkingDuration = Date.now() - last.thinkingStartTs
        }
        if (last.rawContent) {
          const fmt = formatMarkdown(last.rawContent)
          last.content = fmt.content
          if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        }
        if (last.rawReasoning) {
          const fmt = formatMarkdown(last.rawReasoning)
          last.reasoning = fmt.content
        }
      }
      saveHistory(sid)
      if (sid === activeSessionId.value) nextTick(scrollToBottom)
      if (msg.content === 'agent-DONE' && last && last.role === 'assistant' && last.content) {
        const user = list[list.length - 2]
        SaveAIResponseResult('agent', '市场分析', last.content, sid, user?.content, aiConfigId.value)
        // 面板关闭的后台分析完成：发系统桌面通知提醒（受设置页「本地推送」开关控制）
        if (panelVisible && panelVisible.value === false) {
          try {
            SendDesktopNotification('go-stock AI 分析完成', '后台分析已生成，点击右侧 AI 助手胶囊查看结果')
          } catch (_) { /* 通知失败静默 */ }
        }
      }
      return
    }

    const roleLower = String(msg?.role || '').toLowerCase()
    if (roleLower !== 'assistant') {
      return
    }

    const last = list[list.length - 1]
    if (last && last.role === 'assistant') {
      if (msg?.reasoning_content) {
        const rc = msg.reasoning_content
        if (rc.startsWith('[STATS]')) {
          // 本轮统计（后端 sendTurnStats 发送）：提取为结构化字段单独展示，
          // 不混入 reasoning 折叠区
          const m = rc.match(/工具调用\s*(\d+)\s*次｜输入\s*(\d+)\s*token｜输出\s*(\d+)\s*token｜耗时\s*([^\n]+)/)
          if (m) {
            last.stats = {
              tools: parseInt(m[1], 10),
              inputTokens: parseInt(m[2], 10),
              outputTokens: parseInt(m[3], 10),
              duration: m[4].trim().replace(/(\d+)(?:\.\d+)?s$/, '$1s')
            }
          }
        } else if (rc.startsWith('[STEP]')) {
          const stepText = rc.replace(/^\[STEP\]/, '').trim()
          if (stepText) {
            // 📝 任务清单是完整快照：解析为结构化 todos 面板并整体替换，不进步骤列表
            const todos = parseTodosSnapshot(stepText)
            if (todos) {
              last.todos = todos
              if (sid === activeSessionId.value) {
                reasoningExpandedMap.value = {
                  ...reasoningExpandedMap.value,
                  ['p-' + (list.length - 1)]: true
                }
              }
            } else {
              if (!last.steps) last.steps = []
              const parsed = parseStepText(stepText)
              last.steps.push(...parsed)
              // 同步解析工具调用事件，驱动「工具调用」过程卡片（开始/完成/失败）
              const expandKey = sid === activeSessionId.value ? 't-' + (list.length - 1) : null
              updateToolCardsFromStep(last, stepText, expandKey)
            }
          }
        } else {
          // 首个思考片段到达：记录起始时间并自动展开思考区（仅当前激活会话）
          if (!last.thinkingStartTs) {
            last.thinkingStartTs = Date.now()
            if (sid === activeSessionId.value) {
              reasoningExpandedMap.value = {
                ...reasoningExpandedMap.value,
                ['r-' + (list.length - 1)]: true
              }
            }
          }
          last.rawReasoning = (last.rawReasoning || '') + rc
          last.reasoning = last.rawReasoning
        }
      }
      if (msg?.content) {
        // 答案开始输出：定格思考用时并自动收起思考区（仅当前激活会话，对齐主流交互）
        if (last.thinkingStartTs && !last.thinkingDuration) {
          last.thinkingDuration = Date.now() - last.thinkingStartTs
          if (sid === activeSessionId.value) {
            reasoningExpandedMap.value = {
              ...reasoningExpandedMap.value,
              ['r-' + (list.length - 1)]: false
            }
          }
        }
        last.rawContent = (last.rawContent || '') + msg.content
        last.content = last.rawContent
        // PlanExecute 模式：模型在 content 中输出并更新任务清单（- [N] 步骤 [状态]），
        // 从累积文本提取最新快照驱动进度面板
        const plan = extractPlanChecklist(last.rawContent)
        if (plan) last.todos = plan
      }
      if (Array.isArray(msg?.tool_calls) && msg.tool_calls.length > 0) {
        // 部分后端路径透传结构化 tool_calls（含 id，参数可能分片到达）：按 id 聚合进卡片
        if (!last.toolCards) last.toolCards = []
        const expandKey = sid === activeSessionId.value ? 't-' + (list.length - 1) : null
        for (const tool of msg.tool_calls) {
          const name = tool.function && tool.function.name ? tool.function.name : ''
          if (!name) continue
          let card = tool.id ? last.toolCards.find(c => c.id === tool.id) : null
          if (!card) {
            card = { id: tool.id || '', name, args: '', argsPreview: '', status: 'running', startTs: Date.now(), duration: '' }
            last.toolCards.push(card)
            expandToolCards(expandKey)
          }
          const frag = tool.function && tool.function.arguments ? tool.function.arguments : ''
          if (frag) {
            card.args += frag
            card.argsPreview = buildArgsPreview(card.args)
          }
        }
      }
      if (sid === activeSessionId.value) nextTick(scrollToBottom)
    }
  }

  return {
    AGENT_EVENT,
    onAgentMessage,
    abortStream,
    dispatchAgentMessage,
    regenerateMessage,
    editUserMessage,
    startFormatTimer,
    stopFormatTimer,
    stopRunningToolCards
  }
}
