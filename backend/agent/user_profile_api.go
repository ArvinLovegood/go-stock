package agent

import "strings"

// UserProfileSnapshot 返回画像页面所需的统一状态，避免前端多次 IPC 调用产生状态不同步。
type UserProfileSnapshot struct {
	Content     string `json:"content"`
	Enabled     bool   `json:"enabled"`
	UpdatedAt   string `json:"updatedAt"`
	KnownFields int    `json:"knownFields"`
	TotalFields int    `json:"totalFields"`
}

// user_profile_api.go — 用户画像 Wails API（P1）。
//
// 遵循 KnowledgeBaseApi / CronTaskApi 薄包装模式，把 UserProfileLearner 的能力
// 暴露给前端"Agent 对我的了解"页面：预览 / 覆盖 / 重新学习 / 清空。
//
// 用法（app.go 中）：
//
//	agent.NewUserProfileApi().GetUserProfile()
//
// 前端 wailsjs 绑定：
//	GetUserProfile / SaveUserProfile / RelearnUserProfile / ClearUserProfile

// UserProfileApi 用户画像 API（Wails 绑定）
type UserProfileApi struct{}

// NewUserProfileApi 构造用户画像 API 实例
func NewUserProfileApi() *UserProfileApi {
	return &UserProfileApi{}
}

// GetUserProfile 读取当前画像内容（未生成返回空）。
func (a *UserProfileApi) GetUserProfile() string {
	return NewUserProfileLearner().Get()
}

func (a *UserProfileApi) GetUserProfileUpdatedAt() string {
	return NewUserProfileLearner().UpdatedAt()
}

// GetUserProfileSnapshot 一次读取画像内容、启用状态、更新时间和完整度。
func (a *UserProfileApi) GetUserProfileSnapshot() *UserProfileSnapshot {
	content := NewUserProfileLearner().Get()
	total := 7
	known := 0
	for _, label := range []string{"关注市场", "关注标的", "持仓与成本", "风险偏好", "常用分析维度", "偏好格式", "需规避项"} {
		for _, line := range strings.Split(content, "\n") {
			prefix := "- " + label + "："
			if strings.HasPrefix(strings.TrimSpace(line), prefix) {
				value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix))
				if value != "" && value != "未明确" && value != "无" {
					known++
				}
				break
			}
		}
	}
	return &UserProfileSnapshot{Content: content, Enabled: IsUserProfileEnabled(), UpdatedAt: NewUserProfileLearner().UpdatedAt(), KnownFields: known, TotalFields: total}
}

// GetUserProfileEnabled 返回用户画像是否会注入 Agent。
func (a *UserProfileApi) GetUserProfileEnabled() bool {
	return IsUserProfileEnabled()
}

// SetUserProfileEnabled 设置用户画像是否注入 Agent。
func (a *UserProfileApi) SetUserProfileEnabled(enabled bool) error {
	return SetUserProfileEnabled(enabled)
}

// SaveUserProfile 手动覆盖画像。
func (a *UserProfileApi) SaveUserProfile(content string) error {
	return NewUserProfileLearner().Save(content)
}

// RelearnUserProfile 一键重新学习画像，返回新画像内容。
func (a *UserProfileApi) RelearnUserProfile() (string, error) {
	return NewUserProfileLearner().Relearn()
}

// ClearUserProfile 清空画像。
func (a *UserProfileApi) ClearUserProfile() error {
	return NewUserProfileLearner().Clear()
}
