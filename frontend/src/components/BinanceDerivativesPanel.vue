<script setup lang="ts">
// 币安 USDT-M 永续合约衍生指标面板：资金费率（当期 + 下次结算倒计时）、标记价/指数价/基差、
// 未平仓量（OI）曲线、多空比曲线。数据来自 /fapi/v1/premiumIndex、/futures/data/* 等端点。
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {GetBinanceFuturesDerivatives} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";

const props = defineProps({
  symbol: {type: String, default: ''},
  name: {type: String, default: ''},
  darkTheme: {type: Boolean, default: false},
})

const PERIOD_OPTIONS = [
  {label: '5分钟', value: '5m'},
  {label: '15分钟', value: '15m'},
  {label: '1小时', value: '1h'},
  {label: '4小时', value: '4h'},
  {label: '1天', value: '1d'},
]

const period = ref('1h')
const bundle = ref<any>(null)
const loading = ref(false)
const errorMsg = ref('')
const now = ref(Date.now())

const oiRef = ref<HTMLElement | null>(null)
const lsRef = ref<HTMLElement | null>(null)
let oiChart: echarts.ECharts | null = null
let lsChart: echarts.ECharts | null = null
let clock: any = null

function num(v: any): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

const fundingPct = computed(() => (num(bundle.value?.lastFundingRate) * 100).toFixed(4) + '%')
const annualizedPct = computed(() => num(bundle.value?.annualizedRate).toFixed(2) + '%')
const basisPct = computed(() => num(bundle.value?.basis).toFixed(3) + '%')
const oiValueCn = computed(() => {
  const v = num(bundle.value?.openInterestValue)
  if (v >= 1e8) return (v / 1e8).toFixed(2) + ' 亿 USDT'
  if (v >= 1e4) return (v / 1e4).toFixed(2) + ' 万 USDT'
  return v.toFixed(0) + ' USDT'
})

/** 下次结算倒计时 */
const nextFundingCountdown = computed(() => {
  const t = num(bundle.value?.nextFundingTime)
  if (!t) return '--'
  let diff = Math.floor((t - now.value) / 1000)
  if (diff <= 0) return '结算中'
  const h = Math.floor(diff / 3600)
  const m = Math.floor((diff % 3600) / 60)
  const s = diff % 60
  return `${h}h ${m}m ${s}s`
})

function fmtTime(ms: number): string {
  const d = new Date(ms)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

async function fetchData() {
  if (!props.symbol) return
  try {
    loading.value = true
    errorMsg.value = ''
    const res = await GetBinanceFuturesDerivatives(props.symbol, period.value, 96)
    bundle.value = res || null
    if (!res) {
      errorMsg.value = '衍生指标不可达：请在「设置 → 合约代理」中配置代理后重试'
    }
    await nextTick()
    renderCharts()
  } catch (e: any) {
    console.error('fetchBinanceDerivatives error:', e)
    errorMsg.value = '请求失败: ' + (e?.message || e || '未知错误')
  } finally {
    loading.value = false
  }
}

function baseGrid() {
  return {
    grid: {left: 60, right: 20, top: 24, bottom: 10},
    tooltip: {trigger: 'axis'},
    xAxis: {
      type: 'category',
      boundaryGap: false,
      axisLabel: {color: props.darkTheme ? '#aaa' : '#666', fontSize: 11},
      axisLine: {lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}},
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: {color: props.darkTheme ? '#aaa' : '#666', fontSize: 11},
      splitLine: {lineStyle: {color: props.darkTheme ? '#333' : '#eee', type: 'dashed'}},
    },
  }
}

function renderCharts() {
  const b = bundle.value
  if (!b) return
  const points = b.openInterestHistory || []
  const times = points.map((p: any) => fmtTime(num(p.timestamp)))
  const oiValues = points.map((p: any) => num(p.value))

  if (oiRef.value) {
    if (!oiChart) oiChart = echarts.init(oiRef.value)
    oiChart.setOption({
      ...baseGrid(),
      xAxis: {...baseGrid().xAxis, data: times},
      tooltip: {trigger: 'axis', valueFormatter: (v: any) => num(v).toLocaleString()},
      series: [{
        type: 'line',
        name: '未平仓量',
        data: oiValues,
        smooth: true,
        showSymbol: false,
        lineStyle: {color: '#3b82f6', width: 2},
        areaStyle: {color: 'rgba(59,130,246,0.15)'},
      }],
    }, true)
    oiChart.resize()
  }

  const lsPoints = b.longShortHistory || []
  const lsTimes = lsPoints.map((p: any) => fmtTime(num(p.timestamp)))
  const lsValues = lsPoints.map((p: any) => num(p.value))

  if (lsRef.value) {
    if (!lsChart) lsChart = echarts.init(lsRef.value)
    lsChart.setOption({
      ...baseGrid(),
      xAxis: {...baseGrid().xAxis, data: lsTimes},
      tooltip: {trigger: 'axis'},
      series: [{
        type: 'line',
        name: '多空比',
        data: lsValues,
        smooth: true,
        showSymbol: false,
        lineStyle: {color: '#f59e0b', width: 2},
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: {color: props.darkTheme ? '#666' : '#bbb', type: 'dashed'},
          data: [{yAxis: 1, label: {formatter: '1.0'}}],
        },
      }],
    }, true)
    lsChart.resize()
  }
}

function onResize() {
  oiChart?.resize()
  lsChart?.resize()
}

onMounted(() => {
  clock = setInterval(() => {
    now.value = Date.now()
  }, 1000)
  window.addEventListener('resize', onResize)
  fetchData()
})

onBeforeUnmount(() => {
  if (clock) clearInterval(clock)
  window.removeEventListener('resize', onResize)
  oiChart?.dispose()
  lsChart?.dispose()
  oiChart = null
  lsChart = null
})

watch(() => period.value, () => {
  fetchData()
})

watch(() => props.darkTheme, () => {
  nextTick(() => renderCharts())
})

watch(() => props.symbol, () => {
  fetchData()
})
</script>

<template>
  <n-spin :show="loading" size="small">
    <n-flex align="center" :size="8" style="margin-bottom: 10px">
      <n-text depth="3" style="font-size: 12px">统计周期</n-text>
      <n-select
        v-model:value="period"
        size="small"
        style="width: 120px"
        :options="PERIOD_OPTIONS"
      />
      <n-button size="small" @click="fetchData">刷新</n-button>
    </n-flex>

    <n-alert v-if="errorMsg" type="warning" :bordered="false" style="margin-bottom: 10px">
      {{ errorMsg }}
    </n-alert>

    <n-grid :cols="4" :x-gap="10" :y-gap="10" responsive="screen" style="margin-bottom: 12px">
      <n-grid-item>
        <n-card size="small" title="当期资金费率">
          <n-text :type="num(bundle?.lastFundingRate) >= 0 ? 'error' : 'success'" style="font-size: 20px">
            {{ fundingPct }}
          </n-text>
          <div>
            <n-text depth="3" style="font-size: 12px">年化 {{ annualizedPct }}</n-text>
          </div>
          <div>
            <n-text depth="3" style="font-size: 12px">下次结算 {{ nextFundingCountdown }}</n-text>
          </div>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="标记价 / 指数价">
          <n-text style="font-size: 18px">{{ bundle?.markPrice ?? '--' }}</n-text>
          <div>
            <n-text depth="3" style="font-size: 12px">指数 {{ bundle?.indexPrice ?? '--' }}</n-text>
          </div>
          <div>
            <n-text depth="3" style="font-size: 12px">基差 {{ basisPct }}</n-text>
          </div>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="未平仓量（OI）">
          <n-text style="font-size: 18px">{{ oiValueCn }}</n-text>
          <div>
            <n-text depth="3" style="font-size: 12px">区间变化 {{ num(bundle?.openInterestChange).toFixed(2) }}%</n-text>
          </div>
        </n-card>
      </n-grid-item>
      <n-grid-item>
        <n-card size="small" title="多空比 / 主动买卖比">
          <n-text style="font-size: 18px">{{ num(bundle?.longShortRatio).toFixed(3) }}</n-text>
          <div>
            <n-text depth="3" style="font-size: 12px">多头账户 {{ (num(bundle?.longAccount) * 100).toFixed(1) }}%</n-text>
          </div>
          <div>
            <n-text depth="3" style="font-size: 12px">主动买卖比 {{ num(bundle?.takerBuySellRatio).toFixed(3) }}</n-text>
          </div>
        </n-card>
      </n-grid-item>
    </n-grid>

    <n-card size="small" title="未平仓量走势" style="margin-bottom: 12px">
      <div ref="oiRef" style="height: 240px"></div>
    </n-card>
    <n-card size="small" title="多空比走势">
      <div ref="lsRef" style="height: 240px"></div>
    </n-card>
  </n-spin>
</template>

<style scoped>

</style>