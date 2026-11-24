<script setup>
import {onBeforeMount, onBeforeUnmount, onMounted, ref, computed} from 'vue'
import {GetConfig, GetTelegraphList, ReFleshTelegraphList} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import {format} from 'date-fns';
import {zhCN} from 'date-fns/locale';
import {ChevronDownOutline, ChevronForwardOutline} from "@vicons/ionicons5";
import AnalyzeMartket from "./AnalyzeMartket.vue";
import ConceptEventList from "./ConceptEventList.vue";
import RzrqRank from "./RzrqRank.vue";
import NewsList from "./newsList.vue";
import BKFundFlowChart from "./bkFundFlowChart.vue";
import ConceptFundFlowChart from "./conceptFundFlowChart.vue";

const darkTheme = ref(false)
const telegraphList = ref([])
const time = ref(new Date())

const timeText = computed(() => format(time.value, 'yyyy-MM-dd HH:mm:ss EEEE QQQQ', {locale: zhCN}))

const updateTime = () => {
  time.value = new Date()
}

// 资金流向图：折叠/展开（参考韩国市场，localStorage 记忆，默认展开）
const fundFlowVisible = ref(localStorage.getItem('fundFlowChartsVisible') !== '0')

function toggleFundFlow() {
  fundFlowVisible.value = !fundFlowVisible.value
  localStorage.setItem('fundFlowChartsVisible', fundFlowVisible.value ? '1' : '0')
}

let timer = null

onBeforeMount(() => {
  GetConfig().then(res => {
    darkTheme.value = res.darkTheme
  }).catch(err => {
    console.error('Home GetConfig error:', err)
  })
})

onMounted(() => {
  timer = setInterval(updateTime, 1000)
  GetTelegraphList("财联社电报").then(res => {
    telegraphList.value = res || []
  }).catch(err => {
    console.error('GetTelegraphList error:', err)
  })
  EventsOn("newTelegraph", (data) => {
    if (data != null) {
      for (let i = 0; i < data.length; i++) {
        telegraphList.value.pop()
      }
      telegraphList.value.unshift(...data)
    }
  })
})

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer)
  }
  EventsOff("newTelegraph")
})

function refreshTelegraph() {
  ReFleshTelegraphList("财联社电报").then(res => {
    telegraphList.value = res || []
  })
}
</script>

<template>
  <n-card class="home-page-card" size="small" style="--wails-draggable:no-drag">
    <template #header>
      <n-flex align="center" :wrap="false">
        <n-text strong>首页</n-text>
        <n-text depth="3" style="font-size: 12px;">市场总览 · 炒作题材 · 财联社电报</n-text>
        <n-tag :bordered="false" type="info">{{ timeText }}</n-tag>
      </n-flex>
    </template>

    <n-flex class="home-page-body" vertical :size="12">
      <!-- 大盘分析：全球股指跑马灯 + 市场情绪 + 涨跌停/分时/融资融券走势 -->
      <AnalyzeMartket :dark-theme="darkTheme" :chart-height="280"/>

      <!-- 板块/概念资金流向图（并排，可折叠、可切换日期） -->
      <div @click="toggleFundFlow"
           style="display:flex;align-items:center;gap:4px;cursor:pointer;user-select:none;--wails-draggable:no-drag">
        <n-icon size="14" :depth="3" :component="fundFlowVisible ? ChevronDownOutline : ChevronForwardOutline"/>
        <n-text depth="3" style="font-size:12px">资金流向 · 板块 / 概念 多板块对比</n-text>
      </div>
      <n-flex v-if="fundFlowVisible" :size="12" :wrap="false" style="align-items: stretch;">
        <div style="flex: 1; min-width: 0;">
          <BKFundFlowChart :dark-theme="darkTheme" :chart-height="300" compact/>
        </div>
        <div style="flex: 1; min-width: 0;">
          <ConceptFundFlowChart :dark-theme="darkTheme" :chart-height="300" compact/>
        </div>
      </n-flex>

      <!-- 每日炒作题材 + 融资融券 + 财联社电报 三列等高 -->
      <n-flex class="home-page-panels" :size="12" :wrap="false" style="align-items: stretch;">
        <div class="thin-scroll" style="flex: 1; min-width: 0; height: 100%; overflow-y: auto;">
          <ConceptEventList/>
        </div>
        <div class="thin-scroll" style="flex: 1; min-width: 0; height: 100%; overflow-y: auto;">
          <NewsList :newsList="telegraphList" :headerTitle="'财联社电报'" @update:message="refreshTelegraph"/>
        </div>
        <div class="thin-scroll" style="flex: 1; min-width: 0; height: 100%; overflow-y: auto;">
          <RzrqRank :dark-theme="darkTheme"/>
        </div>
      </n-flex>
    </n-flex>
  </n-card>
</template>

<style scoped>
.home-page-card :deep(.n-card-content),
.home-page-body {
  height: 100%;
  min-height: 0;
}
.home-page-body {
  display: flex;
  flex-direction: column;
}
.home-page-panels {
  flex: 1 1 0;
  min-height: 0;
}
:deep(.thin-scroll) {
  scrollbar-width: none;
}
:deep(.thin-scroll::-webkit-scrollbar) {
  display: none;
}
</style>
