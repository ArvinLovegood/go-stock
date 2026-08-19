<script setup>

import {
  GetTodayMarketStatistic,
  GetMarketStatisticByDate,
  GetIndexTline,
  GetSectorAnchors,
  GetMarketEmotion,
  GetIndexQuotes,
  GlobalStockIndexes,
  RzrqTrend,
  GetGlobalIndexTrend,
  GetKoreaDayKLine
} from "../../wailsjs/go/main/App";
import * as echarts from "echarts";
import {onMounted, onUnmounted, ref, computed, nextTick} from "vue";
import {ChevronDownOutline, ChevronForwardOutline} from "@vicons/ionicons5";
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
const rzrqChartRef = ref(null);
const kospiChartRef = ref(null);
const hynixChartRef = ref(null);
const samsungChartRef = ref(null);
let handleChartInterval = null

// 韩国市场图表（KOSPI + SK海力士 + 三星电子）：显示/隐藏 + 分时/日K切换 + 数据与错误状态
// 默认隐藏，用户点击展开后记住选择（存 '1' 才显示）
const koreaChartsVisible = ref(localStorage.getItem('koreaChartsVisible') === '1')
const kospiResult = ref(null)
const hynixResult = ref(null)
const samsungResult = ref(null)
const kospiError = ref('')
const hynixError = ref('')
const samsungError = ref('')
// 日K模式状态（数据源 Naver：分时主源东财失败时后端自动切 Naver 兜底；日K仅 Naver，腾讯韩股历史K线仅返回1根不可用）
const kospiMode = ref('trend')
const hynixMode = ref('trend')
const samsungMode = ref('trend')
const kospiDayK = ref(null)
const hynixDayK = ref(null)
const samsungDayK = ref(null)
const kospiDayError = ref('')
const hynixDayError = ref('')
const samsungDayError = ref('')

// 三张图的统一配置（代码/名称/单位/各状态 ref），驱动通用取数与渲染逻辑
const koreaCards = {
  kospi: {
    code: '100.KS11', name: '韩国KOSPI', unit: '点位',
    chartRef: kospiChartRef, result: kospiResult, error: kospiError,
    mode: kospiMode, dayK: kospiDayK, dayError: kospiDayError,
  },
  hynix: {
    code: '177.000660', name: 'SK海力士', unit: '韩元',
    chartRef: hynixChartRef, result: hynixResult, error: hynixError,
    mode: hynixMode, dayK: hynixDayK, dayError: hynixDayError,
  },
  samsung: {
    code: '177.005930', name: '三星电子', unit: '韩元',
    chartRef: samsungChartRef, result: samsungResult, error: samsungError,
    mode: samsungMode, dayK: samsungDayK, dayError: samsungDayError,
  },
}

// 当前模式下图表应展示的错误文案（空串=无错误）
function koreaCardError(key) {
  const c = koreaCards[key]
  return c.mode.value === 'day' ? c.dayError.value : c.error.value
}

// 切换分时/日K，切换后立即拉取并渲染
function setKoreaMode(key, mode) {
  const c = koreaCards[key]
  if (c.mode.value === mode) return
  c.mode.value = mode
  fetchKoreaCard(key)
}

function toggleKoreaCharts() {
  koreaChartsVisible.value = !koreaChartsVisible.value
  localStorage.setItem('koreaChartsVisible', koreaChartsVisible.value ? '1' : '0')
  // 展开时容器刚变为可见，需等 DOM 更新后再渲染（隐藏状态下 echarts 初始化会得到 0 尺寸）
  if (koreaChartsVisible.value) {
    nextTick(() => renderKoreaCharts())
  }
}

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

// A股四大指数（上证指数/深证成指/创业板指/科创50）- 从财联社API获取
const aShareIndexNames = ['上证指数', '深证成指', '创业板指', '科创50']
const aShareIndexes = ref([])
async function handleIndexQuotes() {
  try {
    const quotes = await GetIndexQuotes()
    if (!quotes || quotes.length === 0) return
    aShareIndexes.value = aShareIndexNames
      .map(n => {
        const q = quotes.find(i => i.secu_name === n || i.secu_name.startsWith(n) || n.startsWith(i.secu_name))
        return q ? {
          name: q.secu_name,
          lastPx: q.last_px,
          change: (q.change * 100).toFixed(2) + '%'
        } : null
      })
      .filter(Boolean)
    console.log('[AnalyzeMartket] aShareIndexes:', aShareIndexes.value)
  } catch (error) {
    console.error('获取A股指数行情失败:', error)
  }
}

onMounted(() => {
  handleChart()
  handleTlineChart()
  handleKoreaCharts()
  handleRzrqChart()
  handleGlobalIndexes()
  handleIndexQuotes()
  handleEmotion()
  handleChartInterval = setInterval(function () {
    handleGlobalIndexes()
    handleIndexQuotes()
    handleEmotion()
    // 韩国KOSPI指数与SK海力士分时始终为当日实时数据（KST交易时段与A股不同，不受日期筛选影响）
    handleKoreaCharts()
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

// 韩国市场图表：KOSPI + SK海力士 + 三星电子
// 分时：主源东财（后端内置 3 次重试 + Naver 兜底，KOSPI 指数 Naver 不支持分时仅东财）
// 日K：Naver（KOSPI 指数与个股全支持，250 根）
// 错误直接展示在图表区域（不吞掉），便于定位（如后端未重启导致绑定缺失、接口失败等）
async function handleKoreaCharts() {
  await Promise.allSettled(Object.keys(koreaCards).map(key => fetchKoreaCard(key)))
}

// 按当前模式拉取单张卡片数据（分时 or 日K）
async function fetchKoreaCard(key) {
  const c = koreaCards[key]
  const isDay = c.mode.value === 'day'
  try {
    if (isDay) {
      const kl = await GetKoreaDayKLine(c.code, 250)
      if (kl && kl.length > 0) {
        c.dayK.value = kl
        c.dayError.value = ''
      } else {
        c.dayK.value = null
        c.dayError.value = '暂无日K数据'
      }
    } else {
      const v = await GetGlobalIndexTrend(c.code)
      if (v && v.items && v.items.length > 0) {
        c.result.value = v
        c.error.value = ''
      } else {
        c.result.value = null
        c.error.value = '暂无分时数据'
      }
    }
  } catch (e) {
    const msg = '获取失败: ' + (e && e.message ? e.message : String(e))
    if (isDay) {
      c.dayK.value = null
      c.dayError.value = msg
    } else {
      c.result.value = null
      c.error.value = msg
    }
    console.error('获取' + c.name + (isDay ? '日K' : '分时') + '数据失败:', e)
  }
  renderKoreaCard(key)
}

// 仅在可见时渲染（隐藏状态下 echarts 初始化会得到 0 尺寸导致空白）
function renderKoreaCharts() {
  if (!koreaChartsVisible.value) return
  Object.keys(koreaCards).forEach(key => renderKoreaCard(key))
}

// 按当前模式渲染单张卡片
function renderKoreaCard(key) {
  if (!koreaChartsVisible.value) return
  const c = koreaCards[key]
  if (c.mode.value === 'day') {
    if (c.dayK.value) {
      renderKoreaDayChart(c.chartRef, c.dayK.value, c.name)
    }
  } else {
    if (c.result.value) {
      renderKoreaTrendChart(c.chartRef, c.result.value, c.name, c.unit)
    }
  }
}

function renderKoreaTrendChart(chartRef, result, defaultName, yUnit) {
  if (!chartRef.value || !result || !result.items || result.items.length === 0) return
  const chart = echarts.init(chartRef.value)

  const times = result.items.map(d => (d.time || '').split(' ')[1])
  const prices = result.items.map(d => d.price)
  const avgPrices = result.items.map(d => d.avgPrice)
  const preClose = result.preClose || 0
  const last = prices[prices.length - 1] || 0
  const changeVal = preClose > 0 ? last - preClose : 0
  const changePct = preClose > 0 ? changeVal / preClose * 100 : 0
  const upColor = '#ef4444'
  const downColor = '#22c55e'
  const priceColor = changePct >= 0 ? upColor : downColor

  const textColor = darkTheme ? '#ccc' : '#333'
  const subTextColor = darkTheme ? '#999' : '#666'
  const lineColor = darkTheme ? '#444' : '#ccc'

  const option = {
    darkMode: darkTheme,
    title: {
      text: (result.name || defaultName) + ' 分时',
      left: 'center',
      textStyle: {color: textColor, fontSize: 14},
      subtext: last.toFixed(2) + '  ' + (changeVal >= 0 ? '+' : '') + changeVal.toFixed(2)
        + '  ' + (changePct >= 0 ? '+' : '') + changePct.toFixed(2) + '%'
        + (result.date ? '（' + result.date + '）' : ''),
      subtextStyle: {color: priceColor, fontSize: 12}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      formatter: function (params) {
        let html = '<b>' + params[0].axisValue + '</b><br/>'
        const idx = params[0].dataIndex
        params.forEach(function (p) {
          if (p.value == null) return
          html += p.marker + ' ' + p.seriesName + ': <b>' + p.value.toFixed(2) + '</b><br/>'
        })
        const price = prices[idx]
        if (preClose > 0 && price != null) {
          const pct = (price - preClose) / preClose * 100
          const color = pct >= 0 ? upColor : downColor
          html += '<span style="color:' + color + '">涨跌幅: ' + (pct >= 0 ? '+' : '') + pct.toFixed(2) + '%</span>'
        }
        return html
      }
    },
    legend: {
      data: [yUnit, '均价'],
      top: 45,
      textStyle: {color: textColor, fontSize: 11}
    },
    grid: {left: '3%', right: '4%', bottom: '3%', top: 70, containLabel: true},
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLabel: {color: subTextColor},
      axisLine: {lineStyle: {color: lineColor}}
    },
    yAxis: {
      type: 'value', name: yUnit, scale: true,
      axisLabel: {color: subTextColor},
      axisLine: {lineStyle: {color: lineColor}},
      splitLine: {lineStyle: {color: darkTheme ? '#333' : '#eee'}}
    },
    series: [
      {
        name: yUnit,
        type: 'line',
        data: prices,
        smooth: false,
        symbol: 'none',
        lineStyle: {color: priceColor, width: 1.5},
        itemStyle: {color: priceColor},
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            {offset: 0, color: changePct >= 0 ? 'rgba(239, 68, 68, 0.15)' : 'rgba(34, 197, 94, 0.15)'},
            {offset: 1, color: 'rgba(59, 130, 246, 0.01)'}
          ])
        },
        markLine: {
          silent: true,
          symbol: 'none',
          label: {
            formatter: '昨收 ' + preClose.toFixed(2),
            position: 'insideEndTop',
            color: subTextColor,
            fontSize: 10
          },
          lineStyle: {color: '#888', type: 'dashed', width: 1},
          data: preClose > 0 ? [{yAxis: preClose}] : []
        }
      },
      {
        name: '均价',
        type: 'line',
        data: avgPrices,
        smooth: false,
        symbol: 'none',
        lineStyle: {color: '#f59e0b', width: 1},
        itemStyle: {color: '#f59e0b'}
      }
    ]
  }

  chart.setOption(option)
  chart.resize()
}

// 韩股日K：蜡烛图 + MA5/10/20 + 成交量（数据源 Naver，KLineData 字段为字符串）
function renderKoreaDayChart(chartRef, klines, name) {
  if (!chartRef.value || !klines || klines.length === 0) return
  const chart = echarts.init(chartRef.value)

  const dates = klines.map(k => k.day)
  const opens = klines.map(k => parseFloat(k.open))
  const closes = klines.map(k => parseFloat(k.close))
  // ECharts candlestick 数据格式：[开, 收, 低, 高]
  const ohlc = klines.map(k => [parseFloat(k.open), parseFloat(k.close), parseFloat(k.low), parseFloat(k.high)])
  const volumes = klines.map((k, i) => ({
    value: parseFloat(k.volume),
    itemStyle: {color: closes[i] >= opens[i] ? 'rgba(239,68,68,0.7)' : 'rgba(34,197,94,0.7)'}
  }))
  const calcMA = n => closes.map((_, i) => {
    if (i < n - 1) return null
    let s = 0
    for (let j = i - n + 1; j <= i; j++) s += closes[j]
    return +(s / n).toFixed(2)
  })

  const last = closes[closes.length - 1]
  const prev = closes.length > 1 ? closes[closes.length - 2] : last
  const changeVal = last - prev
  const changePct = prev > 0 ? changeVal / prev * 100 : 0
  const upColor = '#ef4444'
  const downColor = '#22c55e'
  const priceColor = changeVal >= 0 ? upColor : downColor
  // 韩股个股价格量级大（如三星 248250 韩元）不显示小数；KOSPI 指数保留两位
  const fmt = v => v >= 10000 ? v.toFixed(0) : v.toFixed(2)

  const textColor = darkTheme ? '#ccc' : '#333'
  const subTextColor = darkTheme ? '#999' : '#666'
  const lineColor = darkTheme ? '#444' : '#ccc'

  const option = {
    darkMode: darkTheme,
    title: {
      text: name + ' 日K',
      left: 'center',
      textStyle: {color: textColor, fontSize: 14},
      subtext: fmt(last) + '  ' + (changeVal >= 0 ? '+' : '') + fmt(changeVal)
        + '  ' + (changePct >= 0 ? '+' : '') + changePct.toFixed(2) + '%'
        + '（' + dates[0] + ' ~ ' + dates[dates.length - 1] + '）',
      subtextStyle: {color: priceColor, fontSize: 12}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      formatter: function (params) {
        const idx = params[0].dataIndex
        let html = '<b>' + dates[idx] + '</b><br/>'
        const k = klines[idx]
        if (k) {
          html += '开: ' + k.open + ' 高: ' + k.high + '<br/>低: ' + k.low + ' 收: <b>' + k.close + '</b><br/>量: ' + k.volume
        }
        return html
      }
    },
    legend: {
      data: ['MA5', 'MA10', 'MA20'],
      top: 45,
      textStyle: {color: textColor, fontSize: 11}
    },
    grid: [
      {left: '3%', right: '3%', top: 70, height: '52%'},
      {left: '3%', right: '3%', top: '78%', height: '14%'}
    ],
    xAxis: [
      {
        type: 'category', data: dates, gridIndex: 0,
        axisLabel: {color: subTextColor},
        axisLine: {lineStyle: {color: lineColor}}
      },
      {
        type: 'category', data: dates, gridIndex: 1,
        axisLabel: {show: false}, axisTick: {show: false},
        axisLine: {lineStyle: {color: lineColor}}
      }
    ],
    yAxis: [
      {
        type: 'value', gridIndex: 0, scale: true,
        axisLabel: {color: subTextColor},
        axisLine: {lineStyle: {color: lineColor}},
        splitLine: {lineStyle: {color: darkTheme ? '#333' : '#eee'}}
      },
      {
        type: 'value', gridIndex: 1, scale: true,
        axisLabel: {show: false}, splitLine: {show: false}
      }
    ],
    // 内部缩放默认显示最近 30%（约75根），可滚轮/拖动查看更多
    dataZoom: [{type: 'inside', xAxisIndex: [0, 1], start: 70, end: 100}],
    series: [
      {
        name: 'K线',
        type: 'candlestick',
        data: ohlc,
        xAxisIndex: 0,
        yAxisIndex: 0,
        itemStyle: {color: upColor, color0: downColor, borderColor: upColor, borderColor0: downColor}
      },
      {name: 'MA5', type: 'line', data: calcMA(5), symbol: 'none', smooth: false, lineStyle: {width: 1}, xAxisIndex: 0, yAxisIndex: 0},
      {name: 'MA10', type: 'line', data: calcMA(10), symbol: 'none', smooth: false, lineStyle: {width: 1}, xAxisIndex: 0, yAxisIndex: 0},
      {name: 'MA20', type: 'line', data: calcMA(20), symbol: 'none', smooth: false, lineStyle: {width: 1}, xAxisIndex: 0, yAxisIndex: 0},
      {
        name: '成交量',
        type: 'bar',
        data: volumes,
        xAxisIndex: 1,
        yAxisIndex: 1
      }
    ]
  }

  chart.setOption(option)
  chart.resize()
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
    globalIndexes.value = all.map(item => {
      const zxj = parseFloat(item.zxj)
      return {
        name: item.name || item.code || '',
        lastPx: isNaN(zxj) ? null : zxj,
        change: item.zdf || ''
      }
    }).filter(i => i.name)
    console.log('[AnalyzeMartket] globalIndexes names:', globalIndexes.value.map(i => i.name))
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

// 融资融券走势图
async function handleRzrqChart() {
  try {
    const res = await RzrqTrend('', '')
    if (res && res.items && res.items.length > 0) {
      renderRzrqChart(res.items, res.rzyeUnit || '亿', res.rzjlrUnit || '亿', res.updateTime || '')
    }
  } catch (error) {
    console.error('获取融资融券走势数据失败:', error)
  }
}

function renderRzrqChart(items, rzyeUnit, rzjlrUnit, updateTime) {
  if (!rzrqChartRef.value || !items || items.length === 0) return
  const chart = echarts.init(rzrqChartRef.value)
  const dates = items.map(i => i.date)
  const rzyeVals = items.map(i => parseFloat(i.rzye) || 0)
  const rzjlrVals = items.map(i => parseFloat(i.rzjlr) || 0)
  const textColor = darkTheme ? '#aaa' : '#666'
  const axisColor = darkTheme ? '#444' : '#ccc'
  const splitColor = darkTheme ? '#333' : '#eee'
  const option = {
    darkMode: darkTheme,
    title: {
      text: '融资融券走势' + (updateTime ? '（' + updateTime + '）' : ''),
      left: 'center',
      textStyle: {color: darkTheme ? '#ccc' : '#333', fontSize: 14}
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {type: 'cross'},
      formatter: function (params) {
        let html = '<b>' + params[0].axisValue + '</b><br/>'
        params.forEach(function (p) {
          const val = typeof p.value === 'object' ? p.value.value : p.value
          if (val == null) return
          const unit = p.seriesName === '融资余额' ? rzyeUnit : rzjlrUnit
          const sign = val > 0 ? '+' : ''
          html += p.marker + ' ' + p.seriesName + ': <b>' + sign + val.toFixed(2) + unit + '</b><br/>'
        })
        return html
      }
    },
    legend: {
      data: ['融资余额', '融资净买入'],
      top: 25,
      textStyle: {color: textColor, fontSize: 11}
    },
    grid: {left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true},
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: {lineStyle: {color: axisColor}},
      splitLine: {show: false},
      axisLabel: {
        color: textColor,
        rotate: 30,
        fontSize: 10,
        interval: dates.length <= 30 ? 0 : Math.floor(dates.length / 8)
      }
    },
    yAxis: [
      {
        name: '融资余额(' + rzyeUnit + ')',
        type: 'value',
        position: 'left',
        nameTextStyle: {color: textColor, fontSize: 11},
        axisLine: {show: true, lineStyle: {color: axisColor}},
        splitLine: {lineStyle: {color: splitColor, type: 'dashed'}},
        axisLabel: {color: textColor, fontSize: 11}
      },
      {
        name: '融资净买入(' + rzjlrUnit + ')',
        type: 'value',
        position: 'right',
        nameTextStyle: {color: textColor, fontSize: 11},
        axisLine: {show: true, lineStyle: {color: axisColor}},
        splitLine: {show: false},
        axisLabel: {color: textColor, fontSize: 11}
      }
    ],
    series: [
      {
        name: '融资余额',
        type: 'line',
        yAxisIndex: 0,
        data: rzyeVals,
        smooth: true,
        showSymbol: false,
        lineStyle: {width: 2, color: '#5470c6'},
        itemStyle: {color: '#5470c6'},
        areaStyle: {color: 'rgba(84,112,198,0.1)'}
      },
      {
        name: '融资净买入',
        type: 'bar',
        yAxisIndex: 1,
        data: rzjlrVals.map(function (v) {
          return {value: v, itemStyle: {color: v >= 0 ? '#e88080' : '#00b42a'}}
        }),
        barWidth: '60%'
      }
    ],
    dataZoom: [
      {type: 'inside', xAxisIndex: [0], start: 0, end: 100}
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
          <n-text :type="getChangeType(idx.change)" strong style="margin:0 3px">{{ idx.lastPx != null ? idx.lastPx.toFixed(2) : '--' }}</n-text>
          <n-text :type="getChangeType(idx.change)" strong>{{ formatChange(idx.change) }}</n-text>
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
          <n-text v-for="idx in aShareIndexes" :key="idx.name" :type="getChangeType(idx.change)" strong style="font-size:13px;white-space:nowrap">
            {{ idx.name }}
            <n-text :type="getChangeType(idx.change)" strong style="margin-left:3px">{{ idx.lastPx?.toFixed(2) }}</n-text>
            <n-text :type="getChangeType(idx.change)" strong style="margin-left:3px">{{ formatChange(idx.change) }}</n-text>
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

    <!-- 指数分时图 + 涨跌停图 + 融资融券走势 三图一行 -->
    <div style="display:flex;gap:8px;align-items:stretch;--wails-draggable:no-drag">
      <div ref="tlineChartRef" style="flex:1;min-width:0;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}"></div>
      <div ref="limitChartRef" style="flex:1;min-width:0;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}"></div>
      <div ref="rzrqChartRef" style="flex:1;min-width:0;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}"></div>
    </div>

    <!-- 韩国市场：KOSPI指数 + SK海力士 + 三星电子 分时/日K（点击标题栏折叠/展开，卡片右上角切换分时/日K） -->
    <div @click="toggleKoreaCharts"
         style="display:flex;align-items:center;gap:4px;margin-top:8px;cursor:pointer;user-select:none;--wails-draggable:no-drag">
      <n-icon size="14" :depth="3" :component="koreaChartsVisible ? ChevronDownOutline : ChevronForwardOutline"/>
      <n-text depth="3" style="font-size:12px">韩国市场 · KOSPI / SK海力士 / 三星电子 分时 / 日K</n-text>
    </div>
    <div v-show="koreaChartsVisible"
         style="display:flex;gap:8px;align-items:stretch;margin-top:4px;--wails-draggable:no-drag">
      <div style="flex:1;min-width:0;position:relative;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}">
        <div style="position:absolute;top:2px;right:4px;z-index:5;--wails-draggable:no-drag">
          <n-button-group size="tiny">
            <n-button size="tiny" :type="kospiMode==='trend' ? 'primary' : 'default'"
                      @click="setKoreaMode('kospi','trend')">分时</n-button>
            <n-button size="tiny" :type="kospiMode==='day' ? 'primary' : 'default'"
                      @click="setKoreaMode('kospi','day')">日K</n-button>
          </n-button-group>
        </div>
        <div v-show="!koreaCardError('kospi')" ref="kospiChartRef" style="width:100%;height:100%;--wails-draggable:no-drag"></div>
        <div v-if="koreaCardError('kospi')"
             style="position:absolute;inset:0;display:flex;align-items:center;justify-content:center;padding:16px;text-align:center">
          <n-text type="error" style="font-size:12px">韩国KOSPI {{ koreaCardError('kospi') }}</n-text>
        </div>
      </div>
      <div style="flex:1;min-width:0;position:relative;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}">
        <div style="position:absolute;top:2px;right:4px;z-index:5;--wails-draggable:no-drag">
          <n-button-group size="tiny">
            <n-button size="tiny" :type="hynixMode==='trend' ? 'primary' : 'default'"
                      @click="setKoreaMode('hynix','trend')">分时</n-button>
            <n-button size="tiny" :type="hynixMode==='day' ? 'primary' : 'default'"
                      @click="setKoreaMode('hynix','day')">日K</n-button>
          </n-button-group>
        </div>
        <div v-show="!koreaCardError('hynix')" ref="hynixChartRef" style="width:100%;height:100%;--wails-draggable:no-drag"></div>
        <div v-if="koreaCardError('hynix')"
             style="position:absolute;inset:0;display:flex;align-items:center;justify-content:center;padding:16px;text-align:center">
          <n-text type="error" style="font-size:12px">SK海力士 {{ koreaCardError('hynix') }}</n-text>
        </div>
      </div>
      <div style="flex:1;min-width:0;position:relative;--wails-draggable:no-drag" :style="{height:chartHeight+'px'}">
        <div style="position:absolute;top:2px;right:4px;z-index:5;--wails-draggable:no-drag">
          <n-button-group size="tiny">
            <n-button size="tiny" :type="samsungMode==='trend' ? 'primary' : 'default'"
                      @click="setKoreaMode('samsung','trend')">分时</n-button>
            <n-button size="tiny" :type="samsungMode==='day' ? 'primary' : 'default'"
                      @click="setKoreaMode('samsung','day')">日K</n-button>
          </n-button-group>
        </div>
        <div v-show="!koreaCardError('samsung')" ref="samsungChartRef" style="width:100%;height:100%;--wails-draggable:no-drag"></div>
        <div v-if="koreaCardError('samsung')"
             style="position:absolute;inset:0;display:flex;align-items:center;justify-content:center;padding:16px;text-align:center">
          <n-text type="error" style="font-size:12px">三星电子 {{ koreaCardError('samsung') }}</n-text>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>

</style>
