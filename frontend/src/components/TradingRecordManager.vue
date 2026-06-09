<script setup>
import { h, onMounted, onUnmounted, ref, reactive } from 'vue'
import {
  AddTradingRecord,
  GetTradingRecordList,
  GetTradingRecordStatistics,
  UpdateTradingRecord,
  DeleteTradingRecord,
  CheckFrequentTrading,
  GetAllStockInfoList,
  GetStockRealTimePrice,
  GetConfig
} from '../../wailsjs/go/main/App'
import {
  NButton,
  NDataTable,
  NDatePicker,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NModal,
  NNumberAnimation,
  NSelect,
  NSpace,
  NStatistic,
  NTag,
  NText,
  NAutoComplete,
  useMessage,
  useNotification
} from 'naive-ui'
import sparkLine from "./stockSparkLine.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

const message = useMessage()
const notify = useNotification()

const showKlineModal = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const longStopLossPrice = ref(0)
const longTakeProfitPrice = ref(0)
const costPrice = ref(0)
const darkTheme = ref(false)

const dataRef = ref([])
const loadingRef = ref(true)
const statisticsRef = ref(null)
const refreshTimer = ref(null)

const showAddModal = ref(false)
const showEditModal = ref(false)

const formData = reactive({
  ID: 0,
  StockCode: '',
  StockName: '',
  Direction: '买入',
  Price: 0,
  Volume: 0,
  Amount: 0,
  TradingTime: Date.now(),
  Reason: '',
  StopLossPrice: 0,
  TakeProfitPrice: 0,
  Fee: 0,
  MarketValue: 0,
  Mindset: '',
})

const directionOptions = [
  { label: '全部', value: '' },
  { label: '买入', value: '买入' },
  { label: '卖出', value: '卖出' }
]

const stockCodeOptions = reactive([])
const stockNameOptions = reactive([])

function searchStock(value) {
  if (!value || value.length < 1) {
    stockCodeOptions.splice(0, stockCodeOptions.length)
    stockNameOptions.splice(0, stockNameOptions.length)
    return
  }
  GetAllStockInfoList({
    searchKeyWord: value
  }).then((res) => {
    if (res && res.list) {
      const codeList = res.list.map(item => ({
        label: `${item.SECUCODE} - ${item.SECURITY_NAME_ABBR}`,
        value: item.SECUCODE,
        market: item.MARKET,
        code: item.SECUCODE,
        name: item.SECURITY_NAME_ABBR
      }))
      const nameList = res.list.map(item => ({
        label: `${item.SECURITY_NAME_ABBR} (${item.SECUCODE})`,
        value: item.SECURITY_NAME_ABBR,
        market: item.MARKET,
        code: item.SECUCODE,
        name: item.SECURITY_NAME_ABBR
      }))
      stockCodeOptions.splice(0, stockCodeOptions.length, ...codeList)
      stockNameOptions.splice(0, stockNameOptions.length, ...nameList)
    }
  }).catch(err => {
    console.error('搜索股票失败:', err)
  })
}

function getMarketPrefix(market) {
  if (!market) return ''
  const marketMap = {
    '上海': 'sh',
    '深圳': 'sz',
    '北京': 'bj',
    '沪市': 'sh',
    '深市': 'sz',
    '北交所': 'bj'
  }
  return marketMap[market] || ''
}

function convertToStockCode(code, market) {
  if (!code) return ''
  const upperCode = code.toUpperCase()
  if (upperCode.includes('.SH')) {
    return 'sh' + code.split('.')[0]
  }
  if (upperCode.includes('.SZ')) {
    return 'sz' + code.split('.')[0]
  }
  if (upperCode.includes('.BJ')) {
    return 'bj' + code.split('.')[0]
  }
  if (code.startsWith('hk') || code.startsWith('HK')) {
    return code.toLowerCase()
  }
  if (code.startsWith('us') || code.startsWith('US')) {
    return code.toLowerCase()
  }
  const prefix = getMarketPrefix(market)
  if (prefix) {
    return prefix + code
  }
  if (code.startsWith('6')) return 'sh' + code
  if (code.startsWith('0') || code.startsWith('3')) return 'sz' + code
  if (code.startsWith('8') || code.startsWith('9')) return 'bj' + code
  return code
}

function handleStockCodeSelect(value) {
  const option = stockCodeOptions.find(opt => opt.value === value)
  formData.StockCode = value
  formData.StockName = option ? option.name : ''
  fetchStockPrice(value, option ? option.market : '')
}

function handleStockNameSelect(value) {
  const option = stockNameOptions.find(opt => opt.value === value)
  formData.StockName = value
  formData.StockCode = option ? option.code : ''
  fetchStockPrice(option ? option.code : '', option ? option.market : '')
}

function fetchStockPrice(stockCode, market) {
  if (!stockCode) return
  const fullCode = convertToStockCode(stockCode, market)
  GetStockRealTimePrice(fullCode).then((res) => {
    if (res && res.code === 0 && res.price > 0) {
      formData.Price = res.price
    }
  }).catch(err => {
    console.error('获取股票价格失败:', err)
  })
}

/** 当前自然月 [月初 0 点, 月末当日]（供日期区间选择与 formatDate 查询） */
function currentMonthDateRange() {
  const now = new Date()
  // return [
  //   new Date(now.getFullYear(), now.getMonth(), 1),
  //   new Date(now.getFullYear(), now.getMonth() + 1, 0)
  // ]
  return null
}

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: '',
  direction: '',
  range: currentMonthDateRange(),
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

function formatDate(dateVal) {
  const date = new Date(dateVal)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatAmount(n) {
  return Number(n).toFixed(2)
}

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  // 不带后缀的代码，根据规则添加前缀
  if (c.startsWith('6')) return 'sh' + c.toLowerCase()
  if (c.startsWith('0') || c.startsWith('3')) return 'sz' + c.toLowerCase()
  if (c.startsWith('8') || c.startsWith('9')) return 'bj' + c.toLowerCase()
  return c.toLowerCase()
}

function openKlineChart(row) {
  klineStockCode.value = toEastMoneyCode(row.StockCode)
  klineStockName.value = row.StockName || ''
  showKlineModal.value = true
  longStopLossPrice.value = row.StopLossPrice || 0
  longTakeProfitPrice.value = row.TakeProfitPrice || 0
  costPrice.value = row.Price || 0
}



function formatRowTradingTime(row) {
  console.log('formatRowTradingTime:', row)
  const t = row.TradingTime
  if (t == null || t === '') return '-'
  let date
  if (typeof t === 'string' && t.length >= 19 && t.includes('T')) {
    date = new Date(t.substring(0, 19).replace('T', ' ') + ' UTC')
  } else if (typeof t === 'string') {
    date = new Date(t)
  } else {
    date = new Date(t)
  }
  const utc8Offset = 8 * 60 * 60 * 1000
  const localOffset = date.getTimezoneOffset() * 60 * 1000
  const utc8Time = new Date(date.getTime() + utc8Offset - localOffset)
  const pad = (n) => String(n).padStart(2, '0')
  return `${utc8Time.getFullYear()}-${pad(utc8Time.getMonth() + 1)}-${pad(utc8Time.getDate())} ${pad(utc8Time.getHours())}:${pad(utc8Time.getMinutes())}:${pad(utc8Time.getSeconds())}`
}

/** 统一列表行字段（Wails/JSON 可能为 PascalCase），供表格渲染与刷新使用 */
function normalizeTradingRecordRow(row) {
  if (!row || typeof row !== 'object') return row
  const closePrice = Number(row.closePrice ?? row.ClosePrice ?? 0)
  const profitAmount = Number(row.profitAmount ?? row.ProfitAmount ?? 0)
  const profitPercent = Number(row.profitPercent ?? row.ProfitPercent ?? 0)
  return { ...row, closePrice, profitAmount, profitPercent }
}

function query({ page, pageSize = 12, keyword = '', direction = '', startDate = '', endDate = '' }) {
  return new Promise((resolve, reject) => {
    GetTradingRecordList({
      page,
      pageSize,
      keyword,
      direction,
      startDate,
      endDate
    })
      .then((res) => {
        const raw = res.list ?? []
        const list = raw.map(normalizeTradingRecordRow)
        const total = res.total ?? 0
        const pageCount = res.totalPages ?? 1
        resolve({
          pageCount,
          data: list,
          total,
          page
        })
      })
      .catch(reject)
  })
}

/** 定时静默刷新当前列表与统计，不占用 loadingRef，避免与上次请求重叠时整页停更 */
function silentRefreshCurrentPage() {
  query({
    page: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
    keyword: paginationReactive.keyword,
    direction: paginationReactive.direction,
    startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
    endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
  })
    .then((data) => {
      dataRef.value = data.data
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
    })
    .catch(() => {})
  fetchStatistics()
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      keyword: paginationReactive.keyword,
      direction: paginationReactive.direction,
      startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
      endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
    })
      .then((data) => {
        dataRef.value = data.data
        paginationReactive.page = currentPage
        paginationReactive.pageCount = data.pageCount
        paginationReactive.itemCount = data.total
        loadingRef.value = false
      })
      .catch((e) => {
        message.error(e?.message || '加载交易日志失败')
        loadingRef.value = false
      })
  }
}

function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: 1,
      pageSize: paginationReactive.pageSize,
      keyword: paginationReactive.keyword,
      direction: paginationReactive.direction,
      startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
      endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
    })
      .then((data) => {
        dataRef.value = data.data
        paginationReactive.page = 1
        paginationReactive.pageCount = data.pageCount
        paginationReactive.itemCount = data.total
        loadingRef.value = false
      })
      .catch((e) => {
        message.error(e?.message || '加载交易日志失败')
        loadingRef.value = false
      })
  }
  fetchStatistics()
}

function fetchStatistics() {
  GetTradingRecordStatistics()
    .then((res) => {
      console.log('统计数据返回:', res)
      if (res) {
        statisticsRef.value = res
      }
    })
    .catch((e) => {
      console.error('获取统计数据失败:', e)
    })
}

function resetFilter() {
  paginationReactive.keyword = ''
  paginationReactive.direction = ''
  paginationReactive.range = currentMonthDateRange()
  handleSearch()
}

function openAddModal() {
  Object.assign(formData, {
    ID: 0,
    StockCode: '',
    StockName: '',
    Direction: '买入',
    Price: 0,
    Volume: 0,
    Amount: 0,
    TradingTime: Date.now(),
    Reason: '',
    StopLossPrice: 0,
    TakeProfitPrice: 0,
    Fee: 0,
    MarketValue: 0,
    Mindset: '',
  })
  showAddModal.value = true
}

function openEditModal(row) {
  Object.assign(formData, row)
  formData.TradingTime = new Date(row.TradingTime).getTime()
  showEditModal.value = true
}

function handleAdd() {
  const run = () => {
    formData.Amount = formData.Price * formData.Volume
    AddTradingRecord({
      ...formData,
      TradingTime: new Date(formData.TradingTime)
    })
      .then(() => {
        message.success('添加交易日志成功')
        showAddModal.value = false
        handleSearch()
      })
      .catch((e) => {
        message.error(e?.message || '添加交易日志失败')
      })
  }

  if (formData.Direction === '买入' && formData.StockCode) {
    CheckFrequentTrading(formData.StockCode)
      .then((res) => {
        console.log('检查频繁交易结果:', res)
        const canTrade = res.canTrade
        const msg = res.msg
        if (!canTrade) {
          message.warning(msg)
          return
        }
        run()
      })
      .catch((e) => {
        console.error('检查频繁交易失败:', e)
        run()
      })
  } else {
    run()
  }
}

function handleUpdate() {
  formData.Amount = formData.Price * formData.Volume
  UpdateTradingRecord({
    ...formData,
    TradingTime: new Date(formData.TradingTime)
  })
    .then(() => {
      message.success('更新交易日志成功')
      showEditModal.value = false
      handleSearch()
    })
    .catch((e) => {
      message.error(e?.message || '更新交易日志失败')
    })
}

function deleteTradingRecord(id) {
  DeleteTradingRecord(id)
    .then(() => {
      notify.info({ content: '删除成功', duration: 2000 })
      handleSearch()
    })
    .catch((e) => {
      message.error(e?.message || '删除交易日志失败')
    })
}

const columnsRef = ref([
  {
    title: '股票代码',
    key: 'StockCode',
    render(row) {
      return h(NText, { type: 'info' }, { default: () => row.StockCode })
    }
  },
  {
    title: '股票名称',
    key: 'StockName',
    render(row) {
      return h(NText, { type: 'info' }, { default: () => row.StockName })
    }
  },
  {
    title: '方向',
    key: 'Direction',
    width: 80,
    render(row) {
      return h(
        NTag,
        { type: row.Direction === '买入' ? 'error' : 'success', size: 'small', round: true, bordered: false },
        { default: () => row.Direction }
      )
    }
  },
  {
    title: '价格',
    key: 'Price',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.Price) })
    }
  },
  {
    title: '数量',
    key: 'Volume',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => String(row.Volume) })
    }
  },
  {
    title: '金额',
    key: 'Amount',
    width: 120,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.Amount) })
    }
  },
  {
    title: '收盘/最新价',
    key: 'closePrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, { default: () => formatAmount(row.closePrice) })
    }
  },
  {
    title: '盈亏额',
    key: 'profitAmount',
    width: 100,
    render(row) {
      const color = row.profitAmount > 0 ? 'error' : row.profitAmount < 0 ? 'success' : 'info'
      return h(NText, { type: color }, { default: () => formatAmount(row.profitAmount) })
    }
  },
  {
    title: '收益率',
    key: 'profitPercent',
    width: 100,
    render(row) {
      const color = row.profitPercent > 0 ? 'error' : row.profitPercent < 0 ? 'success' : 'info'
      const prefix = row.profitPercent > 0 ? '+' : ''
      return h(NText, { type: color }, { default: () => prefix + row.profitPercent?.toFixed(2) + '%' })
    }
  },
  {
    title: '时间',
    key: 'TradingTime',
    width: 180,
    render(row) {
      return formatRowTradingTime(row)
    }
  },
  {
    title: '止损价',
    key: 'StopLossPrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, {
        default: () => (row.StopLossPrice > 0 ? formatAmount(row.StopLossPrice) : '-')
      })
    }
  },
  {
    title: '止盈价',
    key: 'TakeProfitPrice',
    width: 100,
    render(row) {
      return h(NText, { type: 'info' }, {
        default: () => (row.TakeProfitPrice > 0 ? formatAmount(row.TakeProfitPrice) : '-')
      })
    }
  },
  {
    title: '交易理由',
    key: 'Reason',
    ellipsis: { tooltip: true }
  },
  {
    title: '操作',
    width: 200,
    render(row) {
      return [
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'info',
            onClick: () => openKlineChart(row)
          },
          { default: () => 'K线' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'warning',
            onClick: () => openEditModal(row)
          },
          { default: () => '编辑' }
        ),
        h(
          NTag,
          {
            strong: true,
            tertiary: true,
            type: 'error',
            onClick: () => deleteTradingRecord(row.ID)
          },
          { default: () => '删除' }
        )
      ]
    }
  }
])

onMounted(() => {
  // 获取主题配置
  GetConfig().then(result => {
    if (result.darkTheme) {
      darkTheme.value = true
    }
  })

  loadingRef.value = true
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    keyword: paginationReactive.keyword,
    direction: paginationReactive.direction,
    startDate: paginationReactive.range ? formatDate(paginationReactive.range[0]) : '',
    endDate: paginationReactive.range ? formatDate(paginationReactive.range[1]) : ''
  })
    .then((data) => {
      dataRef.value = data.data
      paginationReactive.page = 1
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
    .catch((e) => {
      message.error(e?.message || '加载交易日志失败')
      loadingRef.value = false
    })
  fetchStatistics()
  // 定时刷新收盘/最新价与盈亏：不抢 loading，避免请求进行中时跳过后续刷新
  refreshTimer.value = setInterval(() => {
    silentRefreshCurrentPage()
  }, 1000 * 10)
})

onUnmounted(() => {
  // 清除定时器
  if (refreshTimer.value) {
    clearInterval(refreshTimer.value)
  }
})
</script>

<template>
  <n-input-group>
    <n-date-picker v-model:value="paginationReactive.range" type="daterange" style="width: 40%" />
    <n-select
      v-model:value="paginationReactive.direction"
      :options="directionOptions"
      placeholder="交易方向"
      style="width: 15%"
      clearable
    />
    <n-input clearable placeholder="股票代码 / 名称" v-model:value="paginationReactive.keyword" />
    <n-button type="primary" ghost @click="handleSearch">搜索</n-button>
    <n-button @click="resetFilter">重置</n-button>
    <n-button type="primary" ghost @click="openAddModal">添加记录</n-button>
  </n-input-group>

  <n-grid :cols="6" :x-gap="12" style="margin-top: 12px; padding: 12px; border-radius: 4px">
    <n-grid-item>
      <n-statistic label="持仓金额(元)">
        <n-number-animation :from="0" :to="statisticsRef?.holdingsAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="持仓市值(元)">
        <n-number-animation :from="0" :to="statisticsRef?.currentValue || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总买入(元)">
        <n-number-animation :from="0" :to="statisticsRef?.totalBuyAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总卖出(元)">
        <n-number-animation :from="0" :to="statisticsRef?.totalSellAmount || 0" :precision="2" />
      </n-statistic>
    </n-grid-item>
    <n-grid-item>
      <n-statistic label="总收益(元)">
        <n-text :type="statisticsRef?.totalProfit > 0 ? 'error' : 'success'">
          <n-number-animation :from="0" :to="statisticsRef?.totalProfit || 0" :precision="2"  />
        </n-text>
      </n-statistic>
    </n-grid-item>
    <n-grid-item>

      <n-statistic label="收益率">
        <n-text :type="statisticsRef?.profitRate > 0 ? 'error' : 'success'">
          <n-number-animation :from="0" :to="statisticsRef?.profitRate || 0" :precision="2"  />%
        </n-text>
      </n-statistic>

    </n-grid-item>
  </n-grid>

  <n-data-table
    remote
    size="small"
    :columns="columnsRef"
    :data="dataRef"
    :loading="loadingRef"
    :pagination="paginationReactive"
    :row-key="(rowData) => rowData.ID"
    @update:page="handlePageChange"
    flex-height
    style="height: calc(100vh - 310px); margin-top: 10px"
  />

  <n-modal v-model:show="showAddModal" preset="card" title="添加交易日志" style="width: 820px;max-width: calc(100vw - 32px);">
    <n-form label-placement="top" size="small">
      <n-grid :cols="3" :x-gap="12" :y-gap="2">
        <n-grid-item>
          <n-form-item label="股票代码">
            <n-auto-complete
              v-model:value="formData.StockCode"
              :options="stockCodeOptions"
              placeholder="请输入股票代码"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="searchStock"
              @select="handleStockCodeSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票名称">
            <n-auto-complete
              v-model:value="formData.StockName"
              :options="stockNameOptions"
              placeholder="请输入股票名称"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="searchStock"
              @select="handleStockNameSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易方向">
            <n-select
              v-model:value="formData.Direction"
              :options="[
                { label: '买入', value: '买入' },
                { label: '卖出', value: '卖出' }
              ]"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="价格">
            <n-input-number v-model:value="formData.Price" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="成交数量">
            <n-input-number v-model:value="formData.Volume" :min="1" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易时间">
            <n-date-picker v-model:value="formData.TradingTime" type="datetime" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止损价">
            <n-input-number v-model:value="formData.StopLossPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止盈价">
            <n-input-number v-model:value="formData.TakeProfitPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="手续费">
            <n-input-number v-model:value="formData.Fee" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易理由">
            <n-input v-model:value="formData.Reason" type="textarea" placeholder="请输入交易理由" :rows="4"  style="text-align: left" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易心态/感悟/复盘/备注">
            <n-input v-model:value="formData.Mindset" type="textarea" placeholder="请输入交易心态" :rows="5" style="text-align: left" />
          </n-form-item>
        </n-grid-item>
      </n-grid>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showAddModal = false">取消</n-button>
        <n-button type="primary" @click="handleAdd">添加</n-button>
      </n-space>
    </template>
  </n-modal>

  <n-modal v-model:show="showEditModal" preset="card" title="编辑交易日志" style="width: 820px;max-width: calc(100vw - 32px);">
    <n-form label-placement="top" size="small">
      <n-grid :cols="3" :x-gap="12" :y-gap="2">
        <n-grid-item>
          <n-form-item label="股票代码">
            <n-auto-complete
              v-model:value="formData.StockCode"
              :options="stockCodeOptions"
              placeholder="请输入股票代码"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="searchStock"
              @select="handleStockCodeSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="股票名称">
            <n-auto-complete
              v-model:value="formData.StockName"
              :options="stockNameOptions"
              placeholder="请输入股票名称"
              :input-props="{ autocomplete: 'disabled' }"
              clearable
              @update:value="searchStock"
              @select="handleStockNameSelect"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易方向">
            <n-select
              v-model:value="formData.Direction"
              :options="[
                { label: '买入', value: '买入' },
                { label: '卖出', value: '卖出' }
              ]"
            />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="价格">
            <n-input-number v-model:value="formData.Price" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="成交数量">
            <n-input-number v-model:value="formData.Volume" :min="1" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="交易时间">
            <n-date-picker v-model:value="formData.TradingTime" type="datetime" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止损价">
            <n-input-number v-model:value="formData.StopLossPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="止盈价">
            <n-input-number v-model:value="formData.TakeProfitPrice" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item>
          <n-form-item label="手续费">
            <n-input-number v-model:value="formData.Fee" :precision="2" :min="0" style="width: 100%" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易理由">
            <n-input v-model:value="formData.Reason" type="textarea" placeholder="请输入交易理由" :rows="2" />
          </n-form-item>
        </n-grid-item>
        <n-grid-item :span="3">
          <n-form-item label="交易心态">
            <n-input v-model:value="formData.Mindset" type="textarea" placeholder="请输入交易心态" :rows="2" />
          </n-form-item>
        </n-grid-item>
      </n-grid>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showEditModal = false">取消</n-button>
        <n-button type="primary" @click="handleUpdate">更新</n-button>
      </n-space>
    </template>
  </n-modal>

  <n-modal v-model:show="showKlineModal" preset="card" :title="'K线 - ' + klineStockName" style="width: 95vw; max-width: 1400px">
    <StockLightweightKlineChart
      :code="klineStockCode"
      :stock-name="klineStockName"
      :chart-height="500"
      :dark-theme="darkTheme"
      :longStopLossPrice="longStopLossPrice"
      :longTakeProfitPrice="longTakeProfitPrice"
      :costPrice="costPrice"
    />
  </n-modal>
</template>

<style scoped></style>