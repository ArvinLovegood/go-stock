<script setup>
import {h, onBeforeUnmount, onMounted, ref} from "vue";
import {useRouter} from "vue-router";
import {
  AddPrompt,
  DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  SendDingDingMessageByType,
  SendFeishuMessageByType,
  StartFeishuBot,
  StopFeishuBot,
  GetFeishuBotStatus,
  UpdateConfig,
  UpdateAiConfigs,
  CheckSponsorCode,
} from "../../wailsjs/go/main/App";
import {NTag, NTooltip, NIcon, useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";

const message = useMessage()
const router = useRouter()

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
  feishuBot: {
    enable: false,
    appId: '',
    appSecret: '',
    aiConfigId: 0,
    sysPromptId: 0,
    enableTools: true,
    thinking: false,
    memoryEnable: false,
    agentMode: 'react',
    status: 'stopped'
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
  enableOnlyPushRedNews: false,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
  enableAgent: false,
  qgqpBId: '',
  updateChannel: 'release',
  // 广场服务地址固定（定制版不可修改）
  promptPlazaApiBase: 'https://go-stock.sparkmemory.top/api',
})

const updateChannelOptions = [
  { label: 'Release（稳定版）', value: 'release' },
  { label: 'Pre-release（预发布版）', value: 'pre' },
  { label: 'Dev（开发版）', value: 'dev' },
]

const promptTemplates = ref([])

// 跳转到独立的 AI 模型服务管理页面
function goAiConfigs() {
  router.push({name: 'aiConfigs'})
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
    formValue.value.feishuBot = {
      enable: res.feishuBotEnable,
      appId: res.feishuAppId || '',
      appSecret: res.feishuAppSecret || '',
      aiConfigId: res.feishuBotAiConfigId || 0,
      sysPromptId: res.feishuBotSysPromptId || 0,
      enableTools: res.feishuBotEnableTools !== false,
      thinking: res.feishuBotThinking === true,
      memoryEnable: res.feishuBotMemoryEnable === true,
      agentMode: res.feishuBotAgentMode || 'react',
      status: 'stopped'
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
    // 广场服务地址固定（定制版不可修改），后端 GetSettingConfig 始终返回固定值，此处兜底
    formValue.value.promptPlazaApiBase = res.promptPlazaApiBase || 'https://go-stock.sparkmemory.top/api';

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
  // AI 模型服务配置已迁移到独立页面，此处不传 aiConfigs（后端收到 nil 保留现有配置，避免覆盖）
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    feishuPushEnable: formValue.value.feishuPush.enable,
    feishuRobot: formValue.value.feishuPush.feishuRobot,
    feishuSecret: formValue.value.feishuPush.feishuSecret,
    feishuBotEnable: formValue.value.feishuBot.enable,
    feishuAppId: formValue.value.feishuBot.appId,
    feishuAppSecret: formValue.value.feishuBot.appSecret,
    feishuBotAiConfigId: formValue.value.feishuBot.aiConfigId,
    feishuBotSysPromptId: formValue.value.feishuBot.sysPromptId,
    feishuBotEnableTools: formValue.value.feishuBot.enableTools,
    feishuBotThinking: formValue.value.feishuBot.thinking,
    feishuBotMemoryEnable: formValue.value.feishuBot.memoryEnable,
    feishuBotAgentMode: formValue.value.feishuBot.agentMode,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: formValue.value.openAI.enable,
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
    // 广场服务地址固定值（后端 UpdateConfig 也会强制覆盖）
    promptPlazaApiBase: 'https://go-stock.sparkmemory.top/api',
  })

  if (config.sponsorCode) {
    CheckSponsorCode(config.sponsorCode).then(res => {
      if (!res.code) {
        message.warning(res.msg || '赞助码验证失败')
      }
    })
  }

  return UpdateConfig(config).then(res => {
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

// 飞书应用机器人控制函数（与 feishuPush 自定义机器人推送独立）
async function startFeishuBot() {
  // 先保存配置，确保最新 appID/appSecret/aiConfigId 已写入
  await saveConfig()
  try {
    const res = await StartFeishuBot()
    if (res && res.includes('失败')) {
      message.error(res)
    } else {
      message.success(res)
      formValue.value.feishuBot.status = 'running'
    }
  } catch (e) {
    message.error('启动失败：' + e)
  }
}

async function stopFeishuBot() {
  try {
    const res = await StopFeishuBot()
    message.success(res)
    formValue.value.feishuBot.status = 'stopped'
  } catch (e) {
    message.error('停止失败：' + e)
  }
}

async function refreshFeishuBotStatus() {
  try {
    const res = await GetFeishuBotStatus()
    formValue.value.feishuBot.status = res
    message.info('当前状态：' + res)
  } catch (e) {
    formValue.value.feishuBot.status = 'stopped'
  }
}

// AI 配置下拉选项（来自 openAI.aiConfigs）
function aiConfigOptions() {
  if (!formValue.value.openAI || !formValue.value.openAI.aiConfigs) {
    return []
  }
  return formValue.value.openAI.aiConfigs.map(c => ({
    label: `${c.name || '未命名'} [${c.modelName || '未指定'}]`,
    value: Number(c.ID) || 0
  }))
}

// 飞书应用机器人「系统提示词」下拉选项：从提示词模板加载，0=默认（不使用系统提示词）
function promptTemplateOptions() {
  const opts = [{ label: '默认（不使用系统提示词）', value: 0 }]
  if (!promptTemplates.value || promptTemplates.value.length === 0) {
    return opts
  }
  promptTemplates.value.forEach(t => {
    const id = Number(t.ID ?? t.id) || 0
    if (id === 0) return
    const name = t.name ?? '未命名'
    const type = t.type ? ` [${t.type}]` : ''
    opts.push({ label: `${name}${type}`, value: id })
  })
  return opts
}

// 飞书应用机器人「Agent 模式」下拉选项
function agentModeOptions() {
  return [
    { label: '快速模式（React，多轮工具调用）', value: 'react' },
    { label: '规划模式（先规划再执行）', value: 'plan_execute' },
    { label: '深度模式（任务规划+子Agent委派）', value: 'deepagents' },
    { label: '直接模式（跳过Agent框架，直连OpenAI+工具调用）', value: 'direct' },
    { label: '自动（根据问题复杂度判断）', value: '' }
  ]
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
      formValue.value.feishuBot = {
        enable: config.feishuBotEnable,
        appId: config.feishuAppId || '',
        appSecret: config.feishuAppSecret || '',
        aiConfigId: config.feishuBotAiConfigId || 0,
        sysPromptId: config.feishuBotSysPromptId || 0,
        enableTools: config.feishuBotEnableTools !== false,
        thinking: config.feishuBotThinking === true,
        memoryEnable: config.feishuBotMemoryEnable === true,
        agentMode: config.feishuBotAgentMode || 'react',
        status: formValue.value.feishuBot.status || 'stopped'
      }
      formValue.value.localPush = {
        enable: config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      // 导入AI配置（仅用于页面显示，保存时通过 UpdateAiConfigs 单独保存）
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

      // 导入的 AI 配置单独保存到独立管理页面所用的表
      if (Array.isArray(config.aiConfigs) && config.aiConfigs.length > 0) {
        UpdateAiConfigs(config.aiConfigs).then(res => {
          if (res === '保存成功！') {
            message.success('已导入 ' + config.aiConfigs.length + ' 个 AI 模型服务配置')
          } else {
            message.error('AI 模型服务配置导入失败：' + res)
          }
        }).catch(err => {
          message.error('AI 模型服务配置导入失败：' + err)
        })
      }
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
              <n-input type="text" v-model:value="formValue.promptPlazaApiBase" disabled/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    提示词广场服务接口地址（固定配置，不可修改）
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

        <n-card :title="() => h(NTag, { type: 'info', bordered: false }, () => '飞书应用机器人（AI对话）')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="24" label="启用飞书应用机器人" path="feishuBot.enable">
              <n-switch v-model:value="formValue.feishuBot.enable"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                通过长连接接收飞书用户消息，由 AI Agent 自动回复（与上方自定义机器人推送完全独立）
              </n-text>
            </n-form-item-gi>

            <n-form-item-gi :span="12" v-if="formValue.feishuBot.enable" label="App ID"
                            path="feishuBot.appId">
              <n-input placeholder="飞书应用 App ID（cli_xxx）" v-model:value="formValue.feishuBot.appId"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="formValue.feishuBot.enable" label="App Secret"
                            path="feishuBot.appSecret">
              <n-input type="password" show-password-on="click"
                       placeholder="飞书应用 App Secret" v-model:value="formValue.feishuBot.appSecret"/>
            </n-form-item-gi>

            <n-form-item-gi :span="8" v-if="formValue.feishuBot.enable" label="AI 配置"
                            path="feishuBot.aiConfigId">
              <n-select v-model:value="formValue.feishuBot.aiConfigId"
                        :options="aiConfigOptions()"
                        filterable
                        placeholder="选择上方 AI 模型服务配置"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" v-if="formValue.feishuBot.enable" label="系统提示词"
                            path="feishuBot.sysPromptId">
              <n-select v-model:value="formValue.feishuBot.sysPromptId"
                        :options="promptTemplateOptions()"
                        filterable
                        placeholder="选择系统提示词模板（可选）"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" v-if="formValue.feishuBot.enable" label="Agent 模式"
                            path="feishuBot.agentMode">
              <n-select v-model:value="formValue.feishuBot.agentMode"
                        :options="agentModeOptions()"/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" v-if="formValue.feishuBot.enable" label="启用工具调用"
                            path="feishuBot.enableTools">
              <n-switch v-model:value="formValue.feishuBot.enableTools"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                关闭后走单轮对话（不调用工具）
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="6" v-if="formValue.feishuBot.enable" label="深度思考"
                            path="feishuBot.thinking">
              <n-switch v-model:value="formValue.feishuBot.thinking"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                推理模型启用后输出思考过程
              </n-text>
            </n-form-item-gi>

            <n-form-item-gi :span="6" v-if="formValue.feishuBot.enable" label="多轮记忆"
                            path="feishuBot.memoryEnable">
              <n-switch v-model:value="formValue.feishuBot.memoryEnable"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                开启后携带最近一轮对话上下文
              </n-text>
            </n-form-item-gi>

            <n-form-item-gi :span="24" v-if="formValue.feishuBot.enable">
              <n-space>
                <n-button type="primary" @click="startFeishuBot">启动机器人</n-button>
                <n-button type="warning" @click="stopFeishuBot">停止机器人</n-button>
                <n-button @click="refreshFeishuBotStatus">查询状态</n-button>
                <n-tag :type="formValue.feishuBot.status === 'running' ? 'success' : 'default'">
                  {{ formValue.feishuBot.status === 'running' ? '运行中' : '已停止' }}
                </n-tag>
              </n-space>
            </n-form-item-gi>

            <n-form-item-gi :span="24" v-if="formValue.feishuBot.enable">
              <n-gradient-text type="info">
                <div style="font-size: 12px; line-height: 1.6;">
                  配置步骤：<br>
                  1. 在飞书开放平台创建企业自建应用，获取 App ID 和 App Secret<br>
                  2. 启用「机器人」能力，添加事件订阅 im.message.receive_v1<br>
                  3. 事件订阅页选择「使用长连接接收事件」（无需公网 IP）<br>
                  4. 应用需发布并通过审核；权限需添加「获取用户发给机器人的单聊消息」「获取群组中所有消息」或「获取用户在群组中@机器人的消息」<br>
                  5. 单聊直接回复；群聊需 @机器人才会回复；多轮对话记忆按 sessionID 自动隔离<br>
                  文档：https://open.feishu.cn/document/server-side-sdk/golang-sdk-guide/handle-events
                </div>
              </n-gradient-text>
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
              <n-alert type="info" :bordered="false">
                <n-space align="center">
                  <n-text>AI 模型服务配置已迁移到独立页面（共 {{ formValue.openAI.aiConfigs?.length || 0 }} 个配置）</n-text>
                  <n-button type="primary" size="small" tag="a" @click="goAiConfigs">前往管理</n-button>
                </n-space>
              </n-alert>
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