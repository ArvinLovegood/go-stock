<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {
  GetAiRecommendStocksList,
  GetConfig,
  GetSponsorInfo,
  DeleteAiRecommendStocks,
  UpdateAiRecommendStocksAlert,
  ShareAnalysis,
  RunRecommendBacktest,
  ListRecommendBacktest,
  ListRecommendBacktestByPrompt,
  GetRecommendBacktestStats
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
  })
}

function toggleAlert(row, newEnableAlert) {
  UpdateAiRecommendStocksAlert(row.ID, newEnableAlert).then((res) => {
    notify.info({content: res, duration: 2000})
    // 更新本地数据
    row.enableAlert = newEnableAlert
  })
}

// ===== AI 推荐回测（P3）=====
const backtestLoading = ref(false)
const backtestMapRef = ref({})   // recommendId -> outcome(win/lose)
const backtestStatsRef = ref(null)
const backtestStatsVisible = ref(false)
const backtestListRef = ref([])
const backtestTotalRef = ref(0)
const backtestListLoading = ref(false)
const backtestPageRef = ref(1)
const backtestPageSizeRef = ref(10)
// 当前按提示词过滤条件：{ type: 'sys'|'usr', content, label }，null 表示不过滤
const backtestPromptFilter = ref(null)
const backtestPromptFilterLabel = computed(() => {
  const f = backtestPromptFilter.value
  if (!f) return ''
  return (f.type === 'sys' ? '系统提示词' : '用户提示词') + '：' + f.label
})

const backtestListColumns = [
  { title: '推荐时间', key: 'time', render: (row) => row.recommendTimeStr || '-' },
  { title: '股票', key: 'stock', render: (row) => `${row.stockName} ${row.stockCode}` },
  { title: '周期', key: 'periodDays', width: 70 },
  { title: '推荐价', key: 'recommendPrice', width: 90 },
  { title: '期末价', key: 'endPrice', width: 90 },
  { title: '收益%', key: 'returnPct', width: 90, render: (row) => h(NText, { type: row.returnPct >= 0 ? 'error' : 'success' }, { default: () => row.returnPct?.toFixed ? row.returnPct.toFixed(2) : row.returnPct }) },
  { title: '基准%', key: 'benchmarkPct', width: 80, render: (row) => row.benchmarkPct?.toFixed ? row.benchmarkPct.toFixed(2) : row.benchmarkPct },
  { title: '超额%', key: 'excessPct', width: 80, render: (row) => row.excessPct?.toFixed ? row.excessPct.toFixed(2) : row.excessPct },
  { title: '结果', key: 'outcome', width: 90, render: (row) => row.outcome === 'win' ? h(NTag, { size: 'tiny', type: 'error', bordered: false }, { default: () => '达标' }) : h(NTag, { size: 'tiny', type: 'success', bordered: false }, { default: () => '未达标' }) },
]

function normalizeBacktestItem(it) {
  const bt = it.AiRecommendBacktest || it || {}
  const recommendId = bt.RecommendId ?? bt.recommendId
  const outcome = bt.Outcome ?? bt.outcome
  const stockName = bt.StockName ?? bt.stockName
  const stockCode = bt.StockCode ?? bt.stockCode
  const periodDays = bt.PeriodDays ?? bt.periodDays
  const recommendPrice = bt.RecommendPrice ?? bt.recommendPrice
  const endPrice = bt.EndPrice ?? bt.endPrice
  const returnPct = bt.ReturnPct ?? bt.returnPct
  const benchmarkPct = bt.BenchmarkPct ?? bt.benchmarkPct
  const excessPct = bt.ExcessPct ?? bt.excessPct
  return {
    recommendId, outcome,
    stockName, stockCode, periodDays, recommendPrice,
    endPrice, returnPct, benchmarkPct, excessPct,
    recommendTimeStr: it.recommendTimeStr || '',
  }
}

async function loadBacktestMap() {
  let page = 1
  const pageSize = 200
  const map = {}
  let total = 1
  while (true) {
    const res = await ListRecommendBacktest(page, pageSize)
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

function loadBacktestStats() {
  GetRecommendBacktestStats().then((res) => {
    backtestStatsRef.value = res || null
  }).catch(() => {
    backtestStatsRef.value = null
  })
}

function loadBacktestList(p) {
  const page = p || backtestPageRef.value
  backtestPageRef.value = page
  backtestListLoading.value = true
  const f = backtestPromptFilter.value
  const req = f
    ? ListRecommendBacktestByPrompt(page, backtestPageSizeRef.value, f.content, f.type)
    : ListRecommendBacktest(page, backtestPageSizeRef.value)
  try {
    req.then((res) => {
      const list = res?.list || []
      backtestTotalRef.value = res?.total || 0
      backtestListRef.value = list.map(normalizeBacktestItem)
    }).catch((e) => {
      backtestListRef.value = []
      backtestTotalRef.value = 0
      notify.error({ content: '加载回测明细失败：' + (e?.message || e || '未知错误'), duration: 4000 })
    }).finally(() => {
      backtestListLoading.value = false
    })
  } catch (e) {
    backtestListRef.value = []
    backtestTotalRef.value = 0
    backtestListLoading.value = false
    notify.error({ content: '加载回测明细失败：' + (e?.message || e || '未知错误'), duration: 4000 })
  }
}

// 点击某条提示词统计，按该提示词过滤下方「最近回测明细」
function filterBacktestByPrompt(type, g) {
  if (!g || !g.content) {
    notify.warning({ content: '该提示词内容为空，无法过滤', duration: 2000 })
    return
  }
  backtestPromptFilter.value = { type, content: g.content, label: g.name || g.content }
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 清除提示词过滤条件
function clearBacktestPromptFilter() {
  backtestPromptFilter.value = null
  backtestPageRef.value = 1
  loadBacktestList(1)
}

function openBacktestStats() {
  backtestStatsVisible.value = true
  backtestPageRef.value = 1
  backtestPromptFilter.value = null
  loadBacktestStats()
  loadBacktestList(1)
}

function runBacktest() {
  backtestLoading.value = true
  RunRecommendBacktest(5).then((res) => {
    notify.info({ content: res, duration: 4000 })
    backtestLoading.value = false
    loadBacktestMap()
    loadBacktestStats()
    if (backtestStatsVisible.value) loadBacktestList(1)
  }).catch(() => {
    backtestLoading.value = false
  })
}

</script>

<template>
  <n-input-group>
    <n-date-picker  v-model:value="paginationReactive.range" type="daterange"   style="width: 40%"/>
    <n-select v-model:value="paginationReactive.enableAlert" :options="enableAlertOptions" placeholder="预警状态" style="width: 15%" clearable />
    <n-input clearable placeholder="输入关键词搜索" v-model:value="paginationReactive.keyword"/>
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
  </n-input-group>
  <div style="display:flex; gap:8px; align-items:center; margin-top:8px;">
    <n-button size="small" type="primary" ghost :loading="backtestLoading" @click="runBacktest">
      执行回测(5日)
    </n-button>
    <n-button size="small" type="info" ghost @click="openBacktestStats">
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
            style="height: calc(100vh - 210px);margin-top: 10px"
        />

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

  <n-modal v-model:show="backtestStatsVisible" title="AI 推荐回测统计" preset="card" style="max-width: 1000px;">
    <template v-if="backtestStatsRef">
      <n-grid :cols="4" :x-gap="12" style="margin-bottom: 12px;">
        <n-grid-item><n-statistic label="已回测" :value="backtestStatsRef.total || 0" /></n-grid-item>
        <n-grid-item><n-statistic label="达标" :value="backtestStatsRef.win || 0" /></n-grid-item>
        <n-grid-item><n-statistic label="未达标" :value="backtestStatsRef.lose || 0" /></n-grid-item>
        <n-grid-item><n-statistic label="胜率" :value="backtestStatsRef.winRate ? backtestStatsRef.winRate.toFixed(1) : 0" suffix="%" /></n-grid-item>
      </n-grid>
      <n-divider title-placement="left"><n-gradient-text type="info">按评级胜率</n-gradient-text></n-divider>
      <n-table :bordered="false" :single-line="false" size="small" style="margin-bottom: 12px;">
        <thead>
          <tr><th>评级</th><th>总数</th><th>达标</th><th>胜率</th></tr>
        </thead>
        <tbody>
          <tr v-for="(st, rating) in backtestStatsRef.byRating || {}" :key="rating">
            <td>{{rating}}</td>
            <td>{{st.total || 0}}</td>
            <td>{{st.win || 0}}</td>
            <td>{{st.winRate ? st.winRate.toFixed(1) : 0}}%</td>
          </tr>
          <tr v-if="!(backtestStatsRef.byRating && Object.keys(backtestStatsRef.byRating).length)">
            <td colspan="4" style="text-align:center; color:#999;">暂无回测数据，请先点击「执行回测(5日)」</td>
          </tr>
        </tbody>
      </n-table>
      <n-divider title-placement="left"><n-gradient-text type="info">按模型统计</n-gradient-text></n-divider>
      <n-table :bordered="false" :single-line="false" size="small" style="margin-bottom: 12px;">
        <thead>
          <tr><th>模型</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th><th>平均超额</th></tr>
        </thead>
        <tbody>
          <tr v-for="m in backtestStatsRef.byModel || []" :key="m.name">
            <td>{{m.name}}</td>
            <td>{{m.total}}</td>
            <td>{{m.win}}</td>
            <td :style="{color: (m.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{m.winRate ? m.winRate.toFixed(1) : 0}}%</td>
            <td>{{m.avgReturn ? m.avgReturn.toFixed(2) : 0}}%</td>
            <td>{{m.avgExcess ? m.avgExcess.toFixed(2) : 0}}%</td>
          </tr>
          <tr v-if="!(backtestStatsRef.byModel && backtestStatsRef.byModel.length)">
            <td colspan="6" style="text-align:center; color:#999;">暂无数据</td>
          </tr>
        </tbody>
      </n-table>

      <n-divider title-placement="left"><n-gradient-text type="info">按提示词统计</n-gradient-text></n-divider>
      <n-tabs type="line" size="small" style="margin-bottom: 12px;">
        <n-tab-pane name="sys" tab="系统提示词">
          <n-table :bordered="false" :single-line="false" size="small">
            <thead>
              <tr><th>提示词</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th></tr>
            </thead>
            <tbody>
              <tr v-for="(p, i) in backtestStatsRef.bySystemPrompt || []" :key="i">
                <td style="max-width:300px;">
                  <n-tooltip trigger="hover">
                    <template #trigger>
                      <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestByPrompt('sys', p)">{{p.name}}</n-button>
                    </template>
                    点击按该提示词过滤下方「最近回测明细」
                  </n-tooltip>
                </td>
                <td>{{p.total}}</td>
                <td>{{p.win}}</td>
                <td :style="{color: (p.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{p.winRate ? p.winRate.toFixed(1) : 0}}%</td>
                <td>{{p.avgReturn ? p.avgReturn.toFixed(2) : 0}}%</td>
              </tr>
              <tr v-if="!(backtestStatsRef.bySystemPrompt && backtestStatsRef.bySystemPrompt.length)">
                <td colspan="5" style="text-align:center; color:#999;">暂无数据</td>
              </tr>
            </tbody>
          </n-table>
        </n-tab-pane>
        <n-tab-pane name="usr" tab="用户提示词">
          <n-table :bordered="false" :single-line="false" size="small">
            <thead>
              <tr><th>提示词</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th></tr>
            </thead>
            <tbody>
              <tr v-for="(p, i) in backtestStatsRef.byUserPrompt || []" :key="i">
                <td style="max-width:300px;">
                  <n-tooltip trigger="hover">
                    <template #trigger>
                      <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestByPrompt('usr', p)">{{p.name}}</n-button>
                    </template>
                    点击按该提示词过滤下方「最近回测明细」
                  </n-tooltip>
                </td>
                <td>{{p.total}}</td>
                <td>{{p.win}}</td>
                <td :style="{color: (p.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{p.winRate ? p.winRate.toFixed(1) : 0}}%</td>
                <td>{{p.avgReturn ? p.avgReturn.toFixed(2) : 0}}%</td>
              </tr>
              <tr v-if="!(backtestStatsRef.byUserPrompt && backtestStatsRef.byUserPrompt.length)">
                <td colspan="5" style="text-align:center; color:#999;">暂无数据</td>
              </tr>
            </tbody>
          </n-table>
        </n-tab-pane>
      </n-tabs>

      <n-divider title-placement="left"><n-gradient-text type="info">达标率最高</n-gradient-text></n-divider>
      <n-table :bordered="false" :single-line="false" size="small" style="margin-bottom: 12px;">
        <tbody>
          <tr v-if="backtestStatsRef.bestModel"><td style="width:120px;">最佳模型</td><td>{{backtestStatsRef.bestModel.name}}（达标率 {{backtestStatsRef.bestModel.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestModel.total}}）</td></tr>
          <tr v-if="backtestStatsRef.bestSystemPrompt"><td style="width:120px;">最佳系统提示词</td><td>{{backtestStatsRef.bestSystemPrompt.name}}（达标率 {{backtestStatsRef.bestSystemPrompt.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestSystemPrompt.total}}）</td></tr>
          <tr v-if="backtestStatsRef.bestUserPrompt"><td style="width:120px;">最佳用户提示词</td><td>{{backtestStatsRef.bestUserPrompt.name}}（达标率 {{backtestStatsRef.bestUserPrompt.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestUserPrompt.total}}）</td></tr>
          <tr v-if="!backtestStatsRef.bestModel && !backtestStatsRef.bestSystemPrompt && !backtestStatsRef.bestUserPrompt">
            <td colspan="2" style="text-align:center; color:#999;">暂无数据</td>
          </tr>
        </tbody>
      </n-table>

      <n-divider title-placement="left"><n-gradient-text type="info">最近回测明细</n-gradient-text></n-divider>
      <div v-if="backtestPromptFilter" style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
        <n-tag type="warning" closable @close="clearBacktestPromptFilter">{{backtestPromptFilterLabel}}</n-tag>
        <n-text depth="3">共 {{backtestTotalRef}} 条（点击提示词可过滤，关闭标签恢复全部）</n-text>
      </div>
      <n-data-table
        remote
        size="small"
        :columns="backtestListColumns"
        :data="backtestListRef"
        :loading="backtestListLoading"
        :pagination="{ page: backtestPageRef, pageSize: backtestPageSizeRef, itemCount: backtestTotalRef, onChange: (p) => loadBacktestList(p) }"
        style="max-height: 300px;"
      />
    </template>
    <n-empty v-else description="暂无回测数据，请先点击「执行回测(5日)」" />
  </n-modal>
</template>

<style scoped>

</style>