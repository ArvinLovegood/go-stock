<script setup lang="ts">
// Bitget 美股永续合约（RWA）榜单：24h 行情 + 资金费率/标记价/基差，支持搜索、排序与详情查看。
// 数据来自 api.bitget.com/api/v2/mix/market；国内直连受限时由后端自动降级「Bitget 合约代理」，仍不可达则提示配置代理。
import {computed, h, onBeforeUnmount, onBeforeMount, ref} from 'vue'
import {
  GetBitgetFuturesSymbols,
  GetBitgetFuturesTicker,
} from "../../wailsjs/go/main/App";
import BitgetDerivativesPanel from "./BitgetDerivativesPanel.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

const props = defineProps({
  darkTheme: {type: Boolean, default: false},
})

const REFRESH_MS = 5000
const task = ref<any>(null)
const loading = ref(false)
const errorMsg = ref('')
/** 合并后的榜单数据：ticker 为主，叠加 symbol 的展示名 */
const rows = ref<any[]>([])
const keyword = ref('')
const sortBy = ref<'percent' | 'amount' | 'funding'>('amount')

const detailShow = ref(false)
const detailSymbol = ref('')
const detailName = ref('')

/** symbol(大写) → 展示名 */
let symbolNames: Record<string, string> = {}

function num(v: any): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

/** 涨跌配色：红涨绿跌（与 A 股习惯一致） */
function pctColor(n: number): string {
  if (n > 0) return '#ef4444'
  if (n < 0) return '#22c55e'
  return ''
}

/** 成交额（USDT）按中文单位缩写 */
function fmtAmount(v: any): string {
  const n = num(v)
  if (n >= 1e8) return (n / 1e8).toFixed(2) + '亿'
  if (n >= 1e4) return (n / 1e4).toFixed(2) + '万'
  return n.toFixed(0)
}

/** 价格：过大/过小时保留合适精度 */
function fmtPrice(v: any): string {
  const n = num(v)
  if (n >= 1000) return n.toFixed(1)
  if (n >= 1) return n.toFixed(3)
  return n.toPrecision(4)
}

/** 资金费率（小数）→ 百分比字符串，保留 4 位 */
function fmtRate(v: any): string {
  if (v === undefined || v === null || v === '') return '--'
  return (num(v) * 100).toFixed(4) + '%'
}

/** Bitget 只返回比率形式的 change24h（小数），此处换算为百分数 */
function changePercent(t: any): number {
  return num(t.change24h) * 100
}

const filtered = computed(() => {
  const kw = keyword.value.trim().toUpperCase()
  let list = rows.value
  if (kw) {
    list = list.filter((r) =>
      String(r.symbol || '').toUpperCase().includes(kw) ||
      String(r.name || '').toUpperCase().includes(kw),
    )
  }
  const sorted = [...list]
  if (sortBy.value === 'percent') {
    sorted.sort((a, b) => num(b.priceChangePercent) - num(a.priceChangePercent))
  } else if (sortBy.value === 'funding') {
    sorted.sort((a, b) => Math.abs(num(b.fundingRate)) - Math.abs(num(a.fundingRate)))
  } else {
    sorted.sort((a, b) => num(b.quoteVolume) - num(a.quoteVolume))
  }
  return sorted
})

async function fetchData() {
  try {
    loading.value = true
    errorMsg.value = ''
    const [tickers, symbols] = await Promise.all([
      GetBitgetFuturesTicker(''),
      symbolNames && Object.keys(symbolNames).length === 0
        ? GetBitgetFuturesSymbols()
        : Promise.resolve(null),
    ])
    if (symbols) {
      for (const s of symbols || []) {
        symbolNames[String(s.symbol || '').toUpperCase()] = s.displayName || s.symbol
      }
    }
    const merged: any[] = []
    for (const t of tickers || []) {
      const sym = String(t.symbol || '').toUpperCase()
      const mark = num(t.markPrice)
      const index = num(t.indexPrice)
      const basis = index > 0 ? ((mark - index) / index) * 100 : 0
      merged.push({
        symbol: sym,
        name: symbolNames[sym] || sym,
        lastPr: t.lastPr,
        priceChange: num(t.lastPr) - num(t.open24h),
        priceChangePercent: changePercent(t),
        high24h: t.high24h,
        low24h: t.low24h,
        quoteVolume: num(t.quoteVolume),
        markPrice: t.markPrice,
        indexPrice: t.indexPrice,
        basis,
        fundingRate: t.fundingRate,
      })
    }
    rows.value = merged
    if (merged.length === 0) {
      errorMsg.value = '美股永续数据不可达：请在「设置 → 合约代理」中配置代理后重试'
    }
  } catch (e: any) {
    console.error('fetchBitgetFutures error:', e)
    errorMsg.value = '请求失败: ' + (e?.message || e || '未知错误')
  } finally {
    loading.value = false
  }
}

function startRefresh() {
  stopRefresh()
  fetchData()
  // 美股永续 7x24 交易，无需交易时段判断
  task.value = setInterval(fetchData, REFRESH_MS)
}

function stopRefresh() {
  if (task.value) {
    clearInterval(task.value)
    task.value = null
  }
}

function openDetail(row: any) {
  detailSymbol.value = row.symbol
  detailName.value = row.name || row.symbol
  detailShow.value = true
}

onBeforeMount(() => {
  startRefresh()
})

onBeforeUnmount(() => {
  stopRefresh()
})

const columns = computed(() => [
  {
    title: '合约',
    key: 'name',
    width: 180,
    render: (row: any) => `${row.name} ${row.symbol}`,
  },
  {
    title: '最新价',
    key: 'lastPr',
    render: (row: any) => fmtPrice(row.lastPr),
  },
  {
    title: '24h涨跌幅',
    key: 'priceChangePercent',
    render: (row: any) => {
      const n = num(row.priceChangePercent)
      const icon = n > 0 ? '↑' : n < 0 ? '↓' : ''
      const color = pctColor(n)
      return h('span', {style: color ? {color, fontWeight: '600'} : undefined}, `${n.toFixed(2)}% ${icon}`)
    },
  },
  {
    title: '24h成交额',
    key: 'quoteVolume',
    render: (row: any) => fmtAmount(row.quoteVolume),
  },
  {
    title: '资金费率',
    key: 'fundingRate',
    render: (row: any) => fmtRate(row.fundingRate),
  },
  {
    title: '标记价',
    key: 'markPrice',
    render: (row: any) => fmtPrice(row.markPrice),
  },
  {
    title: '基差',
    key: 'basis',
    render: (row: any) => num(row.basis).toFixed(3) + '%',
  },
  {
    title: '操作',
    key: 'action',
    width: 90,
    render: (row: any) => '详情',
  },
])
</script>

<template>
  <div style="--wails-draggable:no-drag">
    <n-flex align="center" :size="8" style="margin-bottom: 10px">
      <n-input
        v-model:value="keyword"
        placeholder="搜索合约（如 AAPL / 苹果）"
        clearable
        size="small"
        style="max-width: 240px"
      />
      <n-select
        v-model:value="sortBy"
        size="small"
        style="width: 150px"
        :options="[
          {label: '按成交额', value: 'amount'},
          {label: '按涨跌幅', value: 'percent'},
          {label: '按资金费率', value: 'funding'},
        ]"
      />
      <n-button size="small" @click="fetchData">刷新</n-button>
      <n-text depth="3" style="font-size: 12px">共 {{ filtered.length }} 个合约 · 每 {{ REFRESH_MS / 1000 }} 秒自动刷新</n-text>
    </n-flex>

    <n-alert v-if="errorMsg && rows.length === 0" type="warning" :bordered="false" style="margin-bottom: 10px">
      <template #header>
        {{ errorMsg }}
        <n-button size="tiny" type="primary" @click="fetchData" style="margin-left: 10px">重试</n-button>
      </template>
    </n-alert>

    <n-spin :show="loading" size="small">
      <n-data-table
        :columns="columns"
        :data="filtered"
        :bordered="false"
        :single-line="false"
        size="small"
        striped
        :max-height="620"
        :row-props="(row) => ({ style: 'cursor: pointer', onClick: () => openDetail(row) })"
        :row-class-name="(row) => (num(row.priceChangePercent) > 0 ? 'bt-row-up' : num(row.priceChangePercent) < 0 ? 'bt-row-down' : '')"
      />
    </n-spin>

    <n-modal
      v-model:show="detailShow"
      preset="card"
      :title="detailName + ' ' + detailSymbol + ' 美股永续详情'"
      style="width: 92vw; max-width: 1180px"
    >
      <n-tabs type="line" animated>
        <n-tab-pane name="derivatives" tab="衍生指标">
          <BitgetDerivativesPanel
            v-if="detailShow"
            :symbol="detailSymbol"
            :name="detailName"
            :dark-theme="props.darkTheme"
          />
        </n-tab-pane>
        <n-tab-pane name="kline" tab="K 线">
          <StockLightweightKlineChart
            v-if="detailShow"
            :code="'bt:' + detailSymbol"
            :stock-name="detailName"
            :dark-theme="props.darkTheme"
            :chart-height="520"
            :realtime-interval-ms="5000"
          />
        </n-tab-pane>
      </n-tabs>
    </n-modal>
  </div>
</template>

<style scoped>
:deep(.bt-row-up td) {
  background-color: rgba(239, 68, 68, 0.06);
}
:deep(.bt-row-down td) {
  background-color: rgba(34, 197, 94, 0.06);
}
</style>