<script setup>
import {computed, h, nextTick, onBeforeMount, onBeforeUnmount, onMounted, reactive, ref, watch} from 'vue'
import * as echarts from 'echarts';
import {
  AddConcept,
  AddGroup,
  AddStockConcept,
  AddStockGroup,
  Follow,
  GetAiConfigs,
  GetAIResponseResult,
  GetAllGroupStocks,
  GetAllStockConcepts,
  GetConceptList,
  GetConfig,
  GetEffectiveSponsorVip,
  GetFollowList,
  GetGroupList,
  GetPromptTemplates,
  GetStockConceptsByStockCode,
  GetStockKLine,
  GetStockList,
  GetStockMinutePriceLineData,
  GetTdxMinuteTimeData,
  GetHistoryTdxMinuteTimeData,
  GetTdxTransactionData,
  GetAllTdxTransactionData,
  GetHistoryTdxTransactionData,
  RefreshAllTdxTransactionData,
  RefreshHistoryTdxTransactionData,
  GetLatestTradingDay,
  IsTradingDay,
  GetStockRealTimePrice,
  GetVersionInfo,
  Greet,
  InitializeGroupSort,
  NewChatStream,
  OpenURL,
  RemoveConcept,
  RemoveGroup,
  RemoveStockConcept,
  RemoveStockGroup,
  RestartAsAdmin,
  SaveAIResponseResult,
  SaveAsMarkdown,
  SaveImage,
  SaveWordFile,
  SendDingDingMessageByType,
  SetAlarmChangePercent,
  SetCostPriceAndVolume,
  SetStockAICron,
  SetStockSort,
  SetTradingPrice,
  ShareAnalysis,
  UnFollow,
  UpdateConcept,
  UpdateGroup,
  UpdateGroupSort
} from '../../wailsjs/go/main/App'
import {
  NAvatar,
  NButton,
  NButtonGroup,
  NDataTable,
  NDatePicker,
  NDropdown,
  NFlex,
  NForm,
  NFormItem,
  NInputNumber,
  NSelect,
  NTag,
  NText,
  useDialog,
  useMessage,
  useNotification
} from 'naive-ui'
import {
  Environment,
  EventsEmit,
  EventsOff,
  EventsOn,
  WindowFullscreen,
  WindowReload,
  WindowUnfullscreen
} from '../../wailsjs/runtime'
import {Add, ChatboxOutline, CreateOutline} from '@vicons/ionicons5'
import {MdEditor, MdPreview} from 'md-editor-v3';
// preview.css相比style.css少了编辑器那部分样式
//import 'md-editor-v3/lib/preview.css';
import 'md-editor-v3/lib/style.css';

import {ExportPDF} from '@vavt/v3-extension';
import '@vavt/v3-extension/lib/asset/ExportPDF.css';
import html2canvas from "html2canvas";
import {asBlob} from 'html-docx-js-typescript';

import vueDanmaku from 'vue3-danmaku'
import {keys, padStart} from "lodash";
import {useRoute, useRouter} from 'vue-router'
import MoneyTrend from "./moneyTrend.vue";
import StockSparkLine from "./stockSparkLine.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

const route = useRoute()
const router = useRouter()

const danmus = ref([])
const ws = ref(null)
const dialog = useDialog()
const toolbars = [0];

const upColor = '#ec0000';
const upBorderColor = '';
const downColor = '#00da3c';
const downBorderColor = '';
const kLineChartRef = ref(null);
const kLineChartRef2 = ref(null);


const handleProgress = (progress) => {
  //console.log(`Export progress: ${progress.ratio * 100}%`);
};
const enableEditor = ref(false)
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const aiResultScrollRef = ref(null)
const tipsRef = ref(null)
const message = useMessage()
const notify = useNotification()
const stocks = ref([])
const results = ref({})
const stockList = ref([])
const followList = ref([])
const groupList = ref([])
// 股票代码 -> 所属分组名数组（用于「全部」标签页表格的分组列渲染）
const codeToGroupNames = ref(new Map())
// 股票代码 -> 所属分组 ID 数组（用于「全部」标签页表格的分组条件筛选，按 ID 匹配避免重名）
const codeToGroupIds = ref(new Map())
const conceptList = ref([])
// 股票代码 -> 所属概念名数组（用于「全部」标签页表格的概念列渲染）
const codeToConceptNames = ref(new Map())
// 股票代码 -> 所属概念 ID 数组（用于「全部」标签页表格的概念条件筛选与下拉勾选判断）
const codeToConceptIds = ref(new Map())
// 概念筛选：0 表示不按概念筛选
const tableConceptFilter = ref(0)
// 「设置概念」时新建概念后待加入的股票（null 表示非设置概念流程打开的概念弹窗）
const pendingAddStockConcept = ref(null)
const options = ref([])
const modalShow = ref(false)
const modalShow2 = ref(false)
const modalShow3 = ref(false)
const modalShow4 = ref(false)
const modalShow5 = ref(false)
const modalShow6 = ref(false)
const modalShow7 = ref(false)
const lwKlineCode = ref('')
const lwKlineName = ref('')
// gotdx 分时明细弹窗状态
const tdxMinuteBundle = ref(null)  // TdxMinuteTimeDataBundle
const tdxMinuteBundleList = ref([]) // 多日模式：[{ dateStr, bundle }]
const tdxTransactionList = ref([]) // []TdxTransactionData
const tdxTransactionLoading = ref(false)
const tdxTransactionChartRef = ref(null)
const tdxTransactionChart = ref(null)
// 实时价格与涨跌幅（弹窗打开时轮询刷新）
const tdxRealTimeInfo = ref({ price: 0, preClose: 0, changePercent: 0 })
// 自动刷新开关与定时器（仅交易日盘中 + 选中范围含今天时生效）
const tdxAutoRefresh = ref(true)
let tdxAutoRefreshTimer = null
// 大单过滤（按成交金额 = 价格 × 成交量 分档，参考东方财富标准）
// 0=全部 1=超大单(≥100万) 2=大单(20-100万) 3=中单(4-20万) 4=小单(<4万)
const tdxAmountFilter = ref(0)
// 日期范围选择：默认今天 [start, end]（时间戳，单位毫秒，NDatePicker daterange 要求）
// 单日选择时 start === end（按天对齐），多日选择时为闭区间 [start, end]
const tdxSelectedDateRange = ref([startOfTodayTs(), startOfTodayTs()])
// 禁选未来日期
const tdxDateDisabled = (ts) => ts > Date.now()
// 将时间戳对齐到当日 00:00:00（避免 daterange 默认携带 12:00 导致 isToday 判断偏差）
function startOfTodayTs() {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}
function startOfDayTs(ts) {
  const d = new Date(ts)
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}
// 格式化日期为 "YYYY-MM-DD"
function formatTdxDate(ts) {
  const d = new Date(ts)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
function isToday(ts) {
  return formatTdxDate(ts) === formatTdxDate(Date.now())
}
// 今天是否为交易日（通过后端 IsTradingDay 接口查询，使用 timor.tech 节假日 API 准确判断）
// showTransactionDetail 时初始化，供 shouldUseCurrentDayApi 同步使用
const todayIsTradingDay = ref(true)
async function refreshTodayTradingDayStatus() {
  try {
    const todayStr = formatTdxDate(Date.now())
    todayIsTradingDay.value = await IsTradingDay(todayStr)
  } catch {
    // API 失败时 fallback 到周末判断
    const d = new Date()
    const day = d.getDay()
    todayIsTradingDay.value = day !== 0 && day !== 6
  }
}
// 是否应走当日实时接口：仅当为今天且今天为交易日时才走当日接口，
// 否则走历史接口（非交易日返回空，避免拿到上一交易日数据标记为今天）
function shouldUseCurrentDayApi(ts) {
  return isToday(ts) && todayIsTradingDay.value
}
// 判断当前选中的日期范围是否跨多天（按日粒度比较 start 与 end）
const tdxIsMultiDay = computed(() => {
  const [s, e] = tdxSelectedDateRange.value || []
  if (s == null || e == null) return false
  return formatTdxDate(s) !== formatTdxDate(e)
})
// 枚举 [startTs, endTs] 闭区间内的所有日期（按日粒度），返回 [{ ts, dateStr, isToday }]
function enumerateDateRange(startTs, endTs) {
  const result = []
  const s = startOfDayTs(startTs)
  const e = startOfDayTs(endTs)
  if (s > e) return result
  const oneDay = 24 * 60 * 60 * 1000
  for (let t = s; t <= e; t += oneDay) {
    result.push({ ts: t, dateStr: formatTdxDate(t), isToday: shouldUseCurrentDayApi(t) })
  }
  return result
}
// 快捷选择「近 N 日」：终点优先选今天（交易日时），否则选最近交易日
function selectRecentDays(n) {
  if (!n || n < 1) return
  const oneDay = 24 * 60 * 60 * 1000
  if (todayIsTradingDay.value) {
    // 今天是交易日，以今天为终点
    const endTs = startOfTodayTs()
    const startTs = endTs - (n - 1) * oneDay
    onTdxDateRangeChange([startTs, endTs])
    return
  }
  // 今天非交易日，以最近交易日为终点
  GetLatestTradingDay().then(latestDay => {
    const endTs = startOfDayTs(new Date(latestDay.replace(/-/g, '/')).getTime())
    const startTs = endTs - (n - 1) * oneDay
    onTdxDateRangeChange([startTs, endTs])
  }).catch(() => {
    const endTs = startOfTodayTs()
    const startTs = endTs - (n - 1) * oneDay
    onTdxDateRangeChange([startTs, endTs])
  })
}
// 当前快捷按钮高亮：若选中范围恰好是「近 N 日」则返回 N，否则 null
const tdxActiveQuickDays = computed(() => {
  const range = tdxSelectedDateRange.value || []
  if (!range || range.length < 2 || range[0] == null || range[1] == null) return null
  // 仅判断跨度是否恰好为 N 天（不再要求终点为今天，因非交易日终点为最近交易日）
  const diffDays = Math.round((startOfDayTs(range[1]) - startOfDayTs(range[0])) / (24 * 60 * 60 * 1000)) + 1
  return [3, 5, 10, 20, 30].includes(diffDays) ? diffDays : null
})
const tdxAmountFilterOptions = [
  { label: '全部', value: 0 },
  { label: '超大单(≥100万)', value: 1 },
  { label: '大单(20-100万)', value: 2 },
  { label: '中单(4-20万)', value: 3 },
  { label: '小单(<4万)', value: 4 }
]
const SUPER_LARGE = 1000000   // 100万
const LARGE = 200000          // 20万
const MEDIUM = 40000          // 4万
// gotdx 返回的 vol 单位为「手」（1 手 = 100 股），金额 = 价格 × 手数 × 100
const SHARE_PER_LOT = 100
function transactionAmount(row) {
  return (row.price || 0) * (row.vol || 0) * SHARE_PER_LOT
}
function classifyAmount(amount) {
  if (amount >= SUPER_LARGE) return 1  // 超大单
  if (amount >= LARGE) return 2       // 大单
  if (amount >= MEDIUM) return 3      // 中单
  return 4                             // 小单
}
function amountTagType(level) {
  switch (level) {
    case 1: return 'error'     // 超大单-红
    case 2: return 'warning'   // 大单-橙
    case 3: return 'info'      // 中单-蓝
    case 4: return 'default'   // 小单-灰
    default: return 'default'
  }
}
function amountTagName(level) {
  switch (level) {
    case 1: return '超大'
    case 2: return '大'
    case 3: return '中'
    case 4: return '小'
    default: return ''
  }
}
// 按过滤条件筛选后的成交明细（表格展示用，倒序：最新成交在前）
// tdxTransactionList 保持升序（从早到晚）供折线图累计使用，表格仅展示层面反转
const filteredTdxTransactionList = computed(() => {
  const filtered = tdxAmountFilter.value === 0
    ? tdxTransactionList.value
    : tdxTransactionList.value.filter(row => {
        const amount = transactionAmount(row)
        return classifyAmount(amount) === tdxAmountFilter.value
      })
  return [...filtered].reverse()
})
// 各档位统计：买卖方向笔数/金额/占比 + 净流入金额
const tdxAmountStats = computed(() => {
  const stats = [
    { level: 1, name: '超大单', buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0 },
    { level: 2, name: '大单',   buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0 },
    { level: 3, name: '中单',   buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0 },
    { level: 4, name: '小单',   buyCount: 0, sellCount: 0, neutralCount: 0, buyAmount: 0, sellAmount: 0, neutralAmount: 0, netInflow: 0, totalAmount: 0, buyPercent: 0, sellPercent: 0, neutralPercent: 0 }
  ]
  for (const row of tdxTransactionList.value) {
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
  return stats
})
// 各档位累计净流入金额序列（按时间顺序累计，用于折线图）
// 多日模式下 x 轴使用 "MM-DD HH:MM" 区分日期，单日模式仅 "HH:MM:SS"
const tdxNetInflowSeries = computed(() => {
  const cumulative = [0, 0, 0, 0]
  const xData = []
  const series = [[], [], [], []]
  const multi = tdxIsMultiDay.value
  for (const row of tdxTransactionList.value) {
    const amount = transactionAmount(row)
    const idx = classifyAmount(amount) - 1
    if (row.buyOrSell === 0) cumulative[idx] += amount
    else if (row.buyOrSell === 1) cumulative[idx] -= amount
    if (multi && row.dateStr) {
      // dateStr "YYYY-MM-DD" → "MM-DD"，与 time 拼成 "MM-DD HH:MM:SS"
      const parts = row.dateStr.split('-')
      const md = parts.length === 3 ? `${parts[1]}-${parts[2]}` : row.dateStr
      xData.push(`${md} ${row.time || ''}`)
    } else {
      xData.push(row.time)
    }
    for (let i = 0; i < 4; i++) series[i].push(cumulative[i])
  }
  return { xData, series }
})
const tdxNetInflowChartRef = ref(null)
const tdxNetInflowChart = ref(null)
function formatWan(v) {
  return (v / 10000).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}
const tdxTransactionPagination = ref({
  page: 1,
  pageSize: 50,
  showSizePicker: true,
  pageSizes: [50, 100, 200, 500],
  itemCount: 0,
  onChange: (page) => { tdxTransactionPagination.value.page = page },
  onUpdatePageSize: (pageSize) => {
    tdxTransactionPagination.value.pageSize = pageSize
    tdxTransactionPagination.value.page = 1
  }
})
// 表格列：多日模式下在「时间」列前插入「日期」列
const tdxTransactionColumns = computed(() => {
  const base = []
  if (tdxIsMultiDay.value) {
    base.push({ title: '日期', key: 'dateStr', width: 110, fixed: 'left' })
  }
  base.push({ title: '时间', key: 'time', width: 100, fixed: 'left' })
  base.push({ title: '价格', key: 'price', width: 90, align: 'right' })
  base.push({ title: '成交量(手)', key: 'vol', width: 110, align: 'right' })
  base.push({
    title: '金额(元)', key: 'amount', width: 180, align: 'right',
    render(row) {
      const amount = transactionAmount(row)
      const level = classifyAmount(amount)
      return h('div', { style: 'display:flex; align-items:center; justify-content:flex-end; gap:6px;' }, [
        h('span', {}, amount.toLocaleString('zh-CN', { maximumFractionDigits: 2 })),
        h(NTag, { type: amountTagType(level), size: 'small', bordered: false, style: 'min-width:36px; text-align:center;' }, { default: () => amountTagName(level) })
      ])
    }
  })
  base.push({ title: '笔数', key: 'num', width: 70, align: 'right' })
  base.push({
    title: '方向', key: 'action', width: 80, align: 'center',
    render(row) {
      if (row.buyOrSell === 0) {
        return h(NTag, { type: 'error', size: 'small', bordered: false }, { default: () => '买' })
      }
      if (row.buyOrSell === 1) {
        return h(NTag, { type: 'success', size: 'small', bordered: false }, { default: () => '卖' })
      }
      return h(NTag, { type: 'default', size: 'small', bordered: false }, { default: () => '中性' })
    }
  })
  return base
})
const currentStockTradingPrice = ref({
  stockCode: '',
  costPrice: 0,
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})
/** 用于功能权限：仅在赞助有效期内为解密等级，否则为 0（与 EffectiveSponsorVipLevel 一致） */
const vipLevel = ref(0)
const klineAutoCloseTimer = ref(null)
const addBTN = ref(true)
const enableTools = ref(true)
const thinkingMode = ref(true)
const formModel = ref({
  name: "",
  code: "",
  costPrice: 0.000,
  volume: 0,
  alarm: 0,
  alarmPrice: 0,
  sort: 999,
  cron: "",
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})

const promptTemplates = ref([])
const aiConfigs = ref([])
const sysPromptOptions = ref([])
const userPromptOptions = ref([])
const data = reactive({
  modelName: "",
  chatId: "",
  question: "",
  sysPromptId: null,
  aiConfigId: null,
  name: "",
  code: "",
  fenshiURL: "",
  kURL: "",
  resultText: "Please enter your name below 👇",
  fullscreen: false,
  airesult: "",
  openAiEnable: false,
  loading: true,
  analysisStatus: "",
  enableDanmu: false,
  darkTheme: false,
  changePercent: 0
})
const feishiInterval = ref(null)
const aiAnalysisTimeout = ref(null)


const currentGroupId = ref(0)


const theme = computed(() => {
  return data.darkTheme ? 'dark' : 'light'
})

const danmakuColor = computed(() => {
  return data.darkTheme ? 'color:#fff' : 'color:#000'
})

// 顶部页签固定吸顶时的背景色（与页面 body 背景一致，避免滚动时内容透出）
const tabNavBgColor = computed(() => {
  return data.darkTheme ? 'rgb(16, 16, 20)' : '#ffffff'
})

const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');

const sortedResults = computed(() => {
  const sortedKeys = keys(results.value).sort();
  const sortedObject = {};
  sortedKeys.forEach(key => {
    sortedObject[key] = results.value[key];
  });
  return sortedObject
});

const groupResults = computed(() => {
  if (currentGroupId.value === 0) {
    return sortedResults.value
  }
  // 用 Set 替换 Array.includes，避免在自选股数量多时退化为 O(n^2) 查找
  const codeSet = new Set(stocks.value)
  const group = {}
  for (const key in sortedResults.value) {
    const item = sortedResults.value[key]
    if (item && codeSet.has(item['股票代码'])) {
      group[key] = item
    }
  }
  return group
})

// ——「全部」标签页：表格分页 + 搜索 ——
const tableSearchKeyword = ref('')
// 「全部」标签页分组筛选：0 表示不按分组筛选，>0 为选中分组 ID
const tableGroupFilter = ref(0)

// 将 sortedResults 对象转为数组，并按关键字（名称/代码）+ 分组/概念条件过滤
const allTableData = computed(() => {
  const arr = []
  for (const key in sortedResults.value) {
    arr.push(sortedResults.value[key])
  }
  // 分组条件过滤：选中分组 ID > 0 时，只保留属于该分组的股票
  const gid = tableGroupFilter.value
  let filtered = gid > 0
    ? arr.filter(item => (codeToGroupIds.value.get(item['股票代码']) || []).includes(gid))
    : arr
  // 概念条件过滤：选中概念 ID > 0 时，只保留属于该概念的股票
  const cid = tableConceptFilter.value
  if (cid > 0) {
    filtered = filtered.filter(item => (codeToConceptIds.value.get(item['股票代码']) || []).includes(cid))
  }
  // 关键字过滤（名称/代码）
  const kw = tableSearchKeyword.value.trim().toLowerCase()
  if (!kw) return filtered
  return filtered.filter(item => {
    const name = String(item['股票名称'] || '').toLowerCase()
    const code = String(item['股票代码'] || '').toLowerCase()
    return name.includes(kw) || code.includes(kw)
  })
})

// 分组筛选下拉选项：首项为「全部分组」，其余来自 groupList
const groupFilterOptions = computed(() => {
  const opts = [{ label: '全部分组', value: 0 }]
  for (const g of groupList.value) {
    if (g && g.ID) opts.push({ label: g.name, value: g.ID })
  }
  return opts
})

// 概念筛选下拉选项：首项为「全部概念」，其余来自 conceptList
const conceptFilterOptions = computed(() => {
  const opts = [{ label: '全部概念', value: 0 }]
  for (const c of conceptList.value) {
    if (c && c.ID) opts.push({ label: c.name, value: c.ID })
  }
  return opts
})

// 客户端分页配置
const allTablePagination = reactive({
  page: 1,
  pageSize: 50,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix({ itemCount }) {
    return `共 ${itemCount} 只`
  },
  onChange: (page) => { allTablePagination.page = page },
  onUpdatePageSize: (pageSize) => {
    allTablePagination.pageSize = pageSize
    allTablePagination.page = 1
  }
})

// 搜索关键字变化时回到第一页
watch(tableSearchKeyword, () => { allTablePagination.page = 1 })

// 分组筛选变化时回到第一页
watch(tableGroupFilter, () => { allTablePagination.page = 1 })

// 概念筛选变化时回到第一页
watch(tableConceptFilter, () => { allTablePagination.page = 1 })

// 「全部」标签页表格列定义（render 用 h()；行高频刷新由 allTableData computed 驱动，与原卡片一致）
const allTableColumns = [
  {
    title: '名称/代码', key: '股票名称', width: 150,
    sorter: (a, b) => String(a['股票名称']).localeCompare(String(b['股票名称'])),
    render(row) {
      return h('div', { style: 'display:flex; flex-direction:column; line-height:1.3;' }, [
        h(NText, { type: row.type, strong: true }, { default: () => row['股票名称'] }),
        h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => row['股票代码'] })
      ])
    }
  },
  {
    title: '分组', key: 'groups', width: 140,
    // 排序按分组名拼接（无分组排最后）
    sorter: (a, b) => {
      const ga = (codeToGroupNames.value.get(a['股票代码']) || []).map(g => g.name).join(',')
      const gb = (codeToGroupNames.value.get(b['股票代码']) || []).map(g => g.name).join(',')
      if (!ga && !gb) return 0
      if (!ga) return 1
      if (!gb) return -1
      return ga.localeCompare(gb)
    },
    render(row) {
      const groups = codeToGroupNames.value.get(row['股票代码']) || []
      if (groups.length === 0) {
        return h(NText, { depth: 3, style: 'font-size:12px;' }, { default: () => '—' })
      }
      // 点击具体分组名跳转到对应分组页签
      return h('div', { style: 'display:flex; flex-wrap:wrap; gap:2px;' },
        groups.map(g => h(NTag, {
          size: 'small', bordered: false, type: 'success',
          style: 'cursor:pointer;',
          onClick: () => updateTab(String(g.id))
        }, { default: () => g.name }))
      )
    }
  },
  {
    title: '概念', key: 'concepts', width: 140,
    // 排序按概念名拼接（无概念排最后）
    sorter: (a, b) => {
      const ca = (codeToConceptNames.value.get(a['股票代码']) || []).map(c => c.name).join(',')
      const cb = (codeToConceptNames.value.get(b['股票代码']) || []).map(c => c.name).join(',')
      if (!ca && !cb) return 0
      if (!ca) return 1
      if (!cb) return -1
      return ca.localeCompare(cb)
    },
    render(row) {
      const concepts = codeToConceptNames.value.get(row['股票代码']) || []
      if (concepts.length === 0) {
        return h(NText, { depth: 3, style: 'font-size:12px;' }, { default: () => '—' })
      }
      // 仅展示，不可点击跳转（概念无页签）
      return h('div', { style: 'display:flex; flex-wrap:wrap; gap:2px;' },
        concepts.map(c => h(NTag, { size: 'small', bordered: false, type: 'info' }, { default: () => c.name }))
      )
    }
  },
  {
    title: '当前价', key: '当前价格', width: 110,
    sorter: (a, b) => Number(a['当前价格']) - Number(b['当前价格']),
    render(row) {
      const children = [h(NText, { type: row.type }, { default: () => Number(row['当前价格']).toFixed(2) })]
      if (row['盘前盘后'] > 0) {
        children.push(h('div', { style: 'font-size:12px;' },
          `${row['盘前盘后']} ${row['盘前盘后涨跌幅']}%`))
      }
      return h('div', { style: 'display:flex; flex-direction:column;' }, children)
    }
  },
  {
    title: '涨跌幅', key: 'changePercent', width: 90,
    sorter: (a, b) => Number(a.changePercent) - Number(b.changePercent),
    defaultSortOrder: 'descend',
    render(row) {
      const sign = row.changePercent >= 0 ? '+' : ''
      return h(NText, { type: row.type }, { default: () => `${sign}${Number(row.changePercent).toFixed(3)}%` })
    }
  },
  {
    title: '最高/最低', key: '今日最高价', width: 160,
    sorter: (a, b) => Number(a['今日最高价']) - Number(b['今日最高价']),
    render(row) {
      return h('div', { style: 'font-size:12px; line-height:1.4;' }, [
        h('div', null, `高 ${row['今日最高价']} (${row.highRate}%)`),
        h('div', null, `低 ${row['今日最低价']} (${row.lowRate}%)`)
      ])
    }
  },
  {
    title: '昨收/今开', key: '昨日收盘价', width: 120,
    sorter: (a, b) => Number(a['昨日收盘价']) - Number(b['昨日收盘价']),
    render(row) {
      return h('div', { style: 'font-size:12px; line-height:1.4;' }, [
        h('div', null, `昨收 ${row['昨日收盘价']}`),
        h('div', null, `今开 ${row['今日开盘价']}`)
      ])
    }
  },
  {
    title: '时间', key: '日期', width: 140,
    sorter: (a, b) => String(a['日期'] + ' ' + a['时间']).localeCompare(String(b['日期'] + ' ' + b['时间'])),
    render(row) {
      return h('div', { style: 'font-size:12px;' }, `${row['日期']} ${row['时间']}`)
    }
  },
  {
    title: '操作', key: 'actions', width: 540, fixed: 'right',
    render(row) {
      const btns = [
        h(NButton, { size: 'tiny', type: 'primary', secondary: true, onClick: () => showLightweightKline(row['股票代码'], row['股票名称']) }, { default: () => '多周期' }),
        h(NButton, { size: 'tiny', type: 'error', secondary: true, style: 'margin-left:4px;', onClick: () => showK(row['股票代码'], row['股票名称']) }, { default: () => '日K' }),
        h(NButton, { size: 'tiny', type: 'error', secondary: true, style: 'margin-left:4px;', onClick: () => showFenshi(row['股票代码'], row['股票名称'], row.changePercent) }, { default: () => '分时' }),
        h(NButton, { size: 'tiny', type: 'info', secondary: true, style: 'margin-left:4px;', onClick: () => showTransactionDetail(row['股票代码'], row['股票名称']) }, { default: () => '成交明细' })
      ]
      if (row['买一报价'] > 0) {
        btns.push(h(NButton, { size: 'tiny', type: 'error', secondary: true, style: 'margin-left:4px;', onClick: () => showMoney(row['股票代码'], row['股票名称']) }, { default: () => '资金' }))
      }
      btns.push(h(NButton, { size: 'tiny', type: 'success', secondary: true, style: 'margin-left:4px;', onClick: () => search(row['股票代码'], row['股票名称']) }, { default: () => '详情' }))
      if (row['买一报价'] > 0) {
        btns.push(h(NButton, { size: 'tiny', type: 'success', secondary: true, style: 'margin-left:4px;', onClick: () => searchNotice(row['股票代码']) }, { default: () => '公告' }))
        btns.push(h(NButton, { size: 'tiny', type: 'success', secondary: true, style: 'margin-left:4px;', onClick: () => searchStockReport(row['股票代码']) }, { default: () => '研报' }))
      }
      btns.push(h(NButton, { size: 'tiny', type: 'warning', secondary: true, style: 'margin-left:4px;', onClick: () => setStock(row['股票代码'], row['股票名称']) }, { default: () => '成本' }))
      if (data.openAiEnable) {
        btns.push(h(NButton, { size: 'tiny', type: 'warning', secondary: true, style: 'margin-left:4px;', onClick: () => aiCheckStock(row['股票名称'], row['股票代码']) }, { default: () => 'AI分析' }))
      }
      // 设置分组下拉：复用统一的 options/renderLabel/onSelect，支持新建分组 + 切换（加入/移出）
      btns.push(h(NDropdown, {
        trigger: 'click', options: setGroupOptions.value,
        menuProps: () => ({ style: 'max-height:300px; overflow-y:auto;' }),
        renderLabel: (option) => renderSetGroupLabel(option, row['股票代码']),
        onSelect: (groupId) => handleSetGroupSelect(groupId, row['股票代码'], row['股票名称'])
      }, {
        default: () => h(NButton, { size: 'tiny', type: 'warning', tertiary: true, style: 'margin-left:4px;' }, { default: () => '设置分组' })
      }))
      // 设置概念下拉：与设置分组一致，支持新建概念 + 切换（加入/移出），概念不产生页签
      btns.push(h(NDropdown, {
        trigger: 'click', options: setConceptOptions.value,
        menuProps: () => ({ style: 'max-height:300px; overflow-y:auto;' }),
        renderLabel: (option) => renderSetConceptLabel(option, row['股票代码']),
        onSelect: (conceptId) => handleSetConceptSelect(conceptId, row['股票代码'], row['股票名称'])
      }, {
        default: () => h(NButton, { size: 'tiny', type: 'info', tertiary: true, style: 'margin-left:4px;' }, { default: () => '设置概念' })
      }))
      btns.push(h(NButton, { size: 'tiny', type: 'error', tertiary: true, style: 'margin-left:4px;', onClick: () => removeMonitor(row['股票代码'], row['股票名称'], row.key) }, { default: () => '取消关注' }))
      return h('div', { style: 'display:flex; flex-wrap:wrap; gap:4px; align-items:center;' }, btns)
    }
  }
]

const showPopover = ref(false)
// 拖拽相关变量
const dragSourceIndex = ref(null)
const dragTargetIndex = ref(null)

// 拖拽处理函数
function handleTabDragStart(event, name) {
  // "全部"标签（name=0）不应该触发拖拽
  if (name === 0) {
    event.preventDefault();
    return;
  }
  dragSourceIndex.value = name;
  event.dataTransfer.effectAllowed = 'move';
  event.target.classList.add('tab-dragging');
}


function handleTabDragOver(event) {
  event.preventDefault()
  event.dataTransfer.dropEffect = 'move'
}

function handleTabDragEnter(event, name) {
  event.preventDefault();
  // "全部"标签（name=0）不应该作为拖拽目标
  if (name > 0) {
    dragTargetIndex.value = name;
    if (event.target.classList) {
      // 查找最近的标签元素并添加高亮样式
      let tabElement = event.target.closest('.n-tabs-tab');
      if (tabElement) {
        tabElement.classList.add('tab-drag-over');
      }
    }
  }
}

function handleTabDragLeave(event) {
  // 查找最近的标签元素并移除高亮样式
  let tabElement = event.target.closest('.n-tabs-tab')
  if (tabElement && tabElement.classList) {
    tabElement.classList.remove('tab-drag-over')
  }
  // 不要重置 dragTargetIndex，因为可能会在元素间快速移动
}

function handleTabDrop(event) {
  event.preventDefault();

  // 移除所有高亮样式
  const tabs = document.querySelectorAll('.n-tabs-tab');
  if(!tabs || tabs.length === 0){
    return
  }
  tabs.forEach(tab => {
    tab.classList.remove('tab-drag-over');
  });

  if (dragSourceIndex.value !== null && dragTargetIndex.value !== null &&
      dragSourceIndex.value !== dragTargetIndex.value) {

    // 确保索引有效（排除"全部"选项卡）
    if (dragSourceIndex.value > 0 && dragTargetIndex.value > 0) {
      // 查找源分组和目标分组
      const sourceGroup = groupList.value.find(g => g.ID === dragSourceIndex.value);
      const targetGroup = groupList.value.find(g => g.ID === dragTargetIndex.value);

      if (sourceGroup && targetGroup) {
        // 计算新的位置序号（使用目标分组的sort值）
        const newSortPosition = targetGroup.sort;

        // 调用后端API更新组排序
        UpdateGroupSort(sourceGroup.ID, newSortPosition).then(result => {
          if (result) {
            message.success('分组排序更新成功');
            // 重新获取分组列表以更新界面
            GetGroupList().then(result => {
              groupList.value = result;
            });
          } else {
            message.error('分组排序更新失败');
          }
        }).catch(error => {
          message.error('分组排序更新失败: ' + error.message);
        });
      }
    }
  }

  // 重置状态
  dragSourceIndex.value = null;
  dragTargetIndex.value = null;
}

function handleTabDragEnd(event) {
  // 移除所有高亮样式
  const tabs = document.querySelectorAll('.n-tabs-tab')
  if(!tabs || tabs.length === 0){
    return
  }
  tabs.forEach(tab => {
    tab.classList.remove('tab-drag-over', 'tab-dragging')
  })

  dragSourceIndex.value = null
  dragTargetIndex.value = null
}

onBeforeMount(() => {
  GetGroupList().then(result => {
    groupList.value = result
    const sorts = result.map(item => item.sort);
    const uniqueSorts = new Set(sorts);
    if (sorts.length !== uniqueSorts.size) {
      fetchGroupList();
    } else {
      if (route.query.groupId) {
        message.success("切换分组:" + route.query.groupName)
        currentGroupId.value = Number(route.query.groupId)
      }
    }
  }).catch(err => { console.error("GetGroupList error:", err) })
  // 加载全量分组归属，用于「全部」标签页表格的分组列
  refreshCodeToGroups()
  // 加载概念列表 + 全量概念归属，用于「全部」标签页表格的概念列与下拉勾选
  GetConceptList().then(result => {
    conceptList.value = result
  }).catch(err => { console.error("GetConceptList error:", err) })
  refreshCodeToConcepts()
  GetStockList("").then(result => {
    stockList.value = result
    options.value = result.map(item => {
      return {
        label: item.name + " - " + item.ts_code,
        value: item.ts_code
      }
    })
  }).catch(err => { console.error("GetStockList error:", err) })
  GetConfig().then(result => {
    if (result.openAiEnable) {
      data.openAiEnable = true
    }
    if (result.enableDanmu) {
      data.enableDanmu = true
    }
    if (result.darkTheme) {
      data.darkTheme = true
    }
  }).catch(err => { console.error("GetConfig error:", err) })
  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res

    sysPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型系统Prompt')
    userPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型用户Prompt')

  }).catch(err => { console.error("GetPromptTemplates error:", err) })

  GetAiConfigs().then(res => {
    aiConfigs.value = res
    if (res && res.length > 0) {
      data.aiConfigId = res[0].ID
    }
  }).catch(err => { console.error("GetAiConfigs error:", err) })

  EventsOn("loadingDone", (data) => {
    message.loading("刷新股票基础数据...")
    GetStockList("").then(result => {
      stockList.value = result
      options.value = result.map(item => {
        return {
          label: item.name + " - " + item.ts_code,
          value: item.ts_code
        }
      })
    })
  })

  EventsOn("refresh", (data) => {
    message.success(data)
  })

  EventsOn("showSearch", (data) => {
    addBTN.value = data === 1;
  })

  EventsOn("stock_price", (data) => {
    updateData(data)
  })

  EventsOn("refreshFollowList", (data) => {

    WindowReload()
  })

  EventsOn("newChatStream", async (msg) => {
    if (msg === "DONE") {
      // 清除超时定时器
      if (aiAnalysisTimeout.value) {
        clearTimeout(aiAnalysisTimeout.value)
        aiAnalysisTimeout.value = null
      }
      SaveAIResponseResult(data.code, data.name, data.airesult, data.chatId, data.question, data.aiConfigId)
      data.loading = false
      data.analysisStatus = "分析完成"
      message.destroyAll()
      notify.success({
        title: 'AI分析完成',
        content: `[${data.name}] 分析已完成`,
        duration: 3000,
      })
      setTimeout(() => {
        data.analysisStatus = ""
      }, 3000)
    } else {
      if (msg.chatId) {
        data.chatId = msg.chatId
      }
      if (msg.question) {
        data.question = msg.question
      }
      if (msg.content || msg.reasoning_content || msg.extraContent) {
        if (!data.airesult) {
          data.analysisStatus = "AI正在分析中..."
        }
        data.loading = false
      }
      if (msg.content) {
        data.airesult = data.airesult + msg.content
      }
      if (msg.reasoning_content) {
        data.airesult = data.airesult + msg.reasoning_content
      }
      if (msg.extraContent) {
        data.airesult = data.airesult + msg.extraContent
      }
      scrollToAiResultBottom()
    }
  })

  EventsOn("changeTab", async (msg) => {
    currentGroupId.value = Number(msg.ID)
    nextTick(() => {
      updateTab(currentGroupId.value);
    });
  })

  // AI 工具修改分组/概念后推送此事件，触发前端刷新缓存
  EventsOn("stockDataChanged", () => {
    refreshCodeToConcepts()
    refreshCodeToGroups()
  })

  EventsOn("updateVersion", async (msg) => {
    const githubTimeStr = msg.published_at;
    const utcDate = new Date(githubTimeStr);
    const date = new Date(utcDate.getTime());
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    const formattedDate = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    notify.info({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '发现新版本: ' + msg.tag_name,
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, {default: () => msg.commit?.message})
      },
      duration: 5000,
      meta: "发布时间:" + formattedDate,
      action: () => {
        return h(NButton, {
          type: 'primary',
          size: 'small',
          onClick: () => {
            Environment().then(env => {
              switch (env.platform) {
                case 'windows':
                  window.open(msg.html_url)
                  break
                default :
                  OpenURL(msg.html_url)
              }
            })
          }
        }, {default: () => '查看'})
      }
    })
  })

  EventsOn("updateNeedAdmin", (msg) => {
    notify.warning({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '更新需要管理员权限',
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, { default: () => '新版本 ' + (msg.version || '') + ' 下载完成，但自动替换文件需要管理员权限。请以管理员身份重启程序后再次检查更新。' })
      },
      duration: 15000,
      action: () => {
        return h(NButton, {
          type: 'warning',
          size: 'small',
          onClick: () => {
            RestartAsAdmin()
          }
        }, { default: () => '以管理员身份重启' })
      }
    })
  })

  EventsOn("warnMsg", async (msg) => {
    notify.error({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '警告',
      duration: 5000,
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, {default: () => msg})
      },
    })
  })
})
// 监听分组列表变化，重新初始化拖拽
const unwatch = watch(groupList, () => {
  nextTick(() => {
    initDraggableTabs();
  });
});

// 在组件卸载时清理监听器
onBeforeUnmount(() => {
  unwatch();
});
onMounted(() => {
  nextTick(() => {
    initDraggableTabs();
  });

  message.loading("Loading...")
  GetFollowList(currentGroupId.value).then(result => {

    followList.value = result
    for (const followedStock of result) {
      if (followedStock.StockCode.startsWith("us")) {
        followedStock.StockCode = "gb_" + followedStock.StockCode.replace("us", "").toLowerCase()
      }
      if (!stocks.value.includes(followedStock.StockCode)) {
        stocks.value.push(followedStock.StockCode)
      }
      Greet(followedStock.StockCode).then(result => {
        updateData(result)
      })
    }
    //monitor()
    message.destroyAll()
  })

  GetVersionInfo().then((res) => {
    icon.value = res.icon
    refreshEffectiveVip()
  })
  // 创建 WebSocket 连接
  ws.value = new WebSocket('ws://8.134.249.145:16688/ws'); // 替换为你的 WebSocket 服务器地址
  //ws.value = new WebSocket('ws://localhost:16688/ws'); // 替换为你的 WebSocket 服务器地址

  ws.value.onopen = () => {
    //console.log('WebSocket 连接已打开');
  };

  ws.value.onmessage = (event) => {
    if (data.enableDanmu) {
      danmus.value.push(event.data);
    }
  };

  ws.value.onerror = (error) => {
    console.error('WebSocket 错误:', error);
  };

  ws.value.onclose = () => {
    //console.log('WebSocket 连接已关闭');
  };
})
// 清理拖拽事件监听器
// 清理拖拽事件监听器
function cleanupDraggableTabs() {
  const tabs = document.querySelectorAll('.n-tabs-tab');
  if(!tabs || tabs.length === 0){
    return
  }
  tabs.forEach((tab) => {
    // 移除所有可能的拖拽事件监听器
    tab.removeEventListener('dragstart', handleTabDragStart);
    tab.removeEventListener('dragover', handleTabDragOver);
    tab.removeEventListener('dragenter', handleTabDragEnter);
    tab.removeEventListener('dragleave', handleTabDragLeave);
    tab.removeEventListener('drop', handleTabDrop);
    tab.removeEventListener('dragend', handleTabDragEnd);
    // 移除draggable属性
    tab.removeAttribute('draggable');
  });
}

// 初始化可拖拽选项卡
function initDraggableTabs() {
  // 移除之前可能添加的事件监听器
  cleanupDraggableTabs();

  // 添加拖拽事件监听器到选项卡元素
  setTimeout(() => {
    const tabs = document.querySelectorAll('.n-tabs-tab');
    if(!tabs || tabs.length === 0){
      return
    }
    tabs.forEach((tab, index) => {
      const dataIndex = tab.getAttribute('data-name');
      const name = parseInt(dataIndex);

      // 只为分组标签（name > 0）添加拖拽功能
      if (name > 0) {
        tab.setAttribute('draggable', 'true');
        tab.addEventListener('dragstart', (e) => handleTabDragStart(e, name));
        tab.addEventListener('dragover', handleTabDragOver);
        tab.addEventListener('dragenter', (e) => handleTabDragEnter(e, name));
        tab.addEventListener('dragleave', handleTabDragLeave);
        tab.addEventListener('drop', handleTabDrop);
        tab.addEventListener('dragend', handleTabDragEnd);
      }
    });
  }, 100);
}

onBeforeUnmount(() => {
  // //console.log(`the component is now unmounted.`)
  //clearInterval(ticker.value)
  ws.value.close()
  message.destroyAll()
  notify.destroyAll()
  clearInterval(feishiInterval.value)
  // 清理 AI 分析超时定时器
  if (aiAnalysisTimeout.value) {
    clearTimeout(aiAnalysisTimeout.value)
    aiAnalysisTimeout.value = null
  }
  // 清理多周期 K 线自动关闭定时器
  if (klineAutoCloseTimer.value) {
    clearTimeout(klineAutoCloseTimer.value)
    klineAutoCloseTimer.value = null
  }

  EventsOff("refresh")
  EventsOff("showSearch")
  EventsOff("stock_price")
  EventsOff("refreshFollowList")
  EventsOff("newChatStream")
  EventsOff("changeTab")
  EventsOff("updateVersion")
  EventsOff("updateNeedAdmin")
  EventsOff("warnMsg")
  EventsOff("loadingDone")

  cleanupDraggableTabs()

})

//判断是否是A股交易时间
function isTradingTime() {
  const now = new Date();
  const day = now.getDay(); // 获取星期几，0表示周日，1-6表示周一至周六
  if (day >= 1 && day <= 5) { // 周一至周五
    const hours = now.getHours();
    const minutes = now.getMinutes();
    const totalMinutes = hours * 60 + minutes;
    const startMorning = 9 * 60 + 15; // 上午9点15分换算成分钟数
    const endMorning = 11 * 60 + 30; // 上午11点30分换算成分钟数
    const startAfternoon = 13 * 60; // 下午13点换算成分钟数
    const endAfternoon = 15 * 60; // 下午15点换算成分钟数
    if ((totalMinutes >= startMorning && totalMinutes < endMorning) ||
        (totalMinutes >= startAfternoon && totalMinutes < endAfternoon)) {
      return true;
    }
  }
  return false;
}

// 添加一个获取分组列表的函数，用于处理初始化逻辑
function fetchGroupList() {
  InitializeGroupSort().then(initResult => {
    if (initResult) {
      GetGroupList().then(result => {
        groupList.value = result
        if (route.query.groupId) {
          message.success("切换分组:" + route.query.groupName)
          currentGroupId.value = Number(route.query.groupId)
        }
      })
    } else {
      message.error("初始化分组序号失败")
    }
  })
}

// 刷新「股票代码 -> 所属分组名/ID 数组」映射，供「全部」标签页表格分组列与分组筛选使用。
// 一次拉取全量 group_stock_info（含 GroupInfo），前端按 stockCode 聚合。
function refreshCodeToGroups() {
  GetAllGroupStocks().then(list => {
    const nameMap = new Map()
    const idMap = new Map()
    if (Array.isArray(list)) {
      for (const gs of list) {
        const code = gs.stockCode
        if (!code) continue
        const gname = gs.groupInfo && gs.groupInfo.name ? gs.groupInfo.name : ''
        const gid = gs.groupInfo && gs.groupInfo.ID ? gs.groupInfo.ID : 0
        if (gname && gid) {
          if (!nameMap.has(code)) nameMap.set(code, [])
          nameMap.get(code).push({ id: gid, name: gname })
        }
        if (gid) {
          if (!idMap.has(code)) idMap.set(code, [])
          idMap.get(code).push(gid)
        }
      }
    }
    codeToGroupNames.value = nameMap
    codeToGroupIds.value = idMap
  }).catch(err => { console.error("GetAllGroupStocks error:", err) })
}

// 刷新「股票代码 -> 所属概念名/ID 数组」映射，供「全部」标签页表格概念列与概念筛选使用。
// 一次拉取全量 stock_concept_relation（含 ConceptInfo），前端按 stockCode 聚合。
function refreshCodeToConcepts() {
  GetAllStockConcepts().then(list => {
    const nameMap = new Map()
    const idMap = new Map()
    if (Array.isArray(list)) {
      for (const cs of list) {
        const code = cs.stockCode
        if (!code) continue
        const cname = cs.conceptInfo && cs.conceptInfo.name ? cs.conceptInfo.name : ''
        const cid = cs.conceptInfo && cs.conceptInfo.ID ? cs.conceptInfo.ID : 0
        if (cname && cid) {
          if (!nameMap.has(code)) nameMap.set(code, [])
          nameMap.get(code).push({ id: cid, name: cname })
        }
        if (cid) {
          if (!idMap.has(code)) idMap.set(code, [])
          idMap.get(code).push(cid)
        }
      }
    }
    codeToConceptNames.value = nameMap
    codeToConceptIds.value = idMap
  }).catch(err => { console.error("GetAllStockConcepts error:", err) })
}

// 关注时的分组选择下拉选项（参考形态选股 allStockList.vue）
const followGroupOptions = computed(() => {
  const opts = [{label: '默认（不分组）', key: 0}]
  groupList.value.forEach(g => opts.push({label: g.name, key: g.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建分组', key: 'new'})
  return opts
})

// 「设置分组」下拉选项：分组列表 + 分隔符 + 新建分组（与关注下拉一致，复用 new 流程）
const setGroupOptions = computed(() => {
  const opts = []
  groupList.value.forEach(g => opts.push({label: g.name, key: g.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建分组', key: 'new'})
  return opts
})

// 「设置概念」下拉选项：概念列表 + 分隔符 + 新建概念（概念不产生页签）
const setConceptOptions = computed(() => {
  const opts = []
  conceptList.value.forEach(c => opts.push({label: c.name, key: c.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建概念', key: 'new'})
  return opts
})

// 新建分组后待关注的股票（null 表示非关注流程打开的分组弹窗）
const pendingFollow = ref(null)
// 「设置分组」时新建分组后待加入的股票（null 表示非设置分组流程打开的分组弹窗）
const pendingAddStockGroup = ref(null)

function groupNameById(id) {
  const g = groupList.value.find(item => item.ID === id)
  return g ? g.name : ''
}

function handleFollowSelect(key) {
  if (key === 'new') {
    if (!data?.code) {
      message.error("请输入有效股票代码")
      showPopover.value = true
      return
    }
    pendingFollow.value = {code: data.code, name: data.name}
    addTabModel.value = {name: '', sort: 1}
    addTabPane.value = true
    return
  }
  doFollowStock(Number(key))
}

// 关注并加入分组（groupId=0 表示不分组），参考形态选股 doFollow
function doFollowStock(groupId) {
  if (!data?.code) {
    message.error("请输入有效股票代码")
    showPopover.value = true
    return
  }
  if (stocks.value.includes(data.code)) {
    message.error("已经关注了")
    return
  }
  Follow(data.code).then(result => {
    if (result === "关注成功") {
      // 后端 Follow 把 us 前缀归一化为 gb_，前端 stocks 数组需同步
      if (data.code.startsWith("us")) {
        data.code = "gb_" + data.code.replace("us", "").toLowerCase()
      }
      stocks.value.push(data.code)
      message.success(groupId > 0 ? `已关注，并加入分组「${groupNameById(groupId)}」` : '关注成功')
      // 加入分组（code 用 gb_ 格式，与后端 followed_stock.stock_code 一致）
      if (groupId > 0) {
        AddStockGroup(groupId, data.code).then(() => {
          GetGroupList().then(gList => { groupList.value = gList })
          // 刷新「全部」标签页表格的分组列映射
          refreshCodeToGroups()
          if (currentGroupId.value === groupId) {
            updateTab(currentGroupId.value)
          }
        }).catch(err => message.error('加入分组失败: ' + (err?.message || err)))
      }
      GetFollowList(currentGroupId.value).then(result => { followList.value = result })
      monitor()
    } else {
      message.error(result)
    }
  }).catch(err => message.error('关注失败: ' + (err?.message || err)))
}


function removeMonitor(code, name, key) {
  //console.log("removeMonitor",name,code,key)
  stocks.value.splice(stocks.value.indexOf(code), 1)
  //console.log("removeMonitor-key",key)
  //console.log("removeMonitor-v",results.value[key])

  delete results.value[key]
  //console.log("removeMonitor-v",results.value[key])

  UnFollow(code).then(result => {
    message.success(result)
    monitor()
  })
}


function SendDanmu() {
  //danmus.value.push(data.name)
  //console.log("SendDanmu",data.name)
  //console.log("SendDanmu-readyState", ws.value.readyState)
  ws.value.send(data.name)
}

// 在线搜索防抖（用于场内 ETF 等本地缓存未覆盖的标的）
let stockSearchTimer = null
let stockSearchSeq = 0

function getStockList(value) {


  // //console.log("getStockList",value)
  let result;
  result = stockList.value.filter(item => item.name.includes(value) || item.ts_code.includes(value))
  options.value = result.map(item => {
    return {
      label: item.name + " - " + item.ts_code,
      value: item.ts_code
    }
  })
  if (value && value.indexOf("-") <= 0) {
    data.code = value
  }

  //console.log("getStockList-options",data.code)

  if (data.code) {
    let findId = data.code
    if (findId.startsWith("us")) {
      findId = "gb_" + findId.replace("us", "").toLowerCase()
    }
    blinkBorder(findId)
  }

  // 非空关键字时，防抖调用后端在线搜索（含场内 ETF：本地 FundBasic 缺失时会在线拉取），
  // 合并本地 stockList 未覆盖的结果，使 513310 等场内基金可被搜到并关注
  if (stockSearchTimer) clearTimeout(stockSearchTimer)
  if (!value) return
  const seq = ++stockSearchSeq
  stockSearchTimer = setTimeout(() => {
    GetStockList(value).then(res => {
      if (seq !== stockSearchSeq || !res || !res.length) return
      const existing = new Set(options.value.map(o => o.value))
      const extra = []
      for (const item of res) {
        if (item.ts_code && !existing.has(item.ts_code)) {
          extra.push({ label: (item.name || '') + " - " + item.ts_code, value: item.ts_code })
          existing.add(item.ts_code)
        }
        if (extra.length >= 20) break
      }
      if (extra.length) options.value = options.value.concat(extra)
    }).catch(() => {})
  }, 300)

}

function blinkBorder(findId) {
  // 获取要滚动到的元素
  let element = document.getElementById(findId);
  //console.log("blinkBorder",findId,element)
  if (element) {
    // 滚动到该元素
    element.scrollIntoView({behavior: 'smooth'});
    const pelement = document.getElementById(findId + '_gi');
    if (pelement) {
      // 添加闪烁效果
      pelement.classList.add('blink-border');
      // 3秒后移除闪烁效果
      setTimeout(() => {
        pelement.classList.remove('blink-border');
      }, 1000 * 5);
    } else {
      console.error(`Element with ID ${findId}_gi not found`);
    }
  }
}

async function updateData(result) {
  ////console.log("stock_price",result['日期'],result['时间'],result['股票代码'],result['股票名称'],result['当前价格'],result['盘前盘后'])

  if (result["当前价格"] <= 0) {
    result["当前价格"] = result["卖一报价"]
  }

  if (result.changePercent > 0) {
    result.type = "error"
    result.color = "#E88080"
  } else if (result.changePercent < 0) {
    result.type = "success"
    result.color = "#63E2B7"
  } else {
    result.type = "default"
    result.color = "#FFFFFF"
  }

  if (result.profitAmount > 0) {
    result.profitType = "error"
  } else if (result.profitAmount < 0) {
    result.profitType = "success"
  }
  if (result["当前价格"]) {
    // if (result.alarmChangePercent > 0 && Math.abs(result.changePercent) >= result.alarmChangePercent) {
    //   SendMessage(result, 1)
    // }

    // if (result.alarmPrice > 0 && result["当前价格"] >= result.alarmPrice) {
    //   SendMessage(result, 2)
    // }

    // if (result.costPrice > 0 && result["当前价格"] >= result.costPrice) {
    //   SendMessage(result, 3)
    // }

    checkPriceLineAlerts(result)
  }

  // 行情系高频推送，避免整体重建 results 触发全卡片重渲染：
  // 只移除同一股票（sort 变化导致 key 变化时）的旧条目，再写入新条目。
  const _stockCode = result["股票代码"]
  result.key = GetSortKey(result.sort, _stockCode)
  let _prev = null
  for (const oldKey in results.value) {
    const old = results.value[oldKey]
    if (old && old["股票代码"] === _stockCode) {
      _prev = old
      if (oldKey !== result.key) delete results.value[oldKey]
      break
    }
  }
  // 缓存上一次推送的数值，供 n-number-animation 平滑过渡（替代 from=0 的高频重启动画）
  result.lastChangePercent = _prev ? _prev.changePercent : 0
  result.lastProfitAmountToday = _prev ? _prev.profitAmountToday : 0
  results.value[result.key] = result
  if (!stocks.value.includes(_stockCode)) {
    delete results.value[result.key]
  }
}


async function monitor() {
  if (stocks.value && stocks.value.length === 0) {
    showPopover.value = true
  }
  for (let code of stocks.value) {
    Greet(code).then(result => {
      updateData(result)
    })
  }
}


function GetSortKey(sort, code) {
  return padStart(sort, 8, '0') + "_" + code
}

function onSelect(item) {
  ////console.log("onSelect",item)

  if (item.indexOf("-") > 0) {
    item = item.split("-")[1].toLowerCase()
  }
  if (item.indexOf(".") > 0) {
    data.code = item.split(".")[1].toLowerCase() + item.split(".")[0]
  }

}

function openCenteredWindow(url, width, height) {
  const left = (window.screen.width - width) / 2;
  const top = (window.screen.height - height) / 2;
  Environment().then(env => {
    switch (env.platform) {
      case 'windows':
        window.open(
            url,
            'centeredWindow',
            `width=${width},height=${height},left=${left},top=${top},location=no,menubar=no,toolbar=no,display=standalone`
        )
        break
      default :
        OpenURL(url)
        break
    }
  })


  //
  // return window.open(
  //     url,
  //     'centeredWindow',
  //     `width=${width},height=${height},left=${left},top=${top}`
  // );
}

function search(code, name) {
  setTimeout(() => {
    //window.open("https://xueqiu.com/S/"+code)
    //window.open("https://www.cls.cn/stock?code="+code)
    //window.open("https://quote.eastmoney.com/"+code+".html")
    //window.open("https://finance.sina.com.cn/realstock/company/"+code+"/nc.shtml")
    //window.open("https://www.iwencai.com/unifiedwap/result?w=" + name)
    //window.open("https://www.iwencai.com/chat/?question="+code)

    openCenteredWindow("https://www.iwencai.com/unifiedwap/result?w=" + name, 1000, 800)

  }, 500)
}

function handleLongEntryPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongEntryPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.entryPrice = newPrice
  console.log('[DEBUG handleLongEntryPriceUpdate] after assignment, entryPrice:', currentStockTradingPrice.value.entryPrice)
  saveTradingPriceToBackend()
}

function handleLongStopLossPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongStopLossPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.stopLossPrice = newPrice
  saveTradingPriceToBackend()
}

function handleLongTakeProfitPriceUpdate(newPrice) {
  console.log('[DEBUG handleLongTakeProfitPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.takeProfitPrice = newPrice
  saveTradingPriceToBackend()
}

function handleCostPriceUpdate(newPrice) {
  console.log('[DEBUG handleCostPriceUpdate] called, newPrice:', newPrice, 'type:', typeof newPrice)
  currentStockTradingPrice.value.costPrice = newPrice
  saveTradingPriceToBackend()
}

function saveTradingPriceToBackend() {
  console.log('[DEBUG saveTradingPriceToBackend] called, stockCode:', currentStockTradingPrice.value.stockCode)
  if (!currentStockTradingPrice.value.stockCode) {
    console.log('[DEBUG saveTradingPriceToBackend] early return - no stockCode')
    return
  }
  const emCode = currentStockTradingPrice.value.stockCode
  const code = fromEastMoneyCode(emCode)
  if (!code) {
    console.warn('[saveTradingPriceToBackend] 无法转换股票代码:', emCode)
    return
  }
  const entryPrice = Number(currentStockTradingPrice.value.entryPrice) || 0
  const takeProfitPrice = Number(currentStockTradingPrice.value.takeProfitPrice) || 0
  const stopLossPrice = Number(currentStockTradingPrice.value.stopLossPrice) || 0
  const costPrice = Number(currentStockTradingPrice.value.costPrice) || 0
  console.log('[DEBUG saveTradingPriceToBackend] calling SetTradingPrice with:', code, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
  SetTradingPrice(
    code,
    entryPrice,
    takeProfitPrice,
    stopLossPrice,
    costPrice
  ).then(result => {
    console.log('[DEBUG saveTradingPriceToBackend] SetTradingPrice result:', result)
    if (result === '设置成功') {
      const emCode = currentStockTradingPrice.value.stockCode
      const internalCode = code
      const followItem = followList.value.find(item => item.StockCode === internalCode || item.StockCode === emCode)
      if (followItem) {
        followItem.EntryPrice = entryPrice
        followItem.TakeProfitPrice = takeProfitPrice
        followItem.StopLossPrice = stopLossPrice
        console.log('[DEBUG saveTradingPriceToBackend] updated followList item')
      }
    }
  }).catch(err => {
    console.error('[DEBUG saveTradingPriceToBackend] SetTradingPrice error:', err)
  })
}

function setStock(code, name) {
  let res = followList.value.filter(item => item.StockCode === code)
  ////console.log("res:",res)
  formModel.value.name = name
  formModel.value.code = code
  formModel.value.volume = res[0].Volume ? res[0].Volume : 0
  formModel.value.costPrice = res[0].CostPrice
  formModel.value.alarm = res[0].AlarmChangePercent
  formModel.value.alarmPrice = res[0].AlarmPrice
  formModel.value.sort = res[0].Sort
  formModel.value.cron = res[0].Cron
  formModel.value.entryPrice = res[0].EntryPrice || 0
  formModel.value.takeProfitPrice = res[0].TakeProfitPrice || 0
  formModel.value.stopLossPrice = res[0].StopLossPrice || 0
  modalShow.value = true
}

function clearFeishi() {
  //console.log("clearFeishi")
  clearInterval(feishiInterval.value)
}

function showFsChart(code, name) {
  data.name = name
  data.code = code
  const chart = echarts.init(kLineChartRef2.value);
  GetStockMinutePriceLineData(code, name).then(result => {
    // console.log("GetStockMinutePriceLineData", result)
    const priceData = result.priceData
    let category = []
    let price = []
    let openprice = 0
    let closeprice = 0
    let volume = []
    let volumeRate = []
    let min = 0
    let max = 0
    openprice = priceData[0].price
    closeprice = priceData[priceData.length - 1].price
    for (let i = 0; i < priceData.length; i++) {
      category.push(priceData[i].time)
      price.push(priceData[i].price)
      if (min === 0 || min > priceData[i].price) {
        min = priceData[i].price
      }
      if (max < priceData[i].price) {
        max = priceData[i].price
      }
      if (i > 0) {
        let b = priceData[i].volume - priceData[i - 1].volume
        volumeRate.push(((b - volume[i - 1]) / volume[i - 1] * 100).toFixed(2))
        volume.push(b)
      } else {
        volume.push(priceData[i].volume)
        volumeRate.push(0)
      }
    }

    let option = {
      title: {
        subtext: "[" + result.date + "] 开盘:" + openprice + " 最新:" + closeprice + " 最高:" + max + " 最低:" + min,
        left: 'center',
        top: '10',
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        }
      },
      legend: {
        data: ['股价', '成交量'],
        //orient: 'vertical',
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
        right: 50,
      },
      darkMode: data.darkTheme,
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          animation: false,
          label: {
            backgroundColor: '#505765'
          }
        }
      },
      axisPointer: {
        link: [
          {
            xAxisIndex: 'all'
          }
        ],
        label: {
          backgroundColor: '#888'
        }
      },
      xAxis: [
        {
          type: 'category',
          data: category,
          axisLabel: {
            show: false
          }
        },
        {
          gridIndex: 1,
          type: 'category',
          data: category,
        },
      ],
      grid: [
        {
          left: '8%',
          right: '8%',
          height: '50%',
        },
        {
          left: '8%',
          right: '8%',
          top: '70%',
          height: '15%'
        },
      ],
      yAxis: [
        {
          axisLine: {
            show: true
          },
          splitLine: {
            show: false
          },
          name: "股价",
          min: (min - min * 0.01).toFixed(2),
          max: (max + max * 0.01).toFixed(2),
          minInterval: 0.01,
          type: 'value'
        },
        {
          gridIndex: 1,
          axisLine: {
            show: true
          },
          splitLine: {
            show: false
          },
          name: "成交量",
          type: 'value',
        },
      ],
      visualMap: {
        type: 'piecewise',
        seriesIndex: 0,
        top: 0,
        left: 10,
        orient: 'horizontal',
        textStyle: {
          color: data.darkTheme ? '#fff' : '#456'
        },
        pieces: [
          {
            text: '低于开盘价',
            gt: 0,
            lte: openprice,
            color: '#31F113',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          },
          {
            text: '大于开盘价小于收盘价',
            gt: openprice,
            lte: closeprice,
            color: '#1651EF',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          },
          {
            text: '大于收盘价',
            gt: closeprice,
            color: '#AC3B2A',
            textStyle: {
              color: data.darkTheme ? '#fff' : '#456'
            },
          }
        ],
      },
      series: [
        {
          name: "股价",
          data: price,
          type: 'line',
          smooth: false,
          showSymbol: false,
          lineStyle: {
            width: 3
          },
          markPoint: {
            symbol: 'arrow',
            symbolRotate: 90,
            symbolSize: [10, 20],
            symbolOffset: [10, 0],
            itemStyle: {
              color: '#FC290D'
            },
            label: {
              position: 'right',
            },
            data: [
              {type: 'max', name: 'Max'},
              {type: 'min', name: 'Min'}
            ]
          },
          markLine: {
            symbol: 'none',
            data: [
              {type: 'average', name: 'Average'},
              {
                lineStyle: {
                  color: '#FFCB00',
                  width: 0.5
                },
                yAxis: openprice,
                name: '开盘价'
              },
              {
                yAxis: closeprice,
                symbol: 'none',
                lineStyle: {
                  color: 'red',
                  width: 0.5
                },
              }
            ]
          },
        },
        {
          xAxisIndex: 1,
          yAxisIndex: 1,
          name: "成交量",
          data: volume,
          type: 'bar',
        },

      ]
    };
    chart.setOption(option);
  })
}

function showFenshi(code, name, changePercent) {
  data.code = code
  data.name = name
  data.changePercent = changePercent
  data.fenshiURL = 'http://image.sinajs.cn/newchart/min/n/' + data.code + '.gif' + "?t=" + Date.now()

  if (code.startsWith('hk')) {
    data.fenshiURL = 'http://image.sinajs.cn/newchart/hk_stock/min/' + data.code.replace("hk", "") + '.gif' + "?t=" + Date.now()
  }
  if (code.startsWith('gb_')) {
    data.fenshiURL = 'http://image.sinajs.cn/newchart/usstock/min/' + data.code.replace("gb_", "") + '.gif' + "?t=" + Date.now()
  }

  modalShow2.value = true
}

function handleFeishi() {
  showFsChart(data.code, data.name);
  feishiInterval.value = setInterval(() => {
    showFsChart(data.code, data.name);
  }, 1000 * 10)
}

// 渲染 gotdx 分时图（价格 + 均价 + 昨收线 + 成交量）
function renderTdxMinuteChart(bundle) {
  if (!bundle || !bundle.items || bundle.items.length === 0 || !tdxTransactionChartRef.value) {
    return
  }
  if (tdxTransactionChart.value) {
    tdxTransactionChart.value.dispose()
    tdxTransactionChart.value = null
  }
  const chart = echarts.init(tdxTransactionChartRef.value)
  tdxTransactionChart.value = chart

  const category = []
  const price = []
  const avg = []
  const vol = []
  let min = 0, max = 0
  for (let i = 0; i < bundle.items.length; i++) {
    const it = bundle.items[i]
    category.push(it.time)
    price.push(it.price)
    avg.push(it.avg)
    vol.push(it.vol)
    if (i === 0) {
      min = it.price
      max = it.price
    } else {
      if (it.price < min) min = it.price
      if (it.price > max) max = it.price
    }
  }
  // 给上下留一点空间
  const span = (max - min) || (max * 0.01 || 1)
  const yMin = (min - span * 0.1).toFixed(2)
  const yMax = (max + span * 0.1).toFixed(2)
  // 昨收基准线
  const preClose = bundle.preClose || 0

  const option = {
    title: {
      subtext: '[' + (bundle.date || '') + '] 昨收:' + preClose + ' 今开:' + (bundle.open || 0) +
        ' 最高:' + (bundle.high || 0) + ' 最低:' + (bundle.low || 0) + ' 收盘:' + (bundle.close || 0) +
        ' 总量:' + (bundle.vol || 0) + ' 总额:' + (bundle.amount || 0).toFixed(2),
      left: 'center',
      top: '6',
      subtextStyle: { color: data.darkTheme ? '#ccc' : '#456', fontSize: 12 }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', label: { backgroundColor: '#505765' } }
    },
    legend: { data: ['价格', '均价', '成交量'], right: 30, top: 6 },
    darkMode: data.darkTheme,
    axisPointer: { link: [{ xAxisIndex: 'all' }], label: { backgroundColor: '#888' } },
    grid: [
      { left: '8%', right: '8%', top: '20%', height: '50%' },
      { left: '8%', right: '8%', top: '76%', height: '16%' }
    ],
    xAxis: [
      { type: 'category', data: category, axisLabel: { show: false } },
      { gridIndex: 1, type: 'category', data: category }
    ],
    yAxis: [
      {
        scale: true,
        min: yMin,
        max: yMax,
        minInterval: 0.01,
        type: 'value',
        name: '价格',
        splitLine: { show: false }
      },
      { gridIndex: 1, type: 'value', name: '量', splitLine: { show: false } }
    ],
    series: [
      {
        name: '价格',
        type: 'line',
        data: price,
        showSymbol: false,
        smooth: false,
        lineStyle: { width: 2 },
        markLine: {
          symbol: 'none',
          data: [
            { type: 'max', name: '最高' },
            { type: 'min', name: '最低' },
            {
              yAxis: preClose,
              name: '昨收',
              lineStyle: { color: '#FFCB00', width: 0.8, type: 'dashed' },
              label: { formatter: '昨收' }
            }
          ]
        }
      },
      { name: '均价', type: 'line', data: avg, showSymbol: false, lineStyle: { width: 1, color: '#FF9900' } },
      { name: '成交量', type: 'bar', xAxisIndex: 1, yAxisIndex: 1, data: vol }
    ]
  }
  chart.setOption(option)
}

// 渲染多日分时图：将各日数据点按时间顺序拼接为连续时间轴（与净流入折线图一致），
// x 轴标签格式 "MM-DD HH:MM"，价格/均价为连续折线，成交量在下方子图展示
function renderTdxMultiDayMinuteChart(bundles) {
  if (!tdxTransactionChartRef.value) return
  if (!bundles || bundles.length === 0) return
  if (tdxTransactionChart.value) {
    tdxTransactionChart.value.dispose()
    tdxTransactionChart.value = null
  }
  const chart = echarts.init(tdxTransactionChartRef.value)
  tdxTransactionChart.value = chart

  // 拼接所有日的数据点，x 轴标签 "MM-DD HH:MM"
  const category = []
  const price = []
  const avg = []
  const vol = []
  let min = 0, max = 0
  let hasData = false
  for (const b of bundles) {
    const items = (b.bundle && b.bundle.items) || []
    const parts = b.dateStr.split('-')
    const md = parts.length === 3 ? `${parts[1]}-${parts[2]}` : b.dateStr
    for (const it of items) {
      category.push(`${md} ${it.time || ''}`)
      price.push(it.price)
      avg.push(it.avg)
      vol.push(it.vol)
      if (!hasData) { min = it.price; max = it.price; hasData = true }
      else { if (it.price < min) min = it.price; if (it.price > max) max = it.price }
    }
  }
  if (!hasData) return

  const span = (max - min) || (max * 0.01 || 1)
  const yMin = (min - span * 0.1).toFixed(2)
  const yMax = (max + span * 0.1).toFixed(2)
  // x 轴标签稀疏化
  const labelInterval = category.length > 8 ? Math.floor(category.length / 8) : 0

  const dateRangeText = `${bundles[0].dateStr} ~ ${bundles[bundles.length - 1].dateStr}（共 ${bundles.length} 日）`

  const option = {
    title: {
      text: '多日分时走势',
      subtext: dateRangeText,
      left: 'center',
      top: 4,
      textStyle: { fontSize: 13, color: data.darkTheme ? '#ccc' : '#333' },
      subtextStyle: { color: data.darkTheme ? '#ccc' : '#456', fontSize: 12 }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', label: { backgroundColor: '#505765' } }
    },
    legend: { data: ['价格', '均价', '成交量'], right: 30, top: 6 },
    darkMode: data.darkTheme,
    axisPointer: { link: [{ xAxisIndex: 'all' }], label: { backgroundColor: '#888' } },
    grid: [
      { left: '8%', right: '8%', top: '20%', height: '50%' },
      { left: '8%', right: '8%', top: '76%', height: '16%' }
    ],
    xAxis: [
      { type: 'category', data: category, axisLabel: { show: false } },
      { gridIndex: 1, type: 'category', data: category, axisLabel: { interval: labelInterval, fontSize: 10 } }
    ],
    yAxis: [
      { scale: true, min: yMin, max: yMax, minInterval: 0.01, type: 'value', name: '价格', splitLine: { show: false } },
      { gridIndex: 1, type: 'value', name: '量', splitLine: { show: false } }
    ],
    series: [
      {
        name: '价格',
        type: 'line',
        data: price,
        showSymbol: false,
        smooth: false,
        lineStyle: { width: 2 }
      },
      { name: '均价', type: 'line', data: avg, showSymbol: false, lineStyle: { width: 1, color: '#FF9900' } },
      { name: '成交量', type: 'bar', xAxisIndex: 1, yAxisIndex: 1, data: vol }
    ]
  }
  chart.setOption(option)
}

// 渲染各档位累计净流入金额变化折线图
function renderTdxNetInflowChart() {
  if (!tdxNetInflowChartRef.value) return
  if (tdxNetInflowChart.value) {
    tdxNetInflowChart.value.dispose()
    tdxNetInflowChart.value = null
  }
  const chart = echarts.init(tdxNetInflowChartRef.value)
  tdxNetInflowChart.value = chart
  const { xData, series } = tdxNetInflowSeries.value
  if (!xData || xData.length === 0) {
    chart.setOption({ title: { text: '暂无数据', left: 'center', top: 'middle', textStyle: { color: '#999', fontSize: 13 } } })
    return
  }
  const names = ['超大单', '大单', '中单', '小单']
  const colors = ['#d03050', '#f0a020', '#2080f0', '#909399']
  // x 轴标签稀疏化，避免拥挤
  const labelInterval = xData.length > 8 ? Math.floor(xData.length / 8) : 0
  const option = {
    title: { text: '各档位累计净流入金额变化', left: 'center', top: 4, textStyle: { fontSize: 13, color: data.darkTheme ? '#ccc' : '#333' } },
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        if (!params || params.length === 0) return ''
        let html = params[0].axisValue + '<br/>'
        for (const p of params) {
          const wan = (p.value / 10000).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
          html += `${p.marker}${p.seriesName}: ${wan} 万<br/>`
        }
        return html
      }
    },
    legend: { data: names, top: 26, textStyle: { color: data.darkTheme ? '#ccc' : '#456' } },
    darkMode: data.darkTheme,
    grid: { left: '10%', right: '6%', top: 60, bottom: 30 },
    xAxis: {
      type: 'category',
      data: xData,
      axisLabel: { interval: labelInterval, fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      name: '净流入(万)',
      axisLabel: { formatter: (v) => (v / 10000).toFixed(0) },
      splitLine: { lineStyle: { type: 'dashed', opacity: 0.3 } }
    },
    series: names.map((name, i) => ({
      name,
      type: 'line',
      data: series[i],
      smooth: false,
      showSymbol: false,
      lineStyle: { width: 2, color: colors[i] },
      itemStyle: { color: colors[i] },
      emphasis: { focus: 'series' }
    }))
  }
  chart.setOption(option)
}
// 拉取实时价格与涨跌幅
function fetchRealTimePrice() {
  if (!data.code) return
  GetStockRealTimePrice(data.code).then(res => {
    if (res && res.code === 0) {
      tdxRealTimeInfo.value = {
        price: res.price || 0,
        preClose: res.preClose || 0,
        changePercent: res.changePercent || 0
      }
    }
  }).catch(() => {})
}
// 启动自动刷新（10 秒轮询；仅交易日 + 选中范围含今天时刷新成交明细，价格始终刷新）
function startAutoRefresh() {
  stopAutoRefresh()
  if (!tdxAutoRefresh.value) return
  tdxAutoRefreshTimer = setInterval(() => {
    // 始终刷新实时价格
    fetchRealTimePrice()
    // 仅当选中范围含今天且今天为交易日时，刷新成交明细
    const range = tdxSelectedDateRange.value || []
    if (range.length >= 2 && range[0] != null && range[1] != null) {
      const todayStr = formatTdxDate(Date.now())
      const startStr = formatTdxDate(range[0])
      const endStr = formatTdxDate(range[1])
      if (todayStr >= startStr && todayStr <= endStr && todayIsTradingDay.value) {
        // Auto refresh must bypass today's 5-minute backend cache. Historical
        // dates in a multi-day range can still use their completed caches.
        loadTdxTransactionByDate(true, true)
      }
    }
  }, 10000)
}
function stopAutoRefresh() {
  if (tdxAutoRefreshTimer) {
    clearInterval(tdxAutoRefreshTimer)
    tdxAutoRefreshTimer = null
  }
}
// 切换自动刷新开关
function toggleAutoRefresh() {
  tdxAutoRefresh.value = !tdxAutoRefresh.value
  if (tdxAutoRefresh.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}
function showTransactionDetail(code, name) {
  data.code = code
  data.name = name
  tdxMinuteBundle.value = null
  tdxMinuteBundleList.value = []
  tdxTransactionList.value = []
  tdxAmountFilter.value = 0
  tdxTransactionPagination.value.itemCount = 0
  tdxRealTimeInfo.value = { price: 0, preClose: 0, changePercent: 0 }
  modalShow7.value = true
  // 拉取实时价格与涨跌幅
  fetchRealTimePrice()
  // 先刷新今日交易日状态（后端通过 timor.tech 节假日 API 准确判断）
  refreshTodayTradingDayStatus().then(() => {
    if (todayIsTradingDay.value) {
      // 今天是交易日：默认选今天（走当日实时接口，盘中可看实时分时/成交）
      const todayTs = startOfTodayTs()
      tdxSelectedDateRange.value = [todayTs, todayTs]
      onTdxDateRangeChange([todayTs, todayTs])
    } else {
      // 今天非交易日：选最近交易日（GetLatestTradingDay 在非交易日返回上一交易日）
      return GetLatestTradingDay().then(latestDay => {
        const ts = startOfDayTs(new Date(latestDay.replace(/-/g, '/')).getTime())
        tdxSelectedDateRange.value = [ts, ts]
        onTdxDateRangeChange([ts, ts])
      })
    }
  }).catch(() => {
    // fallback：使用今天
    const todayTs = startOfTodayTs()
    tdxSelectedDateRange.value = [todayTs, todayTs]
    onTdxDateRangeChange([todayTs, todayTs])
  }).finally(() => {
    // 启动自动刷新（10 秒轮询实时价格 + 成交明细）
    startAutoRefresh()
  })
}

// 按当前选中的日期范围加载分笔成交明细。
// 单日：今天走 GetAllTdxTransactionData，历史日期走 GetHistoryTdxTransactionData。
// 多日：枚举范围内每一天，按日调用对应接口，拼接所有成交明细（按日期升序），每条标记 dateStr。
// skipCache=true 强制刷新所有日期的缓存。
function loadTdxTransactionByDate(skipCache, refreshTodayOnly = false) {
  const code = data.code
  if (!code) return
  if (tdxTransactionLoading.value) return
  const range = tdxSelectedDateRange.value || []
  if (!range || range.length < 2 || range[0] == null || range[1] == null) return
  const days = enumerateDateRange(range[0], range[1])
  if (days.length === 0) return

  tdxTransactionLoading.value = true
  // 每个日期并发拉取，最后按日期顺序合并
  const promises = days.map(day => {
    const forceRefresh = skipCache && (!refreshTodayOnly || day.isToday)
    const p = day.isToday
      ? (forceRefresh ? RefreshAllTdxTransactionData(code) : GetAllTdxTransactionData(code))
      : (forceRefresh ? RefreshHistoryTdxTransactionData(code, day.dateStr) : GetHistoryTdxTransactionData(code, day.dateStr))
    return p.then(list => ({ day, list: list || [] }))
  })
  Promise.all(promises).then(results => {
    // 按日期升序拼接（days 本身已升序）
    let keyIdx = 0
    const combined = []
    for (const { day, list } of results) {
      for (const item of list) {
        combined.push({ ...item, key: keyIdx++, dateStr: day.dateStr })
      }
    }
    // 安全兜底：按 (dateStr, time) 升序排序，确保时间轴从左到右递增
    // 防止后端缓存旧数据或协议返回顺序不一致导致图表倒序
    combined.sort((a, b) => {
      const da = a.dateStr || ''
      const db = b.dateStr || ''
      if (da !== db) return da < db ? -1 : 1
      const ta = a.time || ''
      const tb = b.time || ''
      return ta < tb ? -1 : (ta > tb ? 1 : 0)
    })
    tdxTransactionList.value = combined
    tdxTransactionPagination.value.page = 1
    tdxTransactionPagination.value.itemCount = combined.length
    nextTick(() => renderTdxNetInflowChart())
  }).catch(err => {
    message.error('分笔成交明细加载失败：' + (err && err.message ? err.message : err))
  }).finally(() => {
    tdxTransactionLoading.value = false
  })
}

// 日期范围切换：清空数据并重新加载分时图与分笔成交明细
// 单日范围：渲染该日分时图（今天走 GetTdxMinuteTimeData，历史走 GetHistoryTdxMinuteTimeData）
// 多日范围：并行拉取每日分时数据，叠加渲染为多日对比图（每日一条价格折线）
function onTdxDateRangeChange(range) {
  if (!range || range.length < 2 || range[0] == null || range[1] == null) return
  // daterange 默认返回带时分秒的时间戳，对齐到当天 0 点保证 isToday/format 判断稳定
  const aligned = [startOfDayTs(range[0]), startOfDayTs(range[1])]
  tdxSelectedDateRange.value = aligned
  tdxTransactionList.value = []
  tdxAmountFilter.value = 0
  tdxTransactionPagination.value.itemCount = 0

  if (tdxIsMultiDay.value) {
    // 多日：并行拉取每日分时数据，叠加渲染为对比图
    const days = enumerateDateRange(aligned[0], aligned[1])
    tdxMinuteBundle.value = null
    tdxMinuteBundleList.value = []
    const minutePromises = days.map(day => {
      const p = day.isToday
        ? GetTdxMinuteTimeData(data.code)
        : GetHistoryTdxMinuteTimeData(data.code, day.dateStr)
      return p.then(bundle => ({ dateStr: day.dateStr, bundle })).catch(() => ({ dateStr: day.dateStr, bundle: null }))
    })
    Promise.all(minutePromises).then(results => {
      const valid = results.filter(r => r.bundle && r.bundle.items && r.bundle.items.length > 0)
      tdxMinuteBundleList.value = valid
      if (valid.length > 0) {
        nextTick(() => renderTdxMultiDayMinuteChart(valid))
      } else {
        if (tdxTransactionChart.value) {
          tdxTransactionChart.value.dispose()
          tdxTransactionChart.value = null
        }
      }
    }).catch(err => {
      message.error('多日分时数据加载失败：' + (err && err.message ? err.message : err))
    })
  } else {
    // 单日：渲染分时图（今天且为交易日走当日接口，否则走历史分时接口）
    const ts = aligned[0]
    const dateStr = formatTdxDate(ts)
    const minutePromise = shouldUseCurrentDayApi(ts)
      ? GetTdxMinuteTimeData(data.code)
      : GetHistoryTdxMinuteTimeData(data.code, dateStr)
    minutePromise.then(bundle => {
      tdxMinuteBundle.value = bundle
      nextTick(() => renderTdxMinuteChart(bundle))
    }).catch(err => {
      message.error('分时数据加载失败：' + (err && err.message ? err.message : err))
      if (tdxTransactionChart.value) {
        tdxTransactionChart.value.dispose()
        tdxTransactionChart.value = null
      }
      tdxMinuteBundle.value = null
    })
  }
  // 分笔成交明细（按当前选中日期范围加载）
  loadTdxTransactionByDate(false)
}

function handleTdxTransactionModalClose() {
  stopAutoRefresh()
  if (tdxTransactionChart.value) {
    tdxTransactionChart.value.dispose()
    tdxTransactionChart.value = null
  }
  if (tdxNetInflowChart.value) {
    tdxNetInflowChart.value.dispose()
    tdxNetInflowChart.value = null
  }
  tdxMinuteBundle.value = null
  tdxMinuteBundleList.value = []
  tdxTransactionList.value = []
  tdxAmountFilter.value = 0
  tdxRealTimeInfo.value = { price: 0, preClose: 0, changePercent: 0 }
}

// 手动刷新分时明细（按当前选中日期强制刷新）
function refreshTdxTransaction() {
  loadTdxTransactionByDate(true)
  message.success('已刷新分笔成交明细')
}

function calculateMA(dayCount, values) {
  var result = [];
  for (var i = 0, len = values.length; i < len; i++) {
    if (i < dayCount) {
      result.push('-');
      continue;
    }
    var sum = 0;
    for (var j = 0; j < dayCount; j++) {
      sum += +values[i - j][1];
    }
    result.push((sum / dayCount).toFixed(2));
  }
  return result;
}

function handleKLine() {
  GetStockKLine(data.code, data.name, 365).then(result => {
    //console.log("GetStockKLine",result)
    const chart = echarts.init(kLineChartRef.value);
    const categoryData = [];
    const values = [];
    const volumns = [];
    for (let i = 0; i < result.length; i++) {
      let resultElement = result[i]
      //console.log("resultElement:{}",resultElement)
      categoryData.push(resultElement.day)
      let flag = resultElement.close > resultElement.open ? 1 : -1
      values.push([
        resultElement.open,
        resultElement.close,
        resultElement.low,
        resultElement.high
      ])
      volumns.push([i, resultElement.volume / 10000, flag])
    }
    ////console.log("categoryData",categoryData)
    ////console.log("values",values)
    let option = {
      darkMode: data.darkTheme,
      //backgroundColor: '#1c1c1c',
      // color:['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc'],
      animation: false,
      legend: {
        bottom: 10,
        left: 'center',
        data: ['日K', 'MA5', 'MA10', 'MA20', 'MA30'],
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          lineStyle: {
            color: '#376df4',
            width: 1,
            opacity: 1
          }
        },
        borderWidth: 2,
        borderColor: data.darkTheme ? '#456' : '#ccc',
        backgroundColor: data.darkTheme ? '#456' : '#fff',
        padding: 10,
        textStyle: {
          color: data.darkTheme ? '#ccc' : '#456'
        },
        formatter: function (params) {//修改鼠标划过显示为中文
          //console.log("params",params)
          let volum = params[5].data;//ma5的值
          let ma5 = params[1].data;//ma5的值
          let ma10 = params[2].data;//ma10的值
          let ma20 = params[3].data;//ma20的值
          let ma30 = params[4].data;//ma30的值
          params = params[0];//开盘收盘最低最高数据汇总
          let currentItemData = params.data;

          return params.name + '<br>' +
              '开盘:' + currentItemData[1] + '<br>' +
              '收盘:' + currentItemData[2] + '<br>' +
              '最低:' + currentItemData[3] + '<br>' +
              '最高:' + currentItemData[4] + '<br>' +
              '成交量(万手):' + volum[1] + '<br>' +
              'MA5日均线:' + ma5 + '<br>' +
              'MA10日均线:' + ma10 + '<br>' +
              'MA20日均线:' + ma20 + '<br>' +
              'MA30日均线:' + ma30
        }
        // position: function (pos, params, el, elRect, size) {
        //   const obj = {
        //     top: 10
        //   };
        //   obj[['left', 'right'][+(pos[0] < size.viewSize[0] / 2)]] = 30;
        //   return obj;
        // }
        // extraCssText: 'width: 170px'
      },
      axisPointer: {
        link: [
          {
            xAxisIndex: 'all'
          }
        ],
        label: {
          backgroundColor: '#888'
        }
      },
      visualMap: {
        show: false,
        seriesIndex: 5,
        dimension: 2,
        pieces: [
          {
            value: -1,
            color: downColor
          },
          {
            value: 1,
            color: upColor
          }
        ]
      },
      grid: [
        {
          left: '10%',
          right: '8%',
          height: '50%',
        },
        {
          left: '10%',
          right: '8%',
          top: '63%',
          height: '16%'
        }
      ],
      xAxis: [
        {
          type: 'category',
          data: categoryData,
          boundaryGap: false,
          axisLine: {onZero: false},
          splitLine: {show: false},
          min: 'dataMin',
          max: 'dataMax',
          axisPointer: {
            z: 100
          }
        },
        {
          type: 'category',
          gridIndex: 1,
          data: categoryData,
          boundaryGap: false,
          axisLine: {onZero: false},
          axisTick: {show: false},
          splitLine: {show: false},
          axisLabel: {show: false},
          min: 'dataMin',
          max: 'dataMax'
        }
      ],
      yAxis: [
        {
          scale: true,
          splitArea: {
            show: true
          }
        },
        {
          scale: true,
          gridIndex: 1,
          splitNumber: 2,
          axisLabel: {show: false},
          axisLine: {show: false},
          axisTick: {show: false},
          splitLine: {show: false}
        }
      ],
      dataZoom: [
        {
          type: 'inside',
          xAxisIndex: [0, 1],
          start: 86,
          end: 100
        },
        {
          show: true,
          xAxisIndex: [0, 1],
          type: 'slider',
          top: '85%',
          start: 86,
          end: 100
        }
      ],

      series: [
        {
          name: '日K',
          type: 'candlestick',
          data: values,
          itemStyle: {
            color: upColor,
            color0: downColor,
            // borderColor: upBorderColor,
            // borderColor0: downBorderColor
          },
          markPoint: {
            label: {
              formatter: function (param) {
                return param != null ? param.value + '' : '';
              }
            },
            data: [
              {
                name: '最高',
                type: 'max',
                valueDim: 'highest'
              },
              {
                name: '最低',
                type: 'min',
                valueDim: 'lowest'
              },
              {
                name: '平均收盘价',
                type: 'average',
                valueDim: 'close'
              }
            ],
            tooltip: {
              formatter: function (param) {
                return param.name + '<br>' + (param.data.coord || '');
              }
            }
          },
          markLine: {
            symbol: ['none', 'none'],
            data: [
              [
                {
                  name: 'from lowest to highest',
                  type: 'min',
                  valueDim: 'lowest',
                  symbol: 'circle',
                  symbolSize: 10,
                  label: {
                    show: false
                  },
                  emphasis: {
                    label: {
                      show: false
                    }
                  }
                },
                {
                  type: 'max',
                  valueDim: 'highest',
                  symbol: 'circle',
                  symbolSize: 10,
                  label: {
                    show: false
                  },
                  emphasis: {
                    label: {
                      show: false
                    }
                  }
                }
              ],
              {
                name: 'min line on close',
                type: 'min',
                valueDim: 'close'
              },
              {
                name: 'max line on close',
                type: 'max',
                valueDim: 'close'
              }
            ]
          }
        },
        {
          name: 'MA5',
          type: 'line',
          data: calculateMA(5, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA10',
          type: 'line',
          data: calculateMA(10, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA20',
          type: 'line',
          data: calculateMA(20, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: 'MA30',
          type: 'line',
          data: calculateMA(30, values),
          smooth: true,
          showSymbol: false,
          lineStyle: {
            opacity: 0.6
          }
        },
        {
          name: '成交量(手)',
          type: 'bar',
          xAxisIndex: 1,
          yAxisIndex: 1,
          itemStyle: {
            color: '#7fbe9e'
          },
          data: volumns
        }
      ]
    };
    chart.setOption(option);
    chart.on('click', {seriesName: '日K'}, function (params) {
      //console.log("click:",params);
    });
  })
}

function showMoney(code, name) {
  data.code = code
  data.name = name
  modalShow5.value = true
}

/** 新浪/应用内代码转为东方财富接口常用格式（如 600519.SH） */
function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

/** 东方财富格式转回应用内部代码格式（如 000001.SZ → sh000001） */
function fromEastMoneyCode(emCode) {
  if (!emCode) return ''
  const c = String(emCode).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3)
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3)
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3)
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.US')) return 'us' + c.slice(0, -3).toLowerCase()
  return c.toLowerCase()
}

async function refreshEffectiveVip() {
  try {
    const r = await GetEffectiveSponsorVip()
    const active = !!r?.active
    const lvl = Number(r?.vipLevel ?? 0)
    vipLevel.value = active && !Number.isNaN(lvl) ? lvl : 0
  } catch (_) {
    vipLevel.value = 0
  }
}

async function showLightweightKline(code, name) {
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  lwKlineCode.value = em
  lwKlineName.value = name || ''

  // 刷新自选列表，确保获取最新的交易价格数据
  try {
    const list = await GetFollowList(currentGroupId.value)
    followList.value = list || []
  } catch (e) {
    console.error('[showLightweightKline] 刷新自选列表失败:', e)
  }

  // 从自选列表中获取交易价格
  // lwKlineCode 格式为 000001.SZ，followList 中的 StockCode 格式为 sh000001
  // 需要进行格式转换来匹配
  let followListCode = code
  if (code.startsWith('sh') || code.startsWith('sz') || code.startsWith('bj') || code.startsWith('hk')) {
    // 如果是 sh000001 格式，转换为东方财富格式
    const market = code.slice(0, 2).toUpperCase()
    const stockNum = code.slice(2)
    followListCode = stockNum + '.' + market
  }

  const stockInfo = followList.value.find(item => item.StockCode === code || item.StockCode === followListCode)
  if (stockInfo) {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value  // 使用东方财富格式
    currentStockTradingPrice.value.costPrice = stockInfo.CostPrice || 0
    currentStockTradingPrice.value.entryPrice = stockInfo.EntryPrice || 0
    currentStockTradingPrice.value.takeProfitPrice = stockInfo.TakeProfitPrice || 0
    currentStockTradingPrice.value.stopLossPrice = stockInfo.StopLossPrice || 0
  } else {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value
    currentStockTradingPrice.value.costPrice = 0
    currentStockTradingPrice.value.entryPrice = 0
    currentStockTradingPrice.value.takeProfitPrice = 0
    currentStockTradingPrice.value.stopLossPrice = 0
  }

  await refreshEffectiveVip()
  // 检查 VIP 权限：有效期内 VIP2 及以上（与 AI 助手 Web 端校验一致）
  if (vipLevel.value < 2) {
    message.warning('多周期 K 线仅限 VIP2 及以上用户使用，您当前权限不足，将在 10 秒后自动关闭')
    lwKlineCode.value = em
    lwKlineName.value = name || ''
    modalShow6.value = true
    // 10 秒后自动关闭
    klineAutoCloseTimer.value = setTimeout(() => {
      modalShow6.value = false
      message.info('权限不足，多周期 K 线已自动关闭')
    }, 10000)
    return
  }
  modalShow6.value = true
}

function showK(code, name) {
  data.code = code
  data.name = name
  data.kURL = 'http://image.sinajs.cn/newchart/daily/n/' + data.code + '.gif' + "?t=" + Date.now()
  if (code.startsWith('hk')) {
    data.kURL = 'http://image.sinajs.cn/newchart/hk_stock/daily/' + data.code.replace("hk", "") + '.gif' + "?t=" + Date.now()
  }
  if (code.startsWith('gb_')) {
    data.kURL = 'http://image.sinajs.cn/newchart/usstock/daily/' + data.code.replace("gb_", "") + '.gif' + "?t=" + Date.now()
  }
  modalShow3.value = true
  //https://image.sinajs.cn/newchart/usstock/daily/dji.gif
  //https://image.sinajs.cn/newchart/hk_stock/daily/06030.gif?1740729404273
}


function updateCostPriceAndVolumeNew(code, price, volume, alarm, formModel) {
  if (formModel.sort) {
    SetStockSort(formModel.sort, code).then(result => {
      //message.success(result)
    })
  }
  if (formModel.cron) {
    SetStockAICron(formModel.cron, code).then(result => {
      //message.success(result)
    })
  }

  if (alarm || formModel.alarmPrice) {
    SetAlarmChangePercent(alarm, formModel.alarmPrice, code).then(result => {
      //message.success(result)
    })
  }
  
  // 保存交易价格（开仓价、止盈价、止损价、成本价）
  if (formModel.entryPrice || formModel.takeProfitPrice || formModel.stopLossPrice || formModel.costPrice) {
    SetTradingPrice(code, formModel.entryPrice || 0, formModel.takeProfitPrice || 0, formModel.stopLossPrice || 0, formModel.costPrice || 0).then(result => {
      //message.success(result)
    })
  }
  
  SetCostPriceAndVolume(code, price, volume).then(result => {
    modalShow.value = false
    message.success(result)
    GetFollowList(currentGroupId.value).then(result => {
      followList.value = result
      stocks.value = []
      for (const followedStock of result) {
        if (!stocks.value.includes(followedStock.StockCode)) {
          stocks.value.push(followedStock.StockCode)
        }
      }
      monitor()
      message.destroyAll()
    })
  })
}

function fullscreen() {
  if (data.fullscreen) {
    WindowUnfullscreen()
  } else {
    WindowFullscreen()
  }
  data.fullscreen = !data.fullscreen
}


//type 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
function SendMessage(result, type) {
  let typeName = getTypeName(type)
  let img = 'http://image.sinajs.cn/newchart/min/n/' + result["股票代码"] + '.gif' + "?t=" + Date.now()
  let markdown = "### go-stock [" + typeName + "]\n\n" +
      "### " + result["股票名称"] + "(" + result["股票代码"] + ")\n" +
      "- 当前价格: " + result["当前价格"] + "  " + result.changePercent + "%\n" +
      "- 最高价: " + result["今日最高价"] + "  " + result.highRate + "\n" +
      "- 最低价: " + result["今日最低价"] + "  " + result.lowRate + "\n" +
      "- 昨收价: " + result["昨日收盘价"] + "\n" +
      "- 今开价: " + result["今日开盘价"] + "\n" +
      "- 成本价: " + result.costPrice + "  " + result.profit + "%  " + result.profitAmount + " ¥\n" +
      "- 成本数量: " + result.costVolume + "股\n" +
      "- 日期: " + result["日期"] + "  " + result["时间"] + "\n\n" +
      "![image](" + img + ")\n"
  let title = result["股票名称"] + "(" + result["股票代码"] + ") " + result["当前价格"] + " " + result.changePercent

  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"[' + typeName + "]" + title + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'
  // SendDingDingMessage(msg,result["股票代码"])
  SendDingDingMessageByType(msg, result["股票代码"], type)
}

const priceLineAlertCache = new Map()

function checkPriceLineAlerts(result) {
  const code = result["股票代码"]
  const price = result["当前价格"]
  if (!price || price <= 0) return

  const followedStock = followList.value.find(s => {
    const sCode = s.StockCode || ''
    return sCode === code || sCode === 'sh' + code || sCode === 'sz' + code ||
           sCode === code.replace('sh', '').replace('sz', '') ||
           (sCode.length > 2 && code.length > 2 && sCode.includes(code.slice(2)))
  })

  if (!followedStock) return

  const alerts = []
  let triggeredType = 0
  if (followedStock.EntryPrice > 0) {
    const diff = ((price - followedStock.EntryPrice) / followedStock.EntryPrice * 100).toFixed(2)
    alerts.push(`开仓价: ${followedStock.EntryPrice} (${diff >= 0 ? '+' : ''}${diff}%)`)
  }
  if (followedStock.TakeProfitPrice > 0) {
    if (price >= followedStock.TakeProfitPrice) {
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} ⚠️ 已触及`)
      triggeredType = 4
    } else {
      const diff = ((followedStock.TakeProfitPrice - price) / followedStock.TakeProfitPrice * 100).toFixed(2)
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} (距离 ${diff}%)`)
    }
  }
  if (followedStock.StopLossPrice > 0) {
    if (price <= followedStock.StopLossPrice) {
      alerts.push(`止损价: ${followedStock.StopLossPrice} ⚠️ 已触及`)
      triggeredType = 5
    } else {
      const diff = ((price - followedStock.StopLossPrice) / followedStock.StopLossPrice * 100).toFixed(2)
      alerts.push(`止损价: ${followedStock.StopLossPrice} (+${diff}%)`)
    }
  }

  if (alerts.length === 0) return

  const cacheKey = `${code}_${price}`
  if (priceLineAlertCache.get(cacheKey)) return

  const notifyKey = `${code}_notify`
  const lastNotify = priceLineAlertCache.get(notifyKey) || 0
  const now = Date.now()
  if (now - lastNotify < 60000) return

  priceLineAlertCache.set(cacheKey, true)
  priceLineAlertCache.set(notifyKey, now)

  const stockName = followedStock.Name || followedStock.StockName || result["股票名称"] || code
  const stockCodeDisplay = code.length > 6 ? code : code.toUpperCase()

  // notify.info({
  //   avatar: () => h(NAvatar, { size: 'small', round: false, src: icon.value }),
  //   title: `📈 ${stockName} (${stockCodeDisplay})`,
  //   duration: 5000,
  //   meta: `当前价: ${price}`,
  //   content: () => h('div', { style: { 'text-align': 'left', 'font-size': '13px' } },
  //     alerts.map(a => h('div', { style: { 'margin-bottom': '4px' } }, a))
  //   ),
  // })

  if (triggeredType > 0) {
    const msg = `### 📈 价位线预警\n\n### ${stockName} (${stockCodeDisplay})\n\n- 当前价格: ${price}\n- 预警类型: ${triggeredType === 4 ? '止盈触及' : '止损触及'}\n- 开仓价: ${followedStock.EntryPrice || '-'}\n- 止盈价: ${followedStock.TakeProfitPrice || '-'}\n- 止损价: ${followedStock.StopLossPrice || '-'}`;
    SendDingDingMessageByType(msg, code, triggeredType)
  }
}

function aiReCheckStock(stock, stockCode) {
  if (!data.aiConfigId) {
    message.error("请先选择AI模型配置")
    return
  }
  // 清除之前的超时定时器
  if (aiAnalysisTimeout.value) {
    clearTimeout(aiAnalysisTimeout.value)
    aiAnalysisTimeout.value = null
  }
  data.modelName = ""
  data.airesult = ""
  data.time = ""
  data.name = stock
  data.code = stockCode
  data.loading = true
  modalShow4.value = true
  data.analysisStatus = "正在连接AI服务..."
  message.loading("ai检测中...", {
    duration: 0,
  })
  //

  //message.info("sysPromptId:"+data.sysPromptId)
  NewChatStream(stock, stockCode, data.question, data.aiConfigId, data.sysPromptId, enableTools.value,thinkingMode.value)
    .catch(err => {
      data.loading = false
      data.analysisStatus = ""
      message.destroyAll()
      const errMsg = err?.message || err || "未知错误"
      message.error("AI分析请求失败: " + errMsg)
      data.airesult = "❌ AI分析请求失败: " + errMsg
    })

  // 设置超时兜底（5分钟）
  aiAnalysisTimeout.value = setTimeout(() => {
    if (data.loading) {
      data.loading = false
      data.analysisStatus = ""
      message.destroyAll()
      message.error("AI分析超时，请检查网络连接或AI服务配置")
      if (!data.airesult) {
        data.airesult = "❌ AI分析超时，请检查网络连接或AI服务配置是否正确。"
      }
    }
    aiAnalysisTimeout.value = null
  }, 5 * 60 * 1000)
}

function aiCheckStock(stock, stockCode) {
  GetAIResponseResult(stockCode).then(result => {
    if (result.content) {
      data.modelName = result.modelName
      data.chatId = result.chatId
      data.question = result.question
      data.name = stock
      data.code = stockCode
      data.loading = false
      modalShow4.value = true
      data.airesult = result.content
      const date = new Date(result.CreatedAt);
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      data.time = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
    } else {
      data.modelName = ""
      data.question = ""
      data.airesult = ""
      data.time = ""
      data.name = stock
      data.code = stockCode
      data.loading = false
      modalShow4.value = true
      // message.loading("ai检测中...", {
      //   duration: 0,
      // })
      // NewChatStream(stock, stockCode, "", data.sysPromptId)
    }
  })
}

function getTypeName(type) {
  switch (type) {
    case 1:
      return "涨跌报警"
    case 2:
      return "股价报警"
    case 3:
      return "成本价报警"
    default:
      return ""
  }
}

//获取高度
function getHeight() {
  return document.documentElement.clientHeight
}

window.onerror = function (msg, source, lineno, colno, error) {
  // 将错误信息发送给后端
  EventsEmit("frontendError", {
    page: "stock.vue",
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
    data: data,
    results: results,
    followList: followList,
    stockList: stockList,
    stocks: stocks,
    formModel: formModel,
  });
  message.error("发生错误:" + msg)
  return true;
};

function saveAsImage(name, code) {
  const previewEl = mdPreviewRef.value?.$el || mdEditorRef.value?.$el
  const element = previewEl?.querySelector('.md-editor-preview-wrapper') ||
                  previewEl?.querySelector('.md-editor-preview') ||
                  document.querySelector('.md-editor-preview')
  if (!element) {
    message.error('无法找到分析结果元素')
    return
  }
  const savedStyles = []
  let el = element.parentElement
  while (el && el !== document.body) {
    const style = getComputedStyle(el)
    if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
      savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
      el.style.overflow = 'visible'
      el.style.overflowY = 'visible'
      el.style.height = 'auto'
      el.style.maxHeight = 'none'
    }
    el = el.parentElement
  }
  const savedTargetStyle = { height: element.style.height, maxHeight: element.style.maxHeight, overflow: element.style.overflow, overflowY: element.style.overflowY }
  element.style.height = 'auto'
  element.style.maxHeight = 'none'
  element.style.overflow = 'visible'
  element.style.overflowY = 'visible'
  nextTick(async () => {
    const isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
    try {
      const canvas = await html2canvas(element, {
        useCORS: true,
        scale: 2,
        allowTaint: true,
        logging: false,
        backgroundColor: isDark ? '#1e1e1e' : '#ffffff'
      })
      element.style.height = savedTargetStyle.height
      element.style.maxHeight = savedTargetStyle.maxHeight
      element.style.overflow = savedTargetStyle.overflow
      element.style.overflowY = savedTargetStyle.overflowY
      savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
        el.style.overflow = overflow
        el.style.overflowY = overflowY
        el.style.height = height
        el.style.maxHeight = maxHeight
      })
      const dataUrl = canvas.toDataURL('image/png')
      const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
      const result = await SaveImage(name + '[' + code + ']AI分析', base64)
      if (result && !result.includes('异常') && !result.includes('无法')) {
        message.success('已导出为 PNG 图片：' + result)
      } else {
        message.info(result || '导出取消')
      }
    } catch (e) {
      element.style.height = savedTargetStyle.height
      element.style.maxHeight = savedTargetStyle.maxHeight
      element.style.overflow = savedTargetStyle.overflow
      element.style.overflowY = savedTargetStyle.overflowY
      savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
        el.style.overflow = overflow
        el.style.overflowY = overflowY
        el.style.height = height
        el.style.maxHeight = maxHeight
      })
      message.error('导出图片失败: ' + (e?.message ?? e))
    }
  })
}

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(data.airesult);
    message.success('分析结果已复制到剪切板');
  } catch (err) {
    message.error('复制失败: ' + err);
  }
}

function scrollToAiResultBottom() {
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = aiResultScrollRef.value
      if (el) {
        el.scrollTop = el.scrollHeight
      }
    })
  })
}

function saveAsMarkdown() {
  SaveAsMarkdown(data.code, data.name).then(result => {
    message.success(result)
  })
}

function saveAsMarkdown_old() {
  const blob = new Blob([data.airesult], {type: 'text/markdown;charset=utf-8'});
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = `${data.name}[${data.code}]-${data.time}ai-analysis-result.md`;
  link.click();
  URL.revokeObjectURL(link.href);
  link.remove()
}

function getHtml(ref) {
  if (ref.value) {
    // 获取 MdPreview 组件的根元素
    const rootElement = ref.value.$el;
    // 获取 HTML 内容
    return rootElement.innerHTML;
  } else {
    console.error('mdPreviewRef is not yet available');
    return "";
  }
}

// 导出文档
async function saveAsWord() {
  // 将富文本内容拼接为一个完整的html
  const html = getHtml(mdPreviewRef)
  const tipsHtml = getHtml(tipsRef)
  const value = `
         ${html}
         <hr>
         <div style="font-size: 12px;color: red">
         ${tipsHtml}
          </div>
<br>
本报告由go-stock项目生成：
<p>
<a href="https://github.com/ArvinLovegood/go-stock">
AI赋能股票分析：自选股行情获取，成本盈亏展示，涨跌报警推送，市场整体/个股情绪分析，K线技术指标分析等。数据全部保留在本地。支持DeepSeek，OpenAI， Ollama，LMStudio，AnythingLLM，硅基流动，火山方舟，阿里云百炼等平台或模型。
</a></p>
`
  // landscape就是横着的，portrait是竖着的，默认是竖屏portrait。
  const blob = await asBlob(value, {orientation: 'portrait'})
  const {platform} = await Environment()
  switch (platform) {
    case 'windows':
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = `${data.name}[${data.code}]-ai-analysis-result.docx`;
      a.click()
      // 下载后将标签移除
      URL.revokeObjectURL(a.href);
      a.remove()
      break
    default:
      const arrayBuffer = await blob.arrayBuffer()
      const uint8Array = new Uint8Array(arrayBuffer)
      const binary = uint8Array.reduce((data, byte) => data + String.fromCharCode(byte), '')
      const base64 = btoa(binary)
      await SaveWordFile(`${data.name}[${data.code}]-ai-analysis-result.docx`, base64).then(result => {
        message.success(result)
      })
  }
}

function share(code, name) {
  ShareAnalysis(code, name).then(msg => {
    //message.info(msg)
    notify.info({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '分享到社区',
      duration: 1000 * 30,
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, {default: () => msg})
      },
    })
  })
}

const addTabModel = ref({
  name: '',
  sort: 1,
})
const addTabPane = ref(false)

function addTab() {
  addTabPane.value = true
}

function saveTabPane() {
  AddGroup(addTabModel.value).then(result => {
    message.info(result)
    addTabPane.value = false
    GetGroupList().then(gList => {
      groupList.value = gList
      // 通知 App.vue 菜单栏立即刷新分组子项
      EventsEmit("groupListChanged")
      // 若来自关注流程的新建分组，创建成功后执行关注+加分组
      if (pendingFollow.value) {
        const created = gList.find(g => g.name === addTabModel.value.name)
        const pf = pendingFollow.value
        pendingFollow.value = null
        if (created) {
          data.code = pf.code
          data.name = pf.name
          doFollowStock(created.ID)
        }
      }
      // 若来自「设置分组」流程的新建分组，创建成功后把股票加入新分组
      if (pendingAddStockGroup.value) {
        const created = gList.find(g => g.name === addTabModel.value.name)
        const ps = pendingAddStockGroup.value
        pendingAddStockGroup.value = null
        if (created) {
          AddStockGroupInfo(created.ID, ps.code, ps.name)
        }
      }
    })
  })
}

// 概念标签：新建概念弹窗状态与保存逻辑（名称忽略大小写去重，复用已存在概念）
const addConceptModel = ref({
  name: '',
  sort: 1,
})
const addConceptPane = ref(false)

function saveConceptPane() {
  const rawName = (addConceptModel.value.name || '').trim()
  if (!rawName) {
    message.warning('请输入概念名称')
    return
  }
  // AddConcept 后端做大小写无关去重（幂等），成功后刷新列表
  AddConcept({ name: rawName, sort: addConceptModel.value.sort }).then(result => {
    message.info(result)
    addConceptPane.value = false
    GetConceptList().then(cList => {
      conceptList.value = cList
      // 若来自「设置概念」流程，把股票加入新建（或已存在同名）概念
      if (pendingAddStockConcept.value) {
        const ps = pendingAddStockConcept.value
        pendingAddStockConcept.value = null
        // 大小写无关查找，复用已存在概念（去重的关键）
        const created = cList.find(c => (c.name || '').toLowerCase() === rawName.toLowerCase())
        if (created) {
          AddStockConceptInfo(created.ID, ps.code, ps.name)
        }
      }
    })
  }).catch(err => message.error('添加概念失败: ' + (err?.message || err)))
}

// 修改分组名称
const renameTabPane = ref(false)
const renameModel = reactive({id: 0, name: ''})

function openRenameGroup() {
  const g = groupList.value.find(item => item.ID === currentGroupId.value)
  if (!g) {
    message.warning('请先选择一个分组')
    return
  }
  renameModel.id = g.ID
  renameModel.name = g.name
  renameTabPane.value = true
}

function saveRenameGroup() {
  const newName = renameModel.name.trim()
  if (!newName) {
    message.warning('请输入分组名称')
    return
  }
  UpdateGroup(renameModel.id, newName).then(result => {
    message.info(result)
    renameTabPane.value = false
    GetGroupList().then(gList => {
      groupList.value = gList
      // 通知 App.vue 菜单栏立即刷新分组子项
      EventsEmit("groupListChanged")
    })
  }).catch(err => message.error('修改失败: ' + (err?.message || err)))
}

function AddStockGroupInfo(groupId, code, name) {
  // 注意：不要把 gb_ 前缀转成 us。后端 Follow 已把美股存为 gb_aapl（us→gb_ + ToLower），
  // AddStockGroup 原样写入 group_stock_info.stock_code，GetFollowList(groupId) 再用该字段
  // IN 匹配 followed_stock.stock_code。若转成 usaapl 会导致美股分组关联失败（卡片上看不到）。
  AddStockGroup(groupId, code).then(result => {
    message.info(result)
    GetGroupList().then(gList => {
      groupList.value = gList
    })
    // 刷新「全部」标签页表格的分组列映射
    refreshCodeToGroups()
    // 当前正处于目标分组时，刷新该分组，让新成员立即可见
    if (currentGroupId.value === groupId) {
      updateTab(currentGroupId.value)
    }
  }).catch(err => {
    message.error('设置分组失败: ' + (err?.message || err))
  })
}

// 「设置分组」下拉的统一选中处理：new → 打开新建分组弹窗（创建后把股票加入）；普通项 → 切换（未所属加入 / 已所属移出）
function handleSetGroupSelect(groupId, stockCode, stockName) {
  if (groupId === 'new') {
    pendingAddStockGroup.value = {code: stockCode, name: stockName}
    addTabModel.value = {name: '', sort: 1}
    addTabPane.value = true
    return
  }
  const belongSet = new Set(codeToGroupIds.value.get(stockCode) || [])
  if (belongSet.has(groupId)) {
    // 已所属该分组 → 移出（不切换页签，仅刷新映射）
    RemoveStockGroup(stockCode, stockName, groupId).then(result => {
      message.info(result)
      refreshCodeToGroups()
    })
  } else {
    AddStockGroupInfo(groupId, stockCode, stockName)
  }
}

// 「设置分组」下拉的统一 option 渲染：new 项蓝色加 ➕；普通项右侧显示绿色 ✓（若已所属）
function renderSetGroupLabel(option, stockCode) {
  if (option.key === 'new') {
    return h('div', {style: 'color:#2080f0; font-weight:bold;'}, '➕ 新建分组')
  }
  const belongSet = new Set(codeToGroupIds.value.get(stockCode) || [])
  return h('div', {style: 'display:flex; justify-content:space-between; align-items:center; min-width:120px;'}, [
    h('span', null, option.label),
    belongSet.has(option.key) ? h('span', {style: 'color:#18a058; margin-left:8px; font-weight:bold;'}, '✓') : null
  ])
}

// 把股票加入概念（概念不产生页签，仅刷新映射）
function AddStockConceptInfo(conceptId, code, name) {
  AddStockConcept(conceptId, code).then(result => {
    message.info(result)
    GetConceptList().then(cList => { conceptList.value = cList })
    // 刷新「全部」标签页表格的概念列映射
    refreshCodeToConcepts()
  }).catch(err => {
    message.error('设置概念失败: ' + (err?.message || err))
  })
}

// 「设置概念」下拉的统一选中处理：new → 打开新建概念弹窗（创建后把股票加入）；普通项 → 切换（未所属加入 / 已所属移出）
function handleSetConceptSelect(conceptId, stockCode, stockName) {
  if (conceptId === 'new') {
    pendingAddStockConcept.value = {code: stockCode, name: stockName}
    addConceptModel.value = {name: '', sort: 1}
    addConceptPane.value = true
    return
  }
  const belongSet = new Set(codeToConceptIds.value.get(stockCode) || [])
  if (belongSet.has(conceptId)) {
    // 已所属该概念 → 移出
    RemoveStockConcept(stockCode, stockName, conceptId).then(result => {
      message.info(result)
      refreshCodeToConcepts()
    })
  } else {
    AddStockConceptInfo(conceptId, stockCode, stockName)
  }
}

// 「设置概念」下拉的统一 option 渲染：new 项蓝色加 ➕；普通项右侧显示绿色 ✓（若已所属）
function renderSetConceptLabel(option, stockCode) {
  if (option.key === 'new') {
    return h('div', {style: 'color:#2080f0; font-weight:bold;'}, '➕ 新建概念')
  }
  const belongSet = new Set(codeToConceptIds.value.get(stockCode) || [])
  return h('div', {style: 'display:flex; justify-content:space-between; align-items:center; min-width:120px;'}, [
    h('span', null, option.label),
    belongSet.has(option.key) ? h('span', {style: 'color:#18a058; margin-left:8px; font-weight:bold;'}, '✓') : null
  ])
}

function updateTab(name) {
  stocks.value = []
  const tabId= Number(name)
  currentGroupId.value = tabId;
  GetFollowList(tabId).then(result => {
    followList.value = result

    for (const followedStock of result) {
      if (followedStock.StockCode.startsWith("us")) {
        followedStock.StockCode = "gb_" + followedStock.StockCode.replace("us", "").toLowerCase()
      }
      stocks.value.push(followedStock.StockCode)
      Greet(followedStock.StockCode).then(result => {
        updateData(result)
      })
    }
    //monitor()
    message.destroyAll()
  })
}

function delTab(groupId) {
  let infos = groupList.value = groupList.value.filter(item => item.ID === Number(groupId))
  dialog.create({
    title: '删除分组',
    type: 'warning',
    content: '确定要删除[' + infos[0].name + ']分组吗？分组数据将不能恢复哟！',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      RemoveGroup(Number(groupId)).then(result => {
        message.info(result)
        // 若「全部」标签页正在按被删分组筛选，重置为「全部分组」
        if (tableGroupFilter.value === Number(groupId)) tableGroupFilter.value = 0
        GetGroupList().then(result => {
          groupList.value = result
          // 通知 App.vue 菜单栏立即刷新分组子项
          EventsEmit("groupListChanged")
        })
        // 分组删除后成员关系变化，刷新「全部」标签页表格的分组列映射
        refreshCodeToGroups()
      })
    }
  })
}

function delStockGroup(code, name, groupId) {
  RemoveStockGroup(code, name, groupId).then(result => {
    updateTab(groupId)
    // 刷新「全部」标签页表格的分组列映射
    refreshCodeToGroups()
    message.info(result)
  })
}

function searchNotice(stockCode) {
  router.push({
    name: 'market',
    query: {
      name: '公司公告',
      stockCode: stockCode,
    },
  })
}

function searchStockReport(stockCode) {
  router.push({
    name: 'market',
    query: {
      name: '个股研报',
      stockCode: stockCode,
    },
  })
}

// 监听多周期 K 线模态框关闭，清除定时器
watch(modalShow6, (newVal) => {
  if (!newVal && klineAutoCloseTimer.value) {
    clearTimeout(klineAutoCloseTimer.value)
    klineAutoCloseTimer.value = null
  }
})

// 大单过滤切换后，同步分页 itemCount + 回到第 1 页
watch([tdxAmountFilter, filteredTdxTransactionList], () => {
  tdxTransactionPagination.value.itemCount = filteredTdxTransactionList.value.length
  tdxTransactionPagination.value.page = 1
})
</script>

<template>
  <vue-danmaku v-model:danmus="danmus" useSlot
               style="height:100px; width:100%;z-index: 9;position:absolute; top: 400px; pointer-events: none;">
    <template v-slot:dm="{ index, danmu }">
      <n-gradient-text type="info">
        <n-icon :component="ChatboxOutline"/>
        {{ danmu }}
      </n-gradient-text>
    </template>
  </vue-danmaku>
  <n-tabs type="card" style="--wails-draggable:no-drag"
          :style="{ '--stock-tab-nav-bg': tabNavBgColor }"
          animated addable :data-currentGroupId="currentGroupId"
          :value="String(currentGroupId)" @add="addTab" @update:value="updateTab" placement="top" @close="(key)=>{delTab(key)}">

    <template #suffix>
      <n-button v-if="currentGroupId>0" size="small" tertiary type="primary" @click="openRenameGroup" style="margin-left:4px;">
        <n-icon :component="CreateOutline"/>&nbsp;重命名
      </n-button>
    </template>

    <n-tab-pane closable name="0" :tab="'全部'">
      <div style="margin: 8px;">
        <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px; flex-wrap:wrap;">
          <n-input v-model:value="tableSearchKeyword" clearable placeholder="搜索股票名称/代码"
                   style="width:280px;" />
          <n-select v-model:value="tableGroupFilter" :options="groupFilterOptions"
                    placeholder="全部分组" style="width:180px;" filterable
                    :consistent-menu-width="false" />
          <n-select v-model:value="tableConceptFilter" :options="conceptFilterOptions"
                    placeholder="全部概念" style="width:180px;" filterable
                    :consistent-menu-width="false" />
          <n-text depth="3" style="font-size:12px;">共 {{ allTableData.length }} 只</n-text>
        </div>
        <n-data-table
          :columns="allTableColumns"
          :data="allTableData"
          :pagination="allTablePagination"
          :row-key="(row) => row.key"
          size="small"
          striped
          flex-height
          style="height: calc(100vh - 190px);"
        />
      </div>
    </n-tab-pane>
    <n-tab-pane closable v-for="group in groupList" :group-id="group.ID" :name="String(group.ID)" :tab="group.name">
      <n-grid :x-gap="8" :cols="3" :y-gap="8">
        <n-gi :id="result['股票代码']+'_gi'" v-for="result in groupResults" :key="result.key" style="margin-left: 2px;">
          <n-card :data-sort="result.sort" :id="result['股票代码']" :data-code="result['股票代码']" :bordered="true"
                  :title="result['股票名称']" :closable="false"
                  @close="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
            <n-grid :cols="12" :y-gap="6">
              <n-gi :span="6">
                <n-text :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="result['上次当前价格']"
                                      :to="Number(result['当前价格'])"/>
                  <n-tag size="small" :type="result.type" :bordered="false" v-if="result['盘前盘后']>0">
                    ({{ result['盘前盘后'] }} {{ result['盘前盘后涨跌幅'] }}%)
                  </n-tag>
                </n-text>
                <n-text style="padding-left: 10px;" :type="result.type">
                  <n-number-animation :duration="1000" :precision="3" :from="result.lastChangePercent" :to="result.changePercent"/>
                  %
                </n-text>&nbsp;
                <n-text size="small" v-if="result.costVolume>0" :type="result.type">
                  <n-number-animation :duration="1000" :precision="2" :from="result.lastProfitAmountToday" :to="result.profitAmountToday"/>
                </n-text>
              </n-gi>
              <n-gi :span="6">
                <stock-spark-line :last-price="Number(result['当前价格'])" :open-price="Number(result['昨日收盘价'])"
                                  :stock-code="result['股票代码']" :stock-name="result['股票名称']"></stock-spark-line>
              </n-gi>
            </n-grid>
            <n-grid :cols="2" :y-gap="4" :x-gap="4">
              <n-gi>
                <n-text :type="'info'">{{ "最高 " + result["今日最高价"] + " " + result.highRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "最低 " + result["今日最低价"] + " " + result.lowRate }}%</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "昨收 " + result["昨日收盘价"] }}</n-text>
              </n-gi>
              <n-gi>
                <n-text :type="'info'">{{ "今开 " + result["今日开盘价"] }}</n-text>
              </n-gi>
            </n-grid>
            <n-collapse accordion v-if="result['买一报价']>0">
              <n-collapse-item title="盘口" name="1" v-if="result['买一报价']>0">
                <template #header-extra>
                  <n-flex justify="space-between">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-flex>
                </template>
                <n-grid :cols="2" :y-gap="4" :x-gap="4">
                  <n-gi v-if="result['买一报价']>0">
                    <n-text :type="'info'">{{ "买一 " + result["买一报价"] + '(' + result["买一申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖一报价']>0">
                    <n-text :type="'info'">{{ "卖一 " + result["卖一报价"] + '(' + result["卖一申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买二报价']>0">
                    <n-text :type="'info'">{{ "买二 " + result["买二报价"] + '(' + result["买二申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖二报价']>0">
                    <n-text :type="'info'">{{ "卖二 " + result["卖二报价"] + '(' + result["卖二申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["买三报价"] + '(' + result["买三申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖三报价']>0">
                    <n-text :type="'info'">{{ "买三 " + result["卖三报价"] + '(' + result["卖三申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买四报价']>0">
                    <n-text :type="'info'">{{ "买四 " + result["买四报价"] + '(' + result["买四申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖四报价']>0">
                    <n-text :type="'info'">{{ "卖四 " + result["卖四报价"] + '(' + result["卖四申报"] + ")" }}</n-text>
                  </n-gi>

                  <n-gi v-if="result['买五报价']>0">
                    <n-text :type="'info'">{{ "买五 " + result["买五报价"] + '(' + result["买五申报"] + ")" }}</n-text>
                  </n-gi>
                  <n-gi v-if="result['卖五报价']>0">
                    <n-text :type="'info'">{{ "卖五 " + result["卖五报价"] + '(' + result["卖五申报"] + ")" }}</n-text>
                  </n-gi>
                </n-grid>
              </n-collapse-item>
            </n-collapse>
            <template #header-extra>

              <n-tag size="small" :bordered="false">{{ result['股票代码'] }}</n-tag>&nbsp;
              <n-button size="tiny" secondary type="primary"
                        @click="removeMonitor(result['股票代码'],result['股票名称'],result.key)">
                取消关注
              </n-button>&nbsp;

              <n-button size="tiny" v-if="data.openAiEnable" secondary type="warning"
                        @click="aiCheckStock(result['股票名称'],result['股票代码'])">
                AI分析
              </n-button>
              <n-button secondary type="error" size="tiny"
                        @click="delStockGroup(result['股票代码'],result['股票名称'],group.ID)">移出分组
              </n-button>
            </template>
            <template #footer>
              <n-flex vertical :size="8">
                <n-flex justify="center">
                  <n-text :type="'info'">{{ result["日期"] + " " + result["时间"] }}</n-text>
                  <n-tag size="small" v-if="result.volume>0" :type="result.profitType">{{ result.volume + "股" }}</n-tag>
                  <n-tag size="small" v-if="result.costPrice>0" :type="result.profitType">
                    {{
                      "成本:" + result.costPrice + "*" + result.costVolume + " " + result.profit + "%" + " ( " + result.profitAmount + " ¥ )"
                    }}
                  </n-tag>
                </n-flex>
                <n-flex justify="center">
                  <n-button size="tiny" type="primary" secondary
                            @click="showLightweightKline(result['股票代码'],result['股票名称'])">
                    多周期K线
                  </n-button>
                </n-flex>
              </n-flex>
            </template>
            <template #action>
              <n-flex justify="left">
                <n-button size="tiny" type="warning" @click="setStock(result['股票代码'],result['股票名称'])"> 成本
                </n-button>
                <n-button size="tiny" type="error"
                          @click="showFenshi(result['股票代码'],result['股票名称'],result.changePercent)"> 分时
                </n-button>
                <n-button size="tiny" type="info"
                          @click="showTransactionDetail(result['股票代码'],result['股票名称'])"> 成交明细
                </n-button>
                <n-button size="tiny" type="error" @click="showK(result['股票代码'],result['股票名称'])"> 日K</n-button>
                <n-button size="tiny" type="error" v-if="result['买一报价']>0"
                          @click="showMoney(result['股票代码'],result['股票名称'])"> 资金
                </n-button>
                <n-button size="tiny" type="success" @click="search(result['股票代码'],result['股票名称'])"> 详情
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchNotice(result['股票代码'])"> 公告
                </n-button>
                <n-button v-if="result['买一报价']>0" size="tiny" type="success"
                          @click="searchStockReport(result['股票代码'])"> 研报
                </n-button>
                <n-flex justify="right">
                  <n-dropdown trigger="click" :options="setGroupOptions"
                              :menu-props="() => ({ style: 'max-height:300px; overflow-y:auto;' })"
                              :render-label="(option) => renderSetGroupLabel(option, result['股票代码'])"
                              @select="(groupId) => handleSetGroupSelect(groupId, result['股票代码'], result['股票名称'])">
                    <n-button type="warning" size="tiny">设置分组</n-button>
                  </n-dropdown>
                  <n-dropdown trigger="click" :options="setConceptOptions"
                              :menu-props="() => ({ style: 'max-height:300px; overflow-y:auto;' })"
                              :render-label="(option) => renderSetConceptLabel(option, result['股票代码'])"
                              @select="(conceptId) => handleSetConceptSelect(conceptId, result['股票代码'], result['股票名称'])">
                    <n-button type="info" size="tiny">设置概念</n-button>
                  </n-dropdown>
                </n-flex>
              </n-flex>
            </template>
          </n-card>
        </n-gi>
      </n-grid>
    </n-tab-pane>
  </n-tabs>

  <div style="position: fixed;bottom: 18px;right:5px;z-index: 10;width: 400px">
    <!--    <n-card :bordered="false">-->
    <n-input-group>
      <!--        <n-button  type="error" @click="addBTN=!addBTN" > <n-icon :component="Search"/>&nbsp;<n-text  v-if="addBTN">隐藏</n-text></n-button>-->

      <n-auto-complete v-model:value="data.name" v-if="addBTN"
                       :input-props="{
                                autocomplete: 'disabled',
                              }"
                       :options="options"
                       placeholder="股票指数名称/代码/弹幕"
                       clearable @update-value="getStockList" :on-select="onSelect"/>

      <n-popover trigger="manual" :show="showPopover">
        <template #trigger>
          <n-dropdown trigger="click" :options="followGroupOptions" :menu-props="() => ({ style: 'max-height:300px; overflow-y:auto;' })" @select="handleFollowSelect" placement="top">
            <n-button type="primary" v-if="addBTN">
              <n-icon :component="Add"/> &nbsp;关注
            </n-button>
          </n-dropdown>
        </template>
        <span>输入股票名称/代码关键词开始吧~~~</span>
      </n-popover>

      <n-button type="info" @click="SendDanmu" v-if="data.enableDanmu">
        <n-icon :component="ChatboxOutline"/> &nbsp;发送弹幕
      </n-button>
    </n-input-group>
    <!--    </n-card>-->
  </div>
  <n-modal transform-origin="center" size="small" v-model:show="modalShow" :title="formModel.name" style="width: 800px;max-width: calc(100vw - 32px);"
           :preset="'card'">
    <n-form :model="formModel" :rules="{
              costPrice: { required: true, message: '请输入成本'},
              volume: { required: true, message: '请输入数量'},
              alarm:{required: true, message: '涨跌报警值'} ,
              alarmPrice: { required: true, message: '请输入报警价格'},
              sort: { required: true, message: '请输入排序值'},
            }" label-placement="left" label-width="100px">
      <n-grid :cols="2" :x-gap="12">
        <n-gi>
          <n-form-item label="股票成本" path="costPrice">
            <n-input-number v-model:value="formModel.costPrice" min="0" placeholder="请输入股票成本" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票数量" path="volume">
            <n-input-number v-model:value="formModel.volume" min="0" step="100" placeholder="请输入股票数量" style="width: 100%">
              <template #suffix>
                股
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="涨跌提醒" path="alarm">
            <n-input-number v-model:value="formModel.alarm" min="0" placeholder="涨跌报警值(%)" style="width: 100%">
              <template #suffix>
                %
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股价提醒" path="alarmPrice">
            <n-input-number v-model:value="formModel.alarmPrice" min="0" placeholder="股价报警值" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="开仓价" path="entryPrice">
            <n-input-number v-model:value="formModel.entryPrice" min="0" step="0.01" placeholder="请输入开仓价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票排序" path="sort">
            <n-input-number v-model:value="formModel.sort" min="0" placeholder="排序值" style="width: 100%">
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止盈价" path="takeProfitPrice">
            <n-input-number v-model:value="formModel.takeProfitPrice" min="0" step="0.01" placeholder="请输入止盈价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止损价" path="stopLossPrice">
            <n-input-number v-model:value="formModel.stopLossPrice" min="0" step="0.01" placeholder="请输入止损价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-button type="primary"
                @click="updateCostPriceAndVolumeNew(formModel.code,formModel.costPrice,formModel.volume,formModel.alarm,formModel)">
        保存
      </n-button>
    </template>
  </n-modal>

  <n-modal v-model:show="addTabPane" title="添加分组" style="width: 400px;text-align: left" :preset="'card'">
    <n-form
        :model="addTabModel"
        size="medium"
        label-placement="left"
    >
      <n-grid :cols="2">
        <n-form-item-gi label="分组名称:" path="name" :span="5">
          <n-input v-model:value="addTabModel.name" style="width: 100%" placeholder="请输入分组名称"/>
        </n-form-item-gi>
        <n-form-item-gi label="分组排序:" path="sort" :span="5">
          <n-input-number v-model:value="addTabModel.sort" style="width: 100%" min="0"
                          placeholder="请输入分组排序值"></n-input-number>
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveTabPane">
          保存
        </n-button>
        <n-button type="warning" @click="addTabPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="addConceptPane" title="添加概念" style="width: 400px;text-align: left" :preset="'card'">
    <n-form
        :model="addConceptModel"
        size="medium"
        label-placement="left"
    >
      <n-grid :cols="2">
        <n-form-item-gi label="概念名称:" path="name" :span="5">
          <n-input v-model:value="addConceptModel.name" style="width: 100%" placeholder="请输入概念名称"
                   @keyup.enter="saveConceptPane"/>
        </n-form-item-gi>
        <n-form-item-gi label="概念排序:" path="sort" :span="5">
          <n-input-number v-model:value="addConceptModel.sort" style="width: 100%" min="0"
                          placeholder="请输入概念排序值"></n-input-number>
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveConceptPane">
          保存
        </n-button>
        <n-button type="warning" @click="addConceptPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="renameTabPane" title="修改分组名称" style="width: 400px;text-align: left" :preset="'card'">
    <n-form :model="renameModel" size="medium" label-placement="left">
      <n-form-item-gi label="分组名称:" path="name" :span="5">
        <n-input v-model:value="renameModel.name" style="width: 100%" placeholder="请输入新的分组名称"
                 @keyup.enter="saveRenameGroup"/>
      </n-form-item-gi>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveRenameGroup">
          保存
        </n-button>
        <n-button type="warning" @click="renameTabPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="modalShow2" :title="data.name+' '+ data.changePercent+'%'" style="width: 1000px;max-width: calc(100vw - 32px);"
           :preset="'card'" @after-enter="handleFeishi" @after-leave="clearFeishi">
    <!--    <n-image :src="data.fenshiURL" />-->
    <div ref="kLineChartRef2" style="width: 100%; height: 500px;"></div>
  </n-modal>
  <n-modal v-model:show="modalShow3" :title="data.name" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'"
           @after-enter="handleKLine">
    <!--    <n-image :src="data.kURL" />-->
    <div ref="kLineChartRef" style="width: 100%; height: 500px;"></div>
  </n-modal>

  <n-modal transform-origin="center" v-model:show="modalShow4" preset="card" style="width: 800px;max-width: calc(100vw - 32px);"
           :title="'['+data.name+']AI分析'">
    <n-spin size="small" :show="data.loading && !data.airesult">
      <MdEditor v-if="enableEditor" :toolbars="toolbars" ref="mdEditorRef" style="height: 440px;max-height: 60vh;text-align: left"
                :modelValue="data.airesult" :theme="theme">
        <template #defToolbars>
          <ExportPDF :file-name="data.name+'['+data.code+']AI分析报告'" style="text-align: left"
                     :modelValue="data.airesult" @onProgress="handleProgress"/>
        </template>
      </MdEditor>
      <div v-if="!enableEditor" ref="aiResultScrollRef" style="height: 440px;max-height: 60vh;text-align: left;overflow-y: auto;">
        <MdPreview ref="mdPreviewRef" :modelValue="data.airesult" :theme="theme"/>
      </div>
    </n-spin>
    <template #footer>
      <n-flex justify="space-between" ref="tipsRef">
        <n-text type="info" v-if="data.time">
          <n-tag v-if="data.modelName" type="warning" round :title="data.chatId" :bordered="false">
            {{ data.modelName }}
          </n-tag>
          {{ data.time }}
        </n-text>
        <n-text type="success" v-if="data.analysisStatus">{{ data.analysisStatus }}</n-text>
        <n-text type="error">*AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。</n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="left" style="margin-bottom: 10px">
        <n-switch v-model:value="enableTools" :round="false">
          <template #checked>
            工具调用
          </template>
          <template #unchecked>
            非工具调用
          </template>
        </n-switch>
        <n-switch v-model:value="thinkingMode" :round="false">
          <template #checked>
            思考模式
          </template>
          <template #unchecked>
            非思考模式
          </template>
        </n-switch>
        <n-gradient-text type="error" style="margin-left: 10px">
          *AI函数工具调用可以增强AI获取数据的能力,但会消耗更多tokens。
        </n-gradient-text>
      </n-flex>
      <n-flex justify="space-between" style="margin-bottom: 10px">
        <n-select style="width: 31%" v-model:value="data.aiConfigId" label-field="name" value-field="ID"
                  :options="aiConfigs" placeholder="请选择AI模型服务配置"/>
        <n-select style="width: 31%" v-model:value="data.sysPromptId" label-field="name" value-field="ID"
                  :options="sysPromptOptions" placeholder="请选择系统提示词"/>
        <n-select style="width: 31%" v-model:value="data.question" label-field="name" value-field="content"
                  :options="userPromptOptions" placeholder="请选择用户提示词"/>
      </n-flex>
      <n-flex justify="right">
        <n-input v-model:value="data.question" style="text-align: left" clearable
                 type="textarea"
                 :show-count="true"
                 placeholder="请输入您的问题:例如{{stockName}}[{{stockCode}}]分析和总结"
                 :autosize="{
              minRows: 2,
              maxRows: 5
            }"
        />
        <!--        <n-button size="tiny" type="error" @click="enableEditor=!enableEditor">编辑/预览</n-button>-->
        <n-button size="tiny" type="warning" @click="aiReCheckStock(data.name,data.code)">开始AI分析</n-button>
        <n-button size="tiny" type="info" @click="saveAsImage(data.name,data.code)">保存为图片</n-button>
        <n-button size="tiny" type="success" @click="copyToClipboard">复制到剪切板</n-button>
        <n-button size="tiny" type="primary" @click="saveAsMarkdown">保存为Markdown文件</n-button>
        <n-button size="tiny" type="primary" @click="saveAsWord">保存为Word文件</n-button>
        <n-button size="tiny" type="error" @click="share(data.code,data.name)">分享到项目社区</n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="modalShow5" :title="data.name+'资金趋势'" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'">
    <money-trend :code="data.code" :name="data.name" :days="360" :dark-theme="data.darkTheme"
                 :chart-height="500"></money-trend>
  </n-modal>
  <n-modal
    v-model:show="modalShow6"
    :title="(lwKlineName || '') + ' — 多周期K线'"
    preset="card"
    style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
    :content-style="{
      maxHeight: 'min(85vh, 820px)',
      overflowY: 'auto',
      overflowX: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
  >
    <stock-lightweight-kline-chart
      v-if="modalShow6"
      :key="'lightweight-' + lwKlineCode"
      :code="lwKlineCode"
      :stock-name="lwKlineName"
      :dark-theme="data.darkTheme"
      :chart-height="500"
      :long-entry-price="currentStockTradingPrice.entryPrice"
      :long-stop-loss-price="currentStockTradingPrice.stopLossPrice"
      :long-take-profit-price="currentStockTradingPrice.takeProfitPrice"
      :cost-price="currentStockTradingPrice.costPrice"
      @update:longEntryPrice="handleLongEntryPriceUpdate"
      @update:longStopLossPrice="handleLongStopLossPriceUpdate"
      @update:longTakeProfitPrice="handleLongTakeProfitPriceUpdate"
      @update:costPrice="handleCostPriceUpdate"
    />
  </n-modal>

  <!-- gotdx 分时图 + 分笔成交明细 -->
  <n-modal
    v-model:show="modalShow7"
    preset="card"
    style="width: 1200px; max-width: calc(100vw - 32px);"
    :content-style="{ padding: '8px' }"
    @after-leave="handleTdxTransactionModalClose"
  >
    <template #header>
      <div style="display:flex; align-items:baseline; gap:12px; flex-wrap:wrap;">
        <span>{{ (data.name || '') + '（' + (data.code || '') + '）' }}</span>
        <template v-if="tdxRealTimeInfo.price > 0">
          <span style="font-size:14px; font-weight:bold;" :style="{ color: tdxRealTimeInfo.changePercent >= 0 ? '#d03050' : '#18a058' }">
            {{ tdxRealTimeInfo.price.toFixed(2) }}
          </span>
          <span style="font-size:13px;" :style="{ color: tdxRealTimeInfo.changePercent >= 0 ? '#d03050' : '#18a058' }">
            {{ tdxRealTimeInfo.changePercent >= 0 ? '+' : '' }}{{ tdxRealTimeInfo.changePercent.toFixed(2) }}%
          </span>
        </template>
      </div>
    </template>
    <template #header-extra>
      <n-flex align="center" :size="8" :wrap="true">
        <n-date-picker
          v-model:value="tdxSelectedDateRange"
          type="daterange"
          size="small"
          style="width:240px;"
          :is-date-disabled="tdxDateDisabled"
          :actions="['confirm']"
          @update:value="onTdxDateRangeChange"
        />
        <n-button-group size="small">
          <n-button
            v-for="n in [2,3, 5, 10, 20, 30]"
            :key="n"
            :type="tdxActiveQuickDays === n ? 'primary' : 'default'"
            :tertiary="tdxActiveQuickDays !== n"
            size="small"
            style="min-width:38px;"
            @click="selectRecentDays(n)"
          >近{{ n }}日</n-button>
        </n-button-group>
        <n-select
          v-model:value="tdxAmountFilter"
          :options="tdxAmountFilterOptions"
          size="small"
          style="width:180px;"
          :consistent-menu-width="false"
        />
        <n-button size="small" :type="tdxAutoRefresh ? 'primary' : 'default'" tertiary @click="toggleAutoRefresh">
          {{ tdxAutoRefresh ? '自动刷新' : '已暂停' }}
        </n-button>
        <n-button size="small" type="primary" tertiary @click="refreshTdxTransaction">刷新</n-button>
      </n-flex>
    </template>
    <div style="display:flex; flex-direction:column; gap:8px;">
      <div ref="tdxTransactionChartRef" style="width: 100%; height: 200px;"></div>

      <!-- 各档位买卖方向占比 + 净流入金额统计 -->
      <div style="display:grid; grid-template-columns: repeat(4, 1fr); gap:8px;">
        <div v-for="stat in tdxAmountStats" :key="stat.level"
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

      <!-- 各档位累计净流入金额变化折线图 -->
      <div ref="tdxNetInflowChartRef" style="width: 100%; height: 200px;"></div>

      <n-data-table
        :columns="tdxTransactionColumns"
        :data="filteredTdxTransactionList"
        :pagination="tdxTransactionPagination"
        :loading="tdxTransactionLoading"
        size="small"
        striped
        :row-key="(row) => row.key"
        :max-height="320"
      />
    </div>
  </n-modal>
</template>

<style scoped>
.md-editor-preview h3 {
  text-align: center !important;
}

.md-editor-preview p {
  text-align: left !important;
}

/* 添加闪烁效果的CSS类 */
.blink-border {
  animation: blink-border 1s linear infinite;
  border: 4px solid transparent;
}

@keyframes blink-border {
  0% {
    border-color: red;
  }
  50% {
    border-color: transparent;
  }
  100% {
    border-color: red;
  }
}

/* 所有标签的通用样式 */
:deep(.n-tabs-nav .n-tabs-tab) {
  position: relative;
  cursor: pointer;
}

/* 顶部页签固定吸顶，不随内容滚动 */
:deep(.n-tabs-nav) {
  position: sticky;
  top: 0;
  z-index: 10;
  background-color: var(--stock-tab-nav-bg, #ffffff);
}

/* 可拖拽标签的样式 */
:deep(.n-tabs-nav .n-tabs-tab[draggable="true"]) {
  user-select: none;
  cursor: move;
}

.tab-drag-over {
  background-color: #e6f7ff !important;
  border: 2px dashed #1890ff !important;
  transform: scale(1.02);
  transition: all 0.2s ease;
  z-index: 10;
}

.tab-drag-over::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: -1;
}

.tab-dragging {
  opacity: 0.5;
}
</style>
