<script setup lang="ts">
// 交易所公告资讯：币安官方公告（CMS）+ 币安广场新闻 + OKX 官方公告（v5 公共接口）+ 欧易星球快讯（SSR 抓取）聚合展示。
// 后端 FetchExchangeAnnouncements 双通道（直连优先 → 合约代理），国内直连受限时自动降级。
import {computed, h, onBeforeMount, ref} from 'vue'
import {NTag, NTooltip} from 'naive-ui'
import {GetExchangeAnnouncements} from "../../wailsjs/go/main/App";
import {BrowserOpenURL} from "../../wailsjs/runtime";

const props = defineProps({
  darkTheme: {type: Boolean, default: false},
})

const loading = ref(false)
const errorMsg = ref('')
const rows = ref<any[]>([])
const source = ref<'all' | 'binance' | 'binancenews' | 'okx' | 'okxorbit'>('all')
const catalog = ref('')
const page = ref(1)
const hasMore = ref(false)

/** 分类选项随来源切换；all=多源聚合，okxorbit=快讯无分类 */
const catalogOptions = computed(() => {
  if (source.value === 'binance') {
    return [
      {label: '全部分类', value: ''},
      {label: '新币上线', value: '48'},
      {label: '下架', value: '161'},
      {label: '最新新闻', value: '49'},
      {label: '最新活动', value: '93'},
      {label: '空投', value: '128'},
      {label: 'API 更新', value: '51'},
      {label: '维护更新', value: '157'},
    ]
  }
  if (source.value === 'okx') {
    return [
      {label: '全部分类', value: ''},
      {label: '新币上线', value: 'announcements-new-listings'},
      {label: '交易更新', value: 'announcements-trading-updates'},
      {label: 'API 公告', value: 'announcements-api'},
      {label: 'Web3', value: 'announcements-web3'},
      {label: '充提暂停/恢复', value: 'announcements-deposit-withdrawal-suspension-resumption'},
      {label: 'P2P 交易', value: 'announcements-p2p-trading'},
    ]
  }
  return [{label: '全部分类', value: ''}]
})

const sourceMeta: Record<string, { label: string; tag: 'warning' | 'success' | 'info' | 'default' }> = {
  binance: {label: '币安', tag: 'warning'},
  binancenews: {label: '币安快讯', tag: 'warning'},
  okx: {label: 'OKX', tag: 'success'},
  okxorbit: {label: 'OKX快讯', tag: 'info'},
}

function fmtTime(ms: any): string {
  const n = Number(ms)
  if (!Number.isFinite(n) || n <= 0) return '-'
  const d = new Date(n)
  const p = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function openRow(row: any) {
  if (row.url) BrowserOpenURL(row.url)
}

async function fetchData(nextPage = 1) {
  try {
    loading.value = true
    errorMsg.value = ''
    const list = await GetExchangeAnnouncements(source.value, catalog.value, nextPage)
    rows.value = nextPage > 1 ? [...rows.value, ...(list || [])] : (list || [])
    page.value = nextPage
    hasMore.value = (list || []).length >= 20
  } catch (e: any) {
    errorMsg.value = '获取公告失败：' + (e?.message || e)
  } finally {
    loading.value = false
  }
}

function onSourceChange() {
  catalog.value = ''
  fetchData(1)
}

onBeforeMount(() => fetchData(1))

const titleSpan = (row: any) =>
  h('span',
    {
      style: `cursor:${row.url ? 'pointer' : 'default'}`,
      onClick: () => openRow(row),
    },
    row.title)

const columns = computed(() => [
  {
    title: '时间',
    key: 'publishedAt',
    width: 145,
    render: (row: any) => h('span', {style: 'font-size:12px; color: var(--n-text-color-disabled, #999)'}, fmtTime(row.publishedAt)),
  },
  {
    title: '来源',
    key: 'source',
    width: 90,
    render: (row: any) =>
      h(NTag, {type: sourceMeta[row.source]?.tag || 'default', size: 'small', bordered: false},
        {default: () => sourceMeta[row.source]?.label || row.source}),
  },
  {
    title: '分类',
    key: 'catalog',
    width: 130,
    ellipsis: {tooltip: true},
    render: (row: any) => row.catalog || '-',
  },
  {
    title: '标题',
    key: 'title',
    ellipsis: {tooltip: true},
    render: (row: any) =>
      row.summary
        ? h(NTooltip, {trigger: 'hover', placement: 'top-start', style: 'max-width: 420px'},
            {
              trigger: () => titleSpan(row),
              default: () => h('span', {style: 'white-space: normal; word-break: break-all; font-size: 12px'}, row.summary),
            })
        : titleSpan(row),
  },
])
</script>

<template>
  <div style="--wails-draggable:no-drag">
    <n-flex align="center" :size="8" style="margin-bottom: 10px">
      <n-radio-group v-model:value="source" size="small" @update:value="onSourceChange">
        <n-radio-button value="all">全部</n-radio-button>
        <n-radio-button value="binance">币安</n-radio-button>
        <n-radio-button value="binancenews">币安快讯</n-radio-button>
        <n-radio-button value="okx">OKX</n-radio-button>
        <n-radio-button value="okxorbit">OKX快讯</n-radio-button>
      </n-radio-group>
      <n-select
        v-model:value="catalog"
        size="small"
        style="width: 150px"
        :options="catalogOptions"
        :disabled="source === 'all'"
        @update:value="fetchData(1)"
      />
      <n-button size="small" :loading="loading" @click="fetchData(1)">刷新</n-button>
      <n-text depth="3" style="font-size: 12px">币安/OKX 官方公告 + 币安广场 + 欧易星球快讯 · 点击标题打开原文</n-text>
    </n-flex>

    <n-alert v-if="errorMsg && rows.length === 0" type="warning" :bordered="false" style="margin-bottom: 10px">
      <template #header>
        {{ errorMsg }}
        <n-button size="tiny" type="primary" @click="fetchData(1)" style="margin-left: 10px">重试</n-button>
      </template>
    </n-alert>

    <n-spin :show="loading" size="small">
      <n-data-table
        :columns="columns"
        :data="rows"
        :bordered="false"
        :single-line="false"
        size="small"
        striped
        :max-height="560"
        :row-props="(row) => (row.url ? {style: 'cursor: pointer', onClick: () => openRow(row)} : {})"
      />
    </n-spin>

    <n-flex justify="center" style="margin-top: 10px" v-if="rows.length > 0">
      <n-button size="small" :loading="loading" :disabled="!hasMore" @click="fetchData(page + 1)">
        {{ hasMore ? '加载更多' : '没有更多了' }}
      </n-button>
    </n-flex>
  </div>
</template>
