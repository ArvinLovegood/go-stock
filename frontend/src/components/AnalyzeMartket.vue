<script setup>

import {
  GetTodayMarketStatistic,
  GetMarketStatisticByDate,
  GetIndexTline,
  GetSectorAnchors,
  GetMarketEmotion,
  GlobalStockIndexes
} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";
import {onMounted, onUnmounted, ref, computed} from "vue";
const {darkTheme, chartHeight} = defineProps({
  chartHeight: {
    type: Number,
    default: 500
  },
  darkTheme: {
    type: Boolean,
    default: false
  }
})
const limitChartRef = ref(null);
const tlineChartRef = ref(null);
let handleChartInterval = null

// 日期选择（默认今天，时间戳格式）
const todayTs = Date.now()
const selectedDate = ref(todayTs)
const selectedDateStr = computed(() => {
  const d = new Date(selectedDate.value)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
})
const viewingToday = computed(() => {
  const d = new Date(selectedDate.value)
  const t = new Date()
  return d.getFullYear() === t.getFullYear() && d.getMonth() === t.getMonth() && d.getDate() === t.getDate()
})

// 禁用未来日期
function disableFutureDate(ts) {
  return ts > Date.now()
}

// 大盘成交额 + 市场情绪
const emotion = ref(null)          // 市场情绪数据

// 全球主要股指
const globalIndexes = ref([])

// A股四大指数（上证指数/深证成指/创业板指/科创50）
const aShareIndexNames = ['上证指数', '深证成指', '创业板指', '科创50']
const aShareIndexes = computed(() => {
  return aShareIndexNames
    .map(n => globalIndexes.value.find(i => i.name === n))
    .filter(Boolean)
})

onMounted(() => {
  handleChart()
  handleTlineChart()
  handleGlobalIndexes()
  handleEmotion()
  handleChartInterval = setInterval(function () {
    handleGlobalIndexes()
    handleEmotion()
    // 仅查看当日时自动刷新涨跌停和分时
    if (viewingToday.value) {
      handleChart()
      handleTlineChart()
    }
  }, 1000 * 60)
})

onUnmounted(() => {
  clearInterval(handleChartInterval)
})

// 日期变更时重新加载
function onDateChange(ts) {
  if (ts == null) {
    selectedDate.value = Date.now()
  }
  handleChart()
  handleTlineChart()
}

// 涨跌停图表
async function handleChart() {
  try {
    const data = viewingToday.value
      ? await GetTodayMarketStatistic()
      : await GetMarketStatisticByDate(selectedDateStr.value)
    if (data && data.length > 0) {
      renderLimitChart(data)
    }
  } catch (error) {
    console.error('获取市场统计数据失败:', error)
  }
}

// 指数分时图 + 板块异动标记
async function handleTlineChart() {
  try {
    const [tlineResult, anchors] = await Promise.all([
      GetIndexTline(selectedDateStr.value),
      GetSectorAnchors(selectedDateStr.value)
    ])
    console.log('[AnalyzeMartket] date:', selectedDateStr.value, 'tline:', tlineResult?.items?.length, 'items, anchors:', (anchors || []).length)
    if (tlineResult && tlineResult.items && tlineResult.items.length > 0) {
      renderTlineChart(tlineResult.items, anchors || [])
    }
  } catch (error) {
    console.error('获取指数分时数据失败:', error)
  }
}

// 市场情绪数据
async function handleEmotion() {
  try {
    emotion.value = await GetMarketEmotion()
  } catch (error) {
    console.error('获取市场情绪数据失败:', error)
  }
}

// 全球主要股指
async function handleGlobalIndexes() {
  try {
    const resp = await GlobalStockIndexes()
    if (!resp) return
    const all = [
      ...(resp.common || []),
      ...(resp.asia || []),
      ...(resp.america || [])
    ]
    globalIndexes.value = all.map(item => ({
      name: item.name || item.code || '',
      change: item.zdf || ''
    })).filter(i => i.name)
  } catch (error) {
    console.error('获取全球指数数据失败:', error)
  }
}

// minute 整数转 HH:MM
function minuteToTime(minute) {
  const h = Math.floor(minute / 100)
  const m = minute % 100
  return (h < 10 ? '0' + h : h) + ':' + (m < 10 ? '0' + m : m)
}

// "2026-07-22 09:34:52" -> 934
function parseMinuteFromTime(cTime) {
  const timePart = cTime.split(' ')[1] || ''
  const parts = timePart.split(':')
  if (parts.length >= 2) {
    return parseInt(parts[0]) * 100 + parseInt(parts[1])
  }
  return 0
}

// 格式化成交额
function formatBalance(val) {
  if (!val || val <= 0) return '--'
  if (val >= 1e12) return (val / 1e12).toFixed(2) + '万亿'
  if (val >= 1e8) return (val / 1e8).toFixed(2) + '亿'
  if (val >= 1e4) return (val / 1e4).toFixed(2) + '万'
  return val.toFixed(0)
}

// 解析涨跌幅字符串为数字
function parseChange(val) {
  if (!val) return 0
  const num = parseFloat(val)
  return isNaN(num) ? 0 : num
}

// 涨跌幅对应的 n-tag type
function getChangeType(val) {
  const num = parseChange(val)
  if (num > 0) return 'error'
  if (num < 0) return 'success'
  return 'default'
}

// 格式化涨跌幅显示
function formatChange(val) {
  const num = parseChange(val)
  if (num > 0) return '+' + num.toFixed(2) + '%'
  if (num < 0) return num.toFixed(2) + '%'
  return '0.00%'
}

function renderLimitChart(data) {
  if (!limitChartRef.value || !data || data.length === 0) return

  const chart = echarts.init(limitChartRef.value)

  const times = data.map(d => d.dataTime)
  const limitUps = data.map(d => d.limitUp)
  const limitDowns = data.map(d => d.limitDown)
  const ratios = data.map(d => d.limitRatio.toFixed(2))

  const option = {
    darkMode: darkTheme,
    title: {
      text: '涨跌停家数比',
      left: 'center',
      textStyle: {color: darkTheme ? '#ccc' : '#333', fontSize: 14}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      formatter: function (params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          result += param.marker + ' ' + param.seriesName + ': ' + param.value + '<br/>'
        })
        const idx = params[0].dataIndex
        if (idx < data.length) {
          const d = data[idx]
          result += `<span style="color:#666">涨跌停比: ${d.limitRatio.toFixed(2)}</span><br/>`
        }
        return result
      }
    },
    legend: {
      data: ['涨停家数', '跌停家数', '涨跌停比'],
      top: 25,
      textStyle: {color: darkTheme ? '#ccc' : '#333'}
    },
    grid: {left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true},
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: {color: darkTheme ? '#999' : '#666', rotate: 45},
      axisLine: {lineStyle: {color: darkTheme ? '#444' : '#ccc'}}
    },
    yAxis: [
      {
        type: 'value', name: '家数', position: 'left',
        axisLabel: {color: darkTheme ? '#999' : '#666'},
        axisLine: {lineStyle: {color: darkTheme ? '#444' : '#ccc'}},
        splitLine: {lineStyle: {color: darkTheme ? '#333' : '#eee'}}
      },
      {
        type: 'value', name: '涨跌停比', position: 'right',
        axisLabel: {color: darkTheme ? '#999' : '#666'},
        axisLine: {lineStyle: {color: darkTheme ? '#444' : '#ccc'}},
        splitLine: {show: false}
      }
    ],
    series: [
      {
        name: '涨停家数', type: 'bar', stack: 'total', data: limitUps,
        itemStyle: {color: '#ef4444'}
      },
      {
        name: '跌停家数', type: 'bar', stack: 'total', data: limitDowns,
        itemStyle: {color: '#22c55e'}
      },
      {
        name: '涨跌停比', type: 'line', yAxisIndex: 1, data: ratios, smooth: true,
        lineStyle: {color: '#f59e0b', width: 2},
        itemStyle: {color: '#f59e0b'},
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            {offset: 0, color: 'rgba(245, 158, 11, 0.3)'},
            {offset: 1, color: 'rgba(245, 158, 11, 0.05)'}
          ])
        },
        markLine: {
          silent: true,
          data: [{yAxis: 1, name: '平衡线', lineStyle: {color: '#888', type: 'dashed'}}]
        }
      }
    ]
  }

  chart.setOption(option)
}

function renderTlineChart(items, anchors) {
  if (!tlineChartRef.value || !items || items.length === 0) return

  const chart = echarts.init(tlineChartRef.value)

  const times = items.map(d => minuteToTime(d.minute))
  const prices = items.map(d => d.last_px)
  const balances = items.map(d => d.business_balance)

  // 按分钟分组板块异动
  const anchorMap = {}
  anchors.forEach(a => {
    const minute = parseMinuteFromTime(a.c_time)
    if (!anchorMap[minute]) anchorMap[minute] = []
    anchorMap[minute].push(a)
  })

  // 构建异动标记 scatter 数据（使用数值索引作为 x 值，匹配 category 轴）
  const scatterData = []
  Object.keys(anchorMap).forEach(minuteStr => {
    const minute = parseInt(minuteStr)
    const idx = items.findIndex(d => d.minute === minute)
    if (idx >= 0) {
      const group = anchorMap[minute]
      const hasUp = group.some(a => a.float === 'up')
      const hasDown = group.some(a => a.float === 'down')
      const color = hasUp && hasDown ? '#f59e0b' : (hasUp ? '#ef4444' : '#22c55e')
      scatterData.push({
        value: [idx, prices[idx]],
        itemStyle: {
          color: color,
          borderColor: '#fff',
          borderWidth: 1.5,
          shadowBlur: 4,
          shadowColor: color
        },
        sectors: group.map(g => ({
          name: g.symbol_name,
          direction: g.float,
          time: g.c_time
        }))
      })
    } else {
      console.warn('[AnalyzeMartket] anchor minute', minute, 'not found in tline')
    }
  })
  console.log('[AnalyzeMartket] scatterData:', scatterData.length, 'points')

  const textColor = darkTheme ? '#ccc' : '#333'
  const subTextColor = darkTheme ? '#999' : '#666'
  const lineColor = darkTheme ? '#444' : '#ccc'

  const option = {
    darkMode: darkTheme,
    title: {
      text: '指数分时 · 板块异动标记',
      left: 'center',
      textStyle: {color: textColor, fontSize: 14}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      formatter: function (params) {
        let result = params[0].axisValue + '<br/>'
        params.forEach(param => {
          if (param.seriesName === '指数点位') {
            const val = Array.isArray(param.value) ? param.value[1] : param.value
            result += param.marker + ' ' + param.seriesName + ': ' + (val != null ? val.toFixed(2) : '--') + '<br/>'
          }
          if (param.seriesName === '成交额') {
            const val = Array.isArray(param.value) ? param.value[1] : param.value
            result += param.marker + ' ' + param.seriesName + ': ' + formatBalance(val) + '<br/>'
          }
        })
        // 查找该时间点的板块异动
        const scatterParam = params.find(p => p.seriesName === '板块异动')
        if (scatterParam && scatterParam.data && scatterParam.data.sectors) {
          result += '<b style="color:#f59e0b">板块异动:</b><br/>'
          scatterParam.data.sectors.forEach(s => {
            const color = s.direction === 'up' ? '#ef4444' : '#22c55e'
            const arrow = s.direction === 'up' ? '↑' : '↓'
            result += `<span style="color:${color}">${arrow} ${s.name}</span> <span style="color:#888">${s.time.split(' ')[1]}</span><br/>`
          })
        }
        return result
      }
    },
    legend: {
      data: ['指数点位', '板块异动'],
      top: 25,
      textStyle: {color: textColor}
    },
    grid: {left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true},
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLabel: {color: subTextColor},
      axisLine: {lineStyle: {color: lineColor}}
    },
    yAxis: [
      {
        type: 'value', name: '点位', position: 'left', scale: true,
        axisLabel: {color: subTextColor},
        axisLine: {lineStyle: {color: lineColor}},
        splitLine: {lineStyle: {color: darkTheme ? '#333' : '#eee'}}
      },
      {
        type: 'value', name: '成交额', position: 'right',
        axisLabel: {
          color: subTextColor,
          formatter: function (val) {
            if (val >= 1e8) return (val / 1e8).toFixed(0) + '亿'
            if (val >= 1e4) return (val / 1e4).toFixed(0) + '万'
            return val
          }
        },
        axisLine: {lineStyle: {color: lineColor}},
        splitLine: {show: false}
      }
    ],
    series: [
      {
        name: '指数点位',
        type: 'line',
        data: prices,
        smooth: false,
        symbol: 'none',
        lineStyle: {color: '#3b82f6', width: 1.5},
        itemStyle: {color: '#3b82f6'},
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            {offset: 0, color: 'rgba(59, 130, 246, 0.2)'},
            {offset: 1, color: 'rgba(59, 130, 246, 0.01)'}
          ])
        }
      },
      {
        name: '成交额',
        type: 'bar',
        yAxisIndex: 1,
        data: balances,
        itemStyle: {
          color: darkTheme ? 'rgba(120,120,140,0.3)' : 'rgba(180,180,200,0.4)'
        },
        barWidth: '60%'
      },
      {
        name: '板块异动',
        type: 'scatter',
        data: scatterData,
        symbol: 'circle',
        symbolSize: 12,
        z: 10,
        label: {
          show: true,
          position: 'top',
          distance: 8,
          formatter: function (param) {
            if (param.data && param.data.sectors) {
              return param.data.sectors.map(s => s.name).join('/')
            }
            return ''
          },
          fontSize: 11,
          fontWeight: 'bold',
          color: textColor,
          backgroundColor: darkTheme ? 'rgba(50,50,55,0.9)' : 'rgba(255,255,255,0.9)',
          padding: [2, 5],
          borderRadius: 3,
          borderColor: darkTheme ? '#666' : '#ddd',
          borderWidth: 0.5
        },
        emphasis: {
          focus: 'series',
          itemStyle: {borderWidth: 2},
          label: {
            show: true,
            fontSize: 12
          }
        }
      }
    ]
  }

  chart.setOption(option)
}
</script>

<template>
  <div style="--wails-draggable:no-drag">
    <!-- 全球主要股指跑马灯 -->
    <n-marquee v-if="globalIndexes.length > 0" :speed="40" auto-fill style="margin-bottom: 8px;padding: 4px 0">
      <span v-for="idx in globalIndexes" :key="idx.name" style="display:inline-flex;align-items:center;margin-right:16px;white-space:nowrap">
        <n-tag size="small"  :bordered="false" :type="getChangeType(idx.change)">
          {{ idx.name }}
          <span style="font-weight:bold">{{ formatChange(idx.change) }}</span>
        </n-tag>
      </span>
    </n-marquee>

    <!-- 市场情绪 + 成交额 + 日期选择 + A股四大指数 -->
    <n-flex justify="space-between" align="center" :wrap="false" style="margin-bottom: 8px;padding: 0 4px">
      <n-flex :wrap="false" align="center" style="gap:12px">
        <template v-if="emotion">
          <n-text depth="2" style="font-size:13px;white-space:nowrap">
            热度 <n-text strong :type="parseInt(emotion.market_degree) >= 50 ? 'error' : 'success'" style="font-size:15px">{{ emotion.market_degree }}°</n-text>
          </n-text>
          <n-divider vertical/>
          <n-text depth="2" style="font-size:13px;white-space:nowrap">
            总成交额 <n-text strong style="font-size:15px">{{ emotion.shsz_balance }}</n-text>
            <n-text v-if="emotion.shsz_balance_change_px" :type="emotion.shsz_balance_change_px.startsWith('+') ? 'error' : 'success'" style="margin-left:6px;font-size:12px">
              {{ emotion.shsz_balance_change_px.startsWith('+') ? '放量' : '缩量' }} {{ emotion.shsz_balance_change_px }}
            </n-text>
          </n-text>
          <n-divider vertical/>
          <n-text depth="2" style="font-size:13px;white-space:nowrap">
            上涨 <n-text strong type="error">{{ emotion.up_down_dis?.rise_num || 0 }}</n-text>
            / 下跌 <n-text strong type="success">{{ emotion.up_down_dis?.fall_num || 0 }}</n-text>
          </n-text>
          <n-divider vertical/>
          <n-text depth="2" style="font-size:13px;white-space:nowrap">
            涨停 <n-text strong type="error">{{ emotion.up_down_dis?.up_num || 0 }}</n-text>
            / 跌停 <n-text strong type="success">{{ emotion.up_down_dis?.down_num || 0 }}</n-text>
          </n-text>
        </template>
        <template v-if="aShareIndexes.length > 0">
          <n-divider vertical v-if="emotion"/>
          <n-text v-for="idx in aShareIndexes" :key="idx.name" depth="2" style="font-size:13px;white-space:nowrap">
            {{ idx.name }}
            <n-text strong :type="getChangeType(idx.change)" style="margin-left:3px">{{ formatChange(idx.change) }}</n-text>
          </n-text>
        </template>
      </n-flex>
      <n-date-picker
        v-model:value="selectedDate"
        type="date"
        size="small"
        clearable
        :is-date-disabled="disableFutureDate"
        @update:value="onDateChange"
        style="width:150px"
      />
    </n-flex>

    <!-- 指数分时图 + 涨跌停图 左右布局 -->
    <div style="display:flex;gap:8px;align-items:stretch;--wails-draggable:no-drag">
      <div ref="tlineChartRef" style="flex:1;min-width:0;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}"></div>
      <div ref="limitChartRef" style="flex:1;min-width:0;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}"></div>
    </div>
  </div>
</template>

<style scoped>

</style>
