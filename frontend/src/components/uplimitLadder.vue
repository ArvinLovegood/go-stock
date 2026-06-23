<script setup>
import {onBeforeMount, onBeforeUnmount, ref, computed, h} from 'vue'
import {GetConfig, GetUplimitHot, IsTradingTime, IsTradingDay, GetLatestTradingDay} from "../../wailsjs/go/main/App";
import {NButton, NText, NTag, NTooltip, NProgress, useMessage} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

const message = useMessage()
const loading = ref(false)
const rawData = ref(null)
const selectedPlate = ref(null)
const showPlateModal = ref(false)
const showKlineModal = ref(false)
const klineCode = ref('')
const klineName = ref('')
const activeView = ref('ladder')
const expandedLadders = ref([])
const darkTheme = ref(false)

function getCalendarTodayStr() {
  const t = new Date()
  return `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
}

/** 自然日「今天」，用于打开页默认选中日与仅在查看当天时自动刷新 */
const todayYMD = ref(getCalendarTodayStr())
const selectedDate = ref(getCalendarTodayStr())
let refreshTimer = null

function startAutoRefresh() {
  stopAutoRefresh()
  refreshTimer = setInterval(() => {
    if (selectedDate.value !== todayYMD.value) return
    IsTradingTime().then(trading => {
      if (trading) {
        fetchData(selectedDate.value)
      }
    }).catch(() => {})
  }, 60000)
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      darkTheme.value = true
    }
  })

  const cal = getCalendarTodayStr()
  todayYMD.value = cal

  IsTradingDay(cal)
    .then(isTd => {
      if (isTd) {
        return cal
      }
      return GetLatestTradingDay()
        .then(last => {
          const s = (last && String(last).trim()) || ''
          return s || cal
        })
        .catch(() => cal)
    })
    .catch(() => cal)
    .then(initial => {
      selectedDate.value = initial
      fetchData(initial)
      startAutoRefresh()
    })
})

onBeforeUnmount(() => {
  stopAutoRefresh()
})

function fetchData(date, retryCount = 0) {
  if (!date) return
  loading.value = true
  const d = typeof date === 'string' ? date : formatDate(date)
  selectedDate.value = d
  const loadingMsg = message.loading('正在获取涨停梯队数据...', { duration: 0 })
  GetUplimitHot(d, 20).then(res => {
    if (res && res.code === 20000) {
      const data = res.data
      const hasData = data && data.plate?.length > 0 && data.stocks && data.stocks.trim() !== ''
      if (hasData) {
        rawData.value = data
        loadingMsg.destroy()
        if (data.ban_info && data.max_count) {
          const expanded = []
          for (let i = data.max_count; i >= 1 && expanded.length < 3; i--) {
            const info = data.ban_info[String(i)]
            if (info && info.count > 0) {
              expanded.push(String(i))
            }
          }
          expandedLadders.value = expanded
        }
      } else if (retryCount < 7) {
        const prevDate = new Date(d)
        prevDate.setDate(prevDate.getDate() - 1)
        const prevDateStr = formatDate(prevDate)
        message.info(`当前日期 ${d} 暂无数据，尝试查询前一日：${prevDateStr}`)
        loadingMsg.destroy()
        fetchData(prevDateStr, retryCount + 1)
        return
      } else {
        rawData.value = data
        loadingMsg.destroy()
        message.info('暂无历史数据')
      }
    } else {
      loadingMsg.destroy()
      message.error(res?.message || '获取数据失败')
    }
  }).catch(err => {
    loadingMsg.destroy()
    message.error('请求失败')
    console.error(err)
  }).finally(() => {
    if (retryCount === 0 || rawData.value) {
      loading.value = false
    }
  })
}

function formatDate(d) {
  if (!d) return ''
  const date = new Date(d)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const plateList = computed(() => {
  if (!rawData.value?.plate) return []
  return rawData.value.plate.map(p => ({
    name: p[0],
    code: p[1],
    score: p[2]
  }))
})

const plateInfo = computed(() => rawData.value?.plate_info || {})

const banInfo = computed(() => {
  if (!rawData.value?.ban_info) return []
  const result = []
  for (let i = rawData.value.max_count || 7; i >= 1; i--) {
    const info = rawData.value.ban_info[String(i)]
    if (info) {
      result.push({level: i, count: info.count})
    }
  }
  return result
})

const totalZtCount = computed(() => {
  if (!rawData.value?.stocks) return 0
  return rawData.value.stocks.split(',').filter(s => s.trim()).length
})

const maxCount = computed(() => rawData.value?.max_count || 0)

const ladderData = computed(() => {
  if (!rawData.value?.plate_stocks || !rawData.value?.stock_info) return {}
  const stocksStr = rawData.value.stocks || ''
  const allCodes = stocksStr.split(',').filter(s => s.trim())
  const stockInfo = rawData.value.stock_info || {}

  const ladder = {}
  for (let i = maxCount.value; i >= 1; i--) {
    ladder[i] = []
  }

  const seen = new Set()
  for (const code of allCodes) {
    if (seen.has(code)) continue
    seen.add(code)

    let stockData = null
    for (const plateCode of Object.keys(rawData.value.plate_stocks)) {
      const found = rawData.value.plate_stocks[plateCode].find(s => s.stock_code === code)
      if (found) {
        stockData = found
        break
      }
    }
    if (!stockData) continue

    const keepTimes = stockData.up_limit_keep_times || 0
    if (keepTimes >= 1 && ladder[keepTimes]) {
      const info = stockInfo[code]
      ladder[keepTimes].push({
        ...stockData,
        plates: info?.plates || []
      })
    }
  }

  for (const key of Object.keys(ladder)) {
    ladder[key].sort((a, b) => {
      if (a.up_limit_keep_times !== b.up_limit_keep_times) {
        return b.up_limit_keep_times - a.up_limit_keep_times
      }
      return a.up_limit_time?.localeCompare(b.up_limit_time || '') || 0
    })
  }

  return ladder
})

const stockDetailMap = computed(() => {
  const map = {}
  if (!rawData.value?.plate_stocks) return map
  for (const stocks of Object.values(rawData.value.plate_stocks)) {
    for (const s of stocks) {
      if (s.stock_code && !map[s.stock_code]) {
        map[s.stock_code] = s
      }
    }
  }
  return map
})

const explodedStocks = computed(() => {
  if (!rawData.value?.plate_stocks_zb) return []
  const result = []
  const seen = new Set()
  for (const [plateCode, stocks] of Object.entries(rawData.value.plate_stocks_zb)) {
    for (const s of stocks) {
      if (!seen.has(s.stock_code)) {
        seen.add(s.stock_code)
        const info = rawData.value?.stock_info?.[s.stock_code]
        const detail = stockDetailMap.value[s.stock_code] || {}
        result.push({
          ...s,
          fd_max: s.fd_max || detail.fd_max || '',
          fd_close: s.fd_close || detail.fd_close || '',
          amount: s.amount || detail.amount || '',
          market_c: s.market_c || detail.market_c || '',
          plates: info?.plates || []
        })
      }
    }
  }
  result.sort((a, b) => (a.up_limit_time || '').localeCompare(b.up_limit_time || ''))
  return result
})

const plateStocksFiltered = computed(() => {
  if (!rawData.value?.plate_stocks) return []
  if (!selectedPlate.value) return []
  const stocks = rawData.value.plate_stocks[selectedPlate.value] || []
  const stockInfo = rawData.value?.stock_info || {}
  return stocks.map(s => ({
    ...s,
    plates: stockInfo[s.stock_code]?.plates || []
  }))
})

const stockNameMap = computed(() => {
  const map = {}
  for (const [code, s] of Object.entries(stockDetailMap.value)) {
    map[code] = s.stock_name || ''
  }
  return map
})

const stocksHot = computed(() => {
  if (!rawData.value?.stocks_hot) return []
  const hot = rawData.value.stocks_hot
  const stockInfo = rawData.value?.stock_info || {}
  const result = Object.entries(hot).map(([code, score]) => ({
    code,
    name: stockNameMap.value[code] || '',
    score,
    plates: stockInfo[code]?.plates || []
  }))
  result.sort((a, b) => b.score - a.score)
  return result
})

const hotThreshold = computed(() => rawData.value?.stocks_hot_n || 7)

const relayPlates = computed(() => {
  if (!rawData.value?.relay?.area) return []
  return rawData.value.relay.area.map(item => {
    const info = plateInfo.value[item.p_code]
    return {
      ...item,
      name: info?.name || item.p_code
    }
  })
})

function getTypeColor(type) {
  if (!type) return 'default'
  if (type === '一') return '#e03030'
  if (type === 'T') return '#f0a020'
  if (type === '自') return '#2080f0'
  if (type.startsWith('烂')) return '#f0a020'
  if (type === '炸') return '#999'
  return 'default'
}

function getTypeLabel(type) {
  if (!type) return ''
  if (type === '一') return '一字'
  if (type === 'T') return 'T字'
  if (type === '自') return '自然'
  if (type.startsWith('烂')) return '烂' + type.slice(1) + '板'
  if (type === '炸') return '炸板'
  return type
}

function getFdCloseColor(val) {
  if (val >= 3) return '#18a058'
  if (val >= 1) return '#2080f0'
  if (val > 0) return '#f0a020'
  return '#e03030'
}

function getScoreBarWidth(score, maxScore) {
  if (!maxScore) return 0
  return Math.min(100, (score / maxScore) * 100)
}

const plateTableMaxHeight = computed(() => Math.max(300, window.innerHeight * 0.7))

const plateStockColumns = [
  {title: '代码', key: 'stock_code', width: 90, render: (row) => h(NText, {depth: 3, style: 'font-size:12px'}, () => row.stock_code)},
  {title: '名称', key: 'stock_name', width: 80, render: (row) => h(NText, {strong: true, style: 'cursor:pointer;color:#2080f0;text-decoration:underline;', onClick: () => showKline(row.stock_code, row.stock_name)}, () => row.stock_name)},
  {title: '类型', key: 'up_limit_type', width: 70, render: (row) => h(NTag, {color: {color: getTypeColor(row.up_limit_type), textColor: '#fff'}, size: 'tiny', round: true}, () => getTypeLabel(row.up_limit_type))},
  {title: '描述', key: 'up_limit_desc', width: 70, render: (row) => row.up_limit_desc ? h(NTag, {size: 'tiny', type: row.up_limit_keep_times >= 3 ? 'error' : 'default', round: true}, () => row.up_limit_desc) : ''},
  {title: '时间', key: 'up_limit_time', width: 70, render: (row) => h(NText, {depth: 3, style: 'font-size:12px'}, () => row.up_limit_time || '')},
  {title: '封单比', key: 'fd_max', width: 65, render: (row) => h(NText, {style: 'font-size:12px'}, () => row.fd_max + '%')},
  {title: '收盘封单', key: 'fd_close', width: 75, render: (row) => h(NText, {style: 'color:' + getFdCloseColor(row.fd_close) + ';font-size:12px;font-weight:bold'}, () => row.fd_close + '%')},
  {title: '成交额', key: 'amount', width: 70, render: (row) => h(NText, {style: 'font-size:12px'}, () => row.amount + '亿')},
  {title: '市值', key: 'market_c', width: 70, render: (row) => h(NText, {style: 'font-size:12px'}, () => row.market_c + '亿')},
]

function selectPlate(code) {
  selectedPlate.value = code
  showPlateModal.value = true
}

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

function showKline(code, name) {
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  klineCode.value = em
  klineName.value = name || ''
  showKlineModal.value = true
}
</script>

<template>
  <div style="padding: 0 4px;">
    <n-spin :show="loading">
      <n-space vertical :size="16">
        <n-card size="small" :bordered="true">
          <n-space justify="space-between" align="center">
            <n-space align="center" :size="16">
              <n-date-picker
                v-model:formatted-value="selectedDate"
                value-format="yyyy-MM-dd"
                type="date"
                size="small"
                style="width: 150px"
                :on-update:formatted-value="(v) => { if(v) fetchData(v) }"
              />
              <n-tag type="success" size="small" round>涨停 {{ totalZtCount }} 只</n-tag>
              <n-tag type="warning" size="small" round>最高 {{ maxCount }} 连板</n-tag>
              <n-tag v-if="rawData?.today" type="info" size="small" round>实时数据</n-tag>
            </n-space>
            <n-space>
              <n-button :type="activeView==='ladder'?'primary':'default'" size="small" @click="activeView='ladder'">涨停高度</n-button>
              <n-button :type="activeView==='plate'?'primary':'default'" size="small" @click="activeView='plate'">板块热度</n-button>
              <n-button :type="activeView==='hot'?'primary':'default'" size="small" @click="activeView='hot'">个股热度</n-button>
              <n-button :type="activeView==='exploded'?'primary':'default'" size="small" @click="activeView='exploded'">炸板股</n-button>
            </n-space>
          </n-space>
        </n-card>

        <template v-if="activeView==='ladder'">
          <n-collapse v-model:expanded-names="expandedLadders" :accordion="false">
            <n-collapse-item v-for="level in banInfo" :key="level.level" :name="String(level.level)">
              <template #header>
                <n-space align="center" :size="8">
                  <n-tag :type="level.level>=5?'error':level.level>=3?'warning':'info'" round size="small" style="font-weight:bold;">
                    {{ level.level }} 板
                  </n-tag>
                  <n-text depth="3" style="font-size:12px;">{{ level.count }}只</n-text>
                </n-space>
              </template>
              <n-space vertical :size="8">
                <n-card v-for="stock in ladderData[level.level]" :key="stock.stock_code"
                  size="small" :bordered="true" embedded
                  :style="'border-left: 3px solid ' + getTypeColor(stock.up_limit_type)">
                  <n-space justify="space-between" align="center" wrap :size="8">
                    <n-space align="center" :size="8" wrap>
                      <n-text strong style="font-size:15px;cursor:pointer;color:#2080f0;text-decoration:underline;" @click="showKline(stock.stock_code, stock.stock_name)">{{ stock.stock_name }}</n-text>
                      <n-text depth="3" style="font-size:12px;">{{ stock.stock_code }}</n-text>
                      <n-tag :color="{color: getTypeColor(stock.up_limit_type), textColor:'#fff'}" size="tiny" round>
                        {{ getTypeLabel(stock.up_limit_type) }}
                      </n-tag>
                      <n-tag v-if="stock.up_limit_desc" type="primary" size="tiny" round>{{ stock.up_limit_desc }}</n-tag>
                      <n-text depth="3" style="font-size:12px;">{{ stock.up_limit_time }}</n-text>
                    </n-space>
                    <n-space align="center" :size="12" wrap>
                      <n-space align="center" :size="4">
                        <n-text depth="3" style="font-size:11px;">封单</n-text>
                        <n-text :style="'color:'+getFdCloseColor(stock.fd_close)+';font-weight:bold;font-size:13px;'">{{ stock.fd_close }}%</n-text>
                      </n-space>
                      <n-space align="center" :size="4">
                        <n-text depth="3" style="font-size:11px;">成交</n-text>
                        <n-text style="font-size:13px;">{{ stock.amount }}亿</n-text>
                      </n-space>
                      <n-space align="center" :size="4">
                        <n-text depth="3" style="font-size:11px;">市值</n-text>
                        <n-text style="font-size:13px;">{{ stock.market_c }}亿</n-text>
                      </n-space>
                    </n-space>
                  </n-space>
                  <n-space :size="4" style="margin-top:4px;" v-if="stock.plates && stock.plates.length">
                    <n-tag v-for="p in stock.plates.slice(0,6)" :key="p" size="tiny" :bordered="false" type="info">{{ p }}</n-tag>
                  </n-space>
                </n-card>
              </n-space>
            </n-collapse-item>
          </n-collapse>
          <n-card v-if="banInfo.length === 0 && !loading" size="small">
            <n-empty description="暂无连板数据"/>
          </n-card>
        </template>

        <template v-if="activeView==='plate'">
          <n-space vertical :size="12">
            <n-card size="small" :bordered="true" v-if="relayPlates.length">
              <template #header>
                <n-text style="font-weight:bold;">🔥 接力主线</n-text>
              </template>
              <n-space :size="8" wrap>
                <n-tag v-for="rp in relayPlates" :key="rp.p_code" round
                  :type="rp.p_score > 5000 ? 'error' : rp.p_score > 2000 ? 'warning' : 'info'"
                  style="cursor:pointer;font-size:13px;"
                  @click="selectPlate(rp.p_code)">
                  {{ rp.name }}
                  <template #avatar>
                    <n-text style="font-size:11px;opacity:0.7;margin-right:2px;">{{ rp.count }}只</n-text>
                  </template>
                </n-tag>
              </n-space>
            </n-card>

            <n-grid :cols="2" :x-gap="12" :y-gap="12" responsive="screen">
              <n-gi v-for="plate in plateList" :key="plate.code">
                <n-card size="small" :bordered="true"
                  style="cursor:pointer;" @click="selectPlate(plate.code)">
                  <n-space justify="space-between" align="center">
                    <n-space align="center" :size="8">
                      <n-text strong style="font-size:14px;">{{ plate.name }}</n-text>
                      <n-tag size="tiny" round :type="plate.score>5000?'error':plate.score>2000?'warning':'info'">
                        热度 {{ plate.score }}
                      </n-tag>
                    </n-space>
                    <n-text depth="3" style="font-size:12px;">
                      涨停{{ rawData?.plate_stocks?.[plate.code]?.length || 0 }}只
                      炸板{{ rawData?.plate_stocks_zb?.[plate.code]?.length || 0 }}只
                    </n-text>
                  </n-space>
                  <n-progress
                    :percentage="getScoreBarWidth(plate.score, plateList[0]?.score || 1)"
                    :show-indicator="false"
                    :color="plate.score>5000?'#e03030':plate.score>2000?'#f0a020':'#2080f0'"
                    :height="4"
                    style="margin-top:6px;"
                  />
                </n-card>
              </n-gi>
            </n-grid>
          </n-space>
        </template>

        <template v-if="activeView==='hot'">
          <n-card size="small" :bordered="true">
            <template #header>
              <n-text style="font-weight:bold;">个股热度排行</n-text>
              <n-text depth="3" style="font-size:12px;margin-left:8px;">热度≥{{ hotThreshold }}为超级热门</n-text>
            </template>
            <n-table :single-line="false" striped size="small" style="font-size:13px;">
              <n-thead>
                <n-tr>
                  <n-th width="50px">排名</n-th>
                  <n-th>代码</n-th>
                  <n-th>名称</n-th>
                  <n-th>热度</n-th>
                  <n-th>概念板块</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="(item, idx) in stocksHot" :key="item.code">
                  <n-td>
                    <n-tag v-if="idx<3" type="error" size="tiny" round>{{ idx+1 }}</n-tag>
                    <n-text v-else depth="3">{{ idx+1 }}</n-text>
                  </n-td>
                  <n-td><n-text strong>{{ item.code }}</n-text></n-td>
                  <n-td><n-text strong style="cursor:pointer;color:#2080f0;text-decoration:underline;" @click="showKline(item.code, item.name)">{{ item.name }}</n-text></n-td>
                  <n-td>
                    <n-space align="center" :size="4">
                      <n-progress
                        type="line"
                        :percentage="Math.min(100, (item.score / (stocksHot[0]?.score || 1)) * 100)"
                        :show-indicator="false"
                        :color="item.score >= hotThreshold ? '#e03030' : '#2080f0'"
                        :height="8"
                        style="width:80px;"
                      />
                      <n-text :type="item.score >= hotThreshold ? 'error' : 'default'" strong>{{ item.score }}</n-text>
                    </n-space>
                  </n-td>
                  <n-td>
                    <n-space :size="4" wrap>
                      <n-tag v-for="p in item.plates.slice(0,6)" :key="p" size="tiny" :bordered="false" type="info">{{ p }}</n-tag>
                    </n-space>
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-card>
        </template>

        <template v-if="activeView==='exploded'">
          <n-card size="small" :bordered="true">
            <template #header>
              <n-space align="center" :size="8">
                <n-text style="font-weight:bold;">炸板股</n-text>
                <n-tag type="warning" size="small" round>{{ explodedStocks.length }}只</n-tag>
              </n-space>
            </template>
            <n-table v-if="explodedStocks.length" :single-line="false" striped size="small" style="font-size:13px;">
              <n-thead>
                <n-tr>
                  <n-th>代码</n-th>
                  <n-th>名称</n-th>
                  <n-th>时间</n-th>
                  <n-th>最高封单</n-th>
                  <n-th>成交额</n-th>
                  <n-th>市值</n-th>
                  <n-th>概念板块</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="s in explodedStocks" :key="s.stock_code + s.plate_code">
                  <n-td><n-text depth="3" style="font-size:12px;">{{ s.stock_code }}</n-text></n-td>
                  <n-td><n-text type="error" style="cursor:pointer;text-decoration:underline;" @click="showKline(s.stock_code, s.stock_name)">{{ s.stock_name }}</n-text></n-td>
                  <n-td><n-text depth="3" style="font-size:12px;">{{ s.up_limit_time }}</n-text></n-td>
                  <n-td><n-text style="font-size:12px;">{{ s.fd_max }}%</n-text></n-td>
                  <n-td><n-text style="font-size:12px;">{{ s.amount }}亿</n-text></n-td>
                  <n-td><n-text style="font-size:12px;">{{ s.market_c }}亿</n-text></n-td>
                  <n-td>
                    <n-space :size="4" wrap>
                      <n-tag v-for="p in s.plates.slice(0,5)" :key="p" size="tiny" :bordered="false" type="info">{{ p }}</n-tag>
                    </n-space>
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
            <n-empty v-else description="暂无炸板数据"/>
          </n-card>
        </template>

      </n-space>
    </n-spin>

    <n-modal v-model:show="showPlateModal" preset="card"
      :title="plateInfo[selectedPlate]?.name + ' - 涨停股详情' || '涨停股详情'"
      style="width: 900px; max-width: 95vw;"
      :bordered="true" :segmented="{content:true}">
      <n-data-table :columns="plateStockColumns" :data="plateStocksFiltered"
        :max-height="plateTableMaxHeight" virtual-scroll
        size="small" :bordered="false" striped />
    </n-modal>

    <n-modal v-model:show="showKlineModal" preset="card"
      :title="(klineName || '') + ' — 多周期K线'"
      style="width: 95vw; max-width: 1200px;"
      :bordered="true">
      <stock-lightweight-kline-chart
        v-if="showKlineModal"
        :dark-theme="darkTheme"
        :key="'kline-' + klineCode"
        :code="klineCode"
        :stock-name="klineName"
        :chart-height="500"
      />
    </n-modal>
  </div>
</template>

<style scoped>
:deep(.n-card) {
  transition: all 0.2s ease;
}
:deep(.n-tag) {
  font-size: 12px;
}
</style>
