/**
 * 买卖点 / TEMA 转折提示音（Web Audio 实时合成，无需音频资源文件）。
 *
 * 独立成模块的原因：提示音现在由「后台信号监控引擎」统一裁决（见 kline/signalMonitor.ts），
 * 图表组件只是订阅者之一；若仍内嵌在组件里，组件卸载后后台监控就哑了。
 * 模块级单例的 AudioContext 同时被图表与引擎复用，避免多实例互相抢占输出。
 */

/**
 * 提示音音量（Web Audio 增益，0~1）。两者都取较高音量以免错过信号；
 * 买卖点音是短促双音、听感更轻，故给到 0.7；T确 是 1 秒长音，取 0.9 更醒目。
 */
export const ALERT_VOL_BUY_SELL = 0.7
export const ALERT_VOL_TEMA_CONFIRM = 0.9

/** 语音播报音量（SpeechSynthesisUtterance.volume，0~1）：念股票名称必须听清，取满 */
export const ALERT_VOL_SPEECH = 1

/** 提示音用的 AudioContext（懒创建；在用户点击开关的手势中预热，避免被浏览器自动播放策略拦截） */
let alertAudioCtx = null

/** 已响过的提示音去重集合（窗口内 FIFO 裁剪，避免无限增长） */
const playedAlertKeys = new Set()
const PLAYED_ALERT_LIMIT = 2000

/**
 * 提示音全局去重：同一只票、同一根 K 线、同向信号只允许响一次。
 *
 * 图表组件与后台信号监控引擎各自独立检测（图表跟随当前显示周期与数据窗口，引擎只扫监控池），
 * 当监控池里正好有当前正在看的票时会同时命中，若无共享裁决就会响两遍。
 * 约定：**先判定音量开关、再抢占**，否则「图表关了提示音」会把引擎的声音也一并吞掉。
 */
export function claimAlertToneOnce(key) {
  if (!key) return true
  if (playedAlertKeys.has(key)) return false
  playedAlertKeys.add(key)
  if (playedAlertKeys.size > PLAYED_ALERT_LIMIT) {
    const kept = Array.from(playedAlertKeys).slice(-PLAYED_ALERT_LIMIT / 2)
    playedAlertKeys.clear()
    for (const k of kept) playedAlertKeys.add(k)
  }
  return true
}

/**
 * 确保音频上下文可用：不存在就地创建，处于 suspended 就尝试恢复。
 *
 * 关键点：`resume()` 只有在**用户手势内**才会成功，手势外调用会被自动播放策略拒绝
 * （返回 rejected promise）。所以手势路径与播放路径都调用本函数——
 * 手势路径负责真正解除挂起，播放路径负责「上下文被系统休眠/切换设备弄挂起后自愈」。
 */
function ensureAlertAudioCtx() {
  try {
    const AC = window.AudioContext || window.webkitAudioContext
    if (!AC) return null
    if (!alertAudioCtx) alertAudioCtx = new AC()
    if (alertAudioCtx.state === 'suspended') {
      alertAudioCtx.resume().catch(() => { /* 手势外被拒，等下一次用户手势 */ })
    }
    return alertAudioCtx
  } catch {
    return null
  }
}

/** 当前音频状态：'running' | 'suspended' | 'closed' | 'none'（不存在时）——用于提示音开关的就绪提示 */
export function alertAudioState() {
  return alertAudioCtx ? alertAudioCtx.state : 'none'
}

/** 预热音频上下文：必须在用户手势回调内调用 */
export function primeAlertAudio() {
  ensureAlertAudioCtx()
}

/**
 * 按「音段」合成一段提示音（Web Audio 实时合成，无需音频资源）。
 * segments: [{ at, freq, dur }]，at 为相对起点的秒数；音量与淡入淡出由 vol/attack/release 控制。
 */
export function playAlertSegments(segments, { vol = ALERT_VOL_BUY_SELL, attack = 0.015, release = 0.04 } = {}) {
  const ctx = alertAudioCtx || ensureAlertAudioCtx()
  if (!ctx) return
  try {
    const t0 = ctx.currentTime + 0.01
    for (const seg of segments) {
      const t = t0 + seg.at
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      osc.type = 'sine'
      osc.frequency.setValueAtTime(seg.freq, t)
      // 淡入淡出包络，避免起停爆音
      gain.gain.setValueAtTime(0, t)
      gain.gain.linearRampToValueAtTime(vol, t + attack)
      gain.gain.setValueAtTime(vol, t + seg.dur - release)
      gain.gain.linearRampToValueAtTime(0, t + seg.dur)
      osc.connect(gain).connect(ctx.destination)
      osc.start(t)
      osc.stop(t + seg.dur)
    }
  } catch { /* 静默失败，不影响图表 */ }
}

/**
 * 播放买卖点提示音：买=上行双音（880→1319Hz，低转亮）、卖=下行双音（880→587Hz）、
 * 买卖同时出现=三音依次播报，便于不看屏幕也能分辨方向。
 */
export function playBuySellAlertTone(kind) {
  const seq = kind === 'buy' ? [880, 1318.5]
    : kind === 'sell' ? [880, 587.3]
      : [880, 1318.5, 587.3]
  const DUR = 0.13
  const GAP = 0.04
  playAlertSegments(seq.map((freq, k) => ({ at: k * (DUR + GAP), freq, dur: DUR })), { vol: ALERT_VOL_BUY_SELL })
}

/**
 * 播放「T确」（TEMA 温和确认）提示音：总时长 1 秒、音量高于买卖点提示音，
 * 买向升调、卖向降调，两者一听可辨。
 */
export function playTemaConfirmAlertTone(dir) {
  const rising = dir === 'buy'
  playAlertSegments([
    { at: 0, freq: rising ? 784 : 880, dur: 0.35 },
    { at: 0.35, freq: rising ? 1174.66 : 587.33, dur: 0.65 },
  ], { vol: ALERT_VOL_TEMA_CONFIRM, attack: 0.02, release: 0.18 })
}

// ===== 语音播报（Web Speech API）=====
//
// 提示音只能表达方向（升调买、降调卖），多只票同时命中时无法分辨是哪一只，
// 故在短音之后补一句「贵州茅台 买点」。合成走系统语音，无需音频资源、无需联网。

/** 选定的中文语音（null = 系统没有可用中文语音，此时静默降级为只播提示音） */
let alertVoice = null
/** 是否已挂过 voiceschanged 监听 */
let voiceListBound = false
/** 是否已在手势中做过一次空播预热 */
let speechPrimed = false

/** 挑一个中文语音：优先 zh-CN，其次 zh-Hans，再次任意 zh*；都没有返回 null */
function pickAlertVoice() {
  const synth = typeof window !== 'undefined' ? window.speechSynthesis : null
  if (!synth) return null
  const voices = synth.getVoices() || []
  if (!voices.length) return null
  const zh = voices.filter((v) => /^zh/i.test(v.lang || ''))
  return zh.find((v) => /^zh[-_]CN/i.test(v.lang)) || zh.find((v) => /^zh[-_]Hans/i.test(v.lang)) || zh[0] || null
}

/**
 * 取中文语音。首次调用时 getVoices() 往往还是空数组（Chromium 异步加载语音列表），
 * 故顺带挂一次 voiceschanged 监听，等列表就绪后再补选。
 */
function ensureAlertVoice() {
  const synth = typeof window !== 'undefined' ? window.speechSynthesis : null
  if (!synth) return null
  if (!alertVoice) alertVoice = pickAlertVoice()
  if (!voiceListBound) {
    voiceListBound = true
    try {
      synth.addEventListener('voiceschanged', () => { alertVoice = pickAlertVoice() })
    } catch { /* 老实现只支持 onvoiceschanged，忽略 */ }
  }
  return alertVoice
}

/** 系统是否有可用中文语音 —— 供面板在开启语音播报前提示，避免「开了却没声」 */
export function alertSpeechAvailable() {
  return !!ensureAlertVoice()
}

/**
 * 语音预热：必须在用户手势回调内调用。
 * 与 AudioContext 同理，自动播放策略下首次 speak() 需要用户手势，先空播一个空格唤醒合成器。
 */
export function primeAlertSpeech() {
  const synth = typeof window !== 'undefined' ? window.speechSynthesis : null
  if (!synth || speechPrimed) return
  speechPrimed = true
  ensureAlertVoice()
  try {
    synth.speak(new SpeechSynthesisUtterance(' '))
  } catch { /* 忽略 */ }
}

/**
 * 播报一句提示（如「贵州茅台 买点」）。
 * 来新的一句会打断上一句：一次 tick 命中多只票时不排队堆积，宁可只把最近的说完整。
 * 返回是否真的播了出去（无中文语音时返回 false，调用方无需额外判断）。
 */
export function speakAlertText(text) {
  const synth = typeof window !== 'undefined' ? window.speechSynthesis : null
  const say = (text || '').trim()
  if (!synth || !say) return false
  const voice = ensureAlertVoice()
  if (!voice) return false
  try {
    const u = new SpeechSynthesisUtterance(say)
    u.voice = voice
    u.lang = voice.lang
    // 盘面播报不拖沓，略快于常速
    u.rate = 1.05
    u.volume = ALERT_VOL_SPEECH
    if (synth.speaking || synth.pending) {
      synth.cancel()
      // Chromium 在 cancel() 的同一轮事件循环内 speak() 有概率丢帧，延后一轮再念
      setTimeout(() => {
        try { synth.speak(u) } catch { /* 忽略 */ }
      }, 0)
    } else {
      synth.speak(u)
    }
    return true
  } catch {
    return false
  }
}