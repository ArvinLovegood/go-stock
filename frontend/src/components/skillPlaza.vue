<script setup>
import {computed, onBeforeMount, onMounted, ref, reactive} from 'vue'
import {
  GetConfig, GetSponsorInfo, GetMachineId, CheckDeviceBinding, QuitApp,
  GetEffectiveSponsorVip, PromptPlazaRequest, ListFilesystemSkills,
  PackSkillToBase64, ImportSkillFromBase64
} from "../../wailsjs/go/main/App";
import {useMessage, useDialog} from "naive-ui";
import {MdPreview} from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

// 导入成功后通知父组件刷新本地技能列表
const emit = defineEmits(['imported'])

const message = useMessage()
const dialog = useDialog()

const darkTheme = ref(false)
const editorTheme = ref('light')
const apiBase = ref('http://go-stock.sparkmemory.top:1918/api')
// 与提示词广场共用同一账号体系（同一 token）
const token = ref(localStorage.getItem('promptPlazaToken') || '')
const currentUser = ref(null)
const categories = ref([])
const activeCategory = ref(null)
const activeSort = ref('latest')
const vipOnlyFilter = ref(false)
const keyword = ref('')
const loading = ref(false)
const skills = ref([])
const pagination = reactive({
  page: 1,
  pageSize: 12,
  itemCount: 0,
  pageCount: 1
})

const detailModal = reactive({
  show: false,
  data: null,
  importing: false
})

const loginModal = reactive({
  show: false,
  tab: 'login',
  username: localStorage.getItem('promptPlazaUsername') || '',
  password: localStorage.getItem('promptPlazaPassword') || '',
  nickname: ''
})

const shareModal = reactive({
  show: false,
  localSkills: [],
  dirName: null,
  name: '',
  description: '',
  category: '',
  tags: '',
  vipOnly: false,
  content: '',
  fileCount: 0,
  packageSize: 0,
  packing: false,
  submitting: false
})

const rankingModal = reactive({
  show: false,
  type: 'hot',
  range: 'all',
  list: [],
  loading: false
})

const mySharesModal = reactive({
  show: false,
  list: [],
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
  loadCategories()
  loadSkills()
  if (token.value) {
    fetchCurrentUser()
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
  const resp = await PromptPlazaRequest('POST', apiBase.value, path, null, body ? JSON.stringify(body) : '', token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function apiDelete(path) {
  const resp = await PromptPlazaRequest('DELETE', apiBase.value, path, null, '', token.value)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp.data
}

async function loadCategories() {
  try {
    const data = await apiGet('/skills/categories')
    categories.value = data || []
  } catch (e) {
    console.warn('加载分类失败', e)
  }
}

async function loadSkills() {
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
    const data = await apiGet('/skills', params)
    skills.value = data.list || []
    pagination.itemCount = data.total || 0
    pagination.pageCount = Math.ceil((data.total || 0) / (data.pageSize || pagination.pageSize)) || 1
  } catch (e) {
    message.error('加载技能列表失败: ' + e.message)
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
    // 服务端权威校验赞助码后返回实际 VIP 状态（客户端提交的 vipLevel 仅作参考，服务端不信任）
    const data = await apiPost('/user/vip', body)
    if (currentUser.value) {
      currentUser.value.vipLevel = data.vipLevel
      currentUser.value.vipExpireAt = data.vipExpireAt
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
    message.success('登录成功')
    syncVipInfo()
    checkDeviceLimit()
    loadSkills()
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
    loginModal.username = ''
    loginModal.password = ''
    loginModal.nickname = ''
    message.success('注册成功')
    syncVipInfo()
    checkDeviceLimit()
    loadSkills()
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
      loadSkills()
    }
  })
}

function handlePageChange(page) {
  pagination.page = page
  loadSkills()
}

function handleSearch() {
  pagination.page = 1
  loadSkills()
}

function handleCategoryFilter() {
  pagination.page = 1
  loadSkills()
}

function onSortChange() {
  pagination.page = 1
  loadSkills()
}

async function showDetail(id) {
  try {
    const data = await apiGet(`/skills/${id}`)
    detailModal.data = data
    detailModal.show = true
  } catch (e) {
    message.error('加载详情失败: ' + e.message)
  }
}

async function handleLike(skill) {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  try {
    const data = await apiPost(`/skills/${skill.id}/like`)
    skill.isLiked = data.isLiked
    skill.likesCount = data.likesCount
    if (detailModal.data && detailModal.data.id === skill.id) {
      detailModal.data.isLiked = data.isLiked
      detailModal.data.likesCount = data.likesCount
    }
  } catch (e) {
    message.error('操作失败: ' + e.message)
  }
}

async function handleFavorite(skill) {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  try {
    const data = await apiPost(`/skills/${skill.id}/favorite`)
    skill.isFavorited = data.isFavorited
    if (detailModal.data && detailModal.data.id === skill.id) {
      detailModal.data.isFavorited = data.isFavorited
    }
    loadSkills()
  } catch (e) {
    message.error('操作失败: ' + e.message)
  }
}

// 从广场下载技能包并导入本地 skills 目录
async function handleImport(skill) {
  if (skill.needVip) {
    message.warning('该技能为VIP专属，请先开通VIP')
    return
  }
  detailModal.importing = true
  try {
    const data = await apiGet(`/skills/${skill.id}/download`)
    const result = await ImportSkillFromBase64(data.content)
    if (result && result.includes('成功')) {
      message.success(result)
      skill.downloadsCount = (skill.downloadsCount || 0) + 1
      if (detailModal.data && detailModal.data.id === skill.id) {
        detailModal.data.downloadsCount = (detailModal.data.downloadsCount || 0) + 1
      }
      emit('imported')
    } else {
      message.error(result || '导入失败')
    }
  } catch (e) {
    message.error('导入失败: ' + e.message)
  } finally {
    detailModal.importing = false
  }
}

function handleDeleteSkill(skill) {
  dialog.warning({
    title: '提示',
    content: '确定要从广场删除这个技能分享吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiDelete(`/skills/${skill.id}`)
        detailModal.show = false
        message.success('删除成功')
        loadSkills()
        loadCategories()
      } catch (e) {
        message.error('删除失败: ' + e.message)
      }
    }
  })
}

// ==================== 分享我的技能 ====================
async function showShareModal() {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  shareModal.dirName = null
  shareModal.name = ''
  shareModal.description = ''
  shareModal.category = ''
  shareModal.tags = ''
  shareModal.content = ''
  shareModal.fileCount = 0
  shareModal.packageSize = 0
  try {
    const result = await ListFilesystemSkills()
    shareModal.localSkills = result || []
  } catch (e) {
    shareModal.localSkills = []
    message.error('加载本地技能列表失败: ' + e)
  }
  shareModal.vipOnly = !!(currentUser.value && currentUser.value.vipLevel > 0 && currentUser.value.vipExpireAt && new Date(currentUser.value.vipExpireAt) > new Date())
  shareModal.show = true
}

// 选中本地技能后自动打包并预填表单
async function onShareSkillChange(dirName) {
  if (!dirName) {
    shareModal.content = ''
    return
  }
  shareModal.packing = true
  try {
    const result = await PackSkillToBase64(dirName)
    if (result.code !== 0) {
      message.error(result.msg || '打包失败')
      shareModal.dirName = null
      shareModal.content = ''
      return
    }
    const data = result.data
    shareModal.content = data.content
    shareModal.fileCount = data.fileCount
    shareModal.packageSize = data.packageSize
    shareModal.name = data.name || data.dirName
    shareModal.description = data.description || ''
  } catch (e) {
    message.error('打包失败: ' + e)
    shareModal.dirName = null
    shareModal.content = ''
  } finally {
    shareModal.packing = false
  }
}

async function handleShare() {
  if (!shareModal.dirName || !shareModal.content) {
    message.warning('请先选择要分享的本地技能')
    return
  }
  shareModal.submitting = true
  try {
    const data = await apiPost('/skills', {
      dirName: shareModal.dirName,
      name: shareModal.name,
      description: shareModal.description,
      category: shareModal.category,
      tags: shareModal.tags,
      content: shareModal.content,
      fileCount: shareModal.fileCount,
      packageSize: shareModal.packageSize,
      vipOnly: shareModal.vipOnly
    })
    shareModal.show = false
    message.success(data.message || '分享成功')
    loadSkills()
    loadCategories()
  } catch (e) {
    message.error('分享失败: ' + e.message)
  } finally {
    shareModal.submitting = false
  }
}

// ==================== 排行榜 ====================
async function showRanking(type = 'hot', range = 'all') {
  rankingModal.type = type
  rankingModal.range = range
  rankingModal.show = true
  rankingModal.loading = true
  try {
    const data = await apiGet('/skills/ranking', {type, range, limit: 50})
    rankingModal.list = data.list || []
  } catch (e) {
    message.error('加载排行榜失败: ' + e.message)
  } finally {
    rankingModal.loading = false
  }
}

// ==================== 我的分享 ====================
async function showMyShares() {
  if (!isLoggedIn.value) {
    message.warning('请先登录')
    loginModal.show = true
    return
  }
  mySharesModal.show = true
  mySharesModal.loading = true
  try {
    const data = await apiGet('/user/skills', {page: 1, pageSize: 100})
    mySharesModal.list = data.list || []
  } catch (e) {
    message.error('加载我的分享失败: ' + e.message)
  } finally {
    mySharesModal.loading = false
  }
}

function formatSize(bytes) {
  if (!bytes || bytes <= 0) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB'
}

// 剥离 SKILL.md 的 YAML FrontMatter，仅渲染正文
function stripFrontmatter(md) {
  if (!md) return ''
  const s = md.replace(/^\uFEFF/, '').trimStart()
  if (s.startsWith('---')) {
    const end = s.indexOf('\n---', 3)
    if (end !== -1) {
      return s.slice(end + 4).trim()
    }
  }
  return md
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
            placeholder="搜索技能..."
            clearable
            style="width: 260px"
            @keyup.enter="handleSearch"
          />
          <n-button type="primary" @click="handleSearch">搜索</n-button>
          <n-button quaternary @click="showRanking('hot')">🏆 排行榜</n-button>
          <n-button quaternary @click="showMyShares">📦 我的分享</n-button>
        </n-space>
        <n-space>
          <n-button type="success" @click="showShareModal">📤 分享我的技能</n-button>
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
          <n-radio-button value="downloads">⬇️ 导入</n-radio-button>
        </n-radio-group>
        <n-divider vertical />
        <n-button
          :type="vipOnlyFilter ? 'warning' : 'default'"
          size="small"
          @click="vipOnlyFilter = !vipOnlyFilter; pagination.page = 1; loadSkills()"
        >
          👑 VIP专属
        </n-button>
      </n-space>

      <n-spin :show="loading">
        <n-grid :cols="3" :x-gap="12" :y-gap="12" responsive="screen">
          <n-gi v-for="item in skills" :key="item.id">
            <n-card
              hoverable
              size="small"
              style="cursor: pointer; height: 100%"
              @click="showDetail(item.id)"
            >
              <template #header>
                <n-space align="center" :size="6">
                  <n-text strong style="font-size: 15px">{{ item.name }}</n-text>
                  <n-tag v-if="item.vipOnly" type="warning" size="tiny" round>👑 VIP</n-tag>
                </n-space>
              </template>
              <template #header-extra>
                <n-tag v-if="item.category" size="small" type="info">{{ item.category }}</n-tag>
              </template>
              <n-ellipsis :line-clamp="2" :tooltip="false" style="color: var(--n-text-color-3); font-size: 13px; margin-bottom: 8px">
                {{ item.description || item.summary || '暂无描述' }}
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
                      📁 {{ item.fileCount || 0 }} 文件 · {{ formatSize(item.packageSize) }}
                    </n-text>
                    <n-text :type="item.isLiked ? 'error' : 'default'" style="cursor: pointer" @click.stop="handleLike(item)">
                      {{ item.isLiked ? '❤️' : '🤍' }} {{ item.likesCount || 0 }}
                    </n-text>
                    <n-text :type="item.isFavorited ? 'warning' : 'default'" style="cursor: pointer" @click.stop="handleFavorite(item)">
                      {{ item.isFavorited ? '⭐' : '☆' }} {{ item.favoritesCount || 0 }}
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
        <n-empty v-if="!loading && skills.length === 0" description="暂无技能分享，快来分享第一个技能吧" style="margin-top: 40px" />
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

    <!-- 技能详情 -->
    <n-modal v-model:show="detailModal.show" preset="card" style="width: 900px; max-width: 95vw" :title="detailModal.data?.name || '技能详情'">
      <template v-if="detailModal.data">
        <n-space align="left" justify="space-between" style="margin-bottom: 12px">
          <n-space align="left" :size="8">
            <n-tag v-if="detailModal.data.vipOnly" type="warning" size="small" round>👑 VIP专属</n-tag>
            <n-tag v-if="detailModal.data.category" type="info" size="small">{{ detailModal.data.category }}</n-tag>
            <n-tag size="small">📁 {{ detailModal.data.dirName }}</n-tag>
            <n-tag size="small" :bordered="false">{{ detailModal.data.fileCount || 0 }} 文件 · {{ formatSize(detailModal.data.packageSize) }}</n-tag>
            <n-text depth="3" style="font-size: 12px">
              {{ detailModal.data.user?.nickname || detailModal.data.user?.username || '匿名' }} · {{ formatTime(detailModal.data.createdAt) }}
            </n-text>
          </n-space>
          <n-space :size="8">
            <n-button
              v-if="currentUser && detailModal.data.userId === currentUser.id"
              size="tiny"
              type="error"
              @click="handleDeleteSkill(detailModal.data)"
            >
              🗑️ 删除分享
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
            <n-button
              size="tiny"
              type="success"
              :loading="detailModal.importing"
              @click="handleImport(detailModal.data)"
            >
              ⬇️ 导入到本地
            </n-button>
          </n-space>
        </n-space>

        <n-space vertical :size="8">
          <n-space :size="4" v-if="detailModal.data.tags">
            <n-tag v-for="tag in detailModal.data.tags.split(',').filter(t=>t)" :key="tag" size="small" round>{{ tag.trim() }}</n-tag>
          </n-space>
          <n-text v-if="detailModal.data.description" depth="3" style="font-size: 13px">{{ detailModal.data.description }}</n-text>
          <n-divider style="margin: 4px 0">SKILL.md</n-divider>
          <div style="max-height: 520px; overflow-y: auto; position: relative">
            <MdPreview
              :model-value="stripFrontmatter(detailModal.data.summary)"
              :theme="editorTheme"
              style="text-align: left"
            />
            <div
              v-if="detailModal.data.needVip"
              style="position: absolute; bottom: 0; left: 0; right: 0; height: 120px; background: linear-gradient(to bottom, transparent, var(--n-color)); display: flex; align-items: flex-end; justify-content: center; padding-bottom: 16px"
            >
              <n-space vertical align="center" :size="4">
                <n-tag type="warning" size="medium" round>👑 VIP专属技能</n-tag>
                <n-text depth="3" style="font-size: 12px">开通VIP后可导入完整技能包</n-text>
              </n-space>
            </div>
          </div>
        </n-space>
      </template>
    </n-modal>

    <!-- 登录/注册 -->
    <n-modal v-model:show="loginModal.show" preset="card" style="width: 400px" title="账号">
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

    <!-- 分享我的技能 -->
    <n-modal v-model:show="shareModal.show" preset="card" style="width: 640px; max-width: 95vw" title="分享我的技能到广场">
      <n-space vertical :size="12">
        <n-space align="center" :size="8">
          <n-text style="width: 70px">本地技能</n-text>
          <n-select
            v-model:value="shareModal.dirName"
            :options="shareModal.localSkills.map(s => ({label: (s.name || s.dirName) + ' (' + s.dirName + ')', value: s.dirName}))"
            placeholder="选择要分享的本地技能"
            :loading="shareModal.packing"
            style="width: 400px"
            @update:value="onShareSkillChange"
          />
        </n-space>
        <n-text v-if="shareModal.content" depth="3" style="font-size: 12px; padding-left: 78px">
          📁 {{ shareModal.fileCount }} 个文件 · 压缩包 {{ formatSize(shareModal.packageSize) }}（分享上限 2MB）
        </n-text>
        <n-space align="center" :size="8">
          <n-text style="width: 70px">技能名称</n-text>
          <n-input v-model:value="shareModal.name" placeholder="技能名称" style="width: 400px" />
        </n-space>
        <n-space align="start" :size="8">
          <n-text style="width: 70px; line-height: 32px">描述</n-text>
          <n-input v-model:value="shareModal.description" type="textarea" :rows="2" placeholder="技能简短描述（留空自动取 SKILL.md 的 description）" style="width: 400px" />
        </n-space>
        <n-space align="center" :size="8">
          <n-text style="width: 70px">分类</n-text>
          <n-input v-model:value="shareModal.category" placeholder="如: 技术分析, 数据处理" style="width: 190px" />
          <n-text style="width: 36px">标签</n-text>
          <n-input v-model:value="shareModal.tags" placeholder="逗号分隔" style="width: 174px" />
        </n-space>
        <n-space align="center">
          <n-text>VIP专属</n-text>
          <n-switch v-model:value="shareModal.vipOnly" />
          <n-text depth="3" style="font-size: 12px">仅VIP用户可导入该技能包</n-text>
        </n-space>
        <n-space justify="end">
          <n-button @click="shareModal.show = false">取消</n-button>
          <n-button type="primary" :loading="shareModal.submitting" :disabled="!shareModal.content" @click="handleShare">分享</n-button>
        </n-space>
      </n-space>
    </n-modal>

    <!-- 排行榜 -->
    <n-modal v-model:show="rankingModal.show" preset="card" style="width: 900px; max-width: 95vw" title="🏆 技能排行榜">
      <n-space vertical :size="12">
        <n-space :size="8">
          <n-text depth="3" style="font-size: 13px">类型:</n-text>
          <n-button :type="rankingModal.type === 'hot' ? 'primary' : 'default'" size="small" @click="showRanking('hot', rankingModal.range)">🔥 综合热度</n-button>
          <n-button :type="rankingModal.type === 'likes' ? 'primary' : 'default'" size="small" @click="showRanking('likes', rankingModal.range)">❤️ 点赞</n-button>
          <n-button :type="rankingModal.type === 'downloads' ? 'primary' : 'default'" size="small" @click="showRanking('downloads', rankingModal.range)">⬇️ 导入</n-button>
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
                <n-text strong>{{ item.name }}</n-text>
                <n-tag v-if="item.vipOnly" type="warning" size="tiny" round>VIP</n-tag>
                <n-text depth="3" style="font-size: 12px">{{ item.user?.nickname || item.user?.username || '匿名' }}</n-text>
                <n-text depth="3" style="font-size: 12px">
                  ❤️ {{ item.likesCount || 0 }} · ⭐ {{ item.favoritesCount || 0 }} · ⬇️ {{ item.downloadsCount || 0 }}
                </n-text>
              </n-space>
            </n-list-item>
          </n-list>
          <n-empty v-if="!rankingModal.loading && rankingModal.list.length === 0" description="暂无数据" style="margin-top: 20px" />
        </n-spin>
      </n-space>
    </n-modal>

    <!-- 我的分享 -->
    <n-modal v-model:show="mySharesModal.show" preset="card" style="width: 800px; max-width: 95vw" title="📦 我的技能分享">
      <n-spin :show="mySharesModal.loading">
        <n-list bordered>
          <n-list-item v-for="item in mySharesModal.list" :key="item.id">
            <n-space align="center" justify="space-between" style="width: 100%">
              <n-space align="center" :size="12" style="cursor: pointer" @click="mySharesModal.show = false; showDetail(item.id)">
                <n-text strong>{{ item.name }}</n-text>
                <n-tag v-if="item.vipOnly" type="warning" size="tiny" round>VIP</n-tag>
                <n-tag v-if="item.category" size="tiny">{{ item.category }}</n-tag>
                <n-text depth="3" style="font-size: 12px">{{ timeAgo(item.createdAt) }}</n-text>
                <n-text depth="3" style="font-size: 12px">
                  ❤️ {{ item.likesCount || 0 }} · ⬇️ {{ item.downloadsCount || 0 }}
                </n-text>
              </n-space>
              <n-button size="tiny" type="error" quaternary @click="handleDeleteSkill(item)">删除</n-button>
            </n-space>
          </n-list-item>
        </n-list>
        <n-empty v-if="!mySharesModal.loading && mySharesModal.list.length === 0" description="您还没有分享过技能" style="margin-top: 20px" />
      </n-spin>
    </n-modal>
  </div>
</template>

<style scoped>
:deep(.md-editor-preview) {
  text-align: left;
}
</style>
