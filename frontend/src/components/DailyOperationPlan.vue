<script setup>
import {h, onMounted, onUnmounted, reactive, ref, nextTick} from 'vue'
import * as echarts from 'echarts'
import {
  NAlert, NButton, NCard, NCheckbox, NCollapse, NCollapseItem, NDataTable, NDatePicker,
  NDescriptions, NDescriptionsItem, NDivider, NForm, NFormItem, NGrid, NGi, NInput,
  NInputNumber, NModal, NPopconfirm, NSelect, NSpace, NSwitch, NTag, NText, useMessage
} from 'naive-ui'
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue"
import sparkLine from "./stockSparkLine.vue"
import {
  GetDailyOperationPlanList, SaveDailyOperationPlan, DeleteDailyOperationPlan,
  UpdateDailyOperationPlanStatus, UpdateDailyOperationPlanAlert, GetStockRealTimePrice,
  GetTdxMinuteTimeData, GetAllTdxTransactionData, GetConfig
} from "../../wailsjs/go/main/App"

const message = useMessage()

const loadingRef = ref(false)
const darkTheme = ref(true) // 默认暗黑模式
const dataRef = ref([])
const priceMap = ref({}) // key: stockCode, value: {price, preClose, changePercent}
const showFormModal = ref(false)
const showDetailModal = ref(false)
const formLoading = ref(false)
const detailRef = ref(null)
// K线弹窗
const showKlineModal = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
// 成交明细弹窗
const showTransactionModal = ref(false)
const transactionStockName = ref('')
const transactionStockCode = ref('')
const transactionList = ref([])
const transactionLoading = ref(false)
const transactionChartRef = ref(null)
const transactionChart = ref(null)
const minuteBundle = ref(null)

const paginationReactive = reactive({
  page: 1,
  pageSize: 10,
  pageCount: 1,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page) => {
    paginationReactive.page = page
    loadList()
  },
  onUpdatePageSize: (pageSize) => {
    paginationReactive.pageSize = pageSize
    paginationReactive.page = 1
    loadList()
  }
})

const queryReactive = reactive({
  stockCode: '',
  stockName: '',
  planDate: null,
  status: ''
})

const statusOptions = [
  {label: '全部', value: ''},
  {label: '待执行', value: 'pending'},
  {label: '执行中', value: 'executing'},
  {label: '已完成', value: 'completed'},
  {label: '已过期', value: 'expired'}
]

const actionTypeOptions = [
  {label: '买入', value: 'buy'},
  {label: '观望', value: 'wait'},
  {label: '观察', value: 'observe'},
  {label: '止损', value: 'stop_loss'}
]

const notifyChannelOptions = [
  {label: '软件内提醒', value: 'app'},
  {label: '飞书', value: 'feishu'},
  {label: '钉钉', value: 'dingding'}
]

const actionTypeTagType = {
  buy: 'success',
  wait: 'warning',
  observe: 'info',
  stop_loss: 'error'
}

const actionTypeText = {
  buy: '买入',
  wait: '观望',
  observe: '观察',
  stop_loss: '止损'
}

const statusTagType = {
  pending: 'default',
  executing: 'info',
  completed: 'success',
  expired: 'error'
}

const statusText = {
  pending: '待执行',
  executing: '执行中',
  completed: '已完成',
  expired: '已过期'
}

// 表单数据
function emptyForm() {
  return {
    id: 0,
    planDate: Date.now(),
    planEndDate: null,
    stockCode: '',
    stockName: '',
    overallJudgment: '',
    scenarios: [emptyScenario()],
    discipline: [emptyDiscipline()],
    summary: '',
    riskWarning: '该股近一个月跌幅较大，日内振幅常超10%，属于极高波动品种。以上分析基于公开数据，不构成投资建议。投资有风险，入市需谨慎。请根据自身风险承受能力理性决策。',
    status: 'pending',
    remarks: '',
    enableAlert: true,
    notifyChannels: ['app', 'feishu', 'dingding']
  }
}

function emptyScenario() {
  return {
    title: '',
    condition: '',
    actionType: 'buy',
    action: '',
    position: '',
    buyPriceRange: '',
    stopLossPrice: '',
    target1: '',
    target2: '',
    strategy: '',
    isBest: false,
    triggerPriceMin: null,
    triggerPriceMax: null,
    stopLossPriceNum: null,
    target1Min: null,
    target1Max: null,
    target2Min: null,
    target2Max: null
  }
}

function emptyDiscipline() {
  return {principle: '', detail: ''}
}

const formRef = ref(emptyForm())

function addScenario() {
  formRef.value.scenarios.push(emptyScenario())
}

function removeScenario(index) {
  if (formRef.value.scenarios.length <= 1) {
    message.warning('至少保留一个情景')
    return
  }
  formRef.value.scenarios.splice(index, 1)
}

function addDiscipline() {
  formRef.value.discipline.push(emptyDiscipline())
}

function removeDiscipline(index) {
  if (formRef.value.discipline.length <= 1) {
    message.warning('至少保留一条纪律')
    return
  }
  formRef.value.discipline.splice(index, 1)
}

// 从价格文本解析数值，如 "460-470元" → [460, 470]，"400" → [400, 400]
function parsePriceRange(text) {
  if (!text) return null
  const m = text.match(/(\d+\.?\d*)\s*[-~–]\s*(\d+\.?\d*)/)
  if (m) return [parseFloat(m[1]), parseFloat(m[2])]
  const s = text.match(/(\d+\.?\d*)/)
  if (s) { const v = parseFloat(s[1]); return [v, v] }
  return null
}

function parsePrice(text) {
  if (!text) return 0
  const m = text.match(/(\d+\.?\d*)/)
  return m ? parseFloat(m[1]) : 0
}

// 文本失焦时自动填充量化数值（仅填充空值，不覆盖已有值）
function autoFillNumeric(sc) {
  if (!sc.triggerPriceMin && !sc.triggerPriceMax) {
    const r = parsePriceRange(sc.buyPriceRange)
    if (r) { sc.triggerPriceMin = r[0]; sc.triggerPriceMax = r[1] }
  }
  if (!sc.stopLossPriceNum) {
    sc.stopLossPriceNum = parsePrice(sc.stopLossPrice)
  }
  if (!sc.target1Min && !sc.target1Max) {
    const r = parsePriceRange(sc.target1)
    if (r) { sc.target1Min = r[0]; sc.target1Max = r[1] }
  }
  if (!sc.target2Min && !sc.target2Max) {
    const r = parsePriceRange(sc.target2)
    if (r) { sc.target2Min = r[0]; sc.target2Max = r[1] }
  }
}

function loadList() {
  loadingRef.value = true
  const query = {
    page: paginationReactive.page,
    pageSize: paginationReactive.pageSize,
    stockCode: queryReactive.stockCode,
    stockName: queryReactive.stockName,
    planDate: queryReactive.planDate ? formatDatePicker(queryReactive.planDate) : '',
    status: queryReactive.status
  }
  GetDailyOperationPlanList(query).then((data) => {
    dataRef.value = data.list || []
    paginationReactive.page = data.page
    paginationReactive.pageCount = data.totalPages
    paginationReactive.itemCount = data.total
    fetchPlanPrices(data.list || [])
  }).catch((err) => {
    message.error('加载失败:' + err)
  }).finally(() => {
    loadingRef.value = false
  })
}

// 将计划中的股票代码（603986.SH）转换为 API 格式（sh603986）
function convertCode(code) {
  if (!code) return ''
  const upper = code.toUpperCase()
  if (upper.includes('.SH')) return 'sh' + code.split('.')[0]
  if (upper.includes('.SZ')) return 'sz' + code.split('.')[0]
  if (upper.includes('.BJ')) return 'bj' + code.split('.')[0]
  if (upper.includes('.HK')) return 'hk' + code.split('.')[0]
  if (code.startsWith('us') || code.startsWith('US')) return code.toLowerCase()
  return code
}

// 转为东方财富代码格式（StockLightweightKlineChart 需要）
function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.HK')) return c.slice(0, -3).toLowerCase()
  return c.toLowerCase()
}

function openKlineChart(row) {
  klineStockCode.value = toEastMoneyCode(row.stockCode)
  klineStockName.value = row.stockName || ''
  showKlineModal.value = true
}

// === 成交明细常量与辅助函数（参考 stock.vue） ===
const SUPER_LARGE = 1000000   // 100万
const LARGE_AMOUNT = 200000   // 20万
const MEDIUM_AMOUNT = 40000   // 4万
const SHARE_PER_LOT = 100

function transactionAmount(row) {
  return (row.price || 0) * (row.vol || 0) * SHARE_PER_LOT
}
function classifyAmount(amount) {
  if (amount >= SUPER_LARGE) return 1
  if (amount >= LARGE_AMOUNT) return 2
  if (amount >= MEDIUM_AMOUNT) return 3
  return 4
}
function amountTagType(level) {
  switch (level) {
    case 1: return 'error'
    case 2: return 'warning'
    case 3: return 'info'
    default: return 'default'
  }
}
function amountTagName(level) {
  switch (level) {
    case 1: return '超大'
    case 2: return '大'
    case 3: return '中'
    default: return '小'
  }
}
function formatWan(val) {
  return (val / 10000).toFixed(2)
}

// 各档位统计
const transactionStats = ref([])
function calcTransactionStats() {
  const stats = [
    {level: 1, name: '超大单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0},
    {level: 2, name: '大单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0},
    {level: 3, name: '中单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0},
    {level: 4, name: '小单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0}
  ]
  for (const row of transactionList.value) {
    const amount = transactionAmount(row)
    const s = stats[classifyAmount(amount) - 1]
    if (row.buyOrSell === 0) { s.buyCount++; s.buyAmount += amount }
    else if (row.buyOrSell === 1) { s.sellCount++; s.sellAmount += amount }
    else { s.neutralCount++; s.neutralAmount += amount }
  }
  for (const s of stats) {
    s.totalAmount = s.buyAmount + s.sellAmount + s.neutralAmount
    s.netInflow = s.buyAmount - s.sellAmount
    s.buyPercent = s.totalAmount > 0 ? (s.buyAmount / s.totalAmount * 100) : 0
    s.sellPercent = s.totalAmount > 0 ? (s.sellAmount / s.totalAmount * 100) : 0
    s.neutralPercent = s.totalAmount > 0 ? (s.neutralAmount / s.totalAmount * 100) : 0
  }
  transactionStats.value = stats
}

// 成交明细表格列
const transactionColumns = [
  {title: '时间', key: 'time', width: 100, fixed: 'left'},
  {title: '价格', key: 'price', width: 80, align: 'right', render: (r) => r.price?.toFixed(2) || '-'},
  {title: '成交量(手)', key: 'vol', width: 100, align: 'right'},
  {
    title: '金额(元)', key: 'amount', width: 160, align: 'right',
    render(row) {
      const amount = transactionAmount(row)
      const level = classifyAmount(amount)
      return h('div', {style: 'display:flex; align-items:center; justify-content:flex-end; gap:6px;'}, [
        h('span', {}, amount.toLocaleString('zh-CN', {maximumFractionDigits: 2})),
        h(NTag, {type: amountTagType(level), size: 'small', bordered: false, style: 'min-width:36px; text-align:center;'}, () => amountTagName(level))
      ])
    }
  },
  {title: '笔数', key: 'num', width: 60, align: 'right'},
  {
    title: '方向', key: 'action', width: 70, align: 'center',
    render(row) {
      if (row.buyOrSell === 0) return h(NTag, {type: 'error', size: 'small', bordered: false}, () => '买')
      if (row.buyOrSell === 1) return h(NTag, {type: 'success', size: 'small', bordered: false}, () => '卖')
      return h(NTag, {type: 'default', size: 'small', bordered: false}, () => '中性')
    }
  }
]

// 渲染分时图（价格+成交量）
function renderTransactionChart() {
  const bundle = minuteBundle.value
  if (!bundle || !bundle.items || bundle.items.length === 0 || !transactionChartRef.value) return
  if (transactionChart.value) {
    transactionChart.value.dispose()
    transactionChart.value = null
  }
  const chart = echarts.init(transactionChartRef.value)
  transactionChart.value = chart

  const category = [], price = [], avg = [], vol = []
  let min = 0, max = 0
  for (let i = 0; i < bundle.items.length; i++) {
    const it = bundle.items[i]
    category.push(it.time)
    price.push(it.price)
    avg.push(it.avg)
    vol.push(it.vol)
    if (i === 0) { min = it.price; max = it.price }
    else { if (it.price < min) min = it.price; if (it.price > max) max = it.price }
  }
  const span = (max - min) || (max * 0.01 || 1)
  const yMin = (min - span * 0.1).toFixed(2)
  const yMax = (max + span * 0.1).toFixed(2)
  const preClose = bundle.preClose || 0

  chart.setOption({
    title: {
      subtext: `[${bundle.date || ''}] 昨收:${preClose} 今开:${bundle.open || 0} 最高:${bundle.high || 0} 最低:${bundle.low || 0} 收盘:${bundle.close || 0} 总量:${bundle.vol || 0} 总额:${(bundle.amount || 0).toFixed(2)}`,
      left: 'center', top: 6,
      subtextStyle: {color: darkTheme.value ? '#ccc' : '#456', fontSize: 12}
    },
    tooltip: {trigger: 'axis', axisPointer: {type: 'cross', label: {backgroundColor: '#505765'}}},
    legend: {data: ['价格', '均价', '成交量'], right: 30, top: 6},
    darkMode: darkTheme.value,
    grid: [
      {left: '8%', right: '8%', top: '20%', height: '50%'},
      {left: '8%', right: '8%', top: '76%', height: '16%'}
    ],
    xAxis: [
      {type: 'category', data: category, axisLabel: {show: false}},
      {gridIndex: 1, type: 'category', data: category}
    ],
    yAxis: [
      {scale: true, min: yMin, max: yMax, minInterval: 0.01, type: 'value', name: '价格', splitLine: {show: false}},
      {gridIndex: 1, type: 'value', name: '量', splitLine: {show: false}}
    ],
    series: [
      {name: '价格', type: 'line', data: price, showSymbol: false, lineStyle: {width: 2}, markLine: preClose > 0 ? {silent: true, data: [{yAxis: preClose, lineStyle: {type: 'dashed', color: '#999'}}]} : undefined},
      {name: '均价', type: 'line', data: avg, showSymbol: false, lineStyle: {width: 1, color: '#ff9800'}},
      {name: '成交量', type: 'bar', xAxisIndex: 1, yAxisIndex: 1, data: vol, itemStyle: {color: '#7fb5ec'}}
    ]
  })
}

async function showTransactionDetail(row) {
  transactionStockName.value = row.stockName || ''
  transactionStockCode.value = row.stockCode || ''
  transactionList.value = []
  transactionStats.value = []
  minuteBundle.value = null
  transactionLoading.value = true
  showTransactionModal.value = true
  const apiCode = convertCode(row.stockCode)
  try {
    // 并行获取分时数据和成交明细
    const [bundle, transactions] = await Promise.all([
      GetTdxMinuteTimeData(apiCode),
      GetAllTdxTransactionData(apiCode)
    ])
    if (bundle && bundle.items && bundle.items.length > 0) {
      minuteBundle.value = bundle
      nextTick(() => renderTransactionChart())
    }
    if (transactions && Array.isArray(transactions)) {
      transactionList.value = transactions
      calcTransactionStats()
    }
  } catch (e) {
    message.error('成交明细加载失败：' + (e?.message || e))
  } finally {
    transactionLoading.value = false
  }
}

function handleTransactionModalClose() {
  if (transactionChart.value) {
    transactionChart.value.dispose()
    transactionChart.value = null
  }
  minuteBundle.value = null
  transactionList.value = []
  transactionStats.value = []
}

// 批量获取列表中股票的最新价和涨跌幅
function fetchPlanPrices(list) {
  const codes = [...new Set(list.map(p => p.stockCode).filter(Boolean))]
  if (codes.length === 0) return
  codes.forEach(code => {
    const apiCode = convertCode(code)
    GetStockRealTimePrice(apiCode).then(res => {
      if (res && res.code === 0) {
        priceMap.value[code] = {
          ...priceMap.value[code],
          price: res.price || 0,
          preClose: res.preClose || 0,
          changePercent: res.changePercent || 0
        }
      }
    }).catch(() => {})
  })
}

function formatDatePicker(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function handleSearch() {
  paginationReactive.page = 1
  loadList()
}

function handleReset() {
  queryReactive.stockCode = ''
  queryReactive.stockName = ''
  queryReactive.planDate = null
  queryReactive.status = ''
  paginationReactive.page = 1
  loadList()
}

function openAddForm() {
  formRef.value = emptyForm()
  showFormModal.value = true
}

function openEditForm(row) {
  // 解析 JSON 字段
  let scenarios = []
  let discipline = []
  try {
    scenarios = row.scenarios ? JSON.parse(row.scenarios) : []
  } catch (e) {
    scenarios = []
  }
  try {
    discipline = row.discipline ? JSON.parse(row.discipline) : []
  } catch (e) {
    discipline = []
  }
  if (scenarios.length === 0) scenarios = [emptyScenario()]
  if (discipline.length === 0) discipline = [emptyDiscipline()]

  let planDateTs = Date.now()
  if (row.planDate) {
    const parsed = new Date(row.planDate)
    if (!isNaN(parsed.getTime())) planDateTs = parsed.getTime()
  }
  let planEndDateTs = null
  if (row.planEndDate) {
    const parsedEnd = new Date(row.planEndDate)
    if (!isNaN(parsedEnd.getTime())) planEndDateTs = parsedEnd.getTime()
  }

  let channels = ['app', 'feishu', 'dingding']
  try {
    channels = row.notifyChannels ? JSON.parse(row.notifyChannels) : ['app', 'feishu', 'dingding']
    if (!Array.isArray(channels) || channels.length === 0) {
      channels = ['app', 'feishu', 'dingding']
    }
  } catch (e) {
    channels = ['app', 'feishu', 'dingding']
  }

  formRef.value = {
    id: row.id,
    planDate: planDateTs,
    planEndDate: planEndDateTs,
    stockCode: row.stockCode,
    stockName: row.stockName,
    overallJudgment: row.overallJudgment,
    scenarios: scenarios,
    discipline: discipline,
    summary: row.summary,
    riskWarning: row.riskWarning,
    status: row.status,
    remarks: row.remarks,
    enableAlert: !!row.enableAlert,
    notifyChannels: channels
  }
  showFormModal.value = true
}

function handleSave() {
  if (!formRef.value.stockCode) {
    message.warning('请输入股票代码')
    return
  }
  if (!formRef.value.stockName) {
    message.warning('请输入股票名称')
    return
  }
  formLoading.value = true
  const payload = {
    id: formRef.value.id,
    planDate: formatDatePicker(formRef.value.planDate),
    planEndDate: formRef.value.planEndDate ? formatDatePicker(formRef.value.planEndDate) : '',
    stockCode: formRef.value.stockCode,
    stockName: formRef.value.stockName,
    overallJudgment: formRef.value.overallJudgment,
    scenarios: JSON.stringify(formRef.value.scenarios),
    discipline: JSON.stringify(formRef.value.discipline),
    summary: formRef.value.summary,
    riskWarning: formRef.value.riskWarning,
    status: formRef.value.status,
    remarks: formRef.value.remarks,
    enableAlert: !!formRef.value.enableAlert,
    notifyChannels: JSON.stringify(formRef.value.notifyChannels || [])
  }
  SaveDailyOperationPlan(payload).then((res) => {
    if (res.includes('成功')) {
      message.success(res)
      showFormModal.value = false
      loadList()
    } else {
      message.error(res)
    }
  }).catch((err) => {
    message.error('保存失败:' + err)
  }).finally(() => {
    formLoading.value = false
  })
}

function handleDelete(row) {
  DeleteDailyOperationPlan(row.id).then((res) => {
    if (res.includes('成功')) {
      message.success(res)
      loadList()
    } else {
      message.error(res)
    }
  })
}

function handleStatusChange(row, status) {
  UpdateDailyOperationPlanStatus(row.id, status).then((res) => {
    if (res.includes('成功')) {
      message.success(res)
      loadList()
    } else {
      message.error(res)
    }
  })
}

function handleAlertToggle(row, enable) {
  UpdateDailyOperationPlanAlert(row.id, enable).then((res) => {
    if (res.includes('成功')) {
      message.success(enable ? '已开启盘中预警' : '已关闭盘中预警')
    } else {
      message.error(res)
      loadList()
    }
  })
}

function openDetail(row) {
  let scenarios = []
  let discipline = []
  try {
    scenarios = row.scenarios ? JSON.parse(row.scenarios) : []
  } catch (e) {
  }
  try {
    discipline = row.discipline ? JSON.parse(row.discipline) : []
  } catch (e) {
  }
  detailRef.value = {...row, _scenarios: scenarios, _discipline: discipline}
  showDetailModal.value = true
}

// 从 scenarios JSON 中提取最优方案（isBest=true），没有则返回第一个
function getBestScenario(scenariosJson) {
  if (!scenariosJson) return null
  try {
    const arr = typeof scenariosJson === 'string' ? JSON.parse(scenariosJson) : scenariosJson
    if (!Array.isArray(arr) || arr.length === 0) return null
    return arr.find(s => s.isBest) || arr[0]
  } catch {
    return null
  }
}

const columnsRef = ref([
  {
    title: '计划日期', key: 'planDate', width: 160, fixed: 'left',
    render(row) {
      if (row.planEndDate) {
        return h('span', {}, `${row.planDate} ~ ${row.planEndDate}`)
      }
      return h('span', {}, row.planDate || '-')
    }
  },
  {
    title: '股票', key: 'stock', width: 140, fixed: 'left',
    render(row) {
      return h('div', null, [
        h('div', {style: 'font-weight:600'}, row.stockName),
        h('div', {style: 'font-size:12px;color:#999'}, row.stockCode)
      ])
    }
  },
  {
    title: '最新价', key: 'price', width: 90,
    render(row) {
      const p = priceMap.value[row.stockCode]
      if (!p || !p.price) return h('span', {style: 'color:#999'}, '-')
      return h('span', {style: 'font-weight:600'}, p.price.toFixed(2))
    }
  },
  {
    title: '涨跌幅', key: 'changePercent', width: 90,
    render(row) {
      const p = priceMap.value[row.stockCode]
      if (!p || !p.changePercent) return h('span', {style: 'color:#999'}, '-')
      const pct = p.changePercent.toFixed(2) + '%'
      const color = p.changePercent > 0 ? '#e02020' : p.changePercent < 0 ? '#00a000' : '#999'
      return h('span', {style: `font-weight:600;color:${color}`}, pct)
    }
  },
  {
    title: '分时', key: 'sparkline', width: 120,
    render(row, index) {
      const p = priceMap.value[row.stockCode]
      return h(sparkLine, {
        idSuffix: String(row.id || index),
        stockName: row.stockName || '',
        stockCode: row.stockCode,
        lastPrice: Number(p?.price || 0),
        openPrice: Number(p?.preClose || 0),
        darkTheme: darkTheme.value
      })
    }
  },
  {
    title: '状态', key: 'status', width: 90,
    render(row) {
      return h(NTag, {type: statusTagType[row.status] || 'default', size: 'small'},
        () => statusText[row.status] || row.status)
    }
  },
  {
    title: '盘中预警', key: 'enableAlert', width: 90,
    render(row) {
      return h(NSwitch, {
        size: 'small', value: !!row.enableAlert,
        onUpdateValue: (v) => handleAlertToggle(row, v)
      })
    }
  },
  {
    title: '操作动作', key: 'bestScenario', width: 100,
    render(row) {
      const sc = getBestScenario(row.scenarios)
      if (!sc) return h('span', {style: 'color:#999'}, '-')
      const children = []
      if (sc.actionType) {
        children.push(h(NTag, {
          size: 'small', type: actionTypeTagType[sc.actionType] || 'default', style: 'margin-right:4px'
        }, () => actionTypeText[sc.actionType] || sc.actionType))
      }
      if (sc.isBest) {
        children.push(h(NTag, {size: 'small', type: 'warning', round: true}, () => '⭐'))
      }
      return children.length > 0 ? h('div', null, children) : h('span', {style: 'color:#999'}, '-')
    }
  },
  {
    title: '触发条件', key: 'bestCondition', width: 140, ellipsis: {tooltip: true},
    render(row) {
      const sc = getBestScenario(row.scenarios)
      return h('span', null, sc?.condition || '-')
    }
  },
  {
    title: '买入区间', key: 'bestRange', width: 120,
    render(row) {
      const sc = getBestScenario(row.scenarios)
      if (!sc) return h('span', {style: 'color:#999'}, '-')
      const parts = []
      if (sc.buyPriceRange) parts.push(sc.buyPriceRange)
      if (sc.triggerPriceMin > 0 && sc.triggerPriceMax > 0) {
        parts.push(`触发:${sc.triggerPriceMin}-${sc.triggerPriceMax}`)
      }
      if (sc.stopLossPrice) parts.push(`止损:${sc.stopLossPrice}`)
      return parts.length > 0 ? h('span', {style: 'font-size:12px'}, parts.join(' / ')) : h('span', {style: 'color:#999'}, '-')
    }
  },
  {
    title: '策略说明', key: 'bestStrategy', ellipsis: {tooltip: true},
    render(row) {
      const sc = getBestScenario(row.scenarios)
      return h('span', {style: 'font-size:12px'}, sc?.strategy || '-')
    }
  },
  {
    title: '操作', key: 'actions', width: 340, fixed: 'right',
    render(row) {
      return h(NSpace, {size: 'small'}, () => [
        h(NButton, {size: 'small', type: 'info', text: true, onClick: () => openDetail(row)}, () => '查看'),
        h(NButton, {size: 'small', type: 'primary', text: true, onClick: () => openEditForm(row)}, () => '编辑'),
        h(NButton, {size: 'small', type: 'warning', text: true, onClick: () => openKlineChart(row)}, () => 'K线'),
        h(NButton, {size: 'small', type: 'success', text: true, onClick: () => showTransactionDetail(row)}, () => '明细'),
        h(NSelect, {
          size: 'small', value: row.status, options: statusOptions.filter(o => o.value !== ''),
          style: 'width:110px',
          onUpdateValue: (v) => handleStatusChange(row, v)
        }),
        h(NPopconfirm, {onPositiveClick: () => handleDelete(row)}, {
          trigger: () => h(NButton, {size: 'small', type: 'error', text: true}, () => '删除'),
          default: () => `确定删除 ${row.stockName} 的操作计划吗？`
        })
      ])
    }
  }
])

let refreshTimer = null

// 判断是否在交易时间内（A股 9:25-11:30 / 13:00-15:00）
function isTradingHours() {
  const now = new Date()
  const day = now.getDay()
  if (day === 0 || day === 6) return false
  const h = now.getHours(), m = now.getMinutes()
  const t = h * 60 + m
  return (t >= 565 && t <= 690) || (t >= 780 && t <= 900)
}

onMounted(() => {
  loadList()
  // 读取主题配置
  GetConfig().then(result => {
    if (result && result.darkTheme !== undefined) {
      darkTheme.value = !!result.darkTheme
    }
  }).catch(() => {})
  // 交易时间内每10秒刷新价格（sparkLine 组件 watchEffect 监听 lastPrice 自动刷新分时图）
  refreshTimer = setInterval(() => {
    if (isTradingHours() && dataRef.value.length > 0) {
      fetchPlanPrices(dataRef.value)
    }
  }, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <!-- 筛选栏 -->
  <n-space vertical style="margin-bottom: 12px">
    <n-space>
      <n-input v-model:value="queryReactive.stockCode" placeholder="股票代码" clearable style="width:160px"
               @keyup.enter="handleSearch"/>
      <n-input v-model:value="queryReactive.stockName" placeholder="股票名称" clearable style="width:160px"
               @keyup.enter="handleSearch"/>
      <n-date-picker v-model:value="queryReactive.planDate" type="date" placeholder="计划日期" clearable
                     style="width:160px"/>
      <n-select v-model:value="queryReactive.status" :options="statusOptions" placeholder="状态"
                style="width:140px" clearable/>
      <n-button type="primary" @click="handleSearch">查询</n-button>
      <n-button @click="handleReset">重置</n-button>
      <n-button type="primary" @click="openAddForm">+ 新增计划</n-button>
    </n-space>
  </n-space>

  <!-- 列表 -->
  <n-data-table
    :columns="columnsRef"
    :data="dataRef"
    :loading="loadingRef"
    :pagination="paginationReactive"
    :scroll-x="1200"
    remote
    size="small"
    striped
    flex-height
    style="height: calc(100vh - 210px); margin-top: 10px"
  />

    <!-- 新增/编辑 弹窗 -->
    <n-modal v-model:show="showFormModal" preset="card" :title="formRef.id ? '编辑操作计划' : '新增操作计划'"
             style="width:920px;max-width:95vw;" :mask-closable="false"
             :body-style="{ maxHeight: 'calc(90vh - 110px)', overflowY: 'auto' }">
      <n-form label-placement="top" size="small">
        <!-- 基本信息 -->
        <n-card size="small" title="基本信息" style="margin-bottom:12px" :bordered="false">
          <n-space>
            <n-form-item label="计划日期" required>
              <n-date-picker v-model:value="formRef.planDate" type="date" style="width:160px"/>
            </n-form-item>
            <n-form-item label="结束日期">
              <n-date-picker v-model:value="formRef.planEndDate" type="date" style="width:160px" clearable placeholder="留空=仅当天"/>
            </n-form-item>
            <n-form-item label="股票代码" required>
              <n-input v-model:value="formRef.stockCode" placeholder="如 603986" style="width:160px"/>
            </n-form-item>
            <n-form-item label="股票名称" required>
              <n-input v-model:value="formRef.stockName" placeholder="如 兆易创新" style="width:160px"/>
            </n-form-item>
            <n-form-item label="状态">
              <n-select v-model:value="formRef.status" :options="statusOptions.filter(o => o.value !== '')"
                        style="width:140px"/>
            </n-form-item>
          </n-space>
          <n-form-item label="总体判断">
            <n-input v-model:value="formRef.overallJudgment" type="textarea" :rows="3"
                     placeholder="对该股当前基本面+技术面的总体判断，是否可以买、仓位与止损要求等"/>
          </n-form-item>
          <n-space align="center">
            <n-form-item label="盘中预警">
              <n-switch v-model:value="formRef.enableAlert"/>
            </n-form-item>
            <n-form-item label="通知渠道">
              <n-select v-model:value="formRef.notifyChannels" :options="notifyChannelOptions" multiple
                        style="width:340px" placeholder="不选则全部渠道"/>
            </n-form-item>
          </n-space>
          <n-text depth="3" style="font-size:12px">
            开启后，盘中每分钟监控实时价，达到情景触发区间/止损价/目标价时按所选渠道推送通知（软件内提醒、飞书、钉钉）
          </n-text>
        </n-card>

        <!-- 情景方案 -->
        <n-card size="small" title="情景方案" style="margin-bottom:12px" :bordered="false">
          <n-card v-for="(sc, idx) in formRef.scenarios" :key="idx" size="small" :bordered="true"
                  style="margin-bottom:10px">
            <n-space align="center" style="margin-bottom:8px">
              <n-tag :type="actionTypeTagType[sc.actionType]" size="small">情景{{ idx + 1 }}</n-tag>
              <n-input v-model:value="sc.title" placeholder="情景标题，如：低开在 460-475 区间" style="flex:1"/>
              <n-select v-model:value="sc.actionType" :options="actionTypeOptions" style="width:100px"/>
              <n-checkbox v-model:checked="sc.isBest">最理想</n-checkbox>
              <n-button size="tiny" type="error" quaternary @click="removeScenario(idx)">删除</n-button>
            </n-space>
            <n-grid :cols="4" :x-gap="12" :y-gap="2" style="margin-bottom:8px">
              <n-gi>
                <n-form-item label="触发条件" label-placement="top">
                  <n-input v-model:value="sc.condition" placeholder="如 低开在460-475"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="动作" label-placement="top">
                  <n-input v-model:value="sc.action" placeholder="如 分批买入"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="仓位" label-placement="top">
                  <n-input v-model:value="sc.position" placeholder="如 1/3"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="买入区间" label-placement="top">
                  <n-input v-model:value="sc.buyPriceRange" placeholder="如 460-470" @blur="autoFillNumeric(sc)"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="止损价" label-placement="top">
                  <n-input v-model:value="sc.stopLossPrice" placeholder="如 400" @blur="autoFillNumeric(sc)"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="第一目标" label-placement="top">
                  <n-input v-model:value="sc.target1" placeholder="如 500-505" @blur="autoFillNumeric(sc)"/>
                </n-form-item>
              </n-gi>
              <n-gi>
                <n-form-item label="第二目标" label-placement="top">
                  <n-input v-model:value="sc.target2" placeholder="如 550-560" @blur="autoFillNumeric(sc)"/>
                </n-form-item>
              </n-gi>
            </n-grid>
            <n-collapse>
              <n-collapse-item title="📊 量化监控条件（盘中预警用，填写上方价格后可自动填充）" name="quant">
                <n-grid :cols="4" :x-gap="12" :y-gap="2">
                  <n-gi>
                    <n-form-item label="触发价下限" label-placement="left">
                      <n-input-number v-model:value="sc.triggerPriceMin" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 460"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="触发价上限" label-placement="left">
                      <n-input-number v-model:value="sc.triggerPriceMax" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 475"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="止损价" label-placement="left">
                      <n-input-number v-model:value="sc.stopLossPriceNum" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 400"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="目标1下限" label-placement="left">
                      <n-input-number v-model:value="sc.target1Min" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 500"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="目标1上限" label-placement="left">
                      <n-input-number v-model:value="sc.target1Max" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 505"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="目标2下限" label-placement="left">
                      <n-input-number v-model:value="sc.target2Min" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 550"/>
                    </n-form-item>
                  </n-gi>
                  <n-gi>
                    <n-form-item label="目标2上限" label-placement="left">
                      <n-input-number v-model:value="sc.target2Max" :precision="2" :step="0.01"
                                      :show-button="false" placeholder="如 560"/>
                    </n-form-item>
                  </n-gi>
                </n-grid>
              </n-collapse-item>
            </n-collapse>
            <n-form-item label="策略说明" style="margin-top:4px">
              <n-input v-model:value="sc.strategy" type="textarea" :rows="2"
                       placeholder="该情景下的具体策略/备注"/>
            </n-form-item>
          </n-card>
          <n-button size="small" dashed block @click="addScenario">+ 添加情景</n-button>
        </n-card>

        <!-- 操作纪律 -->
        <n-card size="small" title="操作纪律" style="margin-bottom:12px" :bordered="false">
          <div v-for="(d, idx) in formRef.discipline" :key="idx" style="margin-bottom:8px">
            <n-space align="center">
              <n-tag size="small" type="info">{{ idx + 1 }}</n-tag>
              <n-input v-model:value="d.principle" placeholder="原则，如 仓位控制" style="width:160px"/>
              <n-input v-model:value="d.detail" placeholder="说明，如 首次建仓不超过总计划资金的1/3，留足子弹补仓"
                       style="width:520px"/>
              <n-button size="tiny" type="error" quaternary @click="removeDiscipline(idx)">删除</n-button>
            </n-space>
          </div>
          <n-button size="small" dashed block @click="addDiscipline">+ 添加纪律</n-button>
        </n-card>

        <!-- 总结与风险 -->
        <n-card size="small" title="总结与风险提示" :bordered="false">
          <n-form-item label="一句话总结">
            <n-input v-model:value="formRef.summary" type="textarea" :rows="2"
                     placeholder="对整体操作方案的一句话总结"/>
          </n-form-item>
          <n-form-item label="风险提示">
            <n-input v-model:value="formRef.riskWarning" type="textarea" :rows="3" placeholder="风险提示"/>
          </n-form-item>
          <n-form-item label="备注">
            <n-input v-model:value="formRef.remarks" type="textarea" :rows="2" placeholder="其他备注"/>
          </n-form-item>
        </n-card>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showFormModal = false">取消</n-button>
          <n-button type="primary" :loading="formLoading" @click="handleSave">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 详情弹窗 -->
    <n-modal v-model:show="showDetailModal" preset="card" title="操作计划详情"
             style="width:760px;max-width:95vw;"
             :body-style="{ maxHeight: 'calc(90vh - 80px)', overflowY: 'auto' }">
      <template v-if="detailRef">
        <n-descriptions :column="4" size="small" bordered style="margin-bottom:16px">
          <n-descriptions-item label="状态">
            <n-tag :type="statusTagType[detailRef.status]" size="small">{{ statusText[detailRef.status] }}</n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="股票">
            <n-text strong>{{ detailRef.stockName }}</n-text>
            <n-text depth="3" style="margin-left:6px">{{ detailRef.stockCode }}</n-text>
          </n-descriptions-item>
          <n-descriptions-item label="计划日期">{{ detailRef.planDate }}{{ detailRef.planEndDate ? ' ~ ' + detailRef.planEndDate : '' }}</n-descriptions-item>
          <n-descriptions-item label="盘中预警">
            <n-tag :type="detailRef.enableAlert ? 'success' : 'default'" size="small">
              {{ detailRef.enableAlert ? '已开启' : '未开启' }}
            </n-tag>
          </n-descriptions-item>
        </n-descriptions>

        <template v-if="detailRef.overallJudgment">
          <n-divider title-placement="left" style="margin-top:0">🎯 总体判断</n-divider>
          <n-text style="white-space:pre-wrap">{{ detailRef.overallJudgment }}</n-text>
        </template>

        <template v-if="detailRef._scenarios && detailRef._scenarios.length">
          <n-divider title-placement="left">📋 具体操作方案</n-divider>
          <n-card v-for="(sc, idx) in detailRef._scenarios" :key="idx" size="small" style="margin-bottom:8px">
            <template #header>
              <n-space align="center">
                <n-tag :type="actionTypeTagType[sc.actionType]" size="small">{{ actionTypeText[sc.actionType] }}</n-tag>
                <n-text strong>{{ sc.title || '情景' + (idx + 1) }}</n-text>
                <n-tag v-if="sc.isBest" size="small" type="success">最理想</n-tag>
              </n-space>
            </template>
            <n-descriptions :column="2" size="small" label-placement="left">
              <n-descriptions-item v-if="sc.condition" label="触发条件">{{ sc.condition }}</n-descriptions-item>
              <n-descriptions-item v-if="sc.action" label="动作">{{ sc.action }}</n-descriptions-item>
              <n-descriptions-item v-if="sc.position" label="仓位">{{ sc.position }}</n-descriptions-item>
              <n-descriptions-item v-if="sc.buyPriceRange" label="买入区间">{{ sc.buyPriceRange }}</n-descriptions-item>
              <n-descriptions-item v-if="sc.stopLossPrice" label="止损价">
                <n-text type="error">{{ sc.stopLossPrice }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item v-if="sc.target1" label="第一目标">{{ sc.target1 }}</n-descriptions-item>
              <n-descriptions-item v-if="sc.target2" label="第二目标">{{ sc.target2 }}</n-descriptions-item>
            </n-descriptions>
            <n-text v-if="sc.strategy" depth="2" style="white-space:pre-wrap;margin-top:4px">{{ sc.strategy }}</n-text>
          </n-card>
        </template>

        <template v-if="detailRef._discipline && detailRef._discipline.length">
          <n-divider title-placement="left">📌 操作纪律</n-divider>
          <n-space vertical>
            <n-space v-for="(d, idx) in detailRef._discipline" :key="idx" align="baseline">
              <n-tag size="small" round>{{ idx + 1 }}</n-tag>
              <n-text><n-text strong>{{ d.principle }}</n-text>：{{ d.detail }}</n-text>
            </n-space>
          </n-space>
        </template>

        <template v-if="detailRef.summary">
          <n-divider title-placement="left">📝 一句话总结</n-divider>
          <n-text style="white-space:pre-wrap">{{ detailRef.summary }}</n-text>
        </template>

        <n-alert v-if="detailRef.riskWarning" type="warning" style="margin-top:12px">
          {{ detailRef.riskWarning }}
        </n-alert>
      </template>
    </n-modal>

  <!-- K线弹窗 -->
  <n-modal v-model:show="showKlineModal" preset="card" :title="'K线 - ' + klineStockName" style="width: 95vw; max-width: 1400px">
    <StockLightweightKlineChart
      :code="klineStockCode"
      :stock-name="klineStockName"
      :chart-height="500"
      :dark-theme="darkTheme"
    />
  </n-modal>

  <!-- 成交明细弹窗 -->
  <n-modal v-model:show="showTransactionModal" preset="card" :title="'成交明细 - ' + transactionStockName" style="width: 1200px; max-width: calc(100vw - 32px)" :content-style="{padding: '8px'}" @after-leave="handleTransactionModalClose">
    <div style="display:flex; flex-direction:column; gap:8px;">
      <!-- 分时图 -->
      <div ref="transactionChartRef" style="width: 100%; height: 200px;"></div>

      <!-- 各档位买卖统计 -->
      <div style="display:grid; grid-template-columns: repeat(4, 1fr); gap:8px;">
        <div v-for="stat in transactionStats" :key="stat.level"
             :style="`border:1px solid ${amountTagType(stat.level) === 'default' ? '#dcdfe6' : (
                       amountTagType(stat.level) === 'error' ? '#d03050' :
                       amountTagType(stat.level) === 'warning' ? '#f0a020' :
                       amountTagType(stat.level) === 'info' ? '#2080f0' : '#dcdfe6'
                     )}33; border-radius:6px; padding:8px; font-size:12px;`">
          <div style="display:flex; align-items:center; gap:6px; margin-bottom:6px;">
            <n-tag :type="amountTagType(stat.level)" size="tiny" :bordered="false">{{ stat.name }}</n-tag>
            <n-text depth="3" style="font-size:11px;">共 {{ stat.buyCount + stat.sellCount + stat.neutralCount }} 笔</n-text>
          </div>
          <div style="display:flex; flex-direction:column; gap:2px; line-height:1.5;">
            <div style="display:flex; justify-content:space-between;">
              <span style="color:#d03050;">买 {{ stat.buyPercent.toFixed(1) }}%</span>
              <span style="color:#d03050;">{{ formatWan(stat.buyAmount) }}万</span>
            </div>
            <div style="display:flex; justify-content:space-between;">
              <span style="color:#18a058;">卖 {{ stat.sellPercent.toFixed(1) }}%</span>
              <span style="color:#18a058;">{{ formatWan(stat.sellAmount) }}万</span>
            </div>
            <div style="display:flex; justify-content:space-between;">
              <span style="color:#909399;">中性 {{ stat.neutralPercent.toFixed(1) }}%</span>
              <span style="color:#909399;">{{ formatWan(stat.neutralAmount) }}万</span>
            </div>
            <div style="border-top:1px dashed #dcdfe6; margin-top:4px; padding-top:4px; display:flex; justify-content:space-between; font-weight:bold;">
              <span>净流入</span>
              <span :style="`color:${stat.netInflow >= 0 ? '#d03050' : '#18a058'};`">
                {{ stat.netInflow >= 0 ? '+' : '' }}{{ formatWan(stat.netInflow) }}万
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 成交明细表格 -->
      <n-data-table
        :columns="transactionColumns"
        :data="transactionList"
        :loading="transactionLoading"
        :pagination="{ pageSize: 15 }"
        :max-height="320"
        size="small"
        striped
      />
    </div>
  </n-modal>
</template>

<style scoped>
</style>
