<script setup>
import { onMounted, ref, watch, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { GetStockKLine, GetConfig } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { init, dispose } from 'klinecharts'
import { NButton, NSpace, NText, NButtonGroup } from 'naive-ui'

const route = useRoute()
const router = useRouter()

const stockCode = ref(route.query.code || '')
const stockName = ref(route.query.name || '')
const chartRef = ref(null)
const klineChart = ref(null)

const darkTheme = ref(true)
const currentPeriod = ref('day')

// K线图预设选项
const periods = [
  { label: '日K', value: 'day' },
  { label: '周K', value: 'week' },
  { label: '月K', value: 'month' }
]

const initChart = () => {
  if (chartRef.value) {
    klineChart.value = init(chartRef.value)

    klineChart.value.setStyles(darkTheme.value ? 'dark' : 'light')

    // Optional: tweak chart styles
    klineChart.value.setStyles({
        grid: {
            horizontal: { show: true, color: darkTheme.value ? '#333' : '#ededed', style: 'dashed' },
            vertical: { show: true, color: darkTheme.value ? '#333' : '#ededed', style: 'dashed' }
        },
        candle: {
            bar: {
                upColor: '#ec0000',
                downColor: '#00da3c',
                noChangeColor: '#888888',
                upBorderColor: '#ec0000',
                downBorderColor: '#00da3c',
                noChangeBorderColor: '#888888',
                upWickColor: '#ec0000',
                downWickColor: '#00da3c',
                noChangeWickColor: '#888888'
            }
        }
    })

    // Default indicators
    klineChart.value.createIndicator('MA', false, { id: 'candle_pane' })
    klineChart.value.createIndicator('VOL', false, { height: 120 })
    klineChart.value.createIndicator('MACD', false, { height: 120 })

    fetchData()
  }
}

const fetchData = () => {
  if (!stockCode.value || !klineChart.value) return

  // App.go 中提供了 GetStockKLine（日K）等接口。
  // 注意，App.go 中的 GetStockKLine 默认是调用了 GetHK_KLineData，其内部第二个参数可能是 "day"。
  // 目前 wails 的接口可能不支持传入 period，为了适应后端现有的接口：
  // App.go中的 GetStockKLine 默认获取日线。若想获取其他周期需要后端扩展，暂时我们先只取日K并在此处根据 period 分割（或者如果后端支持我们会改变传入）。
  // 查阅 app.go 发现 GetStockKLine 只有 `days` 参数，所以在此我们只调用并渲染。
  klineChart.value.clearData()

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

const changePeriod = (periodValue) => {
    currentPeriod.value = periodValue
    // 目前没有不同周期的接口，只能暂存 period。如果要转换数据需要写前端聚合适配代码，或后端添加接口。
    // 在这里暂不改变数据，只是提供按钮留作扩展。
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
      <n-space justify="space-between" align="center" style="width: 100%;">
          <n-space align="center">
              <n-button @click="goBack" size="small" type="primary" secondary>返回</n-button>
              <n-text style="font-size: 20px; font-weight: bold; margin-left: 10px;">{{ stockName }} ({{ stockCode }})</n-text>
          </n-space>
          <n-space align="center">
              <!-- 周期的占位按钮，如果后端支持多周期可以在这里调用 -->
              <n-button-group size="small">
                  <n-button
                      v-for="period in periods"
                      :key="period.value"
                      :type="currentPeriod === period.value ? 'primary' : 'default'"
                      @click="changePeriod(period.value)"
                  >
                      {{ period.label }}
                  </n-button>
              </n-button-group>
          </n-space>
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