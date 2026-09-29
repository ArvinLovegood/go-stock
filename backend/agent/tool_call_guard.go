package agent

// tool_call_guard.go — 工具调用中间件的守卫判定函数：
// 瞬态错误识别（自动重试）、只读工具识别（同轮去重缓存）、去重键构造。

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// toolRetryBackoff 瞬态错误自动重试前的退避等待。
const toolRetryBackoff = 800 // ms

// transientErrorMarks 瞬态错误的错误文本标记（小写匹配）。
// 仅覆盖网络/服务端临时故障；参数错误、数据为空等业务错误不在此列。
var transientErrorMarks = []string{
	"timeout",
	"timed out",
	"deadline exceeded",
	"connection reset",
	"connection refused",
	"connection aborted",
	"eof",
	"tls handshake",
	"temporarily unavailable",
	"service unavailable",
	"too many requests",
	"bad gateway",
	"gateway timeout",
	"http 429",
	"http 502",
	"http 503",
	"http 504",
}

// isTransientToolError 判断工具调用错误是否为可自动重试的瞬态错误。
func isTransientToolError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, mark := range transientErrorMarks {
		if strings.Contains(msg, mark) {
			return true
		}
	}
	return false
}

// sideEffectToolPrefixes 有副作用的工具名前缀（小写）。
// 这类工具（写入/发送/执行/变更）同轮重复调用可能产生重复副作用，绝不参与去重缓存。
var sideEffectToolPrefixes = []string{
	"send", "create", "batch", "add", "update", "delete", "save", "set",
	"follow", "cleanup", "mark", "write", "edit", "upload", "remove",
	"execute", "markdowntoimage", "interactiveanswer",
}

// readOnlyToolPrefixes 明确只读的工具名前缀（小写）。
// 内置数据工具命名规范为 Get/Search/Query 等查询动词，同轮同参数调用结果可复用。
var readOnlyToolPrefixes = []string{
	"get", "search", "query", "list", "read", "fetch", "find", "filter",
	"hot", "top", "global", "check", "count",
}

// isDedupSafeTool 判断工具是否可安全参与同轮去重缓存。
//
// 双名单判定：命中副作用前缀 → false；命中只读前缀 → true；
// 两者都不命中（如未知名称的 MCP 工具）→ false（保守不去重，避免
// 缓存到写操作导致"第二次发送被跳过"这类事故）。
func isDedupSafeTool(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	for _, p := range sideEffectToolPrefixes {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	for _, p := range readOnlyToolPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// toolDedupKey 构造同轮去重缓存键：工具名 + 参数 SHA1。
// 参数为模型生成的 JSON 字符串，同轮相同调用产生的字符串一致；
// 用哈希避免超长参数（如大段文本入库类参数）撑大缓存键。
func toolDedupKey(name, args string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(args)))
	return strings.ToLower(strings.TrimSpace(name)) + "|" + hex.EncodeToString(sum[:])
}
