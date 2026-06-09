<script setup>
import { GetStockList, GetConfig } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { NAutoComplete, NButton, NFlex, NText, NInputGroup } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onBeforeMount, onMounted, onBeforeUnmount, ref } from 'vue'

const message = useMessage()
const searchQuery = ref('')
const selectedCode = ref('000001.SH')
const selectedName = ref('上证指数')
const stockList = ref([])
const options = ref([])
const darkTheme = ref(false)
const chartHeight = ref(window.innerHeight - 230)
const recentStocks = ref([])
const unsupportedCode = ref(false)
let stockChangeHandler = null

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (c.toLowerCase().startsWith('gb_')) return ''
  if (/\.(SH|SZ|BJ|HK|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  return ''
}

function findStockList(val) {
  if (!val || !val.trim()) {
    options.value = []
    return
  }
  const q = val.trim().toLowerCase()
  const filtered = stockList.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.ts_code.toLowerCase().includes(q)
  ).slice(0, 30)
  options.value = filtered.map(item => ({
    label: item.name + ' - ' + item.ts_code,
    value: item.ts_code,
  }))
}

function handleSearch(value) {
  const emCode = toEastMoneyCode(value)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  const found = stockList.value.find(item => item.ts_code === value)
  selectedName.value = found ? found.name : ''
  addToRecent(value, selectedName.value)
}

function addToRecent(code, name) {
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name })
  if (list.length > 10) list.length = 10
  recentStocks.value = list
  try {
    localStorage.setItem('kline-recent-stocks', JSON.stringify(list))
  } catch {}
}

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) recentStocks.value = JSON.parse(raw)
  } catch {}
}

function selectRecent(code, name) {
  const emCode = toEastMoneyCode(code)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  selectedName.value = name
  addToRecent(code, name)
}

function updateChartHeight() {
  chartHeight.value = Math.max(400, window.innerHeight - 230)
}

onBeforeMount(() => {
  GetStockList('').then(result => {
    stockList.value = result || []
  }).catch(err => { console.error('GetStockList error:', err) })
  GetConfig().then(result => {
    darkTheme.value = !!result.darkTheme
  }).catch(err => { console.error('GetConfig error:', err) })
})

onMounted(() => {
  loadRecentStocks()
  updateChartHeight()
  window.addEventListener('resize', updateChartHeight)

  stockChangeHandler = (data) => {
    if (data && data.ts_code) {
      const emCode = toEastMoneyCode(data.ts_code)
      if (!emCode) {
        unsupportedCode.value = true
        return
      }
      unsupportedCode.value = false
      selectedCode.value = emCode
      selectedName.value = data.name || ''
      addToRecent(data.ts_code, data.name || '')
    }
  }
  EventsOn('klineSelectStock', stockChangeHandler)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateChartHeight)
})
</script>


<template>
  <div class="kline-analysis-page" :class="{ 'kline-analysis-page--dark': darkTheme }">
    <div class="kline-title-bar">
      <NText :depth="darkTheme ? 1 : 3" style="font-size: 15px; font-weight: 700">{{ selectedName }}&nbsp;</NText>
      <NText depth="3" style="font-size: 13px">{{ selectedCode }}</NText>
    </div>
    <StockLightweightKlineChart
      :key="selectedCode"
      :code="selectedCode"
      :stockName="selectedName"
      :darkTheme="darkTheme"
      :chartHeight="chartHeight"
      :realtimeIntervalMs="60000"
    />

    <div class="kline-search-bar">
      <n-input-group>
        <n-auto-complete
          v-model:value="searchQuery"
          :options="options"
          placeholder="股票名称/代码搜索..."
          clearable
          :on-select="handleSearch"
          @update:value="findStockList"
        />
        <n-button type="primary">
          🔍
        </n-button>
      </n-input-group>
      <NFlex v-if="unsupportedCode" align="center" :size="6" style="margin-top: 4px">
        <NText type="warning" style="font-size: 12px">该股票暂不支持K线图</NText>
      </NFlex>
      <div v-if="recentStocks.length && !selectedCode" class="recent-stocks">
        <NText depth="3" style="font-size: 11px; white-space: nowrap">最近:</NText>
        <n-button
          v-for="s in recentStocks.slice(0, 6)"
          :key="s.code"
          size="tiny"
          secondary
          @click="selectRecent(s.code, s.name)"
        >
          {{ s.name }}
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.kline-analysis-page {
  width: 100%;
  padding: 4px 8px;
  box-sizing: border-box;
  --wails-draggable: no-drag;
  position: relative;
}
.kline-analysis-page--dark {
  background: #0a0a0a;
  color: #e2e8f0;
}
.kline-title-bar {
  padding: 2px 0 4px 0;
}
.kline-search-bar {
  position: fixed;
  bottom: 18px;
  right: 12px;
  z-index: 10;
  width: 320px;
}
.recent-stocks {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  flex-wrap: wrap;
}
</style>
