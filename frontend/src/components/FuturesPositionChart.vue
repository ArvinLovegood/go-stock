<script setup>
import {onBeforeUnmount, onMounted, ref} from 'vue'
import * as echarts from 'echarts'
import {GetFuturesPositionTrend, GetStockKLine} from '../../wailsjs/go/main/App'

const props = defineProps({
  variety: {
    type: String,
    default: 'IF'
  },
  days: {
    type: Number,
    default: 90
  },
  chartHeight: {
    type: Number,
    default: 560
  },
  darkTheme: {
    type: Boolean,
    default: true
  }
})

const varietyOptions = [
  {label: 'IF 沪深300', value: 'IF'},
  {label: 'IH 上证50', value: 'IH'},
  {label: 'IC 中证500', value: 'IC'},
  {label: 'IM 中证1000', value: 'IM'}
]
const currentVariety = ref(props.variety)
const loading = ref(false)
const summary = ref(null)
const metaInfo = ref(null)
const chartRef = ref(null)
let chartInstance = null
let resizeHandler = null

const indexNames = {
  'sh000300': '沪深300',
  'sh000016': '上证50',
  'sh000905': '中证500',
  'sh000852': '中证1000'
}

onMounted(() => {
  chartInstance = echarts.init(chartRef.value)
  chartInstance.showLoading()
  loadData()
  resizeHandler = () => chartInstance && chartInstance.resize()
  window.addEventListener('resize', resizeHandler)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeHandler)
  if (chartInstance) {
    chartInstance.dispose()
    chartInstance = null
  }
})

function changeVariety(v) {
  currentVariety.value = v
  loadData()
}

function normalizeDate(s) {
  if (!s) return ''
  return String(s).slice(0, 10)
}

async function loadData() {
  loading.value = true
  summary.value = null
  metaInfo.value = null
  if (chartInstance) chartInstance.showLoading()
  try {
    const resp = await GetFuturesPositionTrend(currentVariety.value, '', props.days)
    if (!resp || !resp.rows || resp.rows.length === 0) {
      if (chartInstance) {
        chartInstance.hideLoading()
        chartInstance.clear()
      }
      loading.value = false
      return
    }
    metaInfo.value = resp
    const rows = resp.rows
    summary.value = rows[rows.length - 1]

    // 拉取对应现货指数 K 线，用于对照
    let kMap = {}
    try {
      const klines = await GetStockKLine(resp.indexCode, indexNames[resp.indexCode] || resp.indexCode, props.days)
      if (klines && klines.length > 0) {
        for (const k of klines) {
          kMap[normalizeDate(k.day)] = k
        }
      }
    } catch (e) {
      console.warn('期指多空：现货指数K线获取失败，使用持仓接口自带指数收盘价', e)
    }

    render(resp, kMap)
  } catch (e) {
    console.error('期指多空数据加载失败', e)
    if (chartInstance) chartInstance.hideLoading()
  }
  loading.value = false
}

function render(resp, kMap) {
  const rows = resp.rows
  const dates = rows.map(r => r.tradeDate)
  // 现货指数 K 线：优先真实K线，缺失日用持仓接口指数收盘价补平（保证两图 x 轴对齐）
  const kValues = rows.map(r => {
    const k = kMap[r.tradeDate]
    if (k) {
      return [Number(k.open), Number(k.close), Number(k.low), Number(k.high)]
    }
    const c = Number(r.indexClose)
    return c > 0 ? [c, c, c, c] : null
  })
  const longs = rows.map(r => r.longPosition)
  const shorts = rows.map(r => r.shortPosition)
  const nets = rows.map(r => r.netPosition)
  const indexName = indexNames[resp.indexCode] || '现货指数'

  const textColor = props.darkTheme ? '#c2c2c2' : '#333'
  const axisLineColor = props.darkTheme ? '#444' : '#ccc'
  const upColor = '#ec0000'
  const downColor = '#00da3c'

  chartInstance.hideLoading()
  chartInstance.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      backgroundColor: props.darkTheme ? '#2a2a2a' : '#fff',
      borderColor: axisLineColor,
      textStyle: {color: textColor},
      formatter(params) {
        if (!params || !params.length) return ''
        const idx = params[0].dataIndex
        const r = rows[idx]
        const k = kValues[idx]
        let html = `<b>${r.tradeDate}</b>（${resp.contractCode}）<br/>`
        if (k && k[1] > 0) {
          html += `${indexName}：开 ${k[0].toFixed(2)} / 收 <b style="color:${k[1] >= k[0] ? upColor : downColor}">${k[1].toFixed(2)}</b><br/>`
        }
        html += `多单：<b style="color:${upColor}">${r.longPosition}</b>（${r.longChange >= 0 ? '+' : ''}${r.longChange}）<br/>`
        html += `空单：<b style="color:${downColor}">${r.shortPosition}</b>（${r.shortChange >= 0 ? '+' : ''}${r.shortChange}）<br/>`
        html += `净持仓：<b style="color:${r.netPosition >= 0 ? upColor : downColor}">${r.netPosition >= 0 ? '+' : ''}${r.netPosition}</b> 手<br/>`
        if (r.basis || r.settlePrice) {
          html += `结算 ${r.settlePrice.toFixed(2)} / 基差 ${r.basis.toFixed(2)}`
        }
        return html
      }
    },
    axisPointer: {link: [{xAxisIndex: 'all'}]},
    grid: [
      {left: 70, right: 70, top: 30, height: '48%'},
      {left: 70, right: 70, top: '68%', height: '26%'}
    ],
    xAxis: [
      {
        type: 'category', data: dates, gridIndex: 0, boundaryGap: true,
        axisLine: {lineStyle: {color: axisLineColor}},
        axisLabel: {show: false}
      },
      {
        type: 'category', data: dates, gridIndex: 1, boundaryGap: true,
        axisLine: {lineStyle: {color: axisLineColor}},
        axisLabel: {color: textColor}
      }
    ],
    yAxis: [
      {
        scale: true, gridIndex: 0,
        axisLabel: {color: textColor}, splitLine: {lineStyle: {color: axisLineColor, opacity: 0.3}}
      },
      {
        gridIndex: 1, name: '多空(手)', nameTextStyle: {color: textColor},
        axisLabel: {color: textColor}, splitLine: {lineStyle: {color: axisLineColor, opacity: 0.3}}
      },
      {
        gridIndex: 1, position: 'right', name: '净持仓(手)', nameTextStyle: {color: textColor},
        axisLabel: {color: textColor}, splitLine: {show: false}
      }
    ],
    dataZoom: [
      {type: 'inside', xAxisIndex: [0, 1], start: 60, end: 100},
      {type: 'slider', xAxisIndex: [0, 1], bottom: 2, height: 18, borderColor: axisLineColor, textStyle: {color: textColor}}
    ],
    series: [
      {
        name: indexName, type: 'candlestick', data: kValues, xAxisIndex: 0, yAxisIndex: 0,
        itemStyle: {color: upColor, color0: downColor, borderColor: upColor, borderColor0: downColor}
      },
      {
        name: '多单持仓', type: 'bar', data: longs, xAxisIndex: 1, yAxisIndex: 1,
        itemStyle: {color: 'rgba(236,0,0,0.55)'},
        barGap: '-100%' // 与空单柱重叠对比
      },
      {
        name: '空单持仓', type: 'bar', data: shorts, xAxisIndex: 1, yAxisIndex: 1,
        itemStyle: {color: 'rgba(0,218,60,0.55)'},
        z: 1
      },
      {
        name: '净持仓', type: 'line', data: nets, xAxisIndex: 1, yAxisIndex: 2,
        showSymbol: false, smooth: true,
        lineStyle: {width: 2, color: '#f5b70a'},
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            {offset: 0, color: 'rgba(245,183,10,0.35)'},
            {offset: 1, color: 'rgba(245,183,10,0.02)'}
          ])
        },
        markLine: {
          symbol: 'none', silent: true,
          lineStyle: {type: 'dashed', color: '#888'},
          data: [{yAxis: 0}]
        }
      }
    ]
  }, true)
}
</script>

<template>
  <div class="futures-position-chart">
    <div class="fp-header">
      <n-tabs type="segment" size="small" :value="currentVariety" @update-value="changeVariety"
              style="max-width: 480px;display:inline-block;--wails-draggable:no-drag">
        <n-tab v-for="opt in varietyOptions" :key="opt.value" :name="opt.value" :tab="opt.label"/>
      </n-tabs>
      <div v-if="metaInfo" class="fp-meta">
        <n-tag size="small" :bordered="false" type="info">{{ metaInfo.varietyName }}</n-tag>
        <n-tag size="small" :bordered="false">主力合约 {{ metaInfo.contractCode }}</n-tag>
        <n-tag size="small" :bordered="false" type="warning">
          数据源 {{ metaInfo.source === 'eastmoney' ? '东方财富' : '中金所' }}
        </n-tag>
      </div>
    </div>

    <div v-if="summary" class="fp-summary">
      <div class="fp-item">
        <div class="fp-label">最新交易日</div>
        <div class="fp-value">{{ summary.tradeDate }}</div>
      </div>
      <div class="fp-item">
        <div class="fp-label">多单持仓（增减）</div>
        <div class="fp-value up">{{ summary.longPosition?.toLocaleString() }}
          <span class="fp-sub">（{{ summary.longChange >= 0 ? '+' : '' }}{{ summary.longChange?.toLocaleString() }}）</span>
        </div>
      </div>
      <div class="fp-item">
        <div class="fp-label">空单持仓（增减）</div>
        <div class="fp-value down">{{ summary.shortPosition?.toLocaleString() }}
          <span class="fp-sub">（{{ summary.shortChange >= 0 ? '+' : '' }}{{ summary.shortChange?.toLocaleString() }}）</span>
        </div>
      </div>
      <div class="fp-item">
        <div class="fp-label">净持仓</div>
        <div class="fp-value" :class="summary.netPosition >= 0 ? 'up' : 'down'">
          {{ summary.netPosition >= 0 ? '+' : '' }}{{ summary.netPosition?.toLocaleString() }}
        </div>
      </div>
      <div class="fp-item" v-if="summary.indexClose > 0">
        <div class="fp-label">现货指数收盘</div>
        <div class="fp-value">{{ summary.indexClose.toFixed(2) }}</div>
      </div>
      <div class="fp-item" v-if="summary.basis">
        <div class="fp-label">基差</div>
        <div class="fp-value" :class="summary.basis >= 0 ? 'up' : 'down'">{{ summary.basis.toFixed(2) }}</div>
      </div>
    </div>

    <div ref="chartRef" :style="{height: chartHeight + 'px', width: '100%'}"></div>
    <div v-if="!loading && (!metaInfo || !metaInfo.rows || metaInfo.rows.length === 0)" class="fp-empty">
      未获取到期指持仓数据（持仓数据约每交易日 17:30 后更新）
    </div>
    <div class="fp-tip">
      上图：现货指数K线；下图：前20会员多单(红)/空单(绿)持仓与净持仓(黄线)。净空收窄/净多扩大通常偏多；增仓上涨趋势强化，减仓上涨多为空头回补——请与指数走势对照验证。
    </div>
  </div>
</template>

<style scoped>
.futures-position-chart {
  width: 100%;
  text-align: left;
}

.fp-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.fp-meta {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.fp-summary {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
  margin: 6px 0 10px 4px;
}

.fp-item {
  min-width: 90px;
}

.fp-label {
  font-size: 12px;
  opacity: 0.65;
}

.fp-value {
  font-size: 15px;
  font-weight: 600;
}

.fp-value.up {
  color: #d03050;
}

.fp-value.down {
  color: #18a058;
}

.fp-sub {
  font-size: 12px;
  font-weight: 400;
  opacity: 0.75;
}

.fp-empty {
  padding: 40px 0;
  text-align: center;
  opacity: 0.7;
}

.fp-tip {
  font-size: 12px;
  opacity: 0.55;
  margin-top: 4px;
  line-height: 1.6;
}
</style>
