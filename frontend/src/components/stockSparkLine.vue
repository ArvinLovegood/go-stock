<script setup>
import {onMounted, onBeforeUnmount, ref, watch} from "vue";
import * as echarts from 'echarts';
import {GetStockMinutePriceLineData} from "../../wailsjs/go/main/App"; // 如果您使用多个组件，请将此样式导入放在您的主文件中
const {idSuffix,stockCode,stockName,lastPrice,openPrice,darkTheme} = defineProps({
  idSuffix: {
    type: String,
    default: ""
  },
  stockCode: {
    type: String,
    default: ""
  },
  stockName: {
    type: String,
    default: ""
  },
  lastPrice: {
    type: Number,
    default: 0
  },
  openPrice: {
    type: Number,
    default: 0
  },
  darkTheme: {
    type: Boolean,
    default: true
  },
})

const chartRef=ref();
const chart = ref(null)
let loadSeq = 0
let resizeObserver = null
let intersectionObserver = null

async function setChartData() {
  if (!chart.value) return
  const seq = ++loadSeq
  try {
    const result = await GetStockMinutePriceLineData(stockCode, stockName)
    if (seq !== loadSeq || !chart.value) return
    const priceData = result.priceData
    let category = []
    let price = []
    let min = 0
    let max = 0
    for (let i = 0; i < priceData.length; i++) {
      category.push(priceData[i].time)
      price.push(priceData[i].price)
      if (min === 0 || min > priceData[i].price) {
        min = priceData[i].price
      }
      if (max < priceData[i].price) {
        max = priceData[i].price
      }
    }
    const option = {
      padding: [0, 0, 0, 0],
      grid: {
        top: 0,
        left: 0,
        right: 0,
        bottom: 0
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          label: {
            backgroundColor: '#6a7985'
          }
        }
      },
      xAxis: {
        show: false,
        type: 'category',
        data: category
      },
      yAxis: {
        show: false,
        type: 'value',
        min: (min).toFixed(2),
        max: (max).toFixed(2),
        minInterval: 0.01,
      },
      // visualMap: {
      //   show: false,
      //   type: 'piecewise',
      //   pieces: [
      //     {
      //       min: Number(min),
      //       max: Number(openPrice),
      //       color: 'green'
      //     },
      //     {
      //       min: Number(openPrice),
      //       max: Number(max),
      //       color: 'red'
      //     }
      //   ]
      // },
      series: [
        {
          data: price,
          type: 'line',
          smooth: false,
          stack: '总量',
          showSymbol: false,
          lineStyle: {
            color: lastPrice > openPrice ? 'rgba(245, 0, 0, 1)' : 'rgb(6,251,10)'
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
              offset: 0,
              color: lastPrice > openPrice ? 'rgba(245, 0, 0, 1)' : 'rgba(6,251,10, 1)'
            }, {
              offset: 1,
              color: lastPrice > openPrice ? 'rgba(245, 0, 0, 0.25)' : 'rgba(6,251,10, 0.25)'
            }])
          },
        }
      ]
    };
    chart.value.setOption(option, { notMerge: true })
  } catch (error) {
    console.error('加载分时走势图失败:', stockCode, error)
  }
}

function activate() {
  if (chart.value || !chartRef.value) return
  chart.value = echarts.init(chartRef.value)
  setChartData()
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => chart.value?.resize())
    resizeObserver.observe(chartRef.value)
  }
}

onMounted(() => {
  if (typeof IntersectionObserver === 'undefined') {
    activate()
    return
  }
  intersectionObserver = new IntersectionObserver((entries) => {
    if (!entries.some(entry => entry.isIntersecting)) return
    intersectionObserver?.disconnect()
    intersectionObserver = null
    activate()
  }, { rootMargin: '80px 0px' })
  intersectionObserver.observe(chartRef.value)
})

watch(
  () => [stockCode, stockName],
  () => {
    if (chart.value) setChartData()
  },
)

watch(
  () => [lastPrice, openPrice],
  () => {
    if (!chart.value) return
    const up = lastPrice > openPrice
    chart.value.setOption({
      series: [{
        lineStyle: { color: up ? 'rgba(245, 0, 0, 1)' : 'rgb(6,251,10)' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: up ? 'rgba(245, 0, 0, 1)' : 'rgba(6,251,10, 1)' },
            { offset: 1, color: up ? 'rgba(245, 0, 0, 0.25)' : 'rgba(6,251,10, 0.25)' },
          ]),
        },
      }],
    })
  },
)

onBeforeUnmount(() => {
  ++loadSeq
  intersectionObserver?.disconnect()
  resizeObserver?.disconnect()
  chart.value?.dispose()
  chart.value = null
})

</script>
<template>
<div ref="chartRef" style="height: 20px;width: 100%" :id="'sparkLine'+stockCode+idSuffix">
</div>
</template>