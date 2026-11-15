import { GetStockList } from '../../../wailsjs/go/main/App'

/**
 * 股票名称补全：只传代码没传名称时（如 AI 消息里点击代码打开 K 线），
 * 从本地股票库（GetStockList）反查名称，带内存缓存，查不到返回空串。
 */
const nameCache = new Map()

/** 去掉市场前后缀，得到用于匹配的基础代码（000333.SZ / sh000333 / gb_aapl / AAPL.US → 000333 / aapl） */
function normalizeCode(code) {
  return String(code || '')
    .toLowerCase()
    .replace(/^(gb_|sh|sz|bj|hk|us)/, '')
    .replace(/\.(sh|sz|bj|hk|us)$/i, '')
    .trim()
}

/** 按代码补全股票名称；支持东财格式（000333.SZ）、内部格式（sh000333、gb_aapl）等 */
export async function resolveStockName(code) {
  const base = normalizeCode(code)
  if (!base) return ''
  if (nameCache.has(base)) return nameCache.get(base)
  try {
    const list = (await GetStockList(base)) || []
    let name = ''
    for (const item of list) {
      if (normalizeCode(item.ts_code) === base && item.name) {
        name = item.name
        break
      }
    }
    nameCache.set(base, name)
    return name
  } catch (_) {
    // 静默失败：查不到名称不阻断 K 线图打开
    return ''
  }
}
