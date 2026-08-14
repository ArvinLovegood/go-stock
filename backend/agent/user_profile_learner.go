package agent

// user_profile_learner.go — 用户画像自动学习（P1）。
//
// 把用户行为数据（关注列表 / 交易记录 / 显式反馈）低频压缩成精简的
// user_profile.md，交给已有的 loadUserProfile（agent_context.go）注入系统提示词，
// 让 Agent 跨会话"懂用户"。
//
// 设计原则：
//   - 零侵入：复用 loadUserProfile 的读取路径（用户数据目录 memory/user_profile.md），
//     只是把"手写"升级为"自动生成 + 可编辑覆盖"。
//   - 低频异步：画像生成用 LLM 但不阻塞 ChatWithContext 主流程；失败仅记日志并
//     回退到规则模板，不影响主流程。
//   - 透明可控：前端可预览 / 手动覆盖 / 一键重新学习 / 清空（见 user_profile_api.go）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// UserProfileLearner 用户画像学习器（Wails 绑定）
type UserProfileLearner struct{}

var userProfileMu sync.Mutex

// NewUserProfileLearner 构造画像学习器实例
func NewUserProfileLearner() *UserProfileLearner {
	return &UserProfileLearner{}
}

// userProfilePath 返回 user_profile.md 完整路径。
// 与 loadUserProfile（agent_context.go）读取路径保持一致。
// 画像存放在程序（可执行文件）所在目录的 memory 子目录下。
func userProfilePath() string {
	rootDir := deepAgentRootDir()
	if rootDir == "" || rootDir == "." {
		return ""
	}
	return filepath.Join(rootDir, "memory", "user_profile.md")
}

func userProfileDisabledPath() string {
	rootDir := deepAgentRootDir()
	if rootDir == "" || rootDir == "." {
		return ""
	}
	return filepath.Join(rootDir, "memory", "user_profile.disabled")
}

func IsUserProfileEnabled() bool {
	p := userProfileDisabledPath()
	if p == "" {
		return true
	}
	_, err := os.Stat(p)
	return os.IsNotExist(err)
}

func SetUserProfileEnabled(enabled bool) error {
	p := userProfileDisabledPath()
	if p == "" {
		return fmt.Errorf("无法定位用户画像启用状态路径")
	}
	if enabled {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("创建 memory 目录失败: %w", err)
	}
	return os.WriteFile(p, []byte("disabled\n"), 0o644)
}

// Get 读取当前画像内容（未生成或不存在时返回空字符串）。
func (u *UserProfileLearner) Get() string {
	p := userProfilePath()
	if p == "" {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// UpdatedAt 返回画像文件最后更新时间。
func (u *UserProfileLearner) UpdatedAt() string {
	p := userProfilePath()
	if p == "" {
		return ""
	}
	info, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return info.ModTime().Format("2006-01-02 15:04:05")
}

// Save 手动覆盖画像（原子写）。content 为空视为非法，需用 Clear 清空。
func (u *UserProfileLearner) Save(content string) error {
	p := userProfilePath()
	if p == "" {
		return fmt.Errorf("无法定位 user_profile.md 路径")
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("画像内容不能为空（清空请用 ClearUserProfile）")
	}
	return writeUserProfileAtomic(p, content)
}

// Clear 清空画像（删除文件）。
func (u *UserProfileLearner) Clear() error {
	p := userProfilePath()
	if p == "" {
		return nil
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Relearn 重新学习：汇总行为数据 -> LLM 生成 -> 原子写 -> 标记反馈已处理。
// 返回最终写入的画像内容。
func (u *UserProfileLearner) Relearn() (string, error) {
	snapshot := u.gatherProfileData()

	profile, err := u.buildProfileWithLLM(snapshot.text)
	if err != nil {
		logger.SugaredLogger.Warnf("用户画像 LLM 生成失败，回退规则模板: %v", err)
		profile = u.buildProfileRules(snapshot.text)
	}
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", fmt.Errorf("画像生成结果为空")
	}

	p := userProfilePath()
	if p == "" {
		return "", fmt.Errorf("无法定位 user_profile.md 路径")
	}
	if err := writeUserProfileAtomic(p, profile); err != nil {
		return "", err
	}
	u.markFeedbackProcessed(snapshot.feedbackIDs)
	logger.SugaredLogger.Infof("用户画像已重新学习并写入: %s", p)
	return profile, nil
}

// writeUserProfileAtomic 原子写入画像文件（tmp + rename），避免中断损坏。
func writeUserProfileAtomic(path, content string) error {
	userProfileMu.Lock()
	defer userProfileMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 memory 目录失败: %w", err)
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".user_profile-*.tmp")
	if err != nil {
		return fmt.Errorf("创建画像临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if err := tmpFile.Chmod(0o644); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("设置画像临时文件权限失败: %w", err)
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("写入画像临时文件失败: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("同步画像临时文件失败: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭画像临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("替换画像文件失败: %w", err)
	}
	return nil
}

type profileDataSnapshot struct {
	text        string
	feedbackIDs []uint
}

// gatherProfileData 汇总行为数据为文本快照，作为 LLM 生成画像的输入。
// 全部来自本地现有数据，不发起网络请求。
func (u *UserProfileLearner) gatherProfileData() profileDataSnapshot {
	var sb strings.Builder
	var feedbackIDs []uint

	// 关注列表：市场分布、标的、风险偏好
	var follows []data.FollowedStock
	if err := db.Dao.Model(&data.FollowedStock{}).Order("sort asc,time desc").Find(&follows).Error; err == nil && len(follows) > 0 {
		sb.WriteString("【关注列表】\n")
		for _, f := range follows {
			market := marketFromCode(f.StockCode)
			stopPct := riskPctFromPrices(f.EntryPrice, f.StopLossPrice)
			sb.WriteString(fmt.Sprintf("- %s(%s) 市场=%s", f.Name, f.StockCode, market))
			if f.EntryPrice > 0 {
				sb.WriteString(fmt.Sprintf(" 成本=%.2f", f.EntryPrice))
			}
			if stopPct > 0 {
				sb.WriteString(fmt.Sprintf(" 止损≈%.1f%%", stopPct))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("【关注列表】无\n")
	}

	// 交易记录：最近 30 条
	var records []data.TradingRecord
	if err := db.Dao.Model(&data.TradingRecord{}).Order("trading_time desc").Limit(30).Find(&records).Error; err == nil && len(records) > 0 {
		sb.WriteString("\n【最近交易记录】\n")
		for _, r := range records {
			sb.WriteString(fmt.Sprintf("- %s %s %s 价格=%.2f 量=%d",
				r.TradingTime.Format("01-02"), r.Direction, r.StockName, r.Price, r.Volume))
			if r.StopLossPrice > 0 {
				sb.WriteString(fmt.Sprintf(" 止损=%.2f", r.StopLossPrice))
			}
			if r.TakeProfitPrice > 0 {
				sb.WriteString(fmt.Sprintf(" 止盈=%.2f", r.TakeProfitPrice))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n【最近交易记录】无\n")
	}

	// 显式反馈：最近 20 条未处理 + 已处理各取部分，用于识别认可/规避的分析风格
	var feedbacks []models.AgentFeedback
	currentUserKey := CurrentUserKey("")
	// 显式反馈可能带 session 维度，隐式反馈通常只有机器维度；两者都必须
	// 限定在当前机器，不能把其他用户的反馈混入画像。
	feedbackScope := "(user_key = ? OR user_key LIKE ?)"
	if err := db.Dao.Model(&models.AgentFeedback{}).
		Where(feedbackScope, currentUserKey, currentUserKey+":s:%").
		Order("feedback_at desc").Limit(20).Find(&feedbacks).Error; err == nil && len(feedbacks) > 0 {
		sb.WriteString("\n【用户反馈】\n")
		for _, fb := range feedbacks {
			if !fb.Processed {
				feedbackIDs = append(feedbackIDs, fb.ID)
			}
			label := "没用"
			if fb.Rating == 1 {
				label = "有用"
			}
			sb.WriteString(fmt.Sprintf("- [%s] 问题:%s", label, strings.TrimSpace(fb.Question)))
			if fb.Reason != "" {
				sb.WriteString(fmt.Sprintf(" 原因:%s", strings.TrimSpace(fb.Reason)))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n【用户反馈】无\n")
	}

	return profileDataSnapshot{text: sb.String(), feedbackIDs: feedbackIDs}
}

// marketFromCode 从股票代码前缀推断市场（sh/sz=A股，hk=港股，gb=美股，csi=指数）。
func marketFromCode(code string) string {
	lower := strings.ToLower(code)
	switch {
	case strings.HasPrefix(lower, "sh"), strings.HasPrefix(lower, "sz"), strings.HasPrefix(lower, "bj"):
		return "A股"
	case strings.HasPrefix(lower, "hk"):
		return "港股"
	case strings.HasPrefix(lower, "gb"):
		return "美股"
	case strings.HasPrefix(lower, "csi"):
		return "指数"
	default:
		return "未知"
	}
}

// riskPctFromPrices 由成本价与止损价估算风险偏好（止损比例 %，>0 才返回）。
func riskPctFromPrices(entry, stop float64) float64 {
	if entry <= 0 || stop <= 0 || stop >= entry {
		return 0
	}
	return (entry - stop) / entry * 100
}

// profileTemplate LLM 生成画像的固定输出模板（限制 ≤30 行，控制 token）。
const profileTemplate = `你是用户画像分析师。根据给定的用户行为数据（关注列表、交易记录、反馈），生成一份精简的用户画像，输出为固定格式 Markdown，不超过 30 行。只输出画像本身，不要解释。

格式：
## 用户画像
- 关注市场：A股/港股/美股等
- 关注标的：<主要关注的股票名称/代码，最多5个>
- 持仓与成本：<如有，简述>
- 风险偏好：<从止损比例推断：保守/稳健/激进，附平均止损%>
- 常用分析维度：<从问题与记录推断，如基本面/技术面/资金流>
- 偏好格式：<从反馈推断，如表格/简答/详细报告；无则写"未明确">
- 需规避项：<从"没用"反馈推断；无则写"无">

若某维度无法判断，写"未明确"。不要编造数据。`

// buildProfileWithLLM 用 LLM 生成画像。失败返回 error（调用方回退规则模板）。
func (u *UserProfileLearner) buildProfileWithLLM(dataText string) (string, error) {
	cfgID := resolveChatAIConfigID()
	if cfgID == 0 {
		return "", fmt.Errorf("未配置可用的对话 AI 服务")
	}
	out, err := callLLMSync(cfgID, profileTemplate, dataText)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("LLM 返回空画像")
	}
	validated, err := validateGeneratedProfile(out)
	if err != nil {
		return "", err
	}
	return validated, nil
}

// validateGeneratedProfile 只允许固定画像字段进入系统提示词，避免模型把
// 原始反馈中的指令、代码块或其他文本直接写入 user_profile.md。
func validateGeneratedProfile(content string) (string, error) {
	allowed := map[string]bool{
		"关注市场":   true,
		"关注标的":   true,
		"持仓与成本":  true,
		"风险偏好":   true,
		"常用分析维度": true,
		"偏好格式":   true,
		"需规避项":   true,
	}
	values := make(map[string]string)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.Trim(raw, "`"))
		if line == "## 用户画像" || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		idx := strings.Index(line, "：")
		if idx <= 0 {
			continue
		}
		label := strings.TrimSpace(line[:idx])
		if !allowed[label] || values[label] != "" {
			continue
		}
		value := strings.TrimSpace(line[idx+len("："):])
		value = strings.ReplaceAll(value, "\n", " ")
		value = strings.ReplaceAll(value, "```", "")
		if len([]rune(value)) > 240 {
			value = string([]rune(value)[:240]) + "…"
		}
		if value != "" {
			values[label] = value
		}
	}
	if len(values) == 0 {
		return "", fmt.Errorf("LLM 返回的画像不符合固定字段格式")
	}

	labels := []string{"关注市场", "关注标的", "持仓与成本", "风险偏好", "常用分析维度", "偏好格式", "需规避项"}
	var lines []string
	for _, label := range labels {
		value := values[label]
		if value == "" {
			value = "未明确"
		}
		lines = append(lines, "- "+label+"："+value)
	}
	return "## 用户画像\n" + strings.Join(lines, "\n"), nil
}

// buildProfileRules 规则回退：不依赖 LLM，从数据直接提取关键维度。
// 保证即使 AI 服务不可用，画像仍能自动生成（功能降级，不阻断）。
func (u *UserProfileLearner) buildProfileRules(dataText string) string {
	// 简化：提取关注市场与标的名，其余标注"未明确"
	var markets, names []string
	var lines []string
	seen := map[string]bool{}
	for _, line := range strings.Split(dataText, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimPrefix(line, "- ")
		// 形如 "名称(代码) 市场=X"
		if idx := strings.Index(body, "("); idx > 0 {
			name := body[:idx]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		if idx := strings.Index(body, "市场="); idx > 0 {
			m := strings.TrimSpace(body[idx+len("市场="):])
			if m != "" && m != "未知" {
				markets = append(markets, m)
			}
		}
	}
	if len(markets) > 0 {
		lines = append(lines, "- 关注市场："+uniqueJoin(markets, "/"))
	} else {
		lines = append(lines, "- 关注市场：未明确")
	}
	if len(names) > 0 {
		top := names
		if len(top) > 5 {
			top = top[:5]
		}
		lines = append(lines, "- 关注标的："+strings.Join(top, "、"))
	} else {
		lines = append(lines, "- 关注标的：未明确")
	}
	lines = append(lines, "- 风险偏好：未明确", "- 常用分析维度：未明确", "- 偏好格式：未明确", "- 需规避项：无")
	return "## 用户画像\n" + strings.Join(lines, "\n")
}

// uniqueJoin 去重后拼接字符串。
func uniqueJoin(items []string, sep string) string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	return strings.Join(out, sep)
}

// markFeedbackProcessed 将已用于画像学习的反馈标记为已处理，避免重复聚合。
func (u *UserProfileLearner) markFeedbackProcessed(ids []uint) {
	if len(ids) == 0 {
		return
	}
	if err := db.Dao.Model(&models.AgentFeedback{}).
		Where("id IN ? AND processed = ?", ids, false).
		Update("processed", true).Error; err != nil {
		logger.SugaredLogger.Warnf("标记用户画像反馈已处理失败: %v", err)
	}
}
