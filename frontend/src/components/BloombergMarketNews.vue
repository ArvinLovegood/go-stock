<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { GetBloombergMarketNews, OpenURL } from '../../wailsjs/go/main/App'

const items = ref([])
const loading = ref(false)
const error = ref('')
const updatedAt = ref('')
let timer

async function refresh() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const result = await GetBloombergMarketNews()
    items.value = Array.isArray(result) ? result : []
    updatedAt.value = new Date().toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
  } catch (err) {
    error.value = `获取失败：${String(err)}。请检查网络或设置中的海外 HTTP 代理。`
  } finally {
    loading.value = false
  }
}

function formatTime(value) {
  return new Date(value).toLocaleString('zh-CN', {
    timeZone: 'Asia/Shanghai',
    hour12: false,
  })
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, 5 * 60 * 1000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <n-card size="small" title="彭博市场新闻">
    <template #header-extra>
      <n-button size="small" :loading="loading" @click="refresh">刷新</n-button>
    </template>
    <n-text depth="3">来源：Bloomberg Markets RSS · 北京时间 · 原文可能需要订阅</n-text>
    <n-text v-if="updatedAt" depth="3"> · 上次成功更新：{{ updatedAt }}</n-text>
    <n-alert v-if="error" type="error" :bordered="false" style="margin-top: 12px">{{ error }}</n-alert>
    <n-spin :show="loading">
      <n-list v-if="items.length" bordered style="margin-top: 12px">
        <n-list-item v-for="item in items" :key="item.url">
          <n-space vertical :size="6">
            <n-space align="center">
              <n-tag size="small" :bordered="false">{{ formatTime(item.publishedAt) }}</n-tag>
              <n-button text type="primary" @click="OpenURL(item.url)">{{ item.title }}</n-button>
            </n-space>
            <n-text v-if="item.description" depth="2">{{ item.description }}</n-text>
          </n-space>
        </n-list-item>
      </n-list>
      <n-empty v-else-if="!loading && !error" description="暂无新闻" style="padding: 40px 0"/>
    </n-spin>
  </n-card>
</template>
