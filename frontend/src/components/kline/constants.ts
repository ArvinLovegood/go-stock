export const CLR_RISE = '#ef5350'
export const CLR_FALL = '#26a69a'

export const DAILY_LIKE_KLT = new Set(['101', '102', '103', '104', '106'])
export const CN_TZ = 'Asia/Shanghai'

export const HISTORY_PAGE_SIZE = 400
export const BARS_BEFORE_LOAD_MORE = 45
export const DEFAULT_VISIBLE_BARS = 180
export const DEFAULT_RIGHT_LOGICAL_GAP = 18
export const SHOW_CHIP_TOOLBAR_BUTTON = false

// 复权类型：qfq=前复权（默认）、hfq=后复权、none=不复权
// 仅日K及更长周期（DAILY_LIKE_KLT）有效；分时周期传空串走各数据源默认行为
export const DEFAULT_ADJUST = 'qfq'
export const ADJUST_OPTIONS = [
  { value: 'qfq', label: '前复权' },
  { value: 'hfq', label: '后复权' },
  { value: 'none', label: '不复权' },
]

/**
 * 买卖点共振档位：命中的信号路数（等权计数，每路 1 分）>= minScore，且需覆盖 minGroups 个信号组（震荡/动量/量价）。
 * 9 路信号集（震荡组 CCI/RSI/KDJ、动量组 MACD/TEMA/TRIX/ADX、量价组 均价线/放量）下，minGroups 统一取 2：
 * 实测要求覆盖 3 组会强制「放量」共振，而放量的卖向边际为负（-0.42%），标准档卖点超额变差；故三档同用 2 组、仅以路数阈值区分。
 * 等权 3/4/5 路对应原加权制 2/3/4 分档（50 只/32.2 万根箭头级 A/B：卖侧三档持平或改善、买侧 -0.06~0.12pp）。
 * 图表与后台信号监控共用，避免两处档位定义漂移。
 */
export const BUY_SELL_SCORE_OPTIONS = [
  { value: 3, label: '灵敏', minScore: 3, minGroups: 2 },
  { value: 4, label: '标准', minScore: 4, minGroups: 2 },
  { value: 5, label: '严格', minScore: 5, minGroups: 2 },
]

export const INTERVALS = [
  { klt: '1', label: '1分', limit: 1000 },
  { klt: '5', label: '5分', limit: 600 },
  { klt: '15', label: '15分', limit: 500 },
  { klt: '30', label: '30分', limit: 500 },
  { klt: '60', label: '60分', limit: 500 },
  { klt: '101', label: '日K', limit: 800 },
  { klt: '102', label: '周K', limit: 520 },
  { klt: '103', label: '月K', limit: 240 },
  { klt: '104', label: '季K', limit: 120 },
  { klt: '106', label: '年K', limit: 40 },
]
