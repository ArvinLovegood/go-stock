<script setup>
import * as echarts from "echarts";
import {computed, h, nextTick, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref} from 'vue'
import {
  GetAIResponseResult,
  GetConfig,
  GetIndustryRank,
  GetPromptTemplates,
  GetTelegraphList,
  GlobalStockIndexes,
  IsTradingTime,
  IsHKTradingTime,
  IsUSTradingTime,
  ReFleshTelegraphList,
  SaveAIResponseResult,
  SaveAsMarkdown,
  ShareAnalysis,
  SummaryStockNews,
  GetAiConfigs,
} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import NewsList from "./newsList.vue";
import KLineChart from "./KLineChart.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import { CaretDown, CaretUp, PulseOutline,} from "@vicons/ionicons5";
import {NAvatar, NButton, NFlex, NText, useMessage, useNotification} from "naive-ui";
import {MdPreview} from "md-editor-v3";
import {useRoute} from 'vue-router'
import RankTable from "./rankTable.vue";
import IndustryMoneyRank from "./industryMoneyRank.vue";
import StockResearchReportList from "./StockResearchReportList.vue";
import StockNoticeList from "./StockNoticeList.vue";
import LongTigerRankList from "./LongTigerRankList.vue";
import IndustryResearchReportList from "./IndustryResearchReportList.vue";
import HotStockList from "./HotStockList.vue";
import HotEvents from "./HotEvents.vue";
import HotTopics from "./HotTopics.vue";
import InvestCalendarTimeLine from "./InvestCalendarTimeLine.vue";
import ClsCalendarTimeLine from "./ClsCalendarTimeLine.vue";
import Stockhotmap from "./stockhotmap.vue";
import BKFundFlowChart from "./bkFundFlowChart.vue";
import ConceptFundFlowChart from "./conceptFundFlowChart.vue";

const route = useRoute()
const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');

const message = useMessage()
const notify = useNotification()
const panelHeight = ref(window.innerHeight - 240)

const telegraphList = ref([])
const sinaNewsList = ref([])
const foreignNewsList = ref([])
const common = ref([])
const america = ref([])
const europe = ref([])
const asia = ref([])
const other = ref([])
const globalStockIndexes = ref(null)
const summaryModal = ref(false)
const summaryBTN = ref(true)
const darkTheme = ref(false)
const httpProxyEnabled = ref(false)
const theme = computed(() => {
  return darkTheme ? 'dark' : 'light'
})
const aiSummary = ref(``)
const aiSummaryTime = ref("")
const modelName = ref("")
const chatId = ref("")
const question = ref(``)
const aiConfigId = ref(null)
const sysPromptId = ref(null)
const loading = ref(true)
const analysisStatus = ref('')
const aiConfigs = ref([])
const sysPromptOptions = ref([])
const userPromptOptions = ref([])
const promptTemplates = ref([])
const industryRanks = ref([])
const sort = ref("0")
const nowTab = ref("市场快讯")
const indexInterval = ref(null)
const indexIndustryRank = ref(null)
const tradingCheckInterval = ref(null)
const mdPreviewRef = ref(null)
const aiResultScrollRef = ref(null)
const stockCode= ref('')
const enableTools= ref(true)
const thinkingMode = ref(true)
const treemapRef = ref(null);
let treemapchart =null;

function getIndex() {
  GlobalStockIndexes().then((res) => {
    globalStockIndexes.value = res
    common.value = res["common"]
    america.value = res["america"]
    europe.value = res["europe"]
    asia.value = res["asia"]
    other.value = res["other"]
  })
}

onBeforeMount(() => {
  nowTab.value = route.query.name
  stockCode.value = route.query.stockCode
  GetConfig().then(result => {
    summaryBTN.value = result.openAiEnable
    darkTheme.value = result.darkTheme
    httpProxyEnabled.value = result.httpProxyEnabled
  })
  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
    sysPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型系统Prompt')
    userPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型用户Prompt')
  })

  GetAiConfigs().then(res=>{
    aiConfigs.value = res
    aiConfigId.value = res[0].ID
  })
  GetTelegraphList("财联社电报").then((res) => {
    telegraphList.value = res
  })
  GetTelegraphList("新浪财经").then((res) => {
    sinaNewsList.value = res
  })
  GetTelegraphList("外媒").then((res) => {
    foreignNewsList.value = res
  })
  getIndex();
  industryRank();
  startTradingTimers();

  tradingCheckInterval.value = setInterval(async () => {
    const [cn, hk, us] = await Promise.all([
      IsTradingTime().catch(() => false),
      IsHKTradingTime().catch(() => false),
      IsUSTradingTime().catch(() => false)
    ])
    const anyTrading = cn || hk || us
    if (anyTrading && !indexInterval.value) {
      startTradingTimers()
    } else if (!anyTrading && indexInterval.value) {
      stopTradingTimers()
    }
  }, 60000)
})


onBeforeUnmount(() => {
  EventsOff("changeMarketTab")
  EventsOff("newTelegraph")
  EventsOff("newSinaNews")
  EventsOff("summaryStockNews")
  stopTradingTimers()
  if (tradingCheckInterval.value) {
    clearInterval(tradingCheckInterval.value)
  }
})

function startTradingTimers() {
  stopTradingTimers()
  indexInterval.value = setInterval(() => {
    getIndex()
  }, 3000)
  indexIndustryRank.value = setInterval(() => {
    industryRank()
    ReFlesh("财联社电报")
    ReFlesh("新浪财经")
    ReFlesh("外媒")
  }, 1000 * 10)
}

function stopTradingTimers() {
  if (indexInterval.value) {
    clearInterval(indexInterval.value)
    indexInterval.value = null
  }
  if (indexIndustryRank.value) {
    clearInterval(indexIndustryRank.value)
    indexIndustryRank.value = null
  }
}

onUnmounted(() => {

});
EventsOn("changeMarketTab", async (msg) => {
  //message.info(msg.name)
  console.log(msg.name)
  updateTab(msg.name)
})

EventsOn("newTelegraph", (data) => {
  if (data!=null) {
    for (let i = 0; i < data.length; i++) {
      telegraphList.value.pop()
    }
    telegraphList.value.unshift(...data)
  }
})
EventsOn("newSinaNews", (data) => {
  if (data!=null) {
  for (let i = 0; i < data.length; i++) {
    sinaNewsList.value.pop()
  }
  sinaNewsList.value.unshift(...data)
  }
})
EventsOn("tradingViewNews", (data) => {
  if (data!=null) {
    for (let i = 0; i < data.length; i++) {
      foreignNewsList.value.pop()
    }
    foreignNewsList.value.unshift(...data)
  }
})

//获取页面高度
window.onresize = () => {
  panelHeight.value = window.innerHeight - 240
}

function getAreaName(code) {
  switch (code) {
    case "america":
      return "美洲"
    case "europe":
      return "欧洲"
    case "asia":
      return "亚洲"
    case "common":
      return "常用"
    case "other":
      return "其他"
  }
}

function changeIndustryRankSort() {
  if (sort.value === "0") {
    sort.value = "1"
  } else {
    sort.value = "0"
  }
  industryRank()
}

function industryRank() {

  GetIndustryRank(sort.value, 150).then(result => {
    if (result.length > 0) {
      //console.log(result)
      industryRanks.value = result
    } else {
      message.info("暂无数据")
    }
  })
}

function reAiSummary() {
  aiSummary.value = ""
  summaryModal.value = true
  loading.value = true
  analysisStatus.value = "正在连接AI服务..."
  SummaryStockNews(question.value,aiConfigId.value, sysPromptId.value,enableTools.value,thinkingMode.value,"summaryStockNews","")
}

function getAiSummary() {
  summaryModal.value = true
  loading.value = true
  GetAIResponseResult("市场资讯").then(result => {
    loading.value = false
    if (result.content) {
      aiSummary.value = result.content
      question.value = result.question
      loading.value = false

      const date = new Date(result.CreatedAt);
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      aiSummaryTime.value = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
      modelName.value = result.modelName
    } else {
      aiSummaryTime.value = ""
      aiSummary.value = ""
      modelName.value = ""
      //SummaryStockNews(question.value, sysPromptId.value,enableTools.value)
    }
  })
}

function updateTab(name) {
  summaryBTN.value = (name === "市场快讯");
  nowTab.value = name
}

EventsOn("summaryStockNews", async (msg) => {
  if (msg === "DONE") {
    await SaveAIResponseResult("市场资讯", "市场资讯", aiSummary.value, chatId.value, question.value,aiConfigId.value)
    loading.value = false
    analysisStatus.value = "分析完成"
    message.destroyAll()
    notify.success({
      title: 'AI分析完成',
      content: '市场资讯分析已完成',
      duration: 3000,
    })
    setTimeout(() => {
      analysisStatus.value = ""
    }, 3000)
  } else {
    if (msg.chatId) {
      chatId.value = msg.chatId
    }
    if (msg.question) {
      question.value = msg.question
    }
    if (msg.content || msg.reasoning_content || msg.extraContent) {
      if (!aiSummary.value) {
        analysisStatus.value = "AI正在分析中..."
      }
      loading.value = false
    }
    if (msg.content) {
      aiSummary.value = aiSummary.value + msg.content
    }
    if (msg.reasoning_content) {
      aiSummary.value = aiSummary.value + msg.reasoning_content
    }
    if (msg.extraContent) {
      aiSummary.value = aiSummary.value + msg.extraContent
    }
    if (msg.model) {
      modelName.value = msg.model
    }
    if (msg.time) {
      aiSummaryTime.value = msg.time
    }
    scrollToAiResultBottom()
  }
})

function scrollToAiResultBottom() {
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = aiResultScrollRef.value
      if (el) {
        el.scrollTop = el.scrollHeight
      }
    })
  })
}

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(aiSummary.value);
    message.success('分析结果已复制到剪切板');
  } catch (err) {
    message.error('复制失败: ' + err);
  }
}

function saveAsMarkdown() {
  SaveAsMarkdown('市场资讯', '市场资讯').then(result => {
    message.success(result)
  })
}

function share() {
  ShareAnalysis('市场资讯', '市场资讯').then(msg => {
    //message.info(msg)
    notify.info({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '分享到社区',
      duration: 1000 * 30,
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, {default: () => msg})
      },
    })
  })
}

function ReFlesh(source) {
  //console.log("ReFlesh:", source)
  ReFleshTelegraphList(source).then(res => {
    if (source === "财联社电报") {
      telegraphList.value = res
    }
    if (source === "新浪财经") {
      sinaNewsList.value = res
    }
    if (source === "外媒") {
      foreignNewsList.value = res
    }
  })
}
</script>

<template>
  <n-card>
    <n-tabs type="line" animated @update-value="updateTab" :value="nowTab" style="--wails-draggable:no-drag">
      <n-tab-pane name="市场快讯" tab="市场快讯">
        <n-grid :cols="1" :y-gap="0">
          <n-gi>
            <AnalyzeMartket :dark-theme="darkTheme" :chart-height="300" :kDays="1" :name="'最近24小时热词'" />
          </n-gi>
          <n-gi>
            <n-grid :cols="foreignNewsList.length?3:2" :y-gap="0">
              <n-gi>
                <news-list :newsList="telegraphList" :header-title="'财联社电报'" @update:message="ReFlesh"></news-list>
              </n-gi>
              <n-gi>
                <news-list :newsList="sinaNewsList" :header-title="'新浪财经'" @update:message="ReFlesh"></news-list>
              </n-gi>
              <n-gi v-if="foreignNewsList.length>0">
                <news-list :newsList="foreignNewsList" :header-title="'外媒'" @update:message="ReFlesh"></news-list>
              </n-gi>

            </n-grid>
          </n-gi>
        </n-grid>

      </n-tab-pane>
      <n-tab-pane name="全球股指" tab="全球股指">
        <n-tabs type="segment" animated>
          <n-tab-pane name="全球指数" tab="全球指数">
            <n-grid :cols="5" :y-gap="0">
              <n-gi v-for="(val, key) in globalStockIndexes" :key="key">
                <n-list bordered>
                  <template #header>
                    {{ getAreaName(key) }}
                  </template>
                  <n-list-item v-for="item in val" :key="item.code">
                    <n-grid :cols="3" :y-gap="0">
                      <n-gi>

                        <n-text :type="item.zdf>0?'error':'success'">
                          <n-image :src="item.img" width="20"/> &nbsp;{{ item.name }}
                        </n-text>
                      </n-gi>
                      <n-gi>
                        <n-text :type="item.zdf>0?'error':'success'">{{ item.zxj }}</n-text>&nbsp;
                        <n-text :type="item.zdf>0?'error':'success'">
                          <n-number-animation :precision="2" :from="0" :to="item.zdf"/>
                          %
                        </n-text>

                      </n-gi>
                      <n-gi>
                        <n-text :type="item.state === 'open' ? 'success' : 'warning'">{{
                            item.state === 'open' ? '开市' : '休市'
                          }}
                        </n-text>
                      </n-gi>
                    </n-grid>
                  </n-list-item>
                </n-list>
              </n-gi>
            </n-grid>
          </n-tab-pane>
          <n-tab-pane name="上证指数" tab="上证指数">
            <k-line-chart code="sh000001" :chart-height="panelHeight" stockName="上证指数" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="深证成指" tab="深证成指">
            <k-line-chart code="sz399001" :chart-height="panelHeight" stockName="深证成指" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="创业板指" tab="创业板指">
            <k-line-chart code="sz399006" :chart-height="panelHeight" stockName="创业板指" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="恒生指数" tab="恒生指数">
            <k-line-chart code="hkHSI" :chart-height="panelHeight" stockName="恒生指数" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="纳斯达克" tab="纳斯达克">
            <k-line-chart code="us.IXIC" :chart-height="panelHeight" stockName="纳斯达克" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="道琼斯" tab="道琼斯">
            <k-line-chart code="us.DJI" :chart-height="panelHeight" stockName="道琼斯" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="标普500" tab="标普500">
            <k-line-chart code="us.INX" :chart-height="panelHeight" stockName="标普500" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="重大指数" tab="重大指数">
        <n-tabs type="segment" animated>

<!--          <n-tab-pane name="西部数据" tab="西部数据">-->
<!--            <StockLightweightKlineChart code="105.WDC" :chart-height="panelHeight" stock-name="西部数据"-->
<!--                                        :dark-theme="true"></StockLightweightKlineChart>-->
<!--          </n-tab-pane>-->

          <n-tab-pane name="上证指数" tab="上证指数"  >
            <StockLightweightKlineChart code="000001.SH" :chart-height="panelHeight-130" stock-name="上证指数" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="深证指数" tab="深证指数"  >
            <StockLightweightKlineChart code="399001.SZ" :chart-height="panelHeight-130" stock-name="深证指数" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="创业板指" tab="创业板指"  >
            <StockLightweightKlineChart code="399006.SZ" :chart-height="panelHeight-130" stock-name="创业板指" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>

          <n-tab-pane name="恒生指数" tab="恒生指数">
            <StockLightweightKlineChart code="100.HSI" :chart-height="panelHeight" stock-name="恒生指数"
                                        :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="道琼斯" tab="道琼斯">
            <StockLightweightKlineChart code="100.DJIA" :chart-height="panelHeight" stock-name="道琼斯"
                                        :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="标普500" tab="标普500">
            <StockLightweightKlineChart code="100.SPX" :chart-height="panelHeight" stock-name="标普500"
                                        :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="纳斯达克" tab="纳斯达克">
            <StockLightweightKlineChart code="100.NDX" :chart-height="panelHeight" stock-name="纳斯达克"
                                        :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>

          <n-tab-pane name="沪深300" tab="沪深300">
            <StockLightweightKlineChart code="000300.SH" :chart-height="panelHeight-130" stock-name="沪深 300" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="上证50" tab="上证50">
            <StockLightweightKlineChart code="000016.SH" :chart-height="panelHeight-130" stock-name="上证 50" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="中证A500" tab="中证A500">
            <StockLightweightKlineChart code="000510.SH" :chart-height="panelHeight-130" stock-name="中证 A500" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="中证1000" tab="中证1000">
            <StockLightweightKlineChart code="000852.SH" :chart-height="panelHeight-130" stock-name="中证 1000" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>

          <n-tab-pane name="科创50" tab="科创50"  >
            <StockLightweightKlineChart code="000688.SH" :chart-height="panelHeight-130" stock-name="科创 50" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="科创芯片" tab="科创芯片"  >
            <StockLightweightKlineChart code="000685.SH" :chart-height="panelHeight-130" stock-name="科创芯片" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="证券龙头" tab="证券龙头"  >
            <StockLightweightKlineChart code="399437.SZ" :chart-height="panelHeight-130" stock-name="证券龙头" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="高端装备" tab="高端装备"  >
            <StockLightweightKlineChart code="399437.SZ" :chart-height="panelHeight-130" stock-name="高端装备" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="中证银行" tab="中证银行">
            <StockLightweightKlineChart code="399986.SZ" :chart-height="panelHeight-130" stock-name="中证银行" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="上证医药" tab="上证医药">
            <StockLightweightKlineChart code="000037.SH" :chart-height="panelHeight-130" stock-name="上证医药" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="中证白酒" tab="中证白酒">
            <StockLightweightKlineChart code="399997.SZ" :chart-height="panelHeight-130" stock-name="中证白酒" :dark-theme="true"></StockLightweightKlineChart>
          </n-tab-pane>
          <n-tab-pane name="富时中国三倍做多" tab="富时中国三倍做多">
            <k-line-chart code="usYINN.AM" :chart-height="panelHeight" stockName="富时中国三倍做多" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="VIX恐慌指数" tab="VIX恐慌指数">
            <k-line-chart code="usUVXY.AM" :chart-height="panelHeight" stockName="VIX恐慌指数" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="行业排名" tab="行业排名">
        <n-tabs type="card" animated>
          <n-tab-pane name="行业涨幅排名" tab="行业涨幅排名">
            <n-table striped>
              <n-thead>
                <n-tr>
                  <n-th>行业名称</n-th>
                  <n-th @click="changeIndustryRankSort">行业涨幅
                    <n-icon v-if="sort==='0'" :component="CaretDown"/>
                    <n-icon v-if="sort==='1'" :component="CaretUp"/>
                  </n-th>
                  <n-th>行业5日涨幅</n-th>
                  <n-th>行业20日涨幅</n-th>
                  <n-th>领涨股</n-th>
                  <n-th>涨幅</n-th>
                  <n-th>最新价</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="item in industryRanks" :key="item.bd_code">
                  <n-td>
                    <n-tag :bordered=false type="info">{{ item.bd_name }}</n-tag>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf>0?'error':'success'">{{ item.bd_zdf }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf5>0?'error':'success'">{{ item.bd_zdf5 }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf20>0?'error':'success'">{{ item.bd_zdf20 }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'"> {{ item.nzg_name }}
                      <n-text type="info">{{ item.nzg_code }}</n-text>
                    </n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'"> {{ item.nzg_zdf }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'">{{ item.nzg_zxj }}</n-text>
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
            <n-table striped>
              <n-thead>
                <n-tr>
                  <n-th>行业名称</n-th>
                  <n-th @click="changeIndustryRankSort">行业涨幅
                    <n-icon v-if="sort==='0'" :component="CaretDown"/>
                    <n-icon v-if="sort==='1'" :component="CaretUp"/>
                  </n-th>
                  <n-th>行业5日涨幅</n-th>
                  <n-th>行业20日涨幅</n-th>
                  <n-th>领涨股</n-th>
                  <n-th>涨幅</n-th>
                  <n-th>最新价</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="item in industryRanks" :key="item.bd_code">
                  <n-td>
                    <n-tag :bordered=false type="info">{{ item.bd_name }}</n-tag>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf>0?'error':'success'">{{ item.bd_zdf }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf5>0?'error':'success'">{{ item.bd_zdf5 }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.bd_zdf20>0?'error':'success'">{{ item.bd_zdf20 }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'"> {{ item.nzg_name }}
                      <n-text type="info">{{ item.nzg_code }}</n-text>
                    </n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'"> {{ item.nzg_zdf }}%</n-text>
                  </n-td>
                  <n-td>
                    <n-text :type="item.nzg_zdf>0?'error':'success'">{{ item.nzg_zxj }}</n-text>
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-tab-pane>
          <n-tab-pane name="行业资金排名(净流入)" tab="行业资金排名">
            <industryMoneyRank :fenlei="'0'" :header-title="'行业资金排名(净流入)'" :sort="'netamount'"/>
          </n-tab-pane>
          <n-tab-pane name="证监会行业资金排名(净流入)" tab="证监会行业资金排名">
            <industryMoneyRank :fenlei="'2'" :header-title="'证监会行业资金排名(净流入)'" :sort="'netamount'"/>
          </n-tab-pane>
          <n-tab-pane name="概念板块资金排名(净流入)" tab="概念板块资金排名">
            <industryMoneyRank :fenlei="'1'" :header-title="'概念板块资金排名(净流入)'" :sort="'netamount'"/>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="个股资金流向" tab="个股资金流向">
        <n-tabs type="card" animated>
          <n-tab-pane name="netamount" tab="净流入额排名">
            <RankTable :header-title="'净流入额排名'" :sort="'netamount'"/>
          </n-tab-pane>
          <n-tab-pane name="outamount" tab="流出资金排名">
            <RankTable :header-title="'流出资金排名'" :sort="'outamount'"/>
          </n-tab-pane>
          <n-tab-pane name="ratioamount" tab="净流入率排名">
            <RankTable :header-title="'净流入率排名'" :sort="'ratioamount'"/>
          </n-tab-pane>
          <n-tab-pane name="r0_net" tab="主力净流入额排名">
            <RankTable :header-title="'主力净流入额排名'" :sort="'r0_net'"/>
          </n-tab-pane>
          <n-tab-pane name="r0_out" tab="主力流出排名">
            <RankTable :header-title="'主力流出排名'" :sort="'r0_out'"/>
          </n-tab-pane>
          <n-tab-pane name="r0_ratio" tab="主力净流入率排名">
            <RankTable :header-title="'主力净流入率排名'" :sort="'r0_ratio'"/>
          </n-tab-pane>
          <n-tab-pane name="r3_net" tab="散户净流入额排名">
            <RankTable :header-title="'散户净流入额排名'" :sort="'r3_net'"/>
          </n-tab-pane>
          <n-tab-pane name="r3_out" tab="散户流出排名">
            <RankTable :header-title="'散户流出排名'" :sort="'r3_out'"/>
          </n-tab-pane>
          <n-tab-pane name="r3_ratio" tab="散户净流入率排名">
            <RankTable :header-title="'散户净流入率排名'" :sort="'r3_ratio'"/>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="板块资金流向" tab="板块资金流向">
        <BKFundFlowChart :dark-theme="darkTheme" :chart-height="600"/>
      </n-tab-pane>
      <n-tab-pane name="概念资金流向" tab="概念资金流向">
        <ConceptFundFlowChart :dark-theme="darkTheme" :chart-height="600"/>
      </n-tab-pane>
      <n-tab-pane name="龙虎榜" tab="龙虎榜">
        <LongTigerRankList />
      </n-tab-pane>
      <n-tab-pane name="个股研报" tab="个股研报">
        <StockResearchReportList :stock-code="stockCode"/>
      </n-tab-pane>
      <n-tab-pane name="公司公告" tab="公司公告 ">
        <StockNoticeList :stock-code="stockCode" />
      </n-tab-pane>
      <n-tab-pane name="行业研究" tab="行业研究 ">
        <IndustryResearchReportList/>
      </n-tab-pane>
      <n-tab-pane name="当前热门" tab="当前热门">
        <n-tabs type="card" animated>
          <n-tab-pane name="全球" tab="全球">
            <HotStockList :market-type="'10'"/>
          </n-tab-pane>
          <n-tab-pane name="沪深" tab="沪深">
            <HotStockList :market-type="'12'"/>
          </n-tab-pane>
          <n-tab-pane name="港股" tab="港股">
            <HotStockList :market-type="'13'"/>
          </n-tab-pane>
          <n-tab-pane name="美股" tab="美股">
            <HotStockList :market-type="'11'"/>
          </n-tab-pane>
          <n-tab-pane name="热门话题" tab="热门话题">
            <n-grid :cols="1" :y-gap="10">
              <n-grid-item>
                <HotTopics/>
              </n-grid-item>
<!--              <n-grid-item>-->
<!--                <HotEvents/>-->
<!--              </n-grid-item>-->
            </n-grid>
          </n-tab-pane>
          <n-tab-pane name="重大事件时间轴" tab="重大事件时间轴">
            <InvestCalendarTimeLine />
          </n-tab-pane>
          <n-tab-pane name="财经日历" tab="财经日历">
            <ClsCalendarTimeLine />
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="名站优选" tab="名站优选">
        <Stockhotmap />
      </n-tab-pane>
    </n-tabs>
  </n-card>
  <n-modal transform-origin="center" v-model:show="summaryModal" preset="card" style="width: 800px;max-width: calc(100vw - 32px);"
           :title="'AI市场资讯总结'">
    <n-spin size="small" :show="loading && !aiSummary">
      <div ref="aiResultScrollRef" style="height: 440px;max-height: 60vh;text-align: left;overflow-y: auto;">
        <MdPreview ref="mdPreviewRef" :modelValue="aiSummary" :theme="theme"/>
      </div>
    </n-spin>
    <template #footer>
      <n-flex justify="space-between" ref="tipsRef">
        <n-text type="info" v-if="aiSummaryTime">
          <n-tag v-if="modelName" type="warning" round :title="chatId" :bordered="false">{{ modelName }}</n-tag>
          {{ aiSummaryTime }}
        </n-text>
        <n-text type="success" v-if="analysisStatus">{{ analysisStatus }}</n-text>
        <n-text type="error">*AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。</n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="left" style="margin-bottom: 10px">
        <n-switch v-model:value="enableTools" :round="false">
          <template #checked>
            工具调用
          </template>
          <template #unchecked>
            非工具调用
          </template>
        </n-switch>
        <n-switch v-model:value="thinkingMode" :round="false">
          <template #checked>
            思考模式
          </template>
          <template #unchecked>
            非思考模式
          </template>
        </n-switch>


        <n-gradient-text type="error" style="margin-left: 10px">*AI函数工具调用可以增强AI获取数据的能力,但会消耗更多tokens。</n-gradient-text>
      </n-flex>
      <n-flex justify="space-between" style="margin-bottom: 10px">
        <n-select style="width: 32%" v-model:value="aiConfigId" label-field="name" value-field="ID"
                  :options="aiConfigs" placeholder="请选择AI模型服务配置"/>
        <n-select style="width: 32%" v-model:value="sysPromptId" label-field="name" value-field="ID"
                  :options="sysPromptOptions" placeholder="请选择系统提示词"/>
        <n-select style="width: 32%" v-model:value="question" label-field="name" value-field="content"
                  :options="userPromptOptions" placeholder="请选择用户提示词"/>
      </n-flex>
      <n-flex justify="right">
        <n-input v-model:value="question" style="text-align: left" clearable
                 type="textarea"
                 :show-count="true"
                 placeholder="请输入您的问题:例如 总结和分析股票市场新闻中的投资机会"
                 :autosize="{
              minRows: 2,
              maxRows: 5
            }"
        />
        <n-button size="tiny" type="warning" @click="reAiSummary">再次总结</n-button>
        <n-button size="tiny" type="success" @click="copyToClipboard">复制到剪切板</n-button>
        <n-button size="tiny" type="primary" @click="saveAsMarkdown">保存为Markdown文件</n-button>
        <n-button size="tiny" type="error" @click="share">分享到项目社区</n-button>
      </n-flex>
    </template>
  </n-modal>

  <div style="position: fixed;bottom: 18px;right:25px;z-index: 10;" v-if="summaryBTN">
    <n-input-group>
      <n-button type="primary" @click="getAiSummary">
        <n-icon :component="PulseOutline"/> &nbsp;AI总结
      </n-button>
    </n-input-group>
  </div>



</template>
<style scoped>
</style>