package data

import (
	"go-stock/backend/db"
	"strings"

	"gorm.io/gorm"
)

// @Author spark
// @Date 2026/7/10
// @Desc 概念标签（与分组系统完全独立，不产生页签）
// -----------------------------------------------------------------------------------

// Concept 概念标签（与分组系统完全独立，不产生页签）
type Concept struct {
	gorm.Model
	Name string `json:"name" gorm:"uniqueIndex"` // DB 级唯一索引（并发兜底）
	Sort int    `json:"sort"`
}

func (Concept) TableName() string {
	return "stock_concepts"
}

// ConceptStock 概念-股票归属关系（多对多）
type ConceptStock struct {
	gorm.Model
	StockCode   string  `json:"stockCode" gorm:"index"`
	ConceptId   int     `json:"conceptId" gorm:"index"`
	ConceptInfo Concept `json:"conceptInfo" gorm:"foreignKey:ConceptId;references:ID"`
}

func (ConceptStock) TableName() string {
	return "stock_concept_relation"
}

type StockConceptApi struct {
	dao *gorm.DB
}

func NewStockConceptApi(dao *gorm.DB) *StockConceptApi {
	return &StockConceptApi{dao: db.Dao}
}

// AddConcept 新建概念标签，名称忽略大小写、首尾空格去重（已存在则幂等返回 true）
func (receiver StockConceptApi) AddConcept(concept Concept) bool {
	name := strings.TrimSpace(concept.Name)
	if name == "" {
		return false
	}
	// 去重：忽略大小写、首尾空格。若已存在同名概念，幂等返回 true（前端据此复用已存在概念）
	var existing Concept
	err := receiver.dao.Where("lower(name) = ?", strings.ToLower(name)).First(&existing).Error
	if err == nil {
		// 同名概念已存在，幂等返回 true
		return true
	}
	// 不存在则创建（concept.Name 已规范化为 trim 后的值）
	concept.Name = name
	if err := receiver.dao.Create(&concept).Error; err != nil {
		// 并发场景下可能因 uniqueIndex 冲突，再次查找已存在记录
		var again Concept
		if e := receiver.dao.Where("lower(name) = ?", strings.ToLower(name)).First(&again).Error; e == nil && again.ID > 0 {
			return true
		}
		return false
	}
	return true
}

func (receiver StockConceptApi) GetConceptList() []Concept {
	var concepts []Concept
	receiver.dao.Order("sort ASC, id ASC").Find(&concepts)
	return concepts
}

// UpdateConcept 修改概念名称，重名（忽略大小写）拒绝
func (receiver StockConceptApi) UpdateConcept(id int, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	// 检查目标名称是否已被其它概念占用（忽略大小写）
	var dup Concept
	if err := receiver.dao.Where("lower(name) = ? AND id <> ?", strings.ToLower(name), id).First(&dup).Error; err == nil {
		return false // 重名，拒绝
	}
	err := receiver.dao.Model(&Concept{}).Where("id = ?", id).Update("name", name).Error
	return err == nil
}

// RemoveConcept 删除概念标签 + 级联删除归属关系
func (receiver StockConceptApi) RemoveConcept(id int) bool {
	err := receiver.dao.Where("id = ?", id).Delete(&Concept{}).Error
	err = receiver.dao.Where("concept_id = ?", id).Delete(&ConceptStock{}).Error
	return err == nil
}

// AddStockConcept 把股票加入概念（已存在则幂等）
func (receiver StockConceptApi) AddStockConcept(conceptId int, stockCode string) bool {
	err := receiver.dao.Where("concept_id = ? and stock_code = ?", conceptId, stockCode).
		FirstOrCreate(&ConceptStock{ConceptId: conceptId, StockCode: stockCode}).
		Updates(&ConceptStock{ConceptId: conceptId, StockCode: stockCode}).Error
	return err == nil
}

// RemoveStockConcept 把股票移出概念（name 参数仅保持与 RemoveStockGroup 签名一致，不使用）
func (receiver StockConceptApi) RemoveStockConcept(code string, name string, id int) bool {
	err := receiver.dao.Where("concept_id = ? and stock_code = ?", id, code).Delete(&ConceptStock{}).Error
	return err == nil
}

// GetAllStockConcepts 一次返回全部概念-股票归属记录（预加载 ConceptInfo）。
// 用于前端「全部」标签页表格渲染概念列，避免 N+1 查询。
func (receiver StockConceptApi) GetAllStockConcepts() []ConceptStock {
	var list []ConceptStock
	receiver.dao.Preload("ConceptInfo").Find(&list)
	return list
}

// GetStockConceptsByStockCode 查询单只股票所属的概念
func (receiver StockConceptApi) GetStockConceptsByStockCode(stockCode string) []ConceptStock {
	var list []ConceptStock
	receiver.dao.Preload("ConceptInfo").Where("stock_code = ?", stockCode).Find(&list)
	return list
}
