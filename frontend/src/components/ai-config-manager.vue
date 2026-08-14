<script setup>
import {computed, h, onMounted, ref} from "vue";
import {useRouter} from "vue-router";
import {GetAiConfigs, UpdateAiConfigs, FetchAiModels, FetchAiModelInfo} from "../../wailsjs/go/main/App";
import {NButton, NSpace, NTag, useMessage} from "naive-ui";
import {data} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {ChevronLeftIcon, HelpCircleFilledIcon} from "tdesign-icons-vue-next";

const message = useMessage()
const router = useRouter()

// 返回基础设置页面
function goBackToSettings() {
  router.push({name: 'settings'})
}

// AI 配置列表（独立加载，不依赖 settings.vue 的 formValue）
const aiConfigs = ref([])
const saving = ref(false)

// 搜索关键字
const searchKeyword = ref('')
// 分页配置
const pagination = ref({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [5, 10, 20, 50],
  itemCount: 0,
  prefix: ({itemCount}) => `共 ${itemCount} 条`
})

// 抽屉编辑相关
const drawerVisible = ref(false)
const editingIndex = ref(-1) // -1 表示新增/复制
const editingConfig = ref(null)
// 抽屉模式：add 新增 / copy 复制 / edit 编辑
const drawerMode = ref('add')

// 自定义 Header 键值对编辑器
const headerPairs = ref([])
// 模板变量快捷选项
const headerVarOptions = [
  { label: '{{sessionId}} — 会话ID（空时自动生成UUID）', value: '{{sessionId}}' },
  { label: '{{uuid}} — 每次请求生成新UUID', value: '{{uuid}}' },
]

function syncHeaderPairsFromConfig() {
  const raw = editingConfig.value?.extraHeaders
  if (!raw) { headerPairs.value = []; return }
  try {
    const obj = JSON.parse(raw)
    headerPairs.value = Object.entries(obj).map(([k, v]) => ({ key: k, value: v }))
  } catch {
    headerPairs.value = []
  }
}

function syncHeaderPairsToConfig() {
  const pairs = headerPairs.value.filter(p => p.key && p.key.trim())
  if (!pairs.length) { editingConfig.value.extraHeaders = ''; return }
  const obj = {}
  pairs.forEach(p => { obj[p.key.trim()] = p.value || '' })
  editingConfig.value.extraHeaders = JSON.stringify(obj)
}

function addHeaderPair() {
  headerPairs.value.push({ key: '', value: '' })
}

function removeHeaderPair(index) {
  headerPairs.value.splice(index, 1)
}

// 一键填充腾讯云 CodeBuddy Proxy 常用 Header
function fillCodeBuddyHeaders() {
  headerPairs.value = [
    { key: 'x-team-id', value: '' },
    { key: 'x-agent-id', value: '' },
    { key: 'x-task-id', value: '{{uuid}}' },
    { key: 'x-conversation-id', value: '{{sessionId}}' },
  ]
}
// 抽屉标题（根据模式动态切换）
const drawerTitle = computed(() => {
  if (drawerMode.value === 'copy') return '复制新建AI配置'
  if (drawerMode.value === 'edit') return '编辑AI配置'
  return '添加AI配置'
})

// 经过搜索过滤后的列表
const filteredConfigs = computed(() => {
  const keyword = (searchKeyword.value || '').trim().toLowerCase()
  if (!keyword) return aiConfigs.value
  return aiConfigs.value.filter(c => {
    const name = (c.name || '').toLowerCase()
    const modelName = (c.modelName || '').toLowerCase()
    const baseUrl = (c.baseUrl || '').toLowerCase()
    const platformName = getPlatformName(c.baseUrl).toLowerCase()
    // 平台 label 形如 "DeepSeek (https://api.deepseek.com)"，匹配整串以支持中文别名（如"智谱AI"、"火山引擎"）
    const platformOpt = aiPlatformOptions.find(opt => opt.value === c.baseUrl)
    const platformLabel = platformOpt ? platformOpt.label.toLowerCase() : ''
    return name.includes(keyword)
      || modelName.includes(keyword)
      || baseUrl.includes(keyword)
      || platformName.includes(keyword)
      || platformLabel.includes(keyword)
  })
})

// n-data-table 所需的 data（带 key）
const tableData = computed(() => {
  return filteredConfigs.value.map((c, idx) => ({
    ...c,
    _key: c.ID || idx,
    _index: idx
  }))
})

// 平台选项
const aiPlatformOptions = [
  {label: 'DeepSeek (https://api.deepseek.com)', value: 'https://api.deepseek.com'},
  {label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1'},
  {label: '智谱AI(GLM) (https://open.bigmodel.cn/api/paas/v4)', value: 'https://open.bigmodel.cn/api/paas/v4'},
  {label: '智谱GLM Coding Plan (https://open.bigmodel.cn/api/coding/paas/v4)', value: 'https://open.bigmodel.cn/api/coding/paas/v4'},
  {label: '字节豆包(火山引擎) (https://ark.cn-beijing.volces.com/api/v3)', value: 'https://ark.cn-beijing.volces.com/api/v3'},
  {label: '火山引擎Ark Plan (https://ark.cn-beijing.volces.com/api/plan/v3)', value: 'https://ark.cn-beijing.volces.com/api/plan/v3'},
  {label: '火山引擎Ark Coding (https://ark.cn-beijing.volces.com/api/coding/v3)', value: 'https://ark.cn-beijing.volces.com/api/coding/v3'},
  {label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1'},
  {label: '阿里云百炼 Token Plan 团队版 (https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1)', value: 'https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1'},
  {label: '阿里云百炼 Coding Plan (https://coding.dashscope.aliyuncs.com/v1)', value: 'https://coding.dashscope.aliyuncs.com/v1'},
  {label: 'Moonshot(月之暗面) (https://api.moonshot.cn/v1)', value: 'https://api.moonshot.cn/v1'},
  {label: '腾讯混元 (https://api.hunyuan.cloud.tencent.com/v1)', value: 'https://api.hunyuan.cloud.tencent.com/v1'},
  {label: '讯飞星火 (https://spark-api-open.xf-yun.com/v1)', value: 'https://spark-api-open.xf-yun.com/v1'},
  {label: '零一万物 (https://api.lingyiwanwu.com/v1)', value: 'https://api.lingyiwanwu.com/v1'},
  {label: 'MiniMax (https://api.minimax.chat/v1)', value: 'https://api.minimax.chat/v1'},
  {label: '小米MiMo TokenPlan (https://token-plan-cn.xiaomimimo.com/v1)', value: 'https://token-plan-cn.xiaomimimo.com/v1'},
  {label: '小米MiMo (https://api.xiaomimimo.com/v1)', value: 'https://api.xiaomimimo.com/v1'},
  {label: '腾讯云TokenHub (https://tokenhub.tencentmaas.com/v1)', value: 'https://tokenhub.tencentmaas.com/v1'},
  {label: '腾讯云Token Plan 个人版 (https://api.lkeap.cloud.tencent.com/plan/v3)', value: 'https://api.lkeap.cloud.tencent.com/plan/v3'},
  {label: '腾讯云Coding Plan (https://api.lkeap.cloud.tencent.com/coding/v3)', value: 'https://api.lkeap.cloud.tencent.com/coding/v3'},
  {label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1'},
  {label: 'Azure OpenAI (https://YOUR_RESOURCE.openai.azure.com)', value: 'https://YOUR_RESOURCE.openai.azure.com'},
  {label: 'OpenRouter (https://openrouter.ai/api/v1)', value: 'https://openrouter.ai/api/v1'},
  {label: 'Ollama (http://localhost:11434/v1)', value: 'http://localhost:11434/v1'},
]

function getPlatformName(baseUrl) {
  if (!baseUrl) return ''
  const platform = aiPlatformOptions.find(opt => opt.value === baseUrl)
  if (platform) {
    const idx = platform.label.indexOf(' (')
    return idx > 0 ? platform.label.substring(0, idx) : platform.label
  }
  return ''
}

function onBaseUrlChange(aiConfig, newBaseUrl) {
  const platformName = getPlatformName(newBaseUrl)
  if (platformName && aiConfig.name && !aiConfig.name.startsWith(platformName)) {
    aiConfig.name = platformName + '-' + aiConfig.name
  } else if (platformName && !aiConfig.name) {
    aiConfig.name = platformName
  }
}

function onModelNameChange(aiConfig, newModelName) {
  if (!newModelName) return
  const platformName = getPlatformName(aiConfig.baseUrl)
  const baseName = platformName || 'AI'

  if (!aiConfig.name) {
    aiConfig.name = baseName + '-' + newModelName
  } else if (aiConfig.name === platformName) {
    aiConfig.name = platformName + '-' + newModelName
  } else {
    const parts = aiConfig.name.split('-')
    if (parts.length >= 2 && parts[0] === platformName) {
      parts[parts.length - 1] = newModelName
      aiConfig.name = parts.join('-')
    } else if (!aiConfig.name.endsWith(newModelName)) {
      aiConfig.name = aiConfig.name + '-' + newModelName
    }
  }

  fetchModelInfo(aiConfig, newModelName)
}

async function fetchModelInfo(aiConfig, modelName) {
  if (!modelName || !aiConfig.baseUrl) return
  try {
    const info = await FetchAiModelInfo(aiConfig.baseUrl, aiConfig.apiKey || '', modelName, aiConfig.extraHeaders || '')
    if (info) {
      const sourceLabel = info.source === 'api' ? 'API' : '内置数据'
      const parts = []
      if (info.contextWindow > 0) {
        aiConfig.contextWindow = info.contextWindow
        parts.push(`上下文窗口=${info.contextWindow}`)
      }
      if (info.maxTokens > 0) {
        aiConfig.maxTokens = info.maxTokens
        parts.push(`输出上限=${info.maxTokens}`)
      }
      if (parts.length) {
        message.success(`已自动设置 ${modelName} 的 ${parts.join('，')}（来源：${sourceLabel}）`)
      }
    }
  } catch (e) {
    console.error('FetchAiModelInfo error', e)
  }
}

async function fetchAiModels(aiConfig) {
  if (!aiConfig.baseUrl || !aiConfig.apiKey) {
    message.warning('请先填写接口地址和 apiKey')
    return
  }
  if (aiConfig._loadingModels) {
    return
  }
  aiConfig._loadingModels = true
  try {
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey, aiConfig.extraHeaders || '')
    const options = (list || []).map(id => ({label: id, value: id}))
    aiConfig._modelOptions = options
    if (!aiConfig.modelName && options.length > 0) {
      aiConfig.modelName = options[0].value
      onModelNameChange(aiConfig, aiConfig.modelName)
    }
    if (!options.length) {
      message.warning('未从接口获取到可用模型，请检查地址和 apiKey')
    }
  } catch (e) {
    console.error('FetchAiModels error', e)
    message.error('获取模型列表失败，请检查接口地址和 apiKey')
  } finally {
    aiConfig._loadingModels = false
  }
}

// 打开新增抽屉
function openAddDrawer() {
  editingIndex.value = -1
  drawerMode.value = 'add'
  editingConfig.value = new data.AIConfig({
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    modelType: 'chat',
    temperature: 0.1,
    maxTokens: 8192,
    contextWindow: 0,
    timeOut: 6000,
    httpProxy: "",
    httpProxyEnabled: false,
    thinking: true,
    extraHeaders: "",
  })
  syncHeaderPairsFromConfig()
  drawerVisible.value = true
}

// 基于已有配置快速复制新建（清空 ID，名称加「-副本」后缀）
function openCopyDrawer(row) {
  editingIndex.value = -1
  drawerMode.value = 'copy'
  const copy = JSON.parse(JSON.stringify(row))
  copy.ID = 0
  copy.name = (copy.name || '') + '-副本'
  // 清空模型列表缓存，避免沿用旧配置的模型选项
  delete copy._modelOptions
  delete copy._loadingModels
  editingConfig.value = copy
  syncHeaderPairsFromConfig()
  drawerVisible.value = true
}

// 打开编辑抽屉
function openEditDrawer(row) {
  editingIndex.value = row._index
  drawerMode.value = 'edit'
  // 深拷贝避免直接修改原对象
  editingConfig.value = JSON.parse(JSON.stringify(row))
  syncHeaderPairsFromConfig()
  drawerVisible.value = true
}

// 保存抽屉中的配置（写入列表，不立即调后端）
function applyDrawerConfig() {
  const c = editingConfig.value
  if (!c.name || !c.baseUrl || !c.apiKey || !c.modelName) {
    message.warning('名称/接口地址/apiKey/模型名称未填写完整')
    return
  }
  syncHeaderPairsToConfig()
  if (editingIndex.value === -1) {
    aiConfigs.value.push(c)
  } else {
    aiConfigs.value[editingIndex.value] = c
  }
  drawerVisible.value = false
}

// 从列表中删除
function removeConfig(row) {
  const idx = aiConfigs.value.findIndex(c => (c.ID && c.ID === row.ID) || c === row)
  if (idx >= 0) {
    aiConfigs.value.splice(idx, 1)
  }
}

// 表格列定义
const columns = [
  {title: '配置名称', key: 'name', resizable: true, minWidth: 160},
  {
    title: '接口平台',
    key: 'baseUrl',
    resizable: true,
    minWidth: 180,
    render(row) {
      const name = getPlatformName(row.baseUrl)
      return h('span', name || row.baseUrl)
    }
  },
  {title: '模型', key: 'modelName', resizable: true, minWidth: 160},
  {
    title: '深度思考',
    key: 'thinking',
    width: 100,
    align: 'center',
    render(row) {
      return h(NTag, {type: row.thinking ? 'success' : 'default', size: 'small', bordered: false},
        () => row.thinking ? '开启' : '关闭')
    }
  },
  {
    title: 'http代理',
    key: 'httpProxyEnabled',
    width: 100,
    align: 'center',
    render(row) {
      return h(NTag, {type: row.httpProxyEnabled ? 'warning' : 'default', size: 'small', bordered: false},
        () => row.httpProxyEnabled ? '开启' : '关闭')
    }
  },
  {
    title: 'MaxTokens',
    key: 'maxTokens',
    width: 110,
    align: 'right'
  },
  {
    title: '上下文窗口',
    key: 'contextWindow',
    width: 120,
    align: 'right',
    render(row) {
      return row.contextWindow > 0 ? row.contextWindow : h(NTag, {type: 'default', size: 'small', bordered: false}, () => '自动')
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    fixed: 'right',
    render(row) {
      return h(NSpace, {size: 4}, () => [
        h(NButton, {
          size: 'small',
          type: 'primary',
          ghost: true,
          onClick: () => openEditDrawer(row)
        }, () => '编辑'),
        h(NButton, {
          size: 'small',
          type: 'info',
          ghost: true,
          onClick: () => openCopyDrawer(row)
        }, () => '复制'),
        h(NButton, {
          size: 'small',
          type: 'error',
          ghost: true,
          onClick: () => removeConfig(row)
        }, () => '删除')
      ])
    }
  }
]

// 行 key
function rowKey(row) {
  return row._key
}

// 搜索时回到第一页
function onSearchInput() {
  pagination.value.page = 1
}

// 分页变化
function onPageChange(page) {
  pagination.value.page = page
}

function onPageSizeChange(pageSize) {
  pagination.value.pageSize = pageSize
  pagination.value.page = 1
}

// 保存全部配置到后端
function saveAiConfigs() {
  if (saving.value) return
  saving.value = true
  UpdateAiConfigs(aiConfigs.value).then(res => {
    if (res === '保存成功！') {
      message.success(res)
      EventsEmit("updateSettings")
    } else {
      message.error(res)
    }
  }).catch(e => {
    message.error('保存失败：' + e)
  }).finally(() => {
    saving.value = false
  })
}

function loadAiConfigs() {
  GetAiConfigs().then(res => {
    aiConfigs.value = res || []
  })
}

onMounted(() => {
  loadAiConfigs()
})
</script>

<template>
  <n-flex justify="left" style="text-align: left; --wails-draggable:no-drag">
    <n-form :label-placement="'left'" :label-align="'left'" style="width: 100%;">
      <n-space vertical size="large" style="width: 100%;">
        <n-card size="small">
          <template #header>
            <n-space align="center" size="small">
              <n-button quaternary circle size="tiny" @click="goBackToSettings" title="返回基础设置">
                <template #icon>
                  <n-icon><ChevronLeftIcon/></n-icon>
                </template>
              </n-button>
              <n-tag type="primary" :bordered="false">AI模型服务配置</n-tag>
            </n-space>
          </template>
          <template #header-extra>
            <n-space>
              <n-button type="primary" dashed @click="openAddDrawer">+ 添加AI配置</n-button>
              <n-button type="primary" strong :loading="saving" @click="saveAiConfigs">保存配置</n-button>
            </n-space>
          </template>
          <n-space vertical size="medium">
            <n-text depth="3" style="font-size: 12px;">
              管理 AI 模型服务接口配置（可添加多个，用于 AI 诊股、飞书机器人、AI 助手等功能）。支持按名称/模型/接口地址搜索，分页展示。修改后请点击「保存配置」。
            </n-text>
            <n-input
              v-model:value="searchKeyword"
              placeholder="搜索：配置名称 / 平台 / 模型名称 / 接口地址"
              clearable
              @update:value="onSearchInput"
              style="max-width: 480px;"
            />
            <n-data-table
              :columns="columns"
              :data="tableData"
              :row-key="rowKey"
              :pagination="pagination"
              :bordered="false"
              :single-line="false"
              size="small"
              style="height: calc(100vh - 280px);"
              @update:page="onPageChange"
              @update:page-size="onPageSizeChange"
            />
          </n-space>
        </n-card>
      </n-space>
    </n-form>

    <!-- 编辑/新增抽屉 -->
    <n-drawer v-model:show="drawerVisible" :width="640" placement="right">
      <n-drawer-content :title="drawerTitle" closable>
        <n-form v-if="editingConfig" :label-placement="'left'" :label-width="120">
          <n-form-item label="配置名称" required>
            <n-input v-model:value="editingConfig.name" placeholder="配置名称" clearable/>
          </n-form-item>
          <n-form-item label="接口地址" required>
            <n-select
              v-model:value="editingConfig.baseUrl"
              :options="aiPlatformOptions"
              filterable
              tag
              clearable
              placeholder="选择或输入AI接口地址"
              @update:value="(val) => onBaseUrlChange(editingConfig, val)"
            />
          </n-form-item>
          <n-form-item label="令牌(apiKey)" required>
            <n-input type="password" v-model:value="editingConfig.apiKey" placeholder="apiKey"
                     clearable show-password-on="click"/>
          </n-form-item>
          <n-form-item label="模型类型" required>
            <n-radio-group v-model:value="editingConfig.modelType">
              <n-radio-button value="chat">文本对话</n-radio-button>
              <n-radio-button value="embedding">向量模型</n-radio-button>
            </n-radio-group>
            <n-tooltip placement="top">
              <template #trigger>
                <n-icon color="#0e7a0d" size="18" style="margin-left: 6px; cursor: help">
                  <HelpCircleFilledIcon/>
                </n-icon>
              </template>
              <n-gradient-text :type="'warning'">
                <div style="max-width: 380px;text-align: left">
                  同一提供商的对话与向量接口地址可能不同，故拆分为独立条目。<br>
                  <b>文本对话</b>：用于 Agent 对话（/v1/chat/completions）<br>
                  <b>向量模型</b>：用于知识库向量检索（/v1/embeddings），模型名称即向量模型名
                </div>
              </n-gradient-text>
            </n-tooltip>
          </n-form-item>
          <n-form-item :label="editingConfig.modelType === 'embedding' ? '向量模型名称' : '模型名称'" required>
            <n-select
              v-model:value="editingConfig.modelName"
              :options="editingConfig._modelOptions || []"
              filterable
              tag
              :loading="editingConfig._loadingModels"
              :placeholder="editingConfig.modelType === 'embedding' ? '向量模型名，如 text-embedding-3-small / BAAI/bge-m3' : '点击获取模型列表或手动输入'"
              @click="fetchAiModels(editingConfig)"
              @update:value="(val) => onModelNameChange(editingConfig, val)"
            />
          </n-form-item>

          <n-form-item label="Temperature">
            <n-input-number v-model:value="editingConfig.temperature" :step="0.1" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="MaxTokens">
            <n-input-number v-model:value="editingConfig.maxTokens" style="width: 100%;"/>
          </n-form-item>
          <n-form-item>
            <template #label>
              <n-space align="center" :size="4">
                <span>上下文窗口</span>
                <n-tooltip placement="top">
                  <template #trigger>
                    <n-icon color="#0e7a0d" size="18" style="cursor: help">
                      <HelpCircleFilledIcon/>
                    </n-icon>
                  </template>
                  <n-gradient-text :type="'warning'">
                    <div style="max-width: 400px;text-align: left">
                      模型上下文窗口大小（输入+输出总容量）。由模型信息自动获取，也可手动填写。<br>
                      用于摘要中间件和消息压缩的 token 预算计算。<br>
                      留空(0)时按内置模型表/MaxTokens/默认值自动推导。
                    </div>
                  </n-gradient-text>
                </n-tooltip>
              </n-space>
            </template>
            <n-input-number v-model:value="editingConfig.contextWindow" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="Timeout(秒)">
            <n-input-number :min="60" :step="1" v-model:value="editingConfig.timeOut" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="深度思考">
            <n-space align="center">
              <n-switch v-model:value="editingConfig.thinking"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon/>
                  </n-icon>
                </template>
                <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    启用深度思考模式：<br>
                    适用于 DeepSeek-Reasoner、MiMo-V2.5-Pro 等支持推理的模型。<br>
                    如使用普通模型请关闭此选项
                  </div>
                </n-gradient-text>
              </n-tooltip>
            </n-space>
          </n-form-item>
          <n-form-item label="http代理">
            <n-switch v-model:value="editingConfig.httpProxyEnabled"/>
          </n-form-item>
          <n-form-item v-if="editingConfig.httpProxyEnabled" label="http代理地址">
            <n-input v-model:value="editingConfig.httpProxy" placeholder="http代理地址" clearable/>
          </n-form-item>
          <n-form-item label="自定义Header">
            <n-space vertical style="width: 100%" :size="8">
              <n-space align="center" :size="4">
                <n-button size="small" dashed @click="addHeaderPair">+ 添加Header</n-button>
                <n-button size="small" dashed @click="fillCodeBuddyHeaders" type="primary">
                  一键填充 CodeBuddy Proxy
                </n-button>
                <n-tooltip placement="top">
                  <template #trigger>
                    <n-icon color="#0e7a0d" size="20" style="cursor: help">
                      <HelpCircleFilledIcon/>
                    </n-icon>
                  </template>
                  <n-gradient-text :type="'warning'">
                    <div v-pre style="max-width: 400px;text-align: left">
                      自定义 HTTP 请求头，用于对接需携带额外 Header 的代理/网关（如腾讯云 CodeBuddy Proxy）。<br>
                      Authorization: Bearer 由 apiKey 字段自动提供，无需手动添加。<br>
                      值支持模板变量：<b>{{sessionId}}</b>（会话ID，空时自动生成UUID）、<b>{{uuid}}</b>（每次请求生成新UUID）
                    </div>
                  </n-gradient-text>
                </n-tooltip>
              </n-space>
              <div v-if="!headerPairs.length" style="color: #999; font-size: 13px; padding: 4px 0">
                暂无自定义 Header，点击上方按钮添加
              </div>
              <div v-for="(pair, index) in headerPairs" :key="index" style="display: flex; gap: 8px; align-items: center;">
                <n-input
                  v-model:value="pair.key"
                  placeholder="Header 名称（如 x-team-id）"
                  style="width: 180px; flex-shrink: 0"
                  size="small"
                />
                <n-input
                  v-model:value="pair.value"
                  placeholder="Header 值（可输入 {{sessionId}} 或 {{uuid}}）"
                  style="flex: 1"
                  size="small"
                />
                <n-button size="small" quaternary type="error" @click="removeHeaderPair(index)">删除</n-button>
              </div>
            </n-space>
          </n-form-item>
        </n-form>
        <template #footer>
          <n-space>
            <n-button @click="drawerVisible = false">取消</n-button>
            <n-button type="primary" @click="applyDrawerConfig">确定</n-button>
          </n-space>
        </template>
      </n-drawer-content>
    </n-drawer>
  </n-flex>
</template>

<style scoped>
</style>
