<script setup lang="ts">
import {computed, onBeforeMount, ref} from 'vue'
import {ConceptEventList, GetConfig} from "../../wailsjs/go/main/App";
import {useMessage} from "naive-ui";
import {format, parse} from 'date-fns';
import {zhCN} from 'date-fns/locale';
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import ConceptDetailModal from "./ConceptDetailModal.vue";

interface Theme {
  id: string
  type: string
  showName: string
  indexCode: string
  marketId: string
  indexName: string
  blockId: string | null
}

interface TopStock {
  marketId: string
  stockCode: string
  stockName: string
  risePercent: number
  limitUpState: number | null
  reason: string | null
}

interface ConceptEvent {
  eventId: string
  title: string
  heat: number
  themes: Theme[]
  topStocks: TopStock[]
  createTime: number
  hasTopped: boolean
  investmentDirection: string
}

interface ConceptEventDay {
  date: string
  eventList: ConceptEvent[]
}

const message = useMessage()
const list = ref<ConceptEventDay[]>([])
const loading = ref(false)
const dateTs = ref<number | null>(null)
const expandedNames = ref<string[]>([])

const queryDate = computed(() => dateTs.value ? format(new Date(dateTs.value), 'yyyy-MM-dd') : '')

// 按日期分组（一级标题），日期内按投资方向分组（二级标题），事件标题为三级
const groupedByDate = computed(() => {
  return list.value.map(day => {
    const dirMap = new Map<string, ConceptEvent[]>()
    for (const ev of day.eventList) {
      const dir = ev.investmentDirection || '未分类'
      if (!dirMap.has(dir)) dirMap.set(dir, [])
      dirMap.get(dir)!.push(ev)
    }
    const directions = Array.from(dirMap.entries())
      .map(([direction, events]) => ({
        direction,
        events: events.sort((a, b) => b.heat - a.heat),
        totalHeat: events.reduce((s, e) => s + e.heat, 0)
      }))
      .sort((a, b) => b.totalHeat - a.totalHeat)
    const totalEvents = day.eventList.length
    const totalHeat = day.eventList.reduce((s, e) => s + e.heat, 0)
    return { date: day.date, directions, totalEvents, totalHeat }
  })
})

// 主题色（K线弹窗需要）
const darkTheme = ref(false)

// K线弹窗状态
const klineModalShow = ref(false)
const klineCode = ref('')
const klineName = ref('')

// 概念详情弹窗状态
const conceptModalShow = ref(false)
const conceptCode = ref('')
const conceptName = ref('')
const conceptPlateCode = ref('')

/** 同花顺 marketId → 交易所后缀 */
function marketIdToSuffix(marketId: string): string {
  switch (marketId) {
    case '17':
      return '.SH'
    case '33':
      return '.SZ'
    case '146':
      return '.BJ'
    case '116':
    case '71':
      return '.HK'
    default:
      return ''
  }
}

/** 兜底：按股票代码前缀推断交易所后缀 */
function codeToSuffix(stockCode: string): string {
  const code = String(stockCode || '').trim()
  if (!code) return ''
  // 港股：5位且以0/1/2/3开头
  if (/^\d{5}$/.test(code)) return '.HK'
  const first = code.charAt(0)
  if (first === '6' || first === '9' || first === '5' || first === '1') return '.SH'
  if (first === '0' || first === '3') return '.SZ'
  if (first === '8' || first === '4') return '.BJ'
  return ''
}

/** 拼接 StockLightweightKlineChart 所需的代码（如 600000.SH） */
function toKlineCode(stockCode: string, marketId: string): string {
  let suffix = marketIdToSuffix(marketId)
  if (!suffix) suffix = codeToSuffix(stockCode)
  return suffix ? `${stockCode}${suffix}` : stockCode
}

function themeTypeText(type: string) {
  switch (type) {
    case 'concept':
      return '概念'
    case 'industry':
      return '行业'
    case 'concept-subdivision':
      return '细分'
    default:
      return type
  }
}

function themeTagType(type: string): 'info' | 'success' | 'warning' {
  switch (type) {
    case 'concept':
      return 'info'
    case 'industry':
      return 'success'
    case 'concept-subdivision':
      return 'warning'
    default:
      return 'info'
  }
}

function limitUpText(state: number | null): string {
  if (state == null) return ''
  if (state === 1) return '首板'
  if (state === 2) return '二板'
  if (state >= 3) return state + '板'
  return ''
}

function formatTime(ts: number): string {
  if (!ts) return ''
  return format(new Date(ts * 1000), 'HH:mm:ss')
}

function weekday(date: string): string {
  try {
    return format(parse(date, 'yyyy-MM-dd', new Date()), 'EEEE', {locale: zhCN})
  } catch {
    return ''
  }
}

function openStock(stock: TopStock) {
  klineCode.value = toKlineCode(stock.stockCode, stock.marketId)
  klineName.value = stock.stockName
  klineModalShow.value = true
}

function openConcept(theme: Theme) {
  // 打开同花顺概念详情弹窗，展示板块行情、K线、成分股
  // plateCode 传题材 indexCode（板块代码 88xxxx，用于 K线/realhead）
  // conceptCode 留空，由 modal 内部通过 conceptName 字典匹配真正的概念代码(30xxxx)
  conceptPlateCode.value = theme.indexCode || ''
  conceptCode.value = ''
  conceptName.value = theme.showName || theme.indexName || ''
  conceptModalShow.value = true
}

async function fetchData() {
  loading.value = true
  try {
    const res = await ConceptEventList(queryDate.value)
    list.value = res || []
    // 默认展开第一天
    expandedNames.value = list.value.length ? [list.value[0].date] : []
    if (!list.value.length) {
      message.info(`${queryDate.value || '今日'} 暂无炒作题材数据`)
    }
  } catch (e) {
    message.error('获取数据失败: ' + e)
  } finally {
    loading.value = false
  }
}

function refresh() {
  fetchData()
}

function onDateChange() {
  fetchData()
}

onBeforeMount(() => {
  GetConfig().then(res => {
    darkTheme.value = !!res.darkTheme
  }).catch(() => {
  })
  fetchData()
})
</script>

<template>
  <n-card size="small" style="--wails-draggable:no-drag">
    <template #header>
      <n-flex align="center" :wrap="false">
        <n-text strong>每日炒作题材</n-text>
        <n-text depth="3" style="font-size: 12px;">同花顺 · 竞题材</n-text>
      </n-flex>
    </template>
    <template #header-extra>
      <n-flex align="center" :wrap="false">
        <n-date-picker v-model:value="dateTs" type="date" size="small" clearable
                       :is-date-disabled="(ts:number) => ts > Date.now()"
                       @update:value="onDateChange" style="width: 160px"/>
        <n-button size="small" type="primary" :loading="loading" @click="refresh">刷新</n-button>
      </n-flex>
    </template>

    <n-spin :show="loading">
      <n-scrollbar style="max-height: calc(100vh - 280px);">
        <n-collapse v-if="groupedByDate.length" v-model:expanded-names="expandedNames" arrow-placement="left">
          <n-collapse-item v-for="day in groupedByDate" :key="day.date" :name="day.date">
            <template #header>
              <n-flex align="center" :wrap="false">
                <n-text strong>{{ day.date }}</n-text>
                <n-text depth="3" style="font-size: 12px;">{{ weekday(day.date) }}</n-text>
                <n-tag size="small" round :bordered="false" type="info">{{ day.totalEvents }} 事件</n-tag>
                <n-tag size="small" round :bordered="false" type="error">
                  热度 <n-number-animation :from="0" :to="day.totalHeat" show-separator/>
                </n-tag>
              </n-flex>
            </template>
            <div v-for="group in day.directions" :key="group.direction" style="margin-bottom: 4px;">
              <n-divider title-placement="left" style="margin: 8px 0 4px;">
                <n-text strong style="font-size: 14px;">{{ group.direction }}</n-text>
                <n-tag size="tiny" round :bordered="false" type="error" style="margin-left: 6px;">
                  <n-number-animation :from="0" :to="group.totalHeat" show-separator/>
                </n-tag>
              </n-divider>
              <n-list :show-divider="true" hoverable clickable>
                <n-list-item v-for="ev in group.events" :key="ev.eventId">
                  <n-thing>
                    <template #header>
                      <n-text strong style="font-size: 14px;">{{ ev.title }}</n-text>
                    </template>
                    <template #header-extra>
                      <n-flex align="center" :wrap="false">
                        <n-tag size="small" type="error" round :bordered="false">
                          热度 <n-number-animation :from="0" :to="ev.heat" show-separator/>
                        </n-tag>
                        <n-text depth="3" style="font-size: 12px;">{{ formatTime(ev.createTime) }}</n-text>
                      </n-flex>
                    </template>
                    <template #description>
                      <n-flex size="small" style="margin-top: 4px;">
                        <n-tag v-for="t in ev.themes" :key="t.id" size="small"
                               :type="themeTagType(t.type)" :bordered="false"
                               @click="openConcept(t)" style="cursor:pointer">
                          {{ t.showName }}
                          <n-text depth="3" style="font-size: 11px; margin-left: 2px;">
                            ·{{ themeTypeText(t.type) }}
                          </n-text>
                        </n-tag>
                      </n-flex>
                    </template>
                    <template #footer>
                      <n-flex size="small" align="center" style="margin-top: 6px;">
                        <n-text depth="3" style="font-size: 12px;">龙头股：</n-text>
                        <n-tag v-for="s in ev.topStocks" :key="s.stockCode"
                               size="small" :bordered="true"
                               :type="s.risePercent > 0 ? 'error' : 'success'"
                               @click="openStock(s)" style="cursor:pointer">
                          {{ s.stockName }}
                          <n-text style="font-size: 11px; margin-left: 2px;">
                            {{ s.risePercent > 0 ? '+' : '' }}{{ s.risePercent.toFixed(2) }}%
                          </n-text>
                          <n-tag v-if="limitUpText(s.limitUpState)" size="tiny"
                                 type="error" round :bordered="false" style="margin-left: 4px;">
                            {{ limitUpText(s.limitUpState) }}
                          </n-tag>
                        </n-tag>
                      </n-flex>
                    </template>
                  </n-thing>
                </n-list-item>
              </n-list>
            </div>
          </n-collapse-item>
        </n-collapse>
        <n-empty v-else-if="!loading" description="暂无数据" style="padding: 40px 0;"/>
      </n-scrollbar>
    </n-spin>
  </n-card>

  <!-- 龙头股 K线弹窗（项目内组件，不打开外部链接） -->
  <n-modal v-model:show="klineModalShow"
           :title="klineName + ' — K线'"
           preset="card"
           style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
           :content-style="{
             maxHeight: 'min(85vh, 820px)',
             overflowY: 'auto',
             overflowX: 'hidden',
             minWidth: 0,
             boxSizing: 'border-box',
           }">
    <stock-lightweight-kline-chart
      v-if="klineModalShow"
      :key="'concept-kline-' + klineCode"
      :code="klineCode"
      :stock-name="klineName"
      :dark-theme="darkTheme"
      :chart-height="500"
    />
  </n-modal>

  <!-- 概念详情弹窗 -->
  <concept-detail-modal
    v-model:show="conceptModalShow"
    :concept-code="conceptCode"
    :concept-name="conceptName"
    :plate-code="conceptPlateCode"/>
</template>

<style scoped>
</style>
