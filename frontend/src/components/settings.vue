<script setup>

import {computed, onBeforeUnmount, onMounted, ref} from "vue";
import {
  AddPrompt, DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  SendDingDingMessageByType,
  UpdateConfig
} from "../../wailsjs/go/main/App";
import {useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
const message = useMessage()

const formRef = ref(null)
const formValue = ref({
  ID:1,
  tushareToken:'',
  dingPush:{
    enable:false,
    dingRobot: ''
  },
  localPush:{
    enable:true,
  },
  updateBasicInfoOnStart:false,
  refreshInterval:1,
  openAI:{
    enable:false,
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    model: 'deepseek-chat',
    temperature: 0.1,
    maxTokens: 1024,
    prompt:"",
    timeout: 5,
    questionTemplate: "{{stockName}}分析和总结",
    crawlTimeOut:30,
    kDays:30,
  },
  enableDanmu:false,
  browserPath: '',
  enableNews:false,
  darkTheme:true,
  enableFund:false,
})
const promptTemplates=ref([])
onMounted(()=>{
  GetConfig().then(res=>{
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.dingPush = {
      enable:res.dingPushEnable,
      dingRobot:res.dingRobot
    }
    formValue.value.localPush = {
      enable:res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval
    formValue.value.openAI = {
      enable:res.openAiEnable,
      baseUrl: res.openAiBaseUrl,
      apiKey:res.openAiApiKey,
      model:res.openAiModelName,
      temperature:res.openAiTemperature,
      maxTokens:res.openAiMaxTokens,
      prompt:res.prompt,
      timeout:res.openAiApiTimeOut,
      questionTemplate:res.questionTemplate?res.questionTemplate:'{{stockName}}分析和总结',
      crawlTimeOut:res.crawlTimeOut,
      kDays:res.kDays,
    }
    formValue.value.enableDanmu = res.enableDanmu
    formValue.value.browserPath = res.browserPath
    formValue.value.enableNews = res.enableNews
    formValue.value.darkTheme = res.darkTheme
    formValue.value.enableFund = res.enableFund

    //console.log(res)
  })
  //message.info("加载完成")

  GetPromptTemplates("","").then(res=>{
    //console.log(res)
    promptTemplates.value=res
  })
})
onBeforeUnmount(() => {
  message.destroyAll()
})

function saveConfig(){

  let config= new data.Settings({
    ID:formValue.value.ID,
    dingPushEnable:formValue.value.dingPush.enable,
    dingRobot:formValue.value.dingPush.dingRobot,
    localPushEnable:formValue.value.localPush.enable,
    updateBasicInfoOnStart:formValue.value.updateBasicInfoOnStart,
    refreshInterval:formValue.value.refreshInterval,
    openAiEnable:formValue.value.openAI.enable,
    openAiBaseUrl:formValue.value.openAI.baseUrl,
    openAiApiKey:formValue.value.openAI.apiKey,
    openAiModelName:formValue.value.openAI.model,
    openAiMaxTokens:formValue.value.openAI.maxTokens,
    openAiTemperature:formValue.value.openAI.temperature,
    tushareToken:formValue.value.tushareToken,
    prompt:formValue.value.openAI.prompt,
    openAiApiTimeOut:formValue.value.openAI.timeout,
    questionTemplate:formValue.value.openAI.questionTemplate,
    crawlTimeOut:formValue.value.openAI.crawlTimeOut,
    kDays:formValue.value.openAI.kDays,
    enableDanmu:formValue.value.enableDanmu,
    browserPath:formValue.value.browserPath,
    enableNews:formValue.value.enableNews,
    darkTheme:formValue.value.darkTheme,
    enableFund:formValue.value.enableFund,
  })


  //console.log("Settings",config)
  UpdateConfig(config).then(res=>{
    message.success(res)
    EventsEmit("updateSettings", config);
  })
}


function getHeight() {
  return document.documentElement.clientHeight
}
function sendTestNotice(){
  let markdown="### go-stock test\n"+new Date()
  let msg='{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"go-stock'+new Date()+'",' +
      '         "text": "'+markdown+'"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'

  SendDingDingMessageByType(msg, "test-"+new Date().getTime(),1).then(res=>{
    message.info(res)
  })
}

function exportConfig(){
  ExportConfig().then(res=>{
    message.info(res)
  })
}

function importConfig(){
  let input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    let file = e.target.files[0];
    let reader = new FileReader();
    reader.onload = (e) => {
      let config = JSON.parse(e.target.result);
      //console.log(config)
      formValue.value.ID = config.ID
      formValue.value.tushareToken = config.tushareToken
      formValue.value.dingPush = {
        enable:config.dingPushEnable,
        dingRobot:config.dingRobot
      }
      formValue.value.localPush = {
        enable:config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      formValue.value.openAI = {
        enable:config.openAiEnable,
        baseUrl: config.openAiBaseUrl,
        apiKey:config.openAiApiKey,
        model:config.openAiModelName,
        temperature:config.openAiTemperature,
        maxTokens:config.openAiMaxTokens,
        prompt:config.prompt,
        timeout:config.openAiApiTimeOut,
        questionTemplate:config.questionTemplate,
        crawlTimeOut:config.crawlTimeOut,
        kDays:config.kDays
      }
      formValue.value.enableDanmu = config.enableDanmu
      formValue.value.browserPath = config.browserPath
      formValue.value.enableNews = config.enableNews
      formValue.value.darkTheme = config.darkTheme
      formValue.value.enableFund = config.enableFund
     // formRef.value.resetFields()
    };
    reader.readAsText(file);
  };
  input.click();
}


window.onerror = function (event, source, lineno, colno, error) {
  //console.log(event, source, lineno, colno, error)
  // 将错误信息发送给后端
  EventsEmit("frontendError", {
    page: "settings.vue",
    message: event,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null
  });
  //message.error("发生错误:"+event)
  return true;
};

const showManagePromptsModal=ref(false)
const promptTypeOptions=[
  {label:"模型系统Prompt",value:'模型系统Prompt'},
  {label:"模型用户Prompt",value:'模型用户Prompt'},]
const formPromptRef=ref(null)
const formPrompt=ref({
  ID:0,
  Name:'',
  Content:'',
  Type:'',
})
function managePrompts(){
  formPrompt.value.ID=0
  showManagePromptsModal.value=true
}
function savePrompt(){
  AddPrompt(formPrompt.value).then(res=>{
    message.success(res)
    GetPromptTemplates("","").then(res=>{
      //console.log(res)
      promptTemplates.value=res
    })
    showManagePromptsModal.value=false
  })
}
function editPrompt(prompt){
  //console.log(prompt)
  formPrompt.value.ID=prompt.ID
  formPrompt.value.Name=prompt.name
  formPrompt.value.Content=prompt.content
  formPrompt.value.Type=prompt.type
  showManagePromptsModal.value=true
}
function deletePrompt(ID){
  DelPrompt(ID).then(res=>{
    message.success(res)
    GetPromptTemplates("","").then(res=>{
      //console.log(res)
      promptTemplates.value=res
    })
  })
}
</script>

<template>
  <n-flex justify="left" style="margin-top: 12px;padding-left: 12px;">
  <n-form ref="formRef"  :label-placement="'left'" :label-align="'left'" >
      <n-grid :cols="24" :x-gap="24" style="text-align: left" :layout-shift-disabled="true">
        <n-gi :span="24">
          <n-text type="success" style="font-size: 25px;font-weight: bold">基础设置</n-text>
        </n-gi>
        <n-form-item-gi  :span="10" label="Tushare &nbsp;&nbsp;Token：" path="tushareToken"  >
          <n-input  type="text" placeholder="Tushare api token"  v-model:value="formValue.tushareToken" clearable />
        </n-form-item-gi>
        <n-form-item-gi  :span="4" label="启动时更新A股/指数信息：" path="updateBasicInfoOnStart" >
          <n-switch v-model:value="formValue.updateBasicInfoOnStart" />
        </n-form-item-gi>
        <n-form-item-gi  :span="4" label="数据刷新间隔：" path="refreshInterval" >
          <n-input-number v-model:value="formValue.refreshInterval" placeholder="请输入数据刷新间隔(秒)">
            <template #suffix>
              秒
            </template>
          </n-input-number>
        </n-form-item-gi>
        <n-form-item-gi  :span="6" label="暗黑主题：" path="darkTheme" >
          <n-switch v-model:value="formValue.darkTheme" />
        </n-form-item-gi>
        <n-form-item-gi  :span="10" label="浏览器安装路径：" path="browserPath" >
          <n-input  type="text" placeholder="浏览器安装路径"  v-model:value="formValue.browserPath" clearable />
        </n-form-item-gi>
        <n-form-item-gi  :span="6" label="是否启用指数基金：" path="enableFund" >
          <n-switch v-model:value="formValue.enableFund" />
        </n-form-item-gi>
      </n-grid>

        <n-grid :cols="24" :x-gap="24" style="text-align: left">
          <n-gi :span="24">
            <n-text type="success" style="font-size: 25px;font-weight: bold">通知设置</n-text>
          </n-gi>
          <n-form-item-gi  :span="6" label="是否启用钉钉推送：" path="dingPush.enable" >
            <n-switch v-model:value="formValue.dingPush.enable" />
          </n-form-item-gi>
          <n-form-item-gi  :span="6" label="是否启用本地推送：" path="localPush.enable"  >
            <n-switch v-model:value="formValue.localPush.enable" />
          </n-form-item-gi>
          <n-form-item-gi  :span="5" label="弹幕功能：" path="enableDanmu" >
            <n-switch v-model:value="formValue.enableDanmu" />
          </n-form-item-gi>
          <n-form-item-gi  :span="5" label="是否显示滚动快讯(重启生效)：" path="enableNews" >
            <n-switch v-model:value="formValue.enableNews" />
          </n-form-item-gi>
          <n-form-item-gi :span="22"  v-if="formValue.dingPush.enable" label="钉钉机器人接口地址：" path="dingPush.dingRobot" >
            <n-input  placeholder="请输入钉钉机器人接口地址"  v-model:value="formValue.dingPush.dingRobot"/>
            <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
          </n-form-item-gi>
        </n-grid>

    <n-grid :cols="24" :x-gap="24" style="text-align: left;">
      <n-gi :span="24">
        <n-text type="success" style="font-size: 25px;font-weight: bold">OpenAI设置</n-text>
      </n-gi>
      <n-form-item-gi  :span="3" label="是否启用AI诊股：" path="openAI.enable" >
        <n-switch v-model:value="formValue.openAI.enable" />
      </n-form-item-gi>
      <n-form-item-gi :span="9"  v-if="formValue.openAI.enable" label="openAI 接口地址：" path="openAI.baseUrl" >
        <n-input  type="text"  placeholder="AI接口地址"  v-model:value="formValue.openAI.baseUrl" clearable />
      </n-form-item-gi>
      <n-form-item-gi  :span="5" v-if="formValue.openAI.enable" label="AI Timeout(秒)：" title="AI请求超时时间(秒)"  path="openAI.timeout" >
        <n-input-number min="60" step="1" placeholder="AI请求超时时间(秒)"  v-model:value="formValue.openAI.timeout" />
      </n-form-item-gi>
      <n-form-item-gi  :span="5" v-if="formValue.openAI.enable" label="Crawler Timeout(秒)：" title="资讯采集超时时间(秒)" path="openAI.crawlTimeOut" >
        <n-input-number min="30" step="1" placeholder="资讯采集超时时间(秒)"  v-model:value="formValue.openAI.crawlTimeOut" />
      </n-form-item-gi>
      <n-form-item-gi  :span="12" v-if="formValue.openAI.enable" label="openAI 令牌(apiKey)："  path="openAI.apiKey" >
        <n-input  type="text" placeholder="apiKey"  v-model:value="formValue.openAI.apiKey" clearable />
      </n-form-item-gi>
      <n-form-item-gi :span="10"  v-if="formValue.openAI.enable" label="AI模型名称：" path="openAI.model" >
        <n-input  type="text" placeholder="AI模型名称"  v-model:value="formValue.openAI.model" clearable />
      </n-form-item-gi>
      <n-form-item-gi :span="12"  v-if="formValue.openAI.enable" label="openAI temperature：" path="openAI.temperature" >
        <n-input-number  placeholder="temperature"  v-model:value="formValue.openAI.temperature"/>
      </n-form-item-gi>
      <n-form-item-gi :span="5"  v-if="formValue.openAI.enable" label="openAI maxTokens："  path="openAI.maxTokens" >
        <n-input-number  placeholder="maxTokens"  v-model:value="formValue.openAI.maxTokens"/>
      </n-form-item-gi>
      <n-form-item-gi :span="5"  v-if="formValue.openAI.enable" title="天数越多消耗tokens越多" label="日K线数据(天)："  path="openAI.maxTokens" >
        <n-input-number  min="30" step="1" max="365"  placeholder="日K线数据(天)" title="天数越多消耗tokens越多" v-model:value="formValue.openAI.kDays"/>
      </n-form-item-gi>
      <n-form-item-gi :span="11"  v-if="formValue.openAI.enable" label="模型系统 Prompt："  path="openAI.prompt" >
        <n-input v-model:value="formValue.openAI.prompt"
            type="textarea"
            :show-count="true"
            placeholder="请输入系统prompt"
            :autosize="{
              minRows: 5,
              maxRows: 8
            }"
        />
      </n-form-item-gi>
      <n-form-item-gi :span="11"  v-if="formValue.openAI.enable" label="模型用户 Prompt："   path="openAI.questionTemplate" >
        <n-input v-model:value="formValue.openAI.questionTemplate"
            type="textarea"
            :show-count="true"
            placeholder="请输入用户prompt:例如{{stockName}}[{{stockCode}}]分析和总结"
            :autosize="{
              minRows: 5,
              maxRows: 8
            }"
        />
     </n-form-item-gi>
    </n-grid>
    <n-gi :span="24">
        <n-space justify="center">
          <n-button  type="warning" @click="managePrompts">
            添加提示词模板
          </n-button>
        <n-button  type="primary" @click="saveConfig">
          保存
        </n-button>
        <n-button  type="info" @click="exportConfig">
          导出
        </n-button>
        <n-button  type="error" @click="importConfig">
          导入
        </n-button>
        </n-space>
    </n-gi>
  </n-form>
    <n-gi :span="24"  v-if="promptTemplates.length>0"  v-for="prompt in promptTemplates" >
      <n-flex justify="start">
        <n-tag closable  @close="deletePrompt(prompt.ID)" @click="editPrompt(prompt)" :title="prompt.content" :type="prompt.type==='模型系统Prompt'?'success':'info'" :bordered="false"> {{prompt.name}} </n-tag>
      </n-flex>
    </n-gi>
  </n-flex>
  <n-modal v-model:show="showManagePromptsModal" closable  :mask-closable="false">
    <n-card
        style="width: 800px;height: 600px;text-align: left"
        :bordered="false"
        :title="(formPrompt.ID>0?'修改':'添加')+'提示词'"
        size="huge"
        role="dialog"
        aria-modal="true"
    >
      <n-form ref="formPromptRef"  :label-placement="'left'" :label-align="'left'" >
        <n-form-item  label="名称">
          <n-input v-model:value="formPrompt.Name" placeholder="请输入提示词名称" />
        </n-form-item>
        <n-form-item  label="类型">
          <n-select v-model:value="formPrompt.Type" :options="promptTypeOptions" placeholder="请选择提示词类型" />
        </n-form-item>
        <n-form-item  label="内容">
          <n-input v-model:value="formPrompt.Content"
                   type="textarea"
                   :show-count="true"
                   placeholder="请输入prompt"
                   :autosize="{
              minRows: 12,
              maxRows: 12,
            }"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-flex justify="end">
          <n-button type="primary" @click="savePrompt">
            保存
          </n-button>
          <n-button type="warning" @click="showManagePromptsModal=false">
            取消
          </n-button>
        </n-flex>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>

</style>