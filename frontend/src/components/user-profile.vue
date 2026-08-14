<template>
  <div class="user-profile-page">
    <n-card class="profile-card" title="我的画像" :bordered="false">
      <template #header-extra>
        <n-space>
          <n-switch
            v-model:value="profileEnabled"
            :loading="togglingProfile"
            size="small"
            @update:value="toggleProfileEnabled"
          >
            <template #checked>注入 Agent</template>
            <template #unchecked>暂停注入</template>
          </n-switch>
          <n-button size="small" type="primary" :loading="learning" @click="relearn">一键重新学习</n-button>
        </n-space>
      </template>

      <!-- 反馈统计 -->
      <n-grid :cols="4" :x-gap="12" responsive="screen" class="stats-grid">
        <n-grid-item>
          <n-card size="small" :bordered="true" class="stat-card">
            <div class="stat-num">{{ stats.total }}</div>
            <div class="stat-label">总反馈</div>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small" :bordered="true" class="stat-card stat-up">
            <div class="stat-num">{{ stats.upCount }}</div>
            <div class="stat-label">👍 有用</div>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small" :bordered="true" class="stat-card stat-down">
            <div class="stat-num">{{ stats.downCount }}</div>
            <div class="stat-label">👎 没用</div>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small" :bordered="true" class="stat-card">
            <div class="stat-num">{{ stats.upRate.toFixed(1) }}%</div>
            <div class="stat-label">采纳率</div>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- 画像编辑器 -->
      <div class="profile-editor">
        <div class="editor-tip">
          以下为用户画像（自动学习生成，可手动修改覆盖）。该画像会在对话开始时注入 Agent，用于跨会话记住你的偏好。
          关闭“注入 Agent”后，画像仍会保留，但不会参与后续对话。
        </div>
        <div class="profile-meta">
          <span>画像完整度：{{ profileCompleteness }}%</span>
          <span>已识别 {{ profileKnownFields }}/{{ profileTotalFields }} 项</span>
          <span>更新时间：{{ profileUpdatedAt || '尚未生成' }}</span>
        </div>
        <n-alert v-if="!profileEnabled" type="warning" class="profile-alert" :bordered="false">
          当前画像已暂停注入，Agent 会按通用规则回答，不会使用这里的个性化偏好。
        </n-alert>
        <div class="editor-toolbar">
          <n-space>
            <n-radio-group v-model:value="editMode" size="small">
              <n-radio-button value="preview">预览</n-radio-button>
              <n-radio-button value="edit">编辑</n-radio-button>
            </n-radio-group>
          </n-space>
        </div>
        <div class="editor-body" style="text-align:left;">
          <MdEditor v-if="editMode === 'edit'" v-model="profile" :theme="theme" class="profile-md-editor" />
          <MdPreview
            v-else
            :model-value="profile"
            :theme="theme"
            :style="{ textAlign: 'left' }"
            class="profile-md-preview"
          />
        </div>
        <div class="editor-actions">
          <n-button size="small" type="primary" :loading="saving" @click="save">保存修改</n-button>
          <n-button size="small" :disabled="!lastDiff.length" @click="showDiff = true">查看最近变更</n-button>
          <n-button size="small" :loading="clearing" @click="clear">清空画像</n-button>
        </div>
      </div>
    </n-card>

    <!-- 最近反馈记录 -->
    <n-card class="profile-card" title="最近反馈记录" :bordered="false" style="margin-top: 16px;">
      <n-data-table
        :columns="columns"
        :data="feedbacks"
        :pagination="pagination"
        :bordered="false"
        size="small"
        @update:page="loadFeedback"
      />
    </n-card>

    <n-modal v-model:show="showDiff" preset="card" title="最近画像变更" style="max-width: 760px;">
      <div v-if="lastDiff.length" class="diff-list">
        <div v-for="(line, idx) in lastDiff" :key="idx" :class="['diff-line', line.type]">
          <span class="diff-mark">{{ line.type === 'added' ? '+' : '-' }}</span>
          <span>{{ line.text }}</span>
        </div>
      </div>
      <n-empty v-else description="暂无变更记录" />
    </n-modal>
  </div>
</template>

<script setup>
import {ref, h, onMounted, computed} from 'vue'
import {useMessage} from 'naive-ui'
import {MdPreview, MdEditor} from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import {
  GetConfig, GetUserProfile, SaveUserProfile, RelearnUserProfile, ClearUserProfile,
  GetAgentFeedbackStats, ListAgentFeedback, GetUserProfileEnabled, SetUserProfileEnabled,
  GetUserProfileUpdatedAt,
} from "../../wailsjs/go/main/App"

const message = useMessage()

const stats = ref({total: 0, upCount: 0, downCount: 0, upRate: 0})
const profile = ref('')
const feedbacks = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const editMode = ref('preview')
const darkTheme = ref(false)
const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))
const profileEnabled = ref(true)
const togglingProfile = ref(false)
const showDiff = ref(false)
const lastDiff = ref([])
const profileUpdatedAt = ref('')

const profileFields = computed(() => {
  const labels = ['关注市场', '关注标的', '持仓与成本', '风险偏好', '常用分析维度', '偏好格式', '需规避项']
  return labels.map((label) => {
    const match = (profile.value || '').match(new RegExp(`^- ${label}：([^\\n]*)`, 'm'))
    const value = match ? match[1].trim() : ''
    return {label, value, known: !!value && value !== '未明确' && value !== '无'}
  })
})
const profileKnownFields = computed(() => profileFields.value.filter((item) => item.known).length)
const profileTotalFields = computed(() => profileFields.value.length)
const profileCompleteness = computed(() => Math.round(profileKnownFields.value / profileTotalFields.value * 100))

const learning = ref(false)
const saving = ref(false)
const clearing = ref(false)

const pagination = {
  pageSize,
  onChange: (p) => { page.value = p; loadFeedback(p) },
}

const columns = [
  {title: '时间', key: 'feedbackAtStr', width: 130},
  {title: '评价', key: 'ratingLabel', width: 70, render: (row) => h('span', row.rating === 1 ? '👍' : '👎')},
  {title: '分类', key: 'category', width: 110, ellipsis: {tooltip: true}},
  {title: '问题', key: 'question', ellipsis: {tooltip: true}},
  {title: '原因', key: 'reason', ellipsis: {tooltip: true}},
]

function classifyFeedbackReason(reason) {
  const text = (reason || '').trim()
  if (!text) return '未说明'
  if (/数据|价格|行情|财报|新闻|错误|不准|过时/.test(text)) return '数据问题'
  if (/逻辑|推理|依据|原因|矛盾/.test(text)) return '逻辑问题'
  if (/格式|表格|太长|太短|啰嗦|看不懂/.test(text)) return '表达格式'
  if (/风险|止损|仓位|激进|保守/.test(text)) return '风险偏好'
  if (/持仓|成本|自选|关注/.test(text)) return '未结合我'
  return '其他'
}

function buildProfileDiff(beforeText, afterText) {
  const before = new Set((beforeText || '').split('\n').map((line) => line.trim()).filter(Boolean))
  const after = new Set((afterText || '').split('\n').map((line) => line.trim()).filter(Boolean))
  const removed = [...before].filter((line) => !after.has(line)).map((text) => ({type: 'removed', text}))
  const added = [...after].filter((line) => !before.has(line)).map((text) => ({type: 'added', text}))
  return [...removed, ...added]
}

function loadFeedback(p) {
  const cur = p || page.value
  ListAgentFeedback(cur, pageSize).then((res) => {
    const list = res?.list || []
    feedbacks.value = list.map((it) => ({
      ...it,
      ratingLabel: it.AgentFeedback?.Rating === 1 ? '有用' : '没用',
      question: it.AgentFeedback?.Question || '',
      reason: it.AgentFeedback?.Reason || '',
      category: classifyFeedbackReason(it.AgentFeedback?.Reason || ''),
      feedbackAtStr: it.feedbackAtStr || '',
    }))
    total.value = res?.total || 0
  })
}

function loadStats() {
  GetAgentFeedbackStats().then((res) => {
    stats.value = res || stats.value
  })
}

function loadProfile() {
  GetUserProfile().then((res) => {
    profile.value = res || ''
  })
  GetUserProfileUpdatedAt().then((res) => { profileUpdatedAt.value = res || '' })
}

function loadProfileEnabled() {
  GetUserProfileEnabled().then((res) => {
    profileEnabled.value = res !== false
  })
}

function toggleProfileEnabled(value) {
  togglingProfile.value = true
  SetUserProfileEnabled(value)
    .then(() => {
      profileEnabled.value = value
      message.success(value ? '画像将注入后续对话' : '画像已暂停注入')
    })
    .catch((e) => {
      profileEnabled.value = !value
      message.error('切换失败: ' + e)
    })
    .finally(() => { togglingProfile.value = false })
}

function save() {
  saving.value = true
  SaveUserProfile(profile.value)
    .then(() => { profileUpdatedAt.value = new Date().toLocaleString('zh-CN', {hour12: false}); message.success('画像已保存') })
    .catch((e) => { message.error('保存失败: ' + e) })
    .finally(() => { saving.value = false })
}

function relearn() {
  learning.value = true
  const oldProfile = profile.value
  RelearnUserProfile()
    .then((res) => {
      profile.value = res || ''
      lastDiff.value = buildProfileDiff(oldProfile, profile.value)
      if (lastDiff.value.length) {
        showDiff.value = true
      }
      message.success('画像已重新学习')
      loadStats()
      return GetUserProfileUpdatedAt()
    })
    .then((updatedAt) => { if (updatedAt) profileUpdatedAt.value = updatedAt })
    .catch((e) => { message.error('学习失败: ' + e) })
    .finally(() => { learning.value = false })
}

function clear() {
  clearing.value = true
  ClearUserProfile()
    .then(() => {
      profile.value = ''
      message.success('画像已清空')
    })
    .catch((e) => { message.error('清空失败: ' + e) })
    .finally(() => { clearing.value = false })
}

onMounted(() => {
  GetConfig().then((res) => {
    darkTheme.value = !!res.darkTheme
  })
  loadProfile()
  loadProfileEnabled()
  loadStats()
  loadFeedback(1)
})
</script>

<style scoped>
.user-profile-page {
  padding: 16px;
  width: 100%;
  box-sizing: border-box;
}
.profile-card {
  border-radius: 8px;
}
.stats-grid {
  margin-bottom: 16px;
}
.stat-card {
  text-align: left;
}
.stat-num {
  font-size: 24px;
  font-weight: 600;
}
.stat-label {
  color: #888;
  font-size: 12px;
  margin-top: 4px;
}
.stat-up .stat-num { color: #18a058; }
.stat-down .stat-num { color: #d03050; }
.editor-tip {
  color: #888;
  font-size: 12px;
  margin-bottom: 8px;
}
.profile-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: #777;
  font-size: 12px;
  margin-bottom: 8px;
}
.profile-alert {
  margin-bottom: 8px;
}
.editor-toolbar {
  margin-bottom: 8px;
}
.profile-md-editor,
.profile-md-preview {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 6px;
  min-height: 240px;
}
.profile-md-preview {
  padding: 8px 12px;
}
.editor-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
}
.diff-list {
  max-height: 420px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 13px;
}
.diff-line {
  display: flex;
  gap: 8px;
  padding: 4px 6px;
  border-radius: 4px;
}
.diff-line.added {
  color: #18a058;
  background: rgba(24, 160, 88, 0.08);
}
.diff-line.removed {
  color: #d03050;
  background: rgba(208, 48, 80, 0.08);
}
.diff-mark {
  width: 12px;
  flex: 0 0 auto;
}
</style>
