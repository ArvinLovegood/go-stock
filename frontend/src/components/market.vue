<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, ref} from 'vue'
import _ from "lodash";
import {
  GetAIResponseResult,
  GetConfig,
  GetIndustryRank,
  GetPromptTemplates,
  GetTelegraphList,
  GlobalStockIndexes, LongTigerRank,
  ReFleshTelegraphList,
  SaveAIResponseResult,
  SaveAsMarkdown,
  ShareAnalysis,
  SummaryStockNews
} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import NewsList from "./newsList.vue";
import KLineChart from "./KLineChart.vue";
import {ArrowDownOutline, CaretDown, CaretUp, PulseOutline,} from "@vicons/ionicons5";
import {NAvatar, NButton, NFlex, NText, useMessage, useNotification} from "naive-ui";
import {MdPreview} from "md-editor-v3";
import {useRoute} from 'vue-router'
import RankTable from "./rankTable.vue";
import IndustryMoneyRank from "./industryMoneyRank.vue";
import MoneyTrend from "./moneyTrend.vue";
import StockResearchReportList from "./StockResearchReportList.vue";
import StockNoticeList from "./StockNoticeList.vue";

const route = useRoute()
const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');

const message = useMessage()
const notify = useNotification()
const panelHeight = ref(window.innerHeight - 240)

const telegraphList = ref([])
const sinaNewsList = ref([])
const lhbList=  ref([])
const common = ref([])
const america = ref([])
const europe = ref([])
const asia = ref([])
const other = ref([])
const globalStockIndexes = ref(null)
const summaryModal = ref(false)
const summaryBTN = ref(true)
const darkTheme = ref(false)
const theme = computed(() => {
  return darkTheme ? 'dark' : 'light'
})
const aiSummary = ref(``)
const aiSummaryTime = ref("")
const modelName = ref("")
const chatId = ref("")
const question = ref(``)
const sysPromptId = ref(0)
const loading = ref(true)
const sysPromptOptions = ref([])
const userPromptOptions = ref([])
const promptTemplates = ref([])
const industryRanks = ref([])
const industryMoneyRankSina = ref([])
const sort = ref("0")
const sortIcon = ref(h(CaretDown))
const nowTab = ref("市场快讯")
const indexInterval = ref(null)
const indexIndustryRank = ref(null)
const drawerShow=  ref(false)
const EXPLANATIONs = ref([])

const today = new Date();
const year = today.getFullYear();
const month = String(today.getMonth() + 1).padStart(2, '0'); // 月份从0开始，需要+1
const day = String(today.getDate()).padStart(2, '0');

// 常见格式：YYYY-MM-DD
const formattedDate = `${year}-${month}-${day}`;

const SearchForm=  ref({
  dateValue:  formattedDate,
  EXPLANATION:null,
})

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

function longTiger(date) {
  if(date) {
    SearchForm.value.dateValue = date
  }
  let loading1=message.loading("正在获取龙虎榜数据...",{
    duration: 0,
  })
  LongTigerRank(date).then(res => {
    lhbList.value = res
    loading1.destroy()
    if (res.length === 0) {
      message.info("暂无数据,请切换日期")
    }
    EXPLANATIONs.value=_.uniqBy(_.map(lhbList.value,function (item){
      return {
        label: item['EXPLANATION'],
        value: item['EXPLANATION'],
      }
    }),'label');
  })
}

function handleEXPLANATION(value, option){
  SearchForm.value.EXPLANATION = value
  if(value){
    LongTigerRank(SearchForm.value.dateValue).then(res => {
      lhbList.value=_.filter(res, function(o) { return o['EXPLANATION']===value; });
      if (res.length === 0) {
        message.info("暂无数据,请切换日期")
      }
    })
  }else{
    longTiger(SearchForm.value.dateValue)
  }
}

function sortLongTigerRank(e){
  //console.log(e.target.dataset)
  // let field= e.target.dataset.field;
  // lhbList.value= _.sortBy(lhbList.value, function(o) { return o[field]; });
}

onBeforeMount(() => {
  nowTab.value = route.query.name
  GetConfig().then(result => {
    summaryBTN.value = result.openAiEnable
    darkTheme.value = result.darkTheme
  })
  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
    sysPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型系统Prompt')
    userPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型用户Prompt')
  })

  GetTelegraphList("财联社电报").then((res) => {
    telegraphList.value = res
  })
  GetTelegraphList("新浪财经").then((res) => {
    sinaNewsList.value = res
  })
  getIndex();
  industryRank();
  indexInterval.value = setInterval(() => {
    getIndex()
  }, 3000)

  indexIndustryRank.value = setInterval(() => {
    industryRank()
  }, 1000 * 10)
  longTiger(formattedDate);
})

onBeforeUnmount(() => {
  EventsOff("changeMarketTab")
  EventsOff("newTelegraph")
  EventsOff("newSinaNews")
  EventsOff("summaryStockNews")
  clearInterval(indexInterval.value)
  clearInterval(indexIndustryRank.value)
})

EventsOn("changeMarketTab", async (msg) => {
  //message.info(msg.name)
  updateTab(msg.name)
})

EventsOn("newTelegraph", (data) => {
  for (let i = 0; i < data.length; i++) {
    telegraphList.value.pop()
  }
  telegraphList.value.unshift(...data)
})
EventsOn("newSinaNews", (data) => {
  for (let i = 0; i < data.length; i++) {
    sinaNewsList.value.pop()
  }
  sinaNewsList.value.unshift(...data)
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
  SummaryStockNews(question.value, sysPromptId.value)
}

function getAiSummary() {
  summaryModal.value = true
  loading.value = true
  GetAIResponseResult("市场资讯").then(result => {
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
      SummaryStockNews(question.value, sysPromptId.value)
    }
  })
}

function updateTab(name) {
  summaryBTN.value = (name === "市场快讯");
  nowTab.value = name
}

EventsOn("summaryStockNews", async (msg) => {
  loading.value = false
  ////console.log(msg)
  if (msg === "DONE") {
    SaveAIResponseResult("市场资讯", "市场资讯", aiSummary.value, chatId.value, question.value)
    message.info("AI分析完成！")
    message.destroyAll()

  } else {
    if (msg.chatId) {
      chatId.value = msg.chatId
    }
    if (msg.question) {
      question.value = msg.question
    }
    if (msg.content) {
      aiSummary.value = aiSummary.value + msg.content
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
  }
})

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
  })
}
</script>

<template>
  <n-card>
    <n-tabs type="line" animated @update-value="updateTab" :value="nowTab">
      <n-tab-pane name="市场快讯" tab="市场快讯">
        <n-grid :cols="2" :y-gap="0">
          <n-gi>
            <news-list :newsList="telegraphList" :header-title="'财联社电报'" @update:message="ReFlesh"></news-list>
          </n-gi>
          <n-gi>
            <news-list :newsList="sinaNewsList" :header-title="'新浪财经'" @update:message="ReFlesh"></news-list>
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
            <k-line-chart code="sh000001" :chart-height="panelHeight" name="上证指数" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="深证成指" tab="深证成指">
            <k-line-chart code="sz399001" :chart-height="panelHeight" name="深证成指" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="创业板指" tab="创业板指">
            <k-line-chart code="sz399006" :chart-height="panelHeight" name="创业板指" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="恒生指数" tab="恒生指数">
            <k-line-chart code="hkHSI" :chart-height="panelHeight" name="恒生指数" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="纳斯达克" tab="纳斯达克">
            <k-line-chart code="us.IXIC" :chart-height="panelHeight" name="纳斯达克" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="道琼斯" tab="道琼斯">
            <k-line-chart code="us.DJI" :chart-height="panelHeight" name="道琼斯" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="标普500" tab="标普500">
            <k-line-chart code="us.INX" :chart-height="panelHeight" name="标普500" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="指标行情" tab="指标行情">
        <n-tabs type="segment" animated>
          <n-tab-pane name="科创50" tab="科创50">
            <k-line-chart code="sh000688" :chart-height="panelHeight" name="科创50" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="沪深300" tab="沪深300">
            <k-line-chart code="sh000300" :chart-height="panelHeight" name="沪深300" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="上证50" tab="上证50">
            <k-line-chart code="sh000016" :chart-height="panelHeight" name="上证50" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="中证A500" tab="中证A500">
            <k-line-chart code="sh000510" :chart-height="panelHeight" name="中证A500" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="中证1000" tab="中证1000">
            <k-line-chart code="sh000852" :chart-height="panelHeight" name="中证1000" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="中证白酒" tab="中证白酒">
            <k-line-chart code="sz399997" :chart-height="panelHeight" name="中证白酒" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="富时中国三倍做多" tab="富时中国三倍做多">
            <k-line-chart code="usYINN.AM" :chart-height="panelHeight" name="富时中国三倍做多" :k-days="20"
                          :dark-theme="true"></k-line-chart>
          </n-tab-pane>
          <n-tab-pane name="VIX恐慌指数" tab="VIX恐慌指数">
            <k-line-chart code="usUVXY.AM" :chart-height="panelHeight" name="VIX恐慌指数" :k-days="20"
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
      <n-tab-pane name="龙虎榜" tab="龙虎榜">
          <n-form :model="SearchForm" >
            <n-grid :cols="24" :x-gap="24">
            <n-form-item-gi  :span="4" label="日期" path="dateValue" label-placement="left">
              <n-date-picker   v-model:formatted-value="SearchForm.dateValue"
                             value-format="yyyy-MM-dd"  type="date"  :on-update:value="(v,v2)=>longTiger(v2)"/>

            </n-form-item-gi>
            <n-form-item-gi :span="8" label="上榜原因" path="EXPLANATION" label-placement="left">
              <n-select  clearable placeholder="上榜原因过滤" v-model:value="SearchForm.EXPLANATION" :options="EXPLANATIONs" :on-update:value="handleEXPLANATION"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" label=""  label-placement="left">
              <n-text type="error">*当天的龙虎榜数据通常在收盘结束后一小时左右更新</n-text>
            </n-form-item-gi>
            </n-grid>
          </n-form>
          <n-table :single-line="false" striped>
            <n-thead>
              <n-tr>
                <n-th>代码</n-th>
<!--                <n-th width="90px">日期</n-th>-->
                <n-th width="60px">名称</n-th>
                <n-th>收盘价</n-th>
                <n-th width="60px">涨跌幅</n-th>
                <n-th>龙虎榜净买额(万)</n-th>
                <n-th>龙虎榜买入额(万)</n-th>
                <n-th>龙虎榜卖出额(万)</n-th>
                <n-th>龙虎榜成交额(万)</n-th>
<!--                <n-th>市场总成交额(万)</n-th>-->
<!--                <n-th>净买额占总成交比</n-th>-->
<!--                <n-th>成交额占总成交比</n-th>-->
                <n-th width="60px"  data-field="TURNOVERRATE" @click="sortLongTigerRank">换手率<n-icon :component="ArrowDownOutline" /></n-th>
                <n-th>流通市值(亿)</n-th>
                <n-th>上榜原因</n-th>
<!--                <n-th>解读</n-th>-->
              </n-tr>
            </n-thead>
            <n-tbody>
              <n-tr v-for="(item, index) in lhbList" :key="index">
                <n-td>
                  <n-tag :bordered=false type="info">{{ item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0] }}</n-tag>
                </n-td>
<!--                <n-td>
                  {{item.TRADE_DATE.substring(0,10)}}
                </n-td>-->
                <n-td>
<!--                  <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.SECURITY_NAME_ABBR }}</n-text>-->
                  <n-popover trigger="hover" placement="right">
                    <template #trigger>
                      <n-button tag="a"  text :type="item.CHANGE_RATE>0?'error':'success'" :bordered=false >{{ item.SECURITY_NAME_ABBR }}</n-button>
                    </template>
                    <k-line-chart style="width: 800px" :code="item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0]" :chart-height="500" :name="item.SECURITY_NAME_ABBR" :k-days="20" :dark-theme="true"></k-line-chart>
                  </n-popover>
                </n-td>
                <n-td>
                  <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.CLOSE_PRICE }}</n-text>
                </n-td>
                <n-td>
                  <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ (item.CHANGE_RATE).toFixed(2) }}%</n-text>
                </n-td>
                <n-td>
<!--                  <n-text :type="item.BILLBOARD_NET_AMT>0?'error':'success'">{{ (item.BILLBOARD_NET_AMT/10000).toFixed(2) }}</n-text>-->


                  <n-popover trigger="hover" placement="right">
                    <template #trigger>
                      <n-button tag="a"  text :type="item.BILLBOARD_NET_AMT>0?'error':'success'" :bordered=false >{{ (item.BILLBOARD_NET_AMT/10000).toFixed(2) }}</n-button>
                    </template>
                    <money-trend :code="item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0]" :name="item.SECURITY_NAME_ABBR" :days="360" :dark-theme="true" :chart-height="500" style="width: 800px"></money-trend>
                  </n-popover>

                </n-td>
                <n-td>
                  <n-text :type="'error'">{{ (item.BILLBOARD_BUY_AMT/10000).toFixed(2) }}</n-text>
                </n-td>
                <n-td>
                  <n-text :type="'success'">{{ (item.BILLBOARD_SELL_AMT/10000).toFixed(2) }}</n-text>
                </n-td>
                <n-td>
                  <n-text :type="'info'">{{ (item.BILLBOARD_DEAL_AMT/10000).toFixed(2) }}</n-text>
                </n-td>
<!--                <n-td>-->
<!--                  <n-text :type="'info'">{{ (item.ACCUM_AMOUNT/10000).toFixed(2) }}</n-text>-->
<!--                </n-td>-->
<!--                <n-td>-->
<!--                  <n-text :type="item.DEAL_NET_RATIO>0?'error':'success'">{{ (item.DEAL_NET_RATIO).toFixed(2) }}%</n-text>-->
<!--                </n-td>-->
<!--                <n-td>-->
<!--                  <n-text :type="'info'">{{ (item.DEAL_AMOUNT_RATIO).toFixed(2) }}%</n-text>-->
<!--                </n-td>-->
                <n-td>
                  <n-text :type="'info'">{{ (item.TURNOVERRATE).toFixed(2) }}%</n-text>
                </n-td>
                <n-td>
                  <n-text :type="'info'">{{ (item.FREE_MARKET_CAP/100000000).toFixed(2) }}</n-text>
                </n-td>
                <n-td>
                  <n-text :type="'info'">{{ item.EXPLANATION }}</n-text>
                </n-td>
<!--                <n-td>
                  <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.EXPLAIN }}</n-text>
                </n-td>-->
              </n-tr>
            </n-tbody>
          </n-table>
      </n-tab-pane>
      <n-tab-pane name="个股研报" tab="个股研报">
        <StockResearchReportList/>
      </n-tab-pane>
      <n-tab-pane name="公司公告" tab="公司公告 ">
        <StockNoticeList/>
      </n-tab-pane>

    </n-tabs>
  </n-card>
  <n-modal transform-origin="center" v-model:show="summaryModal" preset="card" style="width: 800px;"
           :title="'AI市场资讯总结'">
    <n-spin size="small" :show="loading">
      <MdPreview style="height: 440px;text-align: left" :modelValue="aiSummary" :theme="theme"/>
    </n-spin>
    <template #footer>
      <n-flex justify="space-between" ref="tipsRef">
        <n-text type="info" v-if="aiSummaryTime">
          <n-tag v-if="modelName" type="warning" round :title="chatId" :bordered="false">{{ modelName }}</n-tag>
          {{ aiSummaryTime }}
        </n-text>
        <n-text type="error">*AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。</n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="space-between" style="margin-bottom: 10px">
        <n-select style="width: 49%" v-model:value="sysPromptId" label-field="name" value-field="ID"
                  :options="sysPromptOptions" placeholder="请选择系统提示词"/>
        <n-select style="width: 49%" v-model:value="question" label-field="name" value-field="content"
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

  <n-drawer v-model:show="drawerShow" :width="502">
    <n-drawer-content title="斯通纳" closable>
      《斯通纳》是美国作家约翰·威廉姆斯在 1965 年出版的小说。
    </n-drawer-content>
  </n-drawer>

</template>
<style scoped>

</style>