<script setup lang="ts">
// Bitget 美股永续合约衍生指标面板：资金费率（当期 + 下次结算倒计时 + 年化）、标记价/指数价/基差、
// 当前未平仓量（OI）、资金费率历史曲线。数据来自 /current-fund-rate、/funding-time、/open-interest、/history-fund-rate。
// 说明：Bitget 美股永续不提供多空持仓比，也无 OI 历史端点，故本面板不含多空比曲线与 OI 历史曲线。
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {GetBitgetFuturesDerivatives, GetBitgetFundingRateHistory} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";

const props = defineProps({
  symbol: {type: String, default: ''},
  name: {type: String, default: ''},
  darkTheme: {type: Boolean, default: false},
})

const bundle = ref<any>(null)
const fundingHistory = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')
const now = ref(Date.now())

const fundingRef = ref<HTMLElement | null>(null)
let fundingChart: echarts.ECharts | null = null
let clock: any = null

function num(v: any): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

const fundingPct = computed(() => (num(bundle.value?.fundingRate) * 100).toFixed(4) + '%')
const annualizedPct = computed(() => num(bundle.value?.annualizedRate).toFixed(2) + '%')
const basisPct = computed(() => num(bundle.value?.basis).toFixed(3) + '%')
const oiValueCn = computed(() => {
  const v = num(bundle.value?.openInterestUsd)
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
    const [res, hist] = await Promise.all([
      GetBitgetFuturesDerivatives(props.symbol),
      GetBitgetFundingRateHistory(props.symbol, 24),
    ])
    bundle.value = res || null
    fundingHistory.value = hist || []
    if (!res) {
      errorMsg.value = '衍生指标不可达：请在「设置 → 合约代理」中配置代理后重试'
    }
    await nextTick()
    renderChart()
  } catch (e: any) {
    console.error('fetchBitgetDerivatives error:', e)
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

function renderChart() {
  const points = fundingHistory.value || []
  const times = points.map((p: any) => fmtTime(num(p.fundingTime)))
  // 资金费率以百分数展示
  const rates = points.map((p: any) => num(p.fundingRate) * 100)

  if (fundingRef.value) {
    if (!fundingChart) fundingChart = echarts.init(fundingRef.value)
    fundingChart.setOption({
      ...baseGrid(),
      xAxis: {...baseGrid().xAxis, data: times},
      tooltip: {trigger: 'axis', valueFormatter: (v: any) => num(v).toFixed(4) + '%'},
      series: [{
        type: 'bar',
        name: '资金费率',
        data: rates,
        itemStyle: {color: '#8b5cf6'},
      }],
    }, true)
    fundingChart.resize()
  }
}

function onResize() {
  fundingChart?.resize()
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
  fundingChart?.dispose()
  fundingChart = null
})

watch(() => props.darkTheme, () => {
  nextTick(() => renderChart())
})

watch(() => props.symbol, () => {
  fetchData()
})
</script>

<template>
  <n-spin :show="loading" size="small">
    <n-flex align="center" :size="8" style="margin-bottom: 10px">
      <n-text depth="3" style="font-size: 12px">美股永续合约不提供多空持仓比与 OI 历史，仅展示当期衍生指标</n-text>
      <n-button size="small" @click="fetchData">刷新</n-button>
    </n-flex>

    <n-alert v-if="errorMsg" type="warning" :bordered="false" style="margin-bottom: 10px">
      {{ errorMsg }}
    </n-alert>

    <n-grid :cols="3" :x-gap="10" :y-gap="10" responsive="screen" style="margin-bottom: 12px">
      <n-grid-item>
        <n-card size="small" title="当期资金费率">
          <n-text :type="num(bundle?.fundingRate) >= 0 ? 'error' : 'success'" style="font-size: 20px">
            {{ fundingPct }}
          </n-text>
          <div>
            <n-text depth="3" style="font-size: 12px">年化 {{ annualizedPct }}</n-text>
          </div>
          <div>
            <n-text depth="3" style="font-size: 12px">结算周期 {{ num(bundle?.ratePeriod) }}h · 下次结算 {{ nextFundingCountdown }}</n-text>
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
            <n-text depth="3" style="font-size: 12px">数量 {{ num(bundle?.openInterest).toLocaleString() }}</n-text>
          </div>
        </n-card>
      </n-grid-item>
    </n-grid>

    <n-card size="small" title="资金费率历史（近 24 期）">
      <div ref="fundingRef" style="height: 240px"></div>
    </n-card>
  </n-spin>
</template>

<style scoped>

</style>