<script setup>
import {h, onBeforeUnmount, onMounted, ref} from "vue";
import {
  AddPrompt,
  DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  SendDingDingMessageByType,
  SendFeishuMessageByType,
  UpdateConfig,
  CheckSponsorCode,
  FetchAiModels,
  FetchAiModelInfo
} from "../../wailsjs/go/main/App";
import {NTag, NTooltip, NIcon, useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";

const message = useMessage()

const formRef = ref(null)
const formValue = ref({
  ID: 1,
  tushareToken: '',
  iwencaiApiKey: '',
  emApiKey: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  feishuPush: {
    enable: false,
    feishuRobot: '',
    feishuSecret: ''
  },
  localPush: {
    enable: true,
  },
  updateBasicInfoOnStart: false,
  refreshInterval: 1,
  openAI: {
    enable: false,
    aiConfigs: [], // AI配置列表
    prompt: "",
    questionTemplate: "{{stockName}}分析和总结",
    crawlTimeOut: 30,
    kDays: 30,
    httpProxy:"",
    httpProxyEnabled:false,
  },
  enableDanmu: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: true,
  enableOnlyPushRedNews: true,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
  enableAgent: false,
  qgqpBId: '',
  updateChannel: 'release',
  promptPlazaApiBase: '',
})

// 添加一个新的AI配置到列表
function addAiConfig() {
  formValue.value.openAI.aiConfigs.push(new data.AIConfig({
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    temperature: 0.1,
    maxTokens: 8192,
    timeOut: 6000,
    httpProxy:"",
    httpProxyEnabled:false,
    thinking: true,
  }));
}

// 从列表中移除一个AI配置
function removeAiConfig(index) {
  const originalCount = formValue.value.openAI.aiConfigs.length;
  // 使用filter创建新数组确保响应式更新
  formValue.value.openAI.aiConfigs = formValue.value.openAI.aiConfigs.filter((_, i) => i !== index);
}

const updateChannelOptions = [
  { label: 'Release（稳定版）', value: 'release' },
  { label: 'Pre-release（预发布版）', value: 'pre' },
  { label: 'Dev（开发版）', value: 'dev' },
]

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
    const options = (list || []).map(id => ({ label: id, value: id }))
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


const promptTemplates = ref([])
const aiConfigExpandedNames = ref([])

const aiPlatformOptions = [
  { label: 'DeepSeek (https://api.deepseek.com)', value: 'https://api.deepseek.com' },
  { label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1' },
  { label: '智谱AI(GLM) (https://open.bigmodel.cn/api/paas/v4)', value: 'https://open.bigmodel.cn/api/paas/v4' },
  { label: '字节豆包(火山引擎) (https://ark.cn-beijing.volces.com/api/v3)', value: 'https://ark.cn-beijing.volces.com/api/v3' },
  { label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { label: 'Moonshot(月之暗面) (https://api.moonshot.cn/v1)', value: 'https://api.moonshot.cn/v1' },
  { label: '腾讯混元 (https://api.hunyuan.cloud.tencent.com/v1)', value: 'https://api.hunyuan.cloud.tencent.com/v1' },
  { label: '讯飞星火 (https://spark-api-open.xf-yun.com/v1)', value: 'https://spark-api-open.xf-yun.com/v1' },
  { label: '零一万物 (https://api.lingyiwanwu.com/v1)', value: 'https://api.lingyiwanwu.com/v1' },
  { label: 'MiniMax (https://api.minimax.chat/v1)', value: 'https://api.minimax.chat/v1' },
  { label: '小米MiMo TokenPlan (https://token-plan-cn.xiaomimimo.com/v1)', value: 'https://token-plan-cn.xiaomimimo.com/v1' },
  { label: '小米MiMo (https://api.xiaomimimo.com/v1)', value: 'https://api.xiaomimimo.com/v1' },
  { label: '腾讯云TokenHub (https://tokenhub.tencentmaas.com/v1)', value: 'https://tokenhub.tencentmaas.com/v1' },
  { label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1' },
  { label: 'Azure OpenAI (https://YOUR_RESOURCE.openai.azure.com)', value: 'https://YOUR_RESOURCE.openai.azure.com' },
  { label: 'OpenRouter (https://openrouter.ai/api/v1)', value: 'https://openrouter.ai/api/v1' },
  { label:'Ollama (http://localhost:11434/v1)', value: 'http://localhost:11434/v1' },
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
    const info = await FetchAiModelInfo(aiConfig.baseUrl, aiConfig.apiKey || '', modelName)
    if (info && info.maxTokens > 0) {
      aiConfig.maxTokens = info.maxTokens
      const sourceLabel = info.source === 'api' ? 'API' : '内置数据'
      message.success(`已自动设置 ${modelName} 的 MaxTokens 为 ${info.maxTokens}（来源：${sourceLabel}）`)
    }
  } catch (e) {
    console.error('FetchAiModelInfo error', e)
  }
}

onMounted(() => {
  GetConfig().then(res => {
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.iwencaiApiKey = res.iwencaiApiKey || ''
    formValue.value.emApiKey = res.emApiKey || ''
    formValue.value.dingPush = {
      enable: res.dingPushEnable,
      dingRobot: res.dingRobot
    }
    formValue.value.feishuPush = {
      enable: res.feishuPushEnable,
      feishuRobot: res.feishuRobot,
      feishuSecret: res.feishuSecret
    }
    formValue.value.localPush = {
      enable: res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval
    // 加载AI配置
    formValue.value.openAI = {
      enable: res.openAiEnable,
      aiConfigs: res.aiConfigs || [],
      prompt: res.prompt,
      questionTemplate: res.questionTemplate ? res.questionTemplate : '{{stockName}}分析和总结',
      crawlTimeOut: res.crawlTimeOut,
      kDays: res.kDays,
      httpProxy:"",
      httpProxyEnabled:false,
    }


    formValue.value.enableDanmu = res.enableDanmu
    formValue.value.browserPath = res.browserPath
    formValue.value.enableNews = res.enableNews
    formValue.value.darkTheme = res.darkTheme
    formValue.value.enableFund = res.enableFund
    formValue.value.enablePushNews = res.enablePushNews
    formValue.value.enableOnlyPushRedNews = res.enableOnlyPushRedNews
    formValue.value.sponsorCode = res.sponsorCode
    formValue.value.httpProxy=res.httpProxy;
    formValue.value.httpProxyEnabled=res.httpProxyEnabled;
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = res.qgqpBId;
    formValue.value.updateChannel = res.updateChannel || 'release';
    formValue.value.promptPlazaApiBase = res.promptPlazaApiBase || '';

  })

  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
  })
})
onBeforeUnmount(() => {
  message.destroyAll()
})

function saveConfig() {
  console.log('开始保存设置', formValue.value);
  // 构建配置时，包含aiConfigs列表
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    feishuPushEnable: formValue.value.feishuPush.enable,
    feishuRobot: formValue.value.feishuPush.feishuRobot,
    feishuSecret: formValue.value.feishuPush.feishuSecret,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: formValue.value.openAI.enable,
    aiConfigs: formValue.value.openAI.aiConfigs,
    // 序列化aiConfigs列表以传递给后端
    tushareToken: formValue.value.tushareToken,
    iwencaiApiKey: formValue.value.iwencaiApiKey,
    emApiKey: formValue.value.emApiKey,
    prompt: formValue.value.openAI.prompt,
    questionTemplate: formValue.value.openAI.questionTemplate,
    crawlTimeOut: formValue.value.openAI.crawlTimeOut,
    kDays: formValue.value.openAI.kDays,
    enableDanmu: formValue.value.enableDanmu,
    browserPath: formValue.value.browserPath,
    enableNews: formValue.value.enableNews,
    darkTheme: formValue.value.darkTheme,
    enableFund: formValue.value.enableFund,
    enablePushNews: formValue.value.enablePushNews,
    enableOnlyPushRedNews: formValue.value.enableOnlyPushRedNews,
    sponsorCode: formValue.value.sponsorCode,
    httpProxy:formValue.value.httpProxy,
    httpProxyEnabled:formValue.value.httpProxyEnabled,
    enableAgent: formValue.value.enableAgent,
    qgqpBId: formValue.value.qgqpBId,
    updateChannel: formValue.value.updateChannel,
    promptPlazaApiBase: formValue.value.promptPlazaApiBase,
  })

  if (config.sponsorCode) {
    CheckSponsorCode(config.sponsorCode).then(res => {
      if (!res.code) {
        message.warning(res.msg || '赞助码验证失败')
      }
    })
  }

  UpdateConfig(config).then(res => {
    if (res === '保存成功！') {
      message.success(res)
    } else {
      message.error(res)
    }
    EventsEmit("updateSettings", config);
  })
}


function getHeight() {
  return document.documentElement.clientHeight
}

function sendTestNotice() {
  let markdown = "### go-stock test\n" + new Date()
  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"go-stock' + new Date() + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'

  SendDingDingMessageByType(msg, "test-" + new Date().getTime(), 1).then(res => {
    message.info(res)
  })
}

function sendFeishuTestNotice() {
  let markdown = "### go-stock 飞书测试\n" + new Date()
  // 飞书卡片 JSON 2.0 协议：schema="2.0" + body.elements + markdown 元素
  // 文档：https://open.feishu.cn/document/feishu-cards/card-json-v2-components/content-components/rich-text
  let msg = JSON.stringify({
    msg_type: "interactive",
    card: {
      schema: "2.0",
      header: {
        title: {
          tag: "plain_text",
          content: "go-stock 飞书测试 " + new Date()
        }
      },
      body: {
        elements: [
          {
            tag: "markdown",
            content: '<at id=all></at>\n' + markdown
          }
        ]
      }
    }
  })

  SendFeishuMessageByType(msg, "test-feishu-" + new Date().getTime(), 1).then(res => {
    message.info(res)
  })
}

function exportConfig() {
  ExportConfig().then(res => {
    message.info(res)
  })
}

function importConfig() {
  let input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    let file = e.target.files[0];
    let reader = new FileReader();
    reader.onload = (e) => {
      let config = JSON.parse(e.target.result);
      formValue.value.ID = config.ID
      formValue.value.tushareToken = config.tushareToken
      formValue.value.iwencaiApiKey = config.iwencaiApiKey || ''
      formValue.value.emApiKey = config.emApiKey || ''
      formValue.value.dingPush = {
        enable: config.dingPushEnable,
        dingRobot: config.dingRobot
      }
      formValue.value.feishuPush = {
        enable: config.feishuPushEnable,
        feishuRobot: config.feishuRobot,
        feishuSecret: config.feishuSecret
      }
      formValue.value.localPush = {
        enable: config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      // 导入AI配置
      formValue.value.openAI = {
        enable: config.openAiEnable,
        aiConfigs: config.aiConfigs || [],
        prompt: config.prompt,
        questionTemplate: config.questionTemplate,
        crawlTimeOut: config.crawlTimeOut,
        kDays: config.kDays
      }
      formValue.value.enableDanmu = config.enableDanmu
      formValue.value.browserPath = config.browserPath
      formValue.value.enableNews = config.enableNews
      formValue.value.darkTheme = config.darkTheme
      formValue.value.enableFund = config.enableFund
      formValue.value.enablePushNews = config.enablePushNews
      formValue.value.enableOnlyPushRedNews = config.enableOnlyPushRedNews
      formValue.value.sponsorCode = config.sponsorCode
      formValue.value.httpProxy=config.httpProxy
      formValue.value.httpProxyEnabled=config.httpProxyEnabled
      formValue.value.enableAgent = config.enableAgent
      formValue.value.qgqpBId = config.qgqpBId
      formValue.value.updateChannel = config.updateChannel || 'release'
    };
    reader.readAsText(file);
  };
  input.click();
}


window.onerror = function (event, source, lineno, colno, error) {
  EventsEmit("frontendError", {
    page: "settings.vue",
    message: event,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null
  });
  return true;
};

const showManagePromptsModal = ref(false)
const promptTypeOptions = [
  {label: "模型系统Prompt", value: '模型系统Prompt'},
  {label: "模型用户Prompt", value: '模型用户Prompt'},]
const formPromptRef = ref(null)
const formPrompt = ref({
  ID: 0,
  Name: '',
  Content: '',
  Type: '',
})

function managePrompts() {
  formPrompt.value.ID = 0
  showManagePromptsModal.value = true
}

function savePrompt() {
  AddPrompt(formPrompt.value).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
    showManagePromptsModal.value = false
  })
}

function editPrompt(prompt) {
  formPrompt.value.ID = prompt.ID
  formPrompt.value.Name = prompt.name
  formPrompt.value.Content = prompt.content
  formPrompt.value.Type = prompt.type
  showManagePromptsModal.value = true
}

function deletePrompt(ID) {
  DelPrompt(ID).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
  })
}
</script>

<template>
  <n-flex justify="left" style="text-align: left; --wails-draggable:no-drag">
    <n-form ref="formRef" :label-placement="'left'" :label-align="'left'">
      <n-space vertical size="large">
        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '基础设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
<!--            <n-form-item-gi :span="10" label="Tushare Token：" path="tushareToken">
              <n-input type="text" placeholder="Tushare api token" v-model:value="formValue.tushareToken" clearable/>
            </n-form-item-gi>-->
            <n-form-item-gi :span="4" label="启动时更新基础信息：" path="updateBasicInfoOnStart">
              <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="数据刷新间隔：" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="请输入数据刷新间隔(秒)">
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="暗黑主题：" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="更新通道：" path="updateChannel">
              <n-select v-model:value="formValue.updateChannel" :options="updateChannelOptions" />
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    更新通道说明：<br>
                    <b>Release（稳定版）</b>：仅接收正式发布版本，稳定性最高<br>
                    <b>Pre-release（预发布版）</b>：包含预发布版本，可提前体验新功能<br>
                    <b>Dev（开发版）</b>：包含所有可用版本，获取最新开发进度
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="浏览器安装路径：" path="browserPath">
              <n-input type="text" placeholder="浏览器安装路径" v-model:value="formValue.browserPath" clearable/>
            </n-form-item-gi>
           <n-form-item-gi :span="3" label="指数基金：" path="enableFund">
              <n-switch v-model:value="formValue.enableFund"/>
            </n-form-item-gi>
            <!--      <n-form-item-gi :span="3" label="AI智能体：" path="enableAgent">
                   <n-switch v-model:value="formValue.enableAgent"/>
                 </n-form-item-gi>-->
            <n-form-item-gi :span="11" label="东财唯一标识：" path="qgqpBId">
              <n-input type="text" placeholder="东财唯一标识" v-model:value="formValue.qgqpBId" clearable/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    打开浏览器,访问东财网站，<br>
                    按F12打开开发人员工具-》网络面板，<br>
                    随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="问财API密钥：" path="iwencaiApiKey">
              <n-input type="password" placeholder="同花顺问财开放平台API Key" v-model:value="formValue.iwencaiApiKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问同花顺问财开放平台：<br>
                    <a href="https://open.iwencai.com" target="_blank" style="color: #63e2b7">https://www.iwencai.com/skillhub</a><br>
                    注册并登录后，在控制台获取API Key。<br>
                    配置后可使用问财智能选股、行情查询、研报搜索等功能。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="东财AI密钥：" path="emApiKey">
              <n-input type="password" placeholder="东方财富AI SaaS API Key" v-model:value="formValue.emApiKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问东方财富妙想AI平台获取API Key。
                    https://ai.eastmoney.com/mxClaw<br>
                    配置后可使用个股业绩点评功能。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="赞助码：" path="sponsorCode">
              <n-input-group>
                <n-input :show-count="true" placeholder="联系作者QQ或微信获取，激活VIP功能" v-model:value="formValue.sponsorCode">
                </n-input>
                <n-button type="success" secondary strong
                          @click="CheckSponsorCode(formValue.sponsorCode).then((res) => {message.warning(res.msg)})">验证
                </n-button>
                <n-popover trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon color="#0e7a0d" size="20">
                      <HelpCircleFilledIcon />
                    </n-icon>
                  </template>
                  <n-gradient-text :type="'warning'">
                    <div style="max-width: 400px;text-align: left">
                      赞助码获取方式：<br>
                      联系作者获取赞助码，激活VIP功能<br>
                      享受更多高级功能和优先支持
                    </div>
                  </n-gradient-text>
                </n-popover>
              </n-input-group>
            </n-form-item-gi>

            <n-form-item-gi :span="11" label="提示词广场地址：" path="promptPlazaApiBase">
              <n-input type="text" placeholder="http://go-stock.sparkmemory.top:1918/api" v-model:value="formValue.promptPlazaApiBase" clearable/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    提示词广场服务接口地址<br>
                    默认: http://go-stock.sparkmemory.top:1918/api<br>
                    如已部署提示词广场服务，可修改为实际地址
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '通知设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="钉钉推送：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="飞书推送：" path="feishuPush.enable">
              <n-switch v-model:value="formValue.feishuPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地推送：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="弹幕功能：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="显示滚动快讯：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="市场资讯提醒：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="只提醒红字或关注个股的新闻：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="钉钉机器人接口地址："
                            path="dingPush.dingRobot">
              <n-input placeholder="请输入钉钉机器人接口地址" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.feishuPush.enable" label="飞书机器人接口地址："
                            path="feishuPush.feishuRobot">
              <n-input placeholder="请输入飞书自定义机器人 Webhook 地址（https://open.feishu.cn/open-apis/bot/v2/hook/...）"
                       v-model:value="formValue.feishuPush.feishuRobot"/>
            </n-form-item-gi>
            <n-form-item-gi :span="22" v-if="formValue.feishuPush.enable" label="飞书签名校验 Secret："
                            path="feishuPush.feishuSecret">
              <n-input type="password" show-password-on="click"
                       placeholder="可选：填写机器人安全设置的签名校验 Secret，留空则不签名"
                       v-model:value="formValue.feishuPush.feishuSecret"/>
              <n-button type="primary" @click="sendFeishuTestNotice">发送测试通知</n-button>
            </n-form-item-gi>

          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'AI设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left;">
            <n-form-item-gi :span="24" label="AI诊股：" path="openAI.enable">
              <n-switch v-model:value="formValue.openAI.enable"/>
            </n-form-item-gi>

            <n-form-item-gi :span="6" v-if="formValue.openAI.enable" label="Crawler Timeout(秒)"
                            title="资讯采集超时时间(秒)" path="openAI.crawlTimeOut">
              <n-input-number min="30" step="1" v-model:value="formValue.openAI.crawlTimeOut"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="formValue.openAI.enable" title="天数越多消耗tokens越多"
                            label="日K线数据(天)" path="openAI.kDays">
              <n-input-number min="30" step="1" max="60" v-model:value="formValue.openAI.kDays"/>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="爬虫http代理" path="httpProxyEnabled">
              <n-switch v-model:value="formValue.httpProxyEnabled"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" v-if="formValue.httpProxyEnabled" title="http代理地址"
                            label="http代理地址" path="httpProxy">
              <n-input type="text" placeholder="爬虫http代理地址" v-model:value="formValue.httpProxy" clearable/>
            </n-form-item-gi>


            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">默认提示词设置</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认系统提示词" path="openAI.prompt">
              <n-input v-model:value="formValue.openAI.prompt" type="textarea" :show-count="true"
                       placeholder="请输入系统提示词" :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认个股分析提示词"
                            path="openAI.questionTemplate">
              <n-input v-model:value="formValue.openAI.questionTemplate" type="textarea" :show-count="true"
                       placeholder="请输入个股分析提示词:例如{{stockName}}[{{stockCode}}]分析和总结"
                       :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">AI模型服务配置</n-divider>
            </n-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-space vertical>
                <n-collapse v-model:expanded-names="aiConfigExpandedNames" accordion>
                  <n-collapse-item v-for="(aiConfig, index) in formValue.openAI.aiConfigs" :key="index" :name="String(index)">
                    <template #header>
                      <n-flex justify="space-between" align="center" style="width: 100%;">
                        <n-text>{{ aiConfig.name || `AI 配置 #${index + 1}` }}</n-text>
                        <n-text depth="3" style="font-size: 12px;">{{ aiConfig.modelName || '未选择模型' }}</n-text>
                      </n-flex>
                    </template>
                    <template #header-extra>
                      <n-button type="error" size="tiny" ghost @click.stop="removeAiConfig(index)" style="margin-right: 8px;">删除</n-button>
                    </template>
                    <n-grid :cols="24" :x-gap="24">
                      <n-form-item-gi :span="24" hidden label="配置ID" :path="`openAI.aiConfigs[${index}].ID`">
                        <n-input type="text" placeholder="配置ID" v-model:value="aiConfig.ID" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="配置名称" :path="`openAI.aiConfigs[${index}].name`">
                        <n-input type="text" placeholder="配置名称" v-model:value="aiConfig.name" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="接口地址" :path="`openAI.aiConfigs[${index}].baseUrl`">
                        <n-select
                          v-model:value="aiConfig.baseUrl"
                          :options="aiPlatformOptions"
                          filterable
                          tag
                          clearable
                          placeholder="选择或输入AI接口地址"
                          @update:value="(val) => onBaseUrlChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="令牌(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                        <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                                 show-password-on="click"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="模型名称" :path="`openAI.aiConfigs[${index}].modelName`">
                        <n-select
                          v-model:value="aiConfig.modelName"
                          :options="aiConfig._modelOptions || []"
                          filterable
                          tag
                          :loading="aiConfig._loadingModels"
                          placeholder="点击获取模型列表或手动输入"
                          @click="fetchAiModels(aiConfig)"
                          @update:value="(val) => onModelNameChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                        <n-input-number placeholder="temperature" v-model:value="aiConfig.temperature" :step="0.1"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                        <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="Timeout(秒)" :path="`openAI.aiConfigs[${index}].timeOut`">
                        <n-input-number min="60" step="1" placeholder="超时(秒)" v-model:value="aiConfig.timeOut"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="深度思考">
                        <n-switch v-model:value="aiConfig.thinking"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              启用深度思考模式：<br>
                              适用于 DeepSeek-Reasoner、MiMo-V2.5-Pro 等支持推理的模型。<br>
                              如使用普通模型请关闭此选项
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="http代理" :path="`openAI.aiConfigs[${index}].httpProxyEnabled`">
                        <n-switch v-model:value="aiConfig.httpProxyEnabled"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" v-if="aiConfig.httpProxyEnabled" title="http代理地址" :path="`openAI.aiConfigs[${index}].httpProxy`">
                        <n-input type="text" placeholder="http代理地址" v-model:value="aiConfig.httpProxy" clearable/>
                      </n-form-item-gi>
                    </n-grid>
                  </n-collapse-item>
                </n-collapse>
                <n-button type="primary" dashed @click="addAiConfig" style="width: 100%;">+ 添加AI配置</n-button>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
<!--                  <n-button type="warning" @click="managePrompts">管理提示词模板</n-button>-->
                  <n-button type="primary" strong @click="saveConfig">保存设置</n-button>
                  <n-button type="info" @click="exportConfig">导出配置</n-button>
                  <n-button type="error" @click="importConfig">导入配置</n-button>
                </n-space>

<!--                <n-flex justify="start" style="margin-top: 10px" v-if="promptTemplates.length > 0">-->
<!--                  <n-tag :bordered="false" type="warning">提示词模板:</n-tag>-->
<!--                  <n-tag size="medium" secondary v-for="prompt in promptTemplates" closable-->
<!--                         @close="deletePrompt(prompt.ID)" @click="editPrompt(prompt)" :title="prompt.content"-->
<!--                         :type="prompt.type === '模型系统Prompt' ? 'success' : 'info'" :bordered="false">{{-->
<!--                      prompt.name-->
<!--                    }}-->
<!--                  </n-tag>-->
<!--                </n-flex>-->
              </n-space>
            </n-gi>

          </n-grid>
        </n-card>
      </n-space>
    </n-form>
  </n-flex>

  <n-modal v-model:show="showManagePromptsModal" closable :mask-closable="false">
    <n-card style="width: 800px; height: 600px; text-align: left" :bordered="false"
            :title="(formPrompt.ID > 0 ? '修改' : '添加') + '提示词'" size="huge" role="dialog" aria-modal="true">
      <n-form ref="formPromptRef" :label-placement="'left'" :label-align="'left'">
        <n-form-item label="名称">
          <n-input v-model:value="formPrompt.Name" placeholder="请输入提示词名称"/>
        </n-form-item>
        <n-form-item label="类型">
          <n-select v-model:value="formPrompt.Type" :options="promptTypeOptions" placeholder="请选择提示词类型"/>
        </n-form-item>
        <n-form-item label="内容">
          <n-input v-model:value="formPrompt.Content" type="textarea" :show-count="true" placeholder="请输入prompt"
                   :autosize="{ minRows: 12, maxRows: 12, }"/>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-flex justify="end">
          <n-button type="primary" @click="savePrompt">保存</n-button>
          <n-button type="warning" @click="showManagePromptsModal = false">取消</n-button>
        </n-flex>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.cardHeaderClass {
  font-size: 16px;
  font-weight: bold;
  color: red;
}
</style>