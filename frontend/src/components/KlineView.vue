<script setup>
import { onMounted, ref, watch, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { GetStockKLine, GetConfig } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { init, dispose } from 'klinecharts'
import { NButton, NSpace, NText } from 'naive-ui'

const route = useRoute()
const router = useRouter()

const stockCode = ref(route.query.code || '')
const stockName = ref(route.query.name || '')
const chartRef = ref(null)
const klineChart = ref(null)

const darkTheme = ref(true)

const initChart = () => {
  if (chartRef.value) {
    klineChart.value = init(chartRef.value)

    klineChart.value.setStyles(darkTheme.value ? 'dark' : 'light')

    // Default indicators
    klineChart.value.createIndicator('MA', false, { id: 'candle_pane' })
    klineChart.value.createIndicator('VOL', false, { height: 100 })
    klineChart.value.createIndicator('MACD', false, { height: 100 })

    fetchData()
  }
}

const fetchData = () => {
  if (!stockCode.value || !klineChart.value) return

  GetStockKLine(stockCode.value, stockName.value, 365).then(result => {
    if (!result) return

    const klineData = result.map(item => {
      const timestamp = new Date(item.day).getTime()
      return {
        timestamp: timestamp,
        open: Number(item.open),
        high: Number(item.high),
        low: Number(item.low),
        close: Number(item.close),
        volume: Number(item.volume)
      }
    })

    klineChart.value.applyNewData(klineData)
  })
}

onMounted(() => {
  GetConfig().then(res => {
    darkTheme.value = res.darkTheme
    nextTick(() => {
        initChart()
    })
  })

  EventsOn("updateSettings", (res) => {
    darkTheme.value = res.darkTheme
    if (klineChart.value) {
      klineChart.value.setStyles(darkTheme.value ? 'dark' : 'light')
    }
  })
})

onBeforeUnmount(() => {
  if (klineChart.value) {
    dispose(chartRef.value)
  }
  EventsOff("updateSettings")
})

const goBack = () => {
  router.back()
}
</script>

<template>
  <div class="kline-container" style="--wails-draggable:no-drag">
    <div class="header">
      <n-space align="center">
          <n-button @click="goBack" size="small" type="primary" secondary>返回</n-button>
          <n-text style="font-size: 20px; font-weight: bold;">{{ stockName }} ({{ stockCode }})</n-text>
      </n-space>
    </div>
    <div ref="chartRef" class="chart"></div>
  </div>
</template>

<style scoped>
.kline-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 90vh;
  padding: 10px;
  box-sizing: border-box;
}

.header {
  display: flex;
  align-items: center;
  margin-bottom: 15px;
}

.chart {
  flex: 1;
  width: 100%;
  height: 100%;
  min-height: 500px;
}
</style>