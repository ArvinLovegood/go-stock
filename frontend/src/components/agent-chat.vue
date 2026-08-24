<template>
  <div class="chat-box">
    <t-chat
        ref="chatRef"
        :clear-history="chatList.length > 0 && !isStreamLoad"
        :data="chatList"
        :text-loading="loading"
        :is-stream-load="isStreamLoad"
        style="height: 100%"
        @scroll="handleChatScroll"
        @clear="clearConfirm"
    >
      <!-- eslint-disable vue/no-unused-vars -->
      <template #content="{ item, index }">
        <div v-if="item.role === 'assistant' && item.steps && item.steps.length > 0" class="agent-steps">
          <div class="agent-steps-header">📋 执行步骤 <span class="agent-steps-badge">{{ item.steps.length }}</span></div>
          <div class="agent-steps-list">
            <div v-for="(step, si) in item.steps" :key="si" class="agent-step-item">
              <div class="agent-step-dot" :class="getStepDotClass(step)"></div>
              <span class="agent-step-text">{{ step }}</span>
            </div>
          </div>
        </div>
        <t-chat-reasoning v-if="item.role === 'assistant'"  expand-icon-placement="right">
          <t-chat-loading v-if="isStreamLoad" text="思考中..." />
          <t-chat-content v-if="item.reasoning.length > 0" :content="item.reasoning" />
        </t-chat-reasoning>
        <div v-if="item.role === 'assistant' && item.jsonMarkdown" class="agent-json-md">
          <div class="agent-json-md-header" @click="toggleJsonMd(index)">
            <svg :class="['agent-json-md-arrow', { 'agent-json-md-arrow-expanded': jsonMdExpandedMap[index] }]" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
            <span class="agent-json-md-title">📊 分析报告</span>
          </div>
          <div v-show="jsonMdExpandedMap[index]" class="agent-json-md-content">
            <t-chat-content :content="item.jsonMarkdown" />
          </div>
        </div>
        <t-chat-content v-if="item.content.length > 0" :content="item.content" />
      </template>
      <template #actions="{ item, index }">
        <t-chat-action
            :content="item.content"
            :operation-btn="['copy']"
            @operation="handleOperation"
        />
        <span v-if="item.role === 'assistant' && !item.feedback" class="feedback-btns">
          <t-button size="small" variant="text" title="这个回答有用" @click="submitFeedback(item, 1)">👍</t-button>
          <t-button size="small" variant="text" title="这个回答没用" @click="submitFeedback(item, -1)">👎</t-button>
        </span>
        <span v-else-if="item.role === 'assistant' && item.feedback" class="feedback-done">{{ item.feedback === 1 ? '👍' : '👎' }}</span>
      </template>
      <template #footer>
<!--        <t-chat-input :stop-disabled="isStreamLoad" @send="inputEnter" @stop="onStop"> </t-chat-input>-->
          <t-chat-sender
              ref="chatSenderRef"
              v-model="inputValue"
              class="chat-sender"
              :textarea-props="{
                placeholder: '请输入消息...',
              }"
              :loading="loading"
              :stop-disabled="isStreamLoad"
              @send="inputEnter"
              @stop="onStop"
          >
            <template #suffix>
              <!-- 监听键盘回车发送事件需要在sender组件监听 -->
              <t-button theme="default" variant="text" size="large" class="btn" @click="inputEnter"> 发送 </t-button>
            </template>
            <template #prefix>
              <NFlex>
                <NSelect
                    v-model:value="selectValue"
                    :options="selectOptions"
                    label-field="name" value-field="ID"
                    size="tiny"
                    style="width: 200px;"
                />
                <NSelect
                    v-model:value="agentMode"
                    :options="agentModeOptions"
                    size="tiny"
                    style="width: 120px;"
                />
              </NFlex>
            </template>
          </t-chat-sender>

      </template>
    </t-chat>
    <t-button v-show="isShowToBottom" variant="text" class="bottomBtn" @click="backBottom">
      <div class="to-bottom">
        <ArrowDownIcon />
      </div>
    </t-button>
  </div>
</template>
<script setup lang="ts">
import {ref, onMounted, h, onBeforeUnmount, onBeforeMount} from 'vue';
import {ArrowDownIcon, CheckCircleIcon, SystemSumIcon} from 'tdesign-icons-vue-next';
const fetchCancel = ref(null);
const loading = ref(false);

const inputValue = ref('');
// 流式数据加载中
const isStreamLoad = ref(false);
let formatTimer = null

const chatRef = ref(null);
const isShowToBottom = ref(false);

const icon = ref('https://raw.githubusercontent.com/ArvinLovegood/go-stock/master/build/appicon.png');
import {darkTheme, NFlex, NImage,NSelect} from "naive-ui";
import {ChatWithAgent, GetAiConfigs, GetConfig, GetSponsorInfo, GetVersionInfo, SubmitAgentFeedback} from "../../wailsjs/go/main/App";
import {models} from '../../wailsjs/go/models';
import {EventsOff, EventsOn} from '../../wailsjs/runtime'
import 'tdesign-vue-next/es/style/index.css';


const allowToolTip = ref(true);
const chatSenderRef = ref(null);
const selectOptions = ref([]);
const selectValue = ref("default");
const agentMode = ref('deepagents')
const agentModeOptions = [
  { label: '🤖 自动', value: 'auto' },
  { label: '⚡ 快速', value: 'react' },
  { label: '🧠 规划', value: 'plan_execute' },
  { label: '🔬 DeepAgents', value: 'deepagents' },
]
const jsonMdExpandedMap = ref({})

function toggleJsonMd(index) {
  jsonMdExpandedMap.value = {
    ...jsonMdExpandedMap.value,
    [index]: !jsonMdExpandedMap.value[index]
  }
}

// 定义事件处理函数，方便在挂载和卸载时管理
function getStepDotClass(step) {
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

function startFormatTimer() {
  stopFormatTimer()
  formatTimer = setInterval(() => {
    const lastItem = chatList.value[0]
    if (lastItem && lastItem.role === 'assistant') {
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }, 1500)
}

function stopFormatTimer() {
  if (formatTimer) {
    clearInterval(formatTimer)
    formatTimer = null
  }
}

function formatMarkdown(content) {
  if (!content) return { content: '', jsonMarkdown: '' }

  const { content: cleaned, jsonMarkdown } = wrapInlineJson(content)

  let inCodeBlock = false
  const lines = cleaned.split('\n')
  const result = []

  for (let i = 0; i < lines.length; i++) {
    let line = lines[i]
    const trimmed = line.replace(/^[\t ]+/, '')

    if (trimmed.startsWith('```')) {
      inCodeBlock = !inCodeBlock
      if (!inCodeBlock) {
        result.push(trimmed)
        continue
      }
    }

    if (inCodeBlock) {
      result.push(line)
      continue
    }

    if (trimmed !== line && trimmed !== '') {
      line = trimmed
    }

    if (i > 0 && isBlockElement(trimmed)) {
      const prev = result.length > 0 ? result[result.length - 1] : ''
      if (prev !== '' && !isBlockElement(prev.replace(/^[\t ]+/, ''))) {
        result.push('')
      }
    }

    line = splitInlineHeading(line)

    result.push(line)
  }

  return {
    content: result.join('\n'),
    jsonMarkdown
  }
}

function hasMarkdownContent(str) {
  if (!str || typeof str !== 'string') return false
  return /(^|\n)\s*#{1,6}\s/.test(str) ||
    /(^|\n)\s*\|/.test(str) ||
    /(^|\n)\s*---/.test(str) ||
    /(^|\n)\s*[-*+]\s/.test(str) ||
    /(^|\n)\s*>\s/.test(str) ||
    /(^|\n)\s*```/.test(str)
}

function extractMarkdownFromJson(obj) {
  if (typeof obj === 'string') return obj
  if (Array.isArray(obj)) {
    const items = obj.map(item => typeof item === 'string' ? item : JSON.stringify(item, null, 2))
    return items.join('\n\n')
  }
  if (typeof obj === 'object' && obj !== null) {
    for (const key of ['response', 'content', 'text', 'result', 'answer', 'message', 'output']) {
      if (obj[key] != null) {
        const val = obj[key]
        if (typeof val === 'string' && hasMarkdownContent(val)) return val
        if (typeof val === 'object') {
          const extracted = extractMarkdownFromJson(val)
          if (extracted) return extracted
        }
      }
    }
    const values = Object.values(obj).filter(v => typeof v === 'string' && hasMarkdownContent(v))
    if (values.length > 0) return values.join('\n\n')
    const strValues = Object.values(obj).filter(v => typeof v === 'string')
    if (strValues.length > 0) return strValues.join('\n\n')
  }
  return null
}

function wrapInlineJson(content) {
  if (!content) return { content: '', jsonMarkdown: '' }
  const cleaned = []
  const jsonParts = []
  let i = 0
  const len = content.length
  let inCodeBlock = false

  while (i < len) {
    if (content.substring(i, i + 3) === '```') {
      inCodeBlock = !inCodeBlock
      cleaned.push('```')
      i += 3
      continue
    }

    if (inCodeBlock) {
      cleaned.push(content[i])
      i++
      continue
    }

    if (content[i] === '{') {
      const end = findJsonEnd(content, i)
      if (end > i) {
        const jsonStr = content.substring(i, end + 1)
        try {
          const obj = JSON.parse(jsonStr)
          const md = extractMarkdownFromJson(obj)
          if (md) {
            jsonParts.push(md)
          } else {
            cleaned.push('\n\n```json\n' + jsonStr + '\n```\n\n')
          }
          i = end + 1
          continue
        } catch {}
      }
    }
    cleaned.push(content[i])
    i++
  }

  return {
    content: cleaned.join(''),
    jsonMarkdown: jsonParts.join('\n\n---\n\n')
  }
}

function findJsonEnd(content, start) {
  let depth = 0
  let bracketDepth = 0
  let inStr = false
  let escape = false
  for (let i = start; i < content.length; i++) {
    const ch = content[i]
    if (escape) { escape = false; continue }
    if (ch === '\\' && inStr) { escape = true; continue }
    if (ch === '"') { inStr = !inStr; continue }
    if (inStr) continue
    if (ch === '[') bracketDepth++
    else if (ch === ']') bracketDepth--
    else if (ch === '{') depth++
    else if (ch === '}') {
      depth--
      if (depth === 0 && bracketDepth === 0) return i
    }
  }
  return -1
}

function splitInlineHeading(line) {
  const match = line.match(/(#{1,6}\s+\S)/)
  if (!match) return line
  const idx = match.index
  if (idx === 0) return line
  const prefix = line.substring(0, idx)
  if (prefix.trim() === '') return line
  return prefix + '\n\n' + line.substring(idx)
}

function isBlockElement(line) {
  if (!line || line.length === 0) return false
  if (line[0] === '#') return true
  if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('+ ')) return true
  if (line.startsWith('```')) return true
  if (line.startsWith('> ')) return true
  if (line.length >= 2 && line[0] >= '1' && line[0] <= '9' && line[1] === '.') return true
  if (line.startsWith('---') || line.startsWith('***') || line.startsWith('___')) return true
  if (line.startsWith('|')) return true
  return false
}

function parseStepText(text) {
  if (!text) return [text]
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return [text]
  try {
    const obj = JSON.parse(trimmed)
    if (Array.isArray(obj)) {
      return obj.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
    }
    if (typeof obj === 'object' && obj !== null) {
      const steps = obj.steps || obj.step || obj.plan || obj.items || obj.list
      if (Array.isArray(steps)) {
        return steps.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
      }
      const entries = Object.entries(obj)
      if (entries.length > 0) {
        return entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
      }
    }
    return [text]
  } catch {
    return [text]
  }
}

const handleAgentMessage = (data) => {
  console.log(data)
  if(data['role']==="assistant"){
    loading.value = false;
    const lastItem = chatList.value[0];
    if (data['reasoning_content']){
      const rc = data['reasoning_content']
      if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!lastItem.steps) lastItem.steps = []
          const parsed = parseStepText(stepText)
          lastItem.steps.push(...parsed)
        }
      } else {
        lastItem.rawReasoning = (lastItem.rawReasoning || '') + rc
        lastItem.reasoning = lastItem.rawReasoning
      }
    }
    if (data['content']){
      lastItem.rawContent = (lastItem.rawContent || '') + data['content']
      lastItem.content = lastItem.rawContent
    }
    if(data['tool_calls']){
      for (const tool of  data['tool_calls']) {
          console.log(tool.id, tool.type, tool.function.name, tool.function.arguments);
        lastItem.reasoning += "\n```"+tool.function.name+"\n" +
            "参数："+ (tool.function.arguments?tool.function.arguments:"无")+
            "\n```\n";
      }
    }
  }
  if(data['response_meta']&&data['response_meta'].finish_reason==="stop"){
    isStreamLoad.value = false;
    loading.value = false;
    stopFormatTimer()
    const lastItem = chatList.value[0];
    if (lastItem) {
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }
}

onBeforeUnmount(() => {
  EventsOff("agent-message", handleAgentMessage)
})

onBeforeMount(() => {
  // 每次挂载前都重新注册事件监听
  EventsOn("agent-message", handleAgentMessage)
  GetAiConfigs().then(res=>{
    console.log(res)
    selectOptions.value = res
    selectValue.value = res[0].ID
  })
})

onMounted(() => {
  //chatRef.value.scrollToBottom();

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute("theme-mode", "dark");
    } else {
      document.documentElement.removeAttribute("theme-mode");    }
  })


  GetVersionInfo().then((res) => {
    icon.value = res.icon;
  });

});

// 滚动到底部
const backBottom = () => {
  chatRef.value.scrollToBottom({
    behavior: 'smooth',
  });
};
// 是否显示回到底部按钮
const handleChatScroll = function ({ e }) {
  const scrollTop = e.target.scrollTop;
  isShowToBottom.value = scrollTop < 0;
};
// 清空消息
const clearConfirm = function () {
  chatList.value = [];
};
const handleOperation = function (type, options) {
  console.log('handleOperation', type, options);
};

// 提交对某条回答的反馈（👍/👎）
const submitFeedback = function (item, rating) {
  if (!item) return;
  const fb = models.AgentFeedback.createFrom({
    sessionId: '',
    question: item.question || '',
    response: item.rawContent || item.content || '',
    rating: rating,
    reason: '',
    mode: agentMode.value === 'auto' ? '' : agentMode.value,
  });
  SubmitAgentFeedback(fb).then(() => {
    item.feedback = rating;
    window.$message && window.$message.success(rating === 1 ? '感谢反馈，我会继续优化' : '已收到，我会改进');
  }).catch((e) => {
    console.error('submit feedback error', e);
  });
};
// 倒序渲染
const chatList = ref([
  // {
  //   content: `模型由<span>hunyuan</span>变为<span>GPT4</span>`,
  //   role: 'model-change',
  //   reasoning: '',
  // },
  {
    avatar: h(NImage, { src: icon.value, height: '48px', width: '48px'}),
    name: 'Go-Stock AI',
    datetime: '',
    reasoning: '',
    content: '我是您的AI赋能股票分析助手,您可以问我任何关于股票投资方面的问题。',
    role: 'assistant',
    duration: 10,
  },
  {
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: '',
    content: '介绍下自己？',
    role: 'user',
    reasoning: '',
  },
]);

const onStop = function () {
  if (fetchCancel.value) {
    fetchCancel.value.controller.close();
    loading.value = false;
    isStreamLoad.value = false;
  }
  stopFormatTimer()
  const lastItem = chatList.value[0]
  if (lastItem && lastItem.role === 'assistant') {
    if (lastItem.rawContent) {
      const fmt = formatMarkdown(lastItem.rawContent)
      lastItem.content = fmt.content
      if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
    }
    if (lastItem.rawReasoning) {
      const fmt = formatMarkdown(lastItem.rawReasoning)
      lastItem.reasoning = fmt.content
    }
  }
};

const inputEnter = function () {
  if (isStreamLoad.value) {
    return;
  }
  if (!inputValue.value) return;
  const params = {
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: new Date().toDateString(),
    content: inputValue.value,
    role: 'user',
  };
  chatList.value.unshift(params);
  // 空消息占位
  const params2 = {
    avatar:  h(NImage, { src: icon.value, height: '48px', width: '48px'}),
    name: 'Go-Stock AI',
    datetime: new Date().toDateString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    jsonMarkdown: '',
    question: inputValue.value,
    feedback: 0,
    role: 'assistant',
  };
  chatList.value.unshift(params2);
  loading.value = true;
  isStreamLoad.value = true;
  startFormatTimer()
  jsonMdExpandedMap.value = { ...jsonMdExpandedMap.value, [0]: true }
  ChatWithAgent(inputValue.value,selectValue.value,0,false,0,false,agentMode.value === 'auto' ? '' : agentMode.value,'','')
};
</script>
<style lang="less">
/* 应用滚动条样式 */
::-webkit-scrollbar-thumb {
  background-color: var(--td-scrollbar-color);
}
::-webkit-scrollbar-thumb:horizontal:hover {
  background-color: var(--td-scrollbar-hover-color);
}
::-webkit-scrollbar-track {
  background-color: var(--td-scroll-track-color);
}
.chat-box {
  position: relative;
  height: 100%;
  margin: 5px 10px 5px 10px;
  text-align: left;
  .feedback-btns {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    margin-left: 4px;
    vertical-align: middle;
  }
  .feedback-done {
    margin-left: 6px;
    opacity: 0.7;
    vertical-align: middle;
  }
  .bottomBtn {
    position: absolute;
    left: 50%;
    margin-left: -20px;
    bottom: 210px;
    padding: 0;
    border: 0;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    box-shadow: 0px 8px 10px -5px rgba(0, 0, 0, 0.08), 0px 16px 24px 2px rgba(0, 0, 0, 0.04),
    0px 6px 30px 5px rgba(0, 0, 0, 0.05);
  }
  .to-bottom {
    width: 40px;
    height: 40px;
    border: 1px solid #dcdcdc;
    box-sizing: border-box;
    background: var(--td-bg-color-container);
    border-radius: 50%;
    font-size: 24px;
    line-height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    .t-icon {
      font-size: 24px;
    }
  }
}

.model-select {
  display: flex;
  align-items: center;
  .t-select {
    width: 112px;
    height: 32px;
    margin-right: 8px;
    .t-input {
      border-radius: 32px;
      padding: 0 15px;
    }
  }
  .check-box {
    width: 112px;
    height: 32px;
    border-radius: 32px;
    border: 0;
    background: #e7e7e7;
    color: rgba(0, 0, 0, 0.9);
    box-sizing: border-box;
    flex: 0 0 auto;
    .t-button__text {
      display: flex;
      align-items: center;
      justify-content: center;
      span {
        margin-left: 4px;
      }
    }
  }
  .check-box.is-active {
    border: 1px solid #d9e1ff;
    background: #f2f3ff;
    color: var(--td-brand-color);
  }
}


.chat-sender {
  .btn {
    color: var(--td-text-color-disabled);
    border: none;
    &:hover {
      color: var(--td-brand-color-hover);
      border: none;
      background: none;
    }
  }
  .btn.t-button {
    height: var(--td-comp-size-m);
    padding: 0;
  }
  .model-select {
    display: flex;
    align-items: center;
    .t-select {
      width: 112px;
      height: var(--td-comp-size-m);
      margin-right: var(--td-comp-margin-s);
      .t-input {
        border-radius: 32px;
        padding: 0 15px;
      }
      .t-input.t-is-focused {
        box-shadow: none;
      }
    }
    .check-box {
      width: 112px;
      height: var(--td-comp-size-m);
      border-radius: 32px;
      border: 0;
      background: var(--td-bg-color-component);
      color: var(--td-text-color-primary);
      box-sizing: border-box;
      flex: 0 0 auto;
      .t-button__text {
        display: flex;
        align-items: center;
        justify-content: center;
        span {
          margin-left: var(--td-comp-margin-xs);
        }
      }
    }
    .check-box.is-active {
      border: 1px solid var(--td-brand-color-focus);
      background: var(--td-brand-color-light);
      color: var(--td-text-color-brand);
    }
  }
}

.agent-steps {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.06) 0%, rgba(46, 139, 87, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
}
.agent-steps-badge {
  font-size: 11px;
  background: var(--td-brand-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.agent-steps-list {
  padding: 8px 10px 8px 14px;
  max-height: 250px;
  overflow-y: auto;
}
.agent-step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  position: relative;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}
.agent-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 16px;
  bottom: -3px;
  width: 1px;
  background: var(--td-component-border);
}
.agent-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--td-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.agent-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.agent-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.agent-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.agent-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}

.agent-json-md {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.06) 0%, rgba(5, 150, 105, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
  transition: background 0.2s;
}
.agent-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.12) 0%, rgba(5, 150, 105, 0.12) 100%);
}
.agent-json-md-arrow {
  transition: transform 0.2s;
  flex-shrink: 0;
}
.agent-json-md-arrow-expanded {
  transform: rotate(90deg);
}
.agent-json-md-title {
  font-size: 13px;
  font-weight: 500;
}
.agent-json-md-content {
  padding: 10px;
  max-height: 500px;
  overflow-y: auto;
  text-align: left;
}

</style>