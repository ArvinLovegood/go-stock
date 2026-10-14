<template>
  <div class="chat-box">
    <t-chat
        ref="chatRef"
        :clear-history="chatList.length > 0 && !isStreamLoad"
        :data="chatList"
        :text-loading="loading"
        :is-stream-load="isStreamLoad"
        style="height: 100%"
        @scroll="handleChatScroll"
        @clear="clearConfirm"
    >
      <!-- eslint-disable vue/no-unused-vars -->
      <template #content="{ item, index }">
        <!-- 任务清单进度面板：write_todos 快照（[STEP]📝）与 PlanExecute content 清单（- [N] 步骤 [状态]）均解析至此 -->
        <div v-if="item.role === 'assistant' && item.todos && item.todos.length > 0" class="agent-todos">
          <div class="agent-todos-header" @click="toggleToolCards('p' + index)">
            <span>📋</span>
            <span class="agent-todos-title">任务清单</span>
            <span class="agent-todos-progress">{{ todosProgress(item) }}</span>
            <span v-if="todosRunning(item)" class="agent-tool-spinner"></span>
            <svg :class="['agent-json-md-arrow', { 'agent-json-md-arrow-expanded': toolCardsExpandedMap['p' + index] }]" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
          </div>
          <div v-show="toolCardsExpandedMap['p' + index]" class="agent-todos-list">
            <div v-for="(td, ti) in item.todos" :key="ti" class="agent-todo-item" :class="'todo-' + td.status">
              <span class="agent-todo-icon">{{ td.status === 'completed' ? '✅' : td.status === 'in_progress' ? '🔄' : '⏳' }}</span>
              <span class="agent-todo-text">{{ td.text }}</span>
            </div>
          </div>
        </div>
        <div v-if="item.role === 'assistant' && visibleSteps(item).length > 0" class="agent-steps">
          <div class="agent-steps-header">📋 执行步骤 <span class="agent-steps-badge">{{ visibleSteps(item).length }}</span></div>
          <div class="agent-steps-list">
            <div v-for="(step, si) in visibleSteps(item)" :key="si" class="agent-step-item">
              <div class="agent-step-dot" :class="getStepDotClass(step)"></div>
              <span class="agent-step-text">{{ step }}</span>
            </div>
          </div>
        </div>
        <!-- 工具调用过程卡片：状态机（运行中→成功/失败/停止），数据来自 [STEP] 流与结构化 tool_calls -->
        <div v-if="item.role === 'assistant' && item.toolCards && item.toolCards.length > 0" class="agent-tool-cards">
          <div class="agent-tool-cards-header" @click="toggleToolCards(index)">
            <span>🔧</span>
            <span class="agent-tool-cards-title">工具调用</span>
            <span class="agent-tool-cards-badge">{{ item.toolCards.length }}</span>
            <span v-if="toolCardsRunning(item)" class="agent-tool-cards-live">
              <span class="agent-tool-spinner"></span>
              {{ runningToolName(item) }}
            </span>
            <span v-else class="agent-tool-cards-done">已完成</span>
            <svg :class="['agent-json-md-arrow', { 'agent-json-md-arrow-expanded': toolCardsExpandedMap[index] }]" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
          </div>
          <div v-show="toolCardsExpandedMap[index]" class="agent-tool-cards-list">
            <div v-for="(card, ci) in item.toolCards" :key="ci" class="agent-tool-card" :title="card.args">
              <span class="agent-tool-status" :class="'tool-' + card.status">
                <span v-if="card.status === 'running'" class="agent-tool-spinner"></span>
                <template v-else>{{ card.status === 'success' ? '✅' : card.status === 'error' ? '❌' : '⏹' }}</template>
              </span>
              <span class="agent-tool-name">{{ card.name }}</span>
              <span class="agent-tool-args">{{ card.argsPreview }}</span>
              <span class="agent-tool-duration">{{ card.duration }}</span>
            </div>
          </div>
        </div>
        <t-chat-reasoning v-if="item.role === 'assistant'"  expand-icon-placement="right">
          <t-chat-loading v-if="isStreamLoad && !item.content" text="思考中..." />
          <t-chat-content v-if="item.reasoning.length > 0" :content="item.reasoning" />
        </t-chat-reasoning>
        <div v-if="item.role === 'assistant' && item.jsonMarkdown" class="agent-json-md">
          <div class="agent-json-md-header" @click="toggleJsonMd(index)">
            <svg :class="['agent-json-md-arrow', { 'agent-json-md-arrow-expanded': jsonMdExpandedMap[index] }]" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
            <span class="agent-json-md-title">📊 分析报告</span>
          </div>
          <div v-show="jsonMdExpandedMap[index]" class="agent-json-md-content">
            <t-chat-content :content="item.jsonMarkdown" />
          </div>
        </div>
        <t-chat-content v-if="item.content.length > 0" :content="item.content" />
      </template>
      <template #actions="{ item, index }">
        <t-chat-action
            :content="item.content"
            :operation-btn="['copy']"
            @operation="handleOperation"
        />
        <!-- 重新生成（最后一轮回答）/ 编辑重发（最后一轮提问）：流式进行中隐藏；欢迎语不显示 -->
        <t-button v-if="item.role === 'assistant' && index === 0 && !isStreamLoad && item.content && chatList[1] && chatList[1].role === 'user'"
                  size="small" variant="text" title="撤回本轮回答并重新生成" @click="resetLastTurn(false)">
          🔄 重新生成
        </t-button>
        <t-button v-if="item.role === 'user' && index === 1 && !isStreamLoad"
                  size="small" variant="text" title="修改问题并重新发送" @click="resetLastTurn(true)">
          ✏️ 编辑
        </t-button>
        <span v-if="item.role === 'assistant' && item.stats" class="turn-stats"
              :title="`本轮消耗：输入 ${item.stats.inputTokens.toLocaleString()} tokens，输出 ${item.stats.outputTokens.toLocaleString()} tokens`">
          📊 {{ formatTokens(item.stats.inputTokens) }} 入 / {{ formatTokens(item.stats.outputTokens) }} 出
          / {{ formatTokens(item.stats.inputTokens + item.stats.outputTokens) }} 总 tokens
          · {{ item.stats.tools }} 次工具 · {{ item.stats.duration }}
        </span>
        <span v-if="item.role === 'assistant' && !item.feedback" class="feedback-btns">
          <t-button size="small" variant="text" title="这个回答有用" @click="submitFeedback(item, 1)">👍</t-button>
          <t-button size="small" variant="text" title="这个回答没用" @click="openFeedbackDialog(item)">👎</t-button>
        </span>
        <span v-else-if="item.role === 'assistant' && item.feedback" class="feedback-done">{{ item.feedback === 1 ? '👍' : '👎' }}</span>
      </template>
      <template #footer>
<!--        <t-chat-input :stop-disabled="isStreamLoad" @send="inputEnter" @stop="onStop"> </t-chat-input>-->
          <t-chat-sender
              ref="chatSenderRef"
              v-model="inputValue"
              class="chat-sender"
              :textarea-props="{
                placeholder: '请输入消息...',
              }"
              :loading="loading"
              :stop-disabled="isStreamLoad"
              @send="inputEnter"
              @stop="onStop"
          >
            <template #suffix>
              <!-- 监听键盘回车发送事件需要在sender组件监听 -->
              <t-button theme="default" variant="text" size="large" class="btn" @click="inputEnter"> 发送 </t-button>
            </template>
            <template #prefix>
              <NFlex>
                <NSelect
                    v-model:value="selectValue"
                    :options="selectOptions"
                    label-field="name" value-field="ID"
                    size="tiny"
                    style="width: 200px;"
                />
                <NSelect
                    v-model:value="agentMode"
                    :options="agentModeOptions"
                    size="tiny"
                    style="width: 120px;"
                />
              </NFlex>
            </template>
          </t-chat-sender>

      </template>
    </t-chat>
    <t-button v-show="isShowToBottom" variant="text" class="bottomBtn" @click="backBottom">
      <div class="to-bottom">
        <ArrowDownIcon />
      </div>
    </t-button>

    <!-- 👎 反馈理由弹窗：采集纠正原因，供画像学习"需规避项/偏好格式" -->
    <n-modal
      v-model:show="feedbackDialogShow"
      preset="dialog"
      title="这个回答哪里不行？"
      positive-text="提交反馈"
      negative-text="跳过"
      @positive-click="confirmFeedbackSubmit"
      @negative-click="submitFeedbackSkip"
      @close="submitFeedbackSkip"
    >
      <div style="display:flex; flex-direction:column; gap:8px; text-align:left;">
        <n-select
          v-model:value="feedbackReasonPreset"
          :options="feedbackReasonOptions"
          placeholder="选择主要问题（可选）"
          clearable
          size="small"
        />
        <n-input
          v-model:value="feedbackReasonText"
          type="textarea"
          placeholder="补充说明（可选，例如：没结合我的持仓成本）"
          :rows="2"
          maxlength="200"
          show-count
        />
      </div>
    </n-modal>
  </div>
</template>
<script setup lang="ts">
import {ref, onMounted, h, onBeforeUnmount, onBeforeMount} from 'vue';
import {ArrowDownIcon, CheckCircleIcon, SystemSumIcon} from 'tdesign-icons-vue-next';
const loading = ref(false);

const inputValue = ref('');
// 流式数据加载中
const isStreamLoad = ref(false);
let formatTimer = null

const chatRef = ref(null);
const isShowToBottom = ref(false);

const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');
import {darkTheme, NFlex, NImage,NSelect} from "naive-ui";
import {AbortChatWithAgentBySession, ChatWithAgent, GetAiConfigs, GetConfig, GetSponsorInfo, GetVersionInfo, SubmitAgentFeedback} from "../../wailsjs/go/main/App";
import {models} from '../../wailsjs/go/models';
import {EventsOff, EventsOn} from '../../wailsjs/runtime'
import {stripToolMetaMarkers} from './agent/format.js';
import 'tdesign-vue-next/es/style/index.css';

const AGENT_CHAT_SESSION_ID = `agent-chat-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const allowToolTip = ref(true);
const chatSenderRef = ref(null);
const selectOptions = ref([]);
const selectValue = ref("default");
const agentMode = ref('deepagents')
const agentModeOptions = [
  { label: '🤖 自动', value: 'auto' },
  { label: '⚡ 快速', value: 'react' },
  { label: '🧠 规划', value: 'plan_execute' },
  { label: '🔬 DeepAgents', value: 'deepagents' },
]
const jsonMdExpandedMap = ref({})
const toolCardsExpandedMap = ref({})

// formatTokens 大数值缩写为 k（如 1,234 → 1.2k，12,345 → 12.3k），提升可读性；
// 悬停 title 仍展示精确值。
function formatTokens(n) {
  if (n == null) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

function toggleJsonMd(index) {
  jsonMdExpandedMap.value = {
    ...jsonMdExpandedMap.value,
    [index]: !jsonMdExpandedMap.value[index]
  }
}

// 定义事件处理函数，方便在挂载和卸载时管理
function getStepDotClass(step) {
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

// ---- 工具调用过程卡片 ----
// 后端把工具活动编码进 [STEP] 文本流：🔧 调用工具：name(args) / ✅ name 返回结果（N字）/ ⚠️ ...失败。
// 这里将其解析为结构化卡片并维护状态机：running → success / error / stopped。
// 另兼容部分路径透传的结构化 tool_calls（含 id，参数可能分片到达）。

// ---- 任务清单进度面板（规划模式） ----
// 两种数据源，均为完整快照，新快照整体替换旧清单：
// 1) DeepAgents write_todos：[STEP]📝 任务清单更新（N项：X完成/Y进行中/Z待执行）\n  ✅/🔄/⏳ 各项
// 2) PlanExecute content 清单：模型按提示词约定输出 "- [1] 步骤描述 [pending|in_progress|completed]"

// 解析 [STEP]📝 文本为 todos 数组；非清单文本或解析不出条目时返回 null（走原步骤列表兜底）
function parseTodosSnapshot(text) {
  if (!text) return null
  const trimmed = text.replace(/^📝\s*/, '').trim()
  if (!/^任务清单更新|^任务规划|^计划[:：]/.test(trimmed)) return null
  const lines = trimmed.split('\n').map(l => l.trim()).filter(Boolean)
  const items = []
  for (const line of lines.slice(1)) {
    let m
    if ((m = line.match(/^✅\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'completed' })
    } else if ((m = line.match(/^🔄\s*(.+?)(?:（进行中）)?$/))) {
      items.push({ text: m[1].trim(), status: 'in_progress' })
    } else if ((m = line.match(/^⏳\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'pending' })
    } else if ((m = line.match(/^\d+[.、)）]\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'pending' })
    }
  }
  return items.length > 0 ? items : null
}

// 从 content 累积文本中提取 PlanExecute 提示词约定的任务清单（至少 2 条才认定为清单）
// 提示词要求模型每完成一步重新输出整份清单，累积文本会包含多轮快照；
// 只取最后一个连续清单块（最新快照），否则新旧状态混杂、条目重复，清单看起来"不更新"
function extractPlanChecklist(raw) {
  if (!raw) return null
  const re = /^-\s*\[\d+\]\s*(.+?)\s*\[(pending|in_progress|completed)\]\s*$/
  let latest = null
  let cur = []
  for (const line of raw.split('\n')) {
    const m = line.match(re)
    if (m) {
      cur.push({ text: m[1], status: m[2] })
    } else if (cur.length > 0) {
      if (cur.length >= 2) latest = cur
      cur = []
    }
  }
  if (cur.length >= 2) latest = cur
  return latest
}

function todosProgress(item) {
  const todos = item.todos || []
  const done = todos.filter(t => t.status === 'completed').length
  return `${done}/${todos.length} 完成`
}

function todosRunning(item) {
  return (item.todos || []).some(t => t.status === 'in_progress')
}

// 执行步骤列表只保留阶段/规划类信息；工具开始与完成事件、清单快照已升级为独立面板，不再重复展示
function visibleSteps(item) {
  return (item.steps || []).filter(s => {
    const t = s.trim()
    if (/调用工具[:：]/.test(t) || /✅\s*\S+\s*返回结果/.test(t)) return false
    if (/^📝/.test(t) || /^任务清单更新/.test(t) || /^任务规划|^计划[:：]/.test(t)) return false
    return true
  })
}

function toggleToolCards(index) {
  toolCardsExpandedMap.value = { ...toolCardsExpandedMap.value, [index]: !toolCardsExpandedMap.value[index] }
}

function toolCardsRunning(item) {
  return (item.toolCards || []).some(c => c.status === 'running')
}

function runningToolName(item) {
  const c = (item.toolCards || []).find(c => c.status === 'running')
  return c ? `正在调用 ${c.name}` : ''
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

function autoExpandToolCards(item) {
  if (item === chatList.value[0]) {
    toolCardsExpandedMap.value = { ...toolCardsExpandedMap.value, 0: true }
  }
}

function pushToolCard(item, name, args) {
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
  item.toolCards.push({ name, args, argsPreview: buildArgsPreview(args), status: 'running', startTs: Date.now(), duration: '' })
  autoExpandToolCards(item)
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
function updateToolCardsFromStep(item, text) {
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
      pushToolCard(item, ev.name, args)
    } else {
      completeToolCard(item, ev.name, 'success')
    }
  }
}

function startFormatTimer() {
  stopFormatTimer()
  formatTimer = setInterval(() => {
    const lastItem = chatList.value[0]
    if (lastItem && lastItem.role === 'assistant') {
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }, 1500)
}

function stopFormatTimer() {
  if (formatTimer) {
    clearInterval(formatTimer)
    formatTimer = null
  }
}

function formatMarkdown(content) {
  if (!content) return { content: '', jsonMarkdown: '' }

  const { content: cleaned, jsonMarkdown } = wrapInlineJson(stripToolMetaMarkers(content))

  let inCodeBlock = false
  const lines = cleaned.split('\n')
  const result = []

  for (let i = 0; i < lines.length; i++) {
    let line = lines[i]
    const trimmed = line.replace(/^[\t ]+/, '')

    if (trimmed.startsWith('```')) {
      inCodeBlock = !inCodeBlock
      if (!inCodeBlock) {
        result.push(trimmed)
        continue
      }
    }

    if (inCodeBlock) {
      result.push(line)
      continue
    }

    if (trimmed !== line && trimmed !== '') {
      line = trimmed
    }

    if (i > 0 && isBlockElement(trimmed)) {
      const prev = result.length > 0 ? result[result.length - 1] : ''
      if (prev !== '' && !isBlockElement(prev.replace(/^[\t ]+/, ''))) {
        result.push('')
      }
    }

    line = splitInlineHeading(line)

    result.push(line)
  }

  return {
    content: result.join('\n'),
    jsonMarkdown
  }
}

function hasMarkdownContent(str) {
  if (!str || typeof str !== 'string') return false
  return /(^|\n)\s*#{1,6}\s/.test(str) ||
    /(^|\n)\s*\|/.test(str) ||
    /(^|\n)\s*---/.test(str) ||
    /(^|\n)\s*[-*+]\s/.test(str) ||
    /(^|\n)\s*>\s/.test(str) ||
    /(^|\n)\s*```/.test(str)
}

function extractMarkdownFromJson(obj) {
  if (typeof obj === 'string') return obj
  if (Array.isArray(obj)) {
    const items = obj.map(item => typeof item === 'string' ? item : JSON.stringify(item, null, 2))
    return items.join('\n\n')
  }
  if (typeof obj === 'object' && obj !== null) {
    for (const key of ['response', 'content', 'text', 'result', 'answer', 'message', 'output']) {
      if (obj[key] != null) {
        const val = obj[key]
        if (typeof val === 'string' && hasMarkdownContent(val)) return val
        if (typeof val === 'object') {
          const extracted = extractMarkdownFromJson(val)
          if (extracted) return extracted
        }
      }
    }
    const values = Object.values(obj).filter(v => typeof v === 'string' && hasMarkdownContent(v))
    if (values.length > 0) return values.join('\n\n')
    const strValues = Object.values(obj).filter(v => typeof v === 'string')
    if (strValues.length > 0) return strValues.join('\n\n')
  }
  return null
}

function wrapInlineJson(content) {
  if (!content) return { content: '', jsonMarkdown: '' }
  const cleaned = []
  const jsonParts = []
  let i = 0
  const len = content.length
  let inCodeBlock = false

  while (i < len) {
    if (content.substring(i, i + 3) === '```') {
      inCodeBlock = !inCodeBlock
      cleaned.push('```')
      i += 3
      continue
    }

    if (inCodeBlock) {
      cleaned.push(content[i])
      i++
      continue
    }

    if (content[i] === '{') {
      const end = findJsonEnd(content, i)
      if (end > i) {
        const jsonStr = content.substring(i, end + 1)
        try {
          const obj = JSON.parse(jsonStr)
          const md = extractMarkdownFromJson(obj)
          if (md) {
            jsonParts.push(md)
          } else {
            cleaned.push('\n\n```json\n' + jsonStr + '\n```\n\n')
          }
          i = end + 1
          continue
        } catch {}
      }
    }
    cleaned.push(content[i])
    i++
  }

  return {
    content: cleaned.join(''),
    jsonMarkdown: jsonParts.join('\n\n---\n\n')
  }
}

function findJsonEnd(content, start) {
  let depth = 0
  let bracketDepth = 0
  let inStr = false
  let escape = false
  for (let i = start; i < content.length; i++) {
    const ch = content[i]
    if (escape) { escape = false; continue }
    if (ch === '\\' && inStr) { escape = true; continue }
    if (ch === '"') { inStr = !inStr; continue }
    if (inStr) continue
    if (ch === '[') bracketDepth++
    else if (ch === ']') bracketDepth--
    else if (ch === '{') depth++
    else if (ch === '}') {
      depth--
      if (depth === 0 && bracketDepth === 0) return i
    }
  }
  return -1
}

function splitInlineHeading(line) {
  const match = line.match(/(#{1,6}\s+\S)/)
  if (!match) return line
  const idx = match.index
  if (idx === 0) return line
  const prefix = line.substring(0, idx)
  if (prefix.trim() === '') return line
  return prefix + '\n\n' + line.substring(idx)
}

function isBlockElement(line) {
  if (!line || line.length === 0) return false
  if (line[0] === '#') return true
  if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('+ ')) return true
  if (line.startsWith('```')) return true
  if (line.startsWith('> ')) return true
  if (line.length >= 2 && line[0] >= '1' && line[0] <= '9' && line[1] === '.') return true
  if (line.startsWith('---') || line.startsWith('***') || line.startsWith('___')) return true
  if (line.startsWith('|')) return true
  return false
}

function parseStepText(text) {
  if (!text) return [text]
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return [text]
  try {
    const obj = JSON.parse(trimmed)
    if (Array.isArray(obj)) {
      return obj.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
    }
    if (typeof obj === 'object' && obj !== null) {
      const steps = obj.steps || obj.step || obj.plan || obj.items || obj.list
      if (Array.isArray(steps)) {
        return steps.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
      }
      const entries = Object.entries(obj)
      if (entries.length > 0) {
        return entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
      }
    }
    return [text]
  } catch {
    return [text]
  }
}

const handleAgentMessage = (data) => {
  if (data?.sessionId !== AGENT_CHAT_SESSION_ID) return;
  if (data.content === 'agent-DONE') {
    isStreamLoad.value = false;
    loading.value = false;
    stopFormatTimer();
    const lastItem = chatList.value[0];
    if (lastItem) {
      stopRunningToolCards(lastItem);
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent);
        lastItem.content = fmt.content;
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown;
      }
      if (lastItem.rawReasoning) {
        lastItem.reasoning = formatMarkdown(lastItem.rawReasoning).content;
      }
    }
    return;
  }
  if(data['role']==="assistant"){
    loading.value = false;
    const lastItem = chatList.value[0];
    if (data['reasoning_content']){
      const rc = data['reasoning_content']
      if (rc.startsWith('[STATS]')) {
        // 本轮统计（后端 sendTurnStats 发送）：提取为结构化字段单独展示，
        // 不混入 reasoning 折叠区
        const m = rc.match(/工具调用\s*(\d+)\s*次｜输入\s*(\d+)\s*token｜输出\s*(\d+)\s*token｜耗时\s*([^\n]+)/)
        if (m) {
          lastItem.stats = {
            tools: parseInt(m[1], 10),
            inputTokens: parseInt(m[2], 10),
            outputTokens: parseInt(m[3], 10),
    duration: m[4].trim().replace(/(\d+)(?:\.\d+)?s$/, '$1s'),
          }
        }
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          // 📝 任务清单是完整快照：解析为结构化 todos 面板并整体替换，不进步骤列表
          const todos = parseTodosSnapshot(stepText)
          if (todos) {
            lastItem.todos = todos
            toolCardsExpandedMap.value = { ...toolCardsExpandedMap.value, ['p0']: true }
          } else {
            if (!lastItem.steps) lastItem.steps = []
            const parsed = parseStepText(stepText)
            lastItem.steps.push(...parsed)
            // 同步解析工具调用事件，驱动「工具调用」过程卡片（开始/完成/失败）
            updateToolCardsFromStep(lastItem, stepText)
          }
        }
      } else {
        lastItem.rawReasoning = (lastItem.rawReasoning || '') + rc
        lastItem.reasoning = stripToolMetaMarkers(lastItem.rawReasoning)
      }
    }
    if (data['content']){
      lastItem.rawContent = (lastItem.rawContent || '') + data['content']
      // 流式期间也清洗内部元数据标记，避免 [as_of=...] 等在正文闪现（DONE 时再整体格式化）
      lastItem.content = stripToolMetaMarkers(lastItem.rawContent)
      // PlanExecute 模式：模型在 content 中输出并更新任务清单（- [N] 步骤 [状态]），
      // 从累积文本提取最新快照驱动进度面板
      const plan = extractPlanChecklist(lastItem.rawContent)
      if (plan) lastItem.todos = plan
    }
    if (data['tool_calls'] && data['tool_calls'].length > 0) {
      // 部分后端路径透传结构化 tool_calls（含 id，参数可能分片到达）：按 id 聚合进卡片
      if (!lastItem.toolCards) lastItem.toolCards = []
      for (const tool of data['tool_calls']) {
        const name = tool.function && tool.function.name ? tool.function.name : ''
        if (!name) continue
        let card = tool.id ? lastItem.toolCards.find(c => c.id === tool.id) : null
        if (!card) {
          card = { id: tool.id || '', name, args: '', argsPreview: '', status: 'running', startTs: Date.now(), duration: '' }
          lastItem.toolCards.push(card)
          autoExpandToolCards(lastItem)
        }
        const frag = tool.function && tool.function.arguments ? tool.function.arguments : ''
        if (frag) {
          card.args += frag
          card.argsPreview = buildArgsPreview(card.args)
        }
      }
    }
  }
  if(data['response_meta']&&data['response_meta'].finish_reason==="stop"){
    isStreamLoad.value = false;
    loading.value = false;
    stopFormatTimer()
    const lastItem = chatList.value[0];
    if (lastItem) {
      stopRunningToolCards(lastItem)
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }
}

onBeforeUnmount(() => {
  // 流式进行中离开页面时，formatTimer 不会走 DONE/stop 分支，需在此兜底清理，
  // 否则定时器会持续持有组件闭包（内存泄漏）。
  stopFormatTimer();
  EventsOff("agent-message", handleAgentMessage)
})

onBeforeMount(() => {
  // 每次挂载前都重新注册事件监听
  EventsOn("agent-message", handleAgentMessage)
  GetAiConfigs().then(res=>{
    console.log(res)
    selectOptions.value = res
    selectValue.value = res[0].ID
  })
})

onMounted(() => {
  //chatRef.value.scrollToBottom();

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute("theme-mode", "dark");
    } else {
      document.documentElement.removeAttribute("theme-mode");    }
  })


  GetVersionInfo().then((res) => {
    icon.value = res.icon;
  });

});

// 滚动到底部
const backBottom = () => {
  chatRef.value.scrollToBottom({
    behavior: 'smooth',
  });
};
// 是否显示回到底部按钮
const handleChatScroll = function ({ e }) {
  const scrollTop = e.target.scrollTop;
  isShowToBottom.value = scrollTop < 0;
};
// 清空消息
const clearConfirm = function () {
  chatList.value = [];
};
const handleOperation = function (type, options) {
  console.log('handleOperation', type, options);
};

// 提交对某条回答的反馈（👍 直接提交；👎 先弹理由框，可跳过）
const submitFeedback = function (item, rating, reason = '') {
  if (!item) return;
  const fb = models.AgentFeedback.createFrom({
    sessionId: '',
    question: item.question || '',
    response: item.rawContent || item.content || '',
    rating: rating,
    reason: reason,
    mode: agentMode.value === 'auto' ? '' : agentMode.value,
  });
  SubmitAgentFeedback(fb).then(() => {
    item.feedback = rating;
    window.$message && window.$message.success(rating === 1 ? '感谢反馈，我会继续优化' : '已收到，我会改进');
  }).catch((e) => {
    console.error('submit feedback error', e);
  });
};

// ---- 👎 理由弹窗 ----
const feedbackDialogShow = ref(false);
const feedbackReasonPreset = ref(null);
const feedbackReasonText = ref('');
const feedbackTargetItem = ref(null);
// 预设理由选项：与 user-profile.vue 的 classifyFeedbackReason 分类对应，便于画像学习归类
const feedbackReasonOptions = [
  {label: '数据不准 / 过时', value: '数据不准'},
  {label: '逻辑推理有误', value: '逻辑有误'},
  {label: '太啰嗦 / 格式不佳', value: '太啰嗦'},
  {label: '风险提示不合我的风格', value: '风险偏好不符'},
  {label: '没结合我的持仓 / 关注', value: '没结合我的持仓'},
];

function openFeedbackDialog(item) {
  if (!item) return;
  feedbackTargetItem.value = item;
  feedbackReasonPreset.value = null;
  feedbackReasonText.value = '';
  feedbackDialogShow.value = true;
}

// 拼接预设 + 自由文本
function buildFeedbackReason() {
  return [feedbackReasonPreset.value, feedbackReasonText.value.trim()]
    .filter(Boolean).join('；')
}

function confirmFeedbackSubmit() {
  const item = feedbackTargetItem.value;
  feedbackDialogShow.value = false;
  if (item) submitFeedback(item, -1, buildFeedbackReason());
}

// 跳过：不填理由直接提交 👎
function submitFeedbackSkip() {
  const item = feedbackTargetItem.value;
  feedbackDialogShow.value = false;
  if (item) submitFeedback(item, -1, '');
}
// 倒序渲染
const chatList = ref([
  // {
  //   content: `模型由<span>hunyuan</span>变为<span>GPT4</span>`,
  //   role: 'model-change',
  //   reasoning: '',
  // },
  {
    avatar: h(NImage, { src: icon.value, height: '48px', width: '48px'}),
    name: 'Go-Stock AI',
    datetime: '',
    reasoning: '',
    content: '我是您的AI赋能股票分析助手,您可以问我任何关于股票投资方面的问题。',
    role: 'assistant',
    duration: 10,
  },
  {
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: '',
    content: '介绍下自己？',
    role: 'user',
    reasoning: '',
  },
]);

const onStop = function () {
  if (isStreamLoad.value) {
    AbortChatWithAgentBySession(AGENT_CHAT_SESSION_ID).catch((error) => {
      console.error('abort agent chat error', error);
    });
    loading.value = false;
    isStreamLoad.value = false;
  }
  stopFormatTimer()
  const lastItem = chatList.value[0]
  if (lastItem && lastItem.role === 'assistant') {
    stopRunningToolCards(lastItem)
    if (lastItem.rawContent) {
      const fmt = formatMarkdown(lastItem.rawContent)
      lastItem.content = fmt.content
      if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
    }
    if (lastItem.rawReasoning) {
      const fmt = formatMarkdown(lastItem.rawReasoning)
      lastItem.reasoning = fmt.content
    }
  }
};

const inputEnter = function () {
  if (isStreamLoad.value) {
    return;
  }
  if (!inputValue.value) return;
  const aiConfigId = Number(selectValue.value);
  if (!Number.isInteger(aiConfigId) || aiConfigId <= 0) {
    window.$message?.warning('请先配置并选择 AI 模型');
    return;
  }
  const params = {
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: new Date().toDateString(),
    content: inputValue.value,
    role: 'user',
  };
  chatList.value.unshift(params);
  // 空消息占位
  const params2 = {
    avatar:  h(NImage, { src: icon.value, height: '48px', width: '48px'}),
    name: 'Go-Stock AI',
    datetime: new Date().toDateString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    jsonMarkdown: '',
    toolCards: [],
    todos: [],
    question: inputValue.value,
    feedback: 0,
    stats: null,
    role: 'assistant',
  };
  chatList.value.unshift(params2);
  loading.value = true;
  isStreamLoad.value = true;
  startFormatTimer()
  jsonMdExpandedMap.value = { ...jsonMdExpandedMap.value, [0]: true }
  ChatWithAgent(
    inputValue.value,
    aiConfigId,
    0,
    false,
    0,
    false,
    agentMode.value === 'auto' ? '' : agentMode.value,
    AGENT_CHAT_SESSION_ID,
    '',
    ''
  ).catch((error) => {
    isStreamLoad.value = false;
    loading.value = false;
    stopFormatTimer();
    console.error('start agent chat error', error);
  });
};

// 重新生成 / 编辑重发：撤回最后一轮问答（index 0 为回答、1 为提问）。
// fillInput=true 时把原问题回填输入框由用户修改；false 时直接按原问题重发（对齐主流交互）
function resetLastTurn(fillInput) {
  if (isStreamLoad.value) return;
  const assistantItem = chatList.value[0];
  if (!assistantItem || assistantItem.role !== 'assistant') return;
  const userItem = chatList.value[1] && chatList.value[1].role === 'user' ? chatList.value[1] : null;
  if (!userItem) return;
  const question = userItem.content;
  chatList.value = chatList.value.slice(2);
  inputValue.value = question;
  if (!fillInput) {
    inputEnter();
  }
}
</script>
<style lang="less">
/* 应用滚动条样式 */
::-webkit-scrollbar-thumb {
  background-color: var(--td-scrollbar-color);
}
::-webkit-scrollbar-thumb:horizontal:hover {
  background-color: var(--td-scrollbar-hover-color);
}
::-webkit-scrollbar-track {
  background-color: var(--td-scroll-track-color);
}
.chat-box {
  position: relative;
  height: 100%;
  margin: 5px 10px 5px 10px;
  text-align: left;
  .feedback-btns {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    margin-left: 4px;
    vertical-align: middle;
  }
  .feedback-done {
    margin-left: 6px;
    opacity: 0.7;
    vertical-align: middle;
  }
  .bottomBtn {
    position: absolute;
    left: 50%;
    margin-left: -20px;
    bottom: 210px;
    padding: 0;
    border: 0;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    box-shadow: 0px 8px 10px -5px rgba(0, 0, 0, 0.08), 0px 16px 24px 2px rgba(0, 0, 0, 0.04),
    0px 6px 30px 5px rgba(0, 0, 0, 0.05);
  }
  .to-bottom {
    width: 40px;
    height: 40px;
    border: 1px solid #dcdcdc;
    box-sizing: border-box;
    background: var(--td-bg-color-container);
    border-radius: 50%;
    font-size: 24px;
    line-height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    .t-icon {
      font-size: 24px;
    }
  }
}

.model-select {
  display: flex;
  align-items: center;
  .t-select {
    width: 112px;
    height: 32px;
    margin-right: 8px;
    .t-input {
      border-radius: 32px;
      padding: 0 15px;
    }
  }
  .check-box {
    width: 112px;
    height: 32px;
    border-radius: 32px;
    border: 0;
    background: #e7e7e7;
    color: rgba(0, 0, 0, 0.9);
    box-sizing: border-box;
    flex: 0 0 auto;
    .t-button__text {
      display: flex;
      align-items: center;
      justify-content: center;
      span {
        margin-left: 4px;
      }
    }
  }
  .check-box.is-active {
    border: 1px solid #d9e1ff;
    background: #f2f3ff;
    color: var(--td-brand-color);
  }
}


.chat-sender {
  .btn {
    color: var(--td-text-color-disabled);
    border: none;
    &:hover {
      color: var(--td-brand-color-hover);
      border: none;
      background: none;
    }
  }
  .btn.t-button {
    height: var(--td-comp-size-m);
    padding: 0;
  }
  .model-select {
    display: flex;
    align-items: center;
    .t-select {
      width: 112px;
      height: var(--td-comp-size-m);
      margin-right: var(--td-comp-margin-s);
      .t-input {
        border-radius: 32px;
        padding: 0 15px;
      }
      .t-input.t-is-focused {
        box-shadow: none;
      }
    }
    .check-box {
      width: 112px;
      height: var(--td-comp-size-m);
      border-radius: 32px;
      border: 0;
      background: var(--td-bg-color-component);
      color: var(--td-text-color-primary);
      box-sizing: border-box;
      flex: 0 0 auto;
      .t-button__text {
        display: flex;
        align-items: center;
        justify-content: center;
        span {
          margin-left: var(--td-comp-margin-xs);
        }
      }
    }
    .check-box.is-active {
      border: 1px solid var(--td-brand-color-focus);
      background: var(--td-brand-color-light);
      color: var(--td-text-color-brand);
    }
  }
}

.agent-steps {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.turn-stats {
  display: inline-flex;
  align-items: center;
  margin: 0 8px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  white-space: nowrap;
  cursor: default;
}
.agent-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.06) 0%, rgba(46, 139, 87, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
}
.agent-steps-badge {
  font-size: 11px;
  background: var(--td-brand-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.agent-steps-list {
  padding: 8px 10px 8px 14px;
  max-height: 250px;
  overflow-y: auto;
}
.agent-step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  position: relative;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}
.agent-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 16px;
  bottom: -3px;
  width: 1px;
  background: var(--td-component-border);
}
.agent-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--td-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.agent-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.agent-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.agent-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.agent-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}

.agent-json-md {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.06) 0%, rgba(5, 150, 105, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
  transition: background 0.2s;
}
.agent-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.12) 0%, rgba(5, 150, 105, 0.12) 100%);
}
.agent-json-md-arrow {
  transition: transform 0.2s;
  flex-shrink: 0;
}
.agent-json-md-arrow-expanded {
  transform: rotate(90deg);
}
.agent-json-md-title {
  font-size: 13px;
  font-weight: 500;
}
.agent-json-md-content {
  padding: 10px;
  max-height: 500px;
  overflow-y: auto;
  text-align: left;
}

/* 工具调用过程卡片 */
.agent-tool-cards {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-tool-cards-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.06) 0%, rgba(56, 173, 169, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
  transition: background 0.2s;
}
.agent-tool-cards-header:hover {
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.12) 0%, rgba(56, 173, 169, 0.12) 100%);
}
.agent-tool-cards-title {
  font-size: 13px;
  font-weight: 500;
}
.agent-tool-cards-badge {
  font-size: 11px;
  background: #409eff;
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.agent-tool-cards-live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 400;
  color: #e6a23c;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.agent-tool-cards-done {
  font-size: 12px;
  font-weight: 400;
  color: var(--td-text-color-placeholder);
}
.agent-tool-cards-list {
  padding: 6px 10px;
  max-height: 260px;
  overflow-y: auto;
}
.agent-tool-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
  line-height: 1.5;
}
.agent-tool-card + .agent-tool-card {
  border-top: 1px dashed var(--td-component-border);
}
.agent-tool-status {
  width: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.agent-tool-status.tool-success {
  color: #67c23a;
}
.agent-tool-status.tool-error {
  color: #f56c6c;
}
.agent-tool-status.tool-stopped {
  color: var(--td-text-color-disabled);
}
.agent-tool-name {
  font-weight: 600;
  color: var(--td-text-color-primary);
  font-family: SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  flex-shrink: 0;
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.agent-tool-args {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  font-family: SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  text-align: left;
}
.agent-tool-duration {
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  flex-shrink: 0;
}
.agent-tool-spinner {
  width: 10px;
  height: 10px;
  border: 2px solid rgba(230, 162, 60, 0.25);
  border-top-color: #e6a23c;
  border-radius: 50%;
  display: inline-block;
  animation: agent-tool-spin 0.8s linear infinite;
}
@keyframes agent-tool-spin {
  to { transform: rotate(360deg); }
}

/* 任务清单进度面板（规划模式） */
.agent-todos {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-todos-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.08) 0%, rgba(46, 139, 87, 0.08) 100%);
  border-bottom: 1px solid var(--td-component-border);
  transition: background 0.2s;
}
.agent-todos-header:hover {
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.14) 0%, rgba(46, 139, 87, 0.14) 100%);
}
.agent-todos-title {
  font-size: 13px;
  font-weight: 500;
}
.agent-todos-progress {
  font-size: 11px;
  color: #67c23a;
  background: rgba(103, 194, 58, 0.12);
  border-radius: 10px;
  padding: 0 8px;
  line-height: 18px;
  white-space: nowrap;
}
.agent-todos-list {
  padding: 6px 10px;
  max-height: 260px;
  overflow-y: auto;
}
.agent-todo-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  font-size: 12px;
  line-height: 1.6;
}
.agent-todo-icon {
  flex-shrink: 0;
  width: 16px;
  text-align: center;
}
.agent-todo-text {
  flex: 1;
  min-width: 0;
  color: var(--td-text-color-secondary);
  word-break: break-word;
  text-align: left;
}
.agent-todo-item.todo-in_progress .agent-todo-text {
  color: #e6a23c;
  font-weight: 500;
}
.agent-todo-item.todo-completed .agent-todo-text {
  color: var(--td-text-color-disabled);
  text-decoration: line-through;
}

</style>