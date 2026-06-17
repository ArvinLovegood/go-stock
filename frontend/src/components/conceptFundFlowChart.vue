<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref, watch, computed} from "vue";
import {GetConceptFundFlowListByDate, GetConceptFundFlowTopListByDate, GetAllConceptCodes} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";

const props = defineProps({
  darkTheme: {
    type: Boolean,
    default: false
  },
  chartHeight: {
    type: Number,
    default: 600
  }
})

const chartRef = ref(null)
const topList = ref<any[]>([])
const loading = ref(false)
const refreshInterval = ref<any>(null)
let chart: echarts.ECharts | null = null

// 临时添加的概念
const extraSectors = ref<any[]>([])
const addCodeInput = ref('')
const allConceptCodes = ref<any[]>([])
const addCodeOptions = computed(() => {
  const existCodes = new Set([
    ...inflowList.value.map((i: any) => i.code),
    ...outflowList.value.map((i: any) => i.code),
    ...extraSectors.value.map((i: any) => i.code)
  ])
  return allConceptCodes.value
    .filter((i: any) => !existCodes.has(i.code))
    .map((i: any) => ({label: `${i.name} (${i.code})`, value: i.code, name: i.name}))
})

// 默认当天日期，格式 YYYY-MM-DD
const today = new Date()
const todayStr = today.getFullYear() + '-' +
  String(today.getMonth() + 1).padStart(2, '0') + '-' +
  String(today.getDate()).padStart(2, '0')
const selectedDate = ref(todayStr)

// 是否查看的是今天
const isToday = computed(() => selectedDate.value === todayStr)

// 流入前20
const inflowList = computed(() => topList.value.filter((item: any) => item.netInflow > 0).slice(0, 20))
// 流出前20
const outflowList = computed(() => topList.value.filter((item: any) => item.netInflow < 0).slice(-20).reverse())

// ===== 播放功能 =====
const isPlaying = ref(false)
const playTimer = ref<any>(null)
const playSpeed = ref(1) // 倍速
const playIndex = ref(0)  // 当前播放到第几个时间点
const totalPoints = ref(0) // 总时间点数
const cachedAllData = ref<any[]>([]) // 缓存全量数据
const cachedTimes = ref<string[]>([]) // 缓存全量时间轴

onMounted(async () => {
  try {
    // 加载所有概念代码（供临时添加使用）
    const codes = await GetAllConceptCodes()
    if (codes && Array.isArray(codes)) allConceptCodes.value = codes

    await loadAllData()
    // 交易时间每分钟刷新（仅当天）
    refreshInterval.value = setInterval(async () => {
      if (isToday.value && isTradingTime()) {
        await loadAllData()
      }
    }, 60000)
  } catch (e) {
    console.error('onMounted error:', e)
  }
})

onBeforeUnmount(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
  }
  stopPlay()
  if (chart) {
    chart.dispose()
  }
})

function isTradingTime(): boolean {
  const now = new Date()
  const h = now.getHours()
  const m = now.getMinutes()
  const t = h * 60 + m
  // 9:30 - 11:30 或 13:00 - 15:00
  return (t >= 570 && t <= 690) || (t >= 780 && t <= 900)
}

// 流入前20用红色系，流出前20用绿色系
function getSeriesColor(index: number, isInflow: boolean): string {
  const redShades = [
    '#ee6666', '#d14a61', '#fc8452', '#e8534e', '#c23531',
    '#f4755e', '#d4816a', '#e76f51', '#ef6c4a', '#d9534f',
    '#e74c3c', '#ff6b6b', '#ee5a24', '#f19066', '#e55039',
    '#eb4d4b', '#fc5c65', '#ff6348', '#e71d36', '#c0392b'
  ]
  const greenShades = [
    '#00da3c', '#3ba272', '#91cc75', '#2ecc71', '#27ae60',
    '#52be80', '#58d68d', '#45b39d', '#1abc9c', '#16a085',
    '#239b56', '#28b463', '#73c0de', '#48b8d0', '#87cefa',
    '#1dd1a1', '#10ac84', '#0abde3', '#01a3a4', '#00cec9'
  ]
  return isInflow ? redShades[index % redShades.length] : greenShades[index % greenShades.length]
}

// 日期禁用：不能选未来日期
function isDateDisabled(ts: number): boolean {
  return ts > Date.now()
}

async function loadAllData() {
  loading.value = true
  try {
    const date = selectedDate.value
    // 按日期获取概念排名
    const res = await GetConceptFundFlowTopListByDate(date, 500)
    if (!res || !Array.isArray(res) || res.length === 0) {
      topList.value = []
      return
    }
    topList.value = res

    // 流入前20 和 流出前20
    const inflowTop = res.filter((item: any) => item.netInflow > 0).slice(0, 20)
    const outflowTop = res.filter((item: any) => item.netInflow < 0).slice(-20).reverse()
    const sectors = [...inflowTop, ...outflowTop, ...extraSectors.value]

    const allData = await Promise.all(
      sectors.map(async (item: any) => {
        const points = await GetConceptFundFlowListByDate(item.code, date)
        return {
          code: item.code,
          name: item.name,
          isInflow: item.isInflow !== undefined ? item.isInflow : item.netInflow > 0,
          points: points || []
        }
      })
    )

    stopPlay()
    renderChart(allData, true)
  } catch (e) {
    console.error('loadAllData error:', e)
  } finally {
    loading.value = false
  }
}

function extractTime(snapTime: string): string {
  if (!snapTime || typeof snapTime !== 'string') return ''
  if (snapTime.length >= 16) return snapTime.substring(11, 16)
  return String(snapTime)
}

// 丢弃9:29之前的时间点
function isValidTime(t: string): boolean {
  if (!t || t.length < 5) return false
  const h = parseInt(t.substring(0, 2), 10)
  const m = parseInt(t.substring(3, 5), 10)
  const mins = h * 60 + m
  return mins >= 569 // 9:29
}

function renderChart(allData: { code: string; name: string; isInflow: boolean; points: any[] }[], fullRender: boolean = false) {
  if (!allData || allData.length === 0) return

  // 收集所有时间点，去重并排序，丢弃9:29之前的
  const timeSet = new Set<string>()
  for (const sector of allData) {
    for (const pt of sector.points) {
      const t = extractTime(pt.snapTime)
      if (t && isValidTime(t)) timeSet.add(t)
    }
  }
  const times = Array.from(timeSet).sort()

  // 缓存全量数据供播放使用
  if (fullRender) {
    cachedAllData.value = allData
    cachedTimes.value = times
    totalPoints.value = times.length
    playIndex.value = times.length
  }

  // 分别统计流入/流出概念的索引，用于配色
  let inflowIdx = 0
  let outflowIdx = 0

  // 构建每个概念的 series 数据，对齐到统一时间轴
  const seriesList: any[] = []
  for (let i = 0; i < allData.length; i++) {
    const sector = allData[i]
    const dataMap = new Map<string, number>()
    for (const pt of sector.points) {
      const t = extractTime(pt.snapTime)
      if (t && isValidTime(t)) dataMap.set(t, pt.netInflow || 0)
    }

    // 对齐到统一时间轴，缺失的时间点用 null
    const values = times.map(t => dataMap.has(t) ? dataMap.get(t)! : null)

    const color = getSeriesColor(sector.isInflow ? inflowIdx++ : outflowIdx++, sector.isInflow)

    seriesList.push({
      name: sector.name,
      type: 'line',
      data: values,
      smooth: true,
      showSymbol: false,
      lineStyle: {width: 2, color},
      itemStyle: {color},
      emphasis: {
        lineStyle: {width: 3},
        focus: 'series'
      },
      // 折线末尾显示名称
      endLabel: {
        show: true,
        formatter: '{a}',
        color,
        fontSize: 12,
        fontWeight: 'bold',
        distance: 8
      }
    })
  }

  if (!chart && chartRef.value) {
    chart = echarts.init(chartRef.value)
  }
  if (!chart) return

  const textColor = props.darkTheme ? '#aaa' : '#666'
  const bgColor = props.darkTheme ? '#1a1a2e' : '#fff'
  const dateLabel = selectedDate.value

  const option: echarts.EChartsOption = {
    backgroundColor: bgColor,
    title: {
      text: `${dateLabel} 概念资金流向 - 多概念对比`,
      left: '20px',
      textStyle: {color: props.darkTheme ? '#ccc' : '#456', fontSize: 16}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      borderWidth: 1,
      borderColor: props.darkTheme ? '#456' : '#ddd',
      backgroundColor: props.darkTheme ? 'rgba(30,30,60,0.9)' : 'rgba(255,255,255,0.95)',
      padding: 10,
      textStyle: {color: props.darkTheme ? '#ccc' : '#333', fontSize: 12},
      formatter: (params: any) => {
        if (!Array.isArray(params)) return ''
        const inflowItems = params.filter((p: any) => p.value != null && p.value > 0)
          .sort((a: any, b: any) => (b.value || 0) - (a.value || 0))
        const outflowItems = params.filter((p: any) => p.value != null && p.value < 0)
          .sort((a: any, b: any) => (a.value || 0) - (b.value || 0))

        const renderList = (items: any[]) => items.map((p: any) => {
          const val = (p.value / 100000000).toFixed(2)
          const sign = p.value > 0 ? '+' : ''
          return `${p.marker} ${p.seriesName} <b>${sign}${val}</b>`
        }).join('<br/>')

        let html = `<b>${params[0].axisValue}</b><br/>`
        html += '<div style="display:flex;gap:20px;">'
        html += `<div><div style="color:#ee6666;font-weight:bold;margin-bottom:4px;">流入</div>${renderList(inflowItems) || '-'}</div>`
        html += `<div><div style="color:#00da3c;font-weight:bold;margin-bottom:4px;">流出</div>${renderList(outflowItems) || '-'}</div>`
        html += '</div>'
        return html
      }
    },
    legend: {
      type: 'plain',
      left: 0,
      top: 30,
      orient: 'horizontal',
      align: 'left',
      itemWidth: 14,
      itemHeight: 10,
      itemGap: 8,
      textStyle: {color: textColor, fontSize: 11},
      icon: 'roundRect'
    },
    grid: {
      left: '8%',
      right: '12%',
      top: 120,
      height: '52%'
    },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: {onZero: false, lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}},
      splitLine: {show: false},
      axisLabel: {
        color: textColor,
        rotate: 30,
        fontSize: 11,
        interval: times.length <= 30 ? 0 : Math.floor(times.length / 12)
      },
      axisTick: {lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}}
    },
    yAxis: {
      name: '净流入/亿元',
      type: 'value',
      nameTextStyle: {color: textColor, fontSize: 11},
      axisLine: {show: true, lineStyle: {color: props.darkTheme ? '#444' : '#ccc'}},
      splitLine: {lineStyle: {color: props.darkTheme ? '#333' : '#eee', type: 'dashed'}},
      axisLabel: {
        color: textColor,
        fontSize: 11,
        formatter: (v: number) => (v / 100000000).toFixed(2)
      }
    },
    series: seriesList,
    dataZoom: [
      {
        type: 'inside',
        xAxisIndex: [0],
        start: 0,
        end: 100
      },
      {
        show: true,
        xAxisIndex: [0],
        type: 'slider',
        top: '88%',
        start: 0,
        end: 100,
        borderColor: props.darkTheme ? '#444' : '#ccc',
        fillerColor: props.darkTheme ? 'rgba(100,100,200,0.2)' : 'rgba(100,100,200,0.15)',
        handleStyle: {color: props.darkTheme ? '#666' : '#999'},
        textStyle: {color: textColor}
      }
    ]
  }

  chart.setOption(option, true)
  chart.resize()
}

// ===== 播放控制 =====
function startPlay() {
  if (cachedAllData.value.length === 0 || cachedTimes.value.length === 0) return
  playIndex.value = 0
  isPlaying.value = true
  tickPlay()
}

function tickPlay() {
  if (!isPlaying.value) return
  if (playIndex.value >= totalPoints.value) {
    isPlaying.value = false
    return
  }
  playIndex.value++
  renderPlayFrame()
  const interval = Math.max(50, 300 / playSpeed.value)
  playTimer.value = setTimeout(tickPlay, interval)
}

function renderPlayFrame() {
  const allData = cachedAllData.value
  const times = cachedTimes.value.slice(0, playIndex.value)

  if (times.length === 0) return

  let inflowIdx = 0
  let outflowIdx = 0

  const seriesList: any[] = []
  for (let i = 0; i < allData.length; i++) {
    const sector = allData[i]
    const dataMap = new Map<string, number>()
    for (const pt of sector.points) {
      const t = extractTime(pt.snapTime)
      if (t && isValidTime(t)) dataMap.set(t, pt.netInflow || 0)
    }

    const values = times.map(t => dataMap.has(t) ? dataMap.get(t)! : null)
    const color = getSeriesColor(sector.isInflow ? inflowIdx++ : outflowIdx++, sector.isInflow)

    seriesList.push({
      name: sector.name,
      type: 'line',
      data: values,
      smooth: true,
      showSymbol: false,
      lineStyle: {width: 2, color},
      itemStyle: {color},
      emphasis: {
        lineStyle: {width: 3},
        focus: 'series'
      },
      endLabel: {
        show: true,
        formatter: '{a}',
        color,
        fontSize: 12,
        fontWeight: 'bold',
        distance: 8
      }
    })
  }

  if (!chart) return

  const textColor = props.darkTheme ? '#aaa' : '#666'
  const bgColor = props.darkTheme ? '#1a1a2e' : '#fff'

  chart.setOption({
    backgroundColor: bgColor,
    xAxis: {data: times},
    series: seriesList
  })
}

function pausePlay() {
  isPlaying.value = false
  if (playTimer.value) {
    clearTimeout(playTimer.value)
    playTimer.value = null
  }
}

function stopPlay() {
  isPlaying.value = false
  if (playTimer.value) {
    clearTimeout(playTimer.value)
    playTimer.value = null
  }
  playIndex.value = cachedTimes.value.length
}

function togglePlay() {
  if (isPlaying.value) {
    pausePlay()
  } else {
    if (playIndex.value >= totalPoints.value) {
      playIndex.value = 0
    }
    isPlaying.value = true
    tickPlay()
  }
}

function onPlaySpeedChange(val: number) {
  playSpeed.value = val
}

function onSliderChange(val: number) {
  pausePlay()
  playIndex.value = val
  if (cachedAllData.value.length > 0) {
    renderPlayFrame()
  }
}

function addExtraSector(code: string) {
  if (!code) return
  const found = allConceptCodes.value.find((i: any) => i.code === code)
  if (!found) return
  if (extraSectors.value.some((i: any) => i.code === code)) return
  const displayCodes = new Set([
    ...inflowList.value.map((i: any) => i.code),
    ...outflowList.value.map((i: any) => i.code)
  ])
  if (displayCodes.has(code)) {
    addCodeInput.value = ''
    return
  }
  extraSectors.value.push({code: found.code, name: found.name, netInflow: 0, isInflow: true})
  addCodeInput.value = ''
  loadAllData()
}

function removeExtraSector(code: string) {
  extraSectors.value = extraSectors.value.filter((i: any) => i.code !== code)
  loadAllData()
}

function onDateChange() {
  loadAllData()
}

watch(() => props.darkTheme, () => {
  loadAllData()
})

watch(() => props.chartHeight, () => {
  if (chart) chart.resize()
})
</script>

<template>
  <div style="width: 100%">
    <!-- 控制栏 -->
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap;">
      <n-tag :bordered="false" type="error" size="small">红色系 = 流入前20</n-tag>
      <n-tag :bordered="false" type="success" size="small">绿色系 = 流出前20</n-tag>
      <n-date-picker
          v-model:formatted-value="selectedDate"
          type="date"
          value-format="yyyy-MM-dd"
          :is-date-disabled="isDateDisabled"
          style="width: 150px"
          @update:formatted-value="onDateChange"
      />
      <n-button size="small" :loading="loading" @click="loadAllData">
        刷新
      </n-button>
      <n-select
          v-model:value="addCodeInput"
          :options="addCodeOptions"
          filterable
          placeholder="添加概念"
          style="width: 180px"
          @update:value="addExtraSector"
      />
      <n-tag
          v-for="item in extraSectors"
          :key="item.code"
          closable
          size="small"
          type="warning"
          @close="removeExtraSector(item.code)"
      >
        {{ item.name }}
      </n-tag>
      <n-text v-if="isToday" :depth="3" style="font-size: 12px; margin-left: auto;">
        交易时间自动每分钟刷新 (9:30-15:00)
      </n-text>
      <n-text v-else depth="3" style="font-size: 12px; margin-left: auto;">
        历史数据
      </n-text>
    </div>

    <!-- 播放控制栏 -->
    <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px; flex-wrap: wrap;">
      <n-button size="small" :type="isPlaying ? 'warning' : 'primary'" @click="togglePlay">
        {{ isPlaying ? '暂停' : '播放' }}
      </n-button>
      <n-button size="small" @click="startPlay" :disabled="totalPoints === 0">
        重播
      </n-button>
      <n-text :depth="3" style="font-size: 12px;">
        {{ totalPoints > 0 ? `${playIndex} / ${totalPoints}` : '无数据' }}
      </n-text>
      <n-slider
          v-model:value="playIndex"
          :max="totalPoints"
          :min="0"
          :step="1"
          style="width: 200px"
          :disabled="totalPoints === 0"
          @update:value="onSliderChange"
      />
      <n-text :depth="3" style="font-size: 12px;">倍速</n-text>
      <n-select
          :value="playSpeed"
          :options="[
            {label: '0.5x', value: 0.5},
            {label: '1x', value: 1},
            {label: '2x', value: 2},
            {label: '4x', value: 4},
            {label: '8x', value: 8}
          ]"
          style="width: 75px"
          size="small"
          @update:value="onPlaySpeedChange"
      />
    </div>

    <!-- 折线图 -->
    <div ref="chartRef" style="width: 100%;" :style="{height: chartHeight + 'px'}"></div>

    <!-- 概念资金排名表格 - 并排展示 -->
    <div style="margin-top: 20px; display: flex; gap: 20px; align-items: flex-start;">
      <!-- 流入排名 -->
      <div style="flex: 1; min-width: 0;">
        <n-h3 :style="{color: '#ee6666'}">流入 Top 20</n-h3>
        <n-table striped size="small">
          <n-thead>
            <n-tr>
              <n-th>排名</n-th>
              <n-th>概念名称</n-th>
              <n-th>净流入/亿元</n-th>
            </n-tr>
          </n-thead>
          <n-tbody>
            <n-tr v-for="(item, idx) in inflowList" :key="item.code">
              <n-td>{{ idx + 1 }}</n-td>
              <n-td>
                <n-tag :bordered="false" type="error" size="small">{{ item.name }}</n-tag>
              </n-td>
              <n-td>
                <n-text type="error">+{{ (item.netInflow / 100000000).toFixed(2) }}</n-text>
              </n-td>
            </n-tr>
            <n-tr v-if="inflowList.length === 0">
              <n-td colspan="3" style="text-align: center; color: #999;">暂无数据</n-td>
            </n-tr>
          </n-tbody>
        </n-table>
      </div>
      <!-- 流出排名 -->
      <div style="flex: 1; min-width: 0;">
        <n-h3 :style="{color: '#00da3c'}">流出 Top 20</n-h3>
        <n-table striped size="small">
          <n-thead>
            <n-tr>
              <n-th>排名</n-th>
              <n-th>概念名称</n-th>
              <n-th>净流入/亿元</n-th>
            </n-tr>
          </n-thead>
          <n-tbody>
            <n-tr v-for="(item, idx) in outflowList" :key="item.code">
              <n-td>{{ idx + 1 }}</n-td>
              <n-td>
                <n-tag :bordered="false" type="success" size="small">{{ item.name }}</n-tag>
              </n-td>
              <n-td>
                <n-text type="success">{{ (item.netInflow / 100000000).toFixed(2) }}</n-text>
              </n-td>
            </n-tr>
            <n-tr v-if="outflowList.length === 0">
              <n-td colspan="3" style="text-align: center; color: #999;">暂无数据</n-td>
            </n-tr>
          </n-tbody>
        </n-table>
      </div>
    </div>
  </div>
</template>

<style scoped>
</style>
