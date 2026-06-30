<script setup>
import {computed, h, onBeforeMount, onMounted, ref, reactive} from 'vue'
import {GetConfig, GetSponsorInfo, GetMachineId, CheckDeviceBinding, QuitApp, GetEffectiveSponsorVip, AddPromptTemplate, PromptPlazaRequest} from "../../wailsjs/go/main/App";
import {useMessage, useDialog} from "naive-ui";
import {MdPreview, MdEditor} from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import {EventsEmit} from '../../wailsjs/runtime'

const message = useMessage()
const dialog = useDialog()

const darkTheme = ref(false)
const editorTheme = ref('light')
const apiBase = ref('http://go-stock.sparkmemory.top:1918/api')
const token = ref(localStorage.getItem('promptPlazaToken') || '')
const currentUser = ref(null)
const categories = ref([])
const activeCategory = ref(null)
const activeSort = ref('latest')
const vipOnlyFilter = ref(false)
const keyword = ref('')
const loading = ref(false)
const prompts = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 12,
  itemCount: 0,
  pageCount: 1
})

const detailModal = reactive({
  show: false,
  data: null,
  comments: [],
  commentPage: 1,
  commentPageSize: 10,
  commentTotal: 0,
  commentLoading: false,
  newComment: '',
  replyTo: null
})

const loginModal = reactive({
  show: false,
  tab: 'login',
  username: localStorage.getItem('promptPlazaUsername') || '',
  password: localStorage.getItem('promptPlazaPassword') || '',
  nickname: ''
})

const createModal = reactive({
  show: false,
  title: '',
  content: '',
  description: '',
  category: '',
  tags: '',
  isPublic: true,
  vipOnly: false
})

const rankingModal = reactive({
  show: false,
  type: 'hot',
  range: 'all',
  list: [],
  loading: false
})

const editModal = reactive({
  show: false,
  id: 0,
  title: '',
  content: '',
  description: '',
  category: '',
  tags: '',
  isPublic: true,
  vipOnly: false,
  loading: false
})

const isLoggedIn = computed(() => !!token.value)
const vipRequireLogin = ref(false)

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
  loadCategories()
  loadPrompts()
  if (token.value) {
    fetchCurrentUser()
  } else {
    checkVipAndPromptLogin()
  }
})

async function apiGet(path, params = {}) {
  // 通过 Go 后端代理发起请求，规避 macOS WKWebView 的 ATS 对明文 HTTP 的限制
  const resp = await PromptPlazaRequest('GET', apiBase.value, path, params, '', token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function apiPost(path, body = null) {
  // 通过 Go 后端代理发起请求，规避 macOS WKWebView 的 ATS 对明文 HTTP 的限制
  const resp = await PromptPlazaRequest('POST', apiBase.value, path, null, body ? JSON.stringify(body) : '', token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function apiPut(path, body) {
  // 通过 Go 后端代理发起请求，规避 macOS WKWebView 的 ATS 对明文 HTTP 的限制
  const resp = await PromptPlazaRequest('PUT', apiBase.value, path, null, JSON.stringify(body), token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function apiDelete(path) {
  // 通过 Go 后端代理发起请求，规避 macOS WKWebView 的 ATS 对明文 HTTP 的限制
  const resp = await PromptPlazaRequest('DELETE', apiBase.value, path, null, '', token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function loadCategories() {
  try {
    const data = await apiGet('/prompts/categories')
    categories.value = data || []
  } catch (e) {
    console.warn('加载分类失败', e)
  }
}

async function loadPrompts() {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    if (activeCategory.value) params.category = activeCategory.value
    if (keyword.value) params.keyword = keyword.value
    params.sort = activeSort.value
    if (vipOnlyFilter.value) params.vipOnly = 'true'
    const data = await apiGet('/prompts', params)
    prompts.value = data.list || []
    pagination.itemCount = data.total || 0
    pagination.pageCount = Math.ceil((data.total || 0) / (data.pageSize || pagination.pageSize)) || 1
  } catch (e) {
    message.error('加载提示词列表失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function fetchCurrentUser() {
  try {
    const data = await apiGet('/user/me')
    currentUser.value = data
    syncVipInfo()
    checkDeviceLimit()
  } catch (e) {
    token.value = ''
    localStorage.removeItem('promptPlazaToken')
    currentUser.value = null
  }
}

async function checkVipAndPromptLogin() {
  try {
    const vipInfo = await GetEffectiveSponsorVip()
    if (vipInfo && vipInfo.vipLevel > 0 && vipInfo.active) {
      vipRequireLogin.value = true
      loginModal.show = true
      loginModal.tab = 'login'
      message.info('VIP用户请登录，解锁专属提示词与更多权益')
    }
  } catch (e) {
    console.warn('检查VIP状态失败', e)
  }
}

async function checkDeviceLimit() {
  if (!token.value) return
  try {
    const result = await CheckDeviceBinding(token.value, apiBase.value)
    if (!result.bound && result.deviceCount >= result.maxDevices) {
      let countdown = 30
      const d = dialog.warning({
        title: '设备绑定超限',
        content: `您已绑定 ${result.deviceCount} 台设备，已达上限，当前设备未授权。程序将在 ${countdown} 秒后自动关闭。`,
        positiveText: '立即关闭',
        onPositiveClick: () => {
          QuitApp()
        },
        onMaskClick: () => {},
        onEsc: () => {}
      })
      const timer = setInterval(() => {
        countdown--
        if (countdown <= 0) {
          clearInterval(timer)
          d.destroy()
          QuitApp()
        } else {
          d.content = `您已绑定 ${result.deviceCount} 台设备，已达上限，当前设备未授权。程序将在 ${countdown} 秒后自动关闭。`
        }
      }, 1000)
    }
  } catch (e) {
    console.warn('设备绑定检查失败', e)
  }
}

async function syncVipInfo() {
  if (!token.value) return
  try {
    const sponsorInfo = await GetSponsorInfo()
    const vipLevel = sponsorInfo?.vipLevel ? Number(sponsorInfo.vipLevel) : 0
    const vipExpireAt = sponsorInfo?.vipEndTime || ''
    let uuid = ''
    try {
      uuid = await GetMachineId()
    } catch (e) {
      console.warn('获取机器ID失败', e)
    }
    const body = {vipLevel, uuid}
    if (vipLevel > 0 && vipExpireAt) {
      const d = new Date(vipExpireAt.replace(' ', 'T'))
      body.vipExpireAt = d.toISOString()
    } else {
      body.vipExpireAt = ''
    }
    try {
      const config = await GetConfig()
      if (config?.sponsorCode) {
        body.sponsorCode = config.sponsorCode
      }
    } catch (e) {
      console.warn('获取赞助码失败', e)
    }
    await apiPost('/user/vip', body)
    if (currentUser.value) {
      currentUser.value.vipLevel = vipLevel
      currentUser.value.vipExpireAt = vipExpireAt
    }
  } catch (e) {
    console.warn('同步VIP信息失败', e)
  }
}

async function handleLogin() {
  try {
    const data = await apiPost('/auth/login', {
      username: loginModal.username,
      password: loginModal.password
    })
    token.value = data.token
    localStorage.setItem('promptPlazaToken', data.token)
    localStorage.setItem('promptPlazaUsername', loginModal.username)
    localStorage.setItem('promptPlazaPassword', loginModal.password)
    currentUser.value = data.user
    loginModal.show = false
    vipRequireLogin.value = false
    message.success('登录成功')
    syncVipInfo()
    checkDeviceLimit()
    loadPrompts()
  } catch (e) {
    message.error('登录失败: ' + e.message)
  }
}

async function handleRegister() {
  try {
    const data = await apiPost('/auth/register', {
      username: loginModal.username,
      password: loginModal.password,
      nickname: loginModal.nickname
    })
    token.value = data.token
    localStorage.setItem('promptPlazaToken', data.token)
    localStorage.setItem('promptPlazaUsername', loginModal.username)
    localStorage.setItem('promptPlazaPassword', loginModal.password)
    currentUser.value = data.user
    loginModal.show = false
    vipRequireLogin.value = false
    loginModal.username = ''
    loginModal.password = ''
    loginModal.nickname = ''
    message.success('注册成功')
    syncVipInfo()
    checkDeviceLimit()
    loadPrompts()
  } catch (e) {
    message.error('注册失败: ' + e.message)
  }
}

function handleLogout() {
  dialog.warning({
    title: '提示',
    content: '确定要退出登录吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      token.value = ''
      localStorage.removeItem('promptPlazaToken')
      currentUser.value = null
      message.success('已退出登录')
      loadPrompts()
    }
  })
}

function handlePageChange(page) {
  pagination.page = page
  loadPrompts()
}

function handleSearch() {
  pagination.page = 1
  loadPrompts()
}

function handleCategoryFilter() {
  pagination.page = 1
  loadPrompts()
}

function onSortChange() {
  pagination.page = 1
  loadPrompts()
}

async function showDetail(id) {
  try {
    const data = await apiGet(`/prompts/${id}`)
    detailModal.data = data
    detailModal.show = true
    detailModal.newComment = ''
    detailModal.replyTo = null
    loadComments(id)
  } catch (e) {
    message.error('加载详情失败: ' + e.message)
  }
}

async function loadComments(promptId) {
  detailModal.commentLoading = true
  try {
    const data = await apiGet(`/prompts/${promptId}/comments`, {
      page: detailModal.commentPage,
      pageSize: detailModal.commentPageSize
    })
    detailModal.comments = data.list || []
    detailModal.commentTotal = data.total || 0
  } catch (e) {
    console.warn('加载评论失败', e)
  } finally {
    detailModal.commentLoading = false
  }
}

async function handleLike(prompt) {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  try {
    const data = await apiPost(`/prompts/${prompt.id}/like`)
    prompt.isLiked = data.isLiked
    prompt.likesCount = data.likesCount
    if (detailModal.data && detailModal.data.id === prompt.id) {
      detailModal.data.isLiked = data.isLiked
      detailModal.data.likesCount = data.likesCount
    }
  } catch (e) {
    message.error('操作失败: ' + e.message)
  }
}

async function handleFavorite(prompt) {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  try {
    const data = await apiPost(`/prompts/${prompt.id}/favorite`)
    prompt.isFavorited = data.isFavorited
    prompt.favoritesCount = data.favoritesCount
    if (detailModal.data && detailModal.data.id === prompt.id) {
      detailModal.data.isFavorited = data.isFavorited
      detailModal.data.favoritesCount = data.favoritesCount
    }
  } catch (e) {
    message.error('操作失败: ' + e.message)
  }
}

async function handleDownload(prompt) {
  try {
    const data = await apiGet(`/prompts/${prompt.id}/download`)
    const text = `${data.title}\n\n${data.content}\n\n分类: ${data.category || '无'}\n标签: ${data.tags || '无'}\n作者: ${data.author?.nickname || data.author?.username || '匿名'}\n创建时间: ${data.createdAt}`
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(data.content)
      message.success('提示词内容已复制到剪贴板')
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = data.content
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      message.success('提示词内容已复制到剪贴板')
    }
    prompt.downloadsCount = (prompt.downloadsCount || 0) + 1
  } catch (e) {
    message.error('下载失败: ' + e.message)
  }
}

async function handleCopyContent(content) {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(content)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = content
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    message.success('已复制到剪贴板')
  } catch (e) {
    message.error('复制失败')
  }
}

async function addPromptToTemplate(prompt) {
  if (prompt.needVip) {
    const vipInfo = await GetEffectiveSponsorVip()
    if (!vipInfo || vipInfo.vipLevel <= 0 || !vipInfo.active) {
      message.warning('该提示词为VIP专属，请先开通VIP')
      return
    }
  }
  try {
    const res = await AddPromptTemplate({
      name: prompt.title,
      content: prompt.content,
      type: '模型系统Prompt'
    })
    if (res === '添加成功') {
      message.success('已添加到我的提示词模板')
      EventsEmit('promptTemplatesChanged')
    } else {
      message.warning(res)
    }
  } catch (e) {
    message.error('添加失败: ' + e.message)
  }
}

async function submitComment() {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  if (!detailModal.newComment.trim()) {
    message.warning('请输入评论内容')
    return
  }
  try {
    const body = {content: detailModal.newComment}
    if (detailModal.replyTo) {
      body.parentId = detailModal.replyTo.id
    }
    await apiPost(`/prompts/${detailModal.data.id}/comments`, body)
    detailModal.newComment = ''
    detailModal.replyTo = null
    detailModal.data.commentsCount = (detailModal.data.commentsCount || 0) + 1
    loadComments(detailModal.data.id)
    message.success('评论成功')
  } catch (e) {
    message.error('评论失败: ' + e.message)
  }
}

async function deleteComment(commentId) {
  dialog.warning({
    title: '提示',
    content: '确定要删除这条评论吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiDelete(`/comments/${commentId}`)
        detailModal.data.commentsCount = Math.max(0, (detailModal.data.commentsCount || 1) - 1)
        loadComments(detailModal.data.id)
        message.success('删除成功')
      } catch (e) {
        message.error('删除失败: ' + e.message)
      }
    }
  })
}

function showEditModal(prompt) {
  editModal.id = prompt.id
  editModal.title = prompt.title || ''
  editModal.content = prompt.content || ''
  editModal.description = prompt.description || ''
  editModal.category = prompt.category || ''
  editModal.tags = prompt.tags || ''
  editModal.isPublic = prompt.isPublic !== false
  editModal.vipOnly = prompt.vipOnly === true
  editModal.show = true
}

async function handleEdit() {
  if (!editModal.title || !editModal.content) {
    message.warning('请填写标题和内容')
    return
  }
  editModal.loading = true
  try {
    await apiPut(`/prompts/${editModal.id}`, {
      title: editModal.title,
      content: editModal.content,
      description: editModal.description,
      category: editModal.category,
      tags: editModal.tags,
      isPublic: editModal.isPublic,
      vipOnly: editModal.vipOnly
    })
    editModal.show = false
    detailModal.show = false
    message.success('修改成功')
    loadPrompts()
    loadCategories()
  } catch (e) {
    message.error('修改失败: ' + e.message)
  } finally {
    editModal.loading = false
  }
}

function handleDeletePrompt(prompt) {
  dialog.warning({
    title: '提示',
    content: '确定要删除这个提示词吗？删除后不可恢复。',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiDelete(`/prompts/${prompt.id}`)
        detailModal.show = false
        message.success('删除成功')
        loadPrompts()
        loadCategories()
      } catch (e) {
        message.error('删除失败: ' + e.message)
      }
    }
  })
}

async function showCreateModal() {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  createModal.title = ''
  createModal.content = ''
  createModal.description = ''
  createModal.category = ''
  createModal.tags = ''
  createModal.isPublic = true
  createModal.vipOnly = !!(currentUser.value && currentUser.value.vipLevel > 0 && currentUser.value.vipExpireAt && new Date(currentUser.value.vipExpireAt) > new Date())
  createModal.show = true
}

async function handleCreate() {
  if (!createModal.title || !createModal.content) {
    message.warning('请填写标题和内容')
    return
  }
  try {
    await apiPost('/prompts', {
      title: createModal.title,
      content: createModal.content,
      description: createModal.description,
      category: createModal.category,
      tags: createModal.tags,
      isPublic: createModal.isPublic,
      vipOnly: createModal.vipOnly
    })
    createModal.show = false
    message.success('发布成功')
    loadPrompts()
    loadCategories()
  } catch (e) {
    message.error('发布失败: ' + e.message)
  }
}

async function showRanking(type = 'hot', range = 'all') {
  rankingModal.type = type
  rankingModal.range = range
  rankingModal.show = true
  rankingModal.loading = true
  try {
    const data = await apiGet('/prompts/ranking', {type, range, limit: 50})
    rankingModal.list = data.list || []
  } catch (e) {
    message.error('加载排行榜失败: ' + e.message)
  } finally {
    rankingModal.loading = false
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
    <n-space vertical :size="12">
      <n-space justify="space-between" align="center">
        <n-space align="center">
          <n-input
            v-model:value="keyword"
            placeholder="搜索提示词..."
            clearable
            style="width: 260px"
            @keyup.enter="handleSearch"
          />
          <n-button type="primary" @click="handleSearch">搜索</n-button>
          <n-button quaternary @click="showRanking('hot')">🏆 排行榜</n-button>
        </n-space>
        <n-space>
          <n-button type="success" @click="showCreateModal">✏️ 发布提示词</n-button>
          <template v-if="isLoggedIn">
            <n-tag :type="currentUser?.vipLevel >= 1 ? 'warning' : 'success'" size="medium" round>
              {{ currentUser?.nickname || currentUser?.username || '已登录' }}
              <template v-if="currentUser?.vipLevel >= 1"> · VIP{{ currentUser.vipLevel }}</template>
            </n-tag>
            <n-button size="small" quaternary @click="handleLogout">退出</n-button>
          </template>
          <template v-else>
            <n-button type="info" size="small" @click="loginModal.show = true; loginModal.tab = 'login'">登录 / 注册</n-button>
          </template>
        </n-space>
      </n-space>

      <n-space align="center" :size="8">
        <n-text depth="3" style="font-size: 13px">分类:</n-text>
        <n-radio-group v-model:value="activeCategory" size="small" @update:value="handleCategoryFilter">
          <n-radio-button :value="null">全部</n-radio-button>
          <n-radio-button v-for="cat in categories" :key="cat" :value="cat">{{ cat }}</n-radio-button>
        </n-radio-group>
        <n-divider vertical />
        <n-text depth="3" style="font-size: 13px">排序:</n-text>
        <n-radio-group v-model:value="activeSort" size="small" @update:value="onSortChange">
          <n-radio-button value="latest">🕐 最新</n-radio-button>
          <n-radio-button value="hot">🔥 热度</n-radio-button>
          <n-radio-button value="likes">❤️ 点赞</n-radio-button>
          <n-radio-button value="favorites">⭐ 收藏</n-radio-button>
          <n-radio-button value="downloads">⬇️ 下载</n-radio-button>
          <n-radio-button value="comments">💬 评论</n-radio-button>
        </n-radio-group>
        <n-divider vertical />
        <n-button
          :type="vipOnlyFilter ? 'warning' : 'default'"
          size="small"
          @click="vipOnlyFilter = !vipOnlyFilter; pagination.page = 1; loadPrompts()"
        >
          👑 VIP专属
        </n-button>
      </n-space>

      <n-spin :show="loading">
        <n-grid :cols="3" :x-gap="12" :y-gap="12" responsive="screen">
          <n-gi v-for="item in prompts" :key="item.id">
            <n-card
              hoverable
              size="small"
              style="cursor: pointer; height: 100%"
              @click="showDetail(item.id)"
            >
              <template #header>
                <n-space align="center" :size="6">
                  <n-text strong style="font-size: 15px">{{ item.title }}</n-text>
                  <n-tag v-if="item.vipOnly" type="warning" size="tiny" round>👑 VIP</n-tag>
                </n-space>
              </template>
              <template #header-extra>
                <n-tag v-if="item.category" size="small" type="info">{{ item.category }}</n-tag>
              </template>
              <n-ellipsis :line-clamp="2" :tooltip="false" style="color: var(--n-text-color-3); font-size: 13px; margin-bottom: 8px">
                {{item.summary|| item.description || item.content }}
              </n-ellipsis>
              <template #footer>
                <n-space justify="space-between" align="center">
                  <n-text depth="3" style="font-size: 12px">
                    {{ item.user?.nickname || item.user?.username || '匿名' }}
                    <n-tag v-if="item.user?.vipLevel >= 1" type="warning" size="tiny" round style="margin-left: 2px">VIP{{ item.user.vipLevel }}</n-tag>
                    · {{ timeAgo(item.createdAt) }}
                  </n-text>
                  <n-space :size="12" style="font-size: 12px">
                    <n-text depth="3">
                      👁️ {{ item.viewsCount || 0 }}
                    </n-text>
                    <n-text :type="item.isLiked ? 'error' : 'default'" style="cursor: pointer" @click.stop="handleLike(item)">
                      {{ item.isLiked ? '❤️' : '🤍' }} {{ item.likesCount || 0 }}
                    </n-text>
                    <n-text :type="item.isFavorited ? 'warning' : 'default'" style="cursor: pointer" @click.stop="handleFavorite(item)">
                      {{ item.isFavorited ? '⭐' : '☆' }} {{ item.favoritesCount || 0 }}
                    </n-text>
                    <n-text depth="3">
                      💬 {{ item.commentsCount || 0 }}
                    </n-text>
                    <n-text depth="3">
                      ⬇️ {{ item.downloadsCount || 0 }}
                    </n-text>
                  </n-space>
                </n-space>
              </template>
              <template #action v-if="item.tags">
                <n-space :size="4">
                  <n-tag v-for="tag in item.tags.split(',').filter(t=>t).slice(0, 3)" :key="tag" size="tiny" round>{{ tag.trim() }}</n-tag>
                </n-space>
              </template>
            </n-card>
          </n-gi>
        </n-grid>
        <n-empty v-if="!loading && prompts.length === 0" description="暂无提示词" style="margin-top: 40px" />
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

    <n-modal v-model:show="detailModal.show" preset="card" style="width: 1100px; max-width: 95vw" :title="detailModal.data?.title || '提示词详情'">
      <template v-if="detailModal.data">
        <n-space align="left" justify="space-between" style="margin-bottom: 12px">
          <n-space align="left" :size="8">
            <n-tag v-if="detailModal.data.vipOnly" type="warning" size="small" round>👑 VIP专属</n-tag>
            <n-tag v-if="detailModal.data.category" type="info" size="small">{{ detailModal.data.category }}</n-tag>
            <n-text depth="3" style="font-size: 12px">
              {{ detailModal.data.user?.nickname || detailModal.data.user?.username || '匿名' }} · {{ formatTime(detailModal.data.createdAt) }}
            </n-text>
            <n-text depth="3" style="font-size: 12px" v-if="detailModal.data.updatedAt && detailModal.data.updatedAt !== detailModal.data.createdAt">
              · 更新于 {{ formatTime(detailModal.data.updatedAt) }}
            </n-text>
          </n-space>
          <n-space :size="8">
            <n-button
              v-if="currentUser && detailModal.data.userId === currentUser.id"
              size="tiny"
              type="warning"
              @click="showEditModal(detailModal.data)"
            >
              ✏️ 编辑
            </n-button>
            <n-button
              v-if="currentUser && detailModal.data.userId === currentUser.id"
              size="tiny"
              type="error"
              @click="handleDeletePrompt(detailModal.data)"
            >
              🗑️ 删除
            </n-button>
            <n-button size="tiny" quaternary disabled>
              👁️ {{ detailModal.data.viewsCount || 0 }}
            </n-button>
            <n-button
              :type="detailModal.data.isLiked ? 'error' : 'default'"
              size="tiny"
              @click="handleLike(detailModal.data)"
            >
              {{ detailModal.data.isLiked ? '❤️ 已赞' : '🤍 点赞' }} {{ detailModal.data.likesCount || 0 }}
            </n-button>
            <n-button
              :type="detailModal.data.isFavorited ? 'warning' : 'default'"
              size="tiny"
              @click="handleFavorite(detailModal.data)"
            >
              {{ detailModal.data.isFavorited ? '⭐ 已收藏' : '☆ 收藏' }} {{ detailModal.data.favoritesCount || 0 }}
            </n-button>
            <n-button size="tiny" type="success" @click="handleDownload(detailModal.data)">
              ⬇️ 下载 {{ detailModal.data.downloadsCount || 0 }}
            </n-button>
            <n-button size="tiny" quaternary @click="handleCopyContent(detailModal.data.content)">
              📋 复制
            </n-button>
            <n-button size="tiny" type="info" @click="addPromptToTemplate(detailModal.data)">
              ➕ 添加到我的模板
            </n-button>
          </n-space>
        </n-space>

        <div style="display: flex; gap: 16px">
          <div style="flex: 4; min-width: 0">
            <n-space vertical :size="8">
              <n-space :size="4" v-if="detailModal.data.tags">
                <n-tag v-for="tag in detailModal.data.tags.split(',').filter(t=>t)" :key="tag" size="small" round>{{ tag.trim() }}</n-tag>
              </n-space>
              <div style="max-height: 500px; overflow-y: auto; position: relative">
                <MdPreview
                  :model-value="detailModal.data.content"
                  :theme="editorTheme"
                  style="text-align: left"
                />
                <div
                  v-if="detailModal.data.needVip"
                  style="position: absolute; bottom: 0; left: 0; right: 0; height: 120px; background: linear-gradient(to bottom, transparent, var(--n-color)); display: flex; align-items: flex-end; justify-content: center; padding-bottom: 16px"
                >
                  <n-space vertical align="center" :size="4">
                    <n-tag type="warning" size="medium" round>👑 VIP专属提示词</n-tag>
                    <n-text depth="3" style="font-size: 12px">开通VIP查看完整内容</n-text>
                  </n-space>
                </div>
              </div>
            </n-space>
          </div>
          <div style="flex: 1; min-width: 0">
            <n-space vertical :size="8" style="width: 100%">
              <n-text strong>评论 ({{ detailModal.data.commentsCount || 0 }})</n-text>
              <n-input
                v-model:value="detailModal.newComment"
                type="textarea"
                :placeholder="detailModal.replyTo ? `回复 @${detailModal.replyTo.user?.nickname || detailModal.replyTo.user?.username}...` : '发表评论...'"
                :rows="2"
              />
              <n-space justify="space-between" style="width: 100%">
                <n-text v-if="detailModal.replyTo" depth="3" style="font-size: 12px">
                  回复 @{{ detailModal.replyTo.user?.nickname || detailModal.replyTo.user?.username }}
                  <n-button text size="tiny" type="error" @click="detailModal.replyTo = null">取消</n-button>
                </n-text>
                <span v-else />
                <n-button size="small" type="primary" @click="submitComment">发表评论</n-button>
              </n-space>
              <n-spin :show="detailModal.commentLoading">
                <div style="max-height: 380px; overflow-y: auto; width: 100%">
                  <n-space vertical :size="12" style="width: 100%">
                    <n-card v-for="comment in detailModal.comments" :key="comment.id" size="small" embedded>
                      <template #header>
                        <n-space align="center" :size="8">
                          <n-text strong style="font-size: 13px">{{ comment.user?.nickname || comment.user?.username }}</n-text>
                          <n-text depth="3" style="font-size: 12px">{{ timeAgo(comment.createdAt) }}</n-text>
                        </n-space>
                      </template>
                      <n-text style="font-size: 13px; text-align: left; display: block">{{ comment.content }}</n-text>
                      <template #action>
                        <n-space :size="8">
                          <n-button text size="tiny" @click="detailModal.replyTo = comment">回复</n-button>
                          <n-button
                            v-if="currentUser && comment.userId === currentUser.id"
                            text
                            size="tiny"
                            type="error"
                            @click="deleteComment(comment.id)"
                          >删除</n-button>
                        </n-space>
                      </template>
                    </n-card>
                    <n-empty v-if="!detailModal.commentLoading && detailModal.comments.length === 0" description="暂无评论" size="small" />
                  </n-space>
                </div>
              </n-spin>
            </n-space>
          </div>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="loginModal.show" preset="card" style="width: 400px" :title="vipRequireLogin ? '🎉 VIP专属福利' : '账号'" :closable="!vipRequireLogin" :maskClosable="!vipRequireLogin" :closeOnEsc="!vipRequireLogin">
      <div v-if="vipRequireLogin" style="margin-bottom: 16px; padding: 12px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 8px; color: #fff">
        <div style="font-size: 15px; font-weight: 600; margin-bottom: 8px">✨ 欢迎回来，VIP用户！</div>
        <div style="font-size: 13px; line-height: 1.6; opacity: 0.95">
          登录后即可解锁专属权益：
          <div style="margin-top: 6px; padding-left: 8px">
            📖 查看 <b>VIP专属提示词</b>，获取更精准的分析策略<br/>
            🔒 自动绑定当前设备，保障账号安全<br/>
            💡 与社区用户共享投资灵感
          </div>
        </div>
      </div>
      <n-tabs v-model:value="loginModal.tab" type="line">
        <n-tab-pane name="login" tab="登录">
          <n-space vertical :size="12">
            <n-input v-model:value="loginModal.username" placeholder="用户名" />
            <n-input v-model:value="loginModal.password" type="password" placeholder="密码" show-password-on="click" />
            <n-button type="primary" block @click="handleLogin">登录</n-button>
          </n-space>
        </n-tab-pane>
        <n-tab-pane name="register" tab="注册">
          <n-space vertical :size="12">
            <n-input v-model:value="loginModal.username" placeholder="用户名 (3-50字)" />
            <n-input v-model:value="loginModal.password" type="password" placeholder="密码 (6字以上)" show-password-on="click" />
            <n-input v-model:value="loginModal.nickname" placeholder="昵称 (可选)" />
            <n-button type="primary" block @click="handleRegister">注册</n-button>
          </n-space>
        </n-tab-pane>
      </n-tabs>
    </n-modal>

    <n-modal v-model:show="createModal.show" preset="card" style="width: 1100px; max-width: 95vw" title="发布提示词">
      <n-space vertical :size="12">
        <n-input v-model:value="createModal.title" placeholder="标题" />
        <n-space :size="8">
          <n-input v-model:value="createModal.category" placeholder="分类 (如: AI编程, 数据分析)" style="width: 240px" />
          <n-input v-model:value="createModal.tags" placeholder="标签 (逗号分隔)" style="width: 240px" />
        </n-space>
        <n-input v-model:value="createModal.description" placeholder="简短描述" type="textarea" :rows="2" />
        <MdEditor
          v-model="createModal.content"
          :theme="editorTheme"
          placeholder="提示词内容"
          style="height: 400px"
        />
        <n-space align="center">
          <n-text>公开</n-text>
          <n-switch v-model:value="createModal.isPublic" />
          <n-divider vertical />
          <n-text>VIP专属</n-text>
          <n-switch v-model:value="createModal.vipOnly" />
          <n-text depth="3" style="font-size: 12px">仅VIP用户可查看完整内容</n-text>
        </n-space>
        <n-space justify="end">
          <n-button @click="createModal.show = false">取消</n-button>
          <n-button type="primary" @click="handleCreate">发布</n-button>
        </n-space>
      </n-space>
    </n-modal>

    <n-modal v-model:show="editModal.show" preset="card" style="width: 1100px; max-width: 95vw" title="编辑提示词">
      <n-space vertical :size="12">
        <n-input v-model:value="editModal.title" placeholder="标题" />
        <n-space :size="8">
          <n-input v-model:value="editModal.category" placeholder="分类" style="width: 240px" />
          <n-input v-model:value="editModal.tags" placeholder="标签 (逗号分隔)" style="width: 240px" />
        </n-space>
        <n-input v-model:value="editModal.description" placeholder="简短描述" type="textarea" :rows="2" />
        <MdEditor
          v-model="editModal.content"
          :theme="editorTheme"
          placeholder="提示词内容"
          style="height: 400px"
        />
        <n-space align="center">
          <n-text>公开</n-text>
          <n-switch v-model:value="editModal.isPublic" />
          <n-divider vertical />
          <n-text>VIP专属</n-text>
          <n-switch v-model:value="editModal.vipOnly" />
          <n-text depth="3" style="font-size: 12px">仅VIP用户可查看完整内容</n-text>
        </n-space>
        <n-space justify="end">
          <n-button @click="editModal.show = false">取消</n-button>
          <n-button type="primary" :loading="editModal.loading" @click="handleEdit">保存</n-button>
        </n-space>
      </n-space>
    </n-modal>

    <n-modal v-model:show="rankingModal.show" preset="card" style="width: 1100px; max-width: 95vw" title="🏆 排行榜">
      <n-space vertical :size="12">
        <n-space :size="8">
          <n-text depth="3" style="font-size: 13px">类型:</n-text>
          <n-button :type="rankingModal.type === 'hot' ? 'primary' : 'default'" size="small" @click="showRanking('hot', rankingModal.range)">🔥 综合热度</n-button>
          <n-button :type="rankingModal.type === 'likes' ? 'primary' : 'default'" size="small" @click="showRanking('likes', rankingModal.range)">❤️ 点赞</n-button>
          <n-button :type="rankingModal.type === 'downloads' ? 'primary' : 'default'" size="small" @click="showRanking('downloads', rankingModal.range)">⬇️ 下载</n-button>
          <n-button :type="rankingModal.type === 'favorites' ? 'primary' : 'default'" size="small" @click="showRanking('favorites', rankingModal.range)">⭐ 收藏</n-button>
          <n-divider vertical />
          <n-text depth="3" style="font-size: 13px">时间:</n-text>
          <n-button :type="rankingModal.range === 'all' ? 'primary' : 'default'" size="small" @click="showRanking(rankingModal.type, 'all')">全部</n-button>
          <n-button :type="rankingModal.range === 'daily' ? 'primary' : 'default'" size="small" @click="showRanking(rankingModal.type, 'daily')">今日</n-button>
          <n-button :type="rankingModal.range === 'weekly' ? 'primary' : 'default'" size="small" @click="showRanking(rankingModal.type, 'weekly')">本周</n-button>
          <n-button :type="rankingModal.range === 'monthly' ? 'primary' : 'default'" size="small" @click="showRanking(rankingModal.type, 'monthly')">本月</n-button>
        </n-space>

        <n-spin :show="rankingModal.loading">
          <n-list bordered>
            <n-list-item v-for="item in rankingModal.list" :key="item.id" style="cursor: pointer" @click="rankingModal.show = false; showDetail(item.id)">
              <n-space align="center" :size="12">
                <n-tag
                  :type="item.rank <= 3 ? 'error' : 'default'"
                  round
                  size="small"
                  style="min-width: 28px; text-align: center"
                >{{ item.rank }}</n-tag>
                <n-text strong>{{ item.title }}</n-text>
                <n-tag v-if="item.vipOnly" type="warning" size="tiny" round>👑 VIP</n-tag>
                <n-text depth="3" style="font-size: 12px">
                  {{ item.user?.nickname || item.user?.username || '匿名' }}
                </n-text>
                <n-space :size="8" style="font-size: 12px">
                  <n-text depth="3">❤️ {{ item.likesCount || 0 }}</n-text>
                  <n-text depth="3">⬇️ {{ item.downloadsCount || 0 }}</n-text>
                  <n-text depth="3">⭐ {{ item.favoritesCount || 0 }}</n-text>
                  <n-text depth="3">💬 {{ item.commentsCount || 0 }}</n-text>
                  <n-text v-if="item.hotScore" type="warning" style="font-size: 12px">🔥 {{ item.hotScore }}</n-text>
                </n-space>
              </n-space>
            </n-list-item>
          </n-list>
          <n-empty v-if="!rankingModal.loading && rankingModal.list.length === 0" description="暂无排行数据" />
        </n-spin>
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
:deep(.md-editor-preview div),
:deep(.md-editor-content p),
:deep(.md-editor-content h1),
:deep(.md-editor-content h2),
:deep(.md-editor-content h3),
:deep(.md-editor-content h4),
:deep(.md-editor-content h5),
:deep(.md-editor-content h6),
:deep(.md-editor-content ul),
:deep(.md-editor-content ol),
:deep(.md-editor-content blockquote),
:deep(.md-editor-content pre),
:deep(.md-editor-content div) {
  text-align: left;
}
</style>
