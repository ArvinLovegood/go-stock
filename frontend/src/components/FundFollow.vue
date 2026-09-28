<script setup>
import {h, onBeforeMount, onBeforeUnmount, onMounted, reactive, ref, computed, watch} from "vue";
import {Add, ChatboxOutline, RefreshOutline} from "@vicons/ionicons5";
import {NButton, NEllipsis, NText, useMessage, NTag, NModal, NDataTable, NPopover, NIcon} from "naive-ui";
import {
  FollowFund,
  GetConfig,
  GetFollowedFundPaged,
  GetfundList,
  GetVersionInfo,
  OpenURL,
  UnFollowFund,
  GetFundHistoryNetValue,
  GetFundTop10Holdings
} from "../../wailsjs/go/main/App";
import {Environment} from "../../wailsjs/runtime";
import vueDanmaku from 'vue3-danmaku'
import FundKlineChart from "./FundKlineChart.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from "./kline/useKlineModalFit";

const danmus = ref([])
const ws = ref(null)
const icon = ref(null)
const message = useMessage()
const chartModalShow = ref(false)
const chartFundCode = ref('')
const chartFundName = ref('')
// K 线弹窗：宽度与全站其他 K 线弹窗统一；图表下方还有历史净值表格，故取比例高度不参与铺满收敛
const klineWrapRef = ref(null)
const { chartHeight: klineChartHeight, attach: attachKlineFit, detach: detachKlineFit } = useKlineModalFit(klineWrapRef, { scrollable: true })
watch(chartModalShow, (v) => {
  if (v) {
    attachKlineFit()
    return
  }
  detachKlineFit()
})
// 当前弹窗基金是否为场内基金（ETF）；场内用 StockLightweightKlineChart，场外用 FundKlineChart
const chartOnExchange = computed(() => isOnExchangeFund(chartFundCode.value))
// 场内基金代码转东方财富格式，喂给 StockLightweightKlineChart
const chartEastMoneyCode = computed(() => fundCodeToEastMoney(chartFundCode.value))
const netValueData = ref([])
const netValueLoading = ref(false)
const darkTheme = ref(false)
const showPopover = ref(false)
const holdingsMap = reactive({})
const data = reactive({
  modelName: "",
  chatId: "",
  question: "",
  name: "",
  code: "",
  fullscreen: false,
  airesult: "",
  openAiEnable: false,
  loading: true,
  enableDanmu: false,
})

const followList = ref([])
const followTotalCount = ref(0)
const followTotalPages = ref(1)
const followPage = ref(1)
const followPageSize = 4
const followLoading = ref(false)
const followKeyword = ref('')
let followSearchTimer = null
const options = ref([])
const ticker = ref({})
const REFRESH_INTERVAL = 60
const countdown = ref(REFRESH_INTERVAL)
const refreshing = ref(false)
const countdownTimer = ref({})

function loadFollowedFunds() {
  followLoading.value = true
  GetFollowedFundPaged(followPage.value, followPageSize, followKeyword.value).then(result => {
    if (result) {
      followList.value = result.items || []
      followTotalCount.value = result.totalCount || 0
      followTotalPages.value = result.totalPages || 1
      loadCurrentPageHoldings()
    }
  }).finally(() => {
    followLoading.value = false
  })
}

function onFollowPageChange(page) {
  followPage.value = page
  loadFollowedFunds()
}

function onFollowSearch(val) {
  followKeyword.value = val || ''
  followPage.value = 1
  if (followSearchTimer) clearTimeout(followSearchTimer)
  followSearchTimer = setTimeout(() => {
    loadFollowedFunds()
  }, 500)
}

function loadCurrentPageHoldings() {
  for (const fund of followList.value) {
    if (!holdingsMap[fund.code]) {
      loadHoldings(fund.code)
    }
  }
}

const netValueColumns = computed(() => {
  const onExchange = isOnExchangeFund(chartFundCode.value)
  const cols = [
    { title: '日期', key: 'date', width: 110 },
    { title: onExchange ? '收盘价' : '单位净值', key: 'netValue', width: 100, render: (row) => row.netValue ?? '-' },
  ]
  if (!onExchange) {
    cols.push({ title: '累计净值', key: 'accumValue', width: 100, render: (row) => row.accumValue ?? '-' })
  }
  cols.push({
    title: '日涨幅',
    key: 'dailyGrowth',
    width: 90,
    render: (row) => {
      const v = row.dailyGrowth
      if (v == null) return '-'
      const color = v > 0 ? '#ef5350' : v < 0 ? '#26a69a' : undefined
      return h(NText, { style: { color } }, () => v.toFixed(2) + '%')
    }
  })
  if (!onExchange) {
    cols.push({ title: '申购', key: 'buyStatus', width: 70 })
    cols.push({ title: '赎回', key: 'sellStatus', width: 70 })
  }
  return cols
})

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.openAiEnable) data.openAiEnable = true
    if (result.enableDanmu) data.enableDanmu = true
    if (result.darkTheme) darkTheme.value = true
  })
  loadFollowedFunds()
})

onMounted(() => {
  GetVersionInfo().then((res) => {
    icon.value = res.icon
  })

  ws.value = new WebSocket('ws://8.134.249.145:16688/ws');
  ws.value.onopen = () => {}
  ws.value.onmessage = (event) => {
    if (data.enableDanmu) danmus.value.push(event.data)
  }
  ws.value.onerror = (error) => { console.error('WebSocket 错误:', error) }
  ws.value.onclose = () => {}

  ticker.value = setInterval(() => {
    refreshAllFunds()
  }, 1000 * REFRESH_INTERVAL)

  countdownTimer.value = setInterval(() => {
    if (countdown.value > 0) {
      countdown.value--
    }
  }, 1000)
})

onBeforeUnmount(() => {
  clearInterval(ticker.value)
  clearInterval(countdownTimer.value)
  if (ws.value) ws.value.close()
  message.destroyAll()
})

function refreshAllFunds() {
  refreshing.value = true
  countdown.value = REFRESH_INTERVAL
  loadFollowedFunds()
  setTimeout(() => { refreshing.value = false }, 500)
}

function manualRefresh() {
  if (refreshing.value) return
  refreshAllFunds()
}

function preloadHoldings() {
  loadCurrentPageHoldings()
}

function SendDanmu() {
  ws.value.send(data.name)
}

function AddFund() {
  if (!data.code) {
    showPopover.value = true
    setTimeout(() => { showPopover.value = false }, 3000)
    return
  }
  FollowFund(data.code).then(result => {
    if (result) {
      message.success("关注成功")
      loadFollowedFunds()
    }
  })
}

function unFollow(code) {
  UnFollowFund(code).then(result => {
    if (result) {
      message.success("取消关注成功")
      if (followList.value.length <= 1 && followPage.value > 1) {
        followPage.value--
      }
      loadFollowedFunds()
    }
  })
}

function getFundList(value) {
  GetfundList(value).then(result => {
    options.value = []
    result.forEach(item => {
      options.value.push({
        label: item.name + " [" + item.code + "]",
        value: item.code,
      })
    })
  })
}

function onSelectFund(value) {
  data.code = value
  blinkBorder(value)
}

function search(code) {
  setTimeout(() => {
    Environment().then(env => {
      switch (env.platform) {
        case 'windows':
          window.open("https://fund.eastmoney.com/" + code + ".html", "_blank", "noreferrer,width=1000,top=100,left=100,status=no,toolbar=no,location=no,scrollbars=no")
          break
        default:
          OpenURL("https://fund.eastmoney.com/" + code + ".html")
      }
    })
  }, 300)
}

function showChart(code, name) {
  chartFundCode.value = code
  chartFundName.value = name
  chartModalShow.value = true
  loadNetValueHistory(code)
}

function loadHoldings(code) {
  if (holdingsMap[code]) return
  GetFundTop10Holdings(code).then(result => {
    holdingsMap[code] = result || []
  }).catch(() => {
    holdingsMap[code] = []
  })
}

function loadNetValueHistory(code) {
  netValueLoading.value = true
  netValueData.value = []
  GetFundHistoryNetValue(code, 30, '', '').then(result => {
    netValueData.value = result || []
  }).catch(() => {
    netValueData.value = []
  }).finally(() => {
    netValueLoading.value = false
  })
}

function isOnExchangeFund(code) {
  const p = code?.substring(0, 2)
  return ['15', '16', '50', '51', '52', '53', '56', '58'].includes(p)
}

// 场内基金代码（纯数字）转东方财富格式（如 513310 → 513310.SH，159915 → 159915.SZ）
// 用于把基金代码喂给 StockLightweightKlineChart（其 code prop 接收东方财富格式）
function fundCodeToEastMoney(code) {
  if (!code) return ''
  const c = String(code)
  if (/^(5|6)/.test(c)) return c + '.SH'
  return c + '.SZ'
}

function rateType(rate) {
  if (rate > 0) return 'error'
  if (rate < 0) return 'success'
  return 'default'
}

function growthType(val) {
  if (val > 0) return 'error'
  if (val < 0) return 'success'
  return 'default'
}

function ratioColor(ratio) {
  if (ratio >= 8) return '#ef5350'
  if (ratio >= 5) return '#e6a23c'
  return undefined
}

function splitHalves(arr) {
  if (!arr || !arr.length) return [[], []]
  const mid = Math.ceil(arr.length / 2)
  return [arr.slice(0, mid), arr.slice(mid)]
}

function changeText(rate) {
  if (rate == null) return '-'
  return (rate > 0 ? '+' : '') + rate.toFixed(2) + '%'
}

function changeColor(rate) {
  if (rate == null) return undefined
  return rate > 0 ? '#ef5350' : rate < 0 ? '#19b860' : undefined
}

function blinkBorder(findId) {
  const element = document.getElementById(findId)
  if (element) {
    element.scrollIntoView({ behavior: 'smooth' })
    const pelement = document.getElementById(findId + '_gi')
    if (pelement) {
      pelement.classList.add('blink-border')
      setTimeout(() => { pelement.classList.remove('blink-border') }, 1000 * 5)
    }
  }
}
</script>

<template>
  <vue-danmaku v-model:danmus="danmus" useSlot style="height:100px; width:100%;z-index: 9;position:absolute; top: 400px; pointer-events: none;">
    <template v-slot:dm="{ danmu }">
      <n-gradient-text type="info">
        <n-icon :component="ChatboxOutline"/>{{ danmu }}
      </n-gradient-text>
    </template>
  </vue-danmaku>

  <n-divider style="margin: 4px 0 8px 0"/>

  <n-input
    v-model:value="followKeyword"
    placeholder="搜索基金名称/代码"
    clearable
    size="small"
    style="margin-bottom: 8px;"
    @update:value="onFollowSearch"
  />

  <n-grid :x-gap="10" :y-gap="10" :cols="2" responsive="screen" item-responsive>
    <n-gi v-for="info in followList" :key="info.code" :id="info.code + '_gi'">
      <n-card :id="info.code" size="small" hoverable>
        <template #header>
          <n-text style="font-size: 15px; font-weight: 600;">{{ info.fundBasic?.fullName || info.name }}</n-text>
        </template>
        <template #header-extra>
          <n-flex :wrap="false" align="center" :size="4">
            <n-tag size="small" :bordered="false" type="info">{{ info.code }}</n-tag>
            <n-tag size="small" :bordered="false" type="warning">{{ info.fundBasic?.type || '' }}</n-tag>
          </n-flex>
        </template>

        <n-grid :cols="24" :x-gap="16">
          <n-gi :span="10">
            <n-flex align="center" :size="12" :wrap="false">
              <div v-if="!isOnExchangeFund(info.code) && info.netActualRate != null" style="min-width: 100px;">
                <div style="font-size: 12px; color: #999;">实际净值</div>
                <n-text :type="rateType(info.netActualRate)" style="font-size: 22px; font-weight: 700;">
                  {{ info.netUnitValue }}
                </n-text>
                <n-text :type="rateType(info.netActualRate)" style="font-size: 14px; margin-left: 4px;">
                  {{ info.netActualRate > 0 ? '+' : '' }}{{ info.netActualRate.toFixed(2) }}%
                </n-text>
                <div style="font-size: 11px; color: #999;">{{ info.netUnitValueDate }}</div>
              </div>
              <template v-else>
                <div v-if="info.netEstimatedUnit || info.fundBasic?.netEstimatedUnit" style="min-width: 100px;">
                  <div style="font-size: 12px; color: #999;">{{ isOnExchangeFund(info.code) ? '实时价格' : '估算净值' }}</div>
                  <n-text :type="rateType(info.netEstimatedRate || info.fundBasic?.netEstimatedRate)" style="font-size: 22px; font-weight: 700;">
                    {{ info.netEstimatedUnit || info.fundBasic?.netEstimatedUnit }}
                  </n-text>
                  <n-text :type="rateType(info.netEstimatedRate || info.fundBasic?.netEstimatedRate)" style="font-size: 14px; margin-left: 4px;">
                    {{ (info.netEstimatedRate || info.fundBasic?.netEstimatedRate) > 0 ? '+' : '' }}{{ (info.netEstimatedRate || info.fundBasic?.netEstimatedRate)?.toFixed(2) }}%
                  </n-text>
                </div>
                <div v-else-if="info.netUnitValue || info.fundBasic?.netUnitValue" style="min-width: 100px;">
                  <div style="font-size: 12px; color: #999;">单位净值</div>
                  <n-text style="font-size: 22px; font-weight: 700;">{{ info.netUnitValue || info.fundBasic?.netUnitValue }}</n-text>
                </div>
                <n-divider vertical v-if="(info.netEstimatedUnit || info.fundBasic?.netEstimatedUnit) && (info.netUnitValue || info.fundBasic?.netUnitValue)"/>
                <div v-if="(info.netUnitValue || info.fundBasic?.netUnitValue) && (info.netEstimatedUnit || info.fundBasic?.netEstimatedUnit)">
                  <div style="font-size: 12px; color: #999;">单位净值</div>
                  <n-text style="font-size: 15px;">{{ info.netUnitValue || info.fundBasic?.netUnitValue }}</n-text>
                  <div style="font-size: 11px; color: #999;">{{ info.netUnitValueDate || info.fundBasic?.netUnitValueDate }}</div>
                </div>
              </template>
            </n-flex>

            <n-flex :size="4" style="margin-top: 8px;" :wrap="true">
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth1)" :bordered="false" v-if="info.fundBasic?.netGrowth1">近1月 {{ info.fundBasic.netGrowth1 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth3)" :bordered="false" v-if="info.fundBasic?.netGrowth3">近3月 {{ info.fundBasic.netGrowth3 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth6)" :bordered="false" v-if="info.fundBasic?.netGrowth6">近6月 {{ info.fundBasic.netGrowth6 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth12)" :bordered="false" v-if="info.fundBasic?.netGrowth12">近1年 {{ info.fundBasic.netGrowth12 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth36)" :bordered="false" v-if="info.fundBasic?.netGrowth36">近3年 {{ info.fundBasic.netGrowth36 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowth60)" :bordered="false" v-if="info.fundBasic?.netGrowth60">近5年 {{ info.fundBasic.netGrowth60 }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowthYTD)" :bordered="false" v-if="info.fundBasic?.netGrowthYTD">今年来 {{ info.fundBasic.netGrowthYTD }}%</n-tag>
              <n-tag size="tiny" :type="growthType(info.fundBasic?.netGrowthAll)" :bordered="false" v-if="info.fundBasic?.netGrowthAll">成立来 {{ info.fundBasic.netGrowthAll }}%</n-tag>
            </n-flex>
          </n-gi>

          <n-gi :span="14">
            <div v-if="holdingsMap[info.code] && holdingsMap[info.code].length" class="holdings-panel">
              <div class="holdings-title">
                十大持仓
                <n-text v-if="holdingsMap[info.code][0]?.quarter" depth="3" style="font-size: 11px; margin-left: 4px;">
                  {{ holdingsMap[info.code][0].quarter }}
                </n-text>
              </div>
              <div class="holdings-cols">
                <template v-for="(half, hi) in splitHalves(holdingsMap[info.code])" :key="hi">
                  <div class="holdings-col">
                    <div class="holdings-header">
                      <span>名称</span>
                      <span>占比</span>
                      <span>最新价</span>
                      <span>涨跌幅</span>
                    </div>
                    <div v-for="stock in half" :key="stock.stockCode" class="holding-row">
                      <span class="holding-name" :title="stock.stockName">{{ stock.stockName }}</span>
                      <span class="holding-ratio" :style="{ color: ratioColor(stock.ratio) }">{{ stock.ratio?.toFixed(2) }}%</span>
                      <span class="holding-price" :style="{ color: changeColor(stock.changeRate) }">{{ stock.price ? stock.price.toFixed(2) : '-' }}</span>
                      <span class="holding-change" :style="{ color: changeColor(stock.changeRate) }">{{ changeText(stock.changeRate) }}</span>
                    </div>
                  </div>
                </template>
              </div>
            </div>
            <div v-else class="holdings-panel">
              <n-text depth="3" style="font-size: 12px;">暂无持仓数据</n-text>
            </div>
          </n-gi>
        </n-grid>

        <template #footer>
          <n-flex justify="space-between" align="center">
            <n-text depth="3" style="font-size: 12px;">
              {{ info.fundBasic?.company }} · {{ info.fundBasic?.manager }}
            </n-text>
          </n-flex>
        </template>

        <template #action>
          <n-flex justify="space-between" align="center">
            <n-text depth="3" style="font-size: 11px;">{{ countdown }}s 后刷新</n-text>
            <n-flex :size="8">
              <n-button size="tiny" :loading="refreshing" @click="manualRefresh">
                <template #icon><n-icon :component="RefreshOutline"/></template>
              </n-button>
              <n-button size="tiny" type="error" @click="showChart(info.code, info.name)">历史净值</n-button>
              <n-button size="tiny" type="warning" @click="search(info.code)">详情</n-button>
              <n-button size="tiny" @click="unFollow(info.code)">取消关注</n-button>
            </n-flex>
          </n-flex>
        </template>
      </n-card>
    </n-gi>
  </n-grid>

  <n-flex justify="center" style="margin-top: 8px;" v-if="followTotalPages > 1">
    <n-pagination v-model:page="followPage" :page-count="followTotalPages" :page-size="followPageSize" @update:page="onFollowPageChange" size="small"/>
  </n-flex>

  <n-modal
    v-model:show="chartModalShow"
    :title="chartFundName + ' - ' + chartFundCode"
    preset="card"
    :style="KLINE_MODAL_STYLE"
    :content-style="{
      ...KLINE_MODAL_CONTENT_STYLE,
      overflowY: 'auto',
      // 图表下方还有历史净值表格，给表格留出空间后仍让整卡控制在 94vh 内
      maxHeight: 'calc(94vh - 120px)',
    }"
    :mask-closable="true"
  >
    <div v-if="chartFundCode && chartOnExchange" ref="klineWrapRef">
      <StockLightweightKlineChart
        :key="chartFundCode"
        :code="chartEastMoneyCode"
        :stock-name="chartFundName"
        :dark-theme="darkTheme"
        :chart-height="klineChartHeight"
      />
    </div>
    <FundKlineChart
      v-else-if="chartFundCode"
      :key="chartFundCode"
      :fund-code="chartFundCode"
      :fund-name="chartFundName"
      :dark-theme="darkTheme"
      :chart-height="400"
    />

    <n-divider style="margin: 12px 0 8px 0">{{ isOnExchangeFund(chartFundCode) ? '历史行情' : '历史净值' }}</n-divider>

    <n-data-table
      :columns="netValueColumns"
      :data="netValueData"
      :loading="netValueLoading"
      :pagination="{ pageSize: 10 }"
      size="small"
      :bordered="false"
      :max-height="300"
      striped
    />
  </n-modal>

  <div style="position: fixed;top: 28px;right:5px;z-index: 10;width: 400px">
    <n-input-group>
      <n-auto-complete
        v-model:value="data.name"
        :input-props="{ autocomplete: 'disabled' }"
        :options="options"
        placeholder="基金名称/代码/弹幕"
        clearable
        @update-value="getFundList"
        :on-select="onSelectFund"
      />
      <n-popover trigger="manual" :show="showPopover">
        <template #trigger>
          <n-button type="primary" @click="AddFund">
            <n-icon :component="Add"/>&nbsp;关注
          </n-button>
        </template>
        <span>输入基金名称/代码关键词开始吧~~~</span>
      </n-popover>
      <n-button type="info" @click="SendDanmu" v-if="data.enableDanmu">
        <n-icon :component="ChatboxOutline"/>&nbsp;发送弹幕
      </n-button>
    </n-input-group>
  </div>
</template>

<style scoped>
.blink-border {
  animation: blink-border 1s linear infinite;
  border: 4px solid transparent;
}

@keyframes blink-border {
  0% { border-color: red; }
  50% { border-color: transparent; }
  100% { border-color: red; }
}

.holdings-panel {
  border-left: 1px solid var(--n-border-color, #efeff5);
  padding-left: 12px;
  height: 100%;
}

.holdings-title {
  font-size: 12px;
  font-weight: 600;
  color: #666;
  margin-bottom: 4px;
}

.holdings-cols {
  display: flex;
  gap: 10px;
}

.holdings-col {
  flex: 1;
  min-width: 0;
}

.holdings-header {
  display: grid;
  grid-template-columns: 1fr 44px 48px 52px;
  gap: 0 4px;
  font-size: 10px;
  color: var(--n-text-color-3, #999);
  padding-bottom: 2px;
  border-bottom: 1px solid var(--n-border-color, #efeff5);
  margin-bottom: 2px;
}

.holdings-header span:not(:first-child) {
  text-align: right;
}

.holding-row {
  display: grid;
  grid-template-columns: 1fr 44px 48px 52px;
  gap: 0 4px;
  align-items: center;
  font-size: 11px;
  line-height: 18px;
  white-space: nowrap;
}

.holding-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.holding-ratio {
  text-align: right;
  font-weight: 500;
}

.holding-price {
  text-align: right;
  color: var(--n-text-color-3, #999);
}

.holding-change {
  text-align: right;
  font-size: 11px;
}
</style>
