<script setup>
import {h, onBeforeUnmount, onMounted, ref} from "vue";
import {
  AddPrompt, DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  SendDingDingMessageByType,
  UpdateConfig, CheckSponsorCode
} from "../../wailsjs/go/main/App";
import {NTag, useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";

const message = useMessage()

const formRef = ref(null)
const formValue = ref({
  ID: 1,
  tushareToken: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  localPush: {
    enable: true,
  },
  updateBasicInfoOnStart: false,
  refreshInterval: 1,
  openAI: {
    enable: false,
    aiConfigs: [], // AI configuration list
    prompt: "",
    questionTemplate: "{{stockName}} Analysis and Summary",
    crawlTimeOut: 30,
    kDays: 30,
  },
  enableDanmu: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: false,
  enableOnlyPushRedNews: false,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
})

// Add a new AI configuration to the list
function addAiConfig() {
  formValue.value.openAI.aiConfigs.push(new data.AIConfig({
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-chat',
    temperature: 0.1,
    maxTokens: 1024,
    timeOut: 60,
  }));
}

// Remove an AI configuration from the list
function removeAiConfig(index) {
  const originalCount = formValue.value.openAI.aiConfigs.length;
  // Use filter to create a new array to ensure reactive update
  formValue.value.openAI.aiConfigs = formValue.value.openAI.aiConfigs.filter((_, i) => i !== index);
}


const promptTemplates = ref([])
onMounted(() => {
  GetConfig().then(res => {
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.dingPush = {
      enable: res.dingPushEnable,
      dingRobot: res.dingRobot
    }
    formValue.value.localPush = {
      enable: res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval
    // Load AI configuration
    formValue.value.openAI = {
      enable: res.openAiEnable,
      aiConfigs: res.aiConfigs || [],
      prompt: res.prompt,
      questionTemplate: res.questionTemplate ? res.questionTemplate : '{{stockName}} Analysis and Summary',
      crawlTimeOut: res.crawlTimeOut,
      kDays: res.kDays,
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

  })

  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
  })
})
onBeforeUnmount(() => {
  message.destroyAll()
})

function saveConfig() {
  console.log('Starting to save settings', formValue.value);
  // When building config, include aiConfigs list
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: formValue.value.openAI.enable,
    aiConfigs: formValue.value.openAI.aiConfigs,
    // Serialize aiConfigs list to pass to backend
    tushareToken: formValue.value.tushareToken,
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
  })

  if (config.sponsorCode) {
    CheckSponsorCode(config.sponsorCode).then(res => {
      if (res.code) {
        UpdateConfig(config).then(res => {
          message.success(res)
          EventsEmit("updateSettings", config);
        })
      } else {
        message.error(res.msg)
      }
    })
  } else {
    UpdateConfig(config).then(res => {
      message.success(res)
      EventsEmit("updateSettings", config);
    })
  }
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
      formValue.value.dingPush = {
        enable: config.dingPushEnable,
        dingRobot: config.dingRobot
      }
      formValue.value.localPush = {
        enable: config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      // Import AI configuration
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
  {label: "Model System Prompt", value: 'Model System Prompt'},
  {label: "Model User Prompt", value: 'Model User Prompt'},]
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
        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'Basic Settings')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="10" label="Tushare Token：" path="tushareToken">
              <n-input type="text" placeholder="Tushare api token" v-model:value="formValue.tushareToken" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="Update on Startup：" path="updateBasicInfoOnStart">
              <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="Refresh Interval：" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="Enter refresh interval (seconds)">
                <template #suffix>sec</template>
              </n-input-number>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="Dark Theme：" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="Browser Path：" path="browserPath">
              <n-input type="text" placeholder="Browser installation path" v-model:value="formValue.browserPath" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="Index Funds：" path="enableFund">
              <n-switch v-model:value="formValue.enableFund"/>
            </n-form-item-gi>
            <n-form-item-gi :span="11" label="Sponsor Code：" path="sponsorCode">
              <n-input-group>
                <n-input :show-count="true" placeholder="Sponsor code" v-model:value="formValue.sponsorCode"/>
                <n-button type="success" secondary strong
                          @click="CheckSponsorCode(formValue.sponsorCode).then((res) => {message.warning(res.msg)})">Verify
                </n-button>
              </n-input-group>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'Notification Settings')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="DingTalk Push：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="Local Push：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="Danmaku Feature：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="Show Scrolling News：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="Market News Alerts：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="Only Red or Followed Stock News：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="DingTalk Robot URL："
                            path="dingPush.dingRobot">
              <n-input placeholder="Enter DingTalk robot interface URL" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">Send Test Notification</n-button>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'AI Settings')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left;">
            <n-form-item-gi :span="24" label="AI Stock Diagnosis：" path="openAI.enable">
              <n-switch v-model:value="formValue.openAI.enable"/>
            </n-form-item-gi>

            <n-form-item-gi :span="6" v-if="formValue.openAI.enable" label="Crawler Timeout(sec)"
                            title="News collection timeout (seconds)" path="openAI.crawlTimeOut">
              <n-input-number min="30" step="1" v-model:value="formValue.openAI.crawlTimeOut"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="formValue.openAI.enable" title="More days consume more tokens"
                            label="Daily K-line Data(days)" path="openAI.kDays">
              <n-input-number min="30" step="1" max="365" v-model:value="formValue.openAI.kDays"/>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="HTTP Proxy" path="httpProxyEnabled">
              <n-switch v-model:value="formValue.httpProxyEnabled"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" v-if="formValue.httpProxyEnabled" title="HTTP proxy address"
                            label="HTTP Proxy Address" path="httpProxy">
              <n-input type="text" placeholder="HTTP proxy address" v-model:value="formValue.httpProxy" clearable/>
            </n-form-item-gi>


            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">Prompt Content Settings</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="Model System Prompt" path="openAI.prompt">
              <n-input v-model:value="formValue.openAI.prompt" type="textarea" :show-count="true"
                       placeholder="Enter system prompt" :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="Model User Prompt"
                            path="openAI.questionTemplate">
              <n-input v-model:value="formValue.openAI.questionTemplate" type="textarea" :show-count="true"
                       placeholder="Enter user prompt: e.g. {{stockName}}[{{stockCode}}] Analysis and Summary"
                       :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">AI Model Service Configuration</n-divider>
            </n-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-space vertical>
                <n-card v-for="(aiConfig, index) in formValue.openAI.aiConfigs" :key="index" :bordered="true"
                        size="small">
                  <template #header>
                    <n-flex justify="space-between" align="center">
                      <n-text depth="3">AI Config #{{ index + 1 }}</n-text>
                      <n-button type="error" size="tiny" ghost @click="removeAiConfig(index)">Delete</n-button>
                    </n-flex>
                  </template>
                  <n-grid :cols="24" :x-gap="24">
                    <n-form-item-gi :span="24" hidden label="Config ID" :path="`openAI.aiConfigs[${index}].ID`">
                      <n-input type="text" placeholder="Config ID" v-model:value="aiConfig.ID" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="Config Name" :path="`openAI.aiConfigs[${index}].name`">
                      <n-input type="text" placeholder="Config name" v-model:value="aiConfig.name" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="API URL" :path="`openAI.aiConfigs[${index}].baseUrl`">
                      <n-input type="text" placeholder="AI API URL" v-model:value="aiConfig.baseUrl" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="Token(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                      <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                               show-password-on="click"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="8" label="Model Name" :path="`openAI.aiConfigs[${index}].modelName`">
                      <n-input type="text" placeholder="AI model name" v-model:value="aiConfig.modelName" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                      <n-input-number placeholder="temperature" v-model:value="aiConfig.temperature" :step="0.1"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                      <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="Timeout(sec)" :path="`openAI.aiConfigs[${index}].timeOut`">
                      <n-input-number min="60" step="1" placeholder="Timeout(sec)" v-model:value="aiConfig.timeOut"/>
                    </n-form-item-gi>
                  </n-grid>
                </n-card>
                <n-button type="primary" dashed @click="addAiConfig" style="width: 100%;">+ Add AI Config</n-button>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
                  <n-button type="warning" @click="managePrompts">Manage Prompt Templates</n-button>
                  <n-button type="primary" strong @click="saveConfig">Save Settings</n-button>
                  <n-button type="info" @click="exportConfig">Export Config</n-button>
                  <n-button type="error" @click="importConfig">Import Config</n-button>
                </n-space>

                <n-flex justify="start" style="margin-top: 10px" v-if="promptTemplates.length > 0">
                  <n-tag :bordered="false" type="warning">Prompt Templates:</n-tag>
                  <n-tag size="medium" secondary v-for="prompt in promptTemplates" closable
                         @close="deletePrompt(prompt.ID)" @click="editPrompt(prompt)" :title="prompt.content"
                         :type="prompt.type === 'Model System Prompt' ? 'success' : 'info'" :bordered="false">{{
                      prompt.name
                    }}
                  </n-tag>
                </n-flex>
              </n-space>
            </n-gi>

          </n-grid>
        </n-card>
      </n-space>
    </n-form>
  </n-flex>

  <n-modal v-model:show="showManagePromptsModal" closable :mask-closable="false">
    <n-card style="width: 800px; height: 600px; text-align: left" :bordered="false"
            :title="(formPrompt.ID > 0 ? 'Edit' : 'Add') + ' Prompt'" size="huge" role="dialog" aria-modal="true">
      <n-form ref="formPromptRef" :label-placement="'left'" :label-align="'left'">
        <n-form-item label="Name">
          <n-input v-model:value="formPrompt.Name" placeholder="Enter prompt name"/>
        </n-form-item>
        <n-form-item label="Type">
          <n-select v-model:value="formPrompt.Type" :options="promptTypeOptions" placeholder="Select prompt type"/>
        </n-form-item>
        <n-form-item label="Content">
          <n-input v-model:value="formPrompt.Content" type="textarea" :show-count="true" placeholder="Enter prompt"
                   :autosize="{ minRows: 12, maxRows: 12, }"/>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-flex justify="end">
          <n-button type="primary" @click="savePrompt">Save</n-button>
          <n-button type="warning" @click="showManagePromptsModal = false">Cancel</n-button>
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