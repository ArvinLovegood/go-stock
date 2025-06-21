package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Holiday 表示节假日信息
type Holiday struct {
	Holiday bool   `json:"holiday"`
	Name    string `json:"name"`
	Date    string `json:"date"`
}

// HolidayResponse 表示节假日API响应结构
type HolidayResponse struct {
	Code    int                `json:"code"`
	Holiday map[string]Holiday `json:"holiday"`
}

// GetHolidayData 获取指定年份的节假日数据，优先从本地文件读取，不存在则从API获取并保存
func GetHolidayData(year int) (holidayResponse HolidayResponse, err error) {
	filename := fmt.Sprintf("%d-holiday.json", year)
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return holidayResponse, fmt.Errorf("无法获取当前文件路径")
	}
	fileDir := filepath.Dir(currentFile)
	projectRoot := filepath.Dir(filepath.Dir(fileDir)) // 从backend/data上两级到项目根目录
	filePath := filepath.Join(projectRoot, filename)

	// 检查本地文件是否存在
	if _, err := os.Stat(filePath); err == nil {
		// 读取本地文件
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return holidayResponse, err
		}
		if err := json.Unmarshal(fileContent, &holidayResponse); err != nil {
			logger.SugaredLogger.Errorf("解析节假日数据失败: %v", err)
			return holidayResponse, fmt.Errorf("解析节假日数据失败: %v", err)
		}
		return holidayResponse, nil
	} else {
		return holidayResponse, fmt.Errorf("本地节假日数据文件不存在: %s", filePath)
	}
}

// IsHoliday 判断指定日期是否为节假日
func IsHoliday(date time.Time) (bool, error) {
	holidayData, err := GetHolidayData(date.Year())
	if err != nil {
		return false, err
	}

	dateStr := fmt.Sprintf("%02d-%02d", date.Month(), date.Day())
	holiday, exists := holidayData.Holiday[dateStr]
	if exists && holiday.Holiday {
		return true, nil
	}

	return false, nil
}
