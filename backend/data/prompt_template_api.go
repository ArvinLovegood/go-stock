package data

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

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
			Version: 1,
		}).Error
		if err != nil {
			return "添加失败"
		} else {
			return "添加成功"
		}
	} else {
		// 版本号：内容变更才自增（存量记录 Version 可能为 0，视为 1 起算）。
		// 用 map 更新以避免 gorm 忽略零值（struct updates 会跳过空串/0）。
		version := tmp.Version
		if version <= 0 {
			version = 1
		}
		updates := map[string]any{"version": version}
		if template.Content != tmp.Content {
			updates["version"] = version + 1
			updates["content"] = template.Content
		}
		if template.Name != "" {
			updates["name"] = template.Name
		}
		if template.Type != "" {
			updates["type"] = template.Type
		}
		err := db.Dao.Model(&models.PromptTemplate{}).Where("id=?", template.ID).Updates(updates).Error
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

// GetPromptTemplateByIDWithVersion 按 ID 读取模板内容与版本号，供推荐/回测按提示词版本归因。
func (t PromptTemplateApi) GetPromptTemplateByIDWithVersion(id int) (string, int) {
	prompt := &models.PromptTemplate{}
	db.Dao.Model(&models.PromptTemplate{}).Where("id=?", id).First(prompt)
	return prompt.Content, prompt.Version
}

// ShortPromptHash 计算策略提示词的稳定短哈希（SHA-256 前 16 位十六进制）。
// 计算前对空白做归一化，避免无关空白差异把同一提示词判为不同版本；空串返回空。
func ShortPromptHash(s string) string {
	norm := strings.Join(strings.Fields(s), " ")
	if norm == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:8])
}

func NewPromptTemplateApi() *PromptTemplateApi {
	return &PromptTemplateApi{}
}
