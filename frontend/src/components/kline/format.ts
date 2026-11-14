export function parseNumStr(s) {
  const t = String(s ?? '').replace(/,/g, '').replace(/%/g, '').trim()
  // 空串视为缺失（NaN → 显示 "--"），而非 Number('') === 0
  if (t === '') return NaN
  const n = Number(t)
  return Number.isFinite(n) ? n : NaN
}

/** 小价格合约（如 0.03 USDT）动态精度：|价格| < 1 时保留 6 位小数，否则 2 位 */
export function priceDecimalsFor(v) {
  const n = Math.abs(Number(v))
  if (!Number.isFinite(n) || n === 0) return 2
  return n < 1 ? 6 : 2
}

export function formatPrice2(s) {
  const n = parseNumStr(s)
  return Number.isFinite(n) ? n.toFixed(priceDecimalsFor(n)) : '--'
}

export function formatVolumeCn(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  if (n >= 1e8) return `${(n / 1e8).toFixed(2)}亿`
  if (n >= 1e4) return `${(n / 1e4).toFixed(2)}万`
  return String(Math.round(n))
}

export function formatAmountCn(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  return `${(n / 1e8).toFixed(2)}亿`
}

export function formatPctField(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  return `${n.toFixed(2)}%`
}

export function formatSigned2(s) {
  const n = parseNumStr(s)
  if (!Number.isFinite(n)) return '--'
  // 小价格合约的价差（涨跌额/测量价差）动态升到 6 位小数，避免 0.0003 被舍入成 0.00
  const t = n.toFixed(priceDecimalsFor(n))
  return n > 0 ? `+${t}` : t
}
