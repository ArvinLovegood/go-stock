// Agent 流式文本解析工具（纯函数，无状态）
// 从 FloatingAgentAssistant.vue 拆出：把 [STEP] 文本标记与模型输出文本
// 解析为结构化的任务清单/步骤数据。

// 解析 [STEP]📝 文本为 todos 数组；非清单文本或解析不出条目时返回 null（走原步骤列表兜底）
export function parseTodosSnapshot(text) {
  if (!text) return null
  const trimmed = text.replace(/^📝\s*/, '').trim()
  if (!/^任务清单更新|^任务规划|^计划[:：]/.test(trimmed)) return null
  const lines = trimmed.split('\n').map(l => l.trim()).filter(Boolean)
  const items = []
  for (const line of lines.slice(1)) {
    let m
    if ((m = line.match(/^✅\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'completed' })
    } else if ((m = line.match(/^🔄\s*(.+?)(?:（进行中）)?$/))) {
      items.push({ text: m[1].trim(), status: 'in_progress' })
    } else if ((m = line.match(/^⏳\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'pending' })
    } else if ((m = line.match(/^\d+[.、)）]\s*(.+)$/))) {
      items.push({ text: m[1].trim(), status: 'pending' })
    }
  }
  return items.length > 0 ? items : null
}

// 从 content 累积文本中提取 PlanExecute 提示词约定的任务清单（至少 2 条才认定为清单）
// 约定格式：- [1] 步骤描述 [pending|in_progress|completed]
// 提示词要求模型每完成一步重新输出整份清单，累积文本会包含多轮快照；
// 只取最后一个连续清单块（最新快照），否则新旧状态混杂、条目重复，清单看起来"不更新"
export function extractPlanChecklist(raw) {
  if (!raw) return null
  const re = /^-\s*\[\d+\]\s*(.+?)\s*\[(pending|in_progress|completed)\]\s*$/
  let latest = null
  let cur = []
  for (const line of raw.split('\n')) {
    const m = line.match(re)
    if (m) {
      cur.push({ text: m[1], status: m[2] })
    } else if (cur.length > 0) {
      if (cur.length >= 2) latest = cur
      cur = []
    }
  }
  if (cur.length >= 2) latest = cur
  return latest
}

// 尝试把 JSON 形式的步骤文本展开为编号列表；非 JSON 或无结构时原样返回
export function parseStepText(text) {
  if (!text) return [text]
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return [text]
  try {
    const obj = JSON.parse(trimmed)
    if (Array.isArray(obj)) {
      return obj.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
    }
    if (typeof obj === 'object' && obj !== null) {
      const steps = obj.steps || obj.step || obj.plan || obj.items || obj.list
      if (Array.isArray(steps)) {
        return steps.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
      }
      const entries = Object.entries(obj)
      if (entries.length > 0) {
        return entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
      }
    }
    return [text]
  } catch {
    return [text]
  }
}
