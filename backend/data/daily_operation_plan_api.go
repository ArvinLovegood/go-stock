// Package data daily_operation_plan_api.go 每日操作计划
package data

import (
	"encoding/json"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

type DailyOperationPlanApi struct{}

func NewDailyOperationPlanApi() *DailyOperationPlanApi {
	return &DailyOperationPlanApi{}
}

// GetDailyOperationPlanList 分页查询每日操作计划
func (a *DailyOperationPlanApi) GetDailyOperationPlanList(query *models.DailyOperationPlanQuery) (*models.DailyOperationPlanPageData, error) {
	var list []models.DailyOperationPlan
	var total int64

	q := db.Dao.Model(&models.DailyOperationPlan{})

	if query.StockCode != "" {
		q = q.Where("stock_code LIKE ?", "%"+query.StockCode+"%")
	}
	if query.StockName != "" {
		q = q.Where("stock_name LIKE ?", "%"+query.StockName+"%")
	}
	if query.PlanDate != "" {
		q = q.Where("plan_date = ?", query.PlanDate)
	}
	if query.Status != "" {
		q = q.Where("status = ?", query.Status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("plan_date DESC, created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.DailyOperationPlanPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetDailyOperationPlanByID 根据ID获取操作计划
func (a *DailyOperationPlanApi) GetDailyOperationPlanByID(id uint) (*models.DailyOperationPlan, error) {
	var plan models.DailyOperationPlan
	if err := db.Dao.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// SaveDailyOperationPlan 新增或更新操作计划
func (a *DailyOperationPlanApi) SaveDailyOperationPlan(plan models.DailyOperationPlan) string {
	// 校验情景/纪律 JSON 字段格式
	if plan.Scenarios != "" {
		var scenarios []models.OperationScenario
		if err := json.Unmarshal([]byte(plan.Scenarios), &scenarios); err != nil {
			return "情景方案 JSON 格式错误:" + err.Error()
		}
	}
	if plan.Discipline != "" {
		var discipline []models.OperationDiscipline
		if err := json.Unmarshal([]byte(plan.Discipline), &discipline); err != nil {
			return "操作纪律 JSON 格式错误:" + err.Error()
		}
	}
	if plan.Status == "" {
		plan.Status = "pending"
	}

	if plan.ID == 0 {
		if err := db.Dao.Create(&plan).Error; err != nil {
			return "添加失败:" + err.Error()
		}
		return "添加成功"
	}
	var existing models.DailyOperationPlan
	db.Dao.Where("id = ?", plan.ID).First(&existing)
	if existing.ID == 0 {
		return "计划不存在"
	}
	if err := db.Dao.Model(&models.DailyOperationPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{
		"plan_date":        plan.PlanDate,
		"plan_end_date":    plan.PlanEndDate,
		"stock_code":       plan.StockCode,
		"stock_name":       plan.StockName,
		"overall_judgment": plan.OverallJudgment,
		"scenarios":        plan.Scenarios,
		"discipline":       plan.Discipline,
		"summary":          plan.Summary,
		"risk_warning":     plan.RiskWarning,
		"status":           plan.Status,
		"remarks":          plan.Remarks,
		"enable_alert":     plan.EnableAlert,
		"notify_channels":  plan.NotifyChannels,
	}).Error; err != nil {
		return "更新失败:" + err.Error()
	}
	return "更新成功"
}

// DeleteDailyOperationPlan 根据ID删除操作计划
func (a *DailyOperationPlanApi) DeleteDailyOperationPlan(id uint) string {
	var plan models.DailyOperationPlan
	db.Dao.Where("id = ?", id).First(&plan)
	if plan.ID == 0 {
		return "计划不存在"
	}
	if err := db.Dao.Delete(&plan).Error; err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// UpdateDailyOperationPlanStatus 更新操作计划状态
func (a *DailyOperationPlanApi) UpdateDailyOperationPlanStatus(id uint, status string) error {
	return db.Dao.Model(&models.DailyOperationPlan{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateDailyOperationPlanAlert 更新操作计划盘中预警开关
func (a *DailyOperationPlanApi) UpdateDailyOperationPlanAlert(id uint, enableAlert bool) error {
	return db.Dao.Model(&models.DailyOperationPlan{}).Where("id = ?", id).Update("enable_alert", enableAlert).Error
}
