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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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

// Relearn 重新学习：汇总行为数据 -> 与现有画像增量调和（LLM）-> 原子写 -> 标记反馈已处理。
// 调和语义参考 Mem0 的 ADD/UPDATE/DELETE/NOOP：仍然成立的事实保留、变化的更新、被推翻的删除，
// 避免全量重建导致稳定字段随单次数据波动翻转。
// 返回最终写入的画像内容。
func (u *UserProfileLearner) Relearn() (string, error) {
	snapshot := u.gatherProfileData()
	existing := u.Get()

	profile, err := u.buildProfileWithLLM(snapshot.text, existing)
	if err != nil {
		logger.SugaredLogger.Warnf("用户画像 LLM 生成失败（数据快照 %d 字符），回退规则模板: %v", len([]rune(snapshot.text)), err)
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

// correctionRelearnMu/correctionRelearnAt 纠正触发学习的防抖：
// 负面反馈往往连续到达（用户连点多个 👎），10 分钟窗口内只触发一次增量学习。
var (
	correctionRelearnMu sync.Mutex
	correctionRelearnAt time.Time
)

// RelearnAfterCorrection 纠正即学习（参考 Claude Code Auto Memory：用户纠正 Agent 时立即记笔记）。
// 由负面反馈（👎）触发：异步增量更新画像，把纠正内容合入"需规避项/偏好格式"等字段。
//
// 约束：
//   - 已有画像才学习——不凭单条纠正冷启动（避免噪声覆盖全量数据）。
//   - LLM 失败直接放弃（保留旧画像），不回退规则模板——规则模板无法理解纠正语义。
//   - 10 分钟防抖，避免连续反馈触发多次 LLM 调用。
func (u *UserProfileLearner) RelearnAfterCorrection() {
	go func() {
		correctionRelearnMu.Lock()
		if time.Since(correctionRelearnAt) < 10*time.Minute {
			correctionRelearnMu.Unlock()
			return
		}
		correctionRelearnAt = time.Now()
		correctionRelearnMu.Unlock()

		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("纠正触发的画像学习 panic（已忽略）: %v", r)
			}
		}()

		existing := u.Get()
		if existing == "" {
			return // 尚无画像，不冷启动
		}
		snapshot := u.gatherProfileData()
		profile, err := u.buildProfileWithLLM(snapshot.text, existing)
		if err != nil {
			logger.SugaredLogger.Debugf("纠正触发的画像学习失败（保留旧画像）: %v", err)
			return
		}
		if strings.TrimSpace(profile) == "" {
			return
		}
		p := userProfilePath()
		if p == "" {
			return
		}
		if err := writeUserProfileAtomic(p, profile); err != nil {
			logger.SugaredLogger.Warnf("纠正触发的画像学习写入失败: %v", err)
			return
		}
		u.markFeedbackProcessed(snapshot.feedbackIDs)
		logger.SugaredLogger.Infof("用户画像已根据最新纠正增量更新")
	}()
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
//
// 数据来源（充分采集用户习惯与模式）：
//   - 关注列表：标的、成本、止损、报警设置、定时盯盘任务
//   - 股票分组：分组名反映板块/主题偏好
//   - 交易记录：明细（含交易理由与心态）+ 统计概览（方向分布/持仓周期/止损纪律）
//   - 对话历史 + AI 分析报告归档：用户真实提问（含高频提问统计），反映提问模式与分析习惯
//   - AI 推荐股票：评级与板块分布，反映让 AI 推荐股票的习惯
//   - 显式反馈：有用/没用评价 + Agent 模式偏好
func (u *UserProfileLearner) gatherProfileData() profileDataSnapshot {
	var sb strings.Builder
	var feedbackIDs []uint

	// 关注列表：市场分布、持仓明细（成本/止损/止盈）、报警/盯盘聚合统计、按市场分组标的清单。
	// 注意控制快照体积：报警/盯盘只输出聚合计数，不逐行输出明细（否则大关注列表会撑爆 LLM 输入）。
	var follows []data.FollowedStock
	if err := db.Dao.Model(&data.FollowedStock{}).Order("sort asc,time desc").Find(&follows).Error; err == nil && len(follows) > 0 {
		marketCount := map[string]int{}
		namesByMarket := map[string][]string{}
		var holdingLines []string
		var stopPcts []float64
		alarmCount, cronCount := 0, 0
		cronSet := map[string]bool{}
		for _, f := range follows {
			market := marketFromCode(f.StockCode)
			marketCount[market]++
			namesByMarket[market] = append(namesByMarket[market], f.Name)
			stopPct := riskPctFromPrices(f.EntryPrice, f.StopLossPrice)
			if f.EntryPrice > 0 || stopPct > 0 {
				line := fmt.Sprintf("- %s(%s)", f.Name, f.StockCode)
				if f.EntryPrice > 0 {
					line += fmt.Sprintf(" 成本=%.2f", f.EntryPrice)
				}
				if stopPct > 0 {
					line += fmt.Sprintf(" 止损≈%.1f%%", stopPct)
					stopPcts = append(stopPcts, stopPct)
				}
				if f.TakeProfitPrice > 0 {
					line += fmt.Sprintf(" 止盈=%.2f", f.TakeProfitPrice)
				}
				holdingLines = append(holdingLines, line)
			}
			if f.AlarmChangePercent != 0 || f.AlarmPrice != 0 {
				alarmCount++
			}
			if f.Cron != nil && strings.TrimSpace(*f.Cron) != "" {
				cronCount++
				cronSet[strings.TrimSpace(*f.Cron)] = true
			}
		}
		sb.WriteString(fmt.Sprintf("【关注列表】共%d只；市场分布:", len(follows)))
		for _, m := range []string{"A股", "港股", "美股", "指数"} {
			if marketCount[m] > 0 {
				sb.WriteString(fmt.Sprintf("%s%d只 ", m, marketCount[m]))
			}
		}
		sb.WriteString(fmt.Sprintf("；设置报警%d只；定时盯盘%d只", alarmCount, cronCount))
		if len(cronSet) > 0 {
			var crons []string
			for c := range cronSet {
				crons = append(crons, c)
			}
			sort.Strings(crons)
			sb.WriteString("（表达式:" + strings.Join(crons, "、") + "）")
		}
		if len(stopPcts) > 0 {
			var sum float64
			for _, v := range stopPcts {
				sum += v
			}
			sb.WriteString(fmt.Sprintf("；平均止损≈%.1f%%", sum/float64(len(stopPcts))))
		}
		sb.WriteString("\n")
		if len(holdingLines) > 0 {
			sb.WriteString("持仓/成本明细:\n")
			for _, l := range holdingLines {
				sb.WriteString(l + "\n")
			}
		}
		sb.WriteString("标的清单(按市场):\n")
		for _, m := range []string{"A股", "港股", "美股", "指数"} {
			names := namesByMarket[m]
			if len(names) == 0 {
				continue
			}
			shown := names
			if len(shown) > 80 {
				shown = append(append([]string{}, shown[:80]...), fmt.Sprintf("…等共%d只", len(names)))
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m, strings.Join(shown, "、")))
		}
	} else {
		sb.WriteString("【关注列表】无\n")
	}

	// 股票分组：分组名反映用户关注的板块/主题
	var groups []data.Group
	if err := db.Dao.Model(&data.Group{}).Order("sort asc").Find(&groups).Error; err == nil && len(groups) > 0 {
		var names []string
		for _, g := range groups {
			if n := strings.TrimSpace(g.Name); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 0 {
			sb.WriteString("\n【股票分组】" + strings.Join(names, "、") + "\n")
		}
	}

	// 交易记录：最近 100 条明细（含理由与心态）+ 统计概览
	var records []data.TradingRecord
	if err := db.Dao.Model(&data.TradingRecord{}).Order("trading_time desc").Limit(100).Find(&records).Error; err == nil && len(records) > 0 {
		sb.WriteString("\n【最近交易记录】\n")
		buyCount, sellCount, stopSet, tpSet := 0, 0, 0, 0
		for _, r := range records {
			if r.Direction == "买入" {
				buyCount++
				if r.StopLossPrice > 0 {
					stopSet++
				}
				if r.TakeProfitPrice > 0 {
					tpSet++
				}
			} else if r.Direction == "卖出" {
				sellCount++
			}
			sb.WriteString(fmt.Sprintf("- %s %s %s 价格=%.2f 量=%d",
				r.TradingTime.Format("01-02"), r.Direction, r.StockName, r.Price, r.Volume))
			if r.StopLossPrice > 0 {
				sb.WriteString(fmt.Sprintf(" 止损=%.2f", r.StopLossPrice))
			}
			if r.TakeProfitPrice > 0 {
				sb.WriteString(fmt.Sprintf(" 止盈=%.2f", r.TakeProfitPrice))
			}
			if reason := strings.TrimSpace(r.Reason); reason != "" {
				sb.WriteString(" 理由:" + clampRunes(reason, 60))
			}
			if mindset := strings.TrimSpace(r.Mindset); mindset != "" {
				sb.WriteString(" 心态:" + clampRunes(mindset, 40))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("统计:近%d笔 买入%d/卖出%d 买入设止损%d笔(%.0f%%) 设止盈%d笔(%.0f%%) 平均持仓%.1f天\n",
			len(records), buyCount, sellCount, stopSet, pct(stopSet, buyCount), tpSet, pct(tpSet, buyCount),
			avgHoldingDays(records)))
	} else {
		sb.WriteString("\n【最近交易记录】无\n")
	}

	// 对话历史 + AI 分析报告归档：用户真实提问（反映提问模式与分析习惯）。
	// chatQuestions 按时间倒序；reportQuestions 来自 memory/<日期>/ 归档报告头部元信息。
	// 同时统计活跃时段（小时直方图）与最近7天提问量——时间维度（recency）供画像参考。
	var userMessages []db.ChatMemory
	var chatQuestions []string
	hourCount := map[int]int{}
	recent7d, recent30d := 0, 0
	cutoff7d := time.Now().AddDate(0, 0, -7)
	cutoff30d := time.Now().AddDate(0, 0, -30)
	if err := db.Dao.Model(&db.ChatMemory{}).
		Where("role = ?", "user").
		Order("created_at desc").Limit(500).Find(&userMessages).Error; err == nil {
		for _, m := range userMessages {
			if q := strings.TrimSpace(m.Content); q != "" {
				chatQuestions = append(chatQuestions, q)
			}
			hourCount[m.CreatedAt.Hour()]++
			if m.CreatedAt.After(cutoff7d) {
				recent7d++
			}
			if m.CreatedAt.After(cutoff30d) {
				recent30d++
			}
		}
	}
	if len(userMessages) > 0 {
		sb.WriteString(fmt.Sprintf("\n【使用习惯】近%d条用户消息：最近7天%d条、30天%d条；活跃时段:",
			len(userMessages), recent7d, recent30d))
		for _, it := range topNByCountInt(hourCount, 4) {
			sb.WriteString(fmt.Sprintf("%02d点%d条 ", it.key, it.count))
		}
		sb.WriteString("\n")
	}
	reportQuestions, reportModeCount, reportTotal := gatherArchivedReportQuestions(14)

	// AI 分析报告：数量与模式分布（用户让 AI 分析的频率与习惯）
	if reportTotal > 0 {
		sb.WriteString(fmt.Sprintf("\n【AI分析报告（最近14天）】共%d篇；模式分布:", reportTotal))
		for _, it := range topNByCount(reportModeCount, 5) {
			sb.WriteString(fmt.Sprintf("%s=%d篇 ", it.key, it.count))
		}
		sb.WriteString("\n")
	}

	// 高频提问：合并对话历史与报告问题做频次统计，直接反映"用户经常的提问"
	allQuestions := append(append([]string{}, chatQuestions...), reportQuestions...)
	if freq := gatherFrequentQuestions(allQuestions, 15, 2); len(freq) > 0 {
		sb.WriteString("\n【高频提问 TOP】\n")
		for _, it := range freq {
			sb.WriteString(fmt.Sprintf("- %d次 %s\n", it.count, clampRunes(it.question, 100)))
		}
	}

	// 最近提问原文：保留最近 40 条（新→旧），与高频统计互补（覆盖新出现的问题）
	if len(chatQuestions) > 0 {
		sb.WriteString("\n【用户最近提问（新→旧）】\n")
		prev := ""
		shown := 0
		for _, q := range chatQuestions {
			if shown >= 40 {
				break
			}
			// 跳过连续重复；超长内容（如粘贴的文章）截断，只保留开头语义
			if q == prev {
				continue
			}
			prev = q
			shown++
			sb.WriteString("- " + clampRunes(q, 120) + "\n")
		}
	} else {
		sb.WriteString("\n【用户最近提问】无\n")
	}

	// AI 推荐股票：评级与板块分布，反映用户让 AI 推荐股票的习惯（也是一种经常的提问）
	var recs []models.AiRecommendStocks
	if err := db.Dao.Model(&models.AiRecommendStocks{}).Order("data_time desc").Limit(50).Find(&recs).Error; err == nil && len(recs) > 0 {
		ratingCount := map[string]int{}
		bkCount := map[string]int{}
		for _, r := range recs {
			if rating := strings.TrimSpace(r.Rating); rating != "" {
				ratingCount[rating]++
			}
			if bk := strings.TrimSpace(r.BkName); bk != "" {
				bkCount[bk]++
			}
		}
		sb.WriteString(fmt.Sprintf("\n【AI推荐股票（最近%d条）】评级分布:", len(recs)))
		for _, it := range topNByCount(ratingCount, 6) {
			sb.WriteString(fmt.Sprintf("%s=%d ", it.key, it.count))
		}
		sb.WriteString(" 板块TOP:")
		for _, it := range topNByCount(bkCount, 5) {
			sb.WriteString(fmt.Sprintf("%s(%d) ", it.key, it.count))
		}
		sb.WriteString("\n")
	}

	// 显式反馈：最近 50 条，用于识别认可/规避的分析风格；附带模式偏好统计
	var feedbacks []models.AgentFeedback
	currentUserKey := CurrentUserKey("")
	// 显式反馈可能带 session 维度，隐式反馈通常只有机器维度；两者都必须
	// 限定在当前机器，不能把其他用户的反馈混入画像。
	feedbackScope := "(user_key = ? OR user_key LIKE ?)"
	if err := db.Dao.Model(&models.AgentFeedback{}).
		Where(feedbackScope, currentUserKey, currentUserKey+":s:%").
		Order("feedback_at desc").Limit(50).Find(&feedbacks).Error; err == nil && len(feedbacks) > 0 {
		sb.WriteString("\n【用户反馈】\n")
		modeCount := map[string]int{}
		for _, fb := range feedbacks {
			if !fb.Processed {
				feedbackIDs = append(feedbackIDs, fb.ID)
			}
			if fb.Mode != "" {
				modeCount[fb.Mode]++
			}
			label := "没用"
			if fb.Rating == 1 {
				label = "有用"
			}
			sb.WriteString(fmt.Sprintf("- [%s] 问题:%s", label, clampRunes(strings.TrimSpace(fb.Question), 100)))
			if fb.Reason != "" {
				sb.WriteString(" 原因:" + clampRunes(strings.TrimSpace(fb.Reason), 60))
			}
			sb.WriteString("\n")
		}
		if len(modeCount) > 0 {
			sb.WriteString("反馈时的Agent模式分布:")
			for mode, cnt := range modeCount {
				sb.WriteString(fmt.Sprintf("%s=%d次 ", mode, cnt))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n【用户反馈】无\n")
	}

	return profileDataSnapshot{text: sb.String(), feedbackIDs: feedbackIDs}
}

// gatherArchivedReportQuestions 从 memory/<YYYY-MM-DD>/*.md 分析报告归档中
// 提取最近 days 天的问题与模式。只读文件头部元信息（问题/模式行），不解析全文。
// 归档格式见 archiveAnalysisReport（agent_api.go）。
func gatherArchivedReportQuestions(days int) (questions []string, modeCount map[string]int, total int) {
	rootDir := deepAgentRootDir()
	if rootDir == "" || rootDir == "." {
		return nil, nil, 0
	}
	memoryDir := filepath.Join(rootDir, "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil, nil, 0
	}
	// 收集形如 YYYY-MM-DD 的目录名，按名称（即日期字典序）排序取最近 days 个
	var dateDirs []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 10 {
			if _, err := time.Parse("2006-01-02", e.Name()); err == nil {
				dateDirs = append(dateDirs, e.Name())
			}
		}
	}
	sort.Strings(dateDirs)
	if len(dateDirs) > days {
		dateDirs = dateDirs[len(dateDirs)-days:]
	}
	modeCount = map[string]int{}
	for _, d := range dateDirs {
		files, err := os.ReadDir(filepath.Join(memoryDir, d))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".md") {
				continue
			}
			total++
			q, mode := parseReportMeta(filepath.Join(memoryDir, d, f.Name()))
			if mode != "" {
				modeCount[mode]++
			}
			if q != "" {
				questions = append(questions, q)
			}
		}
	}
	return questions, modeCount, total
}

// parseReportMeta 读取报告归档文件头部，提取"问题"与"模式"元信息行。
// 元信息位于文件头部，读前 4KB 足够。
func parseReportMeta(path string) (question, mode string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	head := string(buf[:n])
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if question == "" && strings.HasPrefix(line, "- **问题**:") {
			question = strings.TrimSpace(strings.TrimPrefix(line, "- **问题**:"))
		}
		if mode == "" && strings.HasPrefix(line, "- **模式**:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "- **模式**:"))
		}
	}
	return question, mode
}

// questionFreq 高频提问统计项。
type questionFreq struct {
	question string
	count    int
}

// gatherFrequentQuestions 统计高频提问：归一化（压缩空白、截断）后计数，
// 返回频次降序 topN（仅保留次数 ≥ minCount 的）。同频按字典序稳定排序。
func gatherFrequentQuestions(questions []string, topN, minCount int) []questionFreq {
	counts := map[string]int{}
	first := map[string]string{} // 归一化 key -> 首个原文（保留原始表达）
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		key := clampRunes(strings.Join(strings.Fields(q), ""), 80)
		if key == "" {
			continue
		}
		counts[key]++
		if _, ok := first[key]; !ok {
			first[key] = q
		}
	}
	var items []questionFreq
	for k, c := range counts {
		if c >= minCount {
			items = append(items, questionFreq{question: first[k], count: c})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].question < items[j].question
	})
	if len(items) > topN {
		items = items[:topN]
	}
	return items
}

// countItemInt 整型键计数统计项（如活跃小时直方图）。
type countItemInt struct {
	key   int
	count int
}

// topNByCountInt 取整型键计数的 topN（频次降序，同频按键升序）。
func topNByCountInt(m map[int]int, topN int) []countItemInt {
	items := make([]countItemInt, 0, len(m))
	for k, c := range m {
		items = append(items, countItemInt{key: k, count: c})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if len(items) > topN {
		items = items[:topN]
	}
	return items
}

// countItem 计数统计项。
type countItem struct {
	key   string
	count int
}

// topNByCount 取计数 topN（频次降序，同频按字典序），用于市场/板块/评级等分布统计。
func topNByCount(m map[string]int, topN int) []countItem {
	items := make([]countItem, 0, len(m))
	for k, c := range m {
		items = append(items, countItem{key: k, count: c})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if len(items) > topN {
		items = items[:topN]
	}
	return items
}

// clampRunes 按字符数截断字符串，超长追加省略号。
func clampRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// pct 计算占比（分母为 0 时返回 0）。
func pct(num, den int) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den) * 100
}

// avgHoldingDays 估算平均持仓周期：按股票代码将买入与其后最近一次卖出配对，
// 取时间差的平均值（天）。无法配对时返回 0。
func avgHoldingDays(records []data.TradingRecord) float64 {
	// records 按时间倒序，转为正序便于配对
	ordered := make([]data.TradingRecord, len(records))
	for i, r := range records {
		ordered[len(records)-1-i] = r
	}
	pending := map[string]time.Time{}
	var durations []float64
	for _, r := range ordered {
		switch r.Direction {
		case "买入":
			pending[r.StockCode] = r.TradingTime
		case "卖出":
			if t, ok := pending[r.StockCode]; ok {
				d := r.TradingTime.Sub(t).Hours() / 24
				if d >= 0 {
					durations = append(durations, d)
				}
				delete(pending, r.StockCode)
			}
		}
	}
	if len(durations) == 0 {
		return 0
	}
	var sum float64
	for _, d := range durations {
		sum += d
	}
	return sum / float64(len(durations))
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

// profileTemplate LLM 生成画像的固定输出模板（限制 ≤40 行，控制 token）。
const profileTemplate = `你是用户画像分析师。根据给定的用户行为数据（关注列表、股票分组、交易记录及统计、对话历史、用户反馈），生成一份精简的用户画像，输出为固定格式 Markdown，不超过 40 行。只输出画像本身，不要解释。

格式：
## 用户画像
- 关注市场：A股/港股/美股等，附各市场占比
- 关注板块：<从股票分组名与持仓/提问推断，如科技/新能源/医药>
- 关注标的：<主要关注的股票名称/代码，最多5个>
- 持仓与成本：<如有，简述>
- 风险偏好：<从止损比例推断：保守/稳健/激进，附平均止损%>
- 交易习惯：<从交易记录与统计推断：频率/持仓周期/止损止盈纪律/买卖风格，尽量具体>
- 常用分析维度：<从提问与记录推断，如基本面/技术面/资金流/消息面>
- 提问模式：<从高频提问与最近提问推断：经常问的问题（尽量点名）、问题类型、关注点、表达习惯；结合 AI 分析报告频率与 AI 推荐记录>
- 偏好格式：<从反馈推断，如表格/简答/详细报告；无则写"未明确">
- 操作习惯：<从报警设置/定时盯盘/活跃时段/提问频率推断；无则写"未明确">
- 需规避项：<从"没用"反馈与交易理由/心态推断；无则写"无">

若某维度无法判断，写"未明确"。不要编造数据。

输出要求（必须严格遵守，否则结果会被丢弃）：
1. 每行严格使用格式"- 字段名：值"：半角横线+空格开头，冒号用中文全角"："，字段名与上面完全一致。
2. 不输出任何其他标题、表格、代码块或解释文字。
3. 每个字段一行，共11行，"## 用户画像"标题行之后紧跟。`

// resolveProfileLearnAIConfigID 解析画像学习用的 AI 配置 ID。
// 优先级：设置项 ProfileLearnAiConfigId（用户指定的"画像学习模型"）>
// 自动模式（resolveChatAIConfigID，第一个可用对话模型）。
// 指定的配置不可用（不存在/缺 Key/Url/非对话模型）时回退自动模式。
func resolveProfileLearnAIConfigID() uint {
	settingConfig := data.GetSettingConfig()
	if settingConfig != nil && settingConfig.ProfileLearnAiConfigId > 0 {
		for _, cfg := range settingConfig.AiConfigs {
			if cfg != nil && int(cfg.ID) == settingConfig.ProfileLearnAiConfigId &&
				cfg.ApiKey != "" && cfg.BaseUrl != "" &&
				(cfg.ModelType == "" || cfg.ModelType == "chat") {
				return cfg.ID
			}
		}
		// 指定的配置失效（被删除/改为向量模型/缺 Key），回退自动模式
	}
	return resolveChatAIConfigID()
}

// buildProfileWithLLM 用 LLM 生成画像。existing 非空时做增量调和（以现有画像为基准）。
// 失败返回 error（调用方回退规则模板）。
func (u *UserProfileLearner) buildProfileWithLLM(dataText, existing string) (string, error) {
	cfgID := resolveProfileLearnAIConfigID()
	if cfgID == 0 {
		return "", fmt.Errorf("未配置可用的对话 AI 服务")
	}
	prompt := profileTemplate
	if strings.TrimSpace(existing) != "" {
		// 增量调和（参考 Mem0 ADD/UPDATE/DELETE/NOOP 四操作）：
		// 现有画像中被新数据继续支持的事实保留（NOOP/确认），发生变化的更新（UPDATE），
		// 被新数据推翻的删除（DELETE），新出现的事实补充（ADD）。避免稳定字段翻转。
		prompt += "\n\n【现有画像】（以此为基准做增量调和，而非从零重写）\n" + strings.TrimSpace(existing) +
			"\n\n调和规则：\n" +
			"1. 现有画像中仍被新数据支持或未被推翻的字段，原样保留（措辞可微调使其更准确）。\n" +
			"2. 新数据与现有画像冲突时（如关注标的已调仓、风险偏好已变化），以新数据为准更新。\n" +
			"3. 明确被新数据推翻的旧事实删除，不要保留过时信息。\n" +
			"4. 新数据揭示的新事实补充进对应字段。\n" +
			"5. 输出仍为完整的 11 行固定格式画像（不是差异补丁）。"
	}
	out, err := callLLMSync(cfgID, prompt, dataText)
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
		"关注板块":   true,
		"关注标的":   true,
		"持仓与成本":  true,
		"风险偏好":   true,
		"交易习惯":   true,
		"常用分析维度": true,
		"提问模式":   true,
		"偏好格式":   true,
		"操作习惯":   true,
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

	labels := userProfileFieldLabels
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
	// 简化：提取关注市场、分组与标的名，其余标注"未明确"
	var markets, names []string
	var lines []string
	var stopPcts []float64
	holdingCount := 0
	seenName := map[string]bool{}
	seenMarket := map[string]bool{}
	groups := ""
	for _, line := range strings.Split(dataText, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "【股票分组】") {
			groups = strings.TrimSpace(strings.TrimPrefix(line, "【股票分组】"))
			continue
		}
		// 市场分布行："【关注列表】共X只；市场分布:A股200只 美股10只；..."
		if strings.Contains(line, "市场分布:") {
			for _, m := range profileMarketCountRe.FindAllStringSubmatch(line, -1) {
				if !seenMarket[m[1]] {
					seenMarket[m[1]] = true
					markets = append(markets, fmt.Sprintf("%s(%s只)", m[1], m[2]))
				}
			}
		}
		// 持仓明细行："- 名称(代码) 成本=x 止损≈y%"
		if strings.HasPrefix(line, "- ") && strings.Contains(line, "成本=") {
			holdingCount++
			if m := profileNameCodeRe.FindStringSubmatch(line); m != nil && !seenName[m[1]] {
				seenName[m[1]] = true
				names = append(names, m[1])
			}
		}
		for _, m := range profileStopPctRe.FindAllStringSubmatch(line, -1) {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				stopPcts = append(stopPcts, v)
			}
		}
		// 标的清单行："- A股: 名称、名称、…"
		if strings.HasPrefix(line, "- ") && strings.Contains(line, ": ") {
			body := strings.TrimPrefix(line, "- ")
			if idx := strings.Index(body, ": "); idx > 0 {
				for _, n := range strings.Split(body[idx+2:], "、") {
					n = strings.TrimSpace(n)
					if n == "" || strings.Contains(n, "…") || seenName[n] {
						continue
					}
					seenName[n] = true
					names = append(names, n)
				}
			}
		}
	}
	if len(markets) > 0 {
		lines = append(lines, "- 关注市场："+strings.Join(markets, "/"))
	} else {
		lines = append(lines, "- 关注市场：未明确")
	}
	if groups != "" {
		lines = append(lines, "- 关注板块："+groups)
	} else {
		lines = append(lines, "- 关注板块：未明确")
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
	if holdingCount > 0 {
		lines = append(lines, fmt.Sprintf("- 持仓与成本：%d只设置成本/止损记录（详见数据快照）", holdingCount))
	} else {
		lines = append(lines, "- 持仓与成本：未明确")
	}
	if len(stopPcts) > 0 {
		var sum float64
		for _, v := range stopPcts {
			sum += v
		}
		avg := sum / float64(len(stopPcts))
		label := "稳健"
		if avg >= 10 {
			label = "激进"
		} else if avg <= 5 {
			label = "保守"
		}
		lines = append(lines, fmt.Sprintf("- 风险偏好：%s，平均止损≈%.1f%%", label, avg))
	} else {
		lines = append(lines, "- 风险偏好：未明确")
	}
	lines = append(lines,
		"- 交易习惯：未明确",
		"- 常用分析维度：未明确",
		"- 提问模式：未明确",
		"- 偏好格式：未明确",
		"- 操作习惯：未明确",
		"- 需规避项：无")
	return "## 用户画像\n" + strings.Join(lines, "\n")
}

// 规则回退解析用的正则（RE2 语法）。
var (
	// 匹配 "A股12只" 形式的市场分布计数
	profileMarketCountRe = regexp.MustCompile(`(A股|港股|美股|指数)(\d+)只`)
	// 匹配 "止损≈6.5%" 形式的止损比例
	profileStopPctRe = regexp.MustCompile(`止损≈([0-9.]+)%`)
	// 匹配持仓明细行的 "名称(代码)" 前缀
	profileNameCodeRe = regexp.MustCompile(`([^\s(（]+)[（(][a-zA-Z0-9.]+[）)]`)
)

// userProfileFieldLabels 画像固定字段（顺序即输出顺序）。
// learner / api / 前端展示共用，新增字段时需同步 user-profile.vue。
var userProfileFieldLabels = []string{
	"关注市场", "关注板块", "关注标的", "持仓与成本", "风险偏好", "交易习惯",
	"常用分析维度", "提问模式", "偏好格式", "操作习惯", "需规避项",
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
