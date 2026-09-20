<script setup>
import {computed, onBeforeMount, onMounted, ref, reactive} from 'vue'
import {GetConfig, PromptPlazaRequest} from "../../wailsjs/go/main/App";
import {useMessage, useDialog} from "naive-ui"
import {MdPreview} from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

const message = useMessage()
const dialog = useDialog()

const darkTheme = ref(false)
const editorTheme = ref('light')
const apiBase = ref('https://go-stock.sparkmemory.top/api')
const token = ref(localStorage.getItem('promptPlazaToken') || '')
const currentUser = ref(null)
const keyword = ref('')
const resolvedFilter = ref('')
const loading = ref(false)
const questions = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 15,
  itemCount: 0,
  pageCount: 1
})

const detailModal = reactive({
  show: false,
  question: null,
  answers: [],
  newAnswer: ''
})

const askModal = reactive({
  show: false,
  title: '',
  content: '',
  promptId: null,
  loading: false
})

const isLoggedIn = computed(() => !!token.value)

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      darkTheme.value = true
      editorTheme.value = 'dark'
    }
    if (result.promptPlazaApiBase) {
      apiBase.value = result.promptPlazaApiBase
    }
  })
})

onMounted(() => {
  loadQuestions()
  if (token.value) {
    fetchCurrentUser()
  }
})

// 与其他广场页面保持一致，统一走 Go 后端代理：规避 webview 跨域与 macOS WKWebView ATS 限制。
async function apiGet(path, params = {}) {
  const resp = await PromptPlazaRequest('GET', apiBase.value, path, params, '', token.value)
  if (resp.code !== 0) throw new Error(resp.message || '请求失败')
  return resp.data
}

async function apiPost(path, body = null) {
  const resp = await PromptPlazaRequest('POST', apiBase.value, path, null, body ? JSON.stringify(body) : '', token.value)
  if (resp.code !== 0) throw new Error(resp.message || '请求失败')
  return resp.data
}

async function apiDelete(path) {
  const resp = await PromptPlazaRequest('DELETE', apiBase.value, path, null, '', token.value)
  if (resp.code !== 0) throw new Error(resp.message || '请求失败')
  return resp.data
}

// 仅当服务端明确表示凭证失效时才清除登录态。网络异常、接口未部署、请求被拦截等
// 临时故障不能清 token——该 token 由「提示词广场」「技能广场」共用，
// 否则进入本页签会把用户的登录状态一并清掉。
function isAuthError(e) {
  const msg = String((e && e.message) || '')
  return /(^|\D)(401|403)(\D|$)|未登录|请先登录|登录已过期|登录状态.*(失效|无效)|(token|令牌|凭证).*(无效|失效|过期|非法)/i.test(msg)
}

async function fetchCurrentUser() {
  try {
    const data = await apiGet('/user/me')
    currentUser.value = data
  } catch (e) {
    currentUser.value = null
    if (isAuthError(e)) {
      token.value = ''
      localStorage.removeItem('promptPlazaToken')
    }
  }
}

const apiAvailable = ref(true)

async function loadQuestions() {
  loading.value = true
  try {
    const params = {page: pagination.page, pageSize: pagination.pageSize}
    if (keyword.value) params.keyword = keyword.value
    if (resolvedFilter.value) params.resolved = resolvedFilter.value
    const data = await apiGet('/questions', params)
    apiAvailable.value = true
    questions.value = data.list || []
    pagination.itemCount = data.total || 0
    pagination.pageCount = Math.ceil((data.total || 0) / pagination.pageSize) || 1
  } catch (e) {
    if (e.message.includes('响应解析失败') || e.message.includes('404') || e.message.includes('Not Found')) {
      apiAvailable.value = false
    } else {
      message.error('加载问题列表失败: ' + e.message)
    }
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  loadQuestions()
}

function handleResolvedFilter(val) {
  resolvedFilter.value = val
  pagination.page = 1
  loadQuestions()
}

function handlePageChange(page) {
  pagination.page = page
  loadQuestions()
}

async function showDetail(questionId) {
  try {
    const data = await apiGet(`/questions/${questionId}`)
    detailModal.question = data.question
    detailModal.answers = data.answers || []
    detailModal.newAnswer = ''
    detailModal.show = true
  } catch (e) {
    message.error('加载问题详情失败: ' + e.message)
  }
}

function showAskModal() {
  if (!isLoggedIn.value) {
    message.warning('请先在"提示词广场"登录后再提问')
    return
  }
  askModal.title = ''
  askModal.content = ''
  askModal.promptId = null
  askModal.show = true
}

async function handleAsk() {
  if (!askModal.title || !askModal.content) {
    message.warning('请填写标题和内容')
    return
  }
  askModal.loading = true
  try {
    const body = {title: askModal.title, content: askModal.content}
    if (askModal.promptId) body.promptId = askModal.promptId
    await apiPost('/questions', body)
    askModal.show = false
    message.success('提问成功')
    loadQuestions()
  } catch (e) {
    message.error('提问失败: ' + e.message)
  } finally {
    askModal.loading = false
  }
}

async function handleDeleteQuestion(question) {
  dialog.warning({
    title: '提示',
    content: '确定要删除这个问题吗？所有回答也会被删除。',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiDelete(`/questions/${question.id}`)
        detailModal.show = false
        message.success('删除成功')
        loadQuestions()
      } catch (e) {
        message.error('删除失败: ' + e.message)
      }
    }
  })
}

async function submitAnswer() {
  if (!isLoggedIn.value) {
    message.warning('请先在"提示词广场"登录后再回答')
    return
  }
  if (!detailModal.newAnswer.trim()) {
    message.warning('请输入回答内容')
    return
  }
  try {
    const data = await apiPost(`/questions/${detailModal.question.id}/answers`, {
      content: detailModal.newAnswer
    })
    detailModal.newAnswer = ''
    detailModal.question.answersCount = (detailModal.question.answersCount || 0) + 1
    detailModal.answers.push(data)
    message.success('回答成功')
  } catch (e) {
    message.error('回答失败: ' + e.message)
  }
}

async function handleAcceptAnswer(answer) {
  if (!isLoggedIn.value) return
  try {
    await apiPost(`/answers/${answer.id}/accept`)
    detailModal.answers.forEach(a => { a.isAccepted = false })
    answer.isAccepted = true
    detailModal.question.isResolved = true
    message.success('已采纳该回答')
    loadQuestions()
  } catch (e) {
    message.error('采纳失败: ' + e.message)
  }
}

async function handleDeleteAnswer(answer) {
  dialog.warning({
    title: '提示',
    content: '确定要删除这条回答吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiDelete(`/answers/${answer.id}`)
        detailModal.answers = detailModal.answers.filter(a => a.id !== answer.id)
        detailModal.question.answersCount = Math.max(0, (detailModal.question.answersCount || 1) - 1)
        message.success('删除成功')
      } catch (e) {
        message.error('删除失败: ' + e.message)
      }
    }
  })
}

async function handleAnswerLike(answer) {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    return
  }
  try {
    const data = await apiPost(`/answers/${answer.id}/like`)
    answer.isLiked = data.isLiked
    answer.likesCount = data.likesCount
  } catch (e) {
    message.error('操作失败: ' + e.message)
  }
}

function formatTime(timeStr) {
  if (!timeStr) return ''
  return timeStr.substring(0, 19).replace('T', ' ')
}

function timeAgo(timeStr) {
  if (!timeStr) return ''
  const now = new Date()
  const time = new Date(timeStr)
  const diff = Math.floor((now - time) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 2592000) return Math.floor(diff / 86400) + '天前'
  return formatTime(timeStr)
}
</script>

<template>
  <div style="padding: 0">
    <n-alert v-if="!apiAvailable" title="问答服务暂不可用" type="warning" style="margin-bottom: 12px">
      问答广场接口未部署或服务未启动，请联系管理员部署最新版本的服务端程序。
    </n-alert>
    <n-space v-if="apiAvailable" vertical :size="12">
      <n-space justify="space-between" align="center">
        <n-space align="center">
          <n-input
            v-model:value="keyword"
            placeholder="搜索问题..."
            clearable
            style="width: 260px"
            @keyup.enter="handleSearch"
          />
          <n-button type="primary" @click="handleSearch">搜索</n-button>
          <n-button :type="resolvedFilter === '' ? 'primary' : 'default'" size="small" @click="handleResolvedFilter('')">全部</n-button>
          <n-button :type="resolvedFilter === 'false' ? 'warning' : 'default'" size="small" @click="handleResolvedFilter('false')">待解决</n-button>
          <n-button :type="resolvedFilter === 'true' ? 'success' : 'default'" size="small" @click="handleResolvedFilter('true')">已解决</n-button>
        </n-space>
        <n-space>
          <n-button type="success" @click="showAskModal">❓ 提问</n-button>
          <template v-if="isLoggedIn">
            <n-tag type="success" size="medium" round>
              {{ currentUser?.nickname || currentUser?.username || '已登录' }}
            </n-tag>
          </template>
          <template v-else>
            <n-text depth="3" style="font-size: 12px">请在"提示词广场"中登录</n-text>
          </template>
        </n-space>
      </n-space>

      <n-spin :show="loading">
        <n-list bordered>
          <n-list-item v-for="item in questions" :key="item.id" style="cursor: pointer" @click="showDetail(item.id)">
            <n-thing>
              <template #header>
                <n-space align="center" :size="8">
                  <n-tag v-if="item.isResolved" type="success" size="small" round>已解决</n-tag>
                  <n-tag v-else type="warning" size="small" round>待解决</n-tag>
                  <n-text strong style="font-size: 15px">{{ item.title }}</n-text>
                </n-space>
              </template>
              <template #header-extra>
                <n-space :size="12" style="font-size: 12px">
                  <n-text depth="3">💬 {{ item.answersCount || 0 }} 回答</n-text>
                </n-space>
              </template>
              <template #description>
                <n-space align="center" :size="8">
                  <n-text depth="3" style="font-size: 12px">{{ item.user?.nickname || item.user?.username || '匿名' }}</n-text>
                  <n-text depth="3" style="font-size: 12px">· {{ timeAgo(item.createdAt) }}</n-text>
                </n-space>
              </template>
            </n-thing>
          </n-list-item>
        </n-list>
        <n-empty v-if="!loading && questions.length === 0" description="暂无问题，快来提问吧" style="margin-top: 40px" />
      </n-spin>

      <n-space justify="center" style="margin-top: 12px" v-if="pagination.pageCount > 1">
        <n-pagination
          v-model:page="pagination.page"
          :page-count="pagination.pageCount"
          :page-size="pagination.pageSize"
          @update:page="handlePageChange"
        />
      </n-space>
    </n-space>

    <n-modal v-model:show="detailModal.show" preset="card" style="width: 1100px; max-width: 95vw" :title="detailModal.question?.title || '问题详情'">
      <template v-if="detailModal.question">
        <n-space vertical :size="16">
          <n-space align="center" justify="space-between">
            <n-space align="center" :size="8">
              <n-tag v-if="detailModal.question.isResolved" type="success" size="small">已解决</n-tag>
              <n-tag v-else type="warning" size="small">待解决</n-tag>
              <n-text depth="3" style="font-size: 12px">
                {{ detailModal.question.user?.nickname || detailModal.question.user?.username || '匿名' }} · {{ formatTime(detailModal.question.createdAt) }}
              </n-text>
            </n-space>
            <n-space :size="8">
              <n-button
                v-if="currentUser && detailModal.question.userId === currentUser.id"
                size="small"
                type="error"
                @click="handleDeleteQuestion(detailModal.question)"
              >🗑️ 删除问题</n-button>
            </n-space>
          </n-space>

          <div style="max-height: 300px; overflow-y: auto; text-align: left">
            <MdPreview
              :model-value="detailModal.question.content"
              :theme="editorTheme"
            />
          </div>

          <n-divider style="margin: 0" />

          <n-text strong>{{ detailModal.answers.length || 0 }} 个回答</n-text>

          <n-space vertical :size="8" style="width: 100%">
            <n-input
              v-model:value="detailModal.newAnswer"
              type="textarea"
              placeholder="写下你的回答..."
              :rows="3"
            />
            <n-space justify="end">
              <n-button size="small" type="primary" @click="submitAnswer">提交回答</n-button>
            </n-space>
          </n-space>

          <n-space vertical :size="12" style="width: 100%">
            <n-card v-for="answer in detailModal.answers" :key="answer.id" size="small" :embedded="answer.isAccepted" :bordered="answer.isAccepted" :style="answer.isAccepted ? 'border-color: #63e2b7' : ''">
              <template #header>
                <n-space align="center" :size="8">
                  <n-text strong style="font-size: 13px">{{ answer.user?.nickname || answer.user?.username }}</n-text>
                  <n-text depth="3" style="font-size: 12px">{{ timeAgo(answer.createdAt) }}</n-text>
                  <n-tag v-if="answer.isAccepted" type="success" size="tiny">✅ 已采纳</n-tag>
                </n-space>
              </template>
              <div style="text-align: left; font-size: 13px">
                <MdPreview :model-value="answer.content" :theme="editorTheme" />
              </div>
              <template #action>
                <n-space :size="8">
                  <n-button text size="tiny" @click="handleAnswerLike(answer)">
                    {{ answer.isLiked ? '❤️' : '🤍' }} {{ answer.likesCount || 0 }}
                  </n-button>
                  <n-button
                    v-if="currentUser && detailModal.question.userId === currentUser.id && !detailModal.question.isResolved"
                    text
                    size="tiny"
                    type="success"
                    @click="handleAcceptAnswer(answer)"
                  >采纳</n-button>
                  <n-button
                    v-if="currentUser && answer.userId === currentUser.id"
                    text
                    size="tiny"
                    type="error"
                    @click="handleDeleteAnswer(answer)"
                  >删除</n-button>
                </n-space>
              </template>
            </n-card>
            <n-empty v-if="detailModal.answers.length === 0" description="暂无回答，来写下第一个回答吧" size="small" />
          </n-space>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="askModal.show" preset="card" style="width: 800px; max-width: 95vw" title="❓ 提问">
      <n-space vertical :size="12">
        <n-input v-model:value="askModal.title" placeholder="问题标题" />
        <n-input
          v-model:value="askModal.content"
          type="textarea"
          placeholder="详细描述你的问题..."
          :rows="6"
        />
        <n-space justify="end">
          <n-button @click="askModal.show = false">取消</n-button>
          <n-button type="primary" :loading="askModal.loading" @click="handleAsk">提交问题</n-button>
        </n-space>
      </n-space>
    </n-modal>
  </div>
</template>

<style scoped>
:deep(.md-editor-preview) {
  padding: 8px 12px;
}
:deep(.md-editor-preview-wrapper) {
  padding: 0;
}
:deep(.md-editor-preview p),
:deep(.md-editor-preview h1),
:deep(.md-editor-preview h2),
:deep(.md-editor-preview h3),
:deep(.md-editor-preview h4),
:deep(.md-editor-preview h5),
:deep(.md-editor-preview h6),
:deep(.md-editor-preview ul),
:deep(.md-editor-preview ol),
:deep(.md-editor-preview blockquote),
:deep(.md-editor-preview pre),
:deep(.md-editor-preview div) {
  text-align: left;
}
</style>
