<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted, onUnmounted, ref, reactive} from 'vue'
import {GetStockChanges, GetConfig, GetStockChangeHistory, SaveStockChangesToHistory, GetAllStockChangesWithPaging} from "../../wailsjs/go/main/App";
import {NButton, NTag, NText, useMessage, useNotification} from "naive-ui";

const notify = useNotification()
const message = useMessage()
const loadingRef = ref(true)
const dataRef = ref([])
const autoRefresh = ref(true)
const refreshInterval = ref(null)
const refreshSeconds = ref(10)
const countdown = ref(10)

const viewMode = ref('realtime')
const isTrading = ref(false)
const marketStatus = ref('')

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 50,
  itemCount: 0,
  keyword: "",
  range: null,
  startTime: null,
  endTime: null,
  minVolume: null,
  minAmount: null,
  minChangeRate: null,
  maxChangeRate: null,
  industry: "",
  concept: "",
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const volumeOptions = [
  { label: '不限', value: null },
  { label: '>100手', value: 10000 },
  { label: '>200手', value: 20000 },
  { label: '>500手', value: 50000 },
  { label: '>1000手', value: 100000 },
  { label: '>2000手', value: 200000 },
  { label: '>5000手', value: 500000 },
  { label: '>10000手', value: 1000000 },
  { label: '>20000手', value: 2000000 },
  { label: '>50000手', value: 5000000 },
]

const amountOptions = [
  { label: '不限', value: null },
  { label: '>100万', value: 1000000 },
  { label: '>500万', value: 5000000 },
  { label: '>1000万', value: 10000000 },
  { label: '>2000万', value: 20000000 },
  { label: '>5000万', value: 50000000 },
  { label: '>1亿', value: 100000000 },
  { label: '>2亿', value: 200000000 },
  { label: '>5亿', value: 500000000 },
]

const changeRateOptions = [
  { label: '不限', value: null },
  { label: '>3%', value: 3 },
  { label: '>5%', value: 5 },
  { label: '>7%', value: 7 },
  { label: '>9%', value: 9 },
  { label: '>涨停', value: 9.9 },
]

const bullishTypes = [
  {label: '火箭发射', value: '8201'},
  {label: '快速反弹', value: '8202'},
  {label: '大笔买入', value: '8193'},
  {label: '封涨停板', value: '4'},
  {label: '打开跌停板', value: '32'},
  {label: '有大买盘', value: '64'},
  {label: '竞价上涨', value: '8207'},
  {label: '高开5日线', value: '8209'},
  {label: '向上缺口', value: '8211'},
  {label: '60日新高', value: '8213'},
  {label: '60日大幅上涨', value: '8215'},
  {label: '打开涨停板', value: '16'},
]

const bearishTypes = [
  {label: '加速下跌', value: '8204'},
  {label: '高台跳水', value: '8203'},
  {label: '大笔卖出', value: '8194'},
  {label: '封跌停板', value: '8'},
  {label: '有大卖盘', value: '128'},
  {label: '竞价下跌', value: '8208'},
  {label: '低开5日线', value: '8210'},
  {label: '向下缺口', value: '8212'},
  {label: '60日新低', value: '8214'},
  {label: '60日大幅下跌', value: '8216'},
]

const allTypeValues = [...bullishTypes, ...bearishTypes].map(t => t.value)
const selectedTypes = ref(allTypeValues)

const columnsRef = ref([
  {
    title: '日期',
    key: 'changeDate',
    width: 120,
    render(row) {
      const date = row.changeDate || row.ChangeDate
      const time = row.changeTime || row.ChangeTime || row.time
      if (date) {
        return h(NText, {type: 'info'}, {default: () => date + ' ' + time})
      }
      return h(NText, {type: 'info'}, {default: () => time})
    }
  },
  {
    title: '代码',
    key: 'code',
    width: 80,
    render(row) {
      const code = row.stockCode || row.StockCode || row.code
      return h(NText, {type: 'info', style: 'cursor: pointer', onClick: () => copyCode(code)}, {default: () => code})
    }
  },
  {
    title: '名称',
    key: 'name',
    width: 100,
    render(row) {
      return row.stockName || row.StockName || row.name
    }
  },
  {
    title: '异动类型',
    key: 'typeName',
    width: 90,
    render(row) {
      const typeName = row.typeName || row.TypeName
      const bullishSet = new Set(['火箭发射', '快速反弹', '大笔买入', '封涨停板', '打开跌停板', '有大买盘', '竞价上涨', '高开5日线', '向上缺口', '60日新高', '60日大幅上涨', '打开涨停板'])
      const bearishSet = new Set(['加速下跌', '高台跳水', '大笔卖出', '封跌停板', '有大卖盘', '竞价下跌', '低开5日线', '向下缺口', '60日新低', '60日大幅下跌'])
      
      let tagType = 'default'
      if (bullishSet.has(typeName)) {
        tagType = 'error'
      } else if (bearishSet.has(typeName)) {
        tagType = 'success'
      }
      return h(NTag, {type: tagType, size: 'small'}, {default: () => typeName})
    }
  },
  {
    title: '价格',
    key: 'price',
    width: 70,
    render(row) {
      const price = row.price || row.Price
      if (price > 0) {
        return price.toFixed(2)
      }
      return '-'
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'changeRate',
    width: 85,
    render(row) {
      const changeRate = row.changeRate || row.ChangeRate
      if (changeRate !== 0) {
        const color = changeRate > 0 ? '#dc2626' : '#16a34a'
        const prefix = changeRate > 0 ? '+' : ''
        return h('span', {style: {color: color, fontWeight: '500'}}, prefix + changeRate.toFixed(2) + '%')
      }
      return '-'
    }
  },
  {
    title: '成交量',
    key: 'volume',
    width: 80,
    render(row) {
      const volume = row.volume || row.Volume
      if (volume > 0) {
        return formatVolume(volume)
      }
      return '-'
    }
  },
  {
    title: '金额',
    key: 'amount',
    width: 100,
    render(row) {
      const amount = row.amount || row.Amount
      if (amount > 0) {
        return formatAmount(amount)
      }
      return '-'
    }
  },
  {
    title: '行业',
    key: 'industry',
    width: 80,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return row.industry || row.Industry || '-'
    }
  },
  {
    title: '概念',
    key: 'concept',
    minWidth: 200,
    render(row) {
      const text = row.concept || row.Concept || '-'
      return h('div', {style: 'white-space: normal; line-height: 1.4;'}, text)
    }
  },
])

function formatVolume(vol) {
  const lots = vol / 100
  if (lots >= 100000000) {
    return (lots / 100000000).toFixed(2) + '亿手'
  } else if (lots >= 10000) {
    return (lots / 10000).toFixed(2) + '万手'
  } else if (lots >= 1) {
    return lots.toFixed(0) + '手'
  }
  return vol + '股'
}

function formatAmount(amount) {
  if (amount >= 100000000) {
    return (amount / 100000000).toFixed(2) + '亿'
  } else if (amount >= 10000) {
    return (amount / 10000).toFixed(2) + '万'
  }
  return amount.toFixed(2)
}

function copyCode(code) {
  navigator.clipboard.writeText(code).then(() => {
    message.success('已复制: ' + code)
  })
}

function checkTradingTime() {
  const now = new Date()
  const day = now.getDay()
  const hour = now.getHours()
  const minute = now.getMinutes()
  const currentTime = hour * 100 + minute

  if (day === 0 || day === 6) {
    isTrading.value = false
    marketStatus.value = '休市（周末）'
    return
  }

  const morningStart = 915
  const morningEnd = 1130
  const afternoonStart = 1257
  const afternoonEnd = 1500

  if (currentTime >= morningStart && currentTime <= morningEnd) {
    isTrading.value = true
    if (currentTime < 930) {
      marketStatus.value = '集合竞价'
    } else {
      marketStatus.value = '上午交易'
    }
  } else if (currentTime >= afternoonStart && currentTime <= afternoonEnd) {
    isTrading.value = true
    if (currentTime < 1300) {
      marketStatus.value = '午间集合竞价'
    } else {
      marketStatus.value = '下午交易'
    }
  } else if (currentTime > morningEnd && currentTime < afternoonStart) {
    isTrading.value = false
    marketStatus.value = '午间休市'
  } else if (currentTime > afternoonEnd) {
    isTrading.value = false
    marketStatus.value = '已收盘'
  } else {
    isTrading.value = false
    marketStatus.value = '未开盘'
  }
}

async function fetchRealtimeData() {
  loadingRef.value = true
  try {
    const types = selectedTypes.value.map(t => parseInt(t))
    const result = await GetStockChanges(types, 0, paginationReactive.pageSize)
    if (result) {
      dataRef.value = result.data || []
      paginationReactive.itemCount = result.totalCount || 0
    }
  } catch (e) {
    console.error('获取异动数据失败:', e)
  } finally {
    loadingRef.value = false
  }
}

async function fetchHistoryData() {
  loadingRef.value = true
  try {
    const query = {
      page: paginationReactive.page,
      pageSize: paginationReactive.pageSize,
    }
    if (paginationReactive.range && paginationReactive.range.length === 2) {
      query.startDate = formatDate(paginationReactive.range[0])
      query.endDate = formatDate(paginationReactive.range[1])
    }
    if (paginationReactive.startTime) {
      query.startTime = formatTime(paginationReactive.startTime)
    }
    if (paginationReactive.endTime) {
      query.endTime = formatTime(paginationReactive.endTime)
    }
    if (paginationReactive.minVolume) {
      query.minVolume = paginationReactive.minVolume
    }
    if (paginationReactive.minAmount) {
      query.minAmount = paginationReactive.minAmount
    }
    if (paginationReactive.minChangeRate) {
      query.minChangeRate = paginationReactive.minChangeRate
    }
    if (paginationReactive.maxChangeRate) {
      query.maxChangeRate = paginationReactive.maxChangeRate
    }
    if (paginationReactive.industry.trim()) {
      query.industry = paginationReactive.industry.trim()
    }
    if (paginationReactive.concept.trim()) {
      query.concept = paginationReactive.concept.trim()
    }
    if (paginationReactive.keyword.trim()) {
      const keyword = paginationReactive.keyword.trim()
      if (/^\d+$/.test(keyword)) {
        query.stockCode = keyword
      } else {
        query.stockName = keyword
      }
    }
    if (selectedTypes.value.length > 0) {
      query.changeTypes = selectedTypes.value.map(t => parseInt(t))
    }
    const result = await GetStockChangeHistory(query)
    if (result) {
      dataRef.value = result.list || []
      paginationReactive.itemCount = result.total || 0
      paginationReactive.pageCount = result.totalPages || 1
    }
  } catch (e) {
    console.error('获取历史数据失败:', e)
  } finally {
    loadingRef.value = false
  }
}

function formatDate(dateValue) {
  if (!dateValue) return ''
  const date = new Date(dateValue)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatTime(timeValue) {
  if (!timeValue) return ''
  const date = new Date(timeValue)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

async function fetchData() {
  if (viewMode.value === 'realtime') {
    await fetchRealtimeData()
  } else {
    await fetchHistoryData()
  }
}

async function saveCurrentData() {
  const types = selectedTypes.value.map(t => parseInt(t))
  const result = await SaveStockChangesToHistory(types)
  message.info(result)
}

function startAutoRefresh() {
  stopAutoRefresh()
  countdown.value = refreshSeconds.value
  refreshInterval.value = setInterval(() => {
    checkTradingTime()
    countdown.value--
    if (countdown.value <= 0) {
      if (viewMode.value === 'realtime' && !isTrading.value) {
        countdown.value = refreshSeconds.value
        return
      }
      fetchData()
      countdown.value = refreshSeconds.value
    }
  }, 1000)
}

function stopAutoRefresh() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

function toggleAutoRefresh() {
  if (autoRefresh.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

function selectAllBullish() {
  const bullishValues = bullishTypes.map(t => t.value)
  const currentBearish = selectedTypes.value.filter(t => bearishTypes.some(b => b.value === t))
  selectedTypes.value = [...bullishValues, ...currentBearish]
  fetchData()
}

function selectAllBearish() {
  const bearishValues = bearishTypes.map(t => t.value)
  const currentBullish = selectedTypes.value.filter(t => bullishTypes.some(b => b.value === t))
  selectedTypes.value = [...currentBullish, ...bearishValues]
  fetchData()
}

function selectAllTypes() {
  selectedTypes.value = allTypeValues
  fetchData()
}

function clearAllTypes() {
  selectedTypes.value = []
  fetchData()
}

function handleViewModeChange() {
  paginationReactive.page = 1
  if (viewMode.value === 'realtime') {
    checkTradingTime()
    if (isTrading.value) {
      autoRefresh.value = true
      startAutoRefresh()
    } else {
      autoRefresh.value = false
    }
  } else {
    autoRefresh.value = false
    stopAutoRefresh()
  }
  fetchData()
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    paginationReactive.page = currentPage
    fetchHistoryData()
  }
}

function handleSearch() {
  paginationReactive.page = 1
  fetchHistoryData()
}

function handleSearchKeyup(e) {
  if (e.key === 'Enter') {
    handleSearch()
  }
}

async function fetchAllCurrentData() {
  loadingRef.value = true
  try {
    const result = await GetAllStockChangesWithPaging(500)
    if (result) {
      dataRef.value = result.data || []
      paginationReactive.itemCount = result.totalCount || 0
      message.success(`获取到 ${result.data?.length || 0} 条当日异动数据`)
    }
  } catch (e) {
    console.error('获取全部异动数据失败:', e)
    message.error('获取全部异动数据失败')
  } finally {
    loadingRef.value = false
  }
}

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      document.documentElement.classList.add('dark')
    }
  })
})

onMounted(() => {
  checkTradingTime()
  fetchData()
  if (viewMode.value === 'realtime' && isTrading.value && autoRefresh.value) {
    startAutoRefresh()
  }
})

onBeforeUnmount(() => {
  stopAutoRefresh()
})
</script>

<template>
  <n-card class="stock-changes-card">
    <template #header>
      <n-space vertical>
        <n-space justify="space-between" align="center">
          <n-space align="center">
            <n-text strong>股票异动监控</n-text>
            <n-tag v-if="viewMode === 'realtime'" :type="isTrading ? 'success' : 'warning'" size="small">
              {{ marketStatus }}
            </n-tag>
            <n-tag :type="viewMode === 'realtime' ? 'error' : 'info'" size="small">
              {{ viewMode === 'realtime' ? '实时数据' : '历史数据' }}
            </n-tag>
            <n-text depth="3" style="font-size: 12px">共 {{ paginationReactive.itemCount }} 条记录</n-text>
          </n-space>
          <n-space align="center">
            <n-radio-group v-model:value="viewMode" @update:value="handleViewModeChange">
              <n-radio-button value="realtime">实时数据</n-radio-button>
              <n-radio-button value="history">历史数据</n-radio-button>
            </n-radio-group>
            
            <template v-if="viewMode === 'realtime'">
              <n-text v-if="autoRefresh && isTrading" depth="3" style="font-size: 12px">
                {{ countdown }}秒后刷新
              </n-text>
              <n-text v-else-if="!isTrading" depth="3" style="font-size: 12px">
                非交易时间暂停刷新
              </n-text>
              <n-switch v-model:value="autoRefresh" @update:value="toggleAutoRefresh" :disabled="!isTrading">
                <template #checked>自动刷新</template>
                <template #unchecked>手动刷新</template>
              </n-switch>
            </template>
            
            <n-button @click="fetchData" :loading="loadingRef" type="primary" size="small">
              刷新
            </n-button>
            
            <n-button v-if="viewMode === 'realtime'" @click="saveCurrentData" size="small">
              保存到历史
            </n-button>
          </n-space>
        </n-space>

        <n-alert v-if="viewMode === 'realtime' && !isTrading" type="info" size="small">
          当前非A股交易时间（周一至周五 9:30-11:30, 13:00-15:00），自动刷新已暂停。您可以查看历史数据或手动刷新。
        </n-alert>

        <n-space v-if="viewMode === 'history'" align="center">
          <n-input 
            v-model:value="paginationReactive.keyword" 
            placeholder="输入股票代码或名称" 
            clearable 
            style="width: 200px"
            @keyup="handleSearchKeyup"
          />
          <n-date-picker v-model:value="paginationReactive.range" type="daterange" clearable />
          <n-time-picker 
            v-model:value="paginationReactive.startTime" 
            placeholder="开始时间" 
            clearable 
            format="HH:mm:ss"
            style="width: 120px"
          />
          <n-time-picker 
            v-model:value="paginationReactive.endTime" 
            placeholder="结束时间" 
            clearable 
            format="HH:mm:ss"
            style="width: 120px"
          />
          <n-button type="primary" @click="handleSearch" :loading="loadingRef">
            查询
          </n-button>
          <n-button @click="fetchAllCurrentData" :loading="loadingRef">
            获取今日全部数据
          </n-button>
        </n-space>
        <n-space v-if="viewMode === 'history'" align="center" style="margin-top: 8px">
          <n-select
            v-model:value="paginationReactive.minVolume"
            :options="volumeOptions"
            placeholder="成交量筛选"
            style="width: 120px"
            clearable
          />
          <n-select
            v-model:value="paginationReactive.minAmount"
            :options="amountOptions"
            placeholder="金额筛选"
            style="width: 120px"
            clearable
          />
          <n-select
            v-model:value="paginationReactive.minChangeRate"
            :options="changeRateOptions"
            placeholder="涨跌幅筛选"
            style="width: 120px"
            clearable
          />
          <n-input 
            v-model:value="paginationReactive.industry" 
            placeholder="行业关键词" 
            clearable 
            style="width: 120px"
          />
          <n-input 
            v-model:value="paginationReactive.concept" 
            placeholder="概念关键词" 
            clearable 
            style="width: 120px"
          />
        </n-space>
        
        <n-space align="center" style="margin-top: 8px">
          <n-text depth="3">异动类型筛选：</n-text>
          <n-button size="tiny" type="primary" @click="selectAllTypes">全选</n-button>
          <n-button size="tiny" @click="clearAllTypes">清空</n-button>
          <n-text depth="3" style="font-size: 12px">已选 {{ selectedTypes.length }}/{{ allTypeValues.length }} 种</n-text>
        </n-space>

        <n-space vertical>
          <n-space align="center">
            <n-text style="color: #dc2626; font-weight: 500;">利好异动</n-text>
            <n-button size="tiny" @click="selectAllBullish">全选利好</n-button>
          </n-space>
          <n-checkbox-group v-model:value="selectedTypes" @update:value="fetchData">
            <n-space>
              <n-checkbox v-for="item in bullishTypes" :key="item.value" :value="item.value" :label="item.label">
                <template #default>
                  <n-text :style="{color: selectedTypes.includes(item.value) ? '#dc2626' : undefined}">{{ item.label }}</n-text>
                </template>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </n-space>

        <n-space vertical>
          <n-space align="center">
            <n-text style="color: #16a34a; font-weight: 500;">利空异动</n-text>
            <n-button size="tiny" @click="selectAllBearish">全选利空</n-button>
          </n-space>
          <n-checkbox-group v-model:value="selectedTypes" @update:value="fetchData">
            <n-space>
              <n-checkbox v-for="item in bearishTypes" :key="item.value" :value="item.value" :label="item.label">
                <template #default>
                  <n-text :style="{color: selectedTypes.includes(item.value) ? '#16a34a' : undefined}">{{ item.label }}</n-text>
                </template>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </n-space>
      </n-space>
    </template>

    <n-data-table
        remote
        :columns="columnsRef"
        :data="dataRef"
        :loading="loadingRef"
        :pagination="viewMode === 'history' ? paginationReactive : false"
        :bordered="false"
        flex-height
        :scroll-x="1300"
        striped
        size="small"
        @update:page="handlePageChange"
    />
  </n-card>
</template>

<style scoped>
.stock-changes-card {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.stock-changes-card :deep(.n-card-content) {
  flex: 1 1 0;
  min-height: 0;
}
.stock-changes-card :deep(.n-data-table) {
  height: 100%;
  min-height: 0;
}
</style>
