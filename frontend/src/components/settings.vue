<script setup>
import {computed, h, onBeforeUnmount, onMounted, reactive, ref} from "vue";
import {useRouter} from "vue-router";
import {
  AddPrompt,
  DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  TestDingDingNotice,
  TestFeishuNotice,
  StartFeishuBot,
  StopFeishuBot,
  GetFeishuBotStatus,
  UpdateConfig,
  UpdateAiConfigs,
  CheckSponsorCode,
  PromptPlazaRequest,
  GetVersionInfo,
} from "../../wailsjs/go/main/App";
import {NTag, NTooltip, NIcon, useMessage, useDialog} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";
import PlazaAuthModal from './plazaAuthModal.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

// 合约行情相关配置（启用合约行情开关、合约代理）在 custom 编译中隐藏；
// 普通编译（build-prod / build-dev 等）正常显示
const isCustomBuild = ref(false)
GetVersionInfo().then(result => {
  isCustomBuild.value = result.customBuild === true
}).catch(err => {
  console.error("GetVersionInfo error:", err)
})

// ===== 账号（与「提示词广场」共用同一套账号与 token） =====
const plazaToken = ref(localStorage.getItem('promptPlazaToken') || '')
const plazaUser = ref(null)
const plazaUserLoading = ref(false)
const plazaAuth = reactive({show: false, tab: 'login'})

function openPlazaAuth(tab) {
  plazaAuth.tab = tab || 'login'
  plazaAuth.show = true
}

// 拉取当前登录账号信息；token 失效时清除本地登录态，避免界面停留在「已登录」
async function fetchPlazaUser() {
  if (!plazaToken.value) {
    plazaUser.value = null
    return
  }
  plazaUserLoading.value = true
  try {
    const resp = await PromptPlazaRequest('GET', formValue.value.promptPlazaApiBase, '/user/me', null, '', plazaToken.value)
    if (resp.code !== 0) {
      throw new Error(resp.message || '获取账号信息失败')
    }
    plazaUser.value = resp.data
  } catch (e) {
    plazaToken.value = ''
    plazaUser.value = null
    localStorage.removeItem('promptPlazaToken')
  } finally {
    plazaUserLoading.value = false
  }
}

function onPlazaLoggedIn(data) {
  plazaToken.value = data?.token || localStorage.getItem('promptPlazaToken') || ''
  plazaUser.value = data?.user || null
  fetchPlazaUser()
}

function handlePlazaLogout() {
  dialog.warning({
    title: '提示',
    content: '确定要退出登录吗？退出后将无法同步 VIP、分享提示词与技能。',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      plazaToken.value = ''
      plazaUser.value = null
      localStorage.removeItem('promptPlazaToken')
      message.success('已退出登录')
    }
  })
}

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
    enable: true, // AI诊股默认开启
    aiConfigs: [], // AI配置列表
    prompt: "",
    questionTemplate: "{{stockName}}分析和总结",
    crawlTimeOut: 30,
    kDays: 30,
    httpProxy:"",
    httpProxyEnabled:false,
    httpProxyMode:"overseas",
    httpProxyMode:"overseas",
  },
  enableDanmu: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enableContracts: true,
  enablePushNews: true,
  enableOnlyPushRedNews: false,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
  binanceProxy:"",
  bitgetProxy:"",
  okxApiKey:"",
  okxSecretKey:"",
  okxPassphrase:"",
  okxLiveApiKey:"",
  okxLiveSecretKey:"",
  okxLivePassphrase:"",
  okxEnableTrade:false,
  okxSimulated:false,
  okxSignalTradeEnable:false,
  okxSignalTradeUSDT:"100",
  okxSignalTradeLive:false,
  okxSignalTradeLiveUSDT:"10",
  okxSignalTradeAmountMode:"fixed",
  okxSignalTradeAmountRatio:"0.25",
  okxRiskMaxPositions:"3",
  okxRiskMaxMarginRatio:"30",
  okxSignalTradeInstType:"SWAP",
  okxSignalTradeLever:"",
  okxSignalTradeKlts:"",
  okxSignalTradeMinScore:"",
  okxSignalTradeCooldownMin:"",
  okxSignalTradeDirMode:"both",
  okxSignalTradeRegime:false,
  okxSignalTradeMaxPerDay:"",
  okxSignalStockEnable:false,
  okxSignalStockUSDT:"100",
  okxTpSlEnable:false,
  okxTakeProfitPct:"",
  okxStopLossPct:"",
  okxTpSlMode:"fixed",
  okxTpSlVolKlt:"15m",
  okxTpSlVolBars:"20",
  okxTpSlTpMult:"1.5",
  okxTpSlSlMult:"1",
  enableAgent: false,
  qgqpBId: '',
  updateChannel: 'release',
  // 广场服务地址固定（定制版不可修改），初始为空，加载配置后由后端下发
  promptPlazaApiBase: '',
})

const updateChannelOptions = [
  { label: 'Release（稳定版）', value: 'release' },
  { label: 'Pre-release（预发布版）', value: 'pre' },
  { label: 'Dev（开发版）', value: 'dev' },
]

// OKX 信号自动交易周期白名单：多选下拉 ↔ 逗号分隔字符串（后端存储格式）
const okxKltOptions = [
  { label: '1分钟', value: '1' },
  { label: '5分钟', value: '5' },
  { label: '15分钟', value: '15' },
  { label: '30分钟', value: '30' },
  { label: '60分钟', value: '60' },
  { label: '日线', value: '101' },
  { label: '周线', value: '102' },
]
const okxSignalTradeKltsSel = computed({
  get: () => (formValue.value.okxSignalTradeKlts || '').split(',').map(s => s.trim()).filter(Boolean),
  set: arr => {
    formValue.value.okxSignalTradeKlts = Array.from(new Set(arr)).join(',')
  },
})

// 仓位比例选项（按可用余额比例下单模式）
const okxAmountRatioOptions = [
  { label: '1/4 仓', value: '0.25' },
  { label: '1/3 仓', value: '0.3333' },
  { label: '1/2 仓', value: '0.5' },
  { label: '全仓', value: '1' },
]

// 杠杆选项（手动下单与信号自动下单共用）
const okxLeverOptions = [
  { label: '账户默认', value: '' },
  { label: '1x', value: '1' },
  { label: '2x', value: '2' },
  { label: '3x', value: '3' },
  { label: '5x', value: '5' },
  { label: '10x', value: '10' },
  { label: '20x', value: '20' },
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
    formValue.value.enableContracts = res.enableContracts !== false
    formValue.value.enablePushNews = res.enablePushNews
    formValue.value.enableOnlyPushRedNews = res.enableOnlyPushRedNews
    formValue.value.sponsorCode = res.sponsorCode
    formValue.value.httpProxy=res.httpProxy;
    formValue.value.httpProxyEnabled=res.httpProxyEnabled;
    formValue.value.httpProxyMode=res.httpProxyMode || "overseas";
    formValue.value.binanceProxy=res.binanceProxy || "";
    formValue.value.bitgetProxy=res.bitgetProxy || "";
    formValue.value.okxApiKey=res.okxApiKey || "";
    formValue.value.okxSecretKey=res.okxSecretKey || "";
    formValue.value.okxPassphrase=res.okxPassphrase || "";
    formValue.value.okxLiveApiKey=res.okxLiveApiKey || "";
    formValue.value.okxLiveSecretKey=res.okxLiveSecretKey || "";
    formValue.value.okxLivePassphrase=res.okxLivePassphrase || "";
    formValue.value.okxEnableTrade=res.okxEnableTrade === true;
    formValue.value.okxSimulated=res.okxSimulated === true;
    formValue.value.okxSignalTradeEnable=res.okxSignalTradeEnable === true;
    formValue.value.okxSignalTradeUSDT=res.okxSignalTradeUSDT || "100";
    formValue.value.okxSignalTradeLive=res.okxSignalTradeLive === true;
    formValue.value.okxSignalTradeLiveUSDT=res.okxSignalTradeLiveUSDT || "10";
    formValue.value.okxSignalTradeAmountMode=res.okxSignalTradeAmountMode || "fixed";
    formValue.value.okxSignalTradeAmountRatio=res.okxSignalTradeAmountRatio || "0.25";
    formValue.value.okxRiskMaxPositions=res.okxRiskMaxPositions ?? "3";
    formValue.value.okxRiskMaxMarginRatio=res.okxRiskMaxMarginRatio ?? "30";
    formValue.value.okxSignalTradeInstType=res.okxSignalTradeInstType || "SWAP";
    formValue.value.okxSignalTradeLever=res.okxSignalTradeLever || "";
    formValue.value.okxSignalTradeKlts=res.okxSignalTradeKlts || "";
    formValue.value.okxSignalTradeMinScore=res.okxSignalTradeMinScore || "";
    formValue.value.okxSignalTradeCooldownMin=res.okxSignalTradeCooldownMin || "";
    formValue.value.okxSignalTradeDirMode=res.okxSignalTradeDirMode || "both";
    formValue.value.okxSignalTradeRegime=res.okxSignalTradeRegime === true;
    formValue.value.okxSignalTradeMaxPerDay=res.okxSignalTradeMaxPerDay || "";
    formValue.value.okxSignalStockEnable=res.okxSignalStockEnable === true;
    formValue.value.okxSignalStockUSDT=res.okxSignalStockUSDT || "100";
    formValue.value.okxTpSlEnable=res.okxTpSlEnable === true;
    formValue.value.okxTakeProfitPct=res.okxTakeProfitPct || "";
    formValue.value.okxStopLossPct=res.okxStopLossPct || "";
    formValue.value.okxTpSlMode=res.okxTpSlMode || "fixed";
    formValue.value.okxTpSlVolKlt=res.okxTpSlVolKlt || "15m";
    formValue.value.okxTpSlVolBars=res.okxTpSlVolBars || "20";
    formValue.value.okxTpSlTpMult=res.okxTpSlTpMult || "1.5";
    formValue.value.okxTpSlSlMult=res.okxTpSlSlMult || "1";
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = res.qgqpBId;
    formValue.value.updateChannel = res.updateChannel || 'release';
    // 广场服务地址固定（定制版不可修改），后端 GetSettingConfig 始终返回固定值；为空时后端按默认地址处理
    formValue.value.promptPlazaApiBase = res.promptPlazaApiBase || '';

  })

  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res
  })

  fetchPlazaUser()
})
onBeforeUnmount(() => {
  message.destroyAll()
})

/**
 * 信号自动交易参数模板。
 * 每个模板是一组「交易周期 / 信号分数门槛 / 开仓方向 / 顺大势门控 / 单日开仓上限 / 冷却 / 止盈止损」的预设，
 * 按自己的交易风格选一组，点「应用模板」填入后仍可对单个参数微调。
 */
const SIGNAL_PRESETS = {
  h1_flat: {
    label: '① 1h 双向 + 顺大势 + 冷却 120 分钟',
    klts: '60', ms: '6', dir: 'both', regime: true, mpd: '2', cd: '120', tpsl: false,
    desc: '交易最稀疏的一组：只做 60 分钟信号、只做高共振（≥6 路）、顺大势过滤、单日最多 2 笔、冷却 120 分钟，适合想尽量少动手的人。',
  },
  h1_long: {
    label: '② 1h 趋势跟随（只做多 + 顺大势）',
    klts: '60', ms: '6', dir: 'long', regime: true, mpd: '2', cd: '120', tpsl: false,
    desc: '只吃上涨段：只做 60 分钟多头信号，空头行情里会长期不出手（既是特点也是取舍）。',
  },
  h1_guard: {
    label: '③ 1h 双向 + 顺大势 + 只止损保护',
    klts: '60', ms: '6', dir: 'both', regime: true, mpd: '2', cd: '120',
    tpsl: true, tpMode: 'vol', volBars: '14', slMult: '8', tpMult: '0',
    desc: '在①的基础上开启止盈止损：用较宽的波动率止损（8 倍振幅）限制单笔亏损，不设固定止盈（账户浮盈 ≥20% 后仍有移动止盈兜底）。',
  },
  m15_sel: {
    label: '④ 15m 精选（7 路共振 + 只多 + 顺大势）',
    klts: '15', ms: '7', dir: 'long', regime: true, mpd: '2', cd: '60', tpsl: false,
    desc: '15 分钟周期的稳健写法：门槛提到 7 路共振、只做多、冷却 60 分钟，出手更少更挑。',
  },
  m5_fast: {
    label: '⑤ 5m 高频（仅适合想练盘感的人）',
    klts: '5', ms: '6', dir: 'long', regime: true, mpd: '2', cd: '60', tpsl: false,
    desc: '5 分钟周期、交易最频繁；信号密集、手续费累积快，仅建议想看盘感的人使用。',
  },
  legacy15: {
    label: '⑥ 15m 双向（上一版默认）',
    klts: '15', ms: '6', dir: 'both', regime: true, mpd: '2', cd: '60', tpsl: false,
    desc: '保留备查的一组：15 分钟双向、6 路共振，比 ④ 门槛低、出手更多。',
  },
  stock15: {
    label: '⑦ 股票类合约（SOXL / NVDA）15m 双向',
    klts: '15', ms: '6', dir: 'both', regime: true, mpd: '0', cd: '60', tpsl: false,
    desc: '面向股票类永续：必须配合「顺大势（标的自身 1H）」使用，用 BTC 当趋势代理会反向；仅对股票类标的适用。',
  },
}
const presetKey = ref('h1_flat')
const signalPresetOptions = Object.entries(SIGNAL_PRESETS).map(([k, v]) => ({ label: v.label, value: k }))
const curPreset = computed(() => SIGNAL_PRESETS[presetKey.value] || null)

/** 一键应用参数模板：只改自动交易相关字段，不碰密钥与下单金额 */
function applySignalPreset(key) {
  if (key === 'off') {
    formValue.value.okxSignalTradeKlts = ''
    message.info('已清除周期过滤（全部周期都会自动下单），记得保存')
    return
  }
  const p = SIGNAL_PRESETS[key]
  if (!p) return
  formValue.value.okxSignalTradeKlts = p.klts
  formValue.value.okxSignalTradeMinScore = p.ms
  formValue.value.okxSignalTradeDirMode = p.dir
  formValue.value.okxSignalTradeRegime = p.regime
  formValue.value.okxSignalTradeMaxPerDay = p.mpd
  formValue.value.okxSignalTradeCooldownMin = p.cd
  formValue.value.okxSignalTradeLever = '1'
  formValue.value.okxTpSlEnable = !!p.tpsl
  if (p.tpsl) {
    formValue.value.okxTpSlMode = p.tpMode || 'vol'
    // TPSL 参考周期用 OKX bar 口径（1H 而非 60m）：非法的 bar 会导致振幅取 0、vol 模式监控静默跳过
    formValue.value.okxTpSlVolKlt = p.klts === '60' ? '1H' : '15m'
    formValue.value.okxTpSlVolBars = p.volBars || '14'
    formValue.value.okxTpSlSlMult = p.slMult || '8'
    formValue.value.okxTpSlTpMult = p.tpMult || '1.5'
  }
  message.success(`已填入「${p.label}」，点击保存生效`)
}

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
    enableContracts: formValue.value.enableContracts,
    enablePushNews: formValue.value.enablePushNews,
    enableOnlyPushRedNews: formValue.value.enableOnlyPushRedNews,
    sponsorCode: formValue.value.sponsorCode,
    httpProxy:formValue.value.httpProxy,
    httpProxyEnabled:formValue.value.httpProxyEnabled,
    httpProxyMode:formValue.value.httpProxyMode || "overseas",
    binanceProxy:formValue.value.binanceProxy,
    bitgetProxy:formValue.value.bitgetProxy,
    okxApiKey:formValue.value.okxApiKey,
    okxSecretKey:formValue.value.okxSecretKey,
    okxPassphrase:formValue.value.okxPassphrase,
    okxLiveApiKey:formValue.value.okxLiveApiKey,
    okxLiveSecretKey:formValue.value.okxLiveSecretKey,
    okxLivePassphrase:formValue.value.okxLivePassphrase,
    okxEnableTrade:formValue.value.okxEnableTrade,
    okxSimulated:formValue.value.okxSimulated,
    okxSignalTradeEnable:formValue.value.okxSignalTradeEnable,
    okxSignalTradeUSDT:formValue.value.okxSignalTradeUSDT,
    okxSignalTradeLive:formValue.value.okxSignalTradeLive,
    okxSignalTradeLiveUSDT:formValue.value.okxSignalTradeLiveUSDT,
    okxSignalTradeAmountMode:formValue.value.okxSignalTradeAmountMode || "fixed",
    okxSignalTradeAmountRatio:formValue.value.okxSignalTradeAmountRatio || "0.25",
    okxRiskMaxPositions:formValue.value.okxRiskMaxPositions ?? "3",
    okxRiskMaxMarginRatio:formValue.value.okxRiskMaxMarginRatio ?? "30",
    okxSignalTradeInstType:formValue.value.okxSignalTradeInstType,
    okxSignalTradeLever:formValue.value.okxSignalTradeLever || "",
    okxSignalTradeKlts:formValue.value.okxSignalTradeKlts,
    okxSignalTradeMinScore:formValue.value.okxSignalTradeMinScore || "",
    okxSignalTradeCooldownMin:formValue.value.okxSignalTradeCooldownMin || "",
    okxSignalTradeDirMode:formValue.value.okxSignalTradeDirMode || "both",
    okxSignalTradeRegime:formValue.value.okxSignalTradeRegime === true,
    okxSignalTradeMaxPerDay:formValue.value.okxSignalTradeMaxPerDay || "",
    okxSignalStockEnable:formValue.value.okxSignalStockEnable,
    okxSignalStockUSDT:formValue.value.okxSignalStockUSDT,
    okxTpSlEnable:formValue.value.okxTpSlEnable,
    okxTakeProfitPct:formValue.value.okxTakeProfitPct,
    okxStopLossPct:formValue.value.okxStopLossPct,
    okxTpSlMode:formValue.value.okxTpSlMode,
    okxTpSlVolKlt:formValue.value.okxTpSlVolKlt,
    okxTpSlVolBars:formValue.value.okxTpSlVolBars,
    okxTpSlTpMult:formValue.value.okxTpSlTpMult,
    okxTpSlSlMult:formValue.value.okxTpSlSlMult,
    enableAgent: formValue.value.enableAgent,
    qgqpBId: formValue.value.qgqpBId,
    updateChannel: formValue.value.updateChannel,
    // 广场服务地址固定值（后端 UpdateConfig 也会强制覆盖），不写死域名
    promptPlazaApiBase: '',
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
  // 测试通知使用页面上当前填写的机器人地址，不依赖已保存的数据库配置
  const robot = (formValue.value.dingPush.dingRobot || '').trim()
  if (!robot) {
    message.warning('请先填写钉钉机器人地址')
    return
  }
  const now = new Date()
  // 必须用 JSON.stringify 生成合法 JSON：手工拼串中的换行会破坏 JSON，钉钉返回 40035
  const msg = JSON.stringify({
    msgtype: "markdown",
    markdown: {
      title: "go-stock " + now,
      text: "### go-stock test\n" + now
    },
    at: {
      isAtAll: true
    }
  })

  TestDingDingNotice(msg, robot).then(res => {
    if (res && res.includes('失败')) {
      message.error(res)
    } else {
      message.info(res)
    }
  })
}

function sendFeishuTestNotice() {
  // 测试通知使用页面上当前填写的机器人地址与签名密钥，不依赖已保存的数据库配置
  const robot = (formValue.value.feishuPush.feishuRobot || '').trim()
  if (!robot) {
    message.warning('请先填写飞书机器人地址')
    return
  }
  const secret = (formValue.value.feishuPush.feishuSecret || '').trim()
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

  TestFeishuNotice(msg, robot, secret).then(res => {
    if (res && res.includes('失败')) {
      message.error(res)
    } else {
      message.info(res)
    }
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
      formValue.value.enableContracts = config.enableContracts !== false
      formValue.value.enablePushNews = config.enablePushNews
      formValue.value.enableOnlyPushRedNews = config.enableOnlyPushRedNews
      formValue.value.sponsorCode = config.sponsorCode
      formValue.value.httpProxy=config.httpProxy
      formValue.value.httpProxyEnabled=config.httpProxyEnabled
      formValue.value.httpProxyMode=config.httpProxyMode || "overseas"
      formValue.value.binanceProxy=config.binanceProxy || ""
      formValue.value.bitgetProxy=config.bitgetProxy || ""
      formValue.value.okxApiKey=config.okxApiKey || ""
      formValue.value.okxSecretKey=config.okxSecretKey || ""
      formValue.value.okxPassphrase=config.okxPassphrase || ""
      formValue.value.okxLiveApiKey=config.okxLiveApiKey || ""
      formValue.value.okxLiveSecretKey=config.okxLiveSecretKey || ""
      formValue.value.okxLivePassphrase=config.okxLivePassphrase || ""
      formValue.value.okxEnableTrade=config.okxEnableTrade === true
      formValue.value.okxSimulated=config.okxSimulated === true
      formValue.value.okxSignalTradeEnable=config.okxSignalTradeEnable === true
      formValue.value.okxSignalTradeUSDT=config.okxSignalTradeUSDT || "100"
      formValue.value.okxSignalTradeLive=config.okxSignalTradeLive === true
      formValue.value.okxSignalTradeLiveUSDT=config.okxSignalTradeLiveUSDT || "10"
      formValue.value.okxSignalTradeAmountMode=config.okxSignalTradeAmountMode || "fixed"
      formValue.value.okxSignalTradeAmountRatio=config.okxSignalTradeAmountRatio || "0.25"
      formValue.value.okxRiskMaxPositions=config.okxRiskMaxPositions ?? "3"
      formValue.value.okxRiskMaxMarginRatio=config.okxRiskMaxMarginRatio ?? "30"
      formValue.value.okxSignalTradeInstType=config.okxSignalTradeInstType || "SWAP"
      formValue.value.okxSignalTradeLever=config.okxSignalTradeLever || ""
      formValue.value.okxSignalTradeKlts=config.okxSignalTradeKlts || ""
      formValue.value.okxSignalTradeMinScore=config.okxSignalTradeMinScore || ""
      formValue.value.okxSignalTradeCooldownMin=config.okxSignalTradeCooldownMin || ""
      formValue.value.okxSignalTradeDirMode=config.okxSignalTradeDirMode || "both"
      formValue.value.okxSignalTradeRegime=config.okxSignalTradeRegime === true
      formValue.value.okxSignalTradeMaxPerDay=config.okxSignalTradeMaxPerDay || ""
      formValue.value.okxSignalStockEnable=config.okxSignalStockEnable === true
      formValue.value.okxSignalStockUSDT=config.okxSignalStockUSDT || "100"
      formValue.value.okxTpSlEnable=config.okxTpSlEnable === true
      formValue.value.okxTakeProfitPct=config.okxTakeProfitPct || ""
      formValue.value.okxStopLossPct=config.okxStopLossPct || ""
      formValue.value.okxTpSlMode=config.okxTpSlMode || "fixed"
      formValue.value.okxTpSlVolKlt=config.okxTpSlVolKlt || "15m"
      formValue.value.okxTpSlVolBars=config.okxTpSlVolBars || "20"
      formValue.value.okxTpSlTpMult=config.okxTpSlTpMult || "1.5"
      formValue.value.okxTpSlSlMult=config.okxTpSlSlMult || "1"
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
            <n-form-item-gi v-if="!isCustomBuild" :span="3" label="合约行情：" path="enableContracts">
              <n-switch v-model:value="formValue.enableContracts"/>
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

            <n-form-item-gi :span="24" path="sponsorCode" class="sponsor-code-item">
              <div class="sponsor-code-box">
                <div class="sponsor-code-label">
                  <n-tag type="warning" size="small" :bordered="false" round>👑 VIP</n-tag>
                  <span class="sponsor-code-label-text">赞助码</span>
                  <span class="sponsor-code-label-sub">填写后激活 VIP 高级功能</span>
                </div>
                <n-input-group>
                  <n-input size="large" :show-count="true" class="sponsor-code-input"
                           placeholder="💎 联系作者QQ或微信获取赞助码，激活VIP功能"
                           v-model:value="formValue.sponsorCode">
                  </n-input>
                  <n-button size="large" type="warning" strong
                            @click="CheckSponsorCode(formValue.sponsorCode).then((res) => {message.warning(res.msg)})">立即验证
                  </n-button>
                  <n-popover trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon color="#f0a020" size="22">
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
              </div>
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

            <n-form-item-gi :span="24" class="plaza-account-item">
              <div class="plaza-account-box">
                <div class="plaza-account-label">
                  <n-tag type="info" size="small" :bordered="false" round>👤 账号</n-tag>
                  <span class="plaza-account-label-text">登录 / 注册</span>
                  <span class="plaza-account-label-sub">与「提示词广场」共用同一账号，登录后可同步 VIP、分享与订阅提示词、技能</span>
                </div>
                <!-- 已登录：账号信息与退出入口 -->
                <n-flex v-if="plazaToken" align="center" :size="10" style="flex-wrap: wrap">
                  <n-text strong style="font-size: 14px">{{ plazaUser?.nickname || plazaUser?.username || '已登录' }}</n-text>
                  <n-tag v-if="plazaUser?.vipLevel > 0" type="warning" size="small" :bordered="false" round>VIP{{ plazaUser.vipLevel }}</n-tag>
                  <n-text v-if="plazaUser?.email" depth="3" style="font-size: 12px">{{ plazaUser.email }}</n-text>
                  <n-button size="small" quaternary :loading="plazaUserLoading" @click="fetchPlazaUser">刷新</n-button>
                  <n-button size="small" type="error" ghost @click="handlePlazaLogout">退出登录</n-button>
                </n-flex>
                <!-- 未登录：登录 / 注册入口 -->
                <n-flex v-else align="center" :size="10" style="flex-wrap: wrap">
                  <n-button type="primary" @click="openPlazaAuth('login')">登录</n-button>
                  <n-button @click="openPlazaAuth('register')">注册</n-button>
                  <n-text depth="3" style="font-size: 12px">注册需邮箱验证码；忘记密码可通过绑定邮箱找回</n-text>
                </n-flex>
              </div>
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
            <n-form-item-gi :span="12" v-if="formValue.httpProxyEnabled" title="选择 http 代理的作用范围：「仅国外站点与数据」保持国内数据源直连（推荐）；「全局所有请求」所有请求均走代理"
                            label="代理作用范围" path="httpProxyMode">
              <n-radio-group v-model:value="formValue.httpProxyMode" name="httpProxyMode">
                <n-radio value="overseas">仅国外站点与数据</n-radio>
                <n-radio value="global">全局所有请求</n-radio>
              </n-radio-group>
            </n-form-item-gi>
            <n-form-item-gi v-if="!isCustomBuild" :span="12" title="币圈数据源专用 HTTP 代理，与上方爬虫代理完全独立；覆盖币安合约、OKX、Bitget 及其公告/快讯接口；留空表示直连"
                            label="合约代理(可选)" path="binanceProxy">
              <n-input type="text" placeholder="如 http://127.0.0.1:7890，留空表示直连" v-model:value="formValue.binanceProxy" clearable/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">OKX（欧易）交易</n-divider>
            </n-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-space vertical :size="6" style="padding: 0 0 14px 0;">
                <n-space align="center" :size="8">
                  <n-text depth="3" style="font-size: 12px;">参数模板：</n-text>
                  <n-select size="small" style="width: 320px" v-model:value="presetKey" :options="signalPresetOptions"/>
                  <n-button size="small" type="primary" secondary @click="applySignalPreset(presetKey)">应用模板</n-button>
                  <n-button size="small" tertiary @click="applySignalPreset('off')">关闭周期过滤</n-button>
                </n-space>
                <n-text v-if="curPreset" depth="3" style="font-size: 12px; line-height: 1.6;">
                  {{ curPreset.desc }}
                </n-text>
              </n-space>
            </n-gi>
            <n-form-item-gi :span="8" label="模拟盘 API Key" path="okxApiKey">
              <n-input type="password" show-password-on="click" placeholder="OKX 模拟交易环境创建的 API Key" v-model:value="formValue.okxApiKey" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="模拟盘 Secret Key" path="okxSecretKey">
              <n-input type="password" show-password-on="click" placeholder="模拟盘 Secret Key" v-model:value="formValue.okxSecretKey" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="模拟盘口令" path="okxPassphrase">
              <n-input type="password" show-password-on="click" placeholder="模拟盘密钥的「口令」（非备注名）" v-model:value="formValue.okxPassphrase" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="实盘 API Key" path="okxLiveApiKey">
              <n-input type="password" show-password-on="click" placeholder="OKX 真实环境创建的 API Key" v-model:value="formValue.okxLiveApiKey" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="实盘 Secret Key" path="okxLiveSecretKey">
              <n-input type="password" show-password-on="click" placeholder="实盘 Secret Key" v-model:value="formValue.okxLiveSecretKey" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="实盘口令" path="okxLivePassphrase">
              <n-input type="password" show-password-on="click" placeholder="实盘密钥的「口令」（非备注名）" v-model:value="formValue.okxLivePassphrase" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="OKX 交易" path="okxEnableTrade">
              <n-switch v-model:value="formValue.okxEnableTrade"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                开启后 AI 助手可实际下单/撤单；关闭时仅允许查询
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="模拟盘自动下单" path="okxSignalTradeEnable">
              <n-switch v-model:value="formValue.okxSignalTradeEnable"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                加密买卖点信号自动在 OKX 模拟盘下单，不动真实资金
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="模拟盘每笔金额(USDT)" path="okxSignalTradeUSDT">
              <n-input type="text" placeholder="模拟盘每笔信号投入金额，默认 100" v-model:value="formValue.okxSignalTradeUSDT" :disabled="formValue.okxSignalTradeAmountMode === 'ratio'" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="信号下单品种" path="okxSignalTradeInstType">
              <n-select v-model:value="formValue.okxSignalTradeInstType"
                        :options="[{label:'永续合约 SWAP',value:'SWAP'},{label:'币币现货 SPOT',value:'SPOT'}]"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="信号下单杠杆" path="okxSignalTradeLever">
              <n-select v-model:value="formValue.okxSignalTradeLever" :options="okxLeverOptions"/>
              <n-text type="warning" depth="3" style="margin-left: 12px; font-size: 12px;">
                仅 SWAP；下单前按环境设置该合约杠杆（全仓），设置失败该笔拒绝下单；留空=不显式设置、沿用账户当前杠杆（此时交易所侧附带止盈止损可能不下发，仅由软件侧监控兜底），建议显式设置
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="实盘自动下单" path="okxSignalTradeLive">
              <n-switch v-model:value="formValue.okxSignalTradeLive">
                <template #checked>启用</template>
                <template #unchecked>暂停</template>
              </n-switch>
              <n-tag type="error" size="small" style="margin-left: 12px;">实盘 · 真实资金！</n-tag>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="实盘每笔金额(USDT)" path="okxSignalTradeLiveUSDT">
              <n-input type="text" placeholder="实盘每笔信号投入金额，默认 10" v-model:value="formValue.okxSignalTradeLiveUSDT" :disabled="formValue.okxSignalTradeAmountMode === 'ratio'" clearable/>
              <n-text type="error" depth="3" style="margin-left: 12px; font-size: 12px;">
                实盘金额独立于模拟盘，请按风险承受能力设置
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="下单金额模式" path="okxSignalTradeAmountMode">
              <n-select v-model:value="formValue.okxSignalTradeAmountMode" :options="[{label:'固定金额',value:'fixed'},{label:'按可用余额比例（仓位）',value:'ratio'}]"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                比例模式：每笔信号按下单环境账户可用 USDT × 仓位比例开仓，模拟盘/实盘共用
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="仓位比例" path="okxSignalTradeAmountRatio">
              <n-select v-model:value="formValue.okxSignalTradeAmountRatio" :options="okxAmountRatioOptions" :disabled="formValue.okxSignalTradeAmountMode !== 'ratio'"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="防爆仓·最大持仓数" path="okxRiskMaxPositions">
              <n-input type="text" placeholder="默认 3；0=不限制" v-model:value="formValue.okxRiskMaxPositions" clearable/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                当前环境账户持仓笔数达到上限后拒绝开新仓（不影响的平仓），防止连续信号累加成超额仓位
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="防爆仓·最大保证金率(%)" path="okxRiskMaxMarginRatio">
              <n-input type="text" placeholder="默认 30；0=不限制" v-model:value="formValue.okxRiskMaxMarginRatio" clearable/>
              <n-text type="warning" depth="3" style="margin-left: 12px; font-size: 12px;">
                维持保证金÷总权益 ≥ 阈值时拒绝开新仓——这是距强平最直接的指标，建议配合止盈止损使用
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="交易周期过滤" path="okxSignalTradeKlts">
              <n-select v-model:value="okxSignalTradeKltsSel" multiple :options="okxKltOptions" placeholder="留空=不过滤（全部周期都会下单）" clearable/>
              <n-text type="warning" depth="3" style="margin-left: 12px; font-size: 12px;">
                建议只勾选「60分钟」：留空时 5m/15m 等短周期信号也会自动下单，周期越短信号越密集、噪声越大，手续费会快速吞噬本金
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="最低信号分数" path="okxSignalTradeMinScore">
              <n-input type="text" placeholder="如 6，留空=不限制" v-model:value="formValue.okxSignalTradeMinScore" clearable/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                买卖点共振路数（满分 9）低于该值不自动下单；路数越多信号越可靠，建议设为 6 以上（如 6~7）过滤低分噪音；留空=所有信号都下单
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="止盈止损冷却(分钟)" path="okxSignalTradeCooldownMin">
              <n-input type="text" placeholder="留空=默认 30 分钟" v-model:value="formValue.okxSignalTradeCooldownMin" clearable/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                止盈/止损平仓后，同合约同方向冷却期内不再自动开仓（防震荡市反复止损）；0=关闭
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="开仓方向" path="okxSignalTradeDirMode">
              <n-select v-model:value="formValue.okxSignalTradeDirMode"
                        :options="[{label:'多空都做',value:'both'},{label:'只做多（卖出信号仅平多）',value:'long'},{label:'只做空（买入信号仅平空）',value:'short'}]"/>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="顺大势门控（标的自身 1H）" path="okxSignalTradeRegime">
              <n-switch v-model:value="formValue.okxSignalTradeRegime"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                多头趋势只开多、空头趋势只开空、过渡区不限制；按标的自身 1H 趋势过滤逆势单，减少下跌段中的错误方向交易
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="单合约单日开仓上限" path="okxSignalTradeMaxPerDay">
              <n-input type="text" placeholder="如 2，留空=不限制" v-model:value="formValue.okxSignalTradeMaxPerDay" clearable/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                限制同一合约单日开仓笔数，避免同一波行情被反复进出、手续费累积；短周期信号尤其需要限制
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="股票信号下单" path="okxSignalStockEnable">
              <n-switch v-model:value="formValue.okxSignalStockEnable"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                股票永续信号（美股/港股）独立开关，与加密信号互不影响；当前暂仅模拟盘
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="股票每笔金额(USDT)" path="okxSignalStockUSDT">
              <n-input type="text" placeholder="股票信号每笔投入金额，默认 100" v-model:value="formValue.okxSignalStockUSDT" clearable/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                OKX 模拟盘未上架股票永续，实盘 xStocks 类产品可用
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="止盈止损(持仓)" path="okxTpSlEnable">
              <n-switch v-model:value="formValue.okxTpSlEnable"/>
              <n-text type="warning" depth="3" style="margin-left: 12px; font-size: 12px;">
                强烈建议开启：每 10 秒检查模拟盘/实盘两个账户的全部持仓，达到平仓线立即市价 reduceOnly 平仓；关闭后仓位无自动兜底，风险敞口显著放大
              </n-text>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="平仓线模式" path="okxTpSlMode">
              <n-select v-model:value="formValue.okxTpSlMode"
                        :options="[{label:'固定收益率(%)',value:'fixed'},{label:'波动率自适应',value:'vol'}]"/>
              <n-text depth="3" style="margin-left: 12px; font-size: 12px;">
                推荐「波动率自适应」：平仓线随各合约自身波动自动缩放，无需手调；固定模式按保证金收益率（含杠杆）与下方固定百分比比较
              </n-text>
            </n-form-item-gi>
            <template v-if="formValue.okxTpSlMode==='fixed'">
              <n-form-item-gi :span="8" label="止盈收益率(%)" path="okxTakeProfitPct">
                <n-input type="text" placeholder="如 5 表示 +5% 平仓；0/留空=不启用" v-model:value="formValue.okxTakeProfitPct" clearable/>
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="止损收益率(%)" path="okxStopLossPct">
                <n-input type="text" placeholder="如 3 表示 -3% 平仓；0/留空=不启用" v-model:value="formValue.okxStopLossPct" clearable/>
              </n-form-item-gi>
            </template>
            <template v-else>
              <n-form-item-gi :span="8" label="波动率参考周期" path="okxTpSlVolKlt">
                <n-select v-model:value="formValue.okxTpSlVolKlt"
                          :options="[{label:'1 分钟',value:'1m'},{label:'5 分钟',value:'5m'},{label:'15 分钟',value:'15m'},{label:'30 分钟',value:'30m'},{label:'1 小时',value:'1H'},{label:'4 小时',value:'4H'},{label:'1 天',value:'1D'}]"/>
                <n-text depth="3" style="font-size: 12px;">实际生效不低于 15 分钟（更短周期振幅噪声大、阈值抖动）</n-text>
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="回看K线根数" path="okxTpSlVolBars">
                <n-input type="text" placeholder="默认 20" v-model:value="formValue.okxTpSlVolBars" clearable/>
                <n-text depth="3" style="font-size: 12px;">参与振幅计算的参考周期 K 线根数（默认 20）；根数越多振幅越平滑、平仓线越稳，过少则阈值抖动</n-text>
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="止盈倍数" path="okxTpSlTpMult">
                <n-input type="text" placeholder="默认 1.5；填 0 = 关闭固定止盈" v-model:value="formValue.okxTpSlTpMult" clearable/>
                <n-text depth="3" style="font-size: 12px;">默认搭配「移动止盈」（账户浮盈 ≥20% 后跟踪、回吐一半即平，价格门槛 = 20% ÷ 实际杠杆）；固定止盈容易让利润过早离场，一般无需收紧。填 0 = 关闭固定止盈（移动止盈仍生效）</n-text>
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="止损倍数" path="okxTpSlSlMult">
                <n-input type="text" placeholder="默认 1；建议 ≥8（约 8 倍平均振幅）" v-model:value="formValue.okxTpSlSlMult" clearable/>
                <n-text depth="3" style="font-size: 12px;">倍数越小止损线越窄、越容易被噪声扫出场，建议放大到 ≥8 倍平均振幅，给波动留足空间；止损下限按账户口径 10% 执行（价格止损线 = 10% ÷ 该持仓实际杠杆），不同杠杆下账户最大亏损一致</n-text>
              </n-form-item-gi>
            </template>
            <n-form-item-gi :span="24">
              <n-gradient-text type="info">
                <div style="font-size: 12px; line-height: 1.6;">
                  配置说明：<br>
                  1. 在 OKX「交易所-API」创建 API 密钥，权限勾选「读取」与「交易」，并将本机出口 IP 加入白名单（IP 不在白名单时私有接口会报错）<br>
                  2. Passphrase 为创建密钥时自行设置的「口令」，不是密钥备注名<br>
                  3. 支持 SPOT 现货与 SWAP 永续合约，AI 助手中说「查一下我的 OKX 持仓」「用 OKX 市价买入 0.01 BTC」即可调用<br>
                  4. 模拟盘与实盘的 API 密钥是两套：模拟盘密钥在 OKX「模拟交易」环境单独创建；
                     实盘密钥在正式环境创建并绑定 IP 白名单。手动交易环境由「合约行情」页的
                     「模拟盘 / 实盘」双页签显式决定（各自固定用对应密钥，互不影响），
                     信号自动交易由「模拟盘自动下单 / 实盘自动下单」两个独立开关控制<br>
                  5. 「模拟盘自动下单」与「实盘自动下单」相互独立，可同时启用或各自暂停；
                     监控池也按环境分成两套（信号监控页签「OKX 模拟盘池 / 实盘池」，
                     交易面板「模拟盘 / 实盘」页签各自编辑对应池）：
                     模拟盘池的信号只在模拟盘下单、实盘池只在实盘下单（真实资金，请谨慎！）；
                     同一加密买卖点信号（buysell 族，ok:/bn:/bt: 代码）放入两池时两个环境各下一笔（金额各自独立）；
                     成交记录见信号监控面板「自动交易」记录，并按环境（模拟盘/实盘）标注；
                     「股票信号下单」为股票永续信号（Name 带美股/港股永续标记）的独立开关与金额，暂仅模拟盘<br>
                  6. 「止盈止损(持仓)」：固定模式按保证金收益率 uplRatio（含杠杆）与固定百分比比较；
                     波动率自适应模式按各合约近 N 根参考周期 K 线的平均振幅动态计算平仓线
                     （止盈线=振幅×止盈倍数、止损线=振幅×止损倍数，均以账户口径=价格口径收益×实际杠杆比较；
                     止损下限恒为账户 10%，价格止损线=10%÷实际杠杆，不同杠杆下账户最大亏损一致），
                     账户浮盈达 20% 后进入移动止盈跟踪、从峰值回吐一半即平仓（价格门槛=20%÷实际杠杆）；
                     手动开仓与信号自动开的仓都会被监控（模拟盘与实盘两个账户独立检查）；
                     平仓事件记录在应用日志（关键词「止盈止损·模拟盘 / 止盈止损·实盘」）
                </div>
              </n-gradient-text>
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

  <!-- 登录/注册/忘记密码：与「提示词广场」共用同一组件与账号 -->
  <PlazaAuthModal
    v-model:show="plazaAuth.show"
    v-model:tab="plazaAuth.tab"
    :api-base="formValue.promptPlazaApiBase"
    @logged-in="onPlazaLoggedIn"
  />
</template>

<style scoped>
.cardHeaderClass {
  font-size: 16px;
  font-weight: bold;
  color: red;
}

/* 赞助码醒目样式 */
.sponsor-code-item :deep(.n-form-item-blank) {
  display: block;
  width: 100%;
}

.sponsor-code-box {
  width: 100%;
  padding: 14px 16px;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(240, 160, 32, 0.10), rgba(240, 160, 32, 0.03));
  border: 1.5px dashed rgba(240, 160, 32, 0.55);
  box-shadow: 0 2px 10px rgba(240, 160, 32, 0.12);
  transition: border-color 0.3s ease, box-shadow 0.3s ease;
}

.sponsor-code-box:hover,
.sponsor-code-box:focus-within {
  border-color: #f0a020;
  border-style: solid;
  box-shadow: 0 2px 14px rgba(240, 160, 32, 0.35);
}

.sponsor-code-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.sponsor-code-label-text {
  font-size: 16px;
  font-weight: bold;
  color: #f0a020;
}

.sponsor-code-label-sub {
  font-size: 12px;
  color: rgba(240, 160, 32, 0.75);
}

.sponsor-code-input :deep(.n-input__input-el) {
  font-weight: 600;
  letter-spacing: 1px;
}

/* 账号（登录/注册）区域样式，与赞助码区块同构但用蓝色区分 */
.plaza-account-item :deep(.n-form-item-blank) {
  display: block;
  width: 100%;
}

.plaza-account-box {
  width: 100%;
  padding: 14px 16px;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(32, 128, 240, 0.10), rgba(32, 128, 240, 0.03));
  border: 1.5px dashed rgba(32, 128, 240, 0.55);
  box-shadow: 0 2px 10px rgba(32, 128, 240, 0.12);
  transition: border-color 0.3s ease, box-shadow 0.3s ease;
}

.plaza-account-box:hover,
.plaza-account-box:focus-within {
  border-color: #2080f0;
  border-style: solid;
  box-shadow: 0 2px 14px rgba(32, 128, 240, 0.35);
}

.plaza-account-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.plaza-account-label-text {
  font-size: 16px;
  font-weight: bold;
  color: #2080f0;
}

.plaza-account-label-sub {
  font-size: 12px;
  color: rgba(32, 128, 240, 0.75);
}
</style>