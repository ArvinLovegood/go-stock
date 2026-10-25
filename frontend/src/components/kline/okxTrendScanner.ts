/**
 * OKX 全市场上升趋势扫描器：公共品种表（SWAP/USDT 结算/live）→ 并发取 K 线 → okxUptrendScan 评分排序。
 * 手动触发的低频工具（OKX 交易面板「趋势扫描」按钮），不与三套信号监控共用节拍；
 * 数据与面板图表同源（后端 GetOkxInstruments / GetOKXCandleList），指标与 calc.ts 全站同口径。
 */
import { GetOKXCandleList, GetOkxInstruments } from '../../../wailsjs/go/main/App'
import { okxUptrendScan } from './calc'

/** 每品种拉取的 K 线根数：MA60 + ADX 平滑期最少 70，200 留足缓冲且请求体积可控 */
const SCAN_BAR_LIMIT = 200
/** 取数并发：与信号监控 FETCH_CONCURRENCY 同量级，避免打满后端 OKX 限频 */
const SCAN_CONCURRENCY = 6

/** 门槛档位：3=宽松 / 4=明显 / 5=严格（5 路趋势证据等权计分） */
export const TREND_SCAN_MIN_SCORES = [
  { value: 3, label: '3/5 宽松' },
  { value: 4, label: '4/5 明显' },
  { value: 5, label: '5/5 严格' },
]

/**
 * OKX 永续合约全量列表（公共品种表）：live 状态 + SWAP + USDT 结算。
 * 过滤口径与信号监控面板 loadContracts 的 OKX 分支一致（settleCcy 只认 USDT）。
 */
export async function okxSwapList(): Promise<string[]> {
  const pub = await GetOkxInstruments()
  const list: string[] = []
  for (const ins of (pub as any[]) || []) {
    const instId = String(ins?.instId || ins?.InstId || '').toUpperCase()
    const state = String(ins?.state || '').toLowerCase()
    const isSwap = String(ins?.instType || '').toUpperCase() === 'SWAP' || /-SWAP$/.test(instId)
    const ccy = String(ins?.settleCcy || '').toUpperCase()
    if (!instId || (state && state !== 'live') || !isSwap || (ccy && ccy !== 'USDT')) continue
    list.push(instId)
  }
  return list
}

/**
 * 单品种趋势判分（趋势跟随引擎用）：供池内但不在全市场扫描结果里的品种（如非 USDT 结算的永续）补判，
 * 拉不到 K 线或根数不足时返回 null —— 调用方应保守跳过、不据此移除。
 */
export async function scanOneTrend(instId: string, bar: string): Promise<any | null> {
  try {
    const list = await GetOKXCandleList(instId, bar, SCAN_BAR_LIMIT)
    const bars: any[] = []
    for (const c of (list as any[]) || []) {
      if (!c || !(c.ts > 0)) continue
      bars.push({ ts: c.ts, high: Number(c.high), low: Number(c.low), close: Number(c.close) })
    }
    bars.sort((a, b) => a.ts - b.ts)
    const scan = okxUptrendScan(bars.map((b) => b.high), bars.map((b) => b.low), bars.map((b) => b.close))
    return scan ? { instId, ...scan } : null
  } catch {
    return null
  }
}

/**
 * 跑一轮全市场趋势扫描。
 *
 * @param {{bar?:string,minScore?:number,onProgress?:(done:number,total:number)=>void,abort?:{aborted:boolean}}} [opts]
 *   bar OKX K 线周期（'1D'/'4H'/'1H'/'15m'...）；minScore 仅用于行内附带，过滤交给调用方（便于同轮结果松紧切换）；
 *   onProgress 进度回调（含失败品种）；abort.aborted 置 true 后不再发起新请求（已在途的自然完成）。
 * @returns {Promise<{rows:Array,total:number}>} rows 按 score 降序、同分按 ADX 降序；total 为本轮实际扫描品种数。
 */
export async function runOkxTrendScan({ bar = '1D', onProgress, abort } = {}): Promise<{ rows: any[]; total: number }> {
  const instIds = await okxSwapList()
  const total = instIds.length
  const rows: any[] = []
  let done = 0
  let cursor = 0
  const step = () => {
    done++
    if (onProgress) onProgress(done, total)
  }
  const workers = new Array(Math.min(SCAN_CONCURRENCY, Math.max(total, 1))).fill(0).map(async () => {
    while (true) {
      if (abort && abort.aborted) return
      const idx = cursor++
      if (idx >= total) return
      const instId = instIds[idx]
      try {
        const list = await GetOKXCandleList(instId, bar, SCAN_BAR_LIMIT)
        const bars: any[] = []
        for (const c of (list as any[]) || []) {
          if (!c || !(c.ts > 0)) continue
          bars.push({ ts: c.ts, high: Number(c.high), low: Number(c.low), close: Number(c.close) })
        }
        // 后端返回顺序不保证升序（面板图表同款 sort），按时间整理后再抽列
        bars.sort((a, b) => a.ts - b.ts)
        const scan = okxUptrendScan(bars.map((b) => b.high), bars.map((b) => b.low), bars.map((b) => b.close))
        if (scan) rows.push({ instId, ...scan })
      } catch (e) {
        // 单品种失败不影响其余（流动性差/新上品种拉不到 K 线属正常）
      }
      step()
    }
  })
  await Promise.all(workers)
  rows.sort((a, b) => b.score - a.score || (b.adx || 0) - (a.adx || 0))
  return { rows, total }
}
