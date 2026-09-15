<template>
  <div style="padding: 16px; text-align: left;">
    <!-- 顶部操作条 -->
    <n-card size="small" style="margin-bottom: 12px;">
      <n-space align="center" :wrap="true">
        <n-button size="small" type="primary" ghost :loading="backtestLoading" @click="runBacktest">
          执行回测(5日)
        </n-button>
        <n-button size="small" type="info" ghost :loading="statsLoading" @click="refreshAll">
          刷新统计
        </n-button>
        <n-text depth="3" style="font-size: 12px;">
          对 AI 已推荐个股按 5 个交易日持有期计算实际收益（vs 沪深300），点击提示词/模板行可过滤下方明细。
        </n-text>
      </n-space>
    </n-card>

    <template v-if="backtestStatsRef">
      <!-- 总体统计 -->
      <n-card title="总体统计" size="small" style="margin-bottom: 12px;">
        <n-grid :cols="4" :x-gap="12" responsive="screen" item-responsive>
          <n-grid-item span="4 s:1"><n-statistic label="已回测" :value="backtestStatsRef.total || 0" /></n-grid-item>
          <n-grid-item span="4 s:1"><n-statistic label="达标" :value="backtestStatsRef.win || 0" /></n-grid-item>
          <n-grid-item span="4 s:1"><n-statistic label="未达标" :value="backtestStatsRef.lose || 0" /></n-grid-item>
          <n-grid-item span="4 s:1"><n-statistic label="胜率" :value="backtestStatsRef.winRate ? backtestStatsRef.winRate.toFixed(1) : 0" suffix="%" /></n-grid-item>
        </n-grid>
      </n-card>

      <!-- 按评级胜率 -->
      <n-card title="按评级胜率" size="small" style="margin-bottom: 12px;">
        <n-table :bordered="false" :single-line="false" size="small">
          <thead>
            <tr><th>评级</th><th>总数</th><th>达标</th><th>胜率</th></tr>
          </thead>
          <tbody>
            <tr v-for="(st, rating) in backtestStatsRef.byRating || {}" :key="rating">
              <td>{{rating}}</td>
              <td>{{st.total || 0}}</td>
              <td>{{st.win || 0}}</td>
              <td>{{st.winRate ? st.winRate.toFixed(1) : 0}}%</td>
            </tr>
            <tr v-if="!(backtestStatsRef.byRating && Object.keys(backtestStatsRef.byRating).length)">
              <td colspan="4" style="text-align:center; color:#999;">暂无回测数据，请先点击「执行回测(5日)」</td>
            </tr>
          </tbody>
        </n-table>
      </n-card>

      <!-- 按提示词统计 -->
      <n-card title="按提示词统计" size="small" style="margin-bottom: 12px;">
        <n-tabs type="line" size="small">
          <n-tab-pane name="sys" tab="系统提示词">
            <n-table :bordered="false" :single-line="false" size="small">
              <thead>
                <tr><th>提示词</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th></tr>
              </thead>
              <tbody>
                <tr v-for="(p, i) in backtestStatsRef.bySystemPrompt || []" :key="i">
                  <td style="max-width:300px;">
                    <n-tooltip trigger="hover">
                      <template #trigger>
                        <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestByPrompt('sys', p)">{{p.name}}</n-button>
                      </template>
                      点击按该提示词过滤下方「最近回测明细」
                    </n-tooltip>
                  </td>
                  <td>{{p.total}}</td>
                  <td>{{p.win}}</td>
                  <td :style="{color: (p.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{p.winRate ? p.winRate.toFixed(1) : 0}}%</td>
                  <td>{{p.avgReturn ? p.avgReturn.toFixed(2) : 0}}%</td>
                </tr>
                <tr v-if="!(backtestStatsRef.bySystemPrompt && backtestStatsRef.bySystemPrompt.length)">
                  <td colspan="5" style="text-align:center; color:#999;">暂无数据</td>
                </tr>
              </tbody>
            </n-table>
          </n-tab-pane>
          <n-tab-pane name="usr" tab="用户提示词">
            <n-table :bordered="false" :single-line="false" size="small">
              <thead>
                <tr><th>提示词</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th></tr>
              </thead>
              <tbody>
                <tr v-for="(p, i) in backtestStatsRef.byUserPrompt || []" :key="i">
                  <td style="max-width:300px;">
                    <n-tooltip trigger="hover">
                      <template #trigger>
                        <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestByPrompt('usr', p)">{{p.name}}</n-button>
                      </template>
                      点击按该提示词过滤下方「最近回测明细」
                    </n-tooltip>
                  </td>
                  <td>{{p.total}}</td>
                  <td>{{p.win}}</td>
                  <td :style="{color: (p.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{p.winRate ? p.winRate.toFixed(1) : 0}}%</td>
                  <td>{{p.avgReturn ? p.avgReturn.toFixed(2) : 0}}%</td>
                </tr>
                <tr v-if="!(backtestStatsRef.byUserPrompt && backtestStatsRef.byUserPrompt.length)">
                  <td colspan="5" style="text-align:center; color:#999;">暂无数据</td>
                </tr>
              </tbody>
            </n-table>
          </n-tab-pane>
        </n-tabs>
      </n-card>

      <!-- 达标率最高 -->
      <n-card title="达标率最高" size="small" style="margin-bottom: 12px;">
        <n-table :bordered="false" :single-line="false" size="small">
          <tbody>
            <tr v-if="backtestStatsRef.bestModel"><td style="width:120px;">最佳模型</td><td>{{backtestStatsRef.bestModel.name}}（达标率 {{backtestStatsRef.bestModel.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestModel.total}}）</td></tr>
            <tr v-if="backtestStatsRef.bestSystemPrompt"><td style="width:120px;">最佳系统提示词</td><td>{{backtestStatsRef.bestSystemPrompt.name}}（达标率 {{backtestStatsRef.bestSystemPrompt.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestSystemPrompt.total}}）</td></tr>
            <tr v-if="backtestStatsRef.bestUserPrompt"><td style="width:120px;">最佳用户提示词</td><td>{{backtestStatsRef.bestUserPrompt.name}}（达标率 {{backtestStatsRef.bestUserPrompt.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestUserPrompt.total}}）</td></tr>
            <tr v-if="backtestStatsRef.bestSkill"><td style="width:120px;">最佳技能</td><td>{{backtestStatsRef.bestSkill.name}}（达标率 {{backtestStatsRef.bestSkill.winRate.toFixed(1)}}%，N={{backtestStatsRef.bestSkill.total}}）</td></tr>
            <tr v-if="!backtestStatsRef.bestModel && !backtestStatsRef.bestSystemPrompt && !backtestStatsRef.bestUserPrompt && !backtestStatsRef.bestSkill">
              <td colspan="2" style="text-align:center; color:#999;">暂无数据</td>
            </tr>
          </tbody>
        </n-table>
      </n-card>

      <!-- 按提示词模板统计 -->
      <n-card title="按提示词模板统计" size="small" style="margin-bottom: 12px;">
        <n-table :bordered="false" :single-line="false" size="small">
          <thead>
            <tr><th>模板</th><th>样本</th><th>超额胜率</th><th>平均收益</th><th>平均超额</th><th>波动率</th><th>CV</th><th>最大回撤</th><th>累计收益</th><th>评分</th></tr>
          </thead>
          <tbody>
            <tr v-for="t in backtestStatsRef.byTemplate || []" :key="t.templateId">
              <td style="max-width:220px;">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestByTemplate(t)">{{t.templateName}}</n-button>
                  </template>
                  点击按该模板过滤下方「最近回测明细」
                </n-tooltip>
              </td>
              <td>{{t.total}}<n-text depth="3" v-if="t.total < 10" style="font-size:12px;">（样本少）</n-text></td>
              <td :style="{color: (t.excessWinRate||0)>=50 ? '#18a058' : '#d03050'}">{{t.excessWinRate ? t.excessWinRate.toFixed(1) : 0}}%</td>
              <td :style="{color: (t.avgReturn||0)>=0 ? '#d03050' : '#18a058'}">{{t.avgReturn ? t.avgReturn.toFixed(2) : 0}}%</td>
              <td :style="{color: (t.avgExcess||0)>=0 ? '#d03050' : '#18a058'}">{{t.avgExcess ? t.avgExcess.toFixed(2) : 0}}%</td>
              <td>{{t.volatility ? t.volatility.toFixed(2) : 0}}%</td>
              <td>{{fmtTemplateCV(t.cv)}}</td>
              <td style="color:#18a058;">{{t.maxDrawdown ? t.maxDrawdown.toFixed(2) : 0}}%</td>
              <td :style="{color: (t.cumReturn||0)>=0 ? '#d03050' : '#18a058'}">{{t.cumReturn ? t.cumReturn.toFixed(2) : 0}}%</td>
              <td><n-tag size="small" :type="(t.score||0)>=60 ? 'success' : ((t.score||0)>=40 ? 'warning' : 'error')" :bordered="false">{{t.score ?? 0}}</n-tag></td>
            </tr>
            <tr v-if="!(backtestStatsRef.byTemplate && backtestStatsRef.byTemplate.length)">
              <td colspan="10" style="text-align:center; color:#999;">暂无数据</td>
            </tr>
          </tbody>
        </n-table>
      </n-card>

      <!-- 按技能统计 -->
      <n-card title="按技能统计" size="small" style="margin-bottom: 12px;">
        <n-table :bordered="false" :single-line="false" size="small">
          <thead>
            <tr><th>技能</th><th>总数</th><th>达标</th><th>达标率</th><th>平均收益</th><th>平均超额</th></tr>
          </thead>
          <tbody>
            <tr v-for="s in backtestStatsRef.bySkill || []" :key="s.name">
              <td style="max-width:260px;">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-button text type="primary" style="cursor:pointer;" @click="filterBacktestBySkill(s)">{{s.name}}</n-button>
                  </template>
                  点击按该技能过滤下方「最近回测明细」
                </n-tooltip>
              </td>
              <td>{{s.total}}</td>
              <td>{{s.win}}</td>
              <td :style="{color: (s.winRate||0)>=50 ? '#18a058' : '#d03050'}">{{s.winRate ? s.winRate.toFixed(1) : 0}}%</td>
              <td>{{s.avgReturn ? s.avgReturn.toFixed(2) : 0}}%</td>
              <td>{{s.avgExcess ? s.avgExcess.toFixed(2) : 0}}%</td>
            </tr>
            <tr v-if="!(backtestStatsRef.bySkill && backtestStatsRef.bySkill.length)">
              <td colspan="6" style="text-align:center; color:#999;">暂无技能推荐数据（使用技能产生推荐并回测后显示；存量记录未记录技能ID）</td>
            </tr>
          </tbody>
        </n-table>
      </n-card>

      <!-- 最近回测明细 -->
      <n-card title="最近回测明细" size="small">
        <div v-if="backtestTemplateFilter || backtestPromptFilter || backtestSkillFilter" style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
          <n-tag v-if="backtestTemplateFilter" type="warning" closable @close="clearBacktestTemplateFilter">{{backtestTemplateFilterLabel}}</n-tag>
          <n-tag v-if="backtestPromptFilter" type="warning" closable @close="clearBacktestPromptFilter">{{backtestPromptFilterLabel}}</n-tag>
          <n-tag v-if="backtestSkillFilter" type="warning" closable @close="clearBacktestSkillFilter">{{backtestSkillFilterLabel}}</n-tag>
          <n-text depth="3">共 {{backtestTotalRef}} 条（点击提示词/模板/技能可过滤，关闭标签恢复全部）</n-text>
        </div>
        <n-data-table
          remote
          size="small"
          :columns="backtestListColumns"
          :data="backtestListRef"
          :loading="backtestListLoading"
          :pagination="{ page: backtestPageRef, pageSize: backtestPageSizeRef, itemCount: backtestTotalRef, onChange: (p) => loadBacktestList(p) }"
          style="max-height: 420px;"
        />
      </n-card>
    </template>
    <n-empty v-else-if="!statsLoading" description="暂无回测数据，请先点击「执行回测(5日)」" style="padding: 60px;" />
  </div>
</template>

<script setup>
import {computed, h, onMounted, ref} from 'vue'
import {NTag, NText, useNotification} from 'naive-ui'
import {
  RunRecommendBacktest, ListRecommendBacktest, ListRecommendBacktestByPrompt,
  ListRecommendBacktestByTemplate, ListRecommendBacktestBySkill, GetRecommendBacktestStats
} from '../../wailsjs/go/main/App'

const notify = useNotification()

// ===== 总体统计 =====
const backtestStatsRef = ref(null)
const statsLoading = ref(false)
// ===== 执行回测 =====
const backtestLoading = ref(false)

// ===== 回测明细 =====
const backtestListRef = ref([])
const backtestTotalRef = ref(0)
const backtestListLoading = ref(false)
const backtestPageRef = ref(1)
const backtestPageSizeRef = ref(10)
// 当前按提示词过滤条件：{ type: 'sys'|'usr', content, label }，null 表示不过滤
const backtestPromptFilter = ref(null)
const backtestPromptFilterLabel = computed(() => {
  const f = backtestPromptFilter.value
  if (!f) return ''
  return (f.type === 'sys' ? '系统提示词' : '用户提示词') + '：' + f.label
})
// 当前按提示词模板过滤条件：{ templateId, label }，null 表示不过滤（优先级高于提示词过滤）
const backtestTemplateFilter = ref(null)
const backtestTemplateFilterLabel = computed(() => {
  const f = backtestTemplateFilter.value
  if (!f) return ''
  return '模板：' + f.label
})
// 当前按技能过滤条件：{ skillId, label }，null 表示不过滤
const backtestSkillFilter = ref(null)
const backtestSkillFilterLabel = computed(() => {
  const f = backtestSkillFilter.value
  if (!f) return ''
  return '技能：' + f.label
})

const backtestListColumns = [
  { title: '推荐时间', key: 'time', render: (row) => row.recommendTimeStr || '-' },
  { title: '股票', key: 'stock', render: (row) => `${row.stockName} ${row.stockCode}` },
  { title: '周期', key: 'periodDays', width: 70 },
  { title: '推荐价', key: 'recommendPrice', width: 90 },
  { title: '期末价', key: 'endPrice', width: 90 },
  { title: '收益%', key: 'returnPct', width: 90, render: (row) => h(NText, { type: row.returnPct >= 0 ? 'error' : 'success' }, { default: () => row.returnPct?.toFixed ? row.returnPct.toFixed(2) : row.returnPct }) },
  { title: '基准%', key: 'benchmarkPct', width: 80, render: (row) => row.benchmarkPct?.toFixed ? row.benchmarkPct.toFixed(2) : row.benchmarkPct },
  { title: '超额%', key: 'excessPct', width: 80, render: (row) => row.excessPct?.toFixed ? row.excessPct.toFixed(2) : row.excessPct },
  { title: '结果', key: 'outcome', width: 90, render: (row) => row.outcome === 'win' ? h(NTag, { size: 'tiny', type: 'error', bordered: false }, { default: () => '达标' }) : h(NTag, { size: 'tiny', type: 'success', bordered: false }, { default: () => '未达标' }) },
  { title: '技能', key: 'skillId', width: 140, ellipsis: { tooltip: true }, render: (row) => row.skillId || '—' },
]

function normalizeBacktestItem(it) {
  const bt = it.AiRecommendBacktest || it || {}
  const recommendId = bt.RecommendId ?? bt.recommendId
  const outcome = bt.Outcome ?? bt.outcome
  const stockName = bt.StockName ?? bt.stockName
  const stockCode = bt.StockCode ?? bt.stockCode
  const periodDays = bt.PeriodDays ?? bt.periodDays
  const recommendPrice = bt.RecommendPrice ?? bt.recommendPrice
  const endPrice = bt.EndPrice ?? bt.endPrice
  const returnPct = bt.ReturnPct ?? bt.returnPct
  const benchmarkPct = bt.BenchmarkPct ?? bt.benchmarkPct
  const excessPct = bt.ExcessPct ?? bt.excessPct
  const skillId = bt.SkillId ?? bt.skillId ?? ''
  return {
    recommendId, outcome,
    stockName, stockCode, periodDays, recommendPrice,
    endPrice, returnPct, benchmarkPct, excessPct, skillId,
    recommendTimeStr: it.recommendTimeStr || '',
  }
}

function loadBacktestStats() {
  statsLoading.value = true
  GetRecommendBacktestStats().then((res) => {
    backtestStatsRef.value = res || null
  }).catch(() => {
    backtestStatsRef.value = null
  }).finally(() => {
    statsLoading.value = false
  })
}

function loadBacktestList(p) {
  const page = p || backtestPageRef.value
  backtestPageRef.value = page
  backtestListLoading.value = true
  const tf = backtestTemplateFilter.value
  const f = backtestPromptFilter.value
  const sf = backtestSkillFilter.value
  let req
  if (tf) {
    req = ListRecommendBacktestByTemplate(page, backtestPageSizeRef.value, tf.templateId)
  } else if (sf) {
    req = ListRecommendBacktestBySkill(page, backtestPageSizeRef.value, sf.skillId)
  } else if (f) {
    req = ListRecommendBacktestByPrompt(page, backtestPageSizeRef.value, f.content, f.type)
  } else {
    req = ListRecommendBacktest(page, backtestPageSizeRef.value)
  }
  try {
    req.then((res) => {
      const list = res?.list || []
      backtestTotalRef.value = res?.total || 0
      backtestListRef.value = list.map(normalizeBacktestItem)
    }).catch((e) => {
      backtestListRef.value = []
      backtestTotalRef.value = 0
      notify.error({ content: '加载回测明细失败：' + (e?.message || e || '未知错误'), duration: 4000 })
    }).finally(() => {
      backtestListLoading.value = false
    })
  } catch (e) {
    backtestListRef.value = []
    backtestTotalRef.value = 0
    backtestListLoading.value = false
    notify.error({ content: '加载回测明细失败：' + (e?.message || e || '未知错误'), duration: 4000 })
  }
}

// 点击某条提示词统计，按该提示词过滤下方「最近回测明细」
function filterBacktestByPrompt(type, g) {
  if (!g || !g.content) {
    notify.warning({ content: '该提示词内容为空，无法过滤', duration: 2000 })
    return
  }
  backtestTemplateFilter.value = null
  backtestSkillFilter.value = null
  backtestPromptFilter.value = { type, content: g.content, label: g.name || g.content }
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 清除提示词过滤条件
function clearBacktestPromptFilter() {
  backtestPromptFilter.value = null
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 点击某条模板统计，按该模板过滤下方「最近回测明细」
function filterBacktestByTemplate(t) {
  if (!t) return
  backtestPromptFilter.value = null
  backtestSkillFilter.value = null
  backtestTemplateFilter.value = { templateId: t.templateId, label: t.templateName || ('模板#' + t.templateId) }
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 清除模板过滤条件
function clearBacktestTemplateFilter() {
  backtestTemplateFilter.value = null
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 点击某条技能统计，按该技能过滤下方「最近回测明细」
function filterBacktestBySkill(s) {
  if (!s || !s.name) return
  backtestPromptFilter.value = null
  backtestTemplateFilter.value = null
  backtestSkillFilter.value = { skillId: s.name, label: s.name }
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 清除技能过滤条件
function clearBacktestSkillFilter() {
  backtestSkillFilter.value = null
  backtestPageRef.value = 1
  loadBacktestList(1)
}

// 模板统计 CV 显示（-1 表示均值≈0 无效）
function fmtTemplateCV(cv) {
  if (cv === null || cv === undefined) return '-'
  if (cv < 0) return '—'
  return Number(cv).toFixed(2)
}

function runBacktest() {
  backtestLoading.value = true
  RunRecommendBacktest(5).then((res) => {
    notify.info({ content: res, duration: 4000 })
    backtestLoading.value = false
    loadBacktestStats()
    loadBacktestList(1)
  }).catch(() => {
    backtestLoading.value = false
  })
}

function refreshAll() {
  loadBacktestStats()
  loadBacktestList(1)
}

onMounted(() => {
  loadBacktestStats()
  loadBacktestList(1)
})
</script>
