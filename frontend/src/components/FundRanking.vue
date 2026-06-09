<script setup>
import {h, ref, computed, reactive, onMounted, watch} from "vue";
import {NButton, NText, NFlex, NTag, NDataTable} from "naive-ui";
import {
  FollowFund,
  GetConfig,
  GetFollowedFund,
  GetFundRanking,
  GetFundTop10Holdings,
  OpenURL,
  SearchFundCodes
} from "../../wailsjs/go/main/App";
import {Environment} from "../../wailsjs/runtime";
import {useMessage} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import StockSparkLine from "./stockSparkLine.vue";

const message = useMessage()

const darkTheme = ref(false)
const marketType = ref('kf')
const rankingFundType = ref('all')
const rankingSortField = ref('jnzf')
const rankingSortOrder = ref('desc')
const rankingLoading = ref(false)
const rankingData = ref([])
const followList = ref([])
const searchKeyword = ref('')
const searchCodes = ref(null)
let searchTimer = null
let skipSortWatch = false

const holdingsModalShow = ref(false)
const holdingsFundCode = ref('')
const holdingsFundName = ref('')
const holdingsData = ref([])
const holdingsLoading = ref(false)

const klineModalShow = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
let klineAutoCloseTimer = null

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 50,
  itemCount: 0,
  prefix({itemCount}) {
    return `${itemCount} 只基金`
  }
})

const filteredData = computed(() => {
  if (searchCodes.value === null) return rankingData.value
  if (searchCodes.value.length === 0) return []
  return rankingData.value.filter(item => searchCodes.value.includes(item.code))
})

function onSearchKeywordChange(kw) {
  if (searchTimer) clearTimeout(searchTimer)
  const trimmed = kw.trim()
  if (!trimmed) {
    searchCodes.value = null
    return
  }
  searchTimer = setTimeout(() => {
    SearchFundCodes(trimmed).then(result => {
      if (result && result.length > 0) {
        searchCodes.value = result.map(item => item.code)
      } else {
        searchCodes.value = []
      }
    }).catch(() => {
      searchCodes.value = []
    })
  }, 500)
}

const marketTypeOptions = [
  {label: '场外基金', value: 'kf'},
  {label: '场内基金', value: 'fb'},
]

const offExchangeFundTypeOptions = [
  {label: '全部', value: 'all'},
  {label: '股票型', value: 'gp'},
  {label: '混合型', value: 'hh'},
  {label: '债券型', value: 'zq'},
  {label: '指数型', value: 'zs'},
  {label: 'QDII', value: 'qdii'},
  {label: 'FOF', value: 'fof'},
]

const onExchangeFundTypeOptions = [
  {label: '全部', value: 'ct'},
  {label: 'ETF', value: 'etf'},
  {label: 'LOF', value: 'lof'},
]

const fundTypeOptions = computed(() => {
  return marketType.value === 'fb' ? onExchangeFundTypeOptions : offExchangeFundTypeOptions
})

const sortFieldOptions = [
  {label: '今年来涨幅', value: 'jnzf'},
  {label: '日涨幅', value: 'rzdf'},
  {label: '近1周涨幅', value: '1yzf'},
  {label: '近1月涨幅', value: '1mzf'},
  {label: '近3月涨幅', value: '3mzf'},
  {label: '近6月涨幅', value: '6mzf'},
  {label: '近1年涨幅', value: '1nzf'},
  {label: '近2年涨幅', value: '2nzf'},
  {label: '近3年涨幅', value: '3nzf'},
  {label: '成立来涨幅', value: 'clzf'},
  {label: '规模', value: 'gm'},
]

watch(marketType, () => {
  rankingFundType.value = marketType.value === 'fb' ? 'ct' : 'all'
  paginationReactive.page = 1
  searchKeyword.value = ''
  searchCodes.value = null
  fetchFundRanking()
})

watch(rankingFundType, () => {
  paginationReactive.page = 1
  fetchFundRanking()
})

watch(rankingSortField, () => {
  if (skipSortWatch) {
    skipSortWatch = false
    return
  }
  rankingSortOrder.value = 'desc'
  paginationReactive.page = 1
  fetchFundRanking()
})

const keyToSortField = {
  dailyGrowth: 'rzdf',
  weekGrowth: '1yzf',
  monthGrowth: '1mzf',
  threeMonthGrowth: '3mzf',
  sixMonthGrowth: '6mzf',
  yearGrowth: '1nzf',
  threeYearGrowth: '3nzf',
  ytdGrowth: 'jnzf',
  sinceInception: 'clzf',
  scale: 'gm',
  netUnitValue: 'dwjz',
  netAccumulated: 'ljjz',
}

const sortFieldToKey = {}
for (const [k, v] of Object.entries(keyToSortField)) {
  sortFieldToKey[v] = k
}

function getSortOrder(key) {
  if (sortFieldToKey[rankingSortField.value] === key) {
    return rankingSortOrder.value === 'asc' ? 'ascend' : 'descend'
  }
  return false
}

function renderGrowth(val) {
  if (val == null) return '-'
  const color = val > 0 ? '#ef5350' : val < 0 ? '#26a69a' : undefined
  return h(NText, {style: {color}}, () => (val > 0 ? '+' : '') + val.toFixed(2) + '%')
}

const rankingColumns = computed(() => {
  const cols = [
    {title: '代码', key: 'code', width: 90, fixed: 'left'},
    {title: '名称', key: 'name', width: 160, ellipsis: {tooltip: true}},
  ]
  if (marketType.value === 'fb') {
    cols.push({title: '类型', key: 'fundTypeDetail', width: 90, render: (row) => row.fundTypeDetail ? h(NTag, {size: 'tiny', bordered: false, type: 'info'}, () => row.fundTypeDetail) : '-'})
  }
  cols.push(
    {title: '净值日期', key: 'netValueDate', width: 95},
    {title: '单位净值', key: 'netUnitValue', width: 85, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('netUnitValue'), render: (row) => row.netUnitValue?.toFixed(4) ?? '-'},
    {title: '累计净值', key: 'netAccumulated', width: 85, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('netAccumulated'), render: (row) => row.netAccumulated?.toFixed(4) ?? '-'},
    {title: '日涨幅', key: 'dailyGrowth', width: 78, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('dailyGrowth'), render: (row) => renderGrowth(row.dailyGrowth)},
    {title: '近1周', key: 'weekGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('weekGrowth'), render: (row) => renderGrowth(row.weekGrowth)},
    {title: '近1月', key: 'monthGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('monthGrowth'), render: (row) => renderGrowth(row.monthGrowth)},
    {title: '近3月', key: 'threeMonthGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('threeMonthGrowth'), render: (row) => renderGrowth(row.threeMonthGrowth)},
    {title: '近6月', key: 'sixMonthGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('sixMonthGrowth'), render: (row) => renderGrowth(row.sixMonthGrowth)},
    {title: '近1年', key: 'yearGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('yearGrowth'), render: (row) => renderGrowth(row.yearGrowth)},
    {title: '近3年', key: 'threeYearGrowth', width: 72, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('threeYearGrowth'), render: (row) => renderGrowth(row.threeYearGrowth)},
    {title: '今年来', key: 'ytdGrowth', width: 78, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('ytdGrowth'), render: (row) => renderGrowth(row.ytdGrowth)},
    {title: '成立来', key: 'sinceInception', width: 78, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('sinceInception'), render: (row) => renderGrowth(row.sinceInception)},
    {title: '规模(亿)', key: 'scale', width: 78, className: 'nowrap-cell', sorter: true, sortOrder: getSortOrder('scale'), render: (row) => row.scale?.toFixed(2) ?? '-'},
    {title: '成立日期', key: 'establishDate', width: 95},
    {
      title: '操作', key: 'actions', width: 170, fixed: 'right',
      render: (row) => {
        const isFollowed = followList.value.some(f => f.code === row.code)
        return h(NFlex, {size: 4, wrap: false}, () => [
          h(NButton, {
            size: 'tiny',
            type: isFollowed ? 'default' : 'primary',
            disabled: isFollowed,
            onClick: () => rankingFollowFund(row.code)
          }, () => isFollowed ? '已关注' : '关注'),
          h(NButton, {size: 'tiny', type: 'info', onClick: () => showHoldings(row.code, row.name)}, () => '持仓'),
          h(NButton, {size: 'tiny', type: 'warning', onClick: () => search(row.code)}, () => '详情'),
        ])
      }
    },
  )
  return cols
})

function fetchFundRanking() {
  rankingLoading.value = true
  GetFundRanking(marketType.value, rankingFundType.value, rankingSortField.value, rankingSortOrder.value, paginationReactive.page, paginationReactive.pageSize).then(result => {
    if (result) {
      rankingData.value = result.items || []
      paginationReactive.pageCount = result.totalPages || 0
      paginationReactive.itemCount = result.totalCount || 0
    }
  }).catch(() => {
    rankingData.value = []
  }).finally(() => {
    rankingLoading.value = false
  })
}

function resetRankingFilter() {
  rankingFundType.value = marketType.value === 'fb' ? 'ct' : 'all'
  rankingSortField.value = 'jnzf'
  rankingSortOrder.value = 'desc'
  paginationReactive.page = 1
  searchKeyword.value = ''
  searchCodes.value = null
  fetchFundRanking()
}

function handleSorterChange(sorter) {
  if (!sorter || sorter.order === false) {
    rankingSortField.value = 'jnzf'
    rankingSortOrder.value = 'desc'
  } else {
    const field = keyToSortField[sorter.columnKey]
    if (field) {
      skipSortWatch = true
      rankingSortField.value = field
      rankingSortOrder.value = sorter.order === 'ascend' ? 'asc' : 'desc'
    }
  }
  paginationReactive.page = 1
  fetchFundRanking()
}

function handlePageChange(currentPage) {
  if (!rankingLoading.value) {
    paginationReactive.page = currentPage
    fetchFundRanking()
  }
}

function rankingFollowFund(code) {
  FollowFund(code).then(result => {
    if (result) {
      message.success('关注成功')
      loadFollowList()
    }
  })
}

function loadFollowList() {
  GetFollowedFund().then(result => {
    followList.value = result
  })
}

function search(code) {
  setTimeout(() => {
    Environment().then(env => {
      switch (env.platform) {
        case 'windows':
          window.open("https://fund.eastmoney.com/" + code + ".html", "_blank", "noreferrer,width=1000,top=100,left=100,status=no,toolbar=no,location=no,scrollbars=no")
          break
        default:
          OpenURL("https://fund.eastmoney.com/" + code + ".html")
      }
    })
  }, 300)
}

function showHoldings(code, name) {
  holdingsFundCode.value = code
  holdingsFundName.value = name
  holdingsModalShow.value = true
  holdingsLoading.value = true
  holdingsData.value = []
  GetFundTop10Holdings(code).then(result => {
    holdingsData.value = result || []
  }).catch(() => {
    holdingsData.value = []
  }).finally(() => {
    holdingsLoading.value = false
  })
}

const holdingsColumns = [
  {title: '排名', key: 'rank', width: 50},
  {title: '代码', key: 'stockCode', width: 75},
  {title: '名称', key: 'stockName', width: 90, ellipsis: {tooltip: true}},
  {title: '占比(%)', key: 'ratio', width: 70, render: (row) => row.ratio?.toFixed(2) ?? '-'},
  {
    title: '涨跌幅', key: 'changeRate', width: 75,
    render: (row) => {
      const v = row.changeRate
      if (v == null) return '-'
      const color = v > 0 ? '#ef5350' : v < 0 ? '#26a69a' : undefined
      return h(NText, {style: {color}}, () => (v > 0 ? '+' : '') + v.toFixed(2) + '%')
    }
  },
  {
    title: '分时', key: 'sparkline', width: 120,
    render: (row) => {
      if (row.market !== 'A') return '-'
      const lastPrice = row.price || 0
      const openPrice = (row.changeRate != null && row.price != null && row.changeRate !== 0)
        ? row.price / (1 + row.changeRate / 100) : lastPrice
      const prefix = /^(6|5)/.test(row.stockCode) ? 'sh' : 'sz'
      return h(StockSparkLine, {
        stockCode: prefix + row.stockCode,
        stockName: row.stockName,
        lastPrice: lastPrice,
        openPrice: openPrice,
        darkTheme: darkTheme.value,
        idSuffix: '_fh_' + row.stockCode,
      })
    }
  },
  {
    title: '操作', key: 'actions', width: 70,
    render: (row) => h(NButton, {
      size: 'tiny', type: 'info',
      onClick: () => showStockKline(row.stockCode, row.stockName, row.market)
    }, () => 'K线')
  },
]

loadFollowList()
fetchFundRanking()
GetConfig().then(result => {
  if (result.darkTheme) darkTheme.value = true
})

function showStockKline(stockCode, stockName, market) {
  let em = ''
  if (market === 'A') {
    em = /^(6|5)/.test(stockCode) ? stockCode + '.SH' : stockCode + '.SZ'
  } else if (market === 'HK') {
    em = stockCode + '.HK'
  } else {
    em = stockCode + '.US'
  }
  klineStockCode.value = em
  klineStockName.value = stockName
  klineModalShow.value = true
  if (klineAutoCloseTimer) clearTimeout(klineAutoCloseTimer)
}
</script>

<template>
    <n-flex :wrap="false" align="center" :size="12" style="flex-shrink: 0; margin-bottom: 8px;">
      <n-select v-model:value="marketType" :options="marketTypeOptions" style="width: 120px;" size="small"/>
      <n-input v-model:value="searchKeyword" placeholder="基金名称/代码" clearable size="small" style="width: 180px;" @update:value="onSearchKeywordChange"/>
      <n-select v-model:value="rankingFundType" :options="fundTypeOptions" style="width: 120px;" size="small"/>
      <n-select v-model:value="rankingSortField" :options="sortFieldOptions" style="width: 140px;" size="small"/>
      <n-button type="primary" size="small" @click="fetchFundRanking" :loading="rankingLoading">查询</n-button>
      <n-button size="small" @click="resetRankingFilter">重置</n-button>
      <n-text depth="3" v-if="searchCodes !== null" style="font-size: 12px;">搜索到 {{ searchCodes.length }} 只，当前页匹配 {{ filteredData.length }} 只</n-text>
    </n-flex>
    <n-data-table
      remote
      :columns="rankingColumns"
      :data="filteredData"
      :loading="rankingLoading"
      :bordered="false"
      size="small"
      striped
      :pagination="paginationReactive"
      @update:page="handlePageChange"
      @update:sorter="handleSorterChange"
      :scroll-x="1700"
      flex-height
      style="height: calc(100vh - 210px);margin-top: 10px"
    />

  <n-modal
    v-model:show="holdingsModalShow"
    :title="holdingsFundName + ' - ' + holdingsFundCode + ' 十大持仓'"
    preset="card"
    style="max-width: 1400px;"
    :mask-closable="true"
  >
    <n-text v-if="holdingsData.length > 0 && holdingsData[0]?.quarter" depth="3" style="font-size: 12px; margin-bottom: 4px; display: inline-block;">
      {{ holdingsData[0].quarter }}
    </n-text>
    <n-data-table
      :columns="holdingsColumns"
      :data="holdingsData"
      :loading="holdingsLoading"
      :pagination="false"
      size="small"
      :bordered="false"
      :max-height="500"
      striped
    />
  </n-modal>

  <n-modal
    v-model:show="klineModalShow"
    :title="klineStockName + ' - ' + klineStockCode + ' K线图'"
    preset="card"
    style="max-width: 1400px;"
    :mask-closable="true"
  >
    <StockLightweightKlineChart
      v-if="klineModalShow && klineStockCode"
      :key="klineStockCode"
      :code="klineStockCode"
      :stock-name="klineStockName"
      :dark-theme="darkTheme"
      :chart-height="460"
    />
  </n-modal>
</template>

<style scoped>
:deep(.nowrap-cell) {
  white-space: nowrap;
}
</style>
