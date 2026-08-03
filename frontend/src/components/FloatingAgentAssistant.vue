<template>
  <Transition name="fade">
    <div
      v-if="showButton"
      :class="['edge-trigger', { 'edge-trigger-busy': hasBackgroundTask }]"
      @click="togglePanel"
      :title="hasBackgroundTask ? 'go-stock AI Agent 助手正在后台分析...' : 'go-stock AI Agent 助手'"
    >
      <div class="edge-trigger-inner">
        <NIcon :component="SparklesOutline" size="18" />
        <span class="edge-trigger-text">AI助手</span>
        <div v-if="hasBackgroundTask" class="edge-trigger-badge" />
      </div>
    </div>
  </Transition>

  <Transition name="drawer-slide">
    <div v-if="panelVisible" class="drawer-wrap">
      <div class="drawer-mask" @click="closePanel" />
      <div class="drawer-panel" @click.stop>
        <NCard
          size="small"
          class="panel-card"
          :bordered="false"
          content-style="padding: 0; display: flex; flex-direction: column; min-height: 0; overflow: hidden;"
        >
          <template #header>
            <div class="panel-header">
              <span class="panel-title">go-stock AI Agent 助手</span>
              <div class="panel-actions">
                <NButton size="small" quaternary @click="startNewChat" title="开始新对话">
                  新对话
                </NButton>
                <NButton quaternary circle size="small" title="分享到社区" :loading="shareLoading" @click="shareAiToCommunity">
                  <template #icon>
                    <NIcon :component="ShareSocialOutline" />
                  </template>
                </NButton>
                <NButton quaternary circle size="small" title="关闭" @click="closePanel">
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
          </template>

            <div class="chat-body">
            <Transition name="hint-fade">
              <div v-if="hintVisible" class="hint-bar">{{ hintText }}</div>
            </Transition>
            <div v-if="shareTipVisible" class="share-tip">
              <div class="share-tip-text">{{ shareTipText }}</div>
              <NButton size="tiny" quaternary class="share-tip-close" @click="shareTipVisible = false">关闭</NButton>
            </div>
            <NScrollbar ref="scrollbarRef" class="chat-scroll">
              <div class="message-list">
                <div
                  v-for="(group, groupIndex) in messageGroups"
                  :key="group.id"
                  class="message-group"
                >
                  <div class="message-group-header" @click="toggleGroup(groupIndex)">
                    <div class="message-group-summary">
                      <NIcon :component="isGroupExpanded(groupIndex) ? ChevronDownOutline : ChevronForwardOutline" size="16" />
                      <span class="message-group-title">{{ group.userMsg.content.slice(0, 50) }}{{ group.userMsg.content.length > 50 ? '...' : '' }}</span>
                      <span class="message-group-time">{{ group.userMsg.time }}</span>
                    </div>
                  </div>
                  <div v-show="isGroupExpanded(groupIndex)" class="message-group-content">
                    <div
                      :class="['message-item', group.userMsg.role]"
                    >
                      <div class="msg-avatar user-avatar">
                        <NIcon :component="PersonCircleOutline" size="18" />
                      </div>
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <div v-if="group.userMsg.time" class="msg-meta msg-meta-user-inner">
                            <span class="msg-time">{{ group.userMsg.time }}</span>
                          </div>
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'right' }"
                            v-if="group.userMsg.content"
                            :model-value="group.userMsg.content"
                            :editor-id="'agent-msg-' + group.userIndex"
                            class="msg-markdown"
                          />
                        </div>
                      </div>
                    </div>
                    <div
                      v-if="group.assistantMsg"
                      :class="['message-item', 'assistant']"
                    >
                      <div class="msg-avatar assistant-avatar">
                        <NIcon :component="SparklesOutline" size="20" />
                      </div>
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <div v-if="group.assistantMsg.steps && group.assistantMsg.steps.length > 0" class="msg-steps-wrapper">
                            <div class="msg-steps-header" @click="toggleReasoning(group.assistantIndex)">
                              <NIcon :component="reasoningExpandedMap[group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                              <span class="msg-steps-title">📋 执行步骤</span>
                              <span class="msg-steps-count">{{ group.assistantMsg.steps.length }}</span>
                            </div>
                            <div v-show="reasoningExpandedMap[group.assistantIndex]" class="msg-steps-content">
                              <div v-for="(step, si) in group.assistantMsg.steps" :key="si" class="msg-step-item">
                                <div class="msg-step-dot" :class="getStepDotClass(step)"></div>
                                <span class="msg-step-text">{{ step }}</span>
                              </div>
                            </div>
                          </div>
                          <div v-if="group.assistantMsg.reasoning" class="msg-reasoning-wrapper">
                            <div class="msg-reasoning-header" @click="toggleReasoning('r-' + group.assistantIndex)">
                              <NIcon :component="reasoningExpandedMap['r-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                              <span class="msg-reasoning-title">💭 思考过程</span>
                            </div>
                            <div v-show="reasoningExpandedMap['r-' + group.assistantIndex]" class="msg-reasoning-content">
                              <MdPreview
                                :theme="theme"
                                :style="{ textAlign: 'left' }"
                                :model-value="group.assistantMsg.reasoning"
                                :editor-id="'agent-reasoning-' + group.assistantIndex"
                                class="msg-markdown"
                              />
                            </div>
                          </div>
                          <div v-if="group.assistantMsg.jsonMarkdown" class="msg-json-md-wrapper">
                            <div class="msg-json-md-header" @click="toggleReasoning('j-' + group.assistantIndex)">
                              <NIcon :component="reasoningExpandedMap['j-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                              <span class="msg-json-md-title">📊 分析报告</span>
                            </div>
                            <div v-show="reasoningExpandedMap['j-' + group.assistantIndex]" class="msg-json-md-content">
                              <MdPreview
                                :theme="theme"
                                :style="{ textAlign: 'left' }"
                                :model-value="group.assistantMsg.jsonMarkdown"
                                :editor-id="'agent-json-md-' + group.assistantIndex"
                                class="msg-markdown"
                                @onHtmlChanged="onMdHtmlChanged"
                              />
                            </div>
                          </div>
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'left' }"
                            :model-value="group.assistantMsg.content || '...'"
                            :editor-id="'agent-msg-' + group.assistantIndex"
                            class="msg-markdown"
                            @onHtmlChanged="onMdHtmlChanged"
                          />
                          <div v-if="isStreamLoad && groupIndex === messageGroups.length - 1 && !group.assistantMsg.content" class="msg-loading">
                            <NSpin size="small" />
                            <span>思考中...</span>
                          </div>
                          <div class="msg-bubble-actions">
                            <div v-if="group.assistantMsg.modelName || group.assistantMsg.time" class="msg-meta-row-assistant">
                              <span v-if="group.assistantMsg.modelName" class="msg-model-name" :title="group.assistantMsg.modelName">{{ group.assistantMsg.modelName }}</span>
                              <span v-if="group.assistantMsg.time" class="msg-time">{{ group.assistantMsg.time }}</span>
                            </div>
                            <NButton quaternary size="tiny" class="msg-toggle-btn" @click="toggleGroup(groupIndex)">
                              <template #icon>
                                <NIcon :component="isGroupExpanded(groupIndex) ? ChevronUpOutline : ChevronDownOutline" />
                              </template>
                              {{ isGroupExpanded(groupIndex) ? '收起' : '展开' }}
                            </NButton>
                            <NButton quaternary size="tiny" class="msg-copy-btn" @click="copyAiContent(group.assistantMsg)">
                              <template #icon>
                                <NIcon :component="CopyOutline" />
                              </template>
                              复制
                            </NButton>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-export-img-btn"
                              :loading="exportImageKey === String(group.assistantIndex)"
                              title="导出为图片"
                              @click="exportAiReplyImage(group.assistantIndex, $event)"
                            >
                              <template #icon>
                                <NIcon :component="ImageOutline" />
                              </template>
                              导出图
                            </NButton>
                            <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="shareAiContent(group.assistantMsg)">
                              <template #icon>
                                <NIcon :component="ShareSocialOutline" />
                              </template>
                              分享
                            </NButton>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </NScrollbar>
            </div>

            <div class="chat-footer">
              <div class="chat-footer-row">
                <NSelect
                  v-model:value="aiConfigId"
                  :options="aiConfigOptions"
                  size="small"
                  filterable
                  to="body"
                  placement="top-start"
                  placeholder="选择模型"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-select"
                />
                <NSelect
                  v-model:value="sysPromptId"
                  :options="sysPromptOptions"
                  size="small"
                  clearable
                  to="body"
                  placement="top-start"
                  placeholder="系统提示词"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-prompt"
                />
                <NSelect
                  v-model:value="userPromptId"
                  :options="userPromptOptions"
                  size="small"
                  clearable
                  to="body"
                  placement="top-start"
                  placeholder="用户提示词"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-prompt"
                  @update:value="onUserPromptChange"
                />
                <div class="chat-footer-thinking">
                  <span class="chat-footer-thinking-label">思考模式</span>
                  <NSwitch v-model:value="thinkingMode" size="small" />
                </div>
                <div class="chat-footer-memory">
                  <span class="chat-footer-thinking-label">记忆模式</span>
                  <NSwitch v-model:value="memoryMode" size="small" />
                  <NSelect
                    v-if="memoryMode"
                    v-model:value="memoryCount"
                    :options="memoryCountOptions"
                    size="small"
                    :consistent-menu-width="false"
                    to="body"
                    placement="top-start"
                    :menu-props="{ style: { zIndex: 10002 } }"
                    class="chat-footer-memory-count"
                  />
                </div>
                <div class="chat-footer-agent-mode">
                  <NSelect
                    v-model:value="agentMode"
                    :options="agentModeOptions"
                    size="small"
                    to="body"
                    placement="top-start"
                    placeholder="Agent模式"
                    :consistent-menu-width="false"
                    :menu-props="{ style: { zIndex: 10002 } }"
                    class="chat-footer-agent-mode-select"
                  />
                </div>
              </div>
              <div class="chat-footer-input">
                <NInput
                  v-model:value="inputValue"
                  type="textarea"
                  placeholder="输入消息，回车发送..."
                  :autosize="{ minRows: 2, maxRows: 4 }"
                  :disabled="isStreamLoad"
                  @keydown.enter.exact.prevent="sendMessage"
                />
                <NButton
                  v-if="isStreamLoad"
                  type="warning"
                  quaternary
                  class="chat-footer-abort"
                  @click="abortStream(true)"
                >
                  中断
                </NButton>
                <NButton
                  type="primary"
                  :loading="isStreamLoad"
                  :disabled="isStreamLoad || !canSend"
                  @click="sendMessage"
                >
                  发送
                </NButton>
              </div>
            </div>
        </NCard>
      </div>
    </div>
  </Transition>

  <NModal
    v-model:show="klineModalShow"
    :title="(klineName || klineCode || '') + ' — 多周期K线'"
    preset="card"
    :z-index="10010"
    style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
    :content-style="{
      maxHeight: 'min(85vh, 820px)',
      overflowY: 'auto',
      overflowX: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
  >
    <StockLightweightKlineChart
      v-if="klineModalShow"
      :key="'agent-kline-' + klineCode"
      :code="klineCode"
      :stock-name="klineName"
      :dark-theme="darkTheme"
      :chart-height="500"
    />
  </NModal>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, onBeforeMount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NIcon, NInput, NModal, NScrollbar, NSelect, NSpin, NSwitch, useMessage } from 'naive-ui'
import {
  CloseOutline,
  SparklesOutline,
  PersonCircleOutline,
  CopyOutline,
  ShareSocialOutline,
  ImageOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ChevronUpOutline
} from '@vicons/ionicons5'
import {
  ChatWithAgent,
  GetAiConfigs,
  GetConfig,
  GetFollowList,
  GetPromptTemplates,
  GetSponsorInfo,
  SaveAiAssistantSession,
  GetAiAssistantSession,
  ShareText,
  AbortChatWithAgent,
  SaveAIResponseResult,
  SaveImage
} from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import html2canvas from 'html2canvas'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'

const STORAGE_KEY_MODEL_ID = 'go-stock-agent-last-model-id'
const STORAGE_KEY_SYS_PROMPT_ID = 'go-stock-agent-last-sys-prompt-id'
const STORAGE_KEY_USER_PROMPT_ID = 'go-stock-agent-last-user-prompt-id'
const STORAGE_KEY_THINKING_MODE = 'go-stock-agent-thinking-mode'
const STORAGE_KEY_MEMORY_MODE = 'go-stock-agent-memory-mode'
const STORAGE_KEY_MEMORY_COUNT = 'go-stock-agent-memory-count'
const STORAGE_KEY_AGENT_MODE = 'go-stock-agent-mode'

// 从 localStorage 读取布尔值，默认 fallback
function loadBool(key, fallback) {
  const v = localStorage.getItem(key)
  if (v === null) return fallback
  return v === 'true'
}
function loadNum(key, fallback) {
  const v = localStorage.getItem(key)
  if (v === null || v === '') return fallback
  const n = Number(v)
  return Number.isNaN(n) ? fallback : n
}
// 校验缓存的 select 值是否在 options 中有效
function validateOption(value, options) {
  if (value == null) return null
  const isValid = options.some(o => o.value === value)
  return isValid ? value : null
}

const route = useRoute()
const message = useMessage()

const showButton = computed(() => route.name !== 'agent')

const panelVisible = ref(false)
const inputValue = ref('')
const isStreamLoad = ref(false)
const sentFromFloating = ref(false)
const messages = ref([])
let formatTimer = null
const sessionId = ref('')
const aiConfigOptions = ref([])
const aiConfigId = ref(null)

function modelLabelForConfig(configId) {
  const opts = aiConfigOptions.value
  if (!opts?.length) return ''
  const id = configId != null ? Number(configId) : Number(opts[0].value)
  const found = opts.find(o => Number(o.value) === id)
  return found?.label != null ? String(found.label) : ''
}

const sysPromptTemplates = ref([])
const sysPromptOptions = computed(() =>
  sysPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const sysPromptId = ref(null)

const userPromptTemplates = ref([])
const userPromptOptions = computed(() =>
  userPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const userPromptId = ref(null)
const thinkingMode = ref(loadBool(STORAGE_KEY_THINKING_MODE, true))
const memoryMode = ref(loadBool(STORAGE_KEY_MEMORY_MODE, false))
const memoryCount = ref(loadNum(STORAGE_KEY_MEMORY_COUNT, 1))
const memoryCountOptions = [
  { label: '1 条', value: 1 },
  { label: '2 条', value: 2 },
  { label: '3 条', value: 3 },
  { label: '4 条', value: 4 },
  { label: '5 条', value: 5 },
  { label: '10 条', value: 10 },
]
const agentMode = ref(localStorage.getItem(STORAGE_KEY_AGENT_MODE) || 'plan_execute')
const agentModeOptions = [
  { label: '🤖 自动选择', value: 'auto' },
  { label: '⚡ 快速模式', value: 'react' },
  { label: '🧠 规划模式', value: 'plan_execute' },
  { label: '🔬 DeepAgents', value: 'deepagents' },
]

watch(agentMode, (val) => {
  if (val === 'react') showHint('⚡ 快速模式推荐使用DeepSeek最新版')
  else if (val === 'plan_execute') showHint('🧠 规划模式推荐使用GLM最新版')
  else if (val === 'deepagents') showHint('🔬 DeepAgents 模式内置任务规划与子Agent委派，适合复杂多步分析，推荐使用Claude/GLM最新版')
})

watch(aiConfigId, (val) => {
  // 默认使用规划模式，不因模型切换而改变 agentMode
  const label = modelLabelForConfig(val).toLowerCase()
  const labelCompact = label.replace(/[\s_-]/g, '')
  if (label.includes('deepseek-chat')) {
    thinkingMode.value = false
    showHint('deepseek-chat 不支持思考模式已关闭，当前使用规划模式')
  } else if (labelCompact.includes('glm5.1')) {
    thinkingMode.value = true
    showHint('GLM 5.1 已开启思考模式，当前使用规划模式')
  } else if (label.includes('deepseek')) {
    showHint('⚡ DeepSeek 当前使用规划模式')
  } else if (label.includes('glm')) {
    showHint('🧠 GLM 当前使用规划模式')
  }
})

function onUserPromptChange(id) {
  if (!id) return
  const t = userPromptTemplates.value.find(x => (x.ID ?? x.id) === id)
  if (t?.content) inputValue.value = t.content
}

const canSend = computed(() => !!inputValue.value.trim())
const scrollbarRef = ref(null)
const darkTheme = ref(false)
const shareLoading = ref(false)
const exportImageKey = ref('')
const shareTipVisible = ref(false)
const shareTipText = ref('')
const hintVisible = ref(false)
const hintText = ref('')
let hintTimer = null

function showHint(text) {
  hintText.value = text
  hintVisible.value = true
  if (hintTimer) clearTimeout(hintTimer)
  hintTimer = setTimeout(() => { hintVisible.value = false }, 3000)
}
const vipLevel = ref(0)
const vipLoaded = ref(false)
const vipLoading = ref(false)
const isAborted = ref(false)
const expandedGroups = ref(new Set())
const reasoningExpandedMap = ref({})

const hasBackgroundTask = computed(() => isStreamLoad.value && sentFromFloating.value && !panelVisible.value)
const AGENT_EVENT = 'agent-message'

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

function getStepDotClass(step) {
  if (step.includes('🎯')) return 'step-skill'
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('📝')) return 'step-todos'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

function onMdHtmlChanged() {
  nextTick(() => {
    document.querySelectorAll('.msg-markdown .md-editor-code-block').forEach(block => {
      if (block.querySelector('.code-collapse-btn')) return
      const codeEl = block.querySelector('code')
      if (!codeEl) return
      const lang = (codeEl.className || '').toLowerCase()
      const isJson = lang.includes('json') || lang.includes('language-json')
      const text = codeEl.textContent || ''
      const lineCount = text.split('\n').length
      if (!isJson && lineCount <= 8) return

      block.classList.add('code-collapsed')
      const btn = document.createElement('span')
      btn.className = 'code-collapse-btn'
      btn.textContent = '展开'
      btn.addEventListener('click', (e) => {
        e.stopPropagation()
        const collapsed = block.classList.toggle('code-collapsed')
        btn.textContent = collapsed ? '展开' : '收起'
      })
      block.appendChild(btn)
    })
    linkifyStocksInPreview()
  })
}

// ===== 股票代码/名称识别与可点击链接 =====
const klineModalShow = ref(false)
const klineCode = ref('')
const klineName = ref('')
/** 自选股票 名称 → 内部代码 映射，用于 AI 输出中识别股票名称 */
const followListNameMap = ref(new Map())

// 匹配股票代码：带显式前缀/后缀的代码（高置信度）+ 6位 A 股代码（首位 6/0/3/8/9）
// 注意：\d{6}\.(?:SH|SZ|BJ) 必须排在 [60389]\d{5} 之前，否则会先匹配纯数字部分
const STOCK_CODE_REGEX = /\b(?:(?:sh|sz|bj)\d{6}|\d{6}\.(?:SH|SZ|BJ)|hk\d{4,5}|\d{4,5}\.HK|gb_[a-zA-Z]{1,6}|[A-Z]{1,6}\.US|\d{4,6}\.CSI|100\.[A-Z]+|[60389]\d{5})\b/g

/** 将各类股票代码归一化为东方财富格式（如 600519.SH / 00700.HK / AAPL.US），与 stock.vue 一致 */
function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (/\.(SH|SZ|BJ|HK|US|SS|CSI)$/i.test(c)) return c.toUpperCase()
  if (/^100\.[A-Za-z]+$/.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

/** 从正则匹配的字符串中提取用于 toEastMoneyCode 的输入 */
function parseStockCodeMatch(matched) {
  return matched.trim()
}

/** 根据代码反查股票名称（来自自选列表） */
function nameForCode(code) {
  for (const [name, fc] of followListNameMap.value) {
    if (fc === code) return name
  }
  return ''
}

/** 加载自选列表，构建 名称 → 代码 映射，用于识别 AI 输出中的股票名称 */
async function loadFollowListForLinks() {
  try {
    const list = await GetFollowList(0)
    const map = new Map()
    ;(list || []).forEach(item => {
      const name = item.StockName || item.stockName || ''
      const code = item.StockCode || item.stockCode || ''
      if (name && code && name.length >= 2) {
        map.set(name, code)
      }
    })
    followListNameMap.value = map
    // 自选列表加载完成后，对已渲染的消息补做一次股票名称链接
    nextTick(() => linkifyStocksInPreview())
  } catch (_) {
    // 静默失败
  }
}

/** 打开多周期 K 线模态框 */
function openStockKline(rawCode, name) {
  const em = toEastMoneyCode(rawCode)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  klineCode.value = em
  klineName.value = name || ''
  klineModalShow.value = true
}

/** 扫描 MdPreview 渲染后的文本节点，将股票代码/名称替换为可点击 <a> 标签 */
function linkifyStocksInPreview() {
  const previews = document.querySelectorAll('.msg-markdown .md-editor-preview')
  if (!previews.length) return

  // 基于自选列表构建名称匹配正则
  const names = [...followListNameMap.value.keys()]
  let nameRegex = null
  if (names.length > 0) {
    const escaped = names
      .filter(n => n && n.length >= 2)
      .sort((a, b) => b.length - a.length)
      .map(n => n.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    if (escaped.length > 0) {
      nameRegex = new RegExp(escaped.join('|'), 'g')
    }
  }

  previews.forEach(preview => {
    const walker = document.createTreeWalker(
      preview,
      NodeFilter.SHOW_TEXT,
      {
        acceptNode(node) {
          if (!node.nodeValue || !node.nodeValue.trim()) {
            return NodeFilter.FILTER_REJECT
          }
          let el = node.parentNode
          while (el && el !== preview) {
            const tag = el.tagName ? el.tagName.toLowerCase() : ''
            // 跳过链接、代码块、pre 内的文本
            if (tag === 'a' || tag === 'code' || tag === 'pre' || tag === 'script' || tag === 'style') {
              return NodeFilter.FILTER_REJECT
            }
            // 跳过已注入的 stock-link 内部文本
            if (el.classList && el.classList.contains('stock-link')) {
              return NodeFilter.FILTER_REJECT
            }
            el = el.parentNode
          }
          return NodeFilter.FILTER_ACCEPT
        }
      }
    )

    const textNodes = []
    while (walker.nextNode()) {
      textNodes.push(walker.currentNode)
    }

    for (const textNode of textNodes) {
      linkifyTextNode(textNode, nameRegex)
    }
  })
}

/** 将单个文本节点中的股票代码/名称替换为 <a> 标签 */
function linkifyTextNode(textNode, nameRegex) {
  const text = textNode.nodeValue
  if (!text) return

  const matches = []

  STOCK_CODE_REGEX.lastIndex = 0
  let m
  while ((m = STOCK_CODE_REGEX.exec(text)) !== null) {
    const matched = m[0]
    const code = parseStockCodeMatch(matched)
    matches.push({
      index: m.index,
      length: matched.length,
      text: matched,
      code,
      name: nameForCode(code)
    })
  }

  if (nameRegex) {
    nameRegex.lastIndex = 0
    while ((m = nameRegex.exec(text)) !== null) {
      const matched = m[0]
      const code = followListNameMap.value.get(matched)
      if (code) {
        matches.push({
          index: m.index,
          length: matched.length,
          text: matched,
          code,
          name: matched
        })
      }
    }
  }

  if (matches.length === 0) return

  // 按位置排序，去除重叠（保留先出现的）
  matches.sort((a, b) => a.index - b.index)
  const filtered = []
  let lastEnd = -1
  for (const match of matches) {
    if (match.index >= lastEnd) {
      filtered.push(match)
      lastEnd = match.index + match.length
    }
  }

  // 用 DocumentFragment 替换原文本节点：保留纯文本 + 插入 <a> 链接
  const fragment = document.createDocumentFragment()
  let lastIdx = 0
  for (const match of filtered) {
    if (match.index > lastIdx) {
      fragment.appendChild(document.createTextNode(text.slice(lastIdx, match.index)))
    }
    const a = document.createElement('a')
    a.className = 'stock-link'
    a.textContent = match.text
    a.dataset.code = match.code
    if (match.name) a.dataset.name = match.name
    a.title = '点击查看 ' + match.text + ' K线图'
    a.addEventListener('click', (e) => {
      e.preventDefault()
      e.stopPropagation()
      openStockKline(match.code, match.name)
    })
    fragment.appendChild(a)
    lastIdx = match.index + match.length
  }
  if (lastIdx < text.length) {
    fragment.appendChild(document.createTextNode(text.slice(lastIdx)))
  }

  textNode.parentNode.replaceChild(fragment, textNode)
}

async function copyAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    message.warning('暂无可复制的 AI 正文内容')
    return
  }
  try {
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
      message.success('已复制 AI 回答内容')
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      message.success('已复制 AI 回答内容')
    }
  } catch (e) {
    message.error('复制失败，请手动选择文本')
  }
}

function shareTextToCommunity(text, title) {
  if (shareLoading.value) return
  shareLoading.value = true
  shareTipText.value = '正在分享到社区...'
  shareTipVisible.value = true
  ShareText(text, title)
    .then((msg) => {
      shareTipText.value = msg
      shareTipVisible.value = true
    })
    .catch((err) => {
      shareTipText.value = '分享失败: ' + (err?.message ?? err)
      shareTipVisible.value = true
    })
    .finally(() => {
      shareLoading.value = false
    })
}

function shareAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 正文内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'go-stock AI Agent助手')
}

function getLastAssistantContent() {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m?.role === 'assistant') {
      const text = (m?.content ?? '').trim()
      if (text) return text
    }
  }
  return ''
}

function shareAiToCommunity() {
  const text = getLastAssistantContent()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 回复内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'go-stock AI Agent助手')
}

async function exportAiReplyImage(assistantIndex, evt) {
  const msg = messages.value[assistantIndex]
  if (msg?.role !== 'assistant') return
  if (!(msg.content ?? '').trim()) {
    shareTipText.value = '暂无可导出的 AI 回答内容'
    shareTipVisible.value = true
    return
  }
  const editorId = 'agent-msg-' + assistantIndex
  const bubble = evt?.currentTarget?.closest?.('.msg-bubble')
  const key = String(assistantIndex)
  if (exportImageKey.value) return
  exportImageKey.value = key
  await nextTick()
  try {
    const target = document.getElementById(`${editorId}-preview-wrapper`) ||
      document.getElementById(`${editorId}-preview`) ||
      bubble?.querySelector('.md-editor-preview') ||
      null
    if (!target) {
      shareTipText.value = '未找到预览区域，请展开回答后重试'
      shareTipVisible.value = true
      return
    }
    const savedStyles = []
    const overflowParents = []
    let el = target.parentElement
    while (el && el !== document.body) {
      const style = getComputedStyle(el)
      if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
        savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
        overflowParents.push(el)
        el.style.overflow = 'visible'
        el.style.overflowY = 'visible'
        el.style.height = 'auto'
        el.style.maxHeight = 'none'
      }
      el = el.parentElement
    }
    const savedTargetStyle = { height: target.style.height, maxHeight: target.style.maxHeight, overflow: target.style.overflow, overflowY: target.style.overflowY }
    target.style.height = 'auto'
    target.style.maxHeight = 'none'
    target.style.overflow = 'visible'
    target.style.overflowY = 'visible'
    await nextTick()
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: darkTheme.value ? '#1e1e1e' : '#ffffff'
    })
    target.style.height = savedTargetStyle.height
    target.style.maxHeight = savedTargetStyle.maxHeight
    target.style.overflow = savedTargetStyle.overflow
    target.style.overflowY = savedTargetStyle.overflowY
    savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
      el.style.overflow = overflow
      el.style.overflowY = overflowY
      el.style.height = height
      el.style.maxHeight = maxHeight
    })
    const dataUrl = canvas.toDataURL('image/png')
    const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
    const result = await SaveImage(`go-stock-agent-${safeTime}`, base64)
    if (result && !result.includes('异常') && !result.includes('无法')) {
      shareTipText.value = '已导出为 PNG 图片：' + result
    } else {
      shareTipText.value = result || '导出取消'
    }
    shareTipVisible.value = true
  } catch (e) {
    shareTipText.value = '导出图片失败: ' + (e?.message ?? e)
    shareTipVisible.value = true
  } finally {
    exportImageKey.value = ''
  }
}

function abortStream(showTip = true) {
  if (!isStreamLoad.value) return
  isAborted.value = true
  isStreamLoad.value = false
  stopFormatTimer()
  const last = messages.value[messages.value.length - 1]
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
  if (showTip) {
    shareTipText.value = '已中断本次 AI 回答'
    shareTipVisible.value = true
  }
  AbortChatWithAgent()
}

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))

async function loadHistory() {
  try {
    const resp = await GetAiAssistantSession('')
    if (resp?.sessionId) {
      sessionId.value = resp.sessionId
    }
    const list = resp?.messages
    if (Array.isArray(list) && list.length > 0) {
      messages.value = list.map(m => ({
        role: m.role ?? '',
        content: m.content ?? '',
        time: m.time ?? '',
        modelName: m.modelName ?? '',
        reasoning: m.reasoning ?? '',
        steps: m.steps ?? [],
        jsonMarkdown: m.jsonMarkdown ?? ''
      }))
      nextTick(() => {
        initDefaultExpanded()
      })
    }
  } catch (_) {
  }
}

function saveHistory() {
  if (messages.value.length === 0) return
  const list = messages.value.map(m => ({
    role: m.role,
    content: m.content,
    time: m.time ?? '',
    modelName: m.modelName ?? '',
    reasoning: m.reasoning ?? '',
    steps: m.steps ?? [],
    jsonMarkdown: m.jsonMarkdown ?? ''
  }))
  SaveAiAssistantSession(sessionId.value, list).catch(() => {})
}

function openPanel() {
  panelVisible.value = true
  if (!sessionId.value) {
    sessionId.value = Date.now().toString()
  }
  if (messages.value.length === 0) {
    messages.value = [
      {
        role: 'assistant',
        content: '我是 go-stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？',
        time: new Date().toLocaleString(),
        modelName: '',
        reasoning: ''
      }
    ]
  }
  // 加载自选列表用于 AI 输出中识别股票名称
  loadFollowListForLinks()
  nextTick(() => {
    initDefaultExpanded()
    scrollToBottom()
  })
}

function closePanel() {
  panelVisible.value = false
}

async function ensureVipInfo() {
  if (vipLoaded.value || vipLoading.value) return
  vipLoading.value = true
  try {
    const res = await GetSponsorInfo()
    const lvl = Number(res?.vipLevel ?? 0)
    vipLevel.value = Number.isNaN(lvl) ? 0 : lvl
  } catch (_) {
    vipLevel.value = 0
  } finally {
    vipLoaded.value = true
    vipLoading.value = false
  }
}

async function togglePanel() {
  if (!panelVisible.value) {
    await ensureVipInfo()
    if ((vipLevel.value ?? 0) < 2) {
      message.warning('go-stock AI Agent 助手功能仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
      return
    }
    openPanel()
  } else {
    closePanel()
  }
}

function scrollToBottom() {
  nextTick(() => {
    scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
  })
}

function sendMessage() {
  if (isStreamLoad.value) {
    abortStream(false)
  }
  const text = inputValue.value.trim()
  if (!text) {
    message.warning('请输入你的问题')
    return
  }

  messages.value.push({
    role: 'user',
    content: text,
    time: new Date().toLocaleString(),
    modelName: '',
    reasoning: '',
    steps: []
  })
  const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
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
    jsonMarkdown: ''
  })
  inputValue.value = ''
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
    scrollToBottom()
  })
  ChatWithAgent(text, configId, sysPromptId.value, memoryMode.value, memoryCount.value, thinkingMode.value, agentMode.value === 'auto' ? '' : agentMode.value, sessionId.value)
}

function startNewChat() {
  if (isStreamLoad.value) {
    message.warning('当前有回答正在生成，请先中断或等待完成')
    return
  }
  messages.value = []
  sessionId.value = Date.now().toString()
}

function startFormatTimer() {
  stopFormatTimer()
  formatTimer = setInterval(() => {
    const last = messages.value[messages.value.length - 1]
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

function stopFormatTimer() {
  if (formatTimer) {
    clearInterval(formatTimer)
    formatTimer = null
  }
}

function formatMarkdown(content) {
  if (!content) return { content: '', jsonMarkdown: '' }

  const { content: cleaned, jsonMarkdown } = extractJsonMarkdown(content)

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

function extractJsonMarkdown(content) {
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

function onAgentMessage(msg) {
  if (isAborted.value) return

  if (msg.content === 'agent-DONE' || (msg?.response_meta?.finish_reason === 'stop')) {
    isStreamLoad.value = false
    sentFromFloating.value = false
    isAborted.value = false
    stopFormatTimer()
    const last = messages.value[messages.value.length - 1]
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
    saveHistory()
    nextTick(scrollToBottom)
    if (msg.content === 'agent-DONE' && last && last.role === 'assistant' && last.content) {
      const user = messages.value[messages.value.length - 2]
      SaveAIResponseResult("agent","市场分析", last.content, sessionId.value,user.content, aiConfigId.value)
    }
    return
  }

  const roleLower = String(msg?.role || '').toLowerCase()
  if (roleLower !== 'assistant') {
    return
  }

  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'assistant') {
    if (msg?.reasoning_content) {
      const rc = msg.reasoning_content
      if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!last.steps) last.steps = []
          const parsed = parseStepText(stepText)
          last.steps.push(...parsed)
        }
      } else {
        last.rawReasoning = (last.rawReasoning || '') + rc
        last.reasoning = last.rawReasoning
      }
    }
    if (msg?.content) {
      last.rawContent = (last.rawContent || '') + msg.content
      last.content = last.rawContent
    }
    nextTick(scrollToBottom)
  }
}

function loadPromptTemplates() {
  GetPromptTemplates('', '').then(res => {
    const list = Array.isArray(res) ? res : []
    sysPromptTemplates.value = list.filter(t => t.type === '模型系统Prompt')
    userPromptTemplates.value = list.filter(t => t.type === '模型用户Prompt')
    // 恢复缓存的提示词选择（仅在尚未选择时恢复，避免覆盖用户当前会话的改动）
    if (sysPromptId.value == null) {
      const cachedSys = localStorage.getItem(STORAGE_KEY_SYS_PROMPT_ID)
      if (cachedSys) {
        const id = Number(cachedSys)
        sysPromptId.value = validateOption(id, sysPromptOptions.value)
      }
    }
    if (userPromptId.value == null) {
      const cachedUser = localStorage.getItem(STORAGE_KEY_USER_PROMPT_ID)
      if (cachedUser) {
        const id = Number(cachedUser)
        const valid = validateOption(id, userPromptOptions.value)
        if (valid != null) {
          userPromptId.value = valid
          // 自动填充输入框内容（与 onUserPromptChange 行为一致）
          onUserPromptChange(valid)
        }
      }
    }
  })
}

watch(panelVisible, (v) => {
  if (v) {
    loadPromptTemplates()
    nextTick(scrollToBottom)
  }
})

onBeforeMount(() => {
  GetConfig().then(result => {
    darkTheme.value = result.darkTheme
  })
})

onMounted(() => {
  EventsOn(AGENT_EVENT, onAgentMessage)
  loadHistory()
  GetAiConfigs().then(res => {
    const list = Array.isArray(res) ? res : []
    aiConfigOptions.value = list.map((c, index) => {
      const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index)
      const name = c.name ?? c.Name ?? ''
      const modelName = c.modelName ?? c.ModelName ?? ''
      return {
        label: name + (modelName ? ' [' + modelName + ']' : ''),
        value: id
      }
    })
    if (aiConfigOptions.value.length) {
      const lastModelId = localStorage.getItem(STORAGE_KEY_MODEL_ID)
      if (lastModelId) {
        const foundId = Number(lastModelId)
        const isValid = aiConfigOptions.value.some(opt => opt.value === foundId)
        aiConfigId.value = isValid ? foundId : aiConfigOptions.value[0].value
      } else {
        aiConfigId.value = aiConfigOptions.value[0].value
      }
    }
  })
  loadPromptTemplates()
})

watch(aiConfigId, (newId) => {
  if (newId != null) {
    localStorage.setItem(STORAGE_KEY_MODEL_ID, String(newId))
  }
})

// 持久化其余执行参数，避免用户每次重新选择
watch(sysPromptId, (v) => {
  if (v != null) localStorage.setItem(STORAGE_KEY_SYS_PROMPT_ID, String(v))
})
watch(userPromptId, (v) => {
  if (v != null) localStorage.setItem(STORAGE_KEY_USER_PROMPT_ID, String(v))
})
watch(thinkingMode, (v) => localStorage.setItem(STORAGE_KEY_THINKING_MODE, String(v)))
watch(memoryMode, (v) => localStorage.setItem(STORAGE_KEY_MEMORY_MODE, String(v)))
watch(memoryCount, (v) => localStorage.setItem(STORAGE_KEY_MEMORY_COUNT, String(v)))
watch(agentMode, (v) => {
  if (v) localStorage.setItem(STORAGE_KEY_AGENT_MODE, v)
})

onBeforeUnmount(() => {
  EventsOff(AGENT_EVENT)
})
</script>

<style scoped>
.edge-trigger {
  position: fixed;
  top: 50%;
  right: 0;
  z-index: 9998;
  transform: translateY(-50%);
  width: 32px;
  height: 120px;
  border-radius: 12px 0 0 12px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: -2px 0 12px rgba(102, 126, 234, 0.4);
  transition: width 0.2s ease, box-shadow 0.2s ease;
}
.edge-trigger-busy {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger:hover {
  width: 40px;
  box-shadow: -4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.edge-trigger-text {
  font-size: 14px;
  writing-mode: vertical-rl;
  letter-spacing: 2px;
  line-height: 1;
  white-space: nowrap;
}
.edge-trigger-badge {
  position: absolute;
  top: 6px;
  left: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f97316;
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.9);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
}
.drawer-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 60vw;
  min-width: 320px;
  max-width: calc(100vw - 48px);
  background: var(--n-color-modal);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-card {
  height: 100%;
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.panel-card :deep(.n-card-header) {
  padding: 12px 16px;
  flex-shrink: 0;
}
.panel-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.panel-title {
  font-weight: 600;
  font-size: 16px;
}

.chat-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}
.hint-bar {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 8px 14px;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
  border: 1px solid rgba(102, 126, 234, 0.25);
  font-size: 13px;
  color: var(--n-text-color-2);
  text-align: center;
  line-height: 1.5;
}
.hint-fade-enter-active,
.hint-fade-leave-active {
  transition: opacity 0.3s, transform 0.3s;
}
.hint-fade-enter-from,
.hint-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.share-tip {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
  border: 1px solid var(--n-border-color);
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.share-tip-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  text-align: left;
}
.share-tip-close {
  flex-shrink: 0;
}
.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.chat-scroll :deep(.n-scrollbar-content) {
  min-height: 0;
}
.message-list {
  padding: 12px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-group {
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  overflow: hidden;
  background: var(--n-color-modal);
}
.message-group-header {
  padding: 10px 14px;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.02);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.message-group-header:hover {
  background: rgba(0, 0, 0, 0.04);
}
.message-group-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}
.message-group-title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.message-group-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}
.message-group-content {
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.message-group-content .message-item {
  padding: 0 14px;
}
.message-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}
.message-item.user {
  align-items: flex-end;
}
.msg-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.assistant-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}
.user-avatar {
  background: linear-gradient(135deg, #34d399 0%, #22c55e 35%, #06b6d4 100%);
  color: #fff;
  box-shadow: 0 6px 14px rgba(34, 197, 94, 0.22);
  border: 1px solid rgba(255, 255, 255, 0.45);
}
.msg-bubble {
  max-width: 100%;
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  display: flex;
  flex-direction: column;
}
.message-item.assistant .msg-bubble {
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
}
.message-item.user .msg-bubble {
  background: var(--n-color-primary);
  color: #fff;
  text-align: right;
}
.message-item.user .msg-content,
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  text-align: right;
}
.msg-content {
  white-space: normal;
  width: 100%;
  min-width: 0;
  flex: 1;
}
.msg-reasoning-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.08) 0%, rgba(46, 139, 87, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-steps-header:hover {
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.14) 0%, rgba(46, 139, 87, 0.14) 100%);
}
.msg-steps-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-steps-count {
  font-size: 11px;
  background: var(--n-primary-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.msg-steps-content {
  padding: 10px 12px 10px 16px;
  max-height: 300px;
  overflow-y: auto;
}
.msg-step-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 4px 0;
  position: relative;
  font-size: 12px;
  color: var(--n-text-color-2);
  line-height: 1.5;
}
.msg-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 18px;
  bottom: -4px;
  width: 1px;
  background: var(--n-border-color);
}
.msg-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--n-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.msg-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.msg-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.msg-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.msg-step-dot.step-skill {
  background: #9c27b0;
  box-shadow: 0 0 6px rgba(156, 39, 176, 0.6);
}
/* 技能激活步骤的文字高亮（紫色加粗，与 dot 颜色呼应） */
.msg-step-dot.step-skill + .msg-step-text {
  color: #9c27b0;
  font-weight: 600;
}
.msg-step-dot.step-todos {
  background: #009688;
  box-shadow: 0 0 6px rgba(0, 150, 136, 0.5);
}
/* 任务清单更新步骤的文字高亮（青色加粗） */
.msg-step-dot.step-todos + .msg-step-text {
  color: #009688;
  font-weight: 600;
}
.msg-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}
.msg-reasoning-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.08) 0%, rgba(118, 75, 162, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-reasoning-header:hover {
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
}
.msg-reasoning-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-reasoning-content {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  padding: 12px;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-json-md-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.08) 0%, rgba(5, 150, 105, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.14) 0%, rgba(5, 150, 105, 0.14) 100%);
}
.msg-json-md-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-json-md-content {
  padding: 12px;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-reasoning {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  background: var(--n-color-hover);
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
  border-left: 3px solid var(--n-primary-color);
}
.msg-bubble-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  margin-top: 8px;
}
.msg-meta-row-assistant {
  flex: 1 1 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--n-text-color-3);
}
.msg-meta-row-assistant .msg-time {
  flex-shrink: 0;
}
.msg-model-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}
.msg-share-btn,
.msg-copy-btn,
.msg-export-img-btn,
.msg-toggle-btn {
  padding: 2px 10px;
  font-size: 12px;
  border-radius: 12px;
  color: var(--n-primary-color);
  background-color: var(--n-primary-color-suppl);
  border: 1px solid var(--n-primary-color);
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;
}
.msg-share-btn:hover,
.msg-copy-btn:hover,
.msg-export-img-btn:hover,
.msg-toggle-btn:hover {
  border-color: var(--n-primary-color);
  background-color: var(--n-primary-color);
  color: #fff;
}
.message-item.user .msg-bubble .msg-share-btn,
.message-item.user .msg-bubble .msg-copy-btn,
.message-item.user .msg-bubble .msg-export-img-btn,
.message-item.user .msg-bubble .msg-toggle-btn {
  color: rgba(255, 255, 255, 0.92);
  background-color: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.65);
}
.message-item.user .msg-bubble .msg-share-btn:hover,
.message-item.user .msg-bubble .msg-copy-btn:hover,
.message-item.user .msg-bubble .msg-export-img-btn:hover,
.message-item.user .msg-bubble .msg-toggle-btn:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.95);
  background-color: rgba(255, 255, 255, 0.32);
}
.msg-content .msg-markdown {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.msg-content .msg-markdown :deep(.md-editor-preview-wrapper) {
  width: 100%;
}
.msg-content .msg-markdown :deep(.md-editor-preview) {
  font-size: 13px;
  line-height: 1.6;
  padding: 0 8px;
  width: 100%;
  box-sizing: border-box;
}
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  color: inherit;
}
.msg-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.msg-meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--n-text-color-3);
  display: flex;
}
.msg-meta-user-inner {
  justify-content: flex-end;
  margin-top: 6px;
  margin-bottom: 0;
}
.message-item.user .msg-meta-user-inner {
  color: rgba(255, 255, 255, 0.78);
}

.chat-footer {
  flex-shrink: 0;
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--n-color-modal);
}
.chat-footer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.chat-footer-select {
  flex: 1;
  min-width: 0;
}
.chat-footer-select .n-select {
  width: 100%;
}
.chat-footer-prompt {
  flex: 0 0 120px;
  min-width: 0;
}
.chat-footer-prompt .n-select {
  width: 100%;
}
.chat-footer-thinking {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-thinking-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}
.chat-footer-memory {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-memory-count {
  width: 70px;
}
.chat-footer-agent-mode-select {
  width: 120px;
}
.chat-footer-memory-count .n-select {
  width: 100%;
}
.chat-footer-input {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}
.chat-footer-input .n-input {
  flex: 1;
  min-width: 0;
}
.chat-footer-input .n-input :deep(textarea) {
  text-align: left;
}
.chat-footer-input .n-button {
  flex-shrink: 0;
}
.chat-footer-abort {
  color: #f97316;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.drawer-slide-enter-active .drawer-mask,
.drawer-slide-leave-active .drawer-mask {
  transition: opacity 0.25s ease;
}
.drawer-slide-enter-active .drawer-panel,
.drawer-slide-leave-active .drawer-panel {
  transition: transform 0.25s ease;
}
.drawer-slide-enter-from .drawer-mask,
.drawer-slide-leave-to .drawer-mask {
  opacity: 0;
}
.drawer-slide-enter-from .drawer-panel,
.drawer-slide-leave-to .drawer-panel {
  transform: translateX(100%);
}
.drawer-slide-enter-to .drawer-mask,
.drawer-slide-leave-from .drawer-mask {
  opacity: 1;
}
.drawer-slide-enter-to .drawer-panel,
.drawer-slide-leave-from .drawer-panel {
  transform: translateX(0);
}
</style>

<style>
body > div:has(.n-select-menu) {
  z-index: 10002 !important;
}

.msg-markdown .md-editor-code-block {
  position: relative;
}
.msg-markdown .md-editor-code-block pre {
  margin: 0;
}
.msg-markdown .md-editor-code-block .code-collapse-btn {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 2;
  padding: 2px 8px;
  font-size: 11px;
  color: var(--n-text-color-3);
  background: var(--n-color-hover);
  border: 1px solid var(--n-border-color);
  border-radius: 0 4px 0 4px;
  cursor: pointer;
  user-select: none;
  opacity: 0;
  transition: opacity 0.2s;
}
.msg-markdown .md-editor-code-block:hover .code-collapse-btn {
  opacity: 1;
}
.msg-markdown .md-editor-code-block.code-collapsed pre {
  max-height: 80px;
  overflow: hidden;
}
.msg-markdown .md-editor-code-block.code-collapsed::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 40px;
  background: linear-gradient(transparent, var(--n-color));
  pointer-events: none;
}

/* AI 输出中的股票代码/名称可点击链接 */
.msg-markdown .md-editor-preview a.stock-link {
  color: var(--n-primary-color, #18a058);
  text-decoration: none;
  cursor: pointer;
  border-bottom: 1px dashed var(--n-primary-color, #18a058);
  padding: 0 1px;
  transition: color 0.15s, background-color 0.15s, border-bottom-style 0.15s;
}
.msg-markdown .md-editor-preview a.stock-link:hover {
  color: #fff;
  background-color: var(--n-primary-color, #18a058);
  border-bottom-style: solid;
}
</style>
