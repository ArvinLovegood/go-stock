package data

import (
	"fmt"
	"testing"
	"time"
)

func TestIsHoliday(t *testing.T) {
	tests := []struct {
		name     string
		date     string
		expected bool
	}{
		{
			name:     "2025-01-01元旦",
			date:     "2025-01-01",
			expected: true,
		},
		{
			name:     "2025-01-28除夕",
			date:     "2025-01-28",
			expected: true,
		},
		{
			name:     "2025-02-10普通工作日",
			date:     "2025-02-10",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tt.date)
			if err != nil {
				t.Fatalf("解析日期失败: %v", err)
			}

			result, err := IsHoliday(date)
			if err != nil {
				t.Errorf("IsHoliday返回错误: %v", err)
			}

			if result != tt.expected {
				t.Errorf("测试用例 %s 失败: 期望 %v, 实际 %v", tt.name, tt.expected, result)
			}

			fmt.Printf("日期: %v 节假日: %v\n", date, result)
		})
	}
}
