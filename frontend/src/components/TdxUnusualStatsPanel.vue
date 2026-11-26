<script setup>
// TdxUnusualStatsPanel.vue — 通达信主力异动统计看板（实时）。
// 数据来源：盘中「通达信异动通知」轮询落库的 tdx_unusual_events 表，
// 后端 GetTdxUnusualDashboard 汇总返回，页面每 60 秒自动刷新。
// 看板内容：今日概览 / 逐日趋势 / 异动类型分布 / 市场分布 / 活跃股排行 /
//           今日首次异动股票 / 分时热度。
import {computed, h, nextTick, onBeforeUnmount, onMounted, reactive, ref} from "vue";
import * as echarts from "echarts";
import {GetTdxUnusualDashboard, GetTdxUnusualEventList, GetTdxUnusualTypeOptions} from "../../wailsjs/go/main/App";
import {GetConfig, GetGroupList, Follow, AddStockGroup, AddGroup, GetTdxMinuteTimeData, GetAllTdxTransactionData} from "../../wailsjs/go/main/App";
import {NButton, NDropdown, NIcon, NTag, useMessage} from "naive-ui";
import {AddOutline, FolderOpenOutline} from "@vicons/ionicons5";
import KLineChart from "./KLineChart.vue";

const message = useMessage()

const REFRESH_MS = 60 * 1000
const days = ref(10)
const loading = ref(false)
const errorMsg = ref('')
const updatedAt = ref('')

const today = ref({total: 0, bull: 0, bear: 0, neutral: 0, activeStocks: 0, date: ''})
const hotStocks = ref([])
const newActive = ref([])
const topTypes = ref([])

let refreshTask = null
let trendChart = null
let typeChart = null
let heatChart = null

function bullBearLabel(trend) {
  if (trend === 1) return '偏多'
  if (trend === -1) return '偏空'
  return '中性'
}
function bullBearType(trend) {
  if (trend === 1) return 'error'
  if (trend === -1) return 'success'
  return 'default'
}
// 与后端口径一致的关键词分类：1=利好 -1=利空 0=中性
const bullKeywords = ['托盘', '买入', '买盘', '拉升', '火箭', '反弹', '封涨停', '涨停', '打开跌停', '回升', '高开', '向上缺口', '新高', '上涨', '抢筹', '净流入']
const bearKeywords = ['压盘', '卖出', '卖盘', '下跌', '跳水', '封跌停', '跌停', '打开涨停', '回落', '低开', '向下缺口', '新低', '砸盘', '净流出', '撤单']
function bullBearTrend(typeName) {
  const name = String(typeName || '').trim()
  if (!name) return 0
  if (bullKeywords.some(k => name.includes(k))) return 1
  if (bearKeywords.some(k => name.includes(k))) return -1
  return 0
}

function renderTrendChart(trend) {
  if (!trendChart) return
  trendChart.setOption({
    tooltip: {trigger: 'axis'},
    legend: {data: ['异动总数', '利好异动', '利空异动'], bottom: 0},
    grid: {left: 40, right: 16, top: 30, bottom: 40},
    xAxis: {type: 'category', data: trend.map(t => t.date)},
    yAxis: {type: 'value', minInterval: 1},
    series: [
      {name: '异动总数', type: 'bar', data: trend.map(t => t.total), barMaxWidth: 26, itemStyle: {color: '#549EC8'}},
      {name: '利好异动', type: 'line', data: trend.map(t => t.bull), smooth: true, itemStyle: {color: '#ef4444'}},
      {name: '利空异动', type: 'line', data: trend.map(t => t.bear), smooth: true, itemStyle: {color: '#22c55e'}},
    ],
  }, true)
}

function renderTypeChart(topTypes) {
  if (!typeChart) return
  // 偏多用红，偏空用绿，中性用蓝（与 A 股红涨绿跌习惯一致）
  const colorOf = (trend) => trend === 1 ? '#ef4444' : trend === -1 ? '#22c55e' : '#549EC8'
  typeChart.setOption({
    tooltip: {trigger: 'item', formatter: '{b}: {c} 次（{d}%）'},
    legend: {type: 'scroll', bottom: 0, textStyle: {fontSize: 11}},
    series: [{
      type: 'pie',
      radius: ['35%', '65%'],
      center: ['50%', '45%'],
      data: topTypes.map(t => ({name: t.typeName, value: t.count, itemStyle: {color: colorOf(t.trend)}})),
      label: {fontSize: 11, formatter: '{b} {c}'},
    }],
  }, true)
}

function renderHeatChart(timeHeat) {
  if (!heatChart) return
  heatChart.setOption({
    tooltip: {trigger: 'axis'},
    grid: {left: 40, right: 16, top: 20, bottom: 30},
    xAxis: {type: 'category', data: timeHeat.map(t => t.slot)},
    yAxis: {type: 'value', minInterval: 1},
    series: [{
      name: '异动次数', type: 'bar', barMaxWidth: 22,
      data: timeHeat.map(t => t.count),
      itemStyle: {color: '#F98C24'},
    }],
  }, true)
}

async function fetchData() {
  try {
    loading.value = true
    errorMsg.value = ''
    const d = await GetTdxUnusualDashboard(days.value, 30)
    if (!d) {
      errorMsg.value = '暂无统计数据'
      return
    }
    updatedAt.value = d.updatedAt || ''
    today.value = d.today || {total: 0, bull: 0, bear: 0, neutral: 0, activeStocks: 0}
    hotStocks.value = d.hotStocks || []
    newActive.value = d.newActive || []
    topTypes.value = d.topTypes || []
    renderTrendChart(d.trend || [])
    renderTypeChart(topTypes.value)
    renderHeatChart(d.timeHeat || [])
    if (!d.trend || d.trend.length === 0) {
      errorMsg.value = '暂无异动数据：需在交易时段开启「设置 → 通达信异动通知」进行采集'
    }
  } catch (e) {
    console.error('GetTdxUnusualDashboard error:', e)
    errorMsg.value = '加载失败: ' + (e?.message || e || '未知错误')
  } finally {
    loading.value = false
  }
}

function onDaysChange(v) {
  days.value = v
  fetchData()
}

onMounted(() => {
  trendChart = echarts.init(document.getElementById('tdx-trend-chart'))
  typeChart = echarts.init(document.getElementById('tdx-type-chart'))
  heatChart = echarts.init(document.getElementById('tdx-heat-chart'))
  fetchData()
  fetchEvents()
  loadTypeOptions()
  loadGroupList()
  GetConfig().then(r => { darkTheme.value = !!r.darkTheme }).catch(() => {})
  refreshTask = setInterval(fetchData, REFRESH_MS)
})

onBeforeUnmount(() => {
  if (refreshTask) clearInterval(refreshTask)
  trendChart?.dispose()
  typeChart?.dispose()
  heatChart?.dispose()
})

const hotColumns = [
  {title: '#', key: 'rank', width: 50, render: (row, idx) => idx + 1},
  {title: '代码', key: 'stockCode', width: 80},
  {title: '名称', key: 'stockName', width: 110, ellipsis: {tooltip: true}},
  {title: '异动次数', key: 'count', width: 90, sorter: (a, b) => a.count - b.count},
  {title: '连续天数', key: 'streak', width: 90, sorter: (a, b) => a.streak - b.streak,
    render: (row) => row.streak >= 3
      ? h('span', {style: {color: '#ef4444', fontWeight: '600'}}, row.streak + ' 天')
      : row.streak + ' 天'},
  {title: '覆盖天数', key: 'days', width: 90},
  {title: '多空倾向', key: 'trend', width: 90,
    render: (row) => h('span', {style: {color: row.trend === 1 ? '#ef4444' : row.trend === -1 ? '#22c55e' : ''}}, bullBearLabel(row.trend))},
  {title: '主要异动类型', key: 'mainTypes', ellipsis: {tooltip: true}},
  {title: '最近异动', key: 'lastSeen', width: 160},
]

const newColumns = [
  {title: '代码', key: 'stockCode', width: 80},
  {title: '名称', key: 'stockName', width: 110, ellipsis: {tooltip: true}},
  {title: '今日次数', key: 'count', width: 90, sorter: (a, b) => a.count - b.count},
  {title: '首次异动', key: 'firstSeen', width: 160},
  {title: '异动类型', key: 'types', ellipsis: {tooltip: true}},
]

const typeColumns = [
  {title: '异动类型', key: 'typeName', ellipsis: {tooltip: true}},
  {title: '次数', key: 'count', width: 80, sorter: (a, b) => a.count - b.count},
  {title: '股票数', key: 'stocks', width: 80},
  {title: '倾向', key: 'trend', width: 80,
    render: (row) => h('span', {style: {color: row.trend === 1 ? '#ef4444' : row.trend === -1 ? '#22c55e' : ''}}, bullBearLabel(row.trend))},
]

// ===== 异动明细（服务端分页 + 多条件查询） =====
function todayRange() {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  return [start.getTime(), start.getTime() + 86399999]
}

const eventQuery = reactive({
  stockCode: '',
  stockName: '',
  typeNames: [], // 多选异动类型
  market: '',
  dateRange: todayRange(), // 默认查今天
})
const eventLoading = ref(false)
const eventList = ref([])
const eventTotal = ref(0)
const typeOptions = ref([]) // 异动类型下拉选项
const eventPagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [20, 50, 100, 200],
  onChange: (page) => {
    eventPagination.page = page
    fetchEvents()
  },
  onUpdatePageSize: (pageSize) => {
    eventPagination.pageSize = pageSize
    eventPagination.page = 1
    fetchEvents()
  },
})

const marketOptions = [
  {label: '全部市场', value: ''},
  {label: '沪市', value: 'sh'},
  {label: '深市', value: 'sz'},
  {label: '北证', value: 'bj'},
]
const typeSelectOptions = computed(() => typeOptions.value.map(t => ({label: t, value: t})))

const eventColumns = [
  {title: '日期', key: 'changeDate', width: 100},
  {title: '时间', key: 'changeTime', width: 90},
  {title: '市场', key: 'market', width: 70,
    render: (row) => h(NTag, {size: 'tiny', bordered: false,
      type: row.market === 1 ? 'error' : row.market === 2 ? 'warning' : 'info'},
      {default: () => row.market === 1 ? '沪' : row.market === 2 ? '北' : '深'})},
  {title: '代码', key: 'stockCode', width: 90},
  {title: '名称', key: 'stockName', width: 120, ellipsis: {tooltip: true}},
  {title: '异动类型', key: 'typeName', width: 130, ellipsis: {tooltip: true},
    render: (row) => {
      const trend = bullBearTrend(row.typeName)
      return h('span', {style: {color: trend === 1 ? '#ef4444' : trend === -1 ? '#22c55e' : ''}}, row.typeName)
    }},
  {title: '数值', key: 'value', ellipsis: {tooltip: true}},
  {
    title: '操作', width: 200,
    render(row) {
      return [
        h(NButton, {secondary: true, size: 'tiny', type: 'error', style: 'margin-right:4px;',
          onClick: () => showKline(row)}, {default: () => '日K'}),
        h(NButton, {secondary: true, size: 'tiny', type: 'info', style: 'margin-right:4px;',
          onClick: () => showTransactionDetail(row)}, {default: () => '成交明细'}),
        h(NDropdown, {
          trigger: 'click',
          options: followGroupOptions.value,
          placement: 'bottom-end',
          menuProps: () => ({style: 'max-height:300px; overflow-y:auto;'}),
          onSelect: (key) => handleFollowSelect(key, row)
        }, {
          default: () => h(NButton, {tertiary: true, size: 'tiny', type: 'success'},
            {default: () => '关注', icon: () => h(NIcon, null, {default: () => h(FolderOpenOutline, {size: 12})})})
        })
      ]
    }
  },
]

async function fetchEvents() {
  try {
    eventLoading.value = true
    const range = eventQuery.dateRange || []
    const res = await GetTdxUnusualEventList({
      page: eventPagination.page,
      pageSize: eventPagination.pageSize,
      stockCode: eventQuery.stockCode || '',
      stockName: eventQuery.stockName || '',
      typeNames: eventQuery.typeNames || [],
      market: eventQuery.market || '',
      startDate: range[0] ? formatDateTime(range[0], '00:00:00') : '',
      endDate: range[1] ? formatDateTime(range[1], '23:59:59') : '',
    })
    eventList.value = res?.list || []
    eventTotal.value = res?.total || 0
    eventPagination.itemCount = res?.total || 0
    eventPagination.page = res?.page || eventPagination.page
  } catch (e) {
    console.error('GetTdxUnusualEventList error:', e)
  } finally {
    eventLoading.value = false
  }
}

function formatDateTime(ts, fallbackTime) {
  const d = new Date(ts)
  const pad = (n) => String(n).padStart(2, '0')
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${time === '00:00:00' && fallbackTime ? fallbackTime : time}`
}

async function loadTypeOptions() {
  try {
    typeOptions.value = await GetTdxUnusualTypeOptions() || []
  } catch (e) {
    console.error('GetTdxUnusualTypeOptions error:', e)
  }
}

function searchEvents() {
  eventPagination.page = 1
  fetchEvents()
}

function resetEvents() {
  eventQuery.stockCode = ''
  eventQuery.stockName = ''
  eventQuery.typeNames = []
  eventQuery.market = ''
  eventQuery.dateRange = todayRange()
  eventPagination.page = 1
  fetchEvents()
}

// ===== 操作列：日K / 成交明细 / 关注 =====
// 行内裸代码（如 600000 / 000001 / 830799）→ 带市场前缀小写代码（sh600000 / sz000001 / bj830799）
function toApiCode(row) {
  const code = String(row.stockCode || '').trim()
  if (!code) return ''
  const prefix = row.market === 1 ? 'sh' : row.market === 2 ? 'bj' : 'sz'
  return prefix + code
}

// 日K弹窗
const darkTheme = ref(false)
const klineModal = reactive({visible: false, title: '', stockCode: '', stockName: ''})
function showKline(row) {
  const code = toApiCode(row)
  if (!code) {
    message.warning('当前行缺少股票代码')
    return
  }
  klineModal.title = row.stockName || code
  klineModal.stockCode = code
  klineModal.stockName = row.stockName || code
  klineModal.visible = true
}

// 关注分组（参考自选/形态选股）
const groupList = ref([])
const showFollowGroupModal = ref(false)
const newGroupName = ref('')
const pendingFollowRow = ref(null)
const followGroupOptions = computed(() => {
  const opts = [{label: '默认（不分组）', key: 0}]
  groupList.value.forEach(g => opts.push({label: g.name, key: g.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建分组', key: 'new', icon: () => h(NIcon, null, {default: () => h(AddOutline)})})
  return opts
})
function loadGroupList() {
  GetGroupList().then(res => {
    groupList.value = res || []
  }).catch(() => {})
}
function groupNameById(id) {
  const g = groupList.value.find(item => item.ID === id)
  return g ? g.name : ''
}
function handleFollowSelect(key, row) {
  if (key === 'new') {
    pendingFollowRow.value = row
    newGroupName.value = ''
    showFollowGroupModal.value = true
    return
  }
  doFollow(row, Number(key))
}
function doFollow(row, groupId) {
  const code = toApiCode(row)
  Follow(code).then(result => {
    if (result === '关注成功') {
      message.success(groupId > 0 ? `已关注，并加入分组「${groupNameById(groupId)}」` : '关注成功')
    } else {
      message.info(result)
    }
    if (groupId > 0) {
      AddStockGroup(groupId, code).then(() => loadGroupList()).catch(() => {})
    }
  }).catch(err => {
    message.error('关注失败: ' + err)
  })
}
function handleCreateGroupAndFollow() {
  const name = newGroupName.value.trim()
  if (!name) {
    message.warning('请输入分组名称')
    return
  }
  const maxSort = groupList.value.reduce((max, g) => Math.max(max, g.sort || 0), 0)
  AddGroup({name, sort: maxSort + 1}).then(res => {
    if (res === '添加成功') {
      GetGroupList().then(list => {
        groupList.value = list || []
        showFollowGroupModal.value = false
        const created = groupList.value.find(g => g.name === name)
        if (created && pendingFollowRow.value) {
          doFollow(pendingFollowRow.value, created.ID)
        }
        pendingFollowRow.value = null
      })
    } else {
      message.error(res)
    }
  }).catch(err => {
    message.error('新建分组失败: ' + err)
  })
}

// ===== 成交明细弹窗（参考每日操作计划，分时图 + 明细表 + 大小单统计） =====
const SUPER_LARGE = 1000000   // 100万
const LARGE_AMOUNT = 200000   // 20万
const MEDIUM_AMOUNT = 40000   // 4万
const SHARE_PER_LOT = 100
const showTransactionModal = ref(false)
const transactionStockCode = ref('')
const transactionStockName = ref('')
const transactionLoading = ref(false)
const transactionList = ref([])
const transactionStats = ref([])
const minuteBundle = ref(null)
let transactionChart = null
const transactionChartRef = ref(null)

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

function calcTransactionStats() {
  const stats = [
    {level: 1, name: '超大单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0},
    {level: 2, name: '大单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0},
    {level: 3, name: '中单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0},
    {level: 4, name: '小单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0}
  ]
  for (const row of transactionList.value) {
    const amount = transactionAmount(row)
    const s = stats[classifyAmount(amount) - 1]
    if (row.buyOrSell === 0) { s.buyCount++; s.buyAmount += amount }
    else if (row.buyOrSell === 1) { s.sellCount++; s.sellAmount += amount }
    else { s.neutralCount++; s.neutralAmount += amount }
  }
  transactionStats.value = stats
}

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

const transactionPagination = reactive({
  page: 1,
  pageSize: 100,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [100, 200, 500, 1000],
  onChange: (page) => { transactionPagination.page = page },
  onUpdatePageSize: (pageSize) => {
    transactionPagination.pageSize = pageSize
    transactionPagination.page = 1
  },
})

// 明细表前端分页数据切片
const pagedTransactionList = computed(() => {
  const start = (transactionPagination.page - 1) * transactionPagination.pageSize
  return transactionList.value.slice(start, start + transactionPagination.pageSize)
})

// 大小单统计表列
const transactionStatsColumns = [
  {title: '档位', key: 'name', width: 80},
  {title: '买量', key: 'buyCount', align: 'right', render: (r) => h('span', {style: {color: '#ef4444'}}, String(r.buyCount))},
  {title: '卖量', key: 'sellCount', align: 'right', render: (r) => h('span', {style: {color: '#22c55e'}}, String(r.sellCount))},
  {title: '中性量', key: 'neutralCount', align: 'right'},
  {title: '买入额(万)', align: 'right', render: (r) => (r.buyAmount / 10000).toFixed(2)},
  {title: '卖出额(万)', align: 'right', render: (r) => (r.sellAmount / 10000).toFixed(2)},
  {title: '净流入(万)', align: 'right', render: (r) => {
    const net = r.buyAmount - r.sellAmount
    return h('span', {style: {color: net >= 0 ? '#ef4444' : '#22c55e'}}, (net / 10000).toFixed(2))
  }},
]

async function showTransactionDetail(row) {
  const code = toApiCode(row)
  if (!code) {
    message.warning('当前行缺少股票代码')
    return
  }
  transactionStockCode.value = code
  transactionStockName.value = row.stockName || code
  transactionList.value = []
  transactionStats.value = []
  minuteBundle.value = null
  transactionLoading.value = true
  showTransactionModal.value = true
  try {
    const [bundle, transactions] = await Promise.all([
      GetTdxMinuteTimeData(code),
      GetAllTdxTransactionData(code)
    ])
    if (bundle && bundle.items && bundle.items.length > 0) {
      minuteBundle.value = bundle
      nextTick(() => renderTransactionChart())
    }
    if (transactions && Array.isArray(transactions)) {
      transactionList.value = transactions
      transactionPagination.itemCount = transactions.length
      transactionPagination.page = 1
      calcTransactionStats()
    }
  } catch (e) {
    message.error('成交明细加载失败：' + (e?.message || e))
  } finally {
    transactionLoading.value = false
  }
}

function handleTransactionModalClose() {
  if (transactionChart) {
    transactionChart.dispose()
    transactionChart = null
  }
  minuteBundle.value = null
  transactionList.value = []
  transactionStats.value = []
}

function renderTransactionChart() {
  const bundle = minuteBundle.value
  if (!bundle || !bundle.items || bundle.items.length === 0 || !transactionChartRef.value) return
  if (transactionChart) {
    transactionChart.dispose()
    transactionChart = null
  }
  const chart = echarts.init(transactionChartRef.value)
  transactionChart = chart

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
</script>

<template>
  <div class="tdx-unusual-stats" style="--wails-draggable:no-drag">
    <n-flex align="center" :size="10" style="margin-bottom: 12px">
      <n-text strong style="font-size: 16px">通达信主力异动统计看板</n-text>
      <n-select :value="days" @update:value="onDaysChange" size="small" style="width: 120px"
                :options="[
                  {label: '近5日', value: 5},
                  {label: '近10日', value: 10},
                  {label: '近20日', value: 20},
                  {label: '近30日', value: 30},
                ]"/>
      <n-button size="small" @click="fetchData">刷新</n-button>
      <n-text depth="3" style="font-size: 12px">每 60 秒自动刷新 · 更新于 {{ updatedAt }}</n-text>
    </n-flex>

    <n-alert v-if="errorMsg" type="warning" :bordered="false" style="margin-bottom: 12px">{{ errorMsg }}</n-alert>

    <n-spin :show="loading" size="small">
      <!-- 今日概览 -->
      <n-grid :cols="5" :x-gap="12" :y-gap="12" style="margin-bottom: 12px">
        <n-grid-item>
          <n-card size="small">
            <n-statistic label="今日异动总数" :value="today.total"/>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small">
            <n-statistic label="利好异动">
              <template #default><span style="color:#ef4444">{{ today.bull }}</span></template>
            </n-statistic>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small">
            <n-statistic label="利空异动">
              <template #default><span style="color:#22c55e">{{ today.bear }}</span></template>
            </n-statistic>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small">
            <n-statistic label="中性异动" :value="today.neutral"/>
          </n-card>
        </n-grid-item>
        <n-grid-item>
          <n-card size="small">
            <n-statistic label="活跃股票数" :value="today.activeStocks"/>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- 异动明细：服务端分页 + 多条件查询（置顶便于查询） -->
      <n-card size="small" :title="'异动明细（共 ' + eventTotal + ' 条）'" style="margin-bottom: 12px">
        <n-flex align="center" :size="8" style="margin-bottom: 10px" :wrap="true">
          <n-input v-model:value="eventQuery.stockCode" placeholder="股票代码" size="small" clearable style="width: 130px" @keyup.enter="searchEvents"/>
          <n-input v-model:value="eventQuery.stockName" placeholder="股票名称" size="small" clearable style="width: 130px" @keyup.enter="searchEvents"/>
          <n-select v-model:value="eventQuery.typeNames" :options="typeSelectOptions" placeholder="异动类型" size="small"
                    multiple clearable filterable :max-tag-count="2" style="width: 220px" @update:value="searchEvents"/>
          <n-select v-model:value="eventQuery.market" :options="marketOptions" size="small" style="width: 110px" @update:value="searchEvents"/>
          <n-date-picker v-model:value="eventQuery.dateRange" type="datetimerange" size="small" clearable style="width: 360px"/>
          <n-button size="small" type="primary" @click="searchEvents">查询</n-button>
          <n-button size="small" @click="resetEvents">重置</n-button>
        </n-flex>
        <n-data-table
          remote
          :columns="eventColumns"
          :data="eventList"
          :loading="eventLoading"
          :pagination="eventPagination"
          size="small"
          :bordered="false"
          :single-line="false"
          striped
          :max-height="380"
        />
      </n-card>

      <!-- 趋势 + 类型分布 -->
      <n-grid :cols="2" :x-gap="12" :y-gap="12" responsive="screen" item-responsive style="margin-bottom: 12px">
        <n-grid-item span="2 m:1">
          <n-card size="small" title="逐日异动趋势（近 N 日）">
            <div id="tdx-trend-chart" style="height: 280px"></div>
          </n-card>
        </n-grid-item>
        <n-grid-item span="2 m:1">
          <n-card size="small" title="异动类型分布（近 N 日）">
            <div id="tdx-type-chart" style="height: 280px"></div>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- 分时热度 + 类型明细表 -->
      <n-grid :cols="2" :x-gap="12" :y-gap="12" responsive="screen" item-responsive style="margin-bottom: 12px">
        <n-grid-item span="2 m:1">
          <n-card size="small" title="今日分时热度（30分钟粒度）">
            <div id="tdx-heat-chart" style="height: 280px"></div>
          </n-card>
        </n-grid-item>
        <n-grid-item span="2 m:1">
          <n-card size="small" title="异动类型明细（近 N 日 Top20）">
            <n-data-table :columns="typeColumns" :data="topTypes" size="small" :bordered="false"
                          :single-line="false" striped :max-height="300"/>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- 活跃股 + 首次异动 -->
      <n-grid :cols="2" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
        <n-grid-item span="2 m:1">
          <n-card size="small" title="活跃股排行（近 N 日异动次数 Top，潜力股线索）">
            <n-data-table :columns="hotColumns" :data="hotStocks" size="small" :bordered="false"
                          :single-line="false" striped :max-height="360"/>
          </n-card>
        </n-grid-item>
        <n-grid-item span="2 m:1">
          <n-card size="small" title="今日首次异动股票（潜在新题材 / 新资金关注）">
            <n-data-table :columns="newColumns" :data="newActive" size="small" :bordered="false"
                          :single-line="false" striped :max-height="360"/>
          </n-card>
        </n-grid-item>
      </n-grid>
    </n-spin>

    <!-- 日K弹窗 -->
    <n-modal v-model:show="klineModal.visible" :title="klineModal.title + ' — 日K'" preset="card"
             style="width: 850px; max-width: calc(100vw - 32px);">
      <n-card size="small">
        <KLineChart v-if="klineModal.visible && klineModal.stockCode" style="width: 100%; max-width: 800px;"
                    :code="klineModal.stockCode" :chart-height="500" :stock-name="klineModal.stockName"
                    :k-days="30" :dark-theme="darkTheme"/>
      </n-card>
    </n-modal>

    <!-- 成交明细弹窗：分时图 + 大小单统计 + 明细表 -->
    <n-modal v-model:show="showTransactionModal" preset="card"
             :title="'成交明细 - ' + transactionStockName" style="width: 1200px; max-width: calc(100vw - 32px)"
             :content-style="{padding: '8px'}" @after-leave="handleTransactionModalClose">
      <n-spin :show="transactionLoading">
        <div ref="transactionChartRef" style="height: 260px"></div>
        <n-data-table :columns="transactionStatsColumns" :data="transactionStats" size="small"
                      :bordered="false" :single-line="false" striped style="margin-top: 8px"/>
        <n-data-table :columns="transactionColumns" :data="pagedTransactionList" remote
                      :pagination="transactionPagination" size="small" :bordered="false"
                      :single-line="false" striped :max-height="320" style="margin-top: 8px"/>
      </n-spin>
    </n-modal>

    <!-- 新建关注分组弹窗 -->
    <n-modal v-model:show="showFollowGroupModal" preset="dialog" title="新建分组" positive-text="创建并关注"
             negative-text="取消" @positive-click="handleCreateGroupAndFollow" style="width: 420px;">
      <n-form label-placement="left" label-width="80">
        <n-form-item label="分组名称">
          <n-input v-model:value="newGroupName" placeholder="请输入分组名称" @keyup.enter="handleCreateGroupAndFollow"/>
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<style scoped>
.tdx-unusual-stats {
  height: 100%;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
}
#tdx-trend-chart, #tdx-type-chart, #tdx-heat-chart {
  width: 100%;
}
</style>
