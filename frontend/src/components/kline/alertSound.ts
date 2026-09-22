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

/** 预热音频上下文：必须在用户手势回调内调用 */
export function primeAlertAudio() {
  try {
    const AC = window.AudioContext || window.webkitAudioContext
    if (!AC) return
    if (!alertAudioCtx) alertAudioCtx = new AC()
    if (alertAudioCtx.state === 'suspended') alertAudioCtx.resume()
  } catch { /* 静默失败，不影响图表 */ }
}

/**
 * 按「音段」合成一段提示音（Web Audio 实时合成，无需音频资源）。
 * segments: [{ at, freq, dur }]，at 为相对起点的秒数；音量与淡入淡出由 vol/attack/release 控制。
 */
export function playAlertSegments(segments, { vol = ALERT_VOL_BUY_SELL, attack = 0.015, release = 0.04 } = {}) {
  if (!alertAudioCtx) return
  try {
    const t0 = alertAudioCtx.currentTime + 0.01
    for (const seg of segments) {
      const t = t0 + seg.at
      const osc = alertAudioCtx.createOscillator()
      const gain = alertAudioCtx.createGain()
      osc.type = 'sine'
      osc.frequency.setValueAtTime(seg.freq, t)
      // 淡入淡出包络，避免起停爆音
      gain.gain.setValueAtTime(0, t)
      gain.gain.linearRampToValueAtTime(vol, t + attack)
      gain.gain.setValueAtTime(vol, t + seg.dur - release)
      gain.gain.linearRampToValueAtTime(0, t + seg.dur)
      osc.connect(gain).connect(alertAudioCtx.destination)
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