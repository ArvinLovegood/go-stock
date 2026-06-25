<template>
  <!-- 边缘触发条：仅在非 agent 页显示 -->
  <Transition name="fade">
    <div
      v-if="showButton"
      :class="['edge-trigger', edgeSideClass, { 'edge-trigger-busy': hasBackgroundTask }]"
      @click="togglePanel"
      :title="hasBackgroundTask ? title + '正在后台分析...' : title"
    >
      <div class="edge-trigger-inner">
        <NIcon :component="ChatbubbleEllipsesOutline" size="22" />
        <div v-if="hasBackgroundTask" class="edge-trigger-badge" />
      </div>
    </div>
  </Transition>

  <!-- 抽屉：点击触发条后滑出 -->
  <Transition name="drawer-slide">
    <div v-if="panelVisible" class="drawer-wrap">
      <div class="drawer-mask" @click="closePanel" />
      <div :class="['drawer-panel', drawerSideClass]" @click.stop>
        <NCard
          size="small"
          class="panel-card"
          :bordered="false"
          content-style="padding: 0; display: flex; flex-direction: column; height: 100%;"
        >
          <template #header>
            <div class="panel-header">
              <span class="panel-title">{{ title }}</span>
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
            <div v-if="shareTipVisible" class="share-tip">
              <div class="share-tip-text">{{ shareTipText }}</div>
              <NButton size="tiny" quaternary class="share-tip-close" @click="shareTipVisible = false">关闭</NButton>
            </div>
            <NScrollbar ref="scrollbarRef" class="chat-scroll">
              <div class="message-list">
                <div v-if="messages.length > DEFAULT_VISIBLE_COUNT" class="message-list-expand">
                  <NButton quaternary size="small" @click="showMoreHistory">
                    {{ expandAll ? '收起' : '展开更多历史' }}{{ expandAll ? '' : '（共 ' + hiddenCount + ' 条）' }}
                  </NButton>
                </div>
                <div
                  v-for="(msg, index) in displayedMessages"
                  :key="displayFromIndex + index"
                  :class="['message-item', msg.role]"
                >
                  <div v-if="msg.role === 'assistant'" class="msg-avatar assistant-avatar">
                    <NIcon :component="SparklesOutline" size="20" />
                  </div>
                  <div class="msg-bubble">
                    <template v-if="needBubbleCollapse(msg) && !isBubbleExpanded(displayFromIndex + index) && !(index === displayedMessages.length - 1 && isStreamLoad)">
                      <div v-if="msg.role === 'assistant'" class="ai-reply-export-root">
                        <div class="msg-content msg-content-collapsed" style="text-align: left">
                          {{ getBubblePreview(msg) }}
                        </div>
                      </div>
                      <template v-else>
                        <div class="msg-content msg-content-collapsed" style="text-align: left">
                          {{ getBubblePreview(msg) }}
                        </div>
                        <div v-if="msg.role === 'user' && msg.time" class="msg-meta msg-meta-user-inner">
                          <span class="msg-time">{{ msg.time }}</span>
                        </div>
                      </template>

                      <div class="msg-bubble-actions" v-if="msg.role === 'assistant'">
                        <div v-if="msg.modelName || msg.time" class="msg-meta-row-assistant">
                          <span v-if="msg.modelName" class="msg-model-name" :title="msg.modelName">{{ msg.modelName }}</span>
                          <span v-if="msg.time" class="msg-time">{{ msg.time }}</span>
                        </div>
                        <NButton quaternary size="tiny" class="msg-expand-btn" @click="toggleBubble(displayFromIndex + index)">
                          <template #icon>
                            <NIcon :component="ChevronDownOutline" />
                          </template>
                          展开
                        </NButton>
                        <NButton quaternary size="tiny" class="msg-copy-btn" @click="copyAiContent(msg)">
                          <template #icon>
                            <NIcon :component="CopyOutline" />
                          </template>
                          复制
                        </NButton>
                        <NButton
                          quaternary
                          size="tiny"
                          class="msg-export-img-btn"
                          :loading="exportImageKey === (displayFromIndex + '-' + index)"
                          title="导出为图片"
                          @click="exportAiReplyImage(displayFromIndex, index, $event)"
                        >
                          <template #icon>
                            <NIcon :component="ImageOutline" />
                          </template>
                          导出图
                        </NButton>
                        <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="shareAiContent(msg)">
                          <template #icon>
                            <NIcon :component="ShareSocialOutline" />
                          </template>
                          分享
                        </NButton>
                      </div>
                    </template>
                    <template v-else>
                      <div v-if="msg.role === 'assistant'" class="ai-reply-export-root">
                        <div v-if="msg.reasoning" class="msg-reasoning-wrapper">
                          <div class="msg-reasoning-header" @click="toggleReasoning(displayFromIndex + index)">
                            <NIcon :component="reasoningExpandedMap[displayFromIndex + index] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                            <span class="msg-reasoning-title">💭 思考过程</span>
                          </div>
                          <div v-show="reasoningExpandedMap[displayFromIndex + index]" class="msg-reasoning-content">
                            {{ msg.reasoning }}
                          </div>
                        </div>
                        <div class="msg-content">
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'left' }"
                            v-if="msg.content"
                            :model-value="msg.content"
                            :editor-id="'ai-msg-' + (displayFromIndex + index)"
                            class="msg-markdown"
                          />
                        </div>
                      </div>
                      <div v-else class="msg-content">
                        <!-- 用户提问时间：气泡内、正文下方右对齐 -->
                        <div v-if="msg.role === 'user' && msg.time" class="msg-meta msg-meta-user-inner">
                          <span class="msg-time">{{ msg.time }}</span>
                        </div>
                        <MdPreview
                          :theme="theme"
                          :style="{ textAlign: 'right' }"
                          v-if="msg.content"
                          :model-value="msg.content"
                          :editor-id="'ai-msg-' + (displayFromIndex + index)"
                          class="msg-markdown"
                        />
                      </div>

                      <div class="msg-bubble-actions" v-if="msg.role === 'assistant'">
                        <div v-if="msg.role === 'assistant' && isStreamLoad && index === displayedMessages.length - 1" class="msg-loading msg-loading-row">
                          <NSpin size="small" />
                          <span>思考中...</span>
                        </div>
                        <div v-if="msg.modelName || msg.time" class="msg-meta-row-assistant">
                          <span v-if="msg.modelName" class="msg-model-name" :title="msg.modelName">{{ msg.modelName }}</span>
                          <span v-if="msg.time" class="msg-time">{{ msg.time }}</span>
                        </div>
                        <NButton v-if="needBubbleCollapse(msg) && !(index === displayedMessages.length - 1 && isStreamLoad)" quaternary size="tiny" class="msg-expand-btn" @click="toggleBubble(displayFromIndex + index)">
                          <template #icon>
                            <NIcon :component="ChevronUpOutline" />
                          </template>
                          收起
                        </NButton>
                        <NButton quaternary size="tiny" class="msg-copy-btn" @click="copyAiContent(msg)">
                          <template #icon>
                            <NIcon :component="CopyOutline" />
                          </template>
                          复制
                        </NButton>
                        <NButton
                          quaternary
                          size="tiny"
                          class="msg-export-img-btn"
                          :loading="exportImageKey === (displayFromIndex + '-' + index)"
                          title="导出为图片"
                          @click="exportAiReplyImage(displayFromIndex, index, $event)"
                        >
                          <template #icon>
                            <NIcon :component="ImageOutline" />
                          </template>
                          导出图
                        </NButton>
                        <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="shareAiContent(msg)">
                          <template #icon>
                            <NIcon :component="ShareSocialOutline" />
                          </template>
                          分享
                        </NButton>
                      </div>
                    </template>
                  </div>
                  <div v-if="msg.role === 'user'" class="msg-avatar user-avatar">
                    <NIcon :component="PersonCircleOutline" size="20" />
                  </div>
                </div>
              </div>
            </NScrollbar>

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
          </div>
        </NCard>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, onBeforeMount, h } from 'vue'
import { useRoute } from 'vue-router'
import { NAvatar, NButton, NCard, NIcon, NInput, NScrollbar, NSelect, NSpin, NSwitch, useMessage, useNotification } from 'naive-ui'
import {
  ChatbubbleEllipsesOutline,
  ChevronDownOutline,
  ChevronUpOutline,
  CloseOutline,
  SparklesOutline,
  PersonCircleOutline,
  CopyOutline,
  ShareSocialOutline,
  ImageOutline
} from '@vicons/ionicons5'
import { AbortSummaryStockNews, GetAiAssistantSession, GetAiConfigs, GetConfig, GetPromptTemplates, GetSponsorInfo, GetVersionInfo, SaveAiAssistantSession, ShareText, SummaryStockNews } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import html2canvas from 'html2canvas'

const DEFAULT_VISIBLE_COUNT = 20
const COLLAPSE_CHAR_LIMIT = 200
const STORAGE_KEY_MODEL_ID = 'go-stock-ai-last-model-id'

const props = defineProps({
  title: {
    type: String,
    default: 'go-stock AI 助手'
  },
  side: {
    type: String,
    default: 'right',
    validator: value => ['left', 'right'].includes(value)
  },
  vipRequired: {
    type: Boolean,
    default: true
  }
})

const route = useRoute()
// 注意：抽屉遮罩层 z-index 很高，使用全局 notification/message 可能被遮罩盖住。
// 这里仍保留注入，供未来需要时使用，但分享结果会直接在抽屉内展示。
const notify = useNotification()
const message = useMessage()

// 仅在非 agent 页显示悬浮按钮
const showButton = computed(() => route.name !== 'agent')

const panelVisible = ref(false)
const inputValue = ref('')
const isStreamLoad = ref(false)
const sentFromFloating = ref(false)
const messages = ref([])
const aiConfigOptions = ref([])
const aiConfigId = ref(null)

/** 当前选中配置在列表中的展示文案（与底部「选择模型」一致），用于写入每条助手回复 */
function modelLabelForConfig(configId) {
  const opts = aiConfigOptions.value
  if (!opts?.length) return ''
  const id = configId != null ? Number(configId) : Number(opts[0].value)
  const found = opts.find(o => Number(o.value) === id)
  return found?.label != null ? String(found.label) : ''
}

// 系统提示词模板（System Prompt）
const sysPromptTemplates = ref([])
const sysPromptOptions = computed(() =>
  sysPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const sysPromptId = ref(null)

// 用户提示词模板（User Prompt）
const userPromptTemplates = ref([])
const userPromptOptions = computed(() =>
  userPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const userPromptId = ref(null)
const thinkingMode = ref(true)
const memoryMode = ref(true)
const memoryCount = ref(5)
const memoryCountOptions = [
  { label: '5 条', value: 5 },
  { label: '10 条', value: 10 },
  { label: '20 条', value: 20 },
  { label: '30 条', value: 30 },
  { label: '50 条', value: 50 }
]

function onUserPromptChange(id) {
  if (!id) return
  const t = userPromptTemplates.value.find(x => (x.ID ?? x.id) === id)
  if (t?.content) inputValue.value = t.content
}

const canSend = computed(() => !!inputValue.value.trim())
const scrollbarRef = ref(null)
const darkTheme = ref(false)
const appIcon = ref('')
const shareLoading = ref(false)
/** 正在导出图片的消息键 `${displayFromIndex}-${index}`，空表示未在导出 */
const exportImageKey = ref('')
const shareTipVisible = ref(false)
const shareTipText = ref('')
const vipLevel = ref(0)
const vipLoaded = ref(false)
const vipLoading = ref(false)
const visibleCount = ref(DEFAULT_VISIBLE_COUNT)
const expandedBubbles = ref({})
const isAborted = ref(false)
const reasoningExpandedMap = ref({})

const hasBackgroundTask = computed(() => isStreamLoad.value && sentFromFloating.value && !panelVisible.value)
const edgeSideClass = computed(() => props.side === 'left' ? 'edge-trigger-left' : 'edge-trigger-right')
const drawerSideClass = computed(() => props.side === 'left' ? 'drawer-panel-left' : 'drawer-panel-right')
const AI_ASSISTANT_EVENT = 'aiAssistantSummaryStockNews'

function getBubbleFullText(msg) {
  const r = (msg.reasoning || '').trim()
  const c = (msg.content || '').trim()
  return r ? r + '\n' + c : c
}
function needBubbleCollapse(msg) {
  return getBubbleFullText(msg).length > COLLAPSE_CHAR_LIMIT
}
function getBubblePreview(msg) {
  const full = getBubbleFullText(msg)
  return full.length <= COLLAPSE_CHAR_LIMIT ? full : full.slice(0, COLLAPSE_CHAR_LIMIT) + '...'
}

function toggleReasoning(index) {
  reasoningExpandedMap.value = {
    ...reasoningExpandedMap.value,
    [index]: !reasoningExpandedMap.value[index]
  }
}
function isBubbleExpanded(index) {
  return !!expandedBubbles.value[index]
}
function toggleBubble(index) {
  expandedBubbles.value = { ...expandedBubbles.value, [index]: !expandedBubbles.value[index] }
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
  shareTextToCommunity(text, 'AI助手')
}

function shareAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 正文内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'AI助手')
}

function assistantReplyExportTarget(editorId, bubble) {
  const root = bubble?.querySelector('.ai-reply-export-root')
  if (root) return root
  return (
    document.getElementById(`${editorId}-preview-wrapper`) ||
    document.getElementById(`${editorId}-preview`) ||
    bubble?.querySelector('.md-editor-preview') ||
    null
  )
}

async function exportAiReplyImage(displayFromIndex, index, evt) {
  const msg = messages.value[displayFromIndex + index]
  if (msg?.role !== 'assistant') return
  if (!getBubbleFullText(msg).trim()) {
    shareTipText.value = '暂无可导出的 AI 回答内容'
    shareTipVisible.value = true
    return
  }
  const editorId = 'ai-msg-' + (displayFromIndex + index)
  const bubble = evt?.currentTarget?.closest?.('.msg-bubble')
  const key = `${displayFromIndex}-${index}`
  if (exportImageKey.value) return
  exportImageKey.value = key
  await nextTick()
  try {
    const target = assistantReplyExportTarget(editorId, bubble)
    if (!target) {
      shareTipText.value = '未找到预览区域，请展开回答后重试'
      shareTipVisible.value = true
      return
    }
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: darkTheme.value ? '#1e1e1e' : '#ffffff'
    })
    const link = document.createElement('a')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
    link.href = canvas.toDataURL('image/png')
    link.download = `go-stock-ai-${safeTime}.png`
    link.click()
    shareTipText.value = '已导出为 PNG 图片'
    shareTipVisible.value = true
  } catch (e) {
    shareTipText.value = '导出图片失败: ' + (e?.message ?? e)
    shareTipVisible.value = true
  } finally {
    exportImageKey.value = ''
  }
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
      // 兜底方案
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

function abortStream(showTip = true) {
  if (!isStreamLoad.value) return
  isAborted.value = true
  isStreamLoad.value = false
  if (showTip) {
    shareTipText.value = '已中断本次 AI 回答'
    shareTipVisible.value = true
  }
  AbortSummaryStockNews()
}

const displayedMessages = computed(() => {
  const total = messages.value.length
  const from = Math.max(0, total - visibleCount.value)
  return messages.value.slice(from)
})
const displayFromIndex = computed(() => Math.max(0, messages.value.length - visibleCount.value))
const hiddenCount = computed(() => Math.max(0, messages.value.length - visibleCount.value))
const expandAll = computed(() => visibleCount.value >= messages.value.length)

function showMoreHistory() {
  if (expandAll.value) {
    visibleCount.value = DEFAULT_VISIBLE_COUNT
  } else {
    visibleCount.value = messages.value.length
  }
  nextTick(scrollToBottom)
}

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))
async function loadHistory() {
  try {
    const list = await GetAiAssistantSession()
    if (Array.isArray(list) && list.length > 0) {
      messages.value = list.map(m => ({
        role: m.role ?? '',
        content: m.content ?? '',
        reasoning: m.reasoning ?? '',
        time: m.time ?? '',
        modelName: m.modelName ?? ''
      }))
      // 默认展开最后一条助手回答
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const msg = messages.value[i]
        if (msg && msg.role === 'assistant' && (msg.content ?? '').trim()) {
          expandedBubbles.value = { ...expandedBubbles.value, [i]: true }
          break
        }
      }
    }
  } catch (_) {
    // ignore
  }
}

function saveHistory() {
  if (messages.value.length === 0) return
  const list = messages.value.map(m => ({
    role: m.role,
    content: m.content,
    reasoning: m.reasoning ?? '',
    time: m.time ?? '',
    modelName: m.modelName ?? ''
  }))
  SaveAiAssistantSession(list).catch(() => {})
}

function openPanel() {
  panelVisible.value = true
  if (messages.value.length === 0) {
    messages.value = [
      {
        role: 'assistant',
        content: '我是 go-stock AI 助手，可随时在这里提问。支持股票、市场、投资等相关问题。',
        reasoning: '',
        time: new Date().toLocaleString(),
        modelName: ''
      }
    ]
  }
  nextTick(scrollToBottom)
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
    ensureSummaryEvent()
    if (props.vipRequired) {
      await ensureVipInfo()
    }
    if (props.vipRequired && (vipLevel.value ?? 0) < 2) {
      message.warning('go-stock AI 助手功能仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
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
  // 若上一次回答还在流式中，先中断后再发新问题
  if (isStreamLoad.value) {
    abortStream(false)
  }
  const text = inputValue.value.trim()
  if (!text) {
    message.warning('请输入你的问题（模板仅作辅助，请先输入内容）')
    return
  }

  messages.value.push({
    role: 'user',
    content: text,
    reasoning: '',
    time: new Date().toLocaleString(),
    modelName: ''
  })
  const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
  const modelName = modelLabelForConfig(configId)
  messages.value.push({
    role: 'assistant',
    content: '',
    reasoning: '',
    time: new Date().toLocaleString(),
    modelName
  })
  inputValue.value = ''
  isStreamLoad.value = true
  isAborted.value = false
  sentFromFloating.value = true
  saveHistory()
  // 系统提示词：传递模板 ID，后端根据 ID 读取 System Prompt
  const sysId = sysPromptId.value != null ? Number(sysPromptId.value) : null
  // 记忆模式：带上最近 N 条对话（排除当前这条空的 assistant 占位）
  let historyJSON = ''
  if (memoryMode.value) {
    // 去掉末尾空的 assistant 占位
    let historyMessages = messages.value.slice(0, -1)
    // 当前用户问题已由 SummaryStockNews 的 question 参数单独传递，勿再写入 history，避免重复占 token
    if (historyMessages.length > 0 && historyMessages[historyMessages.length - 1]?.role === 'user') {
      historyMessages = historyMessages.slice(0, -1)
    }
    const maxHistory = Math.max(1, Number(memoryCount.value) || 5)
    const toSend =
      historyMessages.length <= maxHistory ? historyMessages : historyMessages.slice(-maxHistory)
    historyJSON = JSON.stringify(toSend.map(m => ({ role: m.role, content: m.content ?? '', reasoning: m.reasoning ?? '' })))
  }
  SummaryStockNews(text, configId, sysId, true, thinkingMode.value, AI_ASSISTANT_EVENT, historyJSON)
  nextTick(scrollToBottom)
}

let hasSummaryEvent = false

function onSummaryStockNews(msg) {
  if (msg === 'DONE') {
    isStreamLoad.value = false
    sentFromFloating.value = false
    isAborted.value = false
    // 默认展开最新一条助手回答
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const m = messages.value[i]
      if (m && m.role === 'assistant' && (m.content ?? '').trim()) {
        expandedBubbles.value = { ...expandedBubbles.value, [i]: true }
        break
      }
    }
    saveHistory()
    nextTick(scrollToBottom)
    return
  }
  if (isAborted.value) {
    return
  }
  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'assistant') {
    if (msg?.content) last.content += msg.content
    if (msg?.extraContent) last.content += msg.extraContent
    if (msg?.reasoning_content) last.reasoning += msg.reasoning_content
    nextTick(scrollToBottom)
  }
}

// 开始新对话：清空当前消息，本地视图重置，但不会删除历史会话记录
function startNewChat() {
  if (isStreamLoad.value) {
    message.warning('当前有回答正在生成，请先中断或等待完成')
    return
  }
  messages.value = []
  expandedBubbles.value = {}
  saveHistory() // 空列表不会在后端创建新会话，历史会话仍然保留
}

function ensureSummaryEvent() {
  if (hasSummaryEvent) return
  EventsOn(AI_ASSISTANT_EVENT, onSummaryStockNews)
  hasSummaryEvent = true
}

const PROMPT_TEMPLATES_CHANGED = 'promptTemplatesChanged'
onBeforeUnmount(() => {
  EventsOff(AI_ASSISTANT_EVENT)
  EventsOff(PROMPT_TEMPLATES_CHANGED)
  hasSummaryEvent = false
})

function loadPromptTemplates() {
  GetPromptTemplates('', '').then(res => {
    const list = Array.isArray(res) ? res : []
    sysPromptTemplates.value = list.filter(t => t.type === '模型系统Prompt')
    userPromptTemplates.value = list.filter(t => t.type === '模型用户Prompt')
  })
}

watch(panelVisible, (v) => {
  if (v) {
    loadPromptTemplates()
    nextTick(scrollToBottom)
  }
})

onBeforeMount(()=> {
  GetConfig().then(result => {
    darkTheme.value = result.darkTheme
  })
} )
onMounted(() => {
  EventsOn(PROMPT_TEMPLATES_CHANGED, loadPromptTemplates)
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
      // 优先使用 localStorage 中保存的上一次模型 ID
      const lastModelId = localStorage.getItem(STORAGE_KEY_MODEL_ID)
      if (lastModelId) {
        const foundId = Number(lastModelId)
        // 检查该 ID 是否仍然可用
        const isValid = aiConfigOptions.value.some(opt => opt.value === foundId)
        aiConfigId.value = isValid ? foundId : aiConfigOptions.value[0].value
      } else {
        aiConfigId.value = aiConfigOptions.value[0].value
      }
    }
  })
  loadPromptTemplates()
  GetVersionInfo().then(res => {
    if (res?.icon) appIcon.value = res.icon
  })
})

// 监听模型选择变化，保存到 localStorage
watch(aiConfigId, (newId) => {
  if (newId != null) {
    localStorage.setItem(STORAGE_KEY_MODEL_ID, String(newId))
  }
})
</script>

<style scoped>
/* 边缘触发条 */
.edge-trigger {
  position: fixed;
  top: 50%;
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
.edge-trigger-right {
  right: 0;
  border-radius: 12px 0 0 12px;
}
.edge-trigger-left {
  left: 0;
  border-radius: 0 12px 12px 0;
  box-shadow: 2px 0 12px rgba(102, 126, 234, 0.4);
}
.edge-trigger-busy {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger-left.edge-trigger-busy {
  box-shadow: 4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger:hover {
  width: 40px;
  box-shadow: -4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-left:hover {
  box-shadow: 4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
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
}

/* 抽屉容器 */
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
.drawer-panel-right {
  right: 0;
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
}
.drawer-panel-left {
  left: 0;
  box-shadow: 8px 0 24px rgba(0, 0, 0, 0.15);
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
  flex: 1;
  min-height: 0;
}
.message-list {
  padding: 12px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-list-expand {
  display: flex;
  justify-content: center;
  padding: 8px 0;
  border-bottom: 1px dashed var(--n-border-color);
}
.message-item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.message-item.user {
  justify-content: flex-end;
}
.msg-avatar {
  width: 32px;
  height: 32px;
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
  flex: 1;
  min-width: 0;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 14px;
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
.msg-reasoning-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
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
.msg-reasoning {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  margin-bottom: 6px;
  padding-bottom: 6px;
  border-bottom: 1px dashed var(--n-border-color);
}
.msg-content {
  white-space: normal;
  width: 100%;
  min-width: 0;
  flex: 1;
}
.msg-content-collapsed {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.5;
  opacity: 0.9;
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
.msg-loading-row {
  flex: 1 1 100%;
  justify-content: flex-end;
  margin-top: 0;
}
.msg-share-btn,
.msg-expand-btn,
.msg-copy-btn,
.msg-export-img-btn {
  padding: 2px 10px;
  font-size: 12px;
  border-radius: 12px;
  color: var(--n-primary-color);
  background-color: var(--n-primary-color-suppl);
  border: 1px solid var(--n-primary-color);
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;
}
.msg-share-btn:hover,
.msg-expand-btn:hover,
.msg-copy-btn:hover,
.msg-export-img-btn:hover {
  border-color: var(--n-primary-color);
  background-color: var(--n-primary-color);
  color: #fff;
}
.message-item.user .msg-bubble .msg-share-btn,
.message-item.user .msg-bubble .msg-expand-btn,
.message-item.user .msg-bubble .msg-copy-btn,
.message-item.user .msg-bubble .msg-export-img-btn {
  color: rgba(255, 255, 255, 0.92);
  background-color: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.65);
}
.message-item.user .msg-bubble .msg-share-btn:hover,
.message-item.user .msg-bubble .msg-expand-btn:hover,
.message-item.user .msg-bubble .msg-copy-btn:hover,
.message-item.user .msg-bubble .msg-export-img-btn:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.95);
  background-color: rgba(255, 255, 255, 0.32);
}
.msg-content .msg-markdown {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.msg-content .msg-markdown :deep(.md-editor-preview) {
  font-size: 13px;
  line-height: 1.6;
  padding: 0 8px;
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
.msg-meta-right {
  flex: auto;
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
  gap: 12px;
}
.chat-footer-select {
  flex: 1;
  min-width: 0;
}
.chat-footer-select .n-select {
  width: 100%;
}
.chat-footer-thinking {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-prompt {
  flex: 0 0 120px;
  min-width: 0;
}
.chat-footer-prompt .n-select {
  width: 100%;
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
  width: 80px;
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

/* 抽屉从右往左滑入 */
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
.drawer-slide-enter-from .drawer-panel-right,
.drawer-slide-leave-to .drawer-panel-right {
  transform: translateX(100%);
}
.drawer-slide-enter-from .drawer-panel-left,
.drawer-slide-leave-to .drawer-panel-left {
  transform: translateX(-100%);
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
<!-- 下拉挂载到 body 时需提高 z-index，否则会被抽屉遮挡 -->
<style>
body > div:has(.n-select-menu) {
  z-index: 10002 !important;
}
</style>
