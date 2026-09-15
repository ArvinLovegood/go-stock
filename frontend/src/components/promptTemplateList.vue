<script setup>
import {computed, h, onBeforeMount, onMounted, ref, reactive, nextTick} from 'vue'
import {
  GetPromptTemplateList,
  GetConfig,
  AddPromptTemplate,
  DeletePromptTemplate,
  UpdatePromptTemplate,
  GetPromptTemplateBacktestDetail
} from "../../wailsjs/go/main/App";
import { EventsEmit } from "../../wailsjs/runtime";
import {NButton, NInput, NTag, NText, NSwitch, useMessage, useNotification,useDialog, NModal, NCard, NForm, NFormItem, NSpace, NPopover, NTable, NTooltip, NStatistic, NGrid, NGridItem, NDivider, NGradientText, NAlert} from "naive-ui";
import * as echarts from 'echarts';
import { MdEditor, MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'

const notify = useNotification()
const message = useMessage()
const dialog = useDialog()
const editorDataRef = reactive({
  darkTheme: false
})
const editorTheme = ref('light')

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
      editorTheme.value = 'dark'
    }
  })
})

onMounted(() => {
  query({
    page: 1,
    pageSize: paginationReactive.pageSize
  }).then((data) => {
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.totalPages
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
})

const dataRef = ref([])
const loadingRef = ref(true)

const columnsRef = ref([
  {
    title: '模板名称',
    key: 'name',
    render(row) {
      if (row.type === '模型系统Prompt') {
        return h(NText, { type: "success" }, { default: () => row.name })
      }else{
        return h(NText, { type: "info" }, { default: () => row.name })
      }
    }
  },
  {
    title: '模板类型',
    key: 'type',
    render(row) {
      if (row.type === '模型系统Prompt') {
        return h(NTag, { type: "success" }, { default: () => row.type })
      }else{
        return h(NTag, { type: "info" }, { default: () => row.type })
      }
    }
  },
  {
    title: '创建时间',
    key: 'CreatedAt',
    render(row) {
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '更新时间',
    key: 'UpdatedAt',
    render(row) {
      return row.UpdatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '模板内容',
    key: 'content',
    width: 200,
    render(row) {
      return h(NPopover, {
        trigger: 'hover',
        placement: 'left',
        showArrow: true,
        style: 'max-width: 800px; max-height: 400px; overflow: hidden',
        scrollable: true
      }, {
        trigger: () => h('span', {
          style: 'display: inline-block; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer;'
        }, row.content),
        default: () => h(MdPreview, {
          style:'text-align: left;',
          modelValue: row.content,
          theme: editorTheme.value
        })
      })
    }
  },
  {
    title: '操作',
    width: 320,
    render(row) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-right: 5px',
            onClick: () => showEditModal(row)
          },
          { default: () => '编辑' }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            style: 'margin-right: 5px',
            onClick: () => showShareModal(row)
          },
          { default: () => '分享' }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            style: 'margin-right: 5px',
            onClick: () => showBacktestModal(row)
          },
          { default: () => '回测' }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: () => deletePromptTemplate(row.ID)
          },
          { default: () => '删除' }
        )
      ]
    }
  }
])

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const modalDataRef = reactive({
  visible: false,
  isEdit: false,
  formData: {
    ID: 0,
    name: '',
    type: '',
    content: ''
  }
})

const shareDataRef = reactive({
  visible: false,
  title: '',
  content: '',
  description: '',
  category: '',
  tags: '',
  isPublic: true,
  vipOnly: false,
  loading: false
})

const promptPlazaApiBase = ref('https://go-stock.sparkmemory.top/api')

function query({ page, pageSize = 10, name = "", type = "", content = "" }) {
  return new Promise((resolve) => {
    GetPromptTemplateList({
      "page": page,
      "pageSize": pageSize,
      "name": name,
      "type": type,
      "content": content
    }).then((res) => {
      resolve({
        data: res.list,
        total: res.total,
        page: res.page,
        totalPages: res.totalPages
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      name: searchFormRef.name,
      type: searchFormRef.type,
      content: searchFormRef.content
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.totalPages
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
const promptTypeOptions = [
  {label: "模型系统Prompt", value: '模型系统Prompt'},
  {label: "模型用户Prompt", value: '模型用户Prompt'},]
const searchFormRef = reactive({
  name: "",
  type: null,
  content: ""
})

function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: 1,
      pageSize: paginationReactive.pageSize,
      name: searchFormRef.name,
      type: searchFormRef.type,
      content: searchFormRef.content
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = data.page || 1
      paginationReactive.pageCount = data.totalPages
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}

function showAddModal() {
  modalDataRef.isEdit = false
  modalDataRef.formData = {
    ID: 0,
    name: '',
    type: '',
    content: ''
  }
  modalDataRef.visible = true
}

function showEditModal(row) {
  modalDataRef.isEdit = true
  modalDataRef.formData = {
    ID: row.ID,
    name: row.name,
    type: row.type,
    content: row.content
  }
  modalDataRef.visible = true
}

function savePromptTemplate() {
  if (!modalDataRef.formData.name || !modalDataRef.formData.type || !modalDataRef.formData.content) {
    message.warning('请填写完整信息' )
    return
  }

  const apiCall = modalDataRef.isEdit ? UpdatePromptTemplate : AddPromptTemplate
  apiCall(modalDataRef.formData).then((res) => {
    message.info( res )
    modalDataRef.visible = false
    handleSearch()
    EventsEmit('promptTemplatesChanged')
  })
}

function deletePromptTemplate(id) {

  dialog.warning({
    title: '提示',
    content: '确定要删除这个模板吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      DeletePromptTemplate(id).then((res) => {
        message.info( res )
        handleSearch()
        EventsEmit('promptTemplatesChanged')
      })
    }
  })
}

async function checkUserIsVip() {
  const token = localStorage.getItem('promptPlazaToken')
  if (!token) return false
  try {
    const resp = await fetch(promptPlazaApiBase.value + '/auth/me', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    const json = await resp.json()
    if (json.code === 0 && json.data) {
      const user = json.data
      if (user.vipLevel > 0 && user.vipExpireAt) {
        return new Date(user.vipExpireAt) > new Date()
      }
    }
  } catch (e) { /* ignore */ }
  return false
}

async function showShareModal(row) {
  shareDataRef.title = row.name || ''
  shareDataRef.content = row.content || ''
  shareDataRef.description = ''
  shareDataRef.category = row.type || ''
  shareDataRef.tags = ''
  shareDataRef.isPublic = true
  shareDataRef.vipOnly = false
  shareDataRef.visible = true
  await GetConfig().then(result => {
    if (result.promptPlazaApiBase) {
      promptPlazaApiBase.value = result.promptPlazaApiBase
    }
  })
  const isVip = await checkUserIsVip()
  if (isVip) {
    shareDataRef.vipOnly = true
  }
}

async function handleShare() {
  if (!shareDataRef.title || !shareDataRef.content) {
    message.warning('标题和内容不能为空')
    return
  }
  const token = localStorage.getItem('promptPlazaToken')
  if (!token) {
    message.warning('请先在"提示词广场"登录后再分享')
    return
  }
  shareDataRef.loading = true
  try {
    const resp = await fetch(promptPlazaApiBase.value + '/prompts', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        title: shareDataRef.title,
        content: shareDataRef.content,
        description: shareDataRef.description,
        category: shareDataRef.category,
        tags: shareDataRef.tags,
        isPublic: shareDataRef.isPublic,
        vipOnly: shareDataRef.vipOnly
      })
    })
    const json = await resp.json()
    if (json.code !== 0) {
      if (json.code === 401) {
        message.error('登录已过期，请先在"提示词广场"重新登录')
      } else {
        message.error('分享失败: ' + (json.message || '未知错误'))
      }
      return
    }
    message.success('分享成功！')
    shareDataRef.visible = false
  } catch (e) {
    message.error('分享失败: ' + e.message)
  } finally {
    shareDataRef.loading = false
  }
}

// ---- 提示词模板回测（基于 AI 推荐记录推荐后 N 日实际表现） ----

const backtestModalRef = reactive({
  visible: false,
  loading: false,
  templateId: 0,
  templateName: '',
  stat: null
})
let backtestChart = null

function fmtPct(v) {
  return (v === null || v === undefined || Number.isNaN(v)) ? '-' : Number(v).toFixed(2) + '%'
}

function fmtCV(cv) {
  if (cv === null || cv === undefined) return '-'
  if (cv < 0) return '—（均值≈0）'
  return Number(cv).toFixed(2)
}

function showBacktestModal(row) {
  backtestModalRef.visible = true
  backtestModalRef.loading = true
  backtestModalRef.templateId = row.ID
  backtestModalRef.templateName = row.name
  backtestModalRef.stat = null
  GetPromptTemplateBacktestDetail(row.ID).then((stat) => {
    backtestModalRef.stat = stat || null
    backtestModalRef.loading = false
    nextTick(() => renderBacktestChart(stat))
  }).catch((e) => {
    backtestModalRef.loading = false
    notify.error({ content: '获取回测数据失败：' + (e?.message || e || '未知错误'), duration: 4000 })
  })
}

function renderBacktestChart(stat) {
  const el = document.getElementById('promptBacktestEquityChart')
  if (!el || !stat || !stat.curve || !stat.curve.length) return
  if (backtestChart) {
    backtestChart.dispose()
    backtestChart = null
  }
  backtestChart = echarts.init(el)
  const dates = stat.curve.map((p) => p.date)
  const equity = stat.curve.map((p) => p.equity)
  const up = (stat.cumReturn || 0) >= 0
  const lineColor = up ? 'rgba(207, 48, 48, 1)' : 'rgba(24, 160, 88, 1)'
  backtestChart.setOption({
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v) => '净值 ' + Number(v).toFixed(4)
    },
    grid: { top: 24, left: 56, right: 24, bottom: 28 },
    xAxis: { type: 'category', data: dates },
    yAxis: { type: 'value', scale: true },
    series: [
      {
        name: '等权组合净值',
        data: equity,
        type: 'line',
        showSymbol: false,
        lineStyle: { color: lineColor },
        markLine: {
          symbol: 'none',
          silent: true,
          label: { formatter: '1.0' },
          lineStyle: { color: '#999', type: 'dashed' },
          data: [{ yAxis: 1 }]
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: up ? 'rgba(207, 48, 48, 0.35)' : 'rgba(24, 160, 88, 0.35)' },
            { offset: 1, color: up ? 'rgba(207, 48, 48, 0.02)' : 'rgba(24, 160, 88, 0.02)' }
          ])
        }
      }
    ]
  })
}
</script>

<template>
  <div>
    <!-- 搜索区域 -->
    <n-space vertical style="margin-bottom: 16px">
      <n-space>
        <n-input v-model:value="searchFormRef.name" placeholder="模板名称" clearable />
        <n-select style="width: 200px" v-model:value="searchFormRef.type" :options="promptTypeOptions" placeholder="请选择提示词类型" clearable/>
        <n-input v-model:value="searchFormRef.content" placeholder="内容关键词" clearable />
        <n-button type="success" @click="handleSearch">搜索</n-button>
        <n-button type="warning" @click="showAddModal">新增模板</n-button>
      </n-space>
    </n-space>

    <!-- 数据表格 -->
    <n-data-table
      remote
      size="small"
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="paginationReactive"
      :row-key="(rowData) => rowData.ID"
      @update:page="handlePageChange"
      flex-height
      style="height: calc(100vh - 250px)"
    />

    <!-- 编辑/新增模态框 -->
    <n-modal v-model:show="modalDataRef.visible" preset="card" style="width: 1100px;text-align: left" :title="modalDataRef.formData.ID>0?'修改':'新增'+'Prompt模板'">
      <n-form :model="modalDataRef.formData" label-placement="left" label-width="80">
        <n-form-item label="模板名称" required>
          <n-input v-model:value="modalDataRef.formData.name" placeholder="请输入模板名称" />
        </n-form-item>
        <n-form-item label="模板类型" required>
          <n-select v-model:value="modalDataRef.formData.type" :options="promptTypeOptions" placeholder="请选择提示词类型"/>
        </n-form-item>
        <n-form-item label="模板内容" required>
          <MdEditor
            v-model="modalDataRef.formData.content"
            style="height: 400px"
            :theme="editorTheme"
            :preview="true"
            :toolbarsExclude="['github', 'htmlPreview', 'catalog', 'save']"
            placeholder="请输入模板内容"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="modalDataRef.visible = false">取消</n-button>
          <n-button type="primary" @click="savePromptTemplate">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 提示词模板回测弹窗 -->
    <n-modal v-model:show="backtestModalRef.visible" preset="card" style="width: 960px;text-align: left" :title="'回测表现：' + backtestModalRef.templateName">
      <div v-if="backtestModalRef.loading" style="padding: 40px; text-align: center;">
        <n-text depth="3">正在统计回测数据…</n-text>
      </div>
      <template v-else-if="backtestModalRef.stat && backtestModalRef.stat.total > 0">
        <n-alert type="warning" v-if="backtestModalRef.stat.total < 10" style="margin-bottom: 12px;">
          样本量不足（{{ backtestModalRef.stat.total }} 条），统计指标波动较大，仅供参考。
        </n-alert>
        <n-grid :cols="4" :x-gap="12" style="margin-bottom: 12px;">
          <n-grid-item><n-statistic label="综合评分" :value="backtestModalRef.stat.score ?? 0"><template #suffix>/100</template></n-statistic></n-grid-item>
          <n-grid-item><n-statistic label="已回测推荐" :value="backtestModalRef.stat.total" /></n-grid-item>
          <n-grid-item><n-statistic label="超额胜率" :value="backtestModalRef.stat.excessWinRate ? backtestModalRef.stat.excessWinRate.toFixed(1) : 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="绝对胜率" :value="backtestModalRef.stat.winRate ? backtestModalRef.stat.winRate.toFixed(1) : 0" suffix="%" /></n-grid-item>
        </n-grid>
        <n-grid :cols="4" :x-gap="12" style="margin-bottom: 12px;">
          <n-grid-item><n-statistic label="平均收益率" :value="backtestModalRef.stat.avgReturn ?? 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="平均超额收益" :value="backtestModalRef.stat.avgExcess ?? 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="波动率(σ)" :value="backtestModalRef.stat.volatility ?? 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="稳定性CV" :value="fmtCV(backtestModalRef.stat.cv)" /></n-grid-item>
        </n-grid>
        <n-grid :cols="4" :x-gap="12" style="margin-bottom: 12px;">
          <n-grid-item><n-statistic label="收益中位数" :value="backtestModalRef.stat.medianReturn ?? 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="简版夏普" :value="backtestModalRef.stat.sharpe ?? 0" /></n-grid-item>
          <n-grid-item><n-statistic label="最大回撤" :value="backtestModalRef.stat.maxDrawdown ?? 0" suffix="%" /></n-grid-item>
          <n-grid-item><n-statistic label="累计收益" :value="backtestModalRef.stat.cumReturn ?? 0" suffix="%" /></n-grid-item>
        </n-grid>
        <n-divider title-placement="left"><n-gradient-text type="info">等权组合净值曲线（{{ backtestModalRef.stat.periodDays }}日周期，{{ backtestModalRef.stat.sampleCount }} 个样本，{{ backtestModalRef.stat.firstTime }} ~ {{ backtestModalRef.stat.lastTime }}）</n-gradient-text></n-divider>
        <div id="promptBacktestEquityChart" style="width: 100%; height: 260px;"></div>
        <n-text depth="3" style="font-size: 12px;">
          口径说明：每条推荐按"推荐日后 N 个交易日"计算收益（vs 沪深300 超额）；净值曲线按推荐日等权组合逐日复合。
          综合评分 = 超额胜率×40 + 收益分(tanh)×30 + 稳定分(1-CV)×30。样本在"AI推荐股票"页通过「执行回测」产生。
        </n-text>
      </template>
      <div v-else style="padding: 40px; text-align: center;">
        <n-text depth="3">该模板暂无回测数据。</n-text>
        <br/><br/>
        <n-text depth="3" style="font-size: 12px;">
          需先用此模板作为系统提示词产生 AI 推荐记录，再到「AI推荐股票」页点击「执行回测」后，此处才会出现统计。
        </n-text>
      </div>
    </n-modal>

    <n-modal v-model:show="shareDataRef.visible" preset="card" style="width: 700px;text-align: left" title="分享到提示词广场">
      <n-form :model="shareDataRef" label-placement="left" label-width="80">
        <n-form-item label="标题" required>
          <n-input v-model:value="shareDataRef.title" placeholder="提示词标题" />
        </n-form-item>
        <n-space :size="8">
          <n-form-item label="分类" label-placement="left" style="width: 300px">
            <n-input v-model:value="shareDataRef.category" placeholder="如: AI编程, 数据分析" />
          </n-form-item>
          <n-form-item label="标签" label-placement="left" style="width: 300px">
            <n-input v-model:value="shareDataRef.tags" placeholder="逗号分隔" />
          </n-form-item>
        </n-space>
        <n-form-item label="描述">
          <n-input v-model:value="shareDataRef.description" type="textarea" :rows="2" placeholder="简短描述提示词用途" />
        </n-form-item>
        <n-form-item label="内容" required>
          <n-input v-model:value="shareDataRef.content" type="textarea" :rows="6" placeholder="提示词内容" />
        </n-form-item>
        <n-form-item label="公开">
          <n-space align="center">
            <n-switch v-model:value="shareDataRef.isPublic" />
            <n-divider vertical />
            <n-text>VIP专属</n-text>
            <n-switch v-model:value="shareDataRef.vipOnly" />
            <n-text depth="3" style="font-size: 12px">仅VIP用户可查看完整内容</n-text>
          </n-space>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="shareDataRef.visible = false">取消</n-button>
          <n-button type="primary" :loading="shareDataRef.loading" @click="handleShare">分享</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
:deep(.md-editor) {
  text-align: left;
}
:deep(.n-popover .md-editor-preview) {
  padding: 8px 12px;
}
:deep(.n-popover .md-editor-preview-wrapper) {
  padding: 0;
}
</style>
