<template>
  <div style="padding: 16px;">
    <!-- 新建任务 -->
    <n-card title="新建提示词回测任务" size="small" style="margin-bottom: 16px;text-align: left;">
      <n-alert type="info" :show-icon="true" style="margin-bottom: 12px;">
        批量重放历史交易日：对每个交易日拼装当日可见的市场素材（指数行情/涨停梯队/龙虎榜/板块资金流，防未来数据），
        用所选模板作为系统提示词让 AI 选股，再计算后 N 交易日实际收益，对比各模板的胜率/收益/波动率/稳定性。
        调用次数 = 模板数 × 采样交易日数 × 重复次数，注意 AI 配额成本。
      </n-alert>
      <n-space align="center" :wrap="true" :size="[8, 12]">
        <span class="form-label">提示词模板：</span>
        <n-select v-model:value="form.templateIds" multiple filterable size="small" :options="templateOptions"
                  placeholder="选择要对比的模板（最多5个）" style="width: min(340px, 70vw);" :max-tag-count="3" />
        <span class="form-label">AI 配置：</span>
        <n-select v-model:value="form.aiConfigId" size="small" :options="aiConfigOptions" style="width: 200px;" />
        <span class="form-label">日期区间：</span>
        <n-date-picker v-model:value="dateRange" type="daterange" clearable size="small"
                       :is-date-disabled="(ts) => ts > Date.now()" style="width: 250px;" />
      </n-space>
      <n-space align="center" :wrap="true" :size="[8, 12]" style="margin-top: 8px;">
        <span class="form-label">持有周期：</span>
        <n-select v-model:value="form.periodDays" size="small" :options="periodOptions" style="width: 110px;" />
        <span class="form-label">选股上限：</span>
        <n-select v-model:value="form.topN" size="small" :options="topNOptions" style="width: 90px;" />
        <span class="form-label">重复次数：</span>
        <n-select v-model:value="form.repeatRuns" size="small" :options="repeatOptions" style="width: 150px;" />
        <span class="form-label">采样密度：</span>
        <n-select v-model:value="form.sampleEveryNDays" size="small" :options="sampleOptions" style="width: 130px;" />
        <n-button type="primary" size="small" :loading="creating" @click="createTask">开始回测</n-button>
      </n-space>
    </n-card>

    <!-- 任务列表 -->
    <n-card title="回测任务" size="small" style="text-align: left;">
      <n-data-table :columns="taskColumns" :data="tasks" :loading="loading" size="small" :bordered="false" />
    </n-card>

    <!-- 任务详情 -->
    <n-modal v-model:show="detailVisible" preset="card" style="width: 1100px;text-align: left"
             :title="'回测详情：' + (detail?.task?.name || '')">
      <template v-if="detail">
        <n-descriptions :column="4" size="small" bordered style="margin-bottom: 12px;">
          <n-descriptions-item label="状态">
            <n-tag size="small" :type="statusTag(detail.task.status).type">{{ statusTag(detail.task.status).label }}</n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="进度">{{ detail.task.progress }}%（{{ detail.task.doneCalls }}/{{ detail.task.totalCalls }} 次调用）</n-descriptions-item>
          <n-descriptions-item label="区间">{{ detail.task.startDate }} ~ {{ detail.task.endDate }}</n-descriptions-item>
          <n-descriptions-item label="周期">{{ detail.task.periodDays }} 交易日 / 每日 {{ detail.task.topN }} 只 / 重复 {{ detail.task.repeatRuns }} 次</n-descriptions-item>
        </n-descriptions>
        <n-alert v-if="detail.task.status === 'running'" type="info" style="margin-bottom: 12px;">
          {{ detail.task.progressMsg }}
        </n-alert>
        <n-alert v-if="detail.task.status === 'failed'" type="error" style="margin-bottom: 12px;">
          {{ detail.task.errorMessage }}
        </n-alert>

        <template v-if="detail.stats && detail.stats.length">
          <n-divider title-placement="left"><n-gradient-text type="info">模板对比</n-gradient-text></n-divider>
          <n-table :bordered="false" :single-line="false" size="small" style="margin-bottom: 16px;">
            <thead>
              <tr>
                <th>模板</th><th>选股</th><th>超额胜率</th><th>胜率</th><th>平均收益</th><th>平均超额</th>
                <th>波动率</th><th>CV</th><th>夏普</th><th>最大回撤</th><th>累计收益</th><th>输出稳定性</th><th>评分</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in detail.stats" :key="s.templateId"
                  :style="{cursor:'pointer', background: picksTemplateId === s.templateId ? 'rgba(64,128,255,0.12)' : ''}"
                  @click="loadPicks(detail.task.id, s.templateId)">
                <td>{{ s.templateName }}</td>
                <td>{{ s.total }}<n-text depth="3" v-if="s.skipped > 0" style="font-size:12px;">（{{s.skipped}}跳过）</n-text></td>
                <td :style="{color: (s.excessWinRate||0)>=50 ? '#18a058' : '#d03050'}">{{ s.excessWinRate?.toFixed(1) || 0 }}%</td>
                <td>{{ s.winRate?.toFixed(1) || 0 }}%</td>
                <td :style="{color: (s.avgReturn||0)>=0 ? '#d03050' : '#18a058'}">{{ s.avgReturn?.toFixed(2) || 0 }}%</td>
                <td :style="{color: (s.avgExcess||0)>=0 ? '#d03050' : '#18a058'}">{{ s.avgExcess?.toFixed(2) || 0 }}%</td>
                <td>{{ s.volatility?.toFixed(2) || 0 }}%</td>
                <td>{{ fmtCV(s.cv) }}</td>
                <td>{{ s.sharpe?.toFixed(2) || 0 }}</td>
                <td style="color:#18a058;">{{ s.maxDrawdown?.toFixed(2) || 0 }}%</td>
                <td :style="{color: (s.cumReturn||0)>=0 ? '#d03050' : '#18a058'}">{{ s.cumReturn?.toFixed(2) || 0 }}%</td>
                <td>{{ fmtJaccard(s.jaccard) }}</td>
                <td><n-tag size="small" :bordered="false" :type="(s.score||0)>=60 ? 'success' : ((s.score||0)>=40 ? 'warning' : 'error')">{{ s.score ?? 0 }}</n-tag></td>
              </tr>
            </tbody>
          </n-table>

          <n-divider title-placement="left"><n-gradient-text type="info">等权组合净值曲线对比</n-gradient-text></n-divider>
          <div id="promptBacktestCompareChart" style="width: 100%; height: 300px;"></div>
          <n-text depth="3" style="font-size: 12px;">
            Jaccard 输出稳定性：同日多次调用的选股集合重合度（0~1，越高输出越稳定，重复次数=1 时为"—"）。
            点击对比表行可查看该模板的选股明细。
          </n-text>
        </template>
        <n-empty v-else-if="detail.task.status === 'done'" description="任务完成但无选股记录（AI 可能均未按格式输出）" style="padding: 30px;" />

        <!-- 选股明细 -->
        <template v-if="picks.length">
          <n-divider title-placement="left">
            <n-gradient-text type="info">选股明细{{ picksTemplateName ? '：' + picksTemplateName : '' }}</n-gradient-text>
          </n-divider>
          <n-table :bordered="false" :single-line="false" size="small">
            <thead>
              <tr><th>日期</th><th>股票</th><th>评级</th><th>理由</th><th>买入价</th><th>期末价</th><th>收益</th><th>超额</th><th>次</th></tr>
            </thead>
            <tbody>
              <tr v-for="p in picks" :key="p.id">
                <td>{{ p.tradeDate }}</td>
                <td>{{ p.stockName }}({{ p.stockCode }})</td>
                <td>{{ p.rating }}</td>
                <td style="max-width: 320px; white-space: normal;">{{ p.reason }}</td>
                <td>{{ p.recommendPrice }}</td>
                <td>{{ p.endPrice }}</td>
                <td :style="{color: (p.returnPct||0)>=0 ? '#d03050' : '#18a058'}">{{ p.returnPct?.toFixed(2) || 0 }}%</td>
                <td :style="{color: (p.excessPct||0)>=0 ? '#d03050' : '#18a058'}">{{ p.excessPct?.toFixed(2) || 0 }}%</td>
                <td>{{ p.runIndex }}</td>
              </tr>
            </tbody>
          </n-table>
          <n-pagination style="margin-top: 10px; justify-content: flex-end;" v-model:page="picksPage" :page-size="picksPageSize"
                        :item-count="picksTotal" @update:page="loadPicks(detail.task.id, picksTemplateId)" />
        </template>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import {h, onMounted, onUnmounted, ref, nextTick} from 'vue'
import {
  NAlert, NButton, NCard, NDataTable, NDatePicker, NDescriptions, NDescriptionsItem, NDivider,
  NEmpty, NGradientText, NModal, NPagination, NSelect, NSpace, NTag, NTable, NText,
  useDialog, useMessage
} from 'naive-ui'
import {
  CreatePromptBacktestTask, DeletePromptBacktestTask, GetAiConfigs, GetPromptBacktestPicks,
  GetPromptBacktestTaskDetail, GetPromptBacktestTaskList, GetPromptTemplates
} from '../../wailsjs/go/main/App'
import {EventsOn, EventsOff} from '../../wailsjs/runtime'
import * as echarts from 'echarts'

const message = useMessage()
const dialog = useDialog()

// ---- 表单 ----
const templateOptions = ref([])
const aiConfigOptions = ref([{label: '默认（第一个AI配置）', value: 0}])
const dateRange = ref(null)
const form = ref({
  templateIds: [],
  aiConfigId: 0,
  periodDays: 5,
  topN: 5,
  repeatRuns: 2,
  sampleEveryNDays: 5
})
const periodOptions = [1, 3, 5, 10, 20].map(v => ({label: `${v} 交易日`, value: v}))
const topNOptions = [3, 5, 10].map(v => ({label: `${v} 只`, value: v}))
const repeatOptions = [
  {label: '1 次（不测稳定性）', value: 1},
  {label: '2 次', value: 2},
  {label: '3 次', value: 3}
]
const sampleOptions = [
  {label: '每个交易日', value: 1},
  {label: '每 2 个交易日', value: 2},
  {label: '每周（约5日）', value: 5},
  {label: '每 10 个交易日', value: 10}
]
const creating = ref(false)

// ---- 任务列表 ----
const tasks = ref([])
const loading = ref(false)

const statusTagMap = {
  pending: {label: '待执行', type: 'default'},
  running: {label: '执行中', type: 'info'},
  done: {label: '已完成', type: 'success'},
  failed: {label: '失败', type: 'error'}
}
function statusTag(status) {
  return statusTagMap[status] || {label: status, type: 'default'}
}

const taskColumns = [
  {title: '任务', key: 'name', minWidth: 220, ellipsis: {tooltip: true}},
  {title: '区间', width: 190, render: (row) => `${row.startDate} ~ ${row.endDate}`},
  {title: '周期', width: 90, render: (row) => `${row.periodDays}日`},
  {title: '状态', width: 90, render: (row) => h(NTag, {size: 'small', type: statusTag(row.status).type}, {default: () => statusTag(row.status).label})},
  {
    title: '进度', width: 160, render: (row) => {
      const pct = row.progress || 0
      return h('span', `${pct}%（${row.doneCalls}/${row.totalCalls}）`)
    }
  },
  {title: '耗时', width: 90, render: (row) => row.durationMs > 0 ? `${Math.round(row.durationMs / 1000)}s` : '-'},
  {
    title: '操作', width: 140, render: (row) => [
      h(NButton, {size: 'tiny', type: 'primary', style: 'margin-right:6px;', onClick: () => showDetail(row.id)}, {default: () => '详情'}),
      h(NButton, {
        size: 'tiny', type: 'error',
        disabled: row.status === 'running',
        onClick: () => confirmDelete(row)
      }, {default: () => '删除'})
    ]
  }
]

function confirmDelete(row) {
  dialog.warning({
    title: '删除回测任务',
    content: `确定删除任务「${row.name}」及其全部选股记录？`,
    positiveText: '删除', negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await DeletePromptBacktestTask(row.id)
        message.success('已删除')
        loadTasks()
      } catch (e) {
        message.error('删除失败：' + (e?.message || e))
      }
    }
  })
}

async function createTask() {
  if (!form.value.templateIds.length) {
    message.warning('请至少选择 1 个提示词模板')
    return
  }
  if (!dateRange.value || dateRange.value.length !== 2) {
    message.warning('请选择日期区间')
    return
  }
  creating.value = true
  try {
    await CreatePromptBacktestTask({
      name: '',
      templateIds: form.value.templateIds.join(','),
      aiConfigId: form.value.aiConfigId,
      startDate: tsToDate(dateRange.value[0]),
      endDate: tsToDate(dateRange.value[1]),
      periodDays: form.value.periodDays,
      topN: form.value.topN,
      repeatRuns: form.value.repeatRuns,
      sampleEveryNDays: form.value.sampleEveryNDays
    })
    message.success('回测任务已创建，正在后台执行')
    loadTasks()
  } catch (e) {
    message.error('创建失败：' + (e?.message || e))
  } finally {
    creating.value = false
  }
}

function tsToDate(ts) {
  const d = new Date(ts)
  const p = (v) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

async function loadTasks() {
  loading.value = true
  try {
    tasks.value = (await GetPromptBacktestTaskList()) || []
  } catch (e) {
    console.error('加载任务列表失败', e)
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  try {
    const templates = await GetPromptTemplates('', '')
    templateOptions.value = (templates || []).map(t => ({label: `${t.name}（${t.type}）`, value: t.ID}))
  } catch (e) {
    console.error('加载模板失败', e)
  }
  try {
    const configs = await GetAiConfigs()
    aiConfigOptions.value = [
      {label: '默认（第一个AI配置）', value: 0},
      ...(configs || []).map(c => ({label: `${c.name}[${c.modelName}]`, value: c.ID}))
    ]
  } catch (e) {
    console.error('加载AI配置失败', e)
  }
}

// ---- 详情 ----
const detailVisible = ref(false)
const detail = ref(null)
const picks = ref([])
const picksTemplateId = ref(0)
const picksTemplateName = ref('')
const picksPage = ref(1)
const picksPageSize = 20
const picksTotal = ref(0)
let compareChart = null

async function showDetail(taskId) {
  detailVisible.value = true
  picks.value = []
  picksTemplateId.value = 0
  picksTotal.value = 0
  try {
    detail.value = await GetPromptBacktestTaskDetail(taskId)
    nextTick(() => renderCompareChart())
  } catch (e) {
    message.error('加载详情失败：' + (e?.message || e))
  }
}

function renderCompareChart() {
  const el = document.getElementById('promptBacktestCompareChart')
  if (!el || !detail.value?.stats?.length) return
  if (compareChart) {
    compareChart.dispose()
    compareChart = null
  }
  compareChart = echarts.init(el)
  const series = []
  for (const s of detail.value.stats) {
    if (!s.curve || !s.curve.length) continue
    series.push({
      name: s.templateName,
      type: 'line',
      showSymbol: false,
      data: s.curve.map(p => [p.date, p.equity]),
      emphasis: {focus: 'series'}
    })
  }
  compareChart.setOption({
    tooltip: {trigger: 'axis', valueFormatter: (v) => '净值 ' + Number(v).toFixed(4)},
    legend: {top: 4},
    grid: {top: 40, left: 56, right: 24, bottom: 28},
    xAxis: {type: 'time'},
    yAxis: {type: 'value', scale: true},
    series
  })
}

async function loadPicks(taskId, templateId) {
  if (templateId !== picksTemplateId.value) {
    picksPage.value = 1
  }
  picksTemplateId.value = templateId
  const st = (detail.value?.stats || []).find(s => s.templateId === templateId)
  picksTemplateName.value = st ? st.templateName : ''
  try {
    const res = await GetPromptBacktestPicks(taskId, templateId, picksPage.value, picksPageSize)
    picks.value = res.list || []
    picksTotal.value = res.total || 0
  } catch (e) {
    message.error('加载明细失败：' + (e?.message || e))
  }
}

function fmtCV(cv) {
  if (cv === null || cv === undefined || cv < 0) return '—'
  return Number(cv).toFixed(2)
}

function fmtJaccard(j) {
  if (j === null || j === undefined || j < 0) return '—'
  return Number(j).toFixed(2)
}

// ---- 事件：任务进度推送（节流刷新列表 + 打开的详情） ----
// 引擎每次 AI 调用完成都会推送（一个任务可达数百次），直接刷新会造成高频
// Wails 调用与图表重建，这里 2 秒节流，末次事件保证执行。
let progressTimer = null
let progressPending = null
function onProgress(e) {
  progressPending = e
  if (progressTimer) return
  progressTimer = setTimeout(() => {
    progressTimer = null
    const ev = progressPending
    progressPending = null
    if (!ev) return
    loadTasks()
    if (detailVisible.value && detail.value && ev?.taskId === detail.value.task?.id) {
      GetPromptBacktestTaskDetail(ev.taskId).then((d) => {
        detail.value = d
        nextTick(() => renderCompareChart())
      }).catch(() => {})
    }
  }, 2000)
}

onMounted(() => {
  loadOptions()
  loadTasks()
  EventsOn('promptBacktestProgress', onProgress)
})

onUnmounted(() => {
  EventsOff('promptBacktestProgress')
  if (progressTimer) {
    clearTimeout(progressTimer)
    progressTimer = null
  }
  if (compareChart) {
    compareChart.dispose()
    compareChart = null
  }
})
</script>

<style scoped>
.form-label {
  font-size: 13px;
  white-space: nowrap;
  user-select: none;
}
</style>
