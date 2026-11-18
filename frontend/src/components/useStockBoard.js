/**
 * 股票所属行业/概念映射（共享单例）。
 *
 * 一次性拉取本地 all_stock_info 全表（Go 侧 GetStockBoardInfoList），把 SECUCODE/精简代码
 * 归一成纯数字代码键，供信号列表、全市场扫描面板按「所属板块（行业）/概念」做纯前端过滤。
 * 数据缺失（未同步 all_stock_info / 港美股代码）时对应行匹配不到任何板块，筛选生效时自然被排除。
 */
import { computed, ref } from 'vue'
import { GetStockBoardInfoList } from '../../wailsjs/go/main/App'

const listRef = ref([])
let loaded = false
let pending = null

/** 提取纯数字代码用于匹配：000651 / 600519.SH / sz000651 等格式统一取数字段 */
function codeDigits(code) {
  const m = String(code || '').match(/\d{4,6}/)
  return m ? m[0] : ''
}

/** 懒加载板块数据（只拉一次；失败静默，下次再试） */
async function ensureBoardData() {
  if (loaded) return listRef.value
  if (!pending) {
    pending = GetStockBoardInfoList()
      .then((list) => {
        listRef.value = list || []
        loaded = true
      })
      .catch(() => { pending = null })
  }
  await pending
  return listRef.value
}

/** 数字代码 → { industry, concepts[] } 映射（随全表数据更新） */
const boardByCodeRef = computed(() => {
  const m = new Map()
  for (const b of listRef.value) {
    const d = codeDigits(b.code || b.secuCode)
    if (d && !m.has(d)) m.set(d, b)
  }
  return m
})

/** 行业/概念全部取值（下拉选项用，字典序） */
const industryOptionsRef = computed(() => {
  const seen = new Set()
  for (const b of listRef.value) if (b.industry) seen.add(b.industry)
  return [...seen].sort().map((s) => ({ label: s, value: s }))
})
const conceptOptionsRef = computed(() => {
  const seen = new Set()
  for (const b of listRef.value) for (const c of b.concepts || []) if (c) seen.add(c)
  return [...seen].sort().map((s) => ({ label: s, value: s }))
})

export { ensureBoardData, codeDigits, boardByCodeRef, industryOptionsRef, conceptOptionsRef }
