package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

type PromptTemplateApi struct {
}

func (t PromptTemplateApi) GetPromptTemplates(name string, promptType string) *[]models.PromptTemplate {
	var result []models.PromptTemplate
	if name != "" && promptType != "" {
		db.Dao.Model(&models.PromptTemplate{}).Where("name LIKE ? and type=?", "%"+name+"%", promptType).Find(&result)
	}
	if name != "" && promptType == "" {
		db.Dao.Model(&models.PromptTemplate{}).Where("name LIKE ?", "%"+name+"%").Find(&result)
	}
	if name == "" && promptType != "" {
		db.Dao.Model(&models.PromptTemplate{}).Where("type=?", promptType).Find(&result)
	}
	if name == "" && promptType == "" {
		db.Dao.Model(&models.PromptTemplate{}).Find(&result)
	}

	return &result
}

// GetPromptTemplateList 分页查询PromptTemplate记录
func (t PromptTemplateApi) GetPromptTemplateList(query *models.PromptTemplateQuery) (*models.PromptTemplatePageData, error) {
	var list []models.PromptTemplate
	var total int64

	q := db.Dao.Model(&models.PromptTemplate{})

	// 构建查询条件
	if query.Name != "" {
		q = q.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Type != "" {
		q = q.Where("type LIKE ?", "%"+query.Type+"%")
	}
	if query.Content != "" {
		q = q.Where("content LIKE ?", "%"+query.Content+"%")
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.PromptTemplatePageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (t PromptTemplateApi) AddPrompt(template models.PromptTemplate) string {
	var tmp models.PromptTemplate
	db.Dao.Model(&models.PromptTemplate{}).Where("id=?", template.ID).First(&tmp)
	if tmp.ID == 0 {
		err := db.Dao.Model(&models.PromptTemplate{}).Create(&models.PromptTemplate{
			Content: template.Content,
			Name:    template.Name,
			Type:    template.Type,
		}).Error
		if err != nil {
			return "添加失败"
		} else {
			return "添加成功"
		}
	} else {
		err := db.Dao.Model(&models.PromptTemplate{}).Where("id=?", template.ID).Updates(template).Error
		if err != nil {
			return "更新失败"
		} else {
			return "更新成功"
		}
	}
}

func (t PromptTemplateApi) DelPrompt(Id uint) string {
	template := &models.PromptTemplate{}
	db.Dao.Model(template).Where("id=?", Id).Find(template)
	if template.ID > 0 {
		err := db.Dao.Model(template).Delete(template).Error
		if err != nil {
			return "删除失败"
		} else {
			return "删除成功"
		}
	}
	return "模板信息不存在"
}

func (t PromptTemplateApi) GetPromptTemplateByID(id int) string {
	prompt := &models.PromptTemplate{}
	db.Dao.Model(&models.PromptTemplate{}).Where("id=?", id).First(prompt)
	logger.SugaredLogger.Infof("GetPromptTemplateByID:%d %s", id, prompt.Content)
	return prompt.Content
}
func NewPromptTemplateApi() *PromptTemplateApi {
	return &PromptTemplateApi{}
}
