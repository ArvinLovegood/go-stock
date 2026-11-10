import { TickMarkType } from 'lightweight-charts'
import { CN_TZ } from './constants'

export function eastMoneyDayToUnixSeconds(dayStr) {
  const t = String(dayStr || '').trim().replace(/\//g, '-')
  if (!t || /^\d{4}-\d{2}-\d{2}$/.test(t)) return null
  let iso = t
  if (!t.includes('T')) {
    iso = t.replace(/^(\d{4}-\d{2}-\d{2})\s+/, '$1T')
  }
  if (!/[zZ]|[+-]\d{2}:?\d{2}$/.test(iso)) {
    iso += '+08:00'
  }
  const ms = Date.parse(iso)
  if (!Number.isFinite(ms)) return null
  return Math.floor(ms / 1000)
}

export function eastMoneyKlineFieldToUnixSeconds(s) {
  let sec = eastMoneyDayToUnixSeconds(s)
  if (sec != null) return sec
  const t = String(s || '').trim().replace(/\//g, '-')
  const dm = t.match(/^(\d{4}-\d{2}-\d{2})$/)
  if (dm) {
    const ms = Date.parse(`${dm[1]}T12:00:00+08:00`)
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  const c14 = t.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/)
  if (c14) {
    const ms = Date.parse(
      `${c14[1]}-${c14[2]}-${c14[3]}T${c14[4]}:${c14[5]}:${c14[6]}+08:00`,
    )
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  const c12 = t.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})$/)
  if (c12) {
    const ms = Date.parse(
      `${c12[1]}-${c12[2]}-${c12[3]}T${c12[4]}:${c12[5]}:00+08:00`,
    )
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null
  }
  return null
}

export function chartTimeToUtcMs(time) {
  if (typeof time === 'number') return time * 1000
  if (typeof time === 'string') {
    if (/^\d{4}-\d{2}-\d{2}$/.test(time)) {
      return Date.parse(`${time}T12:00:00+08:00`)
    }
    return Date.parse(time)
  }
  if (time && typeof time === 'object' && 'year' in time && 'month' in time && 'day' in time) {
    const { year, month, day } = time
    const mm = String(month).padStart(2, '0')
    const dd = String(day).padStart(2, '0')
    return Date.parse(`${year}-${mm}-${dd}T12:00:00+08:00`)
  }
  return NaN
}

export function formatTickTime(time, tickMarkType) {
  const ms = chartTimeToUtcMs(time)
  if (!Number.isFinite(ms)) return null
  const d = new Date(ms)
  const loc = 'zh-CN'
  if (tickMarkType === TickMarkType.Year) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, year: 'numeric' }).format(d)
  }
  if (tickMarkType === TickMarkType.Month) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, year: 'numeric', month: '2-digit' }).format(d)
  }
  if (tickMarkType === TickMarkType.DayOfMonth) {
    return new Intl.DateTimeFormat(loc, { timeZone: CN_TZ, month: '2-digit', day: '2-digit' }).format(d)
  }
  return new Intl.DateTimeFormat(loc, {
    timeZone: CN_TZ,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(d)
}

export function sortKey(dayStr) {
  const sec = eastMoneyKlineFieldToUnixSeconds(dayStr)
  if (sec != null) return sec * 1000
  const s = String(dayStr || '').trim()
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (m) {
    return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  }
  return 0
}

export function toChartTime(dayStr) {
  const s = String(dayStr || '').trim()
  if (!s) return null
  // 始终尝试转为 Unix 秒（数字），避免日线返回字符串与分钟线数字混用导致 TradingView 报错
  const sec = eastMoneyKlineFieldToUnixSeconds(s)
  if (sec != null) return sec
  // 兜底：纯日期格式无法解析时，手动计算
  const dm = s.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (dm) {
    const ms = Date.parse(`${dm[1]}-${dm[2]}-${dm[3]}T12:00:00+08:00`)
    if (Number.isFinite(ms)) return Math.floor(ms / 1000)
  }
  return null
}

export function mergeKlineRows(existing, incoming) {
  const map = new Map()
  for (const r of existing) {
    if (r?.day) map.set(r.day, r)
  }
  for (const r of incoming) {
    if (r?.day && !map.has(r.day)) map.set(r.day, r)
  }
  return Array.from(map.values()).sort((a, b) => sortKey(a.day) - sortKey(b.day))
}

export function mergeRefreshWithLatest(existingSorted, latestChunk) {
  const list = Array.isArray(latestChunk) ? latestChunk : []
  if (!list.length) return existingSorted.length ? existingSorted : []
  const sortedLatest = [...list].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const cutoff = sortKey(sortedLatest[0].day)
  const kept = existingSorted.filter((r) => sortKey(r.day) < cutoff)
  const map = new Map()
  for (const r of kept) {
    if (r?.day) map.set(r.day, r)
  }
  for (const r of sortedLatest) {
    if (r?.day) map.set(r.day, r)
  }
  return Array.from(map.values()).sort((a, b) => sortKey(a.day) - sortKey(b.day))
}

export function extractYmdDatePart(s) {
  const t = String(s || '').trim()
  const mDash = t.match(/^(\d{4}-\d{2}-\d{2})/)
  if (mDash) return mDash[1]
  const m8 = t.match(/^(\d{4})(\d{2})(\d{2})/)
  if (m8) return `${m8[1]}-${m8[2]}-${m8[3]}`
  return ''
}

export function barSecondsForMinuteKlt(klt) {
  const n = Number.parseInt(String(klt), 10)
  if (Number.isFinite(n) && n > 0) return n * 60
  return 60
}

/**
 * UTC 毫秒 → 上海时区 8 位日期串 YYYYMMDD（东财/通达信分页 end 参数，日线等日级周期用）。
 * 中国无夏令时、文件内 Date.parse 一律硬编码 +08:00，这里同样用固定偏移避免 Intl 区域格式差异。
 */
export function formatYmdCompactShanghai(ms) {
  if (!Number.isFinite(ms)) return ''
  const d = new Date(ms + 8 * 3600 * 1000)
  const y = d.getUTCFullYear()
  const m = String(d.getUTCMonth() + 1).padStart(2, '0')
  const day = String(d.getUTCDate()).padStart(2, '0')
  return `${y}${m}${day}`
}

/** UTC 毫秒 → 上海时区 14 位串 YYYYMMDDHHmmss（分钟线分页 end 参数用） */
export function formatYmdHmsCompactShanghai(ms) {
  if (!Number.isFinite(ms)) return ''
  const d = new Date(ms + 8 * 3600 * 1000)
  const y = d.getUTCFullYear()
  const m = String(d.getUTCMonth() + 1).padStart(2, '0')
  const day = String(d.getUTCDate()).padStart(2, '0')
  const h = String(d.getUTCHours()).padStart(2, '0')
  const mi = String(d.getUTCMinutes()).padStart(2, '0')
  const s = String(d.getUTCSeconds()).padStart(2, '0')
  return `${y}${m}${day}${h}${mi}${s}`
}
