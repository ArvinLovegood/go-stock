<script setup lang="ts">
import {ref, watch, computed, nextTick, onBeforeUnmount} from 'vue'
import {ConceptDetail, ConceptKLine, ConceptRealHead, ConceptStocks, FindConceptCodeByName, GetAllConceptPlates, GetConfig} from "../../wailsjs/go/main/App"
import {useMessage} from "naive-ui"
import * as echarts from 'echarts'

interface ConceptMarket {
  open: string
  preClose: string
  low: string
  high: string
  volume: string
  changePercent: string
  changeRank: string
  upDownCount: string
  netInflow: string
  dealAmount: string
}

interface ConceptStock {
  code: string
  name: string
  price: string
  changePercent: string
  change: string
  speed: string
  turnover: string
  volumeRatio: string
  amplitude: string
  dealAmount: string
  flowShares: string
  flowMarketCap: string
  peRatio: string
}

interface ConceptDetailInfo {
  conceptCode: string
  plateCode: string
  name: string
  definition: string
  market: ConceptMarket
  stocks: ConceptStock[]
}

interface ConceptKLineItem {
  date: string
  open: number
  close: number
  low: number
  high: number
  volume: number
}

interface ConceptKLineData {
  name: string
  total: number
  start: string
  factor: number
  issuePrice: number
  kLines: ConceptKLineItem[]
}

const props = defineProps({
  show: {type: Boolean, default: false},
  conceptCode: {type: String, default: ''},
  conceptName: {type: String, default: ''},
  plateCode: {type: String, default: ''}
})

const emit = defineEmits(['update:show'])

const message = useMessage()
const loading = ref(false)
const klineLoading = ref(false)
const detail = ref<ConceptDetailInfo | null>(null)
const klineData = ref<ConceptKLineData | null>(null)
const marketData = ref<ConceptMarket | null>(null)
const darkTheme = ref(false)
const klineRange = ref<number>(120) // 默认展示最近120根K线
const chartRef = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

// 成分股分页数据
const stocks = ref<any[]>([])
const stocksLoading = ref(false)
const stocksPage = ref(1)
const stocksHasMore = ref(false)

// 通过名称匹配到的概念代码（30xxxx），用于详情页/成分股；与板块代码(88xxxx)分开
const resolvedConceptCode = ref('')

const showDialog = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v)
})

// 优先使用 realhead 实时行情，回退到详情页抓取的 market
const displayMarket = computed(() => marketData.value || detail.value?.market || null)

// 实际用于详情页/成分股的概念代码：优先 resolvedConceptCode，其次非板块代码的 conceptCode
const isPlateCode = (code: string) => /^88\d{4}$/.test(code || '')
const effectiveConceptCode = computed(() => {
  if (resolvedConceptCode.value) return resolvedConceptCode.value
  if (props.conceptCode && !isPlateCode(props.conceptCode)) return props.conceptCode
  return ''
})

// 成分股表格列
const stockColumns = [
  {title: '代码', key: 'code', width: 80, fixed: 'left'},
  {title: '名称', key: 'name', width: 90, fixed: 'left'},
  {title: '现价', key: 'price', width: 70, align: 'right' as const},
  {
    title: '涨跌幅(%)', key: 'changePercent', width: 90, align: 'right' as const,
    sorter: (a: ConceptStock, b: ConceptStock) => parseFloat(a.changePercent) - parseFloat(b.changePercent)
  },
  {title: '涨跌', key: 'change', width: 70, align: 'right' as const},
  {title: '涨速(%)', key: 'speed', width: 80, align: 'right' as const},
  {title: '换手(%)', key: 'turnover', width: 80, align: 'right' as const},
  {title: '量比', key: 'volumeRatio', width: 70, align: 'right' as const},
  {title: '振幅(%)', key: 'amplitude', width: 80, align: 'right' as const},
  {title: '成交额', key: 'dealAmount', width: 100, align: 'right' as const},
  {title: '流通股', key: 'flowShares', width: 100, align: 'right' as const},
  {title: '流通市值', key: 'flowMarketCap', width: 110, align: 'right' as const},
  {title: '市盈率', key: 'peRatio', width: 80, align: 'right' as const}
]

function rowClassName(row: ConceptStock) {
  const pct = parseFloat(row.changePercent)
  if (!isNaN(pct)) {
    if (pct > 0) return 'row-up'
    if (pct < 0) return 'row-down'
  }
  return ''
}

function formatYi(v: string): string {
  const n = parseFloat(v)
  if (isNaN(n)) return v
  if (Math.abs(n) >= 1e8) return (n / 1e8).toFixed(2) + '亿'
  if (Math.abs(n) >= 1e4) return (n / 1e4).toFixed(2) + '万'
  return n.toFixed(2)
}

// 涨跌幅颜色
function pctColor(p: string): string {
  const n = parseFloat(p)
  if (isNaN(n)) return ''
  if (n > 0) return darkTheme.value ? '#ff6b6b' : '#c23531'
  if (n < 0) return darkTheme.value ? '#5dd39e' : '#009933'
  return ''
}

async function loadConfig() {
  try {
    const cfg = await GetConfig()
    darkTheme.value = !!cfg?.darkTheme
  } catch {
  }
}

async function fetchDetail() {
  // 板块代码（88xxxx）：用于 K线/realhead
  const pc = props.plateCode || (isPlateCode(props.conceptCode) ? props.conceptCode : '')
  // 概念代码（30xxxx）：用于详情页/成分股。若 conceptCode 看起来是板块代码，则通过名称匹配真正的概念代码
  let cc = props.conceptCode && !isPlateCode(props.conceptCode) ? props.conceptCode : ''
  if (!cc && props.conceptName) {
    try {
      const resolved = await FindConceptCodeByName(props.conceptName)
      if (resolved) {
        cc = resolved
        resolvedConceptCode.value = resolved
      }
    } catch (e) {
      // 匹配失败，忽略
    }
  }
  if (!pc && !cc) return
  loading.value = true
  // 并行拉取：K线/实时行情用板块代码 pc，详情/成分股用概念代码 cc
  const tasks: Promise<any>[] = []
  if (pc) {
    tasks.push(fetchKLine(pc))
    tasks.push(fetchRealHead(pc))
  }
  if (cc) tasks.push(fetchConceptDetail(cc))
  // 成分股分页：优先用概念代码，其次用板块代码
  tasks.push(fetchStocks(cc || pc, 1))
  await Promise.all(tasks)
  loading.value = false
}

async function fetchConceptDetail(conceptCode: string) {
  try {
    const res = await ConceptDetail(conceptCode)
    detail.value = res || null
  } catch (e) {
    message.error('获取概念详情失败: ' + e)
  }
}

async function fetchStocks(code: string, page: number) {
  if (!code) return
  if (page === 1) {
    stocksLoading.value = true
  }
  try {
    const res = await ConceptStocks(code, page)
    const list = res || []
    if (page === 1) {
      stocks.value = list
      stocksPage.value = 1
    } else {
      stocks.value = stocks.value.concat(list)
      stocksPage.value = page
    }
    // 每页通常 20 条，不足 20 条说明是最后一页
    stocksHasMore.value = list.length >= 20
  } catch (e) {
    message.error('获取成分股失败: ' + e)
  } finally {
    if (page === 1) {
      stocksLoading.value = false
    }
  }
}

async function fetchRealHead(plateCode: string) {
  try {
    const res = await ConceptRealHead(plateCode)
    marketData.value = res || null
  } catch (e) {
    message.error('获取实时行情失败: ' + e)
  }
}

async function fetchKLine(plateCode: string) {
  if (!plateCode) return
  klineLoading.value = true
  try {
    const res = await ConceptKLine(plateCode)
    klineData.value = res || null
    await nextTick()
    renderKLine()
  } catch (e) {
    message.error('获取K线数据失败: ' + e)
  } finally {
    klineLoading.value = false
  }
}

// YYYYMMDD -> YYYY-MM-DD
function formatDate(d: string): string {
  if (!d || d.length !== 8) return d
  return d.substring(0, 4) + '-' + d.substring(4, 6) + '-' + d.substring(6, 8)
}

function renderKLine() {
  if (!chartRef.value || !klineData.value || !klineData.value.kLines?.length) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
  }

  const all = klineData.value.kLines
  const startIdx = Math.max(0, all.length - klineRange.value)
  const data = all.slice(startIdx)

  const dates = data.map(k => formatDate(k.date))
  // 同花顺 K 线价格需除以 factor 还原为真实点位
  const factor = klineData.value.factor && klineData.value.factor > 0 ? klineData.value.factor : 1
  const ohlc = data.map(k => [k.open / factor, k.close / factor, k.low / factor, k.high / factor])
  const volumes = data.map(k => k.volume)

  const upColor = '#c23531'
  const downColor = '#009933'
  const textColor = darkTheme.value ? '#aaa' : '#666'
  const bgColor = darkTheme.value ? '#1a1a2e' : '#fff'
  const gridColor = darkTheme.value ? '#2a2a3e' : '#e6e6e6'

  const option: echarts.EChartsOption = {
    backgroundColor: bgColor,
    animation: false,
    title: {
      text: `${klineData.value.name || props.conceptName} K线`,
      left: 10,
      textStyle: {color: darkTheme.value ? '#ccc' : '#456', fontSize: 14}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      backgroundColor: darkTheme.value ? 'rgba(30,30,60,0.9)' : 'rgba(255,255,255,0.95)',
      borderColor: darkTheme.value ? '#456' : '#ddd',
      textStyle: {color: darkTheme.value ? '#ccc' : '#333', fontSize: 12},
      valueFormatter: (v: any) => v == null ? '-' : Number(v).toFixed(2)
    },
    axisPointer: {link: [{xAxisIndex: 'all'}]},
    grid: [
      {left: 60, right: 30, top: 40, height: '60%'},
      {left: 60, right: 30, top: '72%', height: '18%'}
    ],
    xAxis: [
      {
        type: 'category', data: dates, scale: true,
        boundaryGap: true,
        axisLine: {lineStyle: {color: gridColor}},
        axisLabel: {color: textColor, fontSize: 10},
        splitLine: {show: false}
      },
      {
        type: 'category', gridIndex: 1, data: dates, scale: true,
        boundaryGap: true,
        axisLabel: {show: false},
        axisLine: {lineStyle: {color: gridColor}}
      }
    ],
    yAxis: [
      {
        type: 'value', scale: true, splitArea: {show: true},
        axisLabel: {color: textColor, fontSize: 10},
        axisLine: {lineStyle: {color: gridColor}},
        splitLine: {lineStyle: {color: gridColor}}
      },
      {
        type: 'value', gridIndex: 1, splitNumber: 2,
        axisLabel: {color: textColor, fontSize: 10},
        axisLine: {lineStyle: {color: gridColor}},
        splitLine: {lineStyle: {color: gridColor}}
      }
    ],
    dataZoom: [
      {type: 'inside', xAxisIndex: [0, 1], start: 0, end: 100},
      {show: true, type: 'slider', xAxisIndex: [0, 1], bottom: 8, height: 18, start: 0, end: 100}
    ],
    series: [
      {
        name: 'K线',
        type: 'candlestick',
        data: ohlc,
        itemStyle: {
          color: upColor,
          color0: downColor,
          borderColor: upColor,
          borderColor0: downColor
        },
        markPoint: {
          symbol: 'pin', symbolSize: 40,
          label: {fontSize: 9},
          data: [
            {type: 'max', name: '最高', valueIndex: 3},
            {type: 'min', name: '最低', valueIndex: 2}
          ]
        }
      },
      {
        name: '成交量',
        type: 'bar',
        xAxisIndex: 1,
        yAxisIndex: 1,
        data: volumes.map((v, i) => ({
          value: v,
          itemStyle: {color: data[i].close >= data[i].open ? upColor : downColor}
        }))
      }
    ]
  }
  chart.setOption(option, true)
}

function onRangeChange() {
  renderKLine()
}

function resizeChart() {
  chart?.resize()
}

watch(() => props.show, (v) => {
  if (v && (props.conceptCode || props.plateCode || props.conceptName)) {
    loadConfig()
    fetchDetail()
  } else {
    detail.value = null
    klineData.value = null
    marketData.value = null
    stocks.value = []
    stocksPage.value = 1
    stocksHasMore.value = false
    resolvedConceptCode.value = ''
    if (chart) {
      chart.dispose()
      chart = null
    }
  }
})

watch(() => [props.conceptCode, props.plateCode, props.conceptName], () => {
  if (props.show && (props.conceptCode || props.plateCode || props.conceptName)) {
    fetchDetail()
  }
})

onBeforeUnmount(() => {
  if (chart) {
    chart.dispose()
    chart = null
  }
})
</script>

<template>
  <n-modal v-model:show="showDialog"
           :title="(detail?.name || conceptName || '概念') + ' — 详情'"
           preset="card"
           :bordered="false"
           style="width: min(1280px, 96vw); max-width: 96vw"
           :content-style="{
             maxHeight: '88vh',
             overflowY: 'auto',
             overflowX: 'hidden',
             padding: '12px'
           }">
    <n-spin :show="loading">
      <!-- 概念简介 + 行情数据 -->
      <n-card v-if="marketData || (detail && (detail.name || detail.market.changePercent))" size="small" :bordered="true" class="detail-card">
        <n-descriptions :column="4" size="small" label-placement="left" bordered>
          <n-descriptions-item label="概念代码">{{ resolvedConceptCode || detail?.conceptCode || '-' }}</n-descriptions-item>
          <n-descriptions-item label="板块代码">{{ detail?.plateCode || plateCode || '-' }}</n-descriptions-item>
          <n-descriptions-item label="板块涨幅">
            <span :style="{color: pctColor(displayMarket?.changePercent || ''), fontWeight: 600}">
              {{ displayMarket?.changePercent || '-' }}%
            </span>
          </n-descriptions-item>
          <n-descriptions-item label="涨幅排名">{{ displayMarket?.changeRank || '-' }}</n-descriptions-item>
          <n-descriptions-item label="今开">{{ displayMarket?.open || '-' }}</n-descriptions-item>
          <n-descriptions-item label="昨收">{{ displayMarket?.preClose || '-' }}</n-descriptions-item>
          <n-descriptions-item label="最高">{{ displayMarket?.high || '-' }}</n-descriptions-item>
          <n-descriptions-item label="最低">{{ displayMarket?.low || '-' }}</n-descriptions-item>
          <n-descriptions-item label="成交量">{{ displayMarket?.volume || '-' }}</n-descriptions-item>
          <n-descriptions-item label="成交额">{{ formatYi(displayMarket?.dealAmount || '') }}</n-descriptions-item>
          <n-descriptions-item label="资金净流入">
            <span :style="{color: pctColor(displayMarket?.netInflow || ''), fontWeight: 600}">
              {{ formatYi(displayMarket?.netInflow || '') }}
            </span>
          </n-descriptions-item>
          <n-descriptions-item label="涨跌家数">{{ displayMarket?.upDownCount || '-' }}</n-descriptions-item>
        </n-descriptions>
        <n-divider v-if="detail?.definition" style="margin: 8px 0"/>
        <n-text v-if="detail?.definition" depth="2" style="font-size: 13px; line-height: 1.6; white-space: pre-wrap;">
          {{ detail.definition }}
        </n-text>
      </n-card>

      <!-- K线图（只要有 plateCode 就展示，不依赖 detail） -->
      <n-card v-if="plateCode || conceptCode" size="small" :bordered="true" class="detail-card" style="margin-top: 10px">
        <template #header>
          <n-flex align="center" :wrap="false">
            <n-text strong>板块K线</n-text>
            <n-text depth="3" style="font-size: 12px" v-if="klineData">
              共 {{ klineData.total }} 根，基准价 {{ klineData.issuePrice.toFixed(2) }}
            </n-text>
            <n-text depth="3" style="font-size: 12px" v-else-if="!klineLoading && plateCode">
              板块代码：{{ plateCode }}
            </n-text>
          </n-flex>
        </template>
        <template #header-extra>
          <n-flex align="center" :wrap="false" size="small">
            <n-text depth="3" style="font-size: 12px">显示</n-text>
            <n-select v-model:value="klineRange" size="small" style="width: 100px"
                      :options="[
                        {label: '最近30', value: 30},
                        {label: '最近60', value: 60},
                        {label: '最近120', value: 120},
                        {label: '最近250', value: 250},
                        {label: '全部', value: 9999}
                      ]"
                      @update:value="onRangeChange"/>
          </n-flex>
        </template>
        <n-spin :show="klineLoading">
          <div ref="chartRef" style="width: 100%; height: 420px;"></div>
          <n-empty v-if="!klineLoading && !klineData?.kLines?.length"
                   description="暂无K线数据" style="padding: 40px 0;"/>
        </n-spin>
      </n-card>

      <!-- 成分股列表（优先用 stocks 分页数据，回退 detail.stocks） -->
      <n-card v-if="stocks.length > 0 || (detail && detail.stocks && detail.stocks.length)" size="small" :bordered="true" class="detail-card" style="margin-top: 10px">
        <template #header>
          <n-flex align="center" :wrap="false">
            <n-text strong>成分股</n-text>
            <n-tag size="small" round :bordered="false" type="info">
              {{ stocks.length || detail.stocks?.length || 0 }} 只
            </n-tag>
          </n-flex>
        </template>
        <n-spin :show="stocksLoading">
          <n-data-table
            :columns="stockColumns"
            :data="stocks.length > 0 ? stocks : (detail.stocks || [])"
            :max-height="420"
            :scroll-x="1200"
            size="small"
            :row-class-name="rowClassName"
            :pagination="{pageSize: 20, showSizePicker: false}"
            striped/>
          <div v-if="stocksHasMore" style="text-align: center; padding: 10px 0;">
            <n-button :loading="stocksLoading" size="small" type="primary" ghost @click="fetchStocks(effectiveConceptCode || plateCode, stocksPage + 1)">
              加载更多
            </n-button>
          </div>
        </n-spin>
      </n-card>

      <n-empty v-if="!loading && !detail && !klineData?.kLines?.length"
               description="暂无数据（题材无板块代码，无法获取K线）" style="padding: 40px 0;"/>
    </n-spin>
  </n-modal>
</template>

<style scoped>
.detail-card :deep(.n-card__content) {
  padding: 8px;
}

:deep(.row-up) {
  color: #c23531;
}

:deep(.row-down) {
  color: #009933;
}

@media (prefers-color-scheme: dark) {
  :deep(.row-up) {
    color: #ff6b6b;
  }

  :deep(.row-down) {
    color: #5dd39e;
  }
}
</style>
