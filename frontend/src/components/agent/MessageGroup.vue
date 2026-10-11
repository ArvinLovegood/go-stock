<template>
  <div class="message-group">
    <div class="message-group-header" @click="emit('toggle-expand', groupIndex)">
      <div class="message-group-summary">
        <NIcon :component="expanded ? ChevronDownOutline : ChevronForwardOutline" size="16" />
        <span class="message-group-title">{{ group.userMsg.content.slice(0, 50) }}{{ group.userMsg.content.length > 50 ? '...' : '' }}</span>
        <span class="message-group-time">{{ group.userMsg.time }}</span>
      </div>
    </div>
    <div v-show="expanded" class="message-group-content">
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
              :style="{ textAlign: 'left' }"
              v-if="group.userMsg.content"
              :model-value="group.userMsg.content"
              :editor-id="'agent-msg-' + group.userIndex"
              :sanitize="sanitizeMd"
              class="msg-markdown"
            />
            <div v-if="group.userMsg.role === 'user' && group.userMsg.images && group.userMsg.images.length" class="msg-image-grid">
              <NImage
                v-for="(img, i) in group.userMsg.images"
                :key="'agent-img-' + group.userIndex + '-' + i"
                :src="img"
                class="msg-image-thumb"
                object-fit="cover"
              />
            </div>
            <div v-if="isLast && !isStreamLoad" class="msg-user-actions">
              <NButton quaternary size="tiny" title="修改问题并重新发送" @click="emit('edit')">
                <template #icon>
                  <NIcon :component="CreateOutline" />
                </template>
                编辑
              </NButton>
            </div>
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
            <!-- 任务清单进度面板：write_todos 快照（[STEP]📝）与 PlanExecute content 清单（- [N] 步骤 [状态]）均解析至此 -->
            <div v-if="!conclusionOnly && group.assistantMsg.todos && group.assistantMsg.todos.length > 0" class="msg-todos-wrapper">
              <div class="msg-todos-header" @click="emit('toggle-reasoning', 'p-' + group.assistantIndex)">
                <NIcon :component="reasoningMap['p-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-todos-title">📋 任务清单</span>
                <span class="msg-todos-progress">{{ todosProgress(group.assistantMsg) }}</span>
                <span v-if="todosRunning(group.assistantMsg)" class="msg-tool-spinner"></span>
              </div>
              <div v-show="reasoningMap['p-' + group.assistantIndex]" class="msg-todos-content">
                <div v-for="(td, ti) in group.assistantMsg.todos" :key="ti" class="msg-todo-item" :class="'todo-' + td.status">
                  <span class="msg-todo-icon">{{ td.status === 'completed' ? '✅' : td.status === 'in_progress' ? '🔄' : '⏳' }}</span>
                  <span class="msg-todo-text">{{ td.text }}</span>
                </div>
              </div>
            </div>
            <div v-if="!conclusionOnly && group.assistantMsg.steps && visibleSteps(group.assistantMsg).length > 0" class="msg-steps-wrapper">
              <div class="msg-steps-header" @click="emit('toggle-reasoning', group.assistantIndex)">
                <NIcon :component="reasoningMap[group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-steps-title">📋 执行步骤</span>
                <span class="msg-steps-count">{{ visibleSteps(group.assistantMsg).length }}</span>
              </div>
              <div v-show="reasoningMap[group.assistantIndex]" class="msg-steps-content">
                <div v-for="(step, si) in visibleSteps(group.assistantMsg)" :key="si" class="msg-step-item">
                  <div class="msg-step-dot" :class="getStepDotClass(step)"></div>
                  <span class="msg-step-text">{{ step }}</span>
                </div>
              </div>
            </div>
            <!-- 工具调用过程卡片：状态机（运行中→成功/失败/停止），数据来自 [STEP] 流与结构化 tool_calls -->
            <div v-if="!conclusionOnly && group.assistantMsg.toolCards && group.assistantMsg.toolCards.length > 0" class="msg-tool-cards-wrapper">
              <div class="msg-tool-cards-header" @click="emit('toggle-reasoning', 't-' + group.assistantIndex)">
                <NIcon :component="reasoningMap['t-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-tool-cards-title">🔧 工具调用</span>
                <span class="msg-tool-cards-count">{{ group.assistantMsg.toolCards.length }}</span>
                <span v-if="toolCardsRunning(group.assistantMsg)" class="msg-tool-cards-live">
                  <span class="msg-tool-spinner"></span>
                  {{ runningToolName(group.assistantMsg) }}
                </span>
                <span v-else class="msg-tool-cards-done">已完成</span>
              </div>
              <div v-show="reasoningMap['t-' + group.assistantIndex]" class="msg-tool-cards-content">
                <div v-for="(card, ci) in group.assistantMsg.toolCards" :key="ci" class="msg-tool-card" :title="card.args">
                  <span class="msg-tool-status" :class="'tool-' + card.status">
                    <span v-if="card.status === 'running'" class="msg-tool-spinner"></span>
                    <template v-else>{{ card.status === 'success' ? '✅' : card.status === 'error' ? '❌' : '⏹' }}</template>
                  </span>
                  <span class="msg-tool-name">{{ card.name }}</span>
                  <span class="msg-tool-args">{{ card.argsPreview }}</span>
                  <span class="msg-tool-duration">{{ card.duration }}</span>
                </div>
              </div>
            </div>
            <div v-if="!conclusionOnly && group.assistantMsg.reasoning" class="msg-reasoning-wrapper">
              <div class="msg-reasoning-header" @click="emit('toggle-reasoning', 'r-' + group.assistantIndex)">
                <NIcon :component="reasoningMap['r-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-reasoning-title" :class="{ 'msg-reasoning-title-active': thinkingActive(group.assistantMsg) }">
                  <template v-if="thinkingActive(group.assistantMsg)">💭 深度思考中<span class="msg-thinking-dots"><i></i><i></i><i></i></span></template>
                  <template v-else-if="group.assistantMsg.thinkingDuration">💭 已深度思考 · 用时 {{ formatThinkingDuration(group.assistantMsg.thinkingDuration) }}</template>
                  <template v-else>💭 思考过程</template>
                </span>
              </div>
              <div v-show="reasoningMap['r-' + group.assistantIndex]" class="msg-reasoning-content">
                <MdPreview
                  :theme="theme"
                  :style="{ textAlign: 'left' }"
                  :model-value="group.assistantMsg.reasoning"
                  :editor-id="'agent-reasoning-' + group.assistantIndex"
                  :sanitize="sanitizeMd"
                  class="msg-markdown"
                />
              </div>
            </div>
            <div v-if="group.assistantMsg.jsonMarkdown" class="msg-json-md-wrapper">
              <div class="msg-json-md-header" @click="emit('toggle-reasoning', 'j-' + group.assistantIndex)">
                <NIcon :component="reasoningMap['j-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline" size="14" />
                <span class="msg-json-md-title">📊 分析报告</span>
                <span
                  class="msg-json-md-open"
                  title="在大窗口中阅读报告"
                  @click.stop="emit('open-report')"
                >
                  <NIcon :component="OpenOutline" size="14" />
                </span>
              </div>
              <div v-show="reasoningMap['j-' + group.assistantIndex]" class="msg-json-md-content">
                <MdPreview
                  :theme="theme"
                  :style="{ textAlign: 'left' }"
                  :model-value="group.assistantMsg.jsonMarkdown"
                  :editor-id="'agent-json-md-' + group.assistantIndex"
                  :sanitize="sanitizeMd"
                  class="msg-markdown"
                  @onHtmlChanged="emit('md-html-changed')"
                />
              </div>
            </div>
            <MdPreview
              :theme="theme"
              :style="{ textAlign: 'left' }"
              :model-value="isStreamLoad && isLast && group.assistantMsg.content ? group.assistantMsg.content + ' ▍' : (group.assistantMsg.content || '...')"
              :editor-id="'agent-msg-' + group.assistantIndex"
              :sanitize="sanitizeMd"
              class="msg-markdown"
              @onHtmlChanged="emit('md-html-changed')"
            />
            <!-- 底部加载态仅在没有思考内容时显示；思考中由思考区标题的动态状态承接，避免双重提示 -->
            <div v-if="isStreamLoad && isLast && !group.assistantMsg.content && !group.assistantMsg.rawReasoning" class="msg-loading">
              <NSpin size="small" />
              <span>思考中...</span>
            </div>
            <div class="msg-bubble-actions">
              <div v-if="group.assistantMsg.modelName || group.assistantMsg.time || group.assistantMsg.stats" class="msg-meta-row-assistant">
                <span v-if="group.assistantMsg.modelName" class="msg-model-name" :title="group.assistantMsg.modelName">{{ group.assistantMsg.modelName }}</span>
                <span v-if="group.assistantMsg.stats" class="msg-turn-stats"
                      :title="`本轮消耗：输入 ${group.assistantMsg.stats.inputTokens.toLocaleString()} tokens，输出 ${group.assistantMsg.stats.outputTokens.toLocaleString()} tokens`">
                  📊 {{ formatTokens(group.assistantMsg.stats.inputTokens) }} 入 / {{ formatTokens(group.assistantMsg.stats.outputTokens) }} 出
                  / {{ formatTokens(group.assistantMsg.stats.inputTokens + group.assistantMsg.stats.outputTokens) }} 总 tokens
                  · {{ group.assistantMsg.stats.tools }} 次工具 · {{ group.assistantMsg.stats.duration }}
                </span>
                <span v-if="group.assistantMsg.time" class="msg-time">{{ group.assistantMsg.time }}</span>
              </div>
              <NButton quaternary size="tiny" class="msg-toggle-btn" @click="emit('toggle-expand', groupIndex)">
                <template #icon>
                  <NIcon :component="expanded ? ChevronUpOutline : ChevronDownOutline" />
                </template>
                {{ expanded ? '收起' : '展开' }}
              </NButton>
              <NButton quaternary size="tiny" class="msg-copy-btn" @click="emit('copy')">
                <template #icon>
                  <NIcon :component="CopyOutline" />
                </template>
                复制
              </NButton>
              <!-- 引用追问（参考 ChatGPT quote reply）：把本轮回答摘要注入输入框，针对该结论继续提问 -->
              <NButton quaternary size="tiny" class="msg-quote-btn" title="引用本轮回答继续追问" @click="emit('quote')">
                <template #icon>
                  <NIcon :component="ChatbubbleEllipsesOutline" />
                </template>
                引用
              </NButton>
              <NButton v-if="!group.assistantMsg.feedback" quaternary size="tiny" class="msg-feedback-btn" title="这个回答有用" @click="emit('feedback', 1)">👍</NButton>
              <NButton v-if="!group.assistantMsg.feedback" quaternary size="tiny" class="msg-feedback-btn" title="这个回答没用" @click="emit('open-feedback')">👎</NButton>
              <span v-else class="msg-feedback-done">{{ group.assistantMsg.feedback === 1 ? '👍' : '👎' }}</span>
              <NButton
                quaternary
                size="tiny"
                class="msg-export-img-btn"
                :loading="exportLoading"
                title="导出为图片"
                @click="emit('export-image', $event)"
              >
                <template #icon>
                  <NIcon :component="ImageOutline" />
                </template>
                导出图
              </NButton>
              <!-- 导出 Markdown（复用 App.ExportSessionMarkdown 保存对话框）：单轮问答另存为 .md -->
              <NButton quaternary size="tiny" class="msg-export-md-btn" title="导出本次分析为 Markdown 文件" @click="emit('export-md')">
                <template #icon>
                  <NIcon :component="DocumentTextOutline" />
                </template>
                导出MD
              </NButton>
              <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="emit('share')">
                <template #icon>
                  <NIcon :component="ShareSocialOutline" />
                </template>
                分享
              </NButton>
              <NButton
                v-if="isLast && !isStreamLoad"
                quaternary
                size="tiny"
                class="msg-regen-btn"
                title="撤回本轮回答并重新生成"
                @click="emit('regenerate')"
              >
                <template #icon>
                  <NIcon :component="RefreshOutline" />
                </template>
                重新生成
              </NButton>
              <!-- 换模型重试（参考 ChatGPT/Poe）：撤回本轮并一次性用所选模型重新生成，不改变全局模型选择 -->
              <NPopover
                v-if="isLast && !isStreamLoad && modelOptions.length > 1"
                trigger="click"
                placement="top"
                :show-arrow="true"
                to="body"
                :z-index="10002"
                raw
              >
                <template #trigger>
                  <NButton quaternary size="tiny" class="msg-regen-btn" title="换一个模型重新生成本轮回答">
                    <template #icon>
                      <NIcon :component="SwapHorizontalOutline" />
                    </template>
                    换模型
                  </NButton>
                </template>
                <div class="msg-model-menu">
                  <div
                    v-for="opt in modelOptions"
                    :key="opt.value"
                    :class="['msg-model-menu-item', { active: opt.value === currentModelId, disabled: opt.value === currentModelId }]"
                    @click="opt.value !== currentModelId && emit('regenerate-with-model', opt.value)"
                  >
                    <span class="msg-model-menu-check">{{ opt.value === currentModelId ? '✓' : '' }}</span>
                    <span class="msg-model-menu-label">{{ opt.label }}</span>
                  </div>
                </div>
              </NPopover>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
// 会话消息组子组件：单条问答对（用户提问 + AI 回答）的纯展示组件。
// 从 FloatingAgentAssistant.vue 拆出（阶段2），展示逻辑内置、业务动作通过事件上抛。
// 展开状态（expanded）与折叠面板状态（reasoningMap）由父组件按会话持久化管理。
import { NButton, NIcon, NImage, NPopover, NSpin } from 'naive-ui'
import {
  ChevronDownOutline,
  ChevronForwardOutline,
  ChevronUpOutline,
  PersonCircleOutline,
  SparklesOutline,
  CreateOutline,
  CopyOutline,
  ImageOutline,
  DocumentTextOutline,
  ShareSocialOutline,
  RefreshOutline,
  SwapHorizontalOutline,
  OpenOutline,
  ChatbubbleEllipsesOutline
} from '@vicons/ionicons5'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import { sanitizeMd } from './format'

const props = defineProps({
  /** 消息组：{ id, userMsg, userIndex, assistantMsg, assistantIndex } */
  group: { type: Object, required: true },
  /** 组在 messageGroups 中的下标，用于展开集合的唯一键 */
  groupIndex: { type: Number, required: true },
  /** 是否为最后一组：末组在非流式时展示编辑/重新生成按钮 */
  isLast: { type: Boolean, default: false },
  /** 当前会话是否流式输出中 */
  isStreamLoad: { type: Boolean, default: false },
  /** MdPreview 主题：light / dark */
  theme: { type: String, default: 'light' },
  /** 本组是否展开 */
  expanded: { type: Boolean, default: false },
  /** 折叠面板展开状态表（reasoning/todos/steps/toolCards/jsonMd） */
  reasoningMap: { type: Object, default: () => ({}) },
  /** 导出图片按钮 loading（父组件按 assistantIndex 匹配） */
  exportLoading: { type: Boolean, default: false },
  /** 分享按钮 loading */
  shareLoading: { type: Boolean, default: false },
  /** 只看结论模式：隐藏任务清单/执行步骤/工具调用/思考过程，仅保留正文与报告 */
  conclusionOnly: { type: Boolean, default: false },
  /** 可选模型列表（换模型重试菜单）：[{ label, value }]；为空或仅 1 项时隐藏菜单 */
  modelOptions: { type: Array, default: () => [] },
  /** 当前选中模型 id（菜单中打勾标识） */
  currentModelId: { type: [Number, String], default: null }
})

const emit = defineEmits([
  'toggle-expand',
  'toggle-reasoning',
  'edit',
  'copy',
  'feedback',
  'open-feedback',
  'export-image',
  'export-md',
  'share',
  'regenerate',
  'regenerate-with-model',
  'open-report',
  'quote',
  'md-html-changed'
])

// ---- 以下为从 FloatingAgentAssistant.vue 原样迁移的展示层纯函数 ----

// formatTokens 大数值缩写为 k（如 1,234 → 1.2k，12,345 → 12.3k），提升可读性；
// 悬停 title 仍展示精确值。
function formatTokens(n) {
  if (n == null) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

function getStepDotClass(step) {
  if (step.includes('🎯')) return 'step-skill'
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('📝')) return 'step-todos'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

function todosProgress(item) {
  const todos = item.todos || []
  const done = todos.filter(t => t.status === 'completed').length
  return `${done}/${todos.length} 完成`
}

function todosRunning(item) {
  return (item.todos || []).some(t => t.status === 'in_progress')
}

// ---- 思考过程状态（对齐 DeepSeek/ChatGPT 交互） ----
// 思考中：本轮仍在流式输出、尚未产生答案内容且已有思考片段；结束后标题定格为「已深度思考 · 用时 X」
function thinkingActive(item) {
  return props.isStreamLoad && !item.thinkingDuration && !!(item.rawReasoning || item.reasoning)
}

function formatThinkingDuration(ms) {
  const s = Math.round(ms / 1000)
  return s >= 60 ? Math.floor(s / 60) + 'm' + (s % 60) + 's' : s + 's'
}

function visibleSteps(item) {
  return (item.steps || []).filter(s => {
    const t = s.trim()
    if (/调用工具[:：]/.test(t) || /✅\s*\S+\s*返回结果/.test(t)) return false
    // 清单快照已升级为独立面板，不再出现在步骤列表（含旧会话历史数据兜底）
    if (/^📝/.test(t) || /^任务清单更新/.test(t) || /^任务规划|^计划[:：]/.test(t)) return false
    return true
  })
}

function toolCardsRunning(item) {
  return (item.toolCards || []).some(c => c.status === 'running')
}

function runningToolName(item) {
  const c = (item.toolCards || []).find(c => c.status === 'running')
  return c ? `正在调用 ${c.name}` : ''
}
</script>

<style scoped>
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
  text-align: left;
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

/* 工具调用过程卡片 */
.msg-tool-cards-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-tool-cards-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.08) 0%, rgba(56, 173, 169, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-tool-cards-header:hover {
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.14) 0%, rgba(56, 173, 169, 0.14) 100%);
}
.msg-tool-cards-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-tool-cards-count {
  font-size: 11px;
  background: #409eff;
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.msg-tool-cards-live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #e6a23c;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.msg-tool-cards-done {
  font-size: 12px;
  color: var(--n-text-color-3);
}
.msg-tool-cards-content {
  padding: 6px 12px;
  max-height: 300px;
  overflow-y: auto;
}
.msg-tool-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 0;
  font-size: 12px;
  line-height: 1.5;
}
.msg-tool-card + .msg-tool-card {
  border-top: 1px dashed var(--n-border-color);
}
.msg-tool-status {
  width: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.msg-tool-status.tool-success {
  color: #67c23a;
}
.msg-tool-status.tool-error {
  color: #f56c6c;
}
.msg-tool-status.tool-stopped {
  color: var(--n-text-color-3);
}
.msg-tool-name {
  font-weight: 600;
  color: var(--n-text-color-1);
  font-family: SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  flex-shrink: 0;
  max-width: 40%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.msg-tool-args {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--n-text-color-3);
  font-size: 11px;
  font-family: SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  text-align: left;
}
.msg-tool-duration {
  color: var(--n-text-color-3);
  font-size: 11px;
  flex-shrink: 0;
}
.msg-tool-spinner {
  width: 10px;
  height: 10px;
  border: 2px solid rgba(230, 162, 60, 0.25);
  border-top-color: #e6a23c;
  border-radius: 50%;
  display: inline-block;
  animation: msg-tool-spin 0.8s linear infinite;
}
@keyframes msg-tool-spin {
  to { transform: rotate(360deg); }
}
.msg-user-actions {
  margin-top: 6px;
  display: flex;
  justify-content: flex-start;
  opacity: 0.75;
}
.msg-user-actions:hover {
  opacity: 1;
}
/* 任务清单进度面板（规划模式） */
.msg-todos-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-todos-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.08) 0%, rgba(46, 139, 87, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-todos-header:hover {
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.14) 0%, rgba(46, 139, 87, 0.14) 100%);
}
.msg-todos-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-todos-progress {
  font-size: 11px;
  color: #67c23a;
  background: rgba(103, 194, 58, 0.12);
  border-radius: 10px;
  padding: 0 8px;
  line-height: 18px;
  white-space: nowrap;
}
.msg-todos-content {
  padding: 8px 12px;
  max-height: 300px;
  overflow-y: auto;
}
.msg-todo-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  font-size: 12px;
  line-height: 1.6;
}
.msg-todo-icon {
  flex-shrink: 0;
  width: 16px;
  text-align: center;
}
.msg-todo-text {
  flex: 1;
  min-width: 0;
  color: var(--n-text-color-2);
  word-break: break-word;
  text-align: left;
}
.msg-todo-item.todo-in_progress .msg-todo-text {
  color: #e6a23c;
  font-weight: 500;
}
.msg-todo-item.todo-completed .msg-todo-text {
  color: var(--n-text-color-3);
  text-decoration: line-through;
}
/* 思考过程动态状态 */
.msg-reasoning-title-active {
  color: #e6a23c;
}
.msg-thinking-dots {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  margin-left: 4px;
}
.msg-thinking-dots i {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.3;
  animation: msg-thinking-blink 1.2s infinite;
}
.msg-thinking-dots i:nth-child(2) {
  animation-delay: 0.2s;
}
.msg-thinking-dots i:nth-child(3) {
  animation-delay: 0.4s;
}
@keyframes msg-thinking-blink {
  0%, 80%, 100% { opacity: 0.25; }
  40% { opacity: 1; }
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
/* 报告大窗打开按钮：右置、阻止冒泡（不触发折叠） */
.msg-json-md-open {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  color: var(--n-text-color-3);
  opacity: 0;
  transition: all 0.15s;
}
.msg-json-md-header:hover .msg-json-md-open {
  opacity: 1;
}
.msg-json-md-open:hover {
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
}
/* 换模型重试菜单（NPopover raw 内容） */
.msg-model-menu {
  min-width: 260px;
  max-height: 320px;
  overflow-y: auto;
  padding: 4px;
  border-radius: 8px;
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.14);
}
.msg-model-menu-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  color: var(--n-text-color-2);
  transition: background 0.15s;
}
.msg-model-menu-item:hover {
  background: var(--n-color-hover);
  color: #409eff;
}
.msg-model-menu-item.active {
  color: #409eff;
}
.msg-model-menu-item.disabled {
  cursor: default;
  opacity: 0.75;
}
.msg-model-menu-check {
  width: 16px;
  flex-shrink: 0;
  text-align: center;
}
.msg-model-menu-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
.msg-feedback-btn {
  font-size: 13px;
  padding: 0 6px;
}
.msg-feedback-done {
  font-size: 13px;
  opacity: 0.75;
  margin-left: 2px;
}
.msg-turn-stats {
  font-size: 12px;
  opacity: 0.65;
  cursor: default;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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
/* 用户消息气泡内图片 */
.msg-image-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 6px;
  margin-bottom: 6px;
}
.msg-image-thumb {
  width: 120px;
  height: 120px;
  border-radius: 8px;
  overflow: hidden;
}
</style>
