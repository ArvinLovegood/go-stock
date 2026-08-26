<script setup>
import {reactive, ref, watch} from 'vue'
import {PromptPlazaRequest} from "../../wailsjs/go/main/App"
import {useMessage} from "naive-ui"

/**
 * 提示词/技能广场共享「绑定邮箱」弹窗（仅未绑定邮箱的旧账号）
 * 服务端契约（均需 Authorization）：
 *   POST /user/bind-email/send-code  {email} -> data.ttl（服务端 60s 重发冷却）
 *   POST /user/bind-email            {email, code(6位)} -> data.email
 * 绑定成功后 emit('bound', {email})，父组件刷新 currentUser。
 */
const props = defineProps({
  show: {type: Boolean, default: false},
  apiBase: {type: String, required: true},
  token: {type: String, default: ''},
  username: {type: String, default: ''}
})

const emit = defineEmits(['update:show', 'bound'])

const message = useMessage()

const bindForm = reactive({
  email: '',
  code: ''
})

const sendLoading = ref(false)
const bindLoading = ref(false)
const codeSent = ref(false)
const codeCountdown = ref(0)
let countdownTimer = null

watch(() => props.show, (val) => {
  if (val) {
    bindForm.email = ''
    bindForm.code = ''
    codeSent.value = false
    codeCountdown.value = 0
    if (countdownTimer) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
  }
})

async function apiPost(path, body = null) {
  const resp = await PromptPlazaRequest('POST', props.apiBase, path, null, body ? JSON.stringify(body) : '', props.token)
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp
}

function startCountdown(seconds) {
  codeCountdown.value = seconds
  if (countdownTimer) clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    codeCountdown.value--
    if (codeCountdown.value <= 0) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
  }, 1000)
}

async function handleSendCode() {
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(bindForm.email.trim())) {
    message.warning('请输入有效的邮箱地址')
    return
  }
  sendLoading.value = true
  try {
    const resp = await apiPost('/user/bind-email/send-code', {email: bindForm.email.trim()})
    codeSent.value = true
    // 服务端 60s 重发冷却，前端同步倒计时
    startCountdown(60)
    message.success(resp.data?.message || '验证码已发送至该邮箱，请查收（注意垃圾邮件箱）')
  } catch (e) {
    message.error(e.message)
  } finally {
    sendLoading.value = false
  }
}

async function handleBind() {
  if (!/^\d{6}$/.test(bindForm.code.trim())) {
    message.warning('请输入 6 位邮箱验证码')
    return
  }
  bindLoading.value = true
  try {
    const resp = await apiPost('/user/bind-email', {
      email: bindForm.email.trim(),
      code: bindForm.code.trim()
    })
    message.success(resp.message || '邮箱绑定成功')
    const email = resp.data?.email || bindForm.email.trim()
    emit('update:show', false)
    emit('bound', {email})
  } catch (e) {
    message.error(e.message)
  } finally {
    bindLoading.value = false
  }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    style="width: 400px"
    title="绑定邮箱"
    @update:show="val => emit('update:show', val)"
  >
    <n-space vertical :size="12">
      <n-alert type="info" :show-icon="true" style="font-size: 13px">
        账号 {{ username || '当前账号' }} 尚未绑定邮箱，绑定后可通过邮箱找回密码。
      </n-alert>
      <n-input v-model:value="bindForm.email" placeholder="邮箱地址" :disabled="codeSent" />
      <template v-if="codeSent">
        <n-space align="center" :size="8">
          <n-input
            v-model:value="bindForm.code"
            placeholder="6 位邮箱验证码"
            style="width: 160px"
            maxlength="6"
            @keyup.enter="handleBind"
          />
          <n-button size="small" :disabled="codeCountdown > 0" @click="handleSendCode">
            {{ codeCountdown > 0 ? `重发(${codeCountdown}s)` : '重发验证码' }}
          </n-button>
        </n-space>
        <n-button type="primary" block :loading="bindLoading" @click="handleBind">确认绑定</n-button>
      </template>
      <n-button v-else type="primary" block :loading="sendLoading" @click="handleSendCode">发送验证码</n-button>
      <n-button v-if="codeSent" quaternary size="tiny" @click="codeSent = false; bindForm.code = ''">更换邮箱地址</n-button>
    </n-space>
  </n-modal>
</template>
