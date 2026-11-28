package data

import "testing"

func TestIsDomesticHost(t *testing.T) {
	cases := []struct {
		host     string
		domestic bool
	}{
		{"tokenhub.tencentmaas.com", true},
		{"api.lkeap.cloud.tencent.com", true},
		{"ark.cn-beijing.volces.com", true},
		{"dashscope.aliyuncs.com", true},
		{"api.deepseek.com", true},
		{"open.bigmodel.cn", true},
		{"api.moonshot.cn", true},
		{"spark-api-open.xf-yun.com", true},
		{"api.xiaomimimo.com", true},
		{"api-inference.modelscope.cn", true},
		{"localhost", true},
		{"127.0.0.1", true},
		{"192.168.1.10", true},
		{"10.0.0.2", true},
		{"myserver", true}, // 无点主机名视为内网
		{"api.openai.com", false},
		{"generativelanguage.googleapis.com", false},
		{"openrouter.ai", false},
		{"api.groq.com", false},
		{"aixlau.me", false},
		{"api.cheaperinference.com", false},
		{"example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsDomesticHost(c.host); got != c.domestic {
			t.Errorf("IsDomesticHost(%q) = %v, want %v", c.host, got, c.domestic)
		}
	}
}
