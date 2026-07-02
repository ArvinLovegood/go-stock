package data

import (
	"go-stock/backend/db"
	"gorm.io/gorm"
)

// @Author spark
// @Date 2025/4/3 11:18
// @Desc
// -----------------------------------------------------------------------------------
type Group struct {
	gorm.Model
	Name string `json:"name" gorm:"index"`
	Sort int    `json:"sort"`
}

func (Group) TableName() string {
	return "stock_groups"
}

type GroupStock struct {
	gorm.Model
	StockCode string `json:"stockCode" gorm:"index"`
	GroupId   int    `json:"groupId" gorm:"index"`
	GroupInfo Group  `json:"groupInfo" gorm:"foreignKey:GroupId;references:ID"`
}

func (GroupStock) TableName() string {
	return "group_stock_info"
}

type StockGroupApi struct {
	dao *gorm.DB
}

func NewStockGroupApi(dao *gorm.DB) *StockGroupApi {
	return &StockGroupApi{dao: db.Dao}
}

func (receiver StockGroupApi) AddGroup(group Group) bool {
	// 检查是否已存在相同sort的组
	var existingGroup Group
	err := receiver.dao.Where("sort = ?", group.Sort).First(&existingGroup).Error

	// 如果存在相同sort的组，则将该组及之后的所有组向后移动一位
	if err == nil {
		// 处理sort冲突：将相同sort值及之后的所有组向后移动一位
		receiver.dao.Model(&Group{}).Where("sort >= ?", group.Sort).Update("sort", gorm.Expr("sort + ?", 1))
	}

	// 创建新组
	err = receiver.dao.Create(&group).Error
	return err == nil
}
func (receiver StockGroupApi) GetGroupList() []Group {
	var groups []Group
	receiver.dao.Order("sort ASC").Find(&groups)
	return groups
}
func (receiver StockGroupApi) UpdateGroupSort(id int, newSort int) bool {
	// First, get the current group to check if it exists
	var currentGroup Group
	if err := receiver.dao.First(&currentGroup, id).Error; err != nil {
		return false
	}

	// If the new sort is the same as current, no need to update
	if currentGroup.Sort == newSort {
		return true
	}

	// Get all groups ordered by sort
	var allGroups []Group
	receiver.dao.Order("sort ASC").Find(&allGroups)

	// Adjust sort numbers to make space for the new sort value
	if newSort > currentGroup.Sort {
		// Moving down: decrease sort of groups between old and new position
		receiver.dao.Model(&Group{}).Where("sort > ? AND sort <= ? AND id != ?", currentGroup.Sort, newSort, id).Update("sort", gorm.Expr("sort - ?", 1))
	} else {
		// Moving up: increase sort of groups between new and old position
		receiver.dao.Model(&Group{}).Where("sort >= ? AND sort < ? AND id != ?", newSort, currentGroup.Sort, id).Update("sort", gorm.Expr("sort + ?", 1))
	}

	// Update the target group's sort
	err := receiver.dao.Model(&Group{}).Where("id = ?", id).Update("sort", newSort).Error
	return err == nil
}

// InitializeGroupSort initializes sort order for all groups based on created time
func (receiver StockGroupApi) InitializeGroupSort() bool {
	// Get all groups ordered by created time
	var groups []Group
	err := receiver.dao.Order("created_at ASC").Find(&groups).Error
	if err != nil {
		return false
	}

	// Update each group with new sort value based on their position
	for i, group := range groups {
		newSort := i + 1
		err := receiver.dao.Model(&Group{}).Where("id = ?", group.ID).Update("sort", newSort).Error
		if err != nil {
			return false
		}
	}
	return true
}
func (receiver StockGroupApi) GetGroupStockByGroupId(groupId int) []GroupStock {
	var stockGroup []GroupStock
	receiver.dao.Preload("GroupInfo").Where("group_id = ?", groupId).Find(&stockGroup)
	return stockGroup
}

// GetAllGroupStocks 一次返回全部分组-股票归属记录（预加载 GroupInfo）。
// 用于前端在「全部」标签页表格中渲染每只股票所属的分组，避免 N+1 查询。
func (receiver StockGroupApi) GetAllGroupStocks() []GroupStock {
	var stockGroup []GroupStock
	receiver.dao.Preload("GroupInfo").Find(&stockGroup)
	return stockGroup
}

func (receiver StockGroupApi) AddStockGroup(groupId int, stockCode string) bool {
	err := receiver.dao.Where("group_id = ? and stock_code = ?", groupId, stockCode).FirstOrCreate(&GroupStock{
		GroupId:   groupId,
		StockCode: stockCode,
	}).Updates(&GroupStock{
		GroupId:   groupId,
		StockCode: stockCode,
	}).Error
	return err == nil
}

func (receiver StockGroupApi) RemoveStockGroup(code string, name string, id int) bool {
	err := receiver.dao.Where("group_id = ? and stock_code = ?", id, code).Delete(&GroupStock{}).Error
	return err == nil
}

func (receiver StockGroupApi) RemoveGroup(id int) bool {
	err := receiver.dao.Where("id = ?", id).Delete(&Group{}).Error
	err = receiver.dao.Where("group_id = ?", id).Delete(&GroupStock{}).Error
	return err == nil

}

// UpdateGroup 仅更新分组名称，sort 保持不变
func (receiver StockGroupApi) UpdateGroup(id int, name string) bool {
	if name == "" {
		return false
	}
	err := receiver.dao.Model(&Group{}).Where("id = ?", id).Update("name", name).Error
	return err == nil
}
