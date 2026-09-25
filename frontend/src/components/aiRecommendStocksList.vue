<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {useRouter} from 'vue-router'
import {
  GetAiRecommendStocksList,
  GetAiRecommendStocksTodayStats,
  GetConfig,
  GetSponsorInfo,
  DeleteAiRecommendStocks,
  UpdateAiRecommendStocksAlert,
  ShareAnalysis,
  RunRecommendBacktest,
  ListRecommendBacktest
} from "../../wailsjs/go/main/App";
import {NAvatar, NButton, NEllipsis, NSwitch, NTag, NText, useMessage, useNotification} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import sparkLine from "./stockSparkLine.vue"
import {MdPreview} from "md-editor-v3";
import {format} from "date-fns";

const notify = useNotification()
const vipLevel=ref("");
const vipStartTime=ref("");
const vipEndTime=ref("");
const expired=ref(false)
const isValidVip=ref(false) // 是否是会员

onBeforeMount(()=> {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })

  GetSponsorInfo().then((res) => {
   // console.log(res)
    vipLevel.value = res.vipLevel;
    vipStartTime.value = res.vipStartTime;
    vipEndTime.value = res.vipEndTime;
    //判断时间是否到期
    if (res.vipLevel) {
      if (res.vipEndTime < format(new Date(), 'yyyy-MM-dd HH:mm:ss')) {
        //notify.warning({content: 'VIP已到期'})
        expired.value = true;
      }
    }else{
      //notify.success({content: '未开通VIP'})
    }
    isValidVip.value = !(vipLevel.value === "" || Number(vipLevel.value) <= 0);
  })
})
onMounted(() => {
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword,
    startDate: paginationReactive.range[0],
    endDate: paginationReactive.range[1]
  }).then((data) => {
    console.log( data)
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.pageCount
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
  loadBacktestMap()
  loadTodayStats()
})
const message = useMessage()
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const editorDataRef = reactive({
  show: false,
  loading: false,
  darkTheme: false,
  chatId: "",
  modelName: "",
  CreatedAt: "",
  stockName: "",
  stockCode: "",
  question: "",
  content: "",
})
const dataRef = ref([])
const loadingRef = ref(true)

// StockClosePrice          string     `json:"StockClosePrice" md:"推荐时股票收盘价格"`
// StockPrePrice            string     `json:"stockPrePricePrice" md:"前一交易日股票价格"`
// RecommendReason          string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
// RecommendBuyPrice        string     `json:"recommendBuyPrice" md:"ai建议买入价"`
// RecommendStopProfitPrice string     `json:"recommendStopProfitPrice" md:"ai建议止盈价"`
// RecommendStopLossPrice   string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
// RiskRemarks              string     `json:"riskRemarks" md:"风险提示"`
// Remarks                  string     `json:"remarks" md:"备注"`
const columnsRef = ref([
  {
    title: '推荐模型',
    key: 'modelName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.modelName })
    }
  },
  {
    title: '评级',
    key: 'rating',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.rating || '-' })
    }
  },
  {
    title: '推荐时间',
    key: 'dataTime',
    render(row, index) {
      //2026-01-14T22:13:27.2693252+08:00 格式化为常用时间格式
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '板块概念',
    key: 'bkName'
  },
  {
    title: '股票名称',
    key: 'stockName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockName })
    }
  },
  {
    title: '股票代码',
    key: 'stockCode'
  },
  {
    title: '最新分时',
    key: 'stockCode',
    render(row, index) {
      return h(sparkLine, { idSuffix:row.ID, stockName: row.stockName, stockCode: row.stockCode, lastPrice: row.stockCurrentPrice, openPrice: row.stockPrePrice, tooltip: true }, )
    }
  },
  {
    title: '最新',
    key: 'stockCurrentPrice',
    minWidth: 120,
    render(row, index) {

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrePrice))/ Number(row.stockPrePrice)*100).toFixed(2)

      if(Number(row.stockCurrentPrice)< Number(row.stockPrePrice)) {
        return [h(NText, { type: "success", bordered: false }, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      } else {
        return [h(NText, { type: "error" , bordered: false}, { default: () => row.stockCurrentPrice+` |  ${diff}%` })]
      }
    }
  },
  {
    title: '推荐时',
    key: 'stockPrice',
    render(row, index) {

      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.stockPrice })
      }

      let diff = ((Number(row.stockCurrentPrice) - Number(row.stockPrice))/ Number(row.stockPrice)*100).toFixed(2)
      let flagStr="暂平"
      let flag="info"
      if(Number(row.stockCurrentPrice)>Number(row.stockPrice)) {
        flagStr="暂赢 "+diff+"%"
        flag="error"
      }else if(Number(row.stockCurrentPrice)===Number(row.stockPrice)){
        flagStr="暂平"
        flag="info"
      }else{
        flagStr="暂亏 "+ diff+"%"
        flag="success"
      }

      return [h(NText, { type: "info" }, { default: () => row.stockPrice }),h(NTag, { type: flag,size: "tiny", bordered: false }, { default: () => flagStr })]
    }
  },
  {
    title: '回测(5日)',
    key: 'backtest',
    width: 100,
    render(row, index) {
      const outcome = backtestMapRef.value[row.ID]
      if (!outcome) {
        return h(NTag, { size: "tiny", type: "default", bordered: false }, { default: () => '未回测' })
      }
      if (outcome === 'win') {
        return h(NTag, { size: "tiny", type: "error", bordered: false }, { default: () => '达标' })
      }
      return h(NTag, { size: "tiny", type: "success", bordered: false }, { default: () => '未达标' })
    }
  },
  {
    title: '昨收',
    key: 'stockPrePrice',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockPrePrice })
    }
  },
  {
    title: '开仓价',
    key: 'recommendBuyPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })
      }


      if(row.recommendBuyPrice.includes("-")){
        let prices= row.recommendBuyPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
        }
      }
      if(row.recommendBuyPriceMin&&row.recommendBuyPriceMax&&Number(row.stockCurrentPrice)<Number(row.recommendBuyPriceMax)&&Number(row.stockCurrentPrice)>Number(row.recommendBuyPriceMin)){
        return [h(NText, { type: "success" }, { default: () => row.recommendBuyPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })

    }
  },
  {
    title: '止盈价',
    key: 'recommendStopProfitPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
      }
      if(row.recommendStopProfitPrice.includes("-")){
        let prices= row.recommendStopProfitPrice.split("-")
        if(Number(row.stockCurrentPrice)>=Number(prices[0])&&Number(row.stockCurrentPrice)<=Number(prices[1])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      if(row.recommendStopProfitPriceMin&&Number(row.stockCurrentPrice)>row.recommendStopProfitPriceMin){
        return [h(NText, { type: "success" }, { default: () => row.recommendStopProfitPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
      }

      return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
    }
  },
  {
    title: '止损价',
    key: 'recommendStopLossPrice',
    render(row, index) {
      if(vipLevel.value===""|| Number(vipLevel.value) <=0){
        return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })
      }
      if(row.recommendStopLossPrice.includes("-")){
        let prices= row.recommendStopLossPrice.split("-")
        if(Number(row.stockCurrentPrice)<=Number(prices[0])){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }else{
        let prices=row.recommendStopLossPrice
        if(Number(row.stockCurrentPrice)<=Number(prices)){
          return [h(NText, { type: "success" }, { default: () => row.recommendStopLossPrice }),h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Sell" })]
        }
      }
      return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })

    }
  },
  {
    title: '推荐理由',
    key: 'recommendReason',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '风险提示',
    key: 'riskRemarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '备注',
    key: 'remarks',
    ellipsis: {
      tooltip: isValidVip
    }
  },
  {
    title: '监控预警',
    key: 'enableAlert',
    width: 80,
    render(row, index) {
      return h(NSwitch, {
        value: row.enableAlert,
        onUpdateValue: (newValue) => toggleAlert(row, newValue)
      })
    }
  },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NTag,
          {
            strong: true,
            tertiary: true,
            //size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showDetail(row)
          },
          { default: () => '查看' }
      ),h(NTag, { strong: true,
        tertiary: true, type: 'error',  onClick: () => deleteAiRecommendStocks(row.ID) }, { default: () => '删除' })]
    }
  },
])
const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  enableAlert: null, // null 表示全部，true 表示已开启，false 表示未开启
  range: [
    new Date(new Date().getTime() - 3 * 24 * 60 * 60 * 1000), // 前3天
    new Date() // 当天
  ],
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const enableAlertOptions = [
  { label: '全部', value: null },
  { label: '已开启预警', value: true },
  { label: '未开启预警', value: false }
]

const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
  /** 实际使用的模型名 */
  modelName: "",
  /** 关联的系统提示词与用户提示词，用于追溯本次推荐的生成上下文 */
  systemPrompt: "",
  userPrompt: "",
  /** 是否显示生成上下文（默认收起，需点击按钮展开） */
  showContext: false,
  /** 传给 K 线组件的多单价位（与 StockLightweightKlineChart v-model 同步） */
  longEntryPrice: '',
  longStopLossPrice: '',
  longTakeProfitPrice: '',
})

const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})

// 查看弹窗 K 线图高度：随视口自适应，占满可用空间（弹窗下方还有内容/风险/上下文卡片）
const klineChartHeight = ref(400)
function updateKlineChartHeight() {
  const vh = window.innerHeight || 800
  // 约 55% 视口高度，上下限 400~700
  klineChartHeight.value = Math.min(700, Math.max(400, Math.floor(vh * 0.55)))
}
onMounted(() => {
  updateKlineChartHeight()
  window.addEventListener('resize', updateKlineChartHeight)
})
onUnmounted(() => {
  window.removeEventListener('resize', updateKlineChartHeight)
})


function query({
                 page,
                 pageSize = 10,
                 order = 'desc',
                 keyword = "",
                 startDate = "",
                 endDate = "",
                 enableAlert = null
               }) {
  return new Promise((resolve) => {

    GetAiRecommendStocksList({
      "page": page,
      "pageSize": pageSize,
      "modelName":keyword,
      "stockName":keyword,
      "stockCode":keyword,
      "bkName":keyword,
      "startDate": startDate,
      "endDate": endDate,
      "enableAlert": enableAlert
    }).then((res) => {
      const pagedData =res.list
      const total = res.total
      const pageCount =res.totalPages
      resolve({
        pageCount,
        data: pagedData,
        total
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]), // Format date to string
      endDate: formatDate(paginationReactive.range[1]), // Format date to string
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: paginationReactive?.page ?? 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1]),
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = data.page
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  // const hours = String(date.getHours()).padStart(2, '0')
  // const minutes = String(date.getMinutes()).padStart(2, '0')
  // const seconds = String(date.getSeconds()).padStart(2, '0')
  //return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  return `${year}-${month}-${day}`
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}

/** 推荐价可能为区间 "a-b"，取左侧作为图上开仓/止损/止盈线参考价 */
function recommendRangeToSinglePrice(p) {
  if (p == null || String(p).trim() === '') return ''
  const s = String(p).trim()
  const i = s.indexOf('-')
  if (i > 0) return s.slice(0, i).trim()
  return s
}

function showDetail(row) {
  if(vipLevel.value===""|| Number(vipLevel.value) <=0){
    notify.warning({content: '未开通VIP或者已经过期'})
    return
  }
  modalDataRef.title = row.stockName
  modalDataRef.content = row.recommendReason
  modalDataRef.riskRemarks = row.riskRemarks
  modalDataRef.stockCode = getStockCode(row.stockCode)
  modalDataRef.stockName = row.stockName
  modalDataRef.visible = true
  modalDataRef.remarks = row.remarks
  modalDataRef.modelName = row.modelName || ""
  modalDataRef.systemPrompt = row.systemPrompt || ""
  modalDataRef.userPrompt = row.userPrompt || ""
  modalDataRef.showContext = false
  modalDataRef.longEntryPrice = recommendRangeToSinglePrice(row.recommendBuyPrice)
  modalDataRef.longStopLossPrice = recommendRangeToSinglePrice(row.recommendStopLossPrice)
  modalDataRef.longTakeProfitPrice = recommendRangeToSinglePrice(row.recommendStopProfitPrice)
}
function rowProps(row) {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      showDetail(row)
    }
  }
}
function deleteAiRecommendStocks(id) {
  DeleteAiRecommendStocks(id).then((res) => {
    notify.info({content: res, duration: 2000})
    handleSearch()
    loadTodayStats()
  })
}

function toggleAlert(row, newEnableAlert) {
  UpdateAiRecommendStocksAlert(row.ID, newEnableAlert).then((res) => {
    notify.info({content: res, duration: 2000})
    // 更新本地数据
    row.enableAlert = newEnableAlert
  })
}

// ===== AI 推荐回测（统计页已独立为 RecommendBacktestStats 菜单页面）=====
const router = useRouter()
const backtestLoading = ref(false)
const backtestMapRef = ref({})   // recommendId -> outcome(win/lose)，主表格达标列使用
// 回测持有期（周期）：主表格「回测(N日)」列与「执行回测」按钮均按该周期
const backtestPeriodRef = ref(5)
const backtestPeriodOptions = [
  { label: '回测 5 日', value: 5 },
  { label: '回测 3 日', value: 3 },
  { label: '回测 10 日', value: 10 },
  { label: '回测 20 日', value: 20 },
  { label: '回测 30 日', value: 30 }
]

// 切换周期时同步列表勾选状态与表头文案
function onBacktestPeriodChange() {
  const col = columnsRef.value.find((c) => c.key === 'backtest')
  if (col) {
    col.title = `回测(${backtestPeriodRef.value}日)`
  }
  backtestMapRef.value = {}
  loadBacktestMap()
}

function normalizeBacktestItem(it) {
  const bt = it.AiRecommendBacktest || it || {}
  const recommendId = bt.RecommendId ?? bt.recommendId
  const outcome = bt.Outcome ?? bt.outcome
  return { recommendId, outcome }
}

async function loadBacktestMap() {
  let page = 1
  const pageSize = 200
  const map = {}
  let total = 1
  while (true) {
    const res = await ListRecommendBacktest(page, pageSize, backtestPeriodRef.value)
    const list = res?.list || []
    total = res?.total || 0
    for (const it of list) {
      const { recommendId, outcome } = normalizeBacktestItem(it)
      if (recommendId != null) map[recommendId] = outcome
    }
    if (list.length < pageSize || page * pageSize >= total) break
    page++
  }
  backtestMapRef.value = map
}

// 跳转独立的回测统计页面
function gotoBacktestStats() {
  router.push({ name: 'recommendBacktestStats' })
}

function runBacktest() {
  backtestLoading.value = true
  RunRecommendBacktest(backtestPeriodRef.value).then((res) => {
    notify.info({ content: res, duration: 4000 })
    backtestLoading.value = false
    loadBacktestMap()
  }).catch(() => {
    backtestLoading.value = false
  })
}

// 当前页签：推荐统计 / 历史表格
const activeTabRef = ref('stats')

// ===== 推荐统计 =====
const todayStatsRef = ref({ date: '', stockCount: 0, totalCount: 0, modelCount: 0, items: [] })
const todayStatsLoading = ref(false)
// 统计区所选日期（时间戳），默认今天
const statsDateRef = ref(Date.now())

// 统计区间快捷选项：以日期选择器所选日期为区间结束日，向前取 N 天
const statsRangeOptions = [
  {label: '今日', value: 1},
  {label: '近3日', value: 3},
  {label: '近5日', value: 5},
  {label: '近10日', value: 10},
  {label: '近20日', value: 20}
]
const statsDaysRef = ref(1)

function loadTodayStats(date) {
  const target = date ?? statsDateRef.value ?? Date.now()
  todayStatsLoading.value = true
  return GetAiRecommendStocksTodayStats(formatDate(new Date(target)), statsDaysRef.value).then((res) => {
    todayStatsRef.value = res && res.items ? res : { date: '', stockCount: 0, totalCount: 0, modelCount: 0, items: [] }
    todayStatsLoading.value = false
  }).catch(() => {
    todayStatsLoading.value = false
  })
}

// 切换统计日期
function onStatsDateChange(value) {
  statsDateRef.value = value
  loadTodayStats(value)
}

// 切换统计区间（今日/近3日/...）
function onStatsRangeChange(days) {
  if (statsDaysRef.value === days) return
  statsDaysRef.value = days
  loadTodayStats()
}

// 统计区间起始日期：结束日为所选日期，向前取 N-1 天
function statsStartDate() {
  const end = new Date(statsDateRef.value ?? Date.now())
  const start = new Date(end)
  start.setDate(start.getDate() - (statsDaysRef.value - 1))
  return start
}

// 现价是否落在建议开仓区间内
function isInBuyZone(row) {
  const cur = Number(row.stockCurrentPrice)
  if (!cur) return false
  const range = String(row.recommendBuyPrice ?? '').trim()
  if (range.includes('-')) {
    const prices = range.split('-')
    return cur >= Number(prices[0]) && cur <= Number(prices[1])
  }
  if (row.recommendBuyPriceMin && row.recommendBuyPriceMax) {
    return cur > Number(row.recommendBuyPriceMin) && cur < Number(row.recommendBuyPriceMax)
  }
  return false
}

// 价位可能是区间 "a-b"，取下沿单值用于比较
function priceLowerBound(price) {
  const str = String(price ?? '').trim()
  if (str === '') return NaN
  const idx = str.indexOf('-')
  return Number((idx > 0 ? str.slice(0, idx) : str).trim())
}

// 现价相对止损价/目标价的所处状态，方便一眼判断能否介入
function zoneStatus(row) {
  const cur = Number(row.stockCurrentPrice)
  if (!cur) return null
  const stopLoss = priceLowerBound(row.recommendStopLossPrice)
  const takeProfit = Number(row.recommendStopProfitPriceMin) || priceLowerBound(row.recommendStopProfitPrice)
  if (!Number.isNaN(stopLoss) && stopLoss > 0 && cur <= stopLoss) return { text: '破止损', type: 'success' }
  if (takeProfit && cur >= takeProfit) return { text: '达目标', type: 'error' }
  if (isInBuyZone(row)) return { text: '开仓区', type: 'warning' }
  return null
}

const todayStatsInBuyCount = computed(() => {
  return (todayStatsRef.value.items || []).filter((it) => isInBuyZone(it)).length
})

// A股习惯：红涨绿跌
function changeTagType(row) {
  const cur = Number(row.stockCurrentPrice)
  const pre = Number(row.stockPrePrice)
  if (!cur || !pre || cur === pre) return 'info'
  return cur > pre ? 'error' : 'success'
}

function changeRateText(row) {
  const cur = Number(row.stockCurrentPrice)
  const pre = Number(row.stockPrePrice)
  if (!cur || !pre) return ''
  return `${((cur - pre) / pre * 100).toFixed(2)}%`
}

function stockCardTip(item) {
  const models = (item.modelNames || []).join('、')
  return `${item.stockName} ${item.stockCode}\n当日推荐 ${item.count} 次｜首次 ${item.firstTime}｜最近 ${item.lastTime}` +
      `${models ? '\n推荐模型：' + models : ''}\n点击跳转到历史表格查看该股推荐记录`
}

// 点击股池卡片：按该股票、统计区所选区间筛历史表格，并自动切到历史表格页签
// 列表 stock_code 存的是带后缀格式（如 600519.SH），统计接口返回的是归一化代码（sh600519），
// 直接用后者搜索匹配不到，这里截取纯数字代码，保证两种格式都能被 LIKE 命中
function filterByStock(item) {
  paginationReactive.keyword = String(item.stockCode || '').replace(/[^0-9]/g, '')
  const end = new Date(statsDateRef.value ?? Date.now())
  paginationReactive.range = [statsStartDate(), end]
  activeTabRef.value = 'table'
  handlePageChange(1)
}

// 历史表格高度：页签拆分后统计区与表格不再同屏，直接用固定高度
const tableHeightStyle = {
  height: 'max(240px, calc(100vh - 250px))',
  marginTop: '10px'
}

</script>

<template>
  <n-tabs type="line" animated size="small" display-directive="show" v-model:value="activeTabRef">
    <n-tab-pane name="stats" tab="推荐统计">
      <div class="today-stats">
        <div class="today-stats__header">
          <n-gradient-text type="primary" :size="16">推荐统计</n-gradient-text>
          <n-date-picker size="tiny" v-model:value="statsDateRef" type="date" :clearable="false"
                         style="width: 130px" @update:value="onStatsDateChange" />
          <n-tag v-for="opt in statsRangeOptions" :key="opt.value" size="small" :bordered="false"
                 :type="statsDaysRef === opt.value ? 'primary' : 'default'" style="cursor:pointer"
                 @click="onStatsRangeChange(opt.value)">{{opt.label}}</n-tag>
          <n-text depth="3" style="font-size:12px">{{todayStatsRef.date}}</n-text>
          <n-tag size="small" :bordered="false" type="info">股池 {{todayStatsRef.stockCount}} 只</n-tag>
          <n-tag size="small" :bordered="false" type="info">推荐 {{todayStatsRef.totalCount}} 次</n-tag>
          <n-tag size="small" :bordered="false" type="info">模型 {{todayStatsRef.modelCount}} 个</n-tag>
          <n-tag size="small" :bordered="false" type="warning">开仓区间 {{todayStatsInBuyCount}} 只</n-tag>
          <n-text depth="3" style="font-size:12px">点击卡片查看该股推荐记录</n-text>
          <div style="flex:1"></div>
          <n-button size="tiny" quaternary :loading="todayStatsLoading" @click="() => loadTodayStats()">刷新</n-button>
          <n-button size="tiny" quaternary @click="todayStatsCollapsed = !todayStatsCollapsed">
            {{ todayStatsCollapsed ? '展开' : '收起' }}
          </n-button>
        </div>
        <div class="today-stats__pool" v-show="!todayStatsCollapsed">
          <div class="today-stats__empty" v-if="!todayStatsRef.items.length">当日暂无推荐记录</div>
          <div class="pool-card" v-for="it in todayStatsRef.items" :key="it.stockCode"
               :title="stockCardTip(it)" @click="filterByStock(it)">
            <div class="pool-card__top">
              <n-text strong style="font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{it.stockName}}</n-text>
              <n-tag size="tiny" type="warning" :bordered="false">×{{it.count}}</n-tag>
              <n-tag v-if="zoneStatus(it)" size="tiny" :bordered="false" :type="zoneStatus(it).type">
                {{zoneStatus(it).text}}
              </n-tag>
            </div>
            <div class="pool-card__top">
              <n-text depth="3" style="font-size:12px">{{it.stockCode}}</n-text>
              <n-tag v-if="it.stockCurrentPrice" size="tiny" :bordered="false" :type="changeTagType(it)">
                {{it.stockCurrentPrice}} {{changeRateText(it)}}
              </n-tag>
              <n-text v-else depth="3" style="font-size:12px">现价 -</n-text>
            </div>
            <div class="pool-card__prices">
              <span><i>开仓</i>{{it.recommendBuyPrice || '-'}}</span>
              <span><i>目标</i>{{it.recommendStopProfitPrice || '-'}}</span>
              <span><i>止损</i>{{it.recommendStopLossPrice || '-'}}</span>
            </div>
            <div class="pool-card__top">
              <n-tag v-if="it.rating" size="tiny" :bordered="false" type="success">{{it.rating}}</n-tag>
              <n-text depth="3" style="font-size:11px" class="pool-card__meta">{{it.bkName || '-'}}</n-text>
              <n-text depth="3" style="font-size:11px">最近 {{it.lastTime}}</n-text>
            </div>
          </div>
        </div>
      </div>
    </n-tab-pane>
    <n-tab-pane name="table" tab="历史表格">
      <n-input-group>
        <n-date-picker  v-model:value="paginationReactive.range" type="daterange"   style="width: 40%"/>
        <n-select v-model:value="paginationReactive.enableAlert" :options="enableAlertOptions" placeholder="预警状态" style="width: 15%" clearable />
        <n-input clearable placeholder="输入关键词搜索" v-model:value="paginationReactive.keyword"/>
        <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
          搜索
        </n-button>
      </n-input-group>
      <div style="display:flex; gap:8px; align-items:center; margin-top:8px;">
        <n-select size="small" v-model:value="backtestPeriodRef" :options="backtestPeriodOptions" style="width: 130px" @update:value="onBacktestPeriodChange" />
        <n-button size="small" type="primary" ghost :loading="backtestLoading" @click="runBacktest">
          执行回测({{backtestPeriodRef}}日)
        </n-button>
        <n-button size="small" type="info" ghost @click="gotoBacktestStats">
          回测统计
        </n-button>
      </div>
      <n-data-table
          remote
          size="small"
          :columns="columnsRef"
          :data="dataRef"
          :loading="loadingRef"
          :pagination="paginationReactive"
          :row-key="(rowData)=>rowData.ID"
          @update:page="handlePageChange"
          flex-height
          :style="tableHeightStyle"
      />
    </n-tab-pane>
  </n-tabs>

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="max-width: 1400px;">
    <n-gradient-text :size="16" type="warning">{{modalDataRef.remarks}}</n-gradient-text>
    <n-card size="small">
      <StockLightweightKlineChart
        style="width: 100%;"
        :code="modalDataRef.stockCode"
        :chart-height="klineChartHeight"
        :stock-name="modalDataRef.stockName"
        :dark-theme="editorDataRef.darkTheme"
        v-model:long-entry-price="modalDataRef.longEntryPrice"
        v-model:long-stop-loss-price="modalDataRef.longStopLossPrice"
        v-model:long-take-profit-price="modalDataRef.longTakeProfitPrice"
      />
    </n-card>
    <n-card size="small">
    <n-text type="info">{{modalDataRef.content}}</n-text>
    <n-divider><n-gradient-text type="error">风险提示</n-gradient-text></n-divider>
    <n-text type="error">{{modalDataRef.riskRemarks}}</n-text>
    </n-card>
    <n-card size="small" v-if="modalDataRef.systemPrompt || modalDataRef.userPrompt">
      <div style="display:flex; align-items:center; gap:8px; margin-bottom: 8px;">
        <n-text depth="3">生成上下文（模型：{{modalDataRef.modelName}}）</n-text>
        <n-button size="tiny" type="info" tertiary @click="modalDataRef.showContext = !modalDataRef.showContext">
          {{ modalDataRef.showContext ? '收起上下文' : '查看生成上下文' }}
        </n-button>
      </div>
      <template v-if="modalDataRef.showContext">
        <div style="text-align:left;" v-if="modalDataRef.systemPrompt">
          <n-text depth="3" style="font-weight: 600;">系统提示词：</n-text>
          <div style="max-height: 240px; overflow-y: auto; padding: 4px 0; text-align:left;">
            <MdPreview :model-value="modalDataRef.systemPrompt" :theme="theme" />
          </div>
        </div>
        <div style="text-align:left;" v-if="modalDataRef.userPrompt">
          <n-text depth="3" style="font-weight: 600;">用户提示词：</n-text>
          <div style="max-height: 240px; overflow-y: auto; padding: 4px 0; text-align:left;">
            <MdPreview :model-value="modalDataRef.userPrompt" :theme="theme" />
          </div>
        </div>
      </template>
    </n-card>
  </n-modal>

</template>

<style scoped>
/* ===== 推荐统计 ===== */
.today-stats {
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  padding: 6px 8px;
  margin-bottom: 4px;
}

.today-stats__header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

/* 卡片换行铺满，不使用任何滚动容器 */
.today-stats__pool {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 6px 2px 4px;
}

.today-stats__empty {
  font-size: 12px;
  color: var(--n-text-color-3);
  padding: 12px 0;
}

.pool-card {
  flex: 0 0 auto;
  width: 178px;
  height: 104px;
  box-sizing: border-box;
  padding: 6px 8px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  overflow: hidden;
  cursor: pointer;
  transition: border-color .2s, box-shadow .2s;
}

.pool-card:hover {
  border-color: var(--n-primary-color-hover);
  box-shadow: 0 1px 6px rgba(0, 0, 0, .15);
}

.pool-card__top {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.pool-card__meta {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pool-card__prices {
  display: flex;
  justify-content: space-between;
  gap: 4px;
  font-size: 12px;
}

.pool-card__prices span {
  display: flex;
  gap: 2px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pool-card__prices i {
  font-style: normal;
  color: var(--n-text-color-3);
}
</style>
