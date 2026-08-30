<script setup>
import {nextTick, reactive, ref, watch} from 'vue'
import {PromptPlazaRequest} from "../../wailsjs/go/main/App"
import {useMessage} from "naive-ui"

/**
 * 提示词/技能广场共享账号弹窗：登录 / 注册 / 忘记密码（两步邮箱验证找回）
 * 服务端契约：
 *   POST /auth/register/send-code           {email, captchaId, captchaCode} -> data.ttl（图形验证码防刷）
 *   POST /auth/register                     {username(3-50), password(6-100), email, code(6位), nickname(可选)} -> data.token+data.user
 *   POST /auth/login                        {username, password} -> data.token+data.user
 *   GET  /trial/captcha                     -> data.{captchaId, image(base64 data URL), ttl}
 *   POST /auth/forgot-password/send-code    {username, email, captchaId, captchaCode} -> data.ttl
 *   POST /auth/forgot-password/reset        {username, email, code(6位), newPassword(6-100)}
 * 登录/注册成功后 emit('logged-in', {token, user})，由父组件保存 token 并刷新数据。
 */
const props = defineProps({
  show: {type: Boolean, default: false},
  tab: {type: String, default: 'login'},
  apiBase: {type: String, required: true},
  // VIP 强制登录模式：弹窗不可关闭（沿用 promptPlaza 原有交互）
  vipRequireLogin: {type: Boolean, default: false}
})

const emit = defineEmits(['update:show', 'update:tab', 'logged-in'])

const message = useMessage()

const loginForm = reactive({
  username: localStorage.getItem('promptPlazaUsername') || '',
  password: localStorage.getItem('promptPlazaPassword') || ''
})

const registerForm = reactive({
  username: '',
  password: '',
  email: '',
  nickname: '',
  // 注册邮箱验证码（图形验证码防刷 → 邮箱验证码 → 提交注册）
  captchaId: '',
  captchaImage: '',
  captchaLoading: false,
  captchaCode: '',
  code: ''
})

// 忘记密码两步流：1 校验账号+邮箱+图形验证码并发送邮箱验证码；2 验证码+新密码重置
const forgot = reactive({
  step: 1,
  username: '',
  email: '',
  captchaId: '',
  captchaImage: '',
  captchaLoading: false,
  captchaCode: '',
  code: '',
  newPassword: '',
  confirmPassword: ''
})

const loginLoading = ref(false)
const registerLoading = ref(false)
const sendCodeLoading = ref(false)
const resetLoading = ref(false)
const regSendLoading = ref(false)

// 通用倒计时（忘记密码 / 注册发码共用模式）
function makeCountdown() {
  const sec = ref(0)
  let timer = null
  const start = (seconds) => {
    sec.value = seconds
    if (timer) clearInterval(timer)
    timer = setInterval(() => {
      sec.value--
      if (sec.value <= 0) {
        clearInterval(timer)
        timer = null
      }
    }, 1000)
  }
  return {sec, start}
}

const forgotCountdown = makeCountdown()
const registerCountdown = makeCountdown()
// 顶层导出以便模板自动解包
const forgotCodeCountdown = forgotCountdown.sec
const registerCodeCountdown = registerCountdown.sec

async function apiPost(path, body = null) {
  const resp = await PromptPlazaRequest('POST', props.apiBase, path, null, body ? JSON.stringify(body) : '', '')
  if (resp.code !== 0) {
    throw new Error(resp.message || '请求失败')
  }
  return resp
}

// 图形验证码加载到指定表单（忘记密码 / 注册共用）
async function loadCaptchaInto(target) {
  target.captchaLoading = true
  try {
    const resp = await PromptPlazaRequest('GET', props.apiBase, '/trial/captcha', null, '', '')
    if (resp.code !== 0) {
      throw new Error(resp.message || '获取验证码失败')
    }
    target.captchaId = resp.data.captchaId || ''
    target.captchaImage = resp.data.image || ''
    target.captchaCode = ''
  } catch (e) {
    message.error(e.message)
  } finally {
    target.captchaLoading = false
  }
}

function loadCaptcha() {
  return loadCaptchaInto(forgot)
}

function loadRegisterCaptcha() {
  return loadCaptchaInto(registerForm)
}

// 打开忘记密码/注册页签时刷新对应图形验证码
watch(() => props.tab, (val) => {
  if (val === 'forgot') {
    nextTick(loadCaptcha)
  } else if (val === 'register') {
    nextTick(loadRegisterCaptcha)
  }
})
watch(() => props.show, (val) => {
  if (val && (props.tab === 'forgot' || props.tab === 'register')) {
    nextTick(() => loadCaptchaInto(props.tab === 'forgot' ? forgot : registerForm))
  }
})

function switchTab(tab) {
  emit('update:tab', tab)
}

function onLoggedIn(data, username, password) {
  localStorage.setItem('promptPlazaToken', data.token)
  localStorage.setItem('promptPlazaUsername', username)
  localStorage.setItem('promptPlazaPassword', password)
  emit('update:show', false)
  emit('logged-in', data)
}

async function handleLogin() {
  if (!loginForm.username || !loginForm.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loginLoading.value = true
  try {
    const resp = await apiPost('/auth/login', {
      username: loginForm.username,
      password: loginForm.password
    })
    onLoggedIn(resp.data, loginForm.username, loginForm.password)
    message.success('登录成功')
  } catch (e) {
    message.error('登录失败: ' + e.message)
  } finally {
    loginLoading.value = false
  }
}

// 注册发码：图形验证码 + 邮箱 → /auth/register/send-code
async function handleRegisterSendCode() {
  if (!registerForm.email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(registerForm.email.trim())) {
    message.warning('请先输入有效的邮箱地址')
    return
  }
  if (!registerForm.captchaCode.trim()) {
    message.warning('请先填写下方图形验证码后再发送')
    return
  }
  regSendLoading.value = true
  try {
    const resp = await apiPost('/auth/register/send-code', {
      email: registerForm.email.trim(),
      captchaId: registerForm.captchaId,
      captchaCode: registerForm.captchaCode.trim()
    })
    registerCountdown.start(60)
    message.success(resp.data?.message || '验证码已发送至该邮箱，请查收（注意垃圾邮件箱）')
  } catch (e) {
    message.error(e.message)
  } finally {
    // 图形验证码一次性（无论成败均已消耗），刷新供下次重发使用
    loadRegisterCaptcha()
    regSendLoading.value = false
  }
}

async function handleRegister() {
  if (!registerForm.username || registerForm.username.trim().length < 3) {
    message.warning('用户名至少 3 个字符')
    return
  }
  if (!registerForm.password || registerForm.password.length < 6) {
    message.warning('密码至少 6 位')
    return
  }
  if (!registerForm.email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(registerForm.email.trim())) {
    message.warning('请输入有效的邮箱地址')
    return
  }
  if (!/^\d{6}$/.test(registerForm.code.trim())) {
    message.warning('请输入 6 位邮箱验证码（点击"发送验证码"获取）')
    return
  }
  registerLoading.value = true
  try {
    const resp = await apiPost('/auth/register', {
      username: registerForm.username.trim(),
      password: registerForm.password,
      email: registerForm.email.trim(),
      code: registerForm.code.trim(),
      nickname: registerForm.nickname.trim()
    })
    onLoggedIn(resp.data, registerForm.username.trim(), registerForm.password)
    registerForm.username = ''
    registerForm.password = ''
    registerForm.email = ''
    registerForm.nickname = ''
    registerForm.code = ''
    message.success('注册成功')
  } catch (e) {
    message.error('注册失败: ' + e.message)
  } finally {
    registerLoading.value = false
  }
}

async function handleForgotSendCode() {
  if (!forgot.username.trim()) {
    message.warning('请输入用户名')
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(forgot.email.trim())) {
    message.warning('请输入有效的邮箱地址')
    return
  }
  if (!forgot.captchaCode.trim()) {
    message.warning('请输入图形验证码')
    return
  }
  sendCodeLoading.value = true
  try {
    const resp = await apiPost('/auth/forgot-password/send-code', {
      username: forgot.username.trim(),
      email: forgot.email.trim(),
      captchaId: forgot.captchaId,
      captchaCode: forgot.captchaCode.trim()
    })
    forgot.step = 2
    // 服务端 60s 重发冷却，前端同步倒计时（ttl 为邮箱验证码有效期，取小者）
    const cooldown = 60
    const ttl = resp.data?.ttl || 600
    forgotCountdown.start(Math.min(cooldown, ttl))
    message.success(resp.data?.message || '验证码已发送至该邮箱，请查收（注意垃圾邮件箱）')
  } catch (e) {
    message.error(e.message)
    // 图形验证码一次性，失败后刷新
    loadCaptcha()
  } finally {
    sendCodeLoading.value = false
  }
}

// 服务端每次发送邮箱验证码都要求新的图形验证码，重发须回到第一步重新校验
function backToForgotStep1() {
  forgot.step = 1
  forgot.captchaCode = ''
  loadCaptcha()
}

async function handleForgotReset() {
  if (!/^\d{6}$/.test(forgot.code.trim())) {
    message.warning('请输入 6 位邮箱验证码')
    return
  }
  if (!forgot.newPassword || forgot.newPassword.length < 6) {
    message.warning('新密码至少 6 位')
    return
  }
  if (forgot.newPassword !== forgot.confirmPassword) {
    message.warning('两次输入的密码不一致')
    return
  }
  resetLoading.value = true
  try {
    const resp = await apiPost('/auth/forgot-password/reset', {
      username: forgot.username.trim(),
      email: forgot.email.trim(),
      code: forgot.code.trim(),
      newPassword: forgot.newPassword
    })
    message.success(resp.message || '密码重置成功，请使用新密码登录')
    // 回到登录页签并预填用户名/新密码
    loginForm.username = forgot.username.trim()
    loginForm.password = forgot.newPassword
    forgot.step = 1
    forgot.username = ''
    forgot.email = ''
    forgot.captchaCode = ''
    forgot.code = ''
    forgot.newPassword = ''
    forgot.confirmPassword = ''
    switchTab('login')
  } catch (e) {
    message.error(e.message)
  } finally {
    resetLoading.value = false
  }
}

function goForgot() {
  forgot.step = 1
  switchTab('forgot')
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    style="width: 400px"
    :title="vipRequireLogin ? '🎉 VIP专属福利' : '账号'"
    :closable="!vipRequireLogin"
    :maskClosable="!vipRequireLogin"
    :closeOnEsc="!vipRequireLogin"
    @update:show="val => emit('update:show', val)"
  >
    <div v-if="vipRequireLogin" style="margin-bottom: 16px; padding: 12px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 8px; color: #fff">
      <div style="font-size: 15px; font-weight: 600; margin-bottom: 8px">✨ 欢迎回来，VIP用户！</div>
      <div style="font-size: 13px; line-height: 1.6; opacity: 0.95">
        登录后即可解锁专属权益：
        <div style="margin-top: 6px; padding-left: 8px">
          📖 查看 <b>VIP专属提示词</b>，获取更精准的分析策略<br/>
          🔒 自动绑定当前设备，保障账号安全<br/>
          💡 与社区用户共享投资灵感
        </div>
      </div>
    </div>
    <n-tabs :value="tab" type="line" @update:value="switchTab">
      <n-tab-pane name="login" tab="登录">
        <n-space vertical :size="12">
          <n-input v-model:value="loginForm.username" placeholder="用户名" @keyup.enter="handleLogin" />
          <n-input v-model:value="loginForm.password" type="password" placeholder="密码" show-password-on="click" @keyup.enter="handleLogin" />
          <n-button type="primary" block :loading="loginLoading" @click="handleLogin">登录</n-button>
          <n-button quaternary size="tiny" style="align-self: flex-end" @click="goForgot">忘记密码？</n-button>
        </n-space>
      </n-tab-pane>
      <n-tab-pane name="register" tab="注册">
        <n-space vertical :size="12">
          <n-input v-model:value="registerForm.username" placeholder="用户名 (3-50字)" />
          <n-input v-model:value="registerForm.password" type="password" placeholder="密码 (6字以上)" show-password-on="click" />
          <n-input v-model:value="registerForm.email" placeholder="邮箱 (用于找回密码)" />
          <!-- 邮箱验证码：紧跟邮箱下方 -->
          <n-space align="center" :size="8">
            <n-input
              v-model:value="registerForm.code"
              placeholder="6 位邮箱验证码"
              style="flex: 1; min-width: 0"
              maxlength="6"
              @keyup.enter="handleRegister"
            />
            <n-button size="small" :loading="regSendLoading" :disabled="registerCodeCountdown > 0" @click="handleRegisterSendCode">
              {{ registerCodeCountdown > 0 ? `重发(${registerCodeCountdown}s)` : '发送验证码' }}
            </n-button>
          </n-space>
          <!-- 图形验证码：发送邮箱验证码前需先通过人机校验 -->
          <n-space align="center" :size="8">
            <img
              v-if="registerForm.captchaImage"
              :src="registerForm.captchaImage"
              title="点击刷新验证码"
              style="height: 34px; border-radius: 3px; cursor: pointer"
              @click="loadRegisterCaptcha"
            />
            <n-button v-else size="small" :loading="registerForm.captchaLoading" @click="loadRegisterCaptcha">加载验证码</n-button>
            <n-input v-model:value="registerForm.captchaCode" placeholder="图形验证码 (发送前需填写)" style="flex: 1; min-width: 0" maxlength="5" />
          </n-space>
          <n-input v-model:value="registerForm.nickname" placeholder="昵称 (可选)" />
          <n-button type="primary" block :loading="registerLoading" @click="handleRegister">注册</n-button>
          <n-text depth="3" style="font-size: 12px">发送邮箱验证码前请先填写图形验证码，验证码 10 分钟内有效</n-text>
        </n-space>
      </n-tab-pane>
      <n-tab-pane name="forgot" tab="忘记密码">
        <!-- 第一步：账号 + 邮箱 + 图形验证码 -->
        <n-space v-if="forgot.step === 1" vertical :size="12">
          <n-input v-model:value="forgot.username" placeholder="用户名" />
          <n-input v-model:value="forgot.email" placeholder="绑定的邮箱地址" />
          <n-space align="center" :size="8">
            <img
              v-if="forgot.captchaImage"
              :src="forgot.captchaImage"
              title="点击刷新验证码"
              style="height: 34px; border-radius: 3px; cursor: pointer"
              @click="loadCaptcha"
            />
            <n-button v-else size="small" :loading="forgot.captchaLoading" @click="loadCaptcha">加载验证码</n-button>
            <n-input
              v-model:value="forgot.captchaCode"
              placeholder="图形验证码"
              style="width: 140px"
              maxlength="5"
              @keyup.enter="handleForgotSendCode"
            />
          </n-space>
          <n-button type="primary" block :loading="sendCodeLoading" @click="handleForgotSendCode">发送邮箱验证码</n-button>
          <n-text depth="3" style="font-size: 12px">将向该账号绑定的邮箱发送 6 位验证码，10 分钟内有效</n-text>
        </n-space>
        <!-- 第二步：邮箱验证码 + 新密码 -->
        <n-space v-else vertical :size="12">
          <n-alert type="info" :show-icon="true" style="font-size: 13px">
            验证码已发送至 {{ forgot.email }}，请查收（注意垃圾邮件箱）
          </n-alert>
          <n-space align="center" :size="8">
            <n-input v-model:value="forgot.code" placeholder="6 位邮箱验证码" style="width: 160px" maxlength="6" @keyup.enter="handleForgotReset" />
            <n-button size="small" :disabled="forgotCodeCountdown > 0" @click="backToForgotStep1">
              {{ forgotCodeCountdown > 0 ? `重发(${forgotCodeCountdown}s)` : '重新发送' }}
            </n-button>
          </n-space>
          <n-input v-model:value="forgot.newPassword" type="password" placeholder="新密码 (6字以上)" show-password-on="click" />
          <n-input v-model:value="forgot.confirmPassword" type="password" placeholder="确认新密码" show-password-on="click" @keyup.enter="handleForgotReset" />
          <n-button type="primary" block :loading="resetLoading" @click="handleForgotReset">重置密码</n-button>
          <n-button quaternary size="tiny" @click="backToForgotStep1">返回上一步</n-button>
        </n-space>
      </n-tab-pane>
    </n-tabs>
  </n-modal>
</template>
