export function parseNumStr(s) {
  const t = String(s ?? '').replace(/,/g, '').replace(/%/g, '').trim()
  // 空串视为缺失（NaN → 显示 "--"），而非 Number('') === 0
  if (t === '') return NaN
  const n = Number(t)
  return Number.isFinite(n) ? n : NaN
}

export function formatPrice2(s) {
  const n = parseNumStr(s)
  return Number.isFinite(n) ? n.toFixed(2) : '--'
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
  const t = n.toFixed(2)
  return n > 0 ? `+${t}` : t
}
