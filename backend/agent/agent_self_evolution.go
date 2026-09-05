package agent

// agent_self_evolution.go — Agent 自我审视与持续进化核心模块。
//
// 对标 Hermes Agent 的核心能力：将 SOUL.md / MEMORY.md / .learnings/LEARNINGS.md
// 注入到系统提示词，并在任务完成后基于规则检测触发反思，自动沉淀经验。
//
// 设计原则：
//   - 零额外 API 成本：反思触发用规则检测（关键词/信号），不调 LLM
//   - 不阻断主流程：所有文件操作失败仅记录日志，不影响 ChatWithContext
//   - 容错：文件不存在 / 读取失败 / 写入失败均安全降级
//   - 全量覆盖铁律：MEMORY.md 修改走 read → write 全量覆盖
//   - 容量约束：SOUL ≤ 200 行 / MEMORY ≤ 80 行（不主动突破）/ LEARNINGS 每条 ≤ 30 行
//
// 接入点：
//   - ChatWithContext（agent_api.go）：sysPrompt += buildSelfEvolutionPrompt(rootDir)
//   - runReact / runDeepAgents / tryPlanExecute：archiveAnalysisReport 后调用
//     triggerPostTaskReflection

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const (
	soulFileName       = "SOUL.md"
	memoryFileName     = "MEMORY.md"
	learningsDirName   = ".learnings"
	learningsFileName  = "LEARNINGS.md"
	memoryDirName      = "memory"
	learningsMarker    = "<!-- 新记录追加在此处上方 -->"
	memoryCriticalHead = "## 关键经验"

	maxSoulLines       = 200
	maxMemoryLines     = 100
	maxRecentLearnings = 3
	maxLearningLines   = 30
	memoryHardLimit    = 80
	recallMaxQuestions = 12
)

// selfEvolutionMu 保护 .learnings/LEARNINGS.md 与 MEMORY.md 的并发写入。
// go-stock 通常单会话使用，但飞书机器人等场景可能并发，加锁保险。
var selfEvolutionMu sync.Mutex

// soulSeedContent 内置 SOUL.md 种子（对标 self-evolution skill 的进化规则 P0-P6，
// 适配股票投资助手语境）。SOUL.md 不存在时自动落盘，保证自进化层开箱即用。
// 用户约定：直接编辑 SOUL.md = 自定义；清空内容 = 禁用；删除文件 = 恢复内置默认。
//
//go:embed soul_seed.md
var soulSeedContent string

// seedSoulIfMissing 在 rootDir 下不存在 SOUL.md 时落盘内置种子。
// 幂等：文件已存在（含用户清空的空文件）则不做任何事，绝不覆盖用户自定义内容。
// 并发安全：多 goroutine 同时首次触发时重复写入相同内容，结果幂等无害。
// 失败仅记日志，不阻断 buildSelfEvolutionPrompt 主流程。
func seedSoulIfMissing(rootDir string) {
	if rootDir == "" || rootDir == "." {
		return
	}
	path := filepath.Join(rootDir, soulFileName)
	if _, err := os.Stat(path); err == nil {
		return
	} else if !os.IsNotExist(err) {
		logger.SugaredLogger.Warnf("检查 SOUL.md 失败: %v (path=%s)", err, path)
		return
	}
	if err := os.WriteFile(path, []byte(soulSeedContent), 0o644); err != nil {
		logger.SugaredLogger.Warnf("落盘内置 SOUL.md 失败（自进化规则不注入）: %v (path=%s)", err, path)
		return
	}
	logger.SugaredLogger.Infof("已落盘内置 SOUL.md 进化规则 (path=%s)", path)
}

// buildSelfEvolutionPrompt 组装自进化片段，注入到系统提示词末尾。
//
// 内容来源（任一缺失则跳过该段，不影响其他段）：
//  1. SOUL.md 全量（进化规则 P0-P6；文件不存在时自动落盘内置种子 soul_seed.md）
//  2. MEMORY.md 全量（长期记忆 + 工作区 + 进化系统参数）
//  3. .learnings/LEARNINGS.md 最近 3 条记录
//  4. 历史相关经验：优先用向量检索（SearchRelevant）按当前问题语义召回 Top-K；
//     向量库未就绪或无结果时降级到 memory/YYYY-MM-DD/ 文件名扫描
//
// 全部内容缺失时返回空字符串，不向系统提示词注入空段落。
//
// question 参数：当前用户问题，用于向量检索语义匹配；为空时跳过向量检索，直接走文件名扫描。
func buildSelfEvolutionPrompt(rootDir, question string) string {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in buildSelfEvolutionPrompt: %v", r)
		}
	}()

	if rootDir == "" {
		rootDir = deepAgentRootDir()
	}
	if rootDir == "" || rootDir == "." {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n【自我进化层】\n")
	hasContent := false

	// SOUL.md 不存在时先落盘内置种子（幂等，不覆盖用户自定义），再读取。
	seedSoulIfMissing(rootDir)

	if soul := loadSoul(rootDir); soul != "" {
		sb.WriteString("## 进化规则（SOUL.md，按优先级 P0→P6 执行）\n")
		sb.WriteString(truncateLines(soul, maxSoulLines))
		sb.WriteString("\n")
		hasContent = true
	}

	if memory := loadMemory(rootDir); memory != "" {
		sb.WriteString("## 长期记忆（MEMORY.md，跨会话生效）\n")
		sb.WriteString(truncateLines(memory, maxMemoryLines))
		sb.WriteString("\n")
		hasContent = true
	}

	if learnings := loadRecentLearnings(rootDir, maxRecentLearnings); learnings != "" {
		sb.WriteString("## 最近经验（LEARNINGS.md）\n")
		sb.WriteString(learnings)
		sb.WriteString("\n")
		hasContent = true
	}

	if recall := loadLastSessionRecall(rootDir, question); recall != "" {
		sb.WriteString("## 历史相关经验\n")
		sb.WriteString(recall)
		sb.WriteString("\n")
		hasContent = true
	}

	if !hasContent {
		return ""
	}

	sb.WriteString("\n---\n")
	sb.WriteString("**自进化约束**：以上规则与记忆必须在下一次行为中调用；不确定→查证（L1/L2/L3）；任务完成后会自动内省；新会话已回顾上次教训。\n")
	return sb.String()
}

// loadSoul 读取 SOUL.md 全量内容。文件不存在返回空字符串。
// 走文件级 mtime 缓存（sysprompt_cache.go），文件未修改时免读盘。
func loadSoul(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	content, _, _ := globalFileCache.readCachedFile(filepath.Join(rootDir, soulFileName))
	return strings.TrimSpace(content)
}

// loadMemory 读取 MEMORY.md 全量内容。文件不存在返回空字符串。
// 走文件级 mtime 缓存（sysprompt_cache.go），文件未修改时免读盘。
func loadMemory(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	content, _, _ := globalFileCache.readCachedFile(filepath.Join(rootDir, memoryFileName))
	return strings.TrimSpace(content)
}

// loadRecentLearnings 读取 LEARNINGS.md 末尾 n 条记录。
//
// 记录以 "## [LRN-" 分隔。返回拼接后的字符串（每条已截断到 maxLearningLines 行）。
// 文件不存在或无记录返回空字符串。
// 走文件级 mtime 缓存（sysprompt_cache.go），文件未修改时免读盘。
func loadRecentLearnings(rootDir string, n int) string {
	if rootDir == "" || n <= 0 {
		return ""
	}
	content, _, _ := globalFileCache.readCachedFile(filepath.Join(rootDir, learningsDirName, learningsFileName))
	if content == "" {
		return ""
	}

	records := splitLearnings(content)
	if len(records) == 0 {
		return ""
	}

	start := len(records) - n
	if start < 0 {
		start = 0
	}
	recent := records[start:]

	var sb strings.Builder
	for _, r := range recent {
		sb.WriteString("---\n")
		sb.WriteString(truncateLines(r, maxLearningLines))
		sb.WriteString("\n")
	}
	return sb.String()
}

// splitLearnings 将 LEARNINGS.md 内容按记录分隔符拆分。
// 记录以 "## [LRN-" 开头，第一段是文件头部（分类目录等），不计入。
func splitLearnings(content string) []string {
	_, records := splitLearningsHead(content)
	return records
}

// splitLearningsHead 将 LEARNINGS.md 内容拆分为头部（首个记录之前的内容）与记录列表。
// 记录以 "## [LRN-" 开头并保留前缀。文件无记录时返回 (原内容, nil)。
func splitLearningsHead(content string) (head string, records []string) {
	idx := strings.Index(content, "## [LRN-")
	if idx < 0 {
		return content, nil
	}
	head = content[:idx]
	rest := content[idx:]
	parts := strings.Split(rest, "## [LRN-")
	records = make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		records = append(records, "## [LRN-"+parts[i])
	}
	return head, records
}

// rebuildLearnings 由头部与记录列表重新拼装 LEARNINGS.md 全文。
func rebuildLearnings(head string, records []string) string {
	return head + strings.Join(records, "")
}

// loadLastSessionRecall 召回与当前问题相关的历史经验。
//
// 主路径：向量检索（SearchRelevant）按 question 语义召回 Top-K 历史问答片段，
//   - 命中时返回格式化后的"历史相关经验"文本（含相似度、日期、模式、问题、回复摘要）
//   - 向量库未初始化、无结果或 question 为空时，降级到文件名扫描 fallback
//
// Fallback：扫描 memory/ 目录下最近的（早于今天的）日期目录，
//
//	提取该目录下所有分析报告的"问题摘要"作为上次对话回顾。
//	文件名格式：HHMMSS_问题摘要.md（由 sanitizeReportFilename 生成）。
//
// 无任何历史数据时返回空字符串。
func loadLastSessionRecall(rootDir, question string) string {
	// 主路径：向量检索
	if strings.TrimSpace(question) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if recalls := SearchRelevant(ctx, question, recallMaxQuestions, CurrentUserKey("")); len(recalls) > 0 {
			return FormatMemoryRecall(recalls, recallMaxQuestions)
		}
	}

	// Fallback：文件名扫描
	return loadLastSessionRecallFromFile(rootDir)
}

// loadLastSessionRecallFromFile 扫描 memory/ 目录下最近的（早于今天的）日期目录，
// 提取该目录下所有分析报告的"问题摘要"作为上次对话回顾。
//
// 文件名格式：HHMMSS_问题摘要.md（由 sanitizeReportFilename 生成）。
// 返回拼接后的字符串，包含日期、问题数量、问题列表。
// 无历史目录返回空字符串。
func loadLastSessionRecallFromFile(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	memoryDir := filepath.Join(rootDir, memoryDirName)
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		// 目录不存在是正常情况（首次使用），静默跳过；其他错误记录日志便于排查。
		if !os.IsNotExist(err) {
			logger.SugaredLogger.Warnf("读取 memory 目录失败: %v (path=%s)", err, memoryDir)
		}
		return ""
	}

	today := time.Now().Format("2006-01-02")
	// 从后往前找最近的、早于今天的日期目录
	var lastDate string
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !isDateDir(name) || name >= today {
			continue
		}
		lastDate = name
		break
	}
	if lastDate == "" {
		return ""
	}

	datePath := filepath.Join(memoryDir, lastDate)
	files, err := os.ReadDir(datePath)
	if err != nil {
		// 日期目录读取失败：NotExist 理论上不会发生（已确认存在），其他错误记录日志。
		if !os.IsNotExist(err) {
			logger.SugaredLogger.Warnf("读取上次对话日期目录失败: %v (path=%s)", err, datePath)
		}
		return ""
	}

	var questions []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		summary := extractQuestionSummary(name)
		if summary != "" {
			questions = append(questions, summary)
		}
	}

	if len(questions) == 0 {
		return ""
	}
	if len(questions) > recallMaxQuestions {
		questions = questions[:recallMaxQuestions]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("上次对话日期: %s\n", lastDate))
	sb.WriteString(fmt.Sprintf("上次问了 %d 个问题（按时间顺序）:\n", len(questions)))
	for i, q := range questions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	sb.WriteString("\n请在回答时参考上次对话的上下文，若用户问题与上次相关可承接；若上次有未完成事项应主动追问。\n")
	return sb.String()
}

// isDateDir 校验目录名是否符合 YYYY-MM-DD 格式。
func isDateDir(name string) bool {
	if len(name) != 10 || name[4] != '-' || name[7] != '-' {
		return false
	}
	for i := 0; i < 10; i++ {
		if i == 4 || i == 7 {
			continue
		}
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}

// extractQuestionSummary 从分析报告文件名提取问题摘要。
// 文件名格式：HHMMSS_问题摘要.md
func extractQuestionSummary(fileName string) string {
	name := strings.TrimSuffix(fileName, ".md")
	idx := strings.Index(name, "_")
	if idx < 0 || idx+1 >= len(name) {
		return name
	}
	return name[idx+1:]
}

// truncateLines 将字符串截断到 maxLines 行，超出部分用 "... (已截断)" 标记。
func truncateLines(s string, maxLines int) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (已截断)\n"
}

// triggerPostTaskReflection 任务完成后基于规则检测触发反思。
//
// 信号检测（源自 SOUL.md P3 自动记忆信号 + 反编造原则）：
//  1. uncertainty_overuse: 回复中"可能/大概/应该/不确定/猜测"出现 ≥ 2 次
//  2. error_recovery: 回复中包含"抱歉/修正/重新分析/纠正/刚才说错"
//  3. user_emotion: 用户问题中包含"算了/没事/好的/不用了/凑合"等情绪词
//  4. long_response_no_tool: 长回复（>2000 字）但未提及工具/查询
//
// 命中任一信号即追加到 .learnings/LEARNINGS.md（LRN 模板）。
// 命中严重信号（uncertainty_overuse / long_response_no_tool）追加到 MEMORY.md 关键经验段。
func triggerPostTaskReflection(question, response string, mode Mode, rootDir string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in triggerPostTaskReflection: %v", r)
		}
	}()

	if strings.TrimSpace(response) == "" {
		return
	}

	if rootDir == "" {
		rootDir = deepAgentRootDir()
	}
	if rootDir == "" || rootDir == "." {
		return
	}

	signals := detectReflectionSignals(question, response)
	if len(signals) == 0 {
		return
	}

	logger.SugaredLogger.Infof("触发任务后反思: mode=%s signals=%v question=%q",
		mode, signals, truncate(question, 60))

	// 追加或去重合并经验：同一问题（WHERE+WHY+归一化问题相同）反复触发时
	// 累计 Recurrence-Count 而非新增重复记录。merged=true 表示命中已有记录。
	merged, err := appendOrMergeLearning(rootDir, question, response, mode, signals)
	if err != nil {
		logger.SugaredLogger.Errorf("追加/合并经验到 LEARNINGS.md 失败: %v", err)
		return
	}
	if merged {
		logger.SugaredLogger.Infof("命中已有经验，Recurrence-Count 累计 (mode=%s signals=%v)", mode, signals)
	} else {
		logger.SugaredLogger.Infof("已沉淀新经验到 LEARNINGS.md (signals=%v)", signals)
	}

	// 严重信号（疑似编造数据 / 不确定表述过多）→ 同步到 MEMORY.md 关键经验段。
	// 仅同步新经验（未命中去重），避免同一教训反复命中关键信号导致 MEMORY.md 重复膨胀。
	if !merged && containsCriticalSignal(signals) {
		learning := formatLearning(question, response, mode, signals, 1, time.Now())
		if err := appendMemoryIfCritical(rootDir, learning); err != nil {
			logger.SugaredLogger.Errorf("追加 MEMORY.md 关键经验失败: %v", err)
		} else {
			logger.SugaredLogger.Infof("已同步严重教训到 MEMORY.md (signals=%v)", signals)
		}
	}
}

// positiveWords 正向反馈/满意度信号词（去噪：仅匹配明确表达满意/认可的词）。
var positiveWords = []string{
	"太有用了", "很有用", "很有帮助", "非常有帮助", "靠谱", "明白了", "清楚了",
	"正合我意", "正是我要", "非常好", "非常满意", "很满意", "谢谢", "感谢", "不错",
}

// detectPositiveSignals 检测用户正向反馈信号，命中任一满意词即视为满意。
func detectPositiveSignals(question string) []string {
	if strings.TrimSpace(question) == "" {
		return nil
	}
	for _, w := range positiveWords {
		if strings.Contains(question, w) {
			return []string{"positive_satisfaction"}
		}
	}
	return nil
}

// triggerPositiveReflection 任务完成后检测用户正向反馈并沉淀正向经验。
//
// 与 triggerPostTaskReflection 互补：负向信号学"错在哪"，正向信号学"什么对用户有用"。
// 命中满意信号时：
//   - 追加/合并到 LEARNINGS.md（signal=positive_satisfaction，Dedup-Key 含 positive）
//   - 记录一条隐式正向反馈（Rating=1）到 agent_feedback，供 P1 画像聚合器使用
//
// 仅在"新经验"（未命中去重）时记录隐式反馈，避免重复满意度反复入库膨胀。
func triggerPositiveReflection(question, response string, mode Mode, rootDir string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in triggerPositiveReflection: %v", r)
		}
	}()

	if strings.TrimSpace(question) == "" {
		return
	}
	if rootDir == "" {
		rootDir = deepAgentRootDir()
	}
	if rootDir == "" || rootDir == "." {
		return
	}

	signals := detectPositiveSignals(question)
	if len(signals) == 0 {
		return
	}

	logger.SugaredLogger.Infof("触发正向反思: mode=%s signals=%v question=%q",
		mode, signals, truncate(question, 60))

	merged, err := appendOrMergeLearning(rootDir, question, response, mode, signals)
	if err != nil {
		logger.SugaredLogger.Errorf("追加正向经验到 LEARNINGS.md 失败: %v", err)
		return
	}
	if merged {
		logger.SugaredLogger.Infof("命中已有正向经验，Recurrence-Count 累计 (mode=%s)", mode)
		return
	}
	logger.SugaredLogger.Infof("已沉淀新正向经验到 LEARNINGS.md (signals=%v)", signals)

	// 新经验时才记录隐式正向反馈，供画像学习使用（失败仅记日志，不阻断）
	if err := recordImplicitFeedback(question, response, mode, true); err != nil {
		logger.SugaredLogger.Warnf("记录隐式正向反馈失败（不影响主流程）: %v", err)
	}
}

// recordImplicitFeedback 将对话中的隐式信号记录为 agent_feedback。
// positive=true → Rating=1；否则 Rating=-1。question/response 截断存储控制体积。
func recordImplicitFeedback(question, response string, mode Mode, positive bool) error {
	rating := -1
	if positive {
		rating = 1
	}
	fb := &models.AgentFeedback{
		UserKey:    CurrentUserKey(""),
		Question:   truncate(question, 500),
		Response:   truncate(response, 2000),
		Rating:     rating,
		Mode:       string(mode),
		FeedbackAt: time.Now(),
	}
	return NewAgentFeedbackApi().SubmitFeedback(fb)
}

// detectReflectionSignals 检测反思信号，返回触发的信号列表。
// 输入：用户问题 + AI 回复。检测基于关键词与长度规则，不调 LLM。
func detectReflectionSignals(question, response string) []string {
	var signals []string

	// 信号1: 不确定表述过多（去噪版）
	// 仅当回复中出现 ≥2 种【不同】的不确定性词才触发，而非按总出现次数计数。
	// 原实现按次数统计，"可能/应该"这类正常分析用语反复出现即误报；
	// 改为区分词种类后，需多种不同的不确定性词并存才判定为真实知识缺口。
	if countDistinctUncertaintyWords(response) >= 2 {
		signals = append(signals, "uncertainty_overuse")
	}

	// 信号2: 错误恢复痕迹（保持精确匹配）
	recoveryWords := []string{"抱歉", "修正", "重新分析", "纠正", "刚才说错", "之前说错", "更正", "说错了"}
	for _, w := range recoveryWords {
		if strings.Contains(response, w) {
			signals = append(signals, "error_recovery")
			break
		}
	}

	// 信号3: 用户情绪信号（去噪版）
	// 仅匹配明确的负面/不满表达，剔除"好的/没事/凑合"等中性常用词（误报率高）。
	emotionWords := []string{"算了", "不用了", "敷衍", "垃圾", "无语", "不满意", "太差了", "没用", "搞什么", "太烂了"}
	for _, w := range emotionWords {
		if strings.Contains(question, w) {
			signals = append(signals, "user_emotion")
			break
		}
	}

	// 信号4: 长回复但未提及工具/查询（可能编造数据）
	if len([]rune(response)) > 2000 &&
		!strings.Contains(response, "工具") &&
		!strings.Contains(response, "查询") &&
		!strings.Contains(response, "调用") {
		signals = append(signals, "long_response_no_tool")
	}

	return signals
}

// countDistinctUncertaintyWords 统计回复中出现的不确定性词【种类】数。
// 同一词重复出现（如"可能"多次）不代表真正的知识缺口；需多种不同的
// 不确定性词并存才更可信地反映"不确定表述过多"，从而显著降低误报。
func countDistinctUncertaintyWords(response string) int {
	uncertaintyWords := []string{"可能", "大概", "应该", "不确定", "猜测", "也许", "或许", "不一定", "难以判断", "有待观察"}
	seen := make(map[string]bool)
	for _, w := range uncertaintyWords {
		if strings.Contains(response, w) {
			seen[w] = true
		}
	}
	return len(seen)
}

// containsCriticalSignal 判断信号集是否包含严重信号。
// 严重信号：uncertainty_overuse（知识缺口）、long_response_no_tool（编造风险）。
func containsCriticalSignal(signals []string) bool {
	for _, s := range signals {
		if s == "long_response_no_tool" || s == "uncertainty_overuse" {
			return true
		}
	}
	return false
}

// formatLearning 按 LRN 模板格式化一条经验记录。
// recurrence 为出现次数（新记录为 1，去重合并时由调用方累计）；去重键 Dedup-Key
// 由 WHERE+WHY+归一化问题生成，供 appendOrMergeLearning 匹配已有记录。
func formatLearning(question, response string, mode Mode, signals []string, recurrence int, now time.Time) string {
	id := fmt.Sprintf("LRN-%s-%s", now.Format("20060102"), now.Format("150405"))
	runeLen := len([]rune(response))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## [%s] reflection\n\n", id))
	sb.WriteString(fmt.Sprintf("**记录时间**: %s\n", now.Format(time.RFC3339)))
	sb.WriteString("**优先级**: medium\n")
	sb.WriteString("**状态**: candidate\n")
	sb.WriteString("**领域**: 通用\n\n")
	sb.WriteString("### 摘要\n")
	sb.WriteString(fmt.Sprintf("任务后反思自动触发，命中信号: %s。问题: %s；回复长度: %d 字符。\n",
		strings.Join(signals, ", "), truncate(question, 100), runeLen))
	sb.WriteString("\n### 详情\n")
	sb.WriteString(fmt.Sprintf("- 模式: %s\n", mode))
	sb.WriteString(fmt.Sprintf("- 触发信号: %s\n", strings.Join(signals, ", ")))
	sb.WriteString("- 信号说明:\n")
	for _, s := range signals {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", s, signalDescription(s)))
	}
	sb.WriteString(fmt.Sprintf("- 问题摘要: %s\n", truncate(question, 200)))
	sb.WriteString("\n### 建议行动\n")
	sb.WriteString("- 后续同类问题注意规避上述信号\n")
	sb.WriteString("- 如涉及数据准确性，下次务必先调用工具查询，不得凭记忆编造\n")
	sb.WriteString("- 若用户表达不满情绪，主动追问而非敷衍\n")
	sb.WriteString("\n### 病理键（v3.0必填）\n")
	sb.WriteString(fmt.Sprintf("- WHERE: %s\n", whereFromMode(mode)))
	sb.WriteString(fmt.Sprintf("- WHY: %s\n", whyFromSignals(signals)))
	sb.WriteString("\n### 元信息\n")
	sb.WriteString("- 来源: auto-reflection\n")
	sb.WriteString("- 相关文件: 无\n")
	sb.WriteString(fmt.Sprintf("- 标签: auto, %s\n", strings.Join(signals, ",")))
	sb.WriteString("- 参见: 无\n")
	sb.WriteString(fmt.Sprintf("- Pattern-Key: %s\n", id))
	sb.WriteString(fmt.Sprintf("- Recurrence-Count: %d\n", recurrence))
	sb.WriteString(fmt.Sprintf("- Dedup-Key: %s\n", computeDedupKey(question, mode, signals)))
	sb.WriteString(fmt.Sprintf("- First-Seen: %s\n", now.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("- Last-Seen: %s\n", now.Format("2006-01-02")))
	sb.WriteString("- VFM: 50\n")
	sb.WriteString("\n---\n")
	return sb.String()
}

// signalDescription 返回信号的人类可读说明。
func signalDescription(s string) string {
	switch s {
	case "uncertainty_overuse":
		return "回复中不确定表述（可能/大概/应该等）出现 ≥ 2 次，存在知识缺口"
	case "error_recovery":
		return "回复中包含错误恢复痕迹（抱歉/修正/重新分析等），存在纠错信号"
	case "user_emotion":
		return "用户问题中包含情绪词（算了/没事/好的等），可能存在不满"
	case "long_response_no_tool":
		return "长回复（>2000字）但未提及工具调用，存在编造数据风险"
	case "positive_satisfaction":
		return "用户对回复表达满意（太有用了/靠谱/谢谢等），说明此类输出风格值得保留"
	}
	return s
}

// whereFromMode 将 Agent 模式映射到病理键 WHERE。
func whereFromMode(mode Mode) string {
	switch mode {
	case React:
		return "react_agent"
	case PlanExecute:
		return "plan_execute"
	case DeepAgents:
		return "deep_agents"
	}
	return "unknown"
}

// whyFromSignals 将信号集映射到病理键 WHY（取第一个匹配）。
func whyFromSignals(signals []string) string {
	for _, s := range signals {
		switch s {
		case "uncertainty_overuse":
			return "knowledge_gap"
		case "error_recovery":
			return "correction"
		case "user_emotion":
			return "emotion_misread"
		case "long_response_no_tool":
			return "data_fabrication_risk"
		case "positive_satisfaction":
			return "positive"
		}
	}
	return "auto"
}

// appendOrMergeLearning 追加或去重合并一条经验到 .learnings/LEARNINGS.md。
//
// 去重逻辑：按 Dedup-Key（WHERE + WHY + 归一化问题）查找已有记录，
//   - 命中：累计该记录的 Recurrence-Count 并更新 Last-Seen，返回 merged=true
//   - 未命中：按 appendLearning 逻辑追加新记录（Recurrence-Count=1），返回 merged=false
//
// 调用方无需持锁；本函数内对 LEARNINGS.md 的 read→modify→write 全程持锁保证并发安全。
func appendOrMergeLearning(rootDir, question, response string, mode Mode, signals []string) (merged bool, err error) {
	selfEvolutionMu.Lock()
	defer selfEvolutionMu.Unlock()

	now := time.Now()
	dedupKey := computeDedupKey(question, mode, signals)

	path := filepath.Join(rootDir, learningsDirName, learningsFileName)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("创建 .learnings 目录失败: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("读取 LEARNINGS.md 失败: %w", err)
	}
	content := string(data)

	head, records := splitLearningsHead(content)
	for _, rec := range records {
		if extractDedupKey(rec) == dedupKey {
			updated := incrementRecurrence(rec, now)
			for i, r := range records {
				if r == rec {
					records[i] = updated
					break
				}
			}
			if err := os.WriteFile(path, []byte(rebuildLearnings(head, records)), 0o644); err != nil {
				return false, fmt.Errorf("更新 LEARNINGS.md 失败: %w", err)
			}
			return true, nil
		}
	}

	// 未命中：追加新记录
	learning := formatLearning(question, response, mode, signals, 1, now)
	if err := appendLearning(path, learning); err != nil {
		return false, err
	}
	return false, nil
}

// computeDedupKey 生成经验去重键：WHERE + WHY + 归一化问题。
// 同一问题（WHERE+WHY 相同）反复触发同一失败信号时合并为一条经验。
func computeDedupKey(question string, mode Mode, signals []string) string {
	return whereFromMode(mode) + "|" + whyFromSignals(signals) + "|" + normalizeQuestionForDedup(question)
}

// normalizeQuestionForDedup 归一化问题用于去重：去除首尾空白、折叠连续空白、统一小写。
// 不改变中文内容，仅对 ASCII（如股票代码、英文）做小写归一。
func normalizeQuestionForDedup(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(q)), " "))
}

// extractDedupKey 从已有经验记录中读取 Dedup-Key（formatLearning 写入的元信息）。
// 旧记录（无该字段）返回空字符串，不会与新记录匹配，保证向后兼容。
func extractDedupKey(rec string) string {
	re := regexp.MustCompile(`(?m)^- Dedup-Key: (.+)$`)
	if m := re.FindStringSubmatch(rec); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// incrementRecurrence 将已有经验记录的 Recurrence-Count 加 1，并更新 Last-Seen 日期。
func incrementRecurrence(rec string, now time.Time) string {
	re := regexp.MustCompile(`(?m)^- Recurrence-Count: (\d+)$`)
	if m := re.FindStringSubmatch(rec); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			rec = re.ReplaceAllString(rec, fmt.Sprintf("- Recurrence-Count: %d", n+1))
		}
	}
	reLS := regexp.MustCompile(`(?m)^- Last-Seen: .*$`)
	if reLS.MatchString(rec) {
		rec = reLS.ReplaceAllString(rec, fmt.Sprintf("- Last-Seen: %s", now.Format("2006-01-02")))
	}
	return rec
}

// appendLearning 追加一条经验到 LEARNINGS.md（追加新记录，不做去重）。
//
// 写入策略：读取全量 → 在 "<!-- 新记录追加在此处上方 -->" 标记前插入新记录 → 全量覆盖。
// 符合 mu-self-evolve 铁律：文件修改走 read → write 全量覆盖。
// 文件不存在时创建初始头部并追加记录。
func appendLearning(path string, learning string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 .learnings 目录失败: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 LEARNINGS.md 失败: %w", err)
	}

	content := string(data)
	var newContent string
	if strings.Contains(content, learningsMarker) {
		// 在标记前插入（保持标记位置）
		newContent = strings.Replace(content, learningsMarker, learning+"\n"+learningsMarker, 1)
	} else if content == "" {
		// 文件不存在或为空，初始化头部
		newContent = "# LEARNINGS.md — 经验沉淀库\n\n" +
			"> 本文件记录可复用的经验教训。\n\n" +
			learning + "\n" + learningsMarker + "\n"
	} else {
		// 无标记，直接追加到末尾
		newContent = content + "\n" + learning + "\n"
	}

	return os.WriteFile(path, []byte(newContent), 0o644)
}

// appendMemoryIfCritical 将严重教训摘要追加到 MEMORY.md 的"关键经验"段。
//
// 严格遵守 MEMORY.md ≤ 80 行铁律：若已达上限，不追加并记录警告日志。
// 写入策略：读取全量 → 在 "## 关键经验" 标记后插入摘要 → 全量覆盖。
func appendMemoryIfCritical(rootDir string, entry string) error {
	path := filepath.Join(rootDir, memoryFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取 MEMORY.md 失败: %w", err)
		}
		// 文件不存在，创建初始结构
		data = []byte("# MEMORY.md — 长期记忆（≤80 行）\n\n## 工作区\n\n## 进化系统\n\n## 关键经验\n")
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) >= memoryHardLimit {
		logger.SugaredLogger.Warnf("MEMORY.md 已达 %d 行，跳过追加（铁律 ≤80 行）", len(lines))
		return nil
	}

	summary := extractLearningSummary(entry)
	if summary == "" {
		return nil
	}

	memoryEntry := fmt.Sprintf("- %s (auto %s)", summary, time.Now().Format("01-02"))

	var newContent string
	if strings.Contains(content, memoryCriticalHead) {
		// 在 "## 关键经验" 后的下一行插入
		idx := strings.Index(content, memoryCriticalHead)
		endOfLine := strings.Index(content[idx:], "\n")
		if endOfLine < 0 {
			newContent = content + "\n" + memoryEntry + "\n"
		} else {
			insertPos := idx + endOfLine + 1
			newContent = content[:insertPos] + memoryEntry + "\n" + content[insertPos:]
		}
	} else {
		newContent = content + "\n" + memoryCriticalHead + "\n" + memoryEntry + "\n"
	}

	return os.WriteFile(path, []byte(newContent), 0o644)
}

// extractLearningSummary 从 formatLearning 生成的记录文本中提取摘要行。
func extractLearningSummary(entry string) string {
	lines := strings.Split(entry, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "### 摘要") && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1])
		}
	}
	return ""
}
