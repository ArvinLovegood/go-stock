package agent

import (
	"fmt"
	"go-stock/backend/data"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// buildAgentTimeContext 注入当前时间与市场状态，避免模型依赖训练数据中的过时时间/行情。
func buildAgentTimeContext() string {
	now := time.Now()
	weekday := data.WeekdayCN(now.Weekday())

	var sb strings.Builder
	sb.WriteString("\n\n【当前环境】\n")
	sb.WriteString(fmt.Sprintf("- 本地时间：%s %s\n", now.Format("2006-01-02 15:04:05"), weekday))

	// 全球指数状态走 30s TTL 缓存（sysprompt_cache.go），避免每轮都调用外部 API。
	if status := getMarketStatusShortTTL(); status != "" {
		sb.WriteString("- 全球市场状态：\n")
		for _, line := range strings.Split(status, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	}

	sb.WriteString("- A股是否交易日请以 IsTradingDay 工具查询为准（含法定节假日与调休）。\n")
	sb.WriteString("- 对话历史中的股价、财务等数字可能已过时；涉及具体数值必须重新调用工具，不得直接引用历史数字或训练记忆。\n")
	sb.WriteString("- 工具返回以 [as_of=...] [tool=...] [status=...] 元数据为准；status=empty/error 时不得编造数据。\n")

	return sb.String()
}

// loadProjectInstructions 从工作目录递归向上查找项目指令文件（.go-stock.md 或 AGENTS.md），
// 合并多级指令内容后拼装到系统提示词。文件全部缺失时返回空字符串，不阻断主流程。
//
// 设计参考：Claude Code（CLAUDE.md 递归向上）、Cursor（.cursorrules）、OpenAI Codex（AGENTS.md）。
// 优先级：同一目录下 .go-stock.md > AGENTS.md（只取一个）；越靠近当前目录优先级越高（放最后）。
// 查找深度上限为 5 层，避免在根目录意外命中系统级 AGENTS.md。
func loadProjectInstructions(workDir string) string {
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil || wd == "" {
			return ""
		}
		workDir = wd
	}

	const maxDepth = 5
	var instructions []string
	seen := make(map[string]bool)

	dir := workDir
	for i := 0; i < maxDepth; i++ {
		for _, name := range []string{".go-stock.md", "AGENTS.md"} {
			path := filepath.Join(dir, name)
			abs, err := filepath.Abs(path)
			if err != nil || seen[abs] {
				continue
			}
			// 走文件级 mtime 缓存（sysprompt_cache.go），mtime 不变时免读盘。
			content, _, _ := globalFileCache.readCachedFile(path)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				instructions = append(instructions, fmt.Sprintf("# 项目指令（%s）\n%s", name, trimmed))
				seen[abs] = true
				break // 同一目录只取一个文件
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if len(instructions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n【项目指令】\n")
	// 从外层到内层合并：根目录在前，当前目录在后（优先级递增）
	for i := len(instructions) - 1; i >= 0; i-- {
		sb.WriteString(instructions[i])
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// loadUserProfile 读取用户偏好文件（用户数据目录 memory/user_profile.md）并注入到系统提示词。
// 文件不存在或为空时返回空字符串，不阻断主流程。
//
// 设计参考：Claude Code（~/.claude/profile.md）、Cursor（用户设置）。
// 让 Agent 跨会话记住用户的风险偏好、关注市场、默认分析模式等。
func loadUserProfile() string {
	if !IsUserProfileEnabled() {
		return ""
	}
	rootDir := deepAgentRootDir()
	if rootDir == "" || rootDir == "." {
		return ""
	}
	path := filepath.Join(rootDir, "memory", "user_profile.md")
	// 走文件级 mtime 缓存（sysprompt_cache.go），文件未修改时免读盘。
	content, _, _ := globalFileCache.readCachedFile(path)
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	// 主动承接引导：让 Agent 开场承接用户关注标的/持仓，但须先经工具确认实时数据。
	return "\n\n【用户偏好】\n" + text +
		"\n\n开场时可主动承接用户的关注标的或持仓（参考以上画像），" +
		"但涉及行情/价格等实时数据时，必须先调用工具获取最新值，不得凭画像直接断言。\n"
}

// sessionStockCodeRe 匹配用户问题中常见的股票代码格式。
// A股：6 位数字且以 6/0/3 开头（沪市/深市/创业板），可选 .SH/.SZ/.BJ 后缀
// 港股：5 位数字 + .HK 后缀
// 不匹配纯 4 位数字（年份如 2024）和 8 位数字（日期如 20260811），减少误匹配。
var sessionStockCodeRe = regexp.MustCompile(`\b([630]\d{5})(\.(?:SH|SZ|BJ))?\b|\b(\d{5}\.HK)\b`)

// buildSessionContext 从用户问题中提取股票代码，注入"当前会话状态"。
// 帮助 Agent 在多轮对话中聚焦用户当前关注的标的，减少反复确认。
// 提取失败或无代码时返回空字符串，不阻断主流程。
func buildSessionContext(question string) string {
	if question == "" {
		return ""
	}
	matches := sessionStockCodeRe.FindAllString(question, -1)
	if len(matches) == 0 {
		return ""
	}
	seen := make(map[string]bool)
	var codes []string
	for _, m := range matches {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		codes = append(codes, m)
	}
	if len(codes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n【当前会话状态】\n")
	sb.WriteString("- 用户问题中提到的标的：" + strings.Join(codes, "、") + "\n")
	sb.WriteString("- 后续工具调用请优先针对这些标的查询\n")
	return sb.String()
}
