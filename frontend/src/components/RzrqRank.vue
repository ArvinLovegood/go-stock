<script setup lang="ts">
import {ref, onMounted, computed, h} from 'vue'
import {RzrqRank} from '../../wailsjs/go/main/App'
import {NDataTable, NDatePicker, NSelect, NSwitch, NButton, NFlex, NStatistic, NCard, NTag} from 'naive-ui'

const props = defineProps({
  darkTheme: {type: Boolean, default: false}
})

type RzrqItem = {
  stockCode: string
  stockName: string
  date: number
  rzye: string
  rzyeRate: string
  rqye: string
  rqyeRate: string
  jmr: string
  jmrRate: string
  rzmre: string
  rzche: string
  rzjmce: string
  lrye: string
  lryeRate: string
  yezf: string
  close_price: string
  close_profit: string
  marketId: string
}

const activeType = ref('hyList')
const dateTs = ref<number | null>(null)
const sortKey = ref('jmr')
const sortType = ref('desc')
const length = ref(20)
const loading = ref(false)
const data = ref<RzrqItem[]>([])

const queryDate = computed(() => {
  if (!dateTs.value) return ''
  const d = new Date(dateTs.value)
  return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
})

const typeOptions = [
  {label: '行业', value: 'hyList'},
  {label: '概念', value: 'gnList'},
  {label: '个股', value: 'ggList'},
]

const sortKeyOptions = [
  {label: '净买入额', value: 'jmr'},
  {label: '融资余额', value: 'rzye'},
  {label: '融券余额', value: 'rqye'},
  {label: '融资买入额', value: 'rzmre'},
  {label: '融资净买入额', value: 'rzjmce'},
  {label: '融资融券余额', value: 'lrye'},
  {label: '余额增幅', value: 'yezf'},
  {label: '涨跌幅', value: 'close_profit'},
]

const sortTypeOptions = [
  {label: '降序', value: 'desc'},
  {label: '升序', value: 'asc'},
]

const lengthOptions = [
  {label: '10条', value: 10},
  {label: '20条', value: 20},
  {label: '50条', value: 50},
  {label: '100条', value: 100},
]

function fmtAmount(v: string): string {
  // API 返回金额字段单位为千元，转换为元后格式化
  const n = parseFloat(v) * 1000
  if (isNaN(n)) return v
  if (Math.abs(n) >= 100000000) return (n / 100000000).toFixed(2) + '亿'
  if (Math.abs(n) >= 10000) return (n / 10000).toFixed(2) + '万'
  return n.toFixed(2)
}

function fmtPct(v: string): string {
  const n = parseFloat(v)
  if (isNaN(n)) return v
  return n.toFixed(2) + '%'
}

function profitColor(v: string): string {
  const n = parseFloat(v)
  if (isNaN(n)) return ''
  return n > 0 ? '#e88080' : n < 0 ? '#00b42a' : ''
}

const columns = computed(() => [
  {title: '排名', key: 'rank', width: 60, render: (_: any, i: number) => i + 1},
  {title: '名称', key: 'stockName', width: 100},
  {
    title: '收盘价', key: 'close_price', width: 80, align: 'right' as const,
    render: (r: RzrqItem) => parseFloat(r.close_price).toFixed(2)
  },
  {
    title: '涨跌幅', key: 'close_profit', width: 90, align: 'right' as const,
    render: (r: RzrqItem) => {
      const v = parseFloat(r.close_profit)
      const color = v > 0 ? '#e88080' : v < 0 ? '#00b42a' : ''
      const sign = v > 0 ? '+' : ''
      return h('span', {style: `color:${color}`}, `${sign}${v.toFixed(2)}%`)
    }
  },
  {
    title: '净买入额', key: 'jmr', width: 110, align: 'right' as const,
    render: (r: RzrqItem) => {
      const v = parseFloat(r.jmr)
      const color = v > 0 ? '#e88080' : v < 0 ? '#00b42a' : ''
      return h('span', {style: `color:${color}`}, fmtAmount(r.jmr))
    }
  },
  {title: '净买入占比', key: 'jmrRate', width: 100, align: 'right' as const, render: (r: RzrqItem) => fmtPct(r.jmrRate)},
  {title: '融资余额', key: 'rzye', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.rzye)},
  {title: '融资占比', key: 'rzyeRate', width: 90, align: 'right' as const, render: (r: RzrqItem) => fmtPct(r.rzyeRate)},
  {title: '融资买入额', key: 'rzmre', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.rzmre)},
  {title: '融资偿还额', key: 'rzche', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.rzche)},
  {title: '融资净买入', key: 'rzjmce', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.rzjmce)},
  {title: '融券余额', key: 'rqye', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.rqye)},
  {title: '融券占比', key: 'rqyeRate', width: 90, align: 'right' as const, render: (r: RzrqItem) => fmtPct(r.rqyeRate)},
  {title: '两融余额', key: 'lrye', width: 110, align: 'right' as const, render: (r: RzrqItem) => fmtAmount(r.lrye)},
  {title: '余额占比', key: 'lryeRate', width: 90, align: 'right' as const, render: (r: RzrqItem) => fmtPct(r.lryeRate)},
  {title: '余额增幅', key: 'yezf', width: 90, align: 'right' as const, render: (r: RzrqItem) => {
      const v = parseFloat(r.yezf)
      const color = v > 0 ? '#e88080' : v < 0 ? '#00b42a' : ''
      return h('span', {style: `color:${color}`}, `${v.toFixed(2)}%`)
    }
  },
])

const summary = computed(() => {
  if (!data.value.length) return null
  const totalJmr = data.value.reduce((s, r) => s + (parseFloat(r.jmr) || 0), 0)
  const totalRzye = data.value.reduce((s, r) => s + (parseFloat(r.rzye) || 0), 0)
  const totalRqye = data.value.reduce((s, r) => s + (parseFloat(r.rqye) || 0), 0)
  const upCount = data.value.filter(r => parseFloat(r.close_profit) > 0).length
  const downCount = data.value.filter(r => parseFloat(r.close_profit) < 0).length
  return {totalJmr, totalRzye, totalRqye, upCount, downCount}
})

const dataDate = computed(() => {
  if (!data.value.length) return ''
  const ts = data.value[0].date
  if (!ts) return ''
  // 兼容秒级和毫秒级时间戳
  const d = ts > 1e12 ? new Date(ts) : new Date(ts * 1000)
  return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
})

async function fetchData() {
  loading.value = true
  try {
    let res = await RzrqRank(activeType.value, sortKey.value, sortType.value, queryDate.value, length.value, 0)
    // 无数据时逐日回退，最多尝试前 7 天
    if (!res?.list?.length) {
      const baseTs = dateTs.value ?? Date.now()
      for (let i = 1; i <= 7; i++) {
        const d = new Date(baseTs - i * 24 * 60 * 60 * 1000)
        const qDate = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
        res = await RzrqRank(activeType.value, sortKey.value, sortType.value, qDate, length.value, 0)
        if (res?.list?.length) break
      }
    }
    data.value = res?.list || []
  } catch (e) {
    console.error('RzrqRank error:', e)
    data.value = []
  } finally {
    loading.value = false
  }
}

function onTypeChange() {
  fetchData()
}

function isDateDisabled(ts: number): boolean {
  return ts > Date.now()
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <n-card size="small" style="--wails-draggable:no-drag">
    <template #header>
      <n-flex align="center" :wrap="false">
        <n-text strong>融资融券余额</n-text>
        <n-text depth="3" style="font-size: 12px;">行业/概念/个股{{ dataDate ? ' · ' + dataDate : '' }}</n-text>
      </n-flex>
    </template>

    <n-flex align="center" :wrap="false" style="margin-bottom: 12px;" gap="12">
      <n-select v-model:value="activeType" :options="typeOptions" size="small"
                style="width: 100px;" @update:value="onTypeChange"/>
      <n-select v-model:value="sortKey" :options="sortKeyOptions" size="small"
                style="width: 150px;" @update:value="fetchData"/>
      <n-select v-model:value="sortType" :options="sortTypeOptions" size="small"
                style="width: 90px;" @update:value="fetchData"/>
      <n-select v-model:value="length" :options="lengthOptions" size="small"
                style="width: 90px;" @update:value="fetchData"/>
      <n-date-picker v-model:value="dateTs" type="date" size="small" clearable
                     :is-date-disabled="isDateDisabled"
                     placeholder="最新日期" style="width: 140px;" @update:value="fetchData"/>
      <n-button size="small" type="primary" @click="fetchData" :loading="loading">刷新</n-button>
    </n-flex>

    <n-card v-if="summary" size="small" style="margin-bottom: 12px;" :bordered="false">
      <n-flex align="center" gap="24">
        <n-statistic label="净买入合计" :value="fmtAmount(String(summary.totalJmr))"/>
        <n-statistic label="融资余额合计" :value="fmtAmount(String(summary.totalRzye))"/>
        <n-statistic label="融券余额合计" :value="fmtAmount(String(summary.totalRqye))"/>
        <n-statistic label="上涨">
          <template #default>
            <n-tag type="error" size="small" :bordered="false">{{ summary.upCount }}</n-tag>
          </template>
        </n-statistic>
        <n-statistic label="下跌">
          <template #default>
            <n-tag type="success" size="small" :bordered="false">{{ summary.downCount }}</n-tag>
          </template>
        </n-statistic>
      </n-flex>
    </n-card>

    <n-data-table
      :columns="columns"
      :data="data"
      :loading="loading"
      :bordered="false"
      :single-line="false"
      size="small"
      :scroll-x="1800"
      :row-key="(r: any) => r.stockCode"
    />
  </n-card>
</template>

<style scoped>
:deep(.n-data-table-thead) {
  position: sticky;
  top: 0;
  z-index: 2;
}
</style>
