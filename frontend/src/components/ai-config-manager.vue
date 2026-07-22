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

const providerOptions = [
  {label: '自动识别（推荐）', value: 'auto'},
  {label: 'OpenAI / GPT', value: 'openai'},
  {label: 'xAI / Grok', value: 'xai'},
  {label: '通用 OpenAI 兼容', value: 'openai_compatible'},
]

const reasoningEffortOptions = [
  {label: '低（更快）', value: 'low'},
  {label: '中（推荐）', value: 'medium'},
  {label: '高（深度分析）', value: 'high'},
]

function supportsStandardReasoningEffort(modelName) {
  const model = (modelName || '').trim().toLowerCase()
  return model.startsWith('gpt-5') || model.startsWith('grok-4.5')
}

function applyRecommendedConfig(aiConfig) {
  const model = (aiConfig.modelName || '').toLowerCase()
  if (model.startsWith('gpt-5')) {
    aiConfig.provider = 'openai'
    aiConfig.maxInputTokens = 96000
    aiConfig.maxOutputTokens = 16384
    aiConfig.reasoningEffort = 'medium'
    aiConfig.temperatureOpt = null
    aiConfig.timeOut = 900
    aiConfig.thinking = true
  } else if (model.startsWith('grok-')) {
    aiConfig.provider = 'xai'
    aiConfig.maxInputTokens = 96000
    aiConfig.maxOutputTokens = 8192
    aiConfig.reasoningEffort = 'medium'
    aiConfig.temperatureOpt = 0.2
    aiConfig.timeOut = 600
    aiConfig.thinking = true
  } else {
    message.info('当前模型没有内置推荐值，请按服务商文档配置')
    return
  }
  message.success('已应用推荐参数，请确认后保存')
}

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
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey)
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

async function fetchModelInfo(aiConfig, modelName) {
  if (!modelName || !aiConfig.baseUrl) return
  try {
    const info = await FetchAiModelInfo(aiConfig.baseUrl, aiConfig.apiKey || '', modelName)
    if (info && info.maxTokens > 0) {
      aiConfig.maxInputTokens = info.maxTokens
      const sourceLabel = info.source === 'api' ? 'API' : '内置数据'
      message.success(`已自动设置 ${modelName} 的最大输入 Tokens 为 ${info.maxTokens}（来源：${sourceLabel}）`)
    }
  } catch (e) {
    console.error('FetchAiModelInfo error', e)
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
    provider: 'auto',
    temperature: 0.1,
    temperatureOpt: null,
    maxTokens: 8192,
    maxInputTokens: 0,
    maxOutputTokens: 0,
    reasoningEffort: '',
    timeOut: 6000,
    httpProxy: "",
    httpProxyEnabled: false,
    thinking: true,
  })
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
  drawerVisible.value = true
}

// 打开编辑抽屉
function openEditDrawer(row) {
  editingIndex.value = row._index
  drawerMode.value = 'edit'
  // 深拷贝避免直接修改原对象
  editingConfig.value = JSON.parse(JSON.stringify(row))
  drawerVisible.value = true
}

// 保存抽屉中的配置（写入列表，不立即调后端）
function applyDrawerConfig() {
  const c = editingConfig.value
  if (!c.name || !c.baseUrl || !c.apiKey || !c.modelName) {
    message.warning('名称/接口地址/apiKey/模型名称未填写完整')
    return
  }
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
    title: '输入 / 输出',
    key: 'maxInputTokens',
    width: 110,
    align: 'right',
    render(row) {
      return h('span', `${row.maxInputTokens || '兼容'} / ${row.maxOutputTokens || row.maxTokens || '-'}`)
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
    aiConfigs.value = (res || []).map(config => ({
      ...config,
      provider: config.provider || 'auto',
      maxInputTokens: config.maxInputTokens || 0,
      maxOutputTokens: config.maxOutputTokens || 0,
      temperatureOpt: config.temperatureOpt ?? null,
      reasoningEffort: config.reasoningEffort || (supportsStandardReasoningEffort(config.modelName) ? 'medium' : ''),
    }))
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
          <n-form-item label="模型名称" required>
            <n-select
              v-model:value="editingConfig.modelName"
              :options="editingConfig._modelOptions || []"
              filterable
              tag
              :loading="editingConfig._loadingModels"
              placeholder="点击获取模型列表或手动输入"
              @click="fetchAiModels(editingConfig)"
              @update:value="(val) => onModelNameChange(editingConfig, val)"
            />
          </n-form-item>
          <n-form-item label="参数模板">
            <n-space align="center">
              <n-button type="primary" dashed @click="applyRecommendedConfig(editingConfig)">应用模型推荐值</n-button>
              <n-text depth="3">GPT-5.6 Sol / Grok 4.5</n-text>
            </n-space>
          </n-form-item>
          <n-form-item label="Provider">
            <n-select v-model:value="editingConfig.provider" :options="providerOptions" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="Temperature">
            <n-input-number v-model:value="editingConfig.temperatureOpt" clearable :step="0.1" placeholder="留空表示不发送" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="最大输入Tokens">
            <n-input-number v-model:value="editingConfig.maxInputTokens" :min="0" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="最大输出Tokens">
            <n-input-number v-model:value="editingConfig.maxOutputTokens" :min="0" style="width: 100%;"/>
          </n-form-item>
          <n-form-item label="推理强度">
            <n-select v-model:value="editingConfig.reasoningEffort" :options="reasoningEffortOptions" clearable style="width: 100%;"/>
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
                    启用模型推理能力。GPT/Grok 会映射为标准 reasoning_effort，<br>
                    其他兼容服务保留原有 thinking 兼容逻辑。普通模型请关闭。
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
