package data

import (
	"strings"
	"testing"
	"time"
)

// 判定基准时间：所有用例都相对它构造时间字段，避免随时间推移失效
var testNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.Local)

func TestEvaluateSponsorVip(t *testing.T) {
	cases := []struct {
		name       string
		info       map[string]any
		wantLevel  int
		wantActive bool
		wantReason string
	}{
		{
			name: "标准格式且未到期",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026-09-01 12:25:31",
				"vipAuthTime": "2026-09-01 12:25:31", "vipEndTime": "2028-03-25 00:00:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "等级为字符串同样识别",
			info: map[string]any{
				"vipLevel": "2", "vipStartTime": "2026-01-01 00:00:00",
				"vipAuthTime": "2026-01-01 00:00:00", "vipEndTime": "2027-01-01 00:00:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "老赞助码缺少生效/授权时间不拦截",
			info: map[string]any{
				"vipLevel": 2, "vipEndTime": "2027-01-01 00:00:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "RFC3339 与斜杠格式可识别",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026-01-01T00:00:00+08:00",
				"vipEndTime": "2027-01-01T00:00:00+08:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "时间戳格式可识别",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": testNow.AddDate(0, 0, -1).Unix(),
				"vipEndTime": testNow.AddDate(0, 0, 1).Unix(),
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "已到期",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2025-01-01 00:00:00",
				"vipAuthTime": "2025-01-01 00:00:00", "vipEndTime": "2026-01-01 00:00:00",
			},
			wantLevel: 2, wantReason: "已到期",
		},
		{
			name: "生效时间在未来",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026-11-01 00:00:00",
				"vipAuthTime": "2026-01-01 00:00:00", "vipEndTime": "2028-03-25 00:00:00",
			},
			wantLevel: 2, wantReason: "尚未生效",
		},
		{
			name: "生效时间字段非法不静默放行",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026年09月01日",
				"vipEndTime": "2028-03-25 00:00:00",
			},
			wantLevel: 2, wantReason: "无法识别",
		},
		{
			name: "缺少到期时间",
			info: map[string]any{"vipLevel": 2},
			// 缺失到期时间无法判定有效期，等级仍然上报
			wantLevel: 2, wantReason: "缺少到期时间",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := evaluateSponsorVip(c.info, testNow)
			if got.Level != c.wantLevel {
				t.Fatalf("level = %d, want %d", got.Level, c.wantLevel)
			}
			if got.Active != c.wantActive {
				t.Fatalf("active = %v, want %v (reason=%s)", got.Active, c.wantActive, got.Reason)
			}
			if c.wantReason != "" && !strings.Contains(got.Reason, c.wantReason) {
				t.Fatalf("reason = %q, want contains %q", got.Reason, c.wantReason)
			}
			if c.wantActive && got.Reason != "" {
				t.Fatalf("active 时 reason 应为空, got %q", got.Reason)
			}
		})
	}
}
