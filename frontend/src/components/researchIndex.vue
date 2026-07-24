<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {GetAIResponseResultList} from "../../wailsjs/go/main/App";
import {NButton, NEllipsis, NText} from "naive-ui";
import ResearchReport from "./researchReport.vue";
import AiRecommendStocksList from "./aiRecommendStocksList.vue";
import PromptTemplateList from "./promptTemplateList.vue";
import AllStockList from "./allStockList.vue";
import AllStockInfoList from "./allStockInfoList.vue";
import CronTaskManager from "./cron-task-manager.vue";
import TradingRecordManager from "./TradingRecordManager.vue";
import StockChangesMonitor from "./stockChangesMonitor.vue";
import MCPServiceManager from "./mcp-server-manager.vue";
import SkillManager from "./skill-manager.vue";
import UplimitLadder from "./uplimitLadder.vue";
import PromptPlaza from "./promptPlaza.vue";
import PromptQa from "./promptQa.vue";
import SelectStock from "./SelectStock.vue";
import DailyOperationPlan from "./DailyOperationPlan.vue";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import {useRoute} from 'vue-router'


const nowTab = ref("AI分析报告")
const route = useRoute()
onBeforeMount(() => {
  nowTab.value = route.query.name
})

onBeforeUnmount(() => {
  EventsOff("changeResearchTab")
})

onUnmounted(() => {

});

EventsOn("changeResearchTab", async (msg) => {
  console.log("changeResearchTab", msg)
  updateTab(msg.name)
})
function updateTab(name) {
  nowTab.value = name
}
</script>

<template>
  <n-card>
    <n-tabs type="line" animated @update-value="updateTab" :value="nowTab" style="--wails-draggable:no-drag">
      <n-tab-pane name="AI分析报告">
        <ResearchReport/>
      </n-tab-pane>
      <n-tab-pane name="股票推荐记录">
        <AiRecommendStocksList/>
      </n-tab-pane>
      <n-tab-pane name="异动监控">
        <StockChangesMonitor/>
      </n-tab-pane>
      <n-tab-pane name="涨停梯队">
        <UplimitLadder/>
      </n-tab-pane>
      <n-tab-pane name="提示词模板">
        <PromptTemplateList/>
      </n-tab-pane>
      <n-tab-pane name="提示词广场">
        <PromptPlaza/>
      </n-tab-pane>
      <n-tab-pane name="问答广场">
        <PromptQa/>
      </n-tab-pane>
      <n-tab-pane name="形态选股">
        <AllStockList/>
      </n-tab-pane>
      <n-tab-pane name="指标选股">
        <SelectStock/>
      </n-tab-pane>
      <n-tab-pane name="定时任务">
        <CronTaskManager />
      </n-tab-pane>
      <n-tab-pane name="交易日志">
        <TradingRecordManager />
      </n-tab-pane>
      <n-tab-pane name="每日操作计划">
        <DailyOperationPlan/>
      </n-tab-pane>
<!--      <n-tab-pane name="全部股票信息">-->
<!--        <AllStockInfoList/>-->
<!--      </n-tab-pane>-->
      <n-tab-pane name="MCP服务">
        <MCPServiceManager/>
      </n-tab-pane>
<!--      <n-tab-pane name="技能管理">-->
<!--        <SkillManager/>-->
<!--      </n-tab-pane>-->
    </n-tabs>
  </n-card>
</template>

<style scoped>
</style>
