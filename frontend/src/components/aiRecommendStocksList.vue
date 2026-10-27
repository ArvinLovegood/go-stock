<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted, ref, reactive, watch} from 'vue'
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
  ListRecommendBacktest,
  GetStocksRealtimeQuotes
} from "../../wailsjs/go/main/App";
import {NAvatar, NButton, NEllipsis, NSwitch, NTag, NText, useMessage, useNotification} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from "./kline/useKlineModalFit";
import { detectRecentBuySell, formatSignalTime, SIGNAL_INTERVAL_OPTIONS } from "./kline/signalMonitor";
import { BUY_SELL_SCORE_OPTIONS } from "./kline/constants";
import { BUY_SELL_MAX_SCORE } from "./kline/calc";
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
    // 追溯用：AI 配置名（用户自定义）+ 提示词模板版本 + 策略提示词哈希前 8 位
    title: '配置/提示词',
    key: 'configName',
    width: 160,
    ellipsis: { tooltip: true },
    render(row) {
      const name = row.configName || '—'
      const extra = [
        row.sysPromptVersion ? `v${row.sysPromptVersion}` : '',
        row.promptHash ? row.promptHash.slice(0, 8) : ''
      ].filter(Boolean).join(' ')
      return h(NText, { type: "info" }, { default: () => (extra ? `${name} · ${extra}` : name) })
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
  /** 归因用：用户自定义 AI 配置名 */
  configName: "",
  /** 归因用：策略提示词哈希（SHA-256 前 16 位） */
  promptHash: "",
  /** 归因用：系统提示词模板版本号（0=内置/无模板） */
  sysPromptVersion: 0,
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

// 查看弹窗：宽度与全站其他 K 线弹窗统一；图表下方还有内容/风险/上下文卡片，故取比例高度不参与铺满收敛
const klineWrapRef = ref(null)
const { chartHeight: klineChartHeight, attach: attachKlineFit, detach: detachKlineFit } = useKlineModalFit(klineWrapRef, { scrollable: true })

watch(() => modalDataRef.visible, (v) => {
  if (v) {
    attachKlineFit()
    return
  }
  detachKlineFit()
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
  modalDataRef.configName = row.configName || ""
  modalDataRef.promptHash = row.promptHash || ""
  modalDataRef.sysPromptVersion = row.sysPromptVersion || 0
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

// ===== 买卖点信号快照 =====
// 扫描「推荐统计」所选区间的推荐股池，复用「信号监控」面板同一套算法（14 路共振 + 图表门控）：
// 取数口径（复权、最近一年窗口、MIN_BARS 门控）与买卖点算法都走 kline/signalMonitor 的 detectRecentBuySell，避免两处定义漂移。
const SIGNAL_FETCH_CONCURRENCY = 4
const signalKltRef = ref('101')
const signalScoreRef = ref(6)
const signalDaysRef = ref(2)
const signalLoading = ref(false)
const signalScanned = ref(false)
const signalInsufficientRef = ref(0)
const signalProgress = reactive({ done: 0, total: 0 })
const signalRowsRef = ref([])

const signalKltOptions = SIGNAL_INTERVAL_OPTIONS.map((it) => ({ label: it.label, value: it.klt }))
const signalScoreOptions = BUY_SELL_SCORE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))
// 信号出现的时间范围：以 K 线最新一根所在交易日为基准回溯，周末/节假日不错位
const signalRangeOptions = [
  { label: '今日', value: 1 },
  { label: '近 2 日', value: 2 },
  { label: '近 5 日', value: 5 }
]

// ===== 定时扫描 + 新信号推送：设置持久化，定时触发与手动同走 scanSignals，只对增量弹通知 =====
const SIGNAL_AUTO_KEY = 'ai-recommend-signal-autoscan'
const signalAutoRef = ref(false)
const signalAutoMinRef = ref(15)
const signalLastScanAtRef = ref('')
const signalAutoOptions = [5, 10, 30, 60].map((m) => ({ label: `${m} 分钟`, value: m }))
let signalAutoTimer = null
/** 上一轮信号键（code|kind|time）与扫描参数：定时扫描据此 diff 出新信号；参数变化即重置基线（换参数后首轮不通知） */
let lastSignalKeys = new Set()
let lastSignalParams = ''

function persistSignalAuto() {
  localStorage.setItem(SIGNAL_AUTO_KEY, JSON.stringify({
    enabled: signalAutoRef.value,
    intervalMin: signalAutoMinRef.value,
    klt: signalKltRef.value,
    score: signalScoreRef.value,
    days: signalDaysRef.value,
  }))
}
try {
  const saved = JSON.parse(localStorage.getItem(SIGNAL_AUTO_KEY) || '{}')
  if (typeof saved.enabled === 'boolean') signalAutoRef.value = saved.enabled
  if (signalAutoOptions.some((o) => o.value === saved.intervalMin)) signalAutoMinRef.value = saved.intervalMin
  if (SIGNAL_INTERVAL_OPTIONS.some((o) => o.klt === saved.klt)) signalKltRef.value = saved.klt
  if (BUY_SELL_SCORE_OPTIONS.some((o) => o.value === saved.score)) signalScoreRef.value = saved.score
  if (signalRangeOptions.some((o) => o.value === saved.days)) signalDaysRef.value = saved.days
} catch { /* 脏数据忽略 */ }

// 强度分档：命中路数越多越强（14 路全中=100%）。阈值按满分比例维持（10 路 7/5 ≈ 14 路 10/7）
function strengthType(score) {
  if (score >= 10) return 'error'
  if (score >= 7) return 'warning'
  return 'info'
}

// ===== 方向筛选：纯前端过滤扫描结果，不影响扫描本身与定时通知基线 =====
const signalKindRef = ref('')
const signalKindOptions = [
  { label: '全部', value: '' },
  { label: '买点', value: 'buy' },
  { label: '卖点', value: 'sell' },
]
const visibleSignalRows = computed(() => {
  const k = signalKindRef.value
  return k ? signalRowsRef.value.filter((s) => s.kind === k) : signalRowsRef.value
})
/** 命中数文案：筛选生效时显示「筛选后/总数」，未筛选只显示总数 */
const signalCountText = computed(() => {
  const total = signalRowsRef.value.length
  return signalKindRef.value ? `${visibleSignalRows.value.length}/${total} 只` : `${total} 只`
})

// ===== 最新行情：随每次扫描批量拉取，供信号表展示最新价/当日涨跌幅/相对信号点涨跌幅 =====
const quoteMapRef = ref({})

/** 行情键归一：新浪美股前缀 gb_ 与 us 等价（gb_aapl ≡ usaapl），其余小写直用 */
function quoteKey(code) {
  const c = (code || '').toLowerCase()
  return c.startsWith('gb_') ? 'us' + c.slice(3) : c
}

/** 批量拉最新行情（失败静默保留旧缓存，不打断信号展示） */
async function refreshSignalQuotes(rows) {
  const codes = [...new Set((rows || []).map((r) => r.code).filter(Boolean))]
  if (!codes.length) {
    quoteMapRef.value = {}
    return
  }
  try {
    const quotes = await GetStocksRealtimeQuotes(codes)
    const map = {}
    for (const q of quotes || []) map[quoteKey(q.code)] = q
    quoteMapRef.value = map
  } catch (e) { /* 行情拉取失败不影响信号列表 */ }
}

// 逐只票扫描最近窗口内的买卖点；并发受限，单只失败不影响其余；扫描不限数量，但先按代码去重
// silent=true 为定时触发：股池为空不弹警告，完成后与上一轮 diff，新增信号走应用内通知
async function scanSignals(silent = false) {
  if (signalLoading.value) return
  const items = todayStatsRef.value.items || []
  if (!items.length) {
    if (!silent) notify.warning({ content: '推荐股池为空，请先调整上方的日期区间', duration: 3000 })
    return
  }
  // 同一只票可能被多日/多模型重复推荐，按归一化代码去重，避免重复取数与重复信号
  const seen = new Set()
  const list = []
  for (const it of items) {
    const code = String(it.stockCode || '').trim()
    if (!code || seen.has(code)) continue
    seen.add(code)
    list.push(it)
  }
  signalLoading.value = true
  signalScanned.value = true
  signalRowsRef.value = []
  signalInsufficientRef.value = 0
  signalProgress.done = 0
  signalProgress.total = list.length
  const klt = signalKltRef.value
  const score = signalScoreRef.value
  const days = signalDaysRef.value
  const out = []
  let cursor = 0
  try {
    const workers = new Array(Math.min(SIGNAL_FETCH_CONCURRENCY, list.length)).fill(0).map(async () => {
      while (true) {
        const idx = cursor++
        if (idx >= list.length) return
        const it = list[idx]
        try {
          const res = await detectRecentBuySell(it.stockCode, it.stockName, klt, score, days)
          if (res.insufficient) signalInsufficientRef.value++
          for (const s of res.signals) out.push(s)
        } catch (e) { /* 单只票取数/计算失败跳过 */ }
        signalProgress.done++
      }
    })
    await Promise.all(workers)
    // 默认按信号强度降序（强度=共振路数）；同强度再按信号时间新的在前
    out.sort((a, b) => ((b.score || 0) - (a.score || 0)) || (b.time - a.time))
    signalRowsRef.value = out
    // 批量刷新最新行情（异步，不阻塞信号表渲染）
    refreshSignalQuotes(out)
    // 通知基线：定时扫描与上轮同参数时对增量弹通知；手动扫描只刷新基线（用户正盯着结果页）
    const params = `${klt}|${score}|${days}`
    const keys = new Set(out.map((s) => `${s.code}|${s.kind}|${s.time}`))
    if (silent && params === lastSignalParams) {
      const fresh = out.filter((s) => !lastSignalKeys.has(`${s.code}|${s.kind}|${s.time}`))
      if (fresh.length) {
        const lines = fresh.slice(0, 6).map((s) => `${s.name} ${s.kind === 'buy' ? '买点' : '卖点'}（${s.score}/${BUY_SELL_MAX_SCORE} 路）`)
        notify.info({
          title: `推荐股池买卖点：新增 ${fresh.length} 个信号`,
          content: lines.join('；') + (fresh.length > 6 ? ' 等' : ''),
          duration: 8000,
        })
      }
    }
    lastSignalKeys = keys
    lastSignalParams = params
    signalLastScanAtRef.value = new Date().toTimeString().slice(0, 5)
  } finally {
    signalLoading.value = false
  }
}

// 定时扫描器：间隔可调，扫描进行中跳过本轮；页面卸载即停
function stopSignalAuto() {
  if (signalAutoTimer) {
    clearInterval(signalAutoTimer)
    signalAutoTimer = null
  }
}
function startSignalAuto() {
  stopSignalAuto()
  signalAutoTimer = setInterval(() => scanSignals(true), signalAutoMinRef.value * 60000)
}
watch(signalAutoRef, (v) => {
  persistSignalAuto()
  if (v) startSignalAuto()
  else stopSignalAuto()
})
watch(signalAutoMinRef, () => {
  persistSignalAuto()
  if (signalAutoRef.value) startSignalAuto()
})
watch([signalKltRef, signalScoreRef, signalDaysRef], persistSignalAuto)
if (signalAutoRef.value) startSignalAuto()
onBeforeUnmount(stopSignalAuto)

// 信号行打开 K 线：复用页面已有的弹窗（代码为归一化格式，与图表口径一致）
function openSignalKline(row) {
  const score = Number(row.score) || 0
  modalDataRef.title = row.name
  modalDataRef.stockCode = row.code
  modalDataRef.stockName = row.name
  modalDataRef.remarks = ''
  modalDataRef.content = `${row.kind === 'buy' ? '买点' : '卖点'}信号 · 强度 ${Math.round((score / BUY_SELL_MAX_SCORE) * 100)}%（${score}/${BUY_SELL_MAX_SCORE} 路）· ${formatSignalTime(row.time)}`
  modalDataRef.riskRemarks = (row.reasons || []).join('、')
  modalDataRef.modelName = ''
  modalDataRef.systemPrompt = ''
  modalDataRef.userPrompt = ''
  modalDataRef.configName = ''
  modalDataRef.promptHash = ''
  modalDataRef.sysPromptVersion = 0
  modalDataRef.showContext = false
  modalDataRef.longEntryPrice = ''
  modalDataRef.longStopLossPrice = ''
  modalDataRef.longTakeProfitPrice = ''
  modalDataRef.visible = true
}

const signalColumns = [
  {
    title: '股票', key: 'name', width: 180,
    render: (row) => [
      h(NText, { type: 'info' }, { default: () => row.name }),
      h(NText, { depth: 3, style: 'font-size:12px; margin-left:6px;' }, { default: () => row.code })
    ]
  },
  {
    title: '方向', key: 'kind', width: 70,
    render: (row) => h(NTag, { size: 'tiny', bordered: false, type: row.kind === 'buy' ? 'error' : 'success' },
      { default: () => row.kind === 'buy' ? '买点' : '卖点' })
  },
  {
    title: '强度', key: 'score', width: 140,
    render: (row) => {
      const score = Number(row.score) || 0
      return [
        h(NTag, { size: 'tiny', bordered: false, type: strengthType(score) }, { default: () => `${Math.round((score / BUY_SELL_MAX_SCORE) * 100)}%` }),
        h(NText, { depth: 3, style: 'font-size:12px; margin-left:6px;' }, { default: () => `${score}/${BUY_SELL_MAX_SCORE} 路` })
      ]
    }
  },
  { title: '信号时间', key: 'time', width: 140, render: (row) => formatSignalTime(row.time) },
  { title: '信号价', key: 'price', width: 80, render: (row) => row.price != null ? row.price.toFixed(2) : '-' },
  {
    // 最新价：主行为最新价，副行为当日涨跌幅（A股习惯红涨绿跌）
    title: '最新价', key: 'last', width: 110,
    render: (row) => {
      const q = quoteMapRef.value[quoteKey(row.code)]
      if (!q || !q.price) return h(NText, { depth: 3, style: 'font-size:12px;' }, { default: () => '—' })
      const pct = Number(q.changePercent) || 0
      return [
        h('div', { style: 'font-weight:600;line-height:1.3;' }, { default: () => q.price.toFixed(2) }),
        h(NText, { type: pct > 0 ? 'error' : pct < 0 ? 'success' : 'default', style: 'font-size:11px;' },
          { default: () => `今 ${pct > 0 ? '+' : ''}${pct.toFixed(2)}%` })
      ]
    }
  },
  {
    // 相对信号点涨跌幅：(最新价 - 信号价) / 信号价；买点看涨、卖点看跌各有含义，按涨跌着色
    title: '相对信号', key: 'chg', width: 90,
    render: (row) => {
      const q = quoteMapRef.value[quoteKey(row.code)]
      const base = Number(row.price) || 0
      if (!q || !q.price || !base) return h(NText, { depth: 3, style: 'font-size:12px;' }, { default: () => '—' })
      const pct = ((q.price - base) / base) * 100
      return h(NText, { type: pct > 0 ? 'error' : pct < 0 ? 'success' : 'default', strong: true },
        { default: () => `${pct > 0 ? '+' : ''}${pct.toFixed(2)}%` })
    }
  },
  { title: '距最新', key: 'lag', width: 80, render: (row) => row.lag === 0 ? '最新根' : `前 ${row.lag} 根` },
  { title: '命中信号', key: 'reasons', ellipsis: { tooltip: true }, render: (row) => (row.reasons || []).join('、') || '—' },
  {
    title: '操作', key: 'op', width: 70,
    render: (row) => h(NButton, { size: 'tiny', quaternary: true, onClick: () => openSignalKline(row) }, { default: () => 'K线' })
  }
]

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
    <n-tab-pane name="signal" tab="买卖点信号">
      <div class="signal-bar">
        <n-date-picker size="tiny" v-model:value="statsDateRef" type="date" :clearable="false"
                       style="width: 130px" @update:value="onStatsDateChange" />
        <n-tag v-for="opt in statsRangeOptions" :key="opt.value" size="small" :bordered="false"
               :type="statsDaysRef === opt.value ? 'primary' : 'default'" style="cursor:pointer"
               @click="onStatsRangeChange(opt.value)">{{opt.label}}</n-tag>
        <n-select size="small" v-model:value="signalKltRef" :options="signalKltOptions" style="width: 110px" />
        <n-select size="small" v-model:value="signalScoreRef" :options="signalScoreOptions" style="width: 110px" />
        <n-select size="small" v-model:value="signalDaysRef" :options="signalRangeOptions" style="width: 110px" />
        <n-button size="small" type="primary" ghost :loading="signalLoading" @click="scanSignals()">扫描买卖点</n-button>
        <n-select size="small" v-model:value="signalKindRef" :options="signalKindOptions" style="width: 92px" />
        <n-divider vertical />
        <n-text depth="3" style="font-size:12px">定时</n-text>
        <n-switch size="small" v-model:value="signalAutoRef" />
        <n-select size="small" v-model:value="signalAutoMinRef" :options="signalAutoOptions"
                  style="width: 92px" :disabled="!signalAutoRef" />
        <n-text depth="3" style="font-size:12px" v-if="signalLastScanAtRef">上次扫描 {{signalLastScanAtRef}}</n-text>
        <n-text depth="3" style="font-size:12px" v-if="signalLoading">扫描中 {{signalProgress.done}}/{{signalProgress.total}}</n-text>
        <n-text depth="3" style="font-size:12px" v-else-if="signalScanned">命中 {{signalCountText}} · 数据不足 {{signalInsufficientRef}} 只</n-text>
      </div>
      <n-text depth="3" style="font-size:12px; display:block; margin: 6px 0;">
        扫描范围：「推荐统计」所选区间（{{todayStatsRef.date || '今日'}}）的推荐股池，按股票代码去重后全量扫描（不限数量）；
        K 线取最近一年（不低于信号门控所需最少根数），检测口径与「信号监控」面板一致（14 路共振 + 图表门控），强度=命中路数/14。
        所选时间范围内命中的信号全部列出（同一股票同方向多条信号会逐条展示）。
        开启「定时」后按间隔自动扫描，发现新增信号推送应用内通知（调整周期/灵敏度/时间范围后的首轮不通知，重新建立基线）。
      </n-text>
      <n-data-table
        size="small"
        :columns="signalColumns"
        :data="visibleSignalRows"
        :loading="signalLoading"
        :row-key="(row) => row.code + '|' + row.kind + '|' + row.time"
        :style="{ height: 'max(240px, calc(100vh - 330px))' }"
      />
    </n-tab-pane>
  </n-tabs>

  <n-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    preset="card"
    :style="KLINE_MODAL_STYLE"
    :content-style="{
      ...KLINE_MODAL_CONTENT_STYLE,
      overflowY: 'auto',
      // 图表之外还有内容/风险/上下文卡片，给它们留出空间后仍让整卡控制在 94vh 内
      maxHeight: 'calc(94vh - 120px)',
    }"
  >
    <n-gradient-text :size="16" type="warning">{{modalDataRef.remarks}}</n-gradient-text>
    <n-card size="small">
      <div ref="klineWrapRef">
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
      </div>
    </n-card>
    <n-card size="small">
    <n-text type="info">{{modalDataRef.content}}</n-text>
    <n-divider><n-gradient-text type="error">风险提示</n-gradient-text></n-divider>
    <n-text type="error">{{modalDataRef.riskRemarks}}</n-text>
    </n-card>
    <n-card size="small" v-if="modalDataRef.systemPrompt || modalDataRef.userPrompt">
      <div style="display:flex; align-items:center; gap:8px; margin-bottom: 8px;">
        <n-text depth="3">生成上下文（模型：{{modalDataRef.modelName}}<template v-if="modalDataRef.configName">；配置：{{modalDataRef.configName}}</template><template v-if="modalDataRef.sysPromptVersion || modalDataRef.promptHash">；模板 v{{modalDataRef.sysPromptVersion || 0 }} / 哈希 {{(modalDataRef.promptHash || '').slice(0, 8) || '—'}}</template>）</n-text>
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
/* ===== 买卖点信号 ===== */
.signal-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

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
