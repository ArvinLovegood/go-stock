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

  <!-- 右侧抽屉：常驻渲染（避免打开时挂载 DOM 卡顿），通过 class 切换滑入滑出 -->
  <div :class="['drawer-wrap', { 'drawer-open': panelVisible }]">
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
                <NButton size="small" quaternary @click="createSession" title="开始新对话">
                  新对话
                </NButton>
                <NPopover v-model:show="sessionMenuVisible" trigger="click" placement="bottom-end" :width="300" :z-index="10002" to="body" raw>
                  <template #trigger>
                    <NButton size="small" quaternary title="会话列表">
                      <template #icon>
                        <NIcon :component="ChatbubblesOutline" />
                      </template>
                      会话
                    </NButton>
                  </template>
                  <div class="session-menu">
                    <div class="session-menu-search">
                      <NInput
                        v-model:value="sessionKeyword"
                        size="small"
                        placeholder="搜索会话内容..."
                        clearable
                        @input="onSessionSearchInput"
                      >
                        <template #prefix>
                          <NIcon :component="SearchOutline" />
                        </template>
                      </NInput>
                    </div>
                    <div class="session-menu-list">
                      <div
                        v-for="s in sessionOrder"
                        :key="s.sessionId"
                        :class="['session-menu-item', { 'session-menu-item-active': s.sessionId === activeSessionId }]"
                        @click="setActiveSession(s.sessionId)"
                      >
                        <div class="session-menu-item-main">
                          <!-- 重命名态：标题替换为输入框，Enter/失焦保存，Esc 取消 -->
                          <NInput
                            v-if="renamingSessionId === s.sessionId"
                            v-model:value="renamingTitle"
                            size="tiny"
                            placeholder="会话标题（留空恢复自动标题）"
                            @click.stop
                            @keyup.enter="confirmRenameSession"
                            @keyup.esc="cancelRenameSession"
                            @blur="confirmRenameSession"
                          />
                          <template v-else>
                            <div class="session-menu-item-title">
                              <span v-if="s.pinned" class="session-pin-mark" title="已置顶">📌</span>
                              {{ s.title || '新会话' }}
                            </div>
                            <div class="session-menu-item-time">{{ s.updatedAt }}</div>
                          </template>
                        </div>
                        <span
                          v-if="sessionStates[s.sessionId] && sessionStates[s.sessionId].isStreamLoad"
                          class="session-running-dot"
                          title="正在生成"
                        />
                        <NButton size="tiny" quaternary class="session-menu-item-act" :title="s.pinned ? '取消置顶' : '置顶会话'" @click.stop="togglePinSession(s)">
                          <template #icon>
                            <NIcon :component="BookmarkOutline" :class="{ 'session-pin-active': s.pinned }" />
                          </template>
                        </NButton>
                        <NButton size="tiny" quaternary class="session-menu-item-act" title="重命名会话" @click.stop="startRenameSession(s)">
                          <template #icon>
                            <NIcon :component="CreateOutline" />
                          </template>
                        </NButton>
                        <NPopconfirm :z-index="10003" @positive-click="deleteSession(s.sessionId)">
                          <template #trigger>
                            <NButton size="tiny" quaternary class="session-menu-item-del" title="删除会话" @click.stop>
                              <template #icon>
                                <NIcon :component="TrashOutline" />
                              </template>
                            </NButton>
                          </template>
                          删除该会话？
                        </NPopconfirm>
                      </div>
                      <NEmpty
                        v-if="sessionOrder.length === 0"
                        size="small"
                        :description="sessionKeyword.trim() ? '未找到匹配的会话' : '暂无会话'"
                      />
                    </div>
                    <div class="session-menu-footer">
                      <NButton size="small" secondary block @click="createSession">
                        <template #icon>
                          <NIcon :component="AddOutline" />
                        </template>
                        新建会话
                      </NButton>
                    </div>
                  </div>
                </NPopover>
                <NButton
                  quaternary
                  circle
                  size="small"
                  :title="conclusionOnly ? '已开启只看结论（隐藏任务清单/步骤/工具调用/思考过程），点击关闭' : '只看结论：隐藏任务清单/步骤/工具调用/思考过程'"
                  @click="conclusionOnly = !conclusionOnly"
                >
                  <template #icon>
                    <NIcon :component="conclusionOnly ? EyeOffOutline : EyeOutline" :class="{ 'conclusion-only-active': conclusionOnly }" />
                  </template>
                </NButton>
                <NButton quaternary circle size="small" title="导出会话为 Markdown" @click="exportSessionMarkdown">
                  <template #icon>
                    <NIcon :component="DownloadOutline" />
                  </template>
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
            <NScrollbar ref="scrollbarRef" class="chat-scroll" @scroll="onMsgScroll">
              <div class="message-list">
                <MessageGroup
                  v-for="(group, groupIndex) in messageGroups"
                  :key="group.id"
                  :group="group"
                  :group-index="groupIndex"
                  :is-last="groupIndex === messageGroups.length - 1"
                  :is-stream-load="isStreamLoad"
                  :theme="theme"
                  :expanded="isGroupExpanded(groupIndex)"
                  :reasoning-map="reasoningExpandedMap"
                  :export-loading="exportImageKey === String(group.assistantIndex)"
                  :share-loading="shareLoading"
                  :conclusion-only="conclusionOnly"
                  :model-options="aiConfigOptions"
                  :current-model-id="aiConfigId"
                  @toggle-expand="toggleGroup"
                  @toggle-reasoning="toggleReasoning"
                  @edit="editUserMessage(group)"
                  @copy="copyAiContent(group.assistantMsg)"
                  @feedback="(r) => submitFeedback(group, r)"
                  @open-feedback="openFeedbackDialog(group)"
                  @export-image="(evt) => exportAiReplyImage(group.assistantIndex, evt)"
                  @export-md="exportGroupMarkdown(group)"
                  @share="shareAiContent(group.assistantMsg)"
                  @regenerate="regenerateMessage(group)"
                  @regenerate-with-model="(mid) => regenerateMessage(group, mid)"
                  @open-report="openReportModal(group)"
                  @quote="quoteGroup(group)"
                  @md-html-changed="onMdHtmlChanged"
                />
              </div>
            </NScrollbar>
            <!-- 回到底部（参考 ChatGPT ↓ 按钮）：上翻阅读历史时出现，点击恢复跟随最新输出 -->
            <Transition name="hint-fade">
              <button v-if="userScrolledUp" class="scroll-bottom-btn" :class="{ dark: darkTheme }" title="回到底部（恢复跟随最新输出）" @click="scrollToBottom(true)">↓</button>
            </Transition>
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
                  :disabled="sysPromptDisabled"
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
              <div v-if="selectedSkills.length" class="chat-footer-skill-tag">
                <NTag
                  v-for="s in selectedSkills"
                  :key="s.dirName"
                  type="info"
                  size="small"
                  closable
                  @close="removeSkill(s.dirName)"
                >
                  🎯 {{ s.name }}
                </NTag>
              </div>
              <div v-if="pendingImages.length" class="pending-images">
                <div v-for="(img, i) in pendingImages" :key="i" class="pending-image-item">
                  <NImage :src="img.preview" class="pending-image-thumb" object-fit="cover" />
                  <div v-if="img.uploading" class="pending-image-uploading" title="正在上传图床">
                    <NSpin size="small" />
                  </div>
                  <NButton
                    quaternary
                    circle
                    size="tiny"
                    class="pending-image-remove"
                    title="移除图片"
                    @click="removePendingImage(i)"
                  >
                    <template #icon>
                      <NIcon :component="CloseOutline" size="14" />
                    </template>
                  </NButton>
                </div>
              </div>
              <!-- 建议追问：最近一轮回复结束后按上下文生成，点击直接发送；无追问时回落到预设快捷指令 -->
              <div v-if="!isStreamLoad && !inputValue && followUpChips.length" class="chat-footer-quick">
                <button v-for="(c, i) in followUpChips" :key="'fu' + i" class="quick-chip follow-up-chip" :class="{ dark: darkTheme }" :title="c.text" @click="sendFollowUpChip(c)">
                  {{ c.icon }} {{ c.label }}
                </button>
              </div>
              <!-- 快捷指令：输入框为空且非流式时显示，点击走完整发送链路 -->
              <div v-else-if="!isStreamLoad && !inputValue" class="chat-footer-quick">
                <button v-for="p in quickPrompts" :key="p.label" class="quick-chip" :class="{ dark: darkTheme }" :title="p.text" @click="sendQuickPrompt(p)">
                  {{ p.icon }} {{ p.label }}
                </button>
              </div>
              <div class="chat-footer-input" style="position: relative;">
                <!-- / 斜杠菜单：快捷指令在前、技能在后，统一 ↑↓ 导航、回车/点击选择 -->
                <div v-if="skillMenuVisible && slashMenuItems.length" class="skill-menu" :class="{ dark: darkTheme }">
                  <div
                    v-for="(item, i) in slashMenuItems"
                    :key="item.type + '-' + item.key"
                    class="skill-menu-item"
                    :class="{ active: i === skillMenuIndex }"
                    @click="skillMenuIndex = i; selectSlashItem()"
                    @mouseenter="skillMenuIndex = i"
                  >
                    <span class="skill-menu-name">{{ item.icon }} {{ item.name }}</span>
                    <span class="skill-menu-desc">{{ item.desc }}</span>
                  </div>
                  <div class="skill-menu-footer">⌨️ 指令回车直接填入模板；🎯 技能可多选（回车选择/取消）；Esc 关闭菜单</div>
                </div>
                <!-- @ 股票联想：输入 @关键词 后弹出，选中替换为 股票名称(代码) 标记 -->
                <div v-if="stockMenuVisible && stockResults.length" class="skill-menu stock-menu" :class="{ dark: darkTheme }">
                  <div
                    v-for="(s, i) in stockResults"
                    :key="s.special ? 'sp-' + s.special : s.market + s.code"
                    class="skill-menu-item"
                    :class="{ active: i === stockMenuIndex }"
                    @click="stockMenuIndex = i; selectStockFromMenu()"
                    @mouseenter="stockMenuIndex = i"
                  >
                    <span class="skill-menu-name">{{ s.icon || '📈' }} {{ s.name }}</span>
                    <span class="skill-menu-desc">{{ s.desc || (s.market ? s.market + s.code : '') }}</span>
                  </div>
                  <div class="skill-menu-footer">↑↓ 切换，回车/点击 插入，Esc 关闭</div>
                </div>
                <NPopover
                  v-if="currentConfigSupportsVision"
                  trigger="click"
                  placement="top-start"
                  :show-arrow="true"
                  to="body"
                  :z-index="10002"
                >
                  <template #trigger>
                    <NButton
                      quaternary
                      size="small"
                      class="chat-footer-img-btn"
                      title="添加图片（支持上传 / 链接 / 粘贴）"
                      :disabled="isStreamLoad"
                    >
                      <template #icon>
                        <NIcon :component="ImageOutline" />
                      </template>
                    </NButton>
                  </template>
                  <div class="image-add-popover">
                    <NButton size="small" dashed block @click="triggerImageSelect">
                      上传本地图片
                    </NButton>
                    <div class="image-url-row">
                      <NInput
                        v-model:value="imageUrlInput"
                        size="small"
                        placeholder="或输入图片链接 https://..."
                        clearable
                      />
                      <NButton size="small" type="primary" ghost @click="addImageUrl">
                        添加
                      </NButton>
                    </div>
                    <div class="image-add-tip">支持 JPEG/PNG/GIF/WebP，单张 ≤ 8MB，最多 {{ MAX_IMAGE_COUNT }} 张；也可直接在输入框粘贴图片。本地图默认自动托管到免费图床转为外链发送（图床为公共免费服务，请勿上传敏感图片），失败时回退 base64 直传</div>
                  </div>
                </NPopover>
                <NButton
                  v-else
                  quaternary
                  size="small"
                  class="chat-footer-img-btn"
                  title="当前模型未开启视觉理解，可在「AI模型服务配置」中开启"
                  :disabled="isStreamLoad"
                  @click="message.warning('当前模型未开启视觉理解，请在「AI模型服务配置」中为该配置打开视觉理解开关，并绑定支持视觉的模型')"
                >
                  <template #icon>
                    <NIcon :component="ImageOutline" />
                  </template>
                </NButton>
                <NInput
                  v-model:value="inputValue"
                  type="textarea"
                  placeholder="输入消息，回车发送... 输入 / 选择技能（可多选），输入 @ 关联股票"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                  :disabled="isStreamLoad"
                  @update:value="onInputUpdate"
                  @keydown="handleInputKeydown"
                  @keydown.enter.exact.prevent="onEnterKey"
                  @paste="onPasteImage"
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
              <input
                ref="imageFileInputRef"
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                multiple
                style="display: none"
                @change="onImageFilesSelected"
              />
            </div>
        </NCard>
      </div>
    </div>

  <NModal
    v-model:show="klineModalShow"
    :title="(klineName || klineCode || '') + ' — 多周期K线'"
    preset="card"
    :z-index="10010"
    :style="KLINE_MODAL_STYLE"
    :content-style="KLINE_MODAL_CONTENT_STYLE"
  >
    <div ref="klineWrapRef">
      <StockLightweightKlineChart
        v-if="klineModalShow"
        :key="'agent-kline-' + klineCode"
        :code="klineCode"
        :stock-name="klineName"
        :dark-theme="darkTheme"
        :chart-height="klineChartHeight"
      />
    </div>
  </NModal>

  <!-- 报告大窗阅读视图：分析报告展开为宽屏弹窗，支持复制 -->
  <NModal
    v-model:show="reportModalShow"
    :title="reportModalTitle + ' — 分析报告'"
    preset="card"
    :z-index="10010"
    style="width: 78vw; max-width: 1100px;"
    content-style="max-height: calc(90vh - 110px); overflow-y: auto; padding: 0 20px;"
  >
    <template #header-extra>
      <NButton size="small" quaternary @click="copyReportContent">复制全文</NButton>
    </template>
    <MdPreview
      :theme="theme"
      :model-value="reportModalContent"
      editor-id="agent-report-modal"
      :sanitize="sanitizeMd"
      class="msg-markdown report-modal-md"
    />
  </NModal>

  <!-- 👎 反馈理由弹窗：采集纠正原因，供画像学习"需规避项/偏好格式" -->
  <NModal
    v-model:show="feedbackDialogShow"
    preset="dialog"
    title="这个回答哪里不行？"
    positive-text="提交反馈"
    negative-text="跳过"
    :z-index="10010"
    @positive-click="confirmFeedbackSubmit"
    @negative-click="submitFeedbackSkip"
    @close="submitFeedbackSkip"
  >
    <div style="display:flex; flex-direction:column; gap:8px; text-align:left;">
      <NSelect
        v-model:value="feedbackReasonPreset"
        :options="feedbackReasonOptions"
        placeholder="选择主要问题（可选）"
        clearable
        size="small"
      />
      <NInput
        v-model:value="feedbackReasonText"
        type="textarea"
        placeholder="补充说明（可选，例如：没结合我的持仓成本）"
        :rows="2"
        maxlength="200"
        show-count
      />
    </div>
  </NModal>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick, onMounted, onBeforeUnmount, onBeforeMount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NEmpty, NIcon, NImage, NInput, NModal, NPopconfirm, NPopover, NScrollbar, NSelect, NSpin, NSwitch, useMessage } from 'naive-ui'
import {
  CloseOutline,
  SparklesOutline,
  ShareSocialOutline,
  ImageOutline,
  AddOutline,
  ChatbubblesOutline,
  SearchOutline,
  TrashOutline,
  CreateOutline,
  BookmarkOutline,
  EyeOutline,
  EyeOffOutline,
  DownloadOutline
} from '@vicons/ionicons5'
import {
  ListFilesystemSkills,
  GetAiConfigs,
  GetConfig,
  GetFollowList,
  GetPromptTemplates,
  GetEffectiveSponsorVip,
  SearchStock,
  ShareText,
  SaveImage,
  SubmitAgentFeedback,
  UploadImageToImageBed,
  ExportSessionMarkdown
} from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import html2canvas from 'html2canvas'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import MessageGroup from './agent/MessageGroup.vue'
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from './kline/useKlineModalFit'
import { sanitizeMd } from './agent/format'
import { useAgentSessions } from './agent/useAgentSessions'
import { useAgentStream, AGENT_EVENT } from './agent/useAgentStream'

const STORAGE_KEY_MODEL_ID = 'go-stock-agent-last-model-id'
const STORAGE_KEY_SYS_PROMPT_ID = 'go-stock-agent-last-sys-prompt-id'
const STORAGE_KEY_USER_PROMPT_ID = 'go-stock-agent-last-user-prompt-id'
const STORAGE_KEY_THINKING_MODE = 'go-stock-agent-thinking-mode'
const STORAGE_KEY_MEMORY_MODE = 'go-stock-agent-memory-mode'
const STORAGE_KEY_MEMORY_COUNT = 'go-stock-agent-memory-count'
const STORAGE_KEY_AGENT_MODE = 'go-stock-agent-mode'
const STORAGE_KEY_SKILL_ID = 'go-stock-agent-skill-id'
const STORAGE_KEY_ACTIVE_SESSION = 'go-stock-agent-active-session'
const STORAGE_KEY_CONCLUSION_ONLY = 'go-stock-agent-conclusion-only'

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
const sessionMenuVisible = ref(false)
const inputValue = ref('')

// ===== 多会话状态仓库（阶段3拆分）：状态/列表/历史持久化在 useAgentSessions =====
// 流式事件处理与发送编排见 useAgentStream（在 aiConfigList 定义之后初始化，因其依赖配置 refs）
const sessionsApi = useAgentSessions({
  scrollToBottom,
  hooks: {
    // 删除会话前停止其流式格式化定时器（定时器由流处理模块持有，此处经钩子解耦）
    beforeDeleteSession: sid => stopFormatTimer(sid)
  }
})
const {
  sessionStates, sessionOrder, activeSessionId, sessionKeyword,
  sessionId, messages, isStreamLoad, reasoningExpandedMap,
  messageGroups, isGroupExpanded, toggleGroup, toggleReasoning,
  renamingSessionId, renamingTitle,
  startRenameSession, confirmRenameSession, cancelRenameSession, togglePinSession,
  refreshSessionList, onSessionSearchInput,
  setActiveSession, deleteSession, createSession, loadHistory
} = sessionsApi

const aiConfigOptions = ref([])
const aiConfigId = ref(null)

const sysPromptTemplates = ref([])
const sysPromptOptions = computed(() =>
  sysPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const sysPromptId = ref(null)

// 技能选择（/ 斜杠指令，支持多选）：选中技能后用技能 SKILL.md 内容覆盖系统提示词，
// 同时技能名以 @技能名 标记追加到输入框，随消息文本一起提交，
// 确保 DeepAgents 子 Agent 委派时也能感知用户指定的技能。
const skills = ref([])
const selectedSkillDirs = ref([])
const selectedSkills = computed(() =>
  selectedSkillDirs.value
    .map(d => skills.value.find(s => s.dirName === d))
    .filter(Boolean)
)
function isSkillSelected(dirName) {
  return selectedSkillDirs.value.includes(dirName)
}
// 技能名在输入框/提交文本中的标记格式
function skillMarker(name) {
  return '@' + name
}
// 技能菜单浮层状态
const skillMenuVisible = ref(false)
const skillMenuIndex = ref(0)
const skillFilterText = ref('')
// 过滤后的技能列表
const filteredSkills = computed(() => {
  const kw = skillFilterText.value.trim().toLowerCase()
  if (!kw) return skills.value
  return skills.value.filter(s =>
    s.name.toLowerCase().includes(kw) || (s.description || '').toLowerCase().includes(kw)
  )
})
const sysPromptDisabled = computed(() => selectedSkillDirs.value.length > 0)

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
const agentMode = ref(localStorage.getItem(STORAGE_KEY_AGENT_MODE) || 'deepagents')
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

const canSend = computed(() => !!inputValue.value.trim() || pendingImages.value.length > 0)

// ===== 视觉理解（图片输入）：仅所选 AI 配置开启「视觉理解」时可用 =====
// 默认外部 URL 图片模式：本地图先托管到免费图床（img.scdn.io）转成外链 URL 再发送，
// 请求体小且多轮对话不膨胀；图床上传失败时自动回退 base64 直传。
const MAX_IMAGE_COUNT = 10
const MAX_IMAGE_SIZE = 8 * 1024 * 1024 // 单张 8MB
const pendingImages = ref([]) // 待发送图片：[{ url: 发送用 URL, preview: 预览图, uploading: 是否上传图床中 }]
const imageUrlInput = ref('')
const imageFileInputRef = ref(null)
const aiConfigList = ref([]) // 原始 AI 配置列表，用于查询 supportVision

// ===== 流式事件处理（阶段3拆分）：流式路由/工具卡片状态机/格式化定时器/发送编排 =====
const {
  onAgentMessage,
  abortStream: abortStreamCore,
  dispatchAgentMessage, regenerateMessage, editUserMessage,
  stopFormatTimer
} = useAgentStream({
  sessions: sessionsApi,
  scrollToBottom,
  config: {
    aiConfigId, aiConfigOptions, sysPromptId, selectedSkillDirs,
    memoryMode, memoryCount, thinkingMode, agentMode,
    inputValue, pendingImages, panelVisible
  }
})

const currentConfigSupportsVision = computed(() => {
  const id = aiConfigId.value ?? aiConfigOptions.value[0]?.value
  const cfg = aiConfigList.value.find(c => Number(c.ID ?? c.id) === Number(id))
  return !!cfg?.supportVision
})

const hasUploadingImage = computed(() => pendingImages.value.some(i => i.uploading))

function triggerImageSelect() {
  imageFileInputRef.value?.click()
}

function onImageFilesSelected(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = '' // 允许重复选择同一文件
  files.forEach(f => addImageFromLocal(f))
}

function readFileAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result)
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

async function addImageFromLocal(file) {
  if (!file.type || !file.type.startsWith('image/')) {
    message.warning('仅支持图片文件')
    return
  }
  if (file.size > MAX_IMAGE_SIZE) {
    message.warning(`图片 ${file.name} 超过 8MB 限制`)
    return
  }
  if (pendingImages.value.length >= MAX_IMAGE_COUNT) {
    message.warning(`最多添加 ${MAX_IMAGE_COUNT} 张图片`)
    return
  }
  let dataUrl
  try {
    dataUrl = await readFileAsDataURL(file)
  } catch (_) {
    message.warning('图片读取失败，请重试')
    return
  }
  if (typeof dataUrl !== 'string' || !dataUrl.startsWith('data:image/')) {
    message.warning('图片读取失败，请重试')
    return
  }
  // 先以本地 base64 作为预览占位，异步上传图床换取外链（reactive 保证上传状态变化触发视图更新）
  const item = reactive({ url: '', preview: dataUrl, uploading: true })
  pendingImages.value.push(item)
  try {
    const url = await UploadImageToImageBed(dataUrl, file.name || 'image.png')
    item.url = url
    item.preview = url // 外链可直接预览
    item.uploading = false
  } catch (e) {
    // 图床不可用时回退 base64 直传（OpenAI 兼容 image_url 同样支持 data URL）
    item.url = dataUrl
    item.uploading = false
    message.warning('图床上传失败，本图将改用 base64 直传（' + (e?.message ?? e) + '）')
  }
}

function addImageUrl() {
  const url = imageUrlInput.value.trim()
  if (!url) return
  if (!/^https?:\/\//i.test(url)) {
    message.warning('请输入 http(s) 图片链接')
    return
  }
  if (pendingImages.value.length >= MAX_IMAGE_COUNT) {
    message.warning(`最多添加 ${MAX_IMAGE_COUNT} 张图片`)
    return
  }
  pendingImages.value.push({ url, preview: url, uploading: false })
  imageUrlInput.value = ''
}

function removePendingImage(index) {
  pendingImages.value.splice(index, 1)
}

// 粘贴图片：直接加入待发送列表（输入框粘贴文本不受影响）
function onPasteImage(e) {
  if (!currentConfigSupportsVision.value) return
  const files = Array.from(e.clipboardData?.files || [])
  const images = files.filter(f => f.type && f.type.startsWith('image/'))
  if (images.length) {
    e.preventDefault()
    images.forEach(f => addImageFromLocal(f))
  }
}

// 切换到不支持视觉的模型时，清空待发送图片避免误发
watch(currentConfigSupportsVision, (v) => {
  if (!v && pendingImages.value.length) {
    pendingImages.value = []
    message.warning('已切换到未开启视觉理解的模型，待发送图片已清空')
  }
})

const scrollbarRef = ref(null)
// 流式期间用户是否主动上翻（距底 > 80px 视为翻阅历史）：true 时暂停自动滚底，避免输出抢占视角
const userScrolledUp = ref(false)

// 消息区滚动：实时判定用户位置；滚回底部附近自动恢复跟随
function onMsgScroll(e) {
  const el = e && e.target
  if (!el || typeof el.scrollHeight !== 'number') return
  const distToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  userScrolledUp.value = distToBottom > 80
}
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
/** 赞助码未生效原因，用于向用户解释为什么 VIP2 权益仍被拦截 */
const vipReason = ref('')
/** 在途的 VIP 校验请求：并发调用共享同一个 Promise，避免其中一次提前返回读到 vipLevel=0 */
let vipInflight = null

// 有任一会话在后台运行且面板未打开时，边角触发按钮显示忙碌角标
const hasBackgroundTask = computed(() =>
  Object.values(sessionStates).some(s => s.isStreamLoad && s.sentFromFloating) && !panelVisible.value
)

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

// K 线弹窗尺寸与图表高度自适应：与全站其他 K 线弹窗统一
const klineWrapRef = ref(null)
const { chartHeight: klineChartHeight, attach: attachKlineFit, detach: detachKlineFit } = useKlineModalFit(klineWrapRef)
watch(klineModalShow, (v) => {
  if (v) {
    attachKlineFit()
    return
  }
  detachKlineFit()
})

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

/** 加载自选列表，构建 名称 → 代码 映射，用于识别 AI 输出中的股票名称；同时保留原始列表供 @引用 展开 */
async function loadFollowListForLinks() {
  try {
    const list = await GetFollowList(0)
    followListRaw.value = Array.isArray(list) ? list : []
    const map = new Map()
    ;(list || []).forEach(item => {
      const name = item.Name || item.StockName || item.stockName || ''
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

    // 处理 md-editor-v3 linkify 自动生成的 <a> 链接（如 600114.SH 被识别为域名）
    // 这些链接的 textContent 就是股票代码，href 为 http://代码 或代码本身
    const autoLinks = preview.querySelectorAll('a:not(.stock-link)')
    autoLinks.forEach(a => {
      const text = (a.textContent || '').trim()
      if (!text) return
      STOCK_CODE_REGEX.lastIndex = 0
      const match = STOCK_CODE_REGEX.exec(text)
      if (!match || match[0] !== text) return
      // 仅处理 linkify 自动链接（href 为代码本身或 http://代码），保留用户真实 markdown 链接
      const href = a.getAttribute('href') || ''
      const code = parseStockCodeMatch(text)
      const isAutoLink = href === text || href === 'http://' + text || href === 'https://' + text
      if (!isAutoLink) return
      const name = nameForCode(code)
      a.classList.add('stock-link')
      a.dataset.code = code
      if (name) a.dataset.name = name
      a.removeAttribute('href')
      a.title = '点击查看 ' + text + ' K线图'
      a.addEventListener('click', (e) => {
        e.preventDefault()
        e.stopPropagation()
        openStockKline(code, name)
      })
    })
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

// ---- 导出会话为 Markdown（参考 ChatGPT/Claude 数据导出）：拼接全部问答，由 Go 弹出系统保存对话框选择位置写入 ----
function exportSessionMarkdown() {
  const list = (messages.value || [])
  if (!list.some(m => m.role === 'user' && m.content)) {
    message.warning('当前会话暂无可导出的对话内容')
    return
  }
  const lines = ['# go-stock AI 助手 · 对话记录', '', `> 会话ID：${sessionId.value} ｜ 导出时间：${new Date().toLocaleString()}`, '']
  for (const m of list) {
    if (m.role === 'user') {
      const imgs = Array.isArray(m.images) && m.images.length ? `\n\n（附图 ${m.images.length} 张）` : ''
      lines.push(`## 🙋 提问`, '', (m.content || '') + imgs, '')
    } else if (m.content || m.jsonMarkdown) {
      const model = m.modelName ? `（${m.modelName}）` : ''
      lines.push(`## 🤖 回答${model}`, '', m.jsonMarkdown || m.content || '', '')
    }
    lines.push('---', '')
  }
  const item = sessionOrder.value.find(s => s.sessionId === activeSessionId.value)
  const safe = (item?.title || 'AI对话').replace(/[\\/:*?"<>|\r\n]/g, '_').slice(0, 40)
  ExportSessionMarkdown(safe, lines.join('\n'))
    .then(res => {
      if (res === 'cancelled') return // 用户取消保存，不提示
      if (res) {
        message.error(res)
        return
      }
      message.success('已导出当前会话为 Markdown 文件')
    })
    .catch(e => {
      message.error('导出失败：' + (e?.message ?? e))
    })
}

// ---- 导出单轮分析为 Markdown（对话卡片"导出MD"按钮）：本次问答另存为 .md，复用 Go 保存对话框 ----
function exportGroupMarkdown(group) {
  const q = group?.userMsg?.content || ''
  const a = group?.assistantMsg
  const body = a?.jsonMarkdown || a?.content || ''
  if (!q && !body) {
    message.warning('本轮暂无可导出的内容')
    return
  }
  const lines = ['# go-stock AI 助手 · 分析记录', '', `> 导出时间：${new Date().toLocaleString()}`, '']
  const imgs = Array.isArray(group.userMsg?.images) && group.userMsg.images.length ? `\n\n（附图 ${group.userMsg.images.length} 张）` : ''
  if (q) lines.push('## 🙋 提问', '', q + imgs, '')
  if (body) {
    const model = a?.modelName ? `（${a.modelName}）` : ''
    lines.push(`## 🤖 分析结果${model}`, '', body, '')
  }
  const digest = q.replace(/\s+/g, ' ').trim().slice(0, 30)
  const safe = (digest || 'AI分析').replace(/[\\/:*?"<>|\r\n]/g, '_') || 'AI分析'
  ExportSessionMarkdown(safe, lines.join('\n'))
    .then(res => {
      if (res === 'cancelled') return
      if (res) {
        message.error(res)
        return
      }
      message.success('已导出本次分析为 Markdown 文件')
    })
    .catch(e => {
      message.error('导出失败：' + (e?.message ?? e))
    })
}

// ---- 引用追问（参考 ChatGPT quote reply）：把某轮回答摘要注入输入框，针对该结论继续提问 ----
function quoteGroup(group) {
  const a = group?.assistantMsg
  const src = (a?.jsonMarkdown || a?.content || '').replace(/[#*`>|]/g, '').replace(/\s+/g, ' ').trim()
  if (!src) {
    message.warning('本轮暂无可引用的内容')
    return
  }
  const q = (group.userMsg?.content || '').replace(/[#*`>|]/g, '').replace(/\s+/g, ' ').trim().slice(0, 60)
  const digest = src.length > 120 ? src.slice(0, 120) + '…' : src
  const block = `> 引用「${q}」的回答：${digest}\n`
  inputValue.value = inputValue.value ? inputValue.value.replace(/\s*$/, '\n') + block : block
  showHint('已引用本轮回答，请继续输入你的问题')
  nextTick(() => document.querySelector('.chat-footer-input textarea')?.focus())
}

// ---- 全局快捷键（参考 Raycast/ChatGPT）：Esc 收起面板、Ctrl+K 新建会话 ----
function onGlobalKeydown(e) {
  if (!panelVisible.value) return
  if (e.key === 'Escape') {
    // 菜单/弹层打开时优先由其自身处理 Esc，不收起面板
    if (skillMenuVisible.value || stockMenuVisible.value) return
    if (reportModalShow.value || feedbackDialogShow.value || klineModalShow.value) return
    closePanel()
  } else if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    if (isStreamLoad.value) {
      message.warning('当前会话正在生成回答，请稍后再新建会话')
      return
    }
    createSession()
    showHint('已新建会话（Ctrl+K）')
  }
}

// 提交对某条回答的反馈（👍 直接提交；👎 先弹理由框，可跳过），group 含 userMsg(问题) 与 assistantMsg(回答)
function submitFeedback(group, rating, reason = '') {
  const question = group.userMsg?.content ?? ''
  const response = group.assistantMsg?.rawContent || group.assistantMsg?.content || ''
  const fb = models.AgentFeedback.createFrom({
    sessionId: sessionId.value,
    question: question,
    response: response,
    rating: rating,
    reason: reason,
    mode: agentMode.value === 'auto' ? '' : agentMode.value,
  })
  SubmitAgentFeedback(fb)
    .then(() => {
      if (group.assistantMsg) group.assistantMsg.feedback = rating
      message.success(rating === 1 ? '感谢反馈，我会继续优化' : '已收到，我会改进')
    })
    .catch((e) => {
      console.error('submit feedback error', e)
    })
}

// ---- 👎 理由弹窗 ----
const feedbackDialogShow = ref(false)
const feedbackReasonPreset = ref(null)
const feedbackReasonText = ref('')
const feedbackTargetGroup = ref(null)
// 预设理由选项：与 user-profile.vue 的 classifyFeedbackReason 分类对应，便于画像学习归类
const feedbackReasonOptions = [
  {label: '数据不准 / 过时', value: '数据不准'},
  {label: '逻辑推理有误', value: '逻辑有误'},
  {label: '太啰嗦 / 格式不佳', value: '太啰嗦'},
  {label: '风险提示不合我的风格', value: '风险偏好不符'},
  {label: '没结合我的持仓 / 关注', value: '没结合我的持仓'},
]

function openFeedbackDialog(group) {
  if (!group) return
  feedbackTargetGroup.value = group
  feedbackReasonPreset.value = null
  feedbackReasonText.value = ''
  feedbackDialogShow.value = true
}

// 拼接预设 + 自由文本
function buildFeedbackReason() {
  return [feedbackReasonPreset.value, feedbackReasonText.value.trim()]
    .filter(Boolean).join('；')
}

function confirmFeedbackSubmit() {
  const group = feedbackTargetGroup.value
  feedbackDialogShow.value = false
  if (group) submitFeedback(group, -1, buildFeedbackReason())
}

// 跳过：不填理由直接提交 👎
function submitFeedbackSkip() {
  const group = feedbackTargetGroup.value
  feedbackDialogShow.value = false
  if (group) submitFeedback(group, -1, '')
}

function shareTextToCommunity(text, title) {
  if (shareLoading.value) return
  shareLoading.value = true
  shareTipText.value = '正在分享到社区...'
  shareTipVisible.value = true
  // title 传用户提问；后端优先从正文提取标题，提取不到时用 title 兜底
  ShareText(text, title || '')
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

function findPrecedingUserQuestion(assistantMsg) {
  if (!assistantMsg) return ''
  const idx = messages.value.indexOf(assistantMsg)
  if (idx < 0) return ''
  for (let i = idx - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m?.role === 'user') {
      const q = (m?.content ?? '').trim()
      if (q) return q
    }
  }
  return ''
}

function shareAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 正文内容'
    shareTipVisible.value = true
    return
  }
  // title 传该回复对应的用户提问，后端提取不到标题时用它兜底
  shareTextToCommunity(text, findPrecedingUserQuestion(msg))
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

function getLastUserQuestion() {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m?.role === 'user') {
      const q = (m?.content ?? '').trim()
      if (q) return q
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
  // title 传最近的用户提问，后端提取不到标题时用它兜底
  shareTextToCommunity(text, getLastUserQuestion())
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

// ---- 报告大窗阅读视图（参考 Claude Artifacts）：分析报告展开为宽屏弹窗，长报告阅读更舒适 ----
const reportModalShow = ref(false)
const reportModalTitle = ref('')
const reportModalContent = ref('')

function openReportModal(group) {
  const md = group?.assistantMsg?.jsonMarkdown
  if (!md) {
    shareTipText.value = '暂无分析报告内容'
    shareTipVisible.value = true
    return
  }
  reportModalTitle.value = (getLastUserQuestion() || '分析报告').slice(0, 60)
  reportModalContent.value = md
  reportModalShow.value = true
}

async function copyReportContent() {
  const text = (reportModalContent.value ?? '').trim()
  if (!text) return
  try {
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
      message.success('已复制报告内容')
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      message.success('已复制报告内容')
    }
  } catch (e) {
    message.error('复制失败，请手动选择文本')
  }
}

// 中断当前会话流：核心收尾在 useAgentStream.abortStream，这里负责 UI 提示
function abortStream(showTip = true, sid = activeSessionId.value) {
  const aborted = abortStreamCore(sid)
  if (aborted && showTip) {
    shareTipText.value = '已中断本次 AI 回答'
    shareTipVisible.value = true
  }
}

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))

function openPanel() {
  // 每次打开面板刷新 AI 配置列表：设置页的改动（如开启视觉理解）及时生效
  loadAiConfigs()
  panelVisible.value = true
  if (!activeSessionId.value) {
    createSession()
  }
  // 加载自选列表用于 AI 输出中识别股票名称
  loadFollowListForLinks()
  nextTick(() => {
    initDefaultExpanded()
    scrollToBottom(true)
  })
}

function closePanel() {
  panelVisible.value = false
}

async function ensureVipInfo() {
  // 注意：不能缓存结果。改用 GetEffectiveSponsorVip（后端每次同步本地解密并判断有效期，无网络 IO），
  // 旧方案读 GetSponsorInfo 依赖启动后台 goroutine（CheckUpdate）异步填充 SponsorInfo，
  // 启动早期预加载会读到空值并把 vipLevel=0 固化，导致 VIP2 用户被误拦。
  // 并发调用必须等待同一个在途请求，不能直接 return：否则调用方会在 vipLevel 仍为 0 时继续判断，误拦 VIP2 用户
  if (vipInflight) return vipInflight
  vipInflight = (async () => {
    try {
      const res = await GetEffectiveSponsorVip()
      const lvl = Number(res?.vipLevel ?? 0)
      const active = !!res?.active
      vipLevel.value = active && !Number.isNaN(lvl) ? lvl : 0
      vipReason.value = active ? '' : String(res?.reason ?? '')
    } catch (_) {
      vipLevel.value = 0
      vipReason.value = ''
    }
  })()
  try {
    await vipInflight
  } finally {
    vipInflight = null
  }
}

async function togglePanel() {
  if (!panelVisible.value) {
    // 每次打开前重新校验（后端为同步本地解密，微秒级，不影响打开速度）
    await ensureVipInfo()
    if ((vipLevel.value ?? 0) < 2) {
      message.warning(vipReason.value
        ? `go-stock AI Agent 助手需要 VIP2 及以上有效赞助：${vipReason.value}`
        : 'go-stock AI Agent 助手功能仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
      return
    }
    openPanel()
  } else {
    closePanel()
  }
}

function scrollToBottom(force = false) {
  // 用户翻阅历史时不打扰；发送消息/切换会话/打开面板等场景 force=true 强制跟随
  if (!force && userScrolledUp.value) return
  nextTick(() => {
    scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
  })
}

function sendMessage() {
  if (isStreamLoad.value) {
    abortStream(false)
  }
  if (hasUploadingImage.value) {
    message.warning('图片正在上传图床，请稍候再发送')
    return
  }
  let text = inputValue.value.trim()
  const images = pendingImages.value.map(i => i.url).filter(Boolean)
  if (!text && images.length === 0) {
    message.warning('请输入你的问题')
    return
  }
  // 视觉图片仅发送给开启了「视觉理解」的模型
  if (images.length > 0 && !currentConfigSupportsVision.value) {
    message.warning('当前模型未开启视觉理解，无法发送图片。请在「AI模型服务配置」中开启该选项，或删除图片后重试')
    return
  }
  // 纯图片提问时补充默认文本
  if (!text) text = '请分析这些图片'
  // 已选技能名（@技能名 标记）随消息文本一起提交；缓存恢复场景输入框可能没有标记，此处补齐
  const missingMarkers = selectedSkills.value
    .map(s => skillMarker(s.name))
    .filter(m => !text.includes(m))
  if (missingMarkers.length) {
    text = missingMarkers.join(' ') + ' ' + text
  }
  skillMenuVisible.value = false
  stockMenuVisible.value = false
  stockResults.value = []
  // @引用标记展开：@我的持仓/@我的自选股 替换为上下文数据块（引用在前、正文在后）
  text = expandAtReferences(text)
  dispatchAgentMessage(text, images)
}

// ---- 快捷指令：预设高频问题，点击后走 sendMessage 完整链路（含 VIP 校验/技能标记） ----
const quickPrompts = [
  { icon: '📊', label: '今日复盘', text: '帮我做今日A股复盘：大盘走势、领涨领跌板块、市场情绪和明日关注要点' },
  { icon: '⚡', label: '异动归因', text: '分析今天盘面异动板块的上涨原因和持续性' },
  { icon: '🔍', label: '分析个股', text: '帮我分析一只股票的投资价值，包括基本面、技术面和资金面（请告诉我股票名称或代码）' },
  { icon: '📰', label: '市场热点', text: '梳理当前市场热点事件、受益板块和相关机会' }
]

function sendQuickPrompt(p) {
  if (isStreamLoad.value) return
  inputValue.value = p.text
  sendMessage()
}

// ---- 只看结论模式（参考 Manus 行动时间线折叠）：隐藏任务清单/步骤/工具卡片/思考区，仅保留正文与报告 ----
const conclusionOnly = ref(loadBool(STORAGE_KEY_CONCLUSION_ONLY, false))
watch(conclusionOnly, (v) => localStorage.setItem(STORAGE_KEY_CONCLUSION_ONLY, String(v)))

// ---- 建议追问 chips（参考 ChatGPT/Kimi）：回复结束后按上下文生成一键追问 ----
// 客户端规则生成（不依赖模型输出解析）：从最后回复中提取股票代码生成标的类追问，
// 无标的时给通用追问（展开深度/总结要点/风险信号），流式开始时清空、结束时重建。
const followUpChips = ref([])

function extractCodesFromText(text) {
  const codes = []
  // 复制正则源避免共享 lastIndex 污染 linkify 逻辑
  const re = new RegExp(STOCK_CODE_REGEX.source, 'g')
  let m
  while ((m = re.exec(text)) !== null) {
    const code = parseStockCodeMatch(m[0])
    if (!codes.includes(code)) codes.push(code)
    if (codes.length >= 2) break
  }
  return codes
}

function buildFollowUpChips() {
  followUpChips.value = []
  const last = getLastAssistantContent()
  if (!last) return
  const chips = []
  for (const code of extractCodesFromText(last)) {
    const label = nameForCode(code) || code
    if (chips.length === 0) {
      chips.push({ icon: '📈', label: `分析 ${label} K线`, text: `请分析 ${code} 的K线技术形态，判断当前趋势、关键支撑与压力位` })
    } else {
      chips.push({ icon: '💰', label: `查 ${label} 资金面`, text: `查一下 ${code} 的主力资金流向和近期龙虎榜动向` })
    }
    if (chips.length >= 2) break
  }
  chips.push({ icon: '⚠️', label: '风险与跟踪信号', text: '上面的分析有哪些主要风险点和需要持续跟踪的信号？' })
  followUpChips.value = chips.slice(0, 3)
}

function sendFollowUpChip(chip) {
  if (isStreamLoad.value) return
  inputValue.value = chip.text
  followUpChips.value = []
  sendMessage()
}

// 流结束（含重新生成/编辑重发）后按最新回复重建追问；开始新一轮流式时清空；切换会话时按该会话重建
watch(isStreamLoad, (loading, old) => {
  if (loading) {
    followUpChips.value = []
  } else if (old) {
    nextTick(() => buildFollowUpChips())
  }
})
// 切换会话时保存/恢复各自未发送的输入草稿（参考 Claude/Cursor），避免多会话草稿互相覆盖
watch(activeSessionId, (nv, ov) => {
  if (ov && sessionStates[ov]) sessionStates[ov].draft = inputValue.value
  inputValue.value = (nv && sessionStates[nv]?.draft) || ''
  followUpChips.value = []
  nextTick(() => buildFollowUpChips())
})

// 文本格式化与流式解析纯函数见 ./agent/format.js、./agent/streamParse.js（阶段1拆分）；
// 消息组渲染（含 sanitizeMd 及消息样式）已拆分至 ./agent/MessageGroup.vue（阶段2组件拆分）；
// 会话状态与历史持久化已拆分至 ./agent/useAgentSessions.js，
// 流式事件处理（onAgentMessage/工具卡片状态机/格式化定时器/发送编排）已拆分至 ./agent/useAgentStream.js（阶段3拆分）


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

// 加载技能列表并恢复缓存选择（与技能管理页面同源：文件系统技能，支持多选）。
// 停用技能（技能管理页开关关闭）不进入可选列表，已选中的停用技能会被自动移除。
function loadSkills() {
  ListFilesystemSkills().then(res => {
    skills.value = (Array.isArray(res) ? res : []).filter(s => !s.disabled)
    // 已选技能中若有被停用的，自动移除并同步持久化
    if (selectedSkillDirs.value.length) {
      const valid = selectedSkillDirs.value.filter(d => skills.value.some(s => s.dirName === d))
      if (valid.length !== selectedSkillDirs.value.length) {
        selectedSkillDirs.value = valid
        persistSkills()
      }
    }
    if (!selectedSkillDirs.value.length) {
      const cached = localStorage.getItem(STORAGE_KEY_SKILL_ID)
      if (cached) {
        const dirs = cached.split(',').filter(d => skills.value.some(s => s.dirName === d))
        if (dirs.length) selectedSkillDirs.value = dirs
      }
    }
  }).catch(() => {})
}

// 持久化已选技能（逗号分隔，支持多选）
function persistSkills() {
  if (selectedSkillDirs.value.length) {
    localStorage.setItem(STORAGE_KEY_SKILL_ID, selectedSkillDirs.value.join(','))
  } else {
    localStorage.removeItem(STORAGE_KEY_SKILL_ID)
  }
}

// 检测输入框内容是否为 / 斜杠指令（匹配最后一个 / 开头的词，便于在已选技能标记后继续追加）
function checkSlashCommand(val) {
  const m = val.match(/(?:^|\s)\/([^\s]*)$/)
  if (m) {
    skillFilterText.value = m[1]
    skillMenuVisible.value = true
    skillMenuIndex.value = 0
    stockMenuVisible.value = false
  } else {
    skillMenuVisible.value = false
  }
}

// ---- / 斜杠快捷指令（参考 Cursor/豆包）：预设提示词模板，与技能菜单合并展示、统一键盘导航 ----
const slashCommands = [
  { cmd: '/复盘', desc: '今日A股全面复盘', text: '帮我做今日A股复盘：大盘走势、领涨领跌板块、市场情绪和明日关注要点' },
  { cmd: '/对比', desc: '多只个股横向对比', text: '帮我对比分析以下几只股票的基本面、技术面和资金面，并给出综合排序：' },
  { cmd: '/盯盘', desc: '盘中快速盯盘要点', text: '现在是盘中时段，请快速分析当前大盘走势与板块异动，给出盯盘要点和需要警惕的信号' },
  { cmd: '/估值', desc: '个股估值水平分析', text: '请结合历史估值分位与同行业对比，分析这只股票当前估值是高估还是低估（股票：' }
]

const filteredSlashCommands = computed(() => {
  const kw = skillFilterText.value.trim().toLowerCase()
  if (!kw) return slashCommands
  return slashCommands.filter(c =>
    c.cmd.slice(1).toLowerCase().includes(kw) || c.desc.toLowerCase().includes(kw)
  )
})

// 合并导航列表：指令在前、技能在后，skillMenuIndex 作为统一索引
const slashMenuItems = computed(() => [
  ...filteredSlashCommands.value.map(c => ({
    type: 'cmd', key: c.cmd, icon: '⌨️', name: c.cmd, desc: c.desc, cmd: c
  })),
  ...filteredSkills.value.map(s => ({
    type: 'skill', key: s.dirName, icon: isSkillSelected(s.dirName) ? '✅' : '🎯', name: s.name, desc: s.description, skill: s
  }))
])

// 统一分发：指令条目填入提示词模板，技能条目走原有多选逻辑
function selectSlashItem() {
  const item = slashMenuItems.value[skillMenuIndex.value]
  if (!item) return
  if (item.type === 'cmd') {
    inputValue.value = inputValue.value.replace(/(?:^|\s)\/[^\s]*$/, item.cmd.text)
    skillMenuVisible.value = false
    showHint(`已插入指令「${item.cmd.cmd}」模板，请补充股票代码或细节`)
    return
  }
  selectSkillFromMenu()
}

// ---- @ 股票联想：输入 @关键词 防抖搜索东财智能选股接口，选中后替换为 股票名称(代码) ----
// 另含引用特殊条目（参考 Cursor @files 上下文注入）：@我的持仓 / @我的自选股，
// 发送时由 expandAtReferences 将标记展开为上下文数据块（数据来自 GetFollowList）。
const stockMenuVisible = ref(false)
const stockMenuIndex = ref(0)
const stockResults = ref([])
let stockSearchTimer = null

// 自选列表原始数据（FollowedStock：StockCode/Name/Volume/CostPrice/Price...），供 @引用 展开
const followListRaw = ref([])
// @引用 标记与消息正文中的展开块前缀
const AT_REF_POSITION_MARKER = '@我的持仓'
const AT_REF_WATCHLIST_MARKER = '@我的自选股'
const AT_SPECIAL_ITEMS = [
  { special: 'positions', icon: '💰', name: AT_REF_POSITION_MARKER.slice(1), match: ['持仓', '仓位', '成本'], desc: '发送时注入持仓成本/现价/盈亏' },
  { special: 'watchlist', icon: '📌', name: AT_REF_WATCHLIST_MARKER.slice(1), match: ['自选', '关注'], desc: '发送时注入全部自选股列表' }
]

// 输入统一入口：/ 触发技能+指令菜单，@ 触发股票联想与引用条目（互斥，技能菜单优先）
function onInputUpdate(val) {
  checkSlashCommand(val)
  checkAtMention(val)
}

function checkAtMention(val) {
  const m = val.match(/(?:^|\s)@([^\s]*)$/)
  // 技能菜单打开时不出股票菜单
  if (!m || skillMenuVisible.value) {
    stockMenuVisible.value = false
    stockResults.value = []
    return
  }
  const kw = m[1]
  const matchedSpecials = AT_SPECIAL_ITEMS
    .filter(s => !kw || s.name.includes(kw) || s.match.some(k => k.startsWith(kw) || k.includes(kw)))
    .map(s => ({ ...s }))
  // @ 后为空：直接显示引用条目，不触发网络搜索
  if (!kw) {
    stockResults.value = matchedSpecials
    stockMenuIndex.value = 0
    stockMenuVisible.value = matchedSpecials.length > 0
    return
  }
  if (stockSearchTimer) clearTimeout(stockSearchTimer)
  stockSearchTimer = setTimeout(async () => {
    let searched = []
    try {
      const res = await SearchStock(kw)
      const dataList = res && res.code === 100 && res.data?.result?.dataList ? res.data.result.dataList : []
      // 按代码去重（接口偶尔对同一标的返回多条），再截断条数
      const seen = new Set()
      searched = dataList
        .map(r => ({
          name: r.SECURITY_NAME_ABBR || r.SECURITY_NAME || '',
          code: r.SECURITY_CODE || '',
          market: (r.MARKET_SHORT_NAME || '').toLowerCase()
        }))
        .filter(s => {
          if (!s.code || !s.name || seen.has(s.code)) return false
          seen.add(s.code)
          return true
        })
        .slice(0, 8)
    } catch (_) {
      searched = []
    }
    stockResults.value = [...matchedSpecials, ...searched]
    stockMenuIndex.value = 0
    stockMenuVisible.value = stockResults.value.length > 0
  }, 300)
}

// 选中条目：引用条目插入 @标记（发送时展开为数据块）；股票条目替换为 股票名称(代码)
function selectStockFromMenu() {
  const s = stockResults.value[stockMenuIndex.value]
  if (!s) return
  if (s.special) {
    const marker = s.special === 'positions' ? AT_REF_POSITION_MARKER : AT_REF_WATCHLIST_MARKER
    inputValue.value = inputValue.value.replace(/(?:^|\s)@[^\s]*$/, ' ' + marker + ' ')
    stockMenuVisible.value = false
    stockResults.value = []
    showHint(`已插入引用「${s.name}」，发送时将自动附上${s.special === 'positions' ? '持仓明细' : '自选股列表'}`)
    return
  }
  inputValue.value = inputValue.value.replace(/(?:^|\s)@[^\s]*$/, ' ' + s.name + '(' + s.market + s.code + ') ')
  stockMenuVisible.value = false
  stockResults.value = []
  showHint(`已插入股票「${s.name}(${s.market}${s.code})」`)
}

// 将消息文本中的 @引用 标记展开为上下文数据块（引用块在前、正文在后），无标记时原样返回
function expandAtReferences(text) {
  let body = text
  const blocks = []
  const stocks = followListRaw.value || []
  const itemName = it => it.Name || it.StockName || it.stockName || it.name || ''
  const itemCode = it => it.StockCode || it.stockCode || it.code || ''
  if (text.includes(AT_REF_POSITION_MARKER)) {
    body = body.split(AT_REF_POSITION_MARKER).join('')
    const held = stocks.filter(s => (s.Volume ?? 0) > 0)
    if (held.length) {
      const lines = held.map(s => {
        const cost = Number(s.CostPrice ?? 0)
        const price = Number(s.Price ?? 0)
        const pnl = cost > 0 && price > 0 ? `，浮盈 ${(((price - cost) / cost) * 100).toFixed(2)}%` : ''
        return `- ${itemName(s)}(${itemCode(s)})：成本 ${cost.toFixed(2)}，现价 ${price.toFixed(2)}，持仓 ${s.Volume ?? 0} 股${pnl}`
      })
      blocks.push('【引用 · 我的持仓】\n' + lines.join('\n') + '\n请优先结合以上持仓成本分析盈亏与操作建议。')
    } else {
      blocks.push('【引用 · 我的持仓】（当前未设置持仓成本，请提示我先在股票列表为个股设置成本与持仓量）')
    }
  }
  if (text.includes(AT_REF_WATCHLIST_MARKER)) {
    body = body.split(AT_REF_WATCHLIST_MARKER).join('')
    if (stocks.length) {
      const MAX_ITEMS = 30
      const items = stocks.slice(0, MAX_ITEMS).map(s => `${itemName(s)}(${itemCode(s)})`)
      const more = stocks.length > MAX_ITEMS ? ` 等共 ${stocks.length} 只` : ''
      blocks.push('【引用 · 我的自选股】\n' + items.join('、') + more)
    } else {
      blocks.push('【引用 · 我的自选股】（自选股列表为空）')
    }
  }
  if (!blocks.length) return text
  const cleaned = body.replace(/\s{2,}/g, ' ').trim()
  return blocks.join('\n\n') + '\n\n' + cleaned
}

// 处理输入框按键：股票菜单 / 技能菜单可见时拦截导航键（股票菜单优先）
function handleInputKeydown(e) {
  if (stockMenuVisible.value && stockResults.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      stockMenuIndex.value = (stockMenuIndex.value + 1) % stockResults.value.length
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      stockMenuIndex.value = (stockMenuIndex.value - 1 + stockResults.value.length) % stockResults.value.length
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      selectStockFromMenu()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      stockMenuVisible.value = false
    }
    return
  }
  if (skillMenuVisible.value && slashMenuItems.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      skillMenuIndex.value = (skillMenuIndex.value + 1) % slashMenuItems.value.length
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      skillMenuIndex.value = (skillMenuIndex.value - 1 + slashMenuItems.value.length) % slashMenuItems.value.length
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      selectSlashItem()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      skillMenuVisible.value = false
    }
  }
}

// 回车发送守卫：股票/技能菜单打开时回车用于选择，不发送消息
function onEnterKey() {
  if (stockMenuVisible.value && stockResults.value.length > 0) {
    return
  }
  if (skillMenuVisible.value && slashMenuItems.value.length > 0) {
    return
  }
  sendMessage()
}

// 从浮层选中/取消技能（可多选）：技能名以 @技能名 追加到输入框，随消息一起提交
function selectSkillFromMenu() {
  // 合并导航列表中技能条目在指令之后，须从 slashMenuItems 按统一索引取，避免 filteredSkills 索引偏移
  const item = slashMenuItems.value[skillMenuIndex.value]
  const skill = item && item.type === 'skill' ? item.skill : null
  if (!skill) return
  if (isSkillSelected(skill.dirName)) {
    removeSkill(skill.dirName)
    return
  }
  // 先移除输入框末尾的 /xxx 过滤词，再追加技能名标记
  inputValue.value = inputValue.value.replace(/(?:^|\s)\/[^\s]*$/, '')
  selectedSkillDirs.value.push(skill.dirName)
  persistSkills()
  // 技能名追加到输入框，作为提示随消息一起提交
  const marker = skillMarker(skill.name)
  if (!inputValue.value.includes(marker)) {
    inputValue.value = (inputValue.value ? inputValue.value.trimEnd() + ' ' : '') + marker + ' '
  }
  // 菜单保持打开便于继续多选
  skillFilterText.value = ''
  skillMenuIndex.value = 0
  skillMenuVisible.value = true
  showHint(`已选择技能「${skill.name}」，技能名已加入输入框，将随消息一起提交`)
}

// 移除已选技能：同步删除输入框中对应的 @技能名 标记
function removeSkill(dirName) {
  const idx = selectedSkillDirs.value.indexOf(dirName)
  if (idx < 0) return
  selectedSkillDirs.value.splice(idx, 1)
  persistSkills()
  const s = skills.value.find(x => x.dirName === dirName)
  if (s) {
    const marker = skillMarker(s.name)
    // 删除标记及其后跟随的多余空格（技能名做正则转义，避免特殊字符干扰）
    const escaped = marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    inputValue.value = inputValue.value
      .replace(new RegExp(escaped + '\\s*', 'g'), '')
      .replace(/\s{2,}/g, ' ')
      .trimStart()
  }
}

watch(panelVisible, (v) => {
  if (v) {
    loadPromptTemplates()
    loadSkills()
    nextTick(() => scrollToBottom(true))
  }
})

// 打开会话菜单时重置搜索并加载最近 10 条，保证默认只展示近期会话
watch(sessionMenuVisible, (v) => {
  if (v) {
    sessionKeyword.value = ''
    refreshSessionList('')
  }
})

onBeforeMount(() => {
  GetConfig().then(result => {
    darkTheme.value = result.darkTheme
  })
})

// 加载 AI 配置列表：挂载、每次打开面板、设置页保存（updateSettings 事件）后均会调用，
// 保证「AI模型服务配置」中开启视觉理解等变更及时反映到图片按钮与配置下拉。
function loadAiConfigs() {
  GetAiConfigs().then(res => {
    const list = Array.isArray(res) ? res : []
    aiConfigList.value = list
    aiConfigOptions.value = list.map((c, index) => {
      const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index)
      const name = c.name ?? c.Name ?? ''
      const modelName = c.modelName ?? c.ModelName ?? ''
      return {
        label: name + (modelName ? ' [' + modelName + ']' : '') + (c.supportVision ? ' [视觉]' : ''),
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
}

onMounted(() => {
  EventsOn(AGENT_EVENT, onAgentMessage)
  // 设置页保存 AI 配置后广播 updateSettings，刷新配置列表（视觉理解开关等及时生效）
  EventsOn('updateSettings', loadAiConfigs)
  // 全局快捷键：Esc 收起面板 / Ctrl+K 新建会话（仅面板打开时生效，见 onGlobalKeydown）
  document.addEventListener('keydown', onGlobalKeydown)
  // 预加载技能列表，首次点击打开抽屉时无需等待（VIP 校验须在打开时实时获取，见 ensureVipInfo）
  loadSkills()
  loadHistory()
  loadAiConfigs()
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
  EventsOff('updateSettings')
  document.removeEventListener('keydown', onGlobalKeydown)
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

/* 抽屉容器：常驻渲染，关闭态隐藏且不响应交互，打开时瞬时可见 */
.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  visibility: hidden;
  transition: visibility 0s 0.25s;
}
.drawer-wrap.drawer-open {
  visibility: visible;
  transition: visibility 0s;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.25s ease;
}
.drawer-wrap.drawer-open .drawer-mask {
  opacity: 1;
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
  transform: translateX(100%);
  transition: transform 0.25s ease;
}
.drawer-wrap.drawer-open .drawer-panel {
  transform: translateX(0);
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

/* 会话列表（header 会话按钮弹出的 NPopover 内容） */
.session-menu {
  display: flex;
  flex-direction: column;
  width: 300px;
  max-width: calc(100vw - 48px);
  max-height: 420px;
  background: var(--n-color, #fff);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.16);
  overflow: hidden;
}
.session-menu-search {
  flex-shrink: 0;
  padding: 8px 8px 4px;
}
.session-menu-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px;
}
.session-menu-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
}
.session-menu-item:hover {
  background: rgba(128, 128, 128, 0.12);
}
.session-menu-item-active {
  background: rgba(128, 128, 128, 0.18);
}
.session-menu-item-main {
  flex: 1;
  min-width: 0;
}
.session-menu-item-title {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.session-menu-item-time {
  font-size: 11px;
  opacity: 0.6;
}
.session-menu-item-del {
  opacity: 0.7;
  flex-shrink: 0;
}
/* 会话重命名/置顶操作按钮：默认淡显，悬停会话项时高亮 */
.session-menu-item-act {
  opacity: 0;
  flex-shrink: 0;
  transition: opacity 0.15s;
}
.session-menu-item:hover .session-menu-item-act {
  opacity: 0.7;
}
.session-menu-item:hover .session-menu-item-act:hover {
  opacity: 1;
}
.session-pin-mark {
  margin-right: 2px;
  font-size: 11px;
}
.session-pin-active {
  color: #409eff;
}
/* 快捷指令 chips */
.chat-footer-quick {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}
.quick-chip {
  border: 1px solid var(--n-border-color);
  background: var(--n-color-hover);
  color: var(--n-text-color-2);
  border-radius: 14px;
  padding: 3px 10px;
  font-size: 12px;
  line-height: 1.4;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s;
}
.quick-chip:hover {
  border-color: #409eff;
  color: #409eff;
  background: rgba(64, 158, 255, 0.08);
}
.quick-chip.dark {
  border-color: rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.75);
}
/* 追问 chips：区别于预设快捷指令的强调色，提示"基于上一轮回答生成" */
.follow-up-chip {
  border-color: rgba(103, 126, 234, 0.45);
  background: rgba(103, 126, 234, 0.1);
  color: #667eea;
}
.follow-up-chip:hover {
  border-color: #667eea;
  color: #667eea;
  background: rgba(103, 126, 234, 0.18);
}
.follow-up-chip.dark {
  border-color: rgba(139, 157, 255, 0.4);
  background: rgba(139, 157, 255, 0.12);
  color: #8b9dff;
}
/* 只看结论开关激活态：图标着色提示当前处于精简视图 */
.conclusion-only-active {
  color: #667eea;
}
/* 报告大窗：正文左对齐，浅色边框卡片感 */
.report-modal-md {
  text-align: left;
  padding: 4px 0 16px;
}
.session-running-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #18a058;
  flex-shrink: 0;
  animation: session-dot-pulse 1.2s ease-in-out infinite;
}
@keyframes session-dot-pulse {
  0%, 100% { opacity: 0.35; }
  50% { opacity: 1; }
}
.session-menu-footer {
  flex-shrink: 0;
  padding: 6px;
  border-top: 1px solid rgba(128, 128, 128, 0.2);
}

.chat-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}
/* 回到底部悬浮按钮：上翻阅读历史时出现在消息区右下角 */
.scroll-bottom-btn {
  position: absolute;
  right: 16px;
  bottom: 12px;
  z-index: 5;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 1px solid #e0e0e6;
  background: #fff;
  color: #555;
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, .12);
  display: flex;
  align-items: center;
  justify-content: center;
}
.scroll-bottom-btn:hover {
  background: #f5f5fa;
}
.scroll-bottom-btn.dark {
  background: #2a2a2e;
  border-color: #444;
  color: #ccc;
}
.scroll-bottom-btn.dark:hover {
  background: #333;
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
.chat-footer-skill-tag {
  padding: 0 2px 4px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.skill-menu {
  position: absolute;
  bottom: 100%;
  left: 0;
  right: 0;
  max-height: 240px;
  overflow-y: auto;
  background: #fff;
  border: 1px solid #e0e0e6;
  border-radius: 6px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, .15);
  z-index: 10003;
  margin-bottom: 4px;
}
.skill-menu.dark {
  background: #18181c;
  border-color: #333;
}
.skill-menu-item {
  padding: 8px 12px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: left;
}
.skill-menu-item:hover,
.skill-menu-item.active {
  background: #f5f5f5;
}
.skill-menu.dark .skill-menu-item:hover,
.skill-menu.dark .skill-menu-item.active {
  background: #2a2a2e;
}
.skill-menu-name {
  font-size: 13px;
  font-weight: 500;
}
.skill-menu-desc {
  font-size: 11px;
  opacity: .6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.skill-menu-footer {
  position: sticky;
  bottom: 0;
  padding: 5px 12px;
  font-size: 11px;
  opacity: .65;
  background: #f5f5f7;
  border-top: 1px solid #e0e0e6;
}
.skill-menu.dark .skill-menu-footer {
  background: #202024;
  border-top-color: #333;
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
/* 图片按钮 */
.chat-footer-img-btn {
  flex-shrink: 0;
}
/* 待发送图片预览条 */
.pending-images {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 8px;
}
.pending-image-item {
  position: relative;
}
.pending-image-thumb {
  width: 64px;
  height: 64px;
  border-radius: 8px;
  overflow: hidden;
}
.pending-image-uploading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.35);
  border-radius: 8px;
}
.pending-image-remove {
  position: absolute;
  top: -7px;
  right: -7px;
  z-index: 1;
}
/* 图片添加弹层 */
.image-add-popover {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 260px;
}
.image-url-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.image-url-row .n-input {
  flex: 1;
}
.image-add-tip {
  font-size: 12px;
  color: var(--n-text-color-3);
  line-height: 1.5;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
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
