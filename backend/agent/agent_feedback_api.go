package agent

// agent_feedback_api.go — Agent 反馈采集与统计 Wails API。
//
// 将用户对 Agent 回答的显式反馈（👍/👎 + 可选原因）入库，作为"懂用户"双向学习的
// 输入（P0）。反馈数据驱动 P1 的用户画像聚合器。
//
// 设计原则：
//   - 与 KnowledgeBaseApi / CronTaskApi 模式一致：薄包装层，业务简单直连 db
//   - 幂等：同一 session+question 已评则更新，避免重复记录
//   - 全部本地 SQLite 存储，不阻断主流程
//
// 用法（app.go 中）：
//
//	agent.NewAgentFeedbackApi().SubmitFeedback(fb)

import (
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/machineid"
	"go-stock/backend/models"
)

// AgentFeedbackApi 反馈 API（Wails 绑定）
type AgentFeedbackApi struct{}

// NewAgentFeedbackApi 构造反馈 API 实例
func NewAgentFeedbackApi() *AgentFeedbackApi {
	return &AgentFeedbackApi{}
}

// CurrentUserKey 构建当前用户标识（无账号体系约定）。
// 组合 machineID（机器维度）+ sessionID（会话维度），见方案文档 6.1。
func CurrentUserKey(sessionID string) string {
	mid := machineid.GetMachineId()
	if sessionID == "" {
		return "m:" + mid
	}
	return "m:" + mid + ":s:" + sessionID
}

// SubmitFeedback 提交一条反馈（幂等：同 session+question 已评则更新）。
// rating: 1=有用/up，-1=没用/down。
func (a *AgentFeedbackApi) SubmitFeedback(fb *models.AgentFeedback) error {
	if fb == nil {
		return fmt.Errorf("反馈数据不能为空")
	}
	if fb.Rating != 1 && fb.Rating != -1 {
		return fmt.Errorf("rating 必须为 1（有用）或 -1（没用）")
	}
	if fb.UserKey == "" {
		fb.UserKey = CurrentUserKey(fb.SessionID)
	}
	if fb.FeedbackAt.IsZero() {
		fb.FeedbackAt = time.Now()
	}

	// 幂等更新：同一 session + 同一问题已评则更新 rating/reason
	if fb.SessionID != "" && fb.Question != "" {
		var existing models.AgentFeedback
		err := db.Dao.Where("session_id = ? AND question = ? AND rating != 0",
			fb.SessionID, fb.Question).First(&existing).Error
		if err == nil {
			existing.Rating = fb.Rating
			existing.Reason = fb.Reason
			existing.FeedbackAt = time.Now()
			return db.Dao.Save(&existing).Error
		}
	}
	return db.Dao.Create(fb).Error
}

// FeedbackItem 反馈列表条目（含格式化时间，便于前端展示）
type FeedbackItem struct {
	models.AgentFeedback
	FeedbackAtStr string `json:"feedbackAtStr"`
}

// FeedbackPageData 反馈记录分页结果。
type FeedbackPageData struct {
	List  []FeedbackItem `json:"list"`
	Total int64          `json:"total"`
}

// ListFeedback 分页查询反馈记录。
func (a *AgentFeedbackApi) ListFeedback(page, pageSize int) (FeedbackPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := db.Dao.Model(&models.AgentFeedback{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return FeedbackPageData{}, err
	}
	var list []models.AgentFeedback
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("feedback_at DESC").Find(&list).Error; err != nil {
		return FeedbackPageData{}, err
	}
	items := make([]FeedbackItem, 0, len(list))
	for _, f := range list {
		items = append(items, FeedbackItem{
			AgentFeedback: f,
			FeedbackAtStr: f.FeedbackAt.Format("2006-01-02 15:04:05"),
		})
	}
	return FeedbackPageData{List: items, Total: total}, nil
}

// FeedbackStats 反馈聚合统计（前端"Agent 对我的了解"页展示）。
type FeedbackStats struct {
	Total     int `json:"total"`     // 总反馈数
	UpCount   int `json:"upCount"`   // 有用数
	DownCount int `json:"downCount"` // 没用数
	// UpRate 采纳率（%）保留一位小数
	UpRate float64 `json:"upRate"`
}

// FeedbackStats 返回反馈统计。
func (a *AgentFeedbackApi) FeedbackStats() (*FeedbackStats, error) {
	var total, up int64
	db.Dao.Model(&models.AgentFeedback{}).Count(&total)
	db.Dao.Model(&models.AgentFeedback{}).Where("rating = ?", 1).Count(&up)
	down := total - up
	rate := 0.0
	if total > 0 {
		rate = float64(up) / float64(total) * 100
	}
	return &FeedbackStats{
		Total:     int(total),
		UpCount:   int(up),
		DownCount: int(down),
		UpRate:    rate,
	}, nil
}

// DeleteFeedback 删除单条反馈。
func (a *AgentFeedbackApi) DeleteFeedback(id uint) error {
	return db.Dao.Where("id = ?", id).Delete(&models.AgentFeedback{}).Error
}

// ClearFeedback 清空所有反馈。
func (a *AgentFeedbackApi) ClearFeedback() error {
	return db.Dao.Where("1 = 1").Delete(&models.AgentFeedback{}).Error
}
