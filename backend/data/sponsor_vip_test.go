package data

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/cryptor"
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
			name: "生效时间在未来不影响放行（只校验到期时间）",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026-11-01 00:00:00",
				"vipAuthTime": "2026-01-01 00:00:00", "vipEndTime": "2028-03-25 00:00:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "生效时间字段非法不影响放行（仅展示用）",
			info: map[string]any{
				"vipLevel": 2, "vipStartTime": "2026年09月01日",
				"vipEndTime": "2028-03-25 00:00:00",
			},
			wantLevel: 2, wantActive: true,
		},
		{
			name: "非东八区机器解析发码时刻也不误判：试用码发码即生效",
			info: map[string]any{
				// 服务端按东八区写入，时区为 UTC 的机器按 time.Local 解析会把该时刻算到未来
				"vipLevel": 2, "vipStartTime": "2026-10-09 12:09:53",
				"vipAuthTime": "2026-10-09 12:09:53", "vipEndTime": "2026-10-16 12:09:53",
			},
			wantLevel: 2, wantActive: true,
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

// 复制赞助码时不同平台会带入不同的不可见字符（macOS 上尤其常见硬换行与零宽字符），
// 只要残留一个字符 hex 解码就会失败，表现为"同一个码 Windows 能过、mac 不能"。
func TestNormalizeSponsorCode(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"纯 hex 不变", "deadbeef", "deadbeef"},
		{"硬换行", "dead\nbeef", "deadbeef"},
		{"Windows 换行", "dead\r\nbeef", "deadbeef"},
		{"制表符与空格", " dead\tbeef ", "deadbeef"},
		{"NBSP（U+00A0）", "dead\u00a0beef", "deadbeef"},
		{"零宽空格（U+200B）", "dead\u200bbeef", "deadbeef"},
		{"零宽连字（U+200D）", "dead\u200dbeef", "deadbeef"},
		{"BOM（U+FEFF）", "\ufeffdeadbeef", "deadbeef"},
		{"词连接符（U+2060）", "dead\u2060beef", "deadbeef"},
		{"全角空格（U+3000）", "dead\u3000beef", "deadbeef"},
		{"非法字符保留以便报格式错", "dead-beef", "dead-beef"},
		{"空字符串", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeSponsorCode(c.input); got != c.want {
				t.Fatalf("NormalizeSponsorCode(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// 校验失败必须分类明确：格式问题引导用户重新完整复制，密钥不匹配则是码本身无效。
func TestSafeDecryptSponsorCode(t *testing.T) {
	const keyHex = "00112233445566778899aabbccddeeff"
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatalf("测试密钥解码失败: %v", err)
	}
	plain := []byte(`{"user":"tester","vipLevel":2,"vipEndTime":"2028-03-25 00:00:00"}`)
	code := hex.EncodeToString(cryptor.AesEcbEncrypt(plain, key))

	raw, err := SafeDecryptSponsorCode(code, keyHex)
	if err != nil {
		t.Fatalf("正常赞助码解密失败: %v", err)
	}
	if string(raw) != string(plain) {
		t.Fatalf("解密内容 = %s, want %s", raw, plain)
	}

	// 带换行/空格的码也要能解：这是 mac 端"验证失败"最可疑的来源
	for _, dirty := range []string{
		" " + code + "\n",
		code[:10] + "\r\n" + code[10:],
		code[:20] + "\u200b" + code[20:],
		code[:30] + "\u00a0" + code[30:],
	} {
		if _, err := SafeDecryptSponsorCode(dirty, keyHex); err != nil {
			t.Fatalf("归一化后应能解密, dirty=%q err=%v", dirty, err)
		}
	}

	t.Run("空码", func(t *testing.T) {
		if _, err := SafeDecryptSponsorCode("  \n\t ", keyHex); !errors.Is(err, ErrSponsorCodeEmpty) {
			t.Fatalf("err = %v, want ErrSponsorCodeEmpty", err)
		}
	})
	t.Run("非 hex 字符", func(t *testing.T) {
		if _, err := SafeDecryptSponsorCode("zz"+code, keyHex); !errors.Is(err, ErrSponsorCodeFormat) {
			t.Fatalf("err = %v, want ErrSponsorCodeFormat", err)
		}
	})
	t.Run("密文长度非法", func(t *testing.T) {
		if _, err := SafeDecryptSponsorCode("deadbeef", keyHex); !errors.Is(err, ErrSponsorCodeFormat) {
			t.Fatalf("err = %v, want ErrSponsorCodeFormat", err)
		}
	})
	t.Run("密钥不匹配", func(t *testing.T) {
		_, err := SafeDecryptSponsorCode(code, "ffffffffffffffffffffffffffffffff")
		if !errors.Is(err, ErrSponsorCodeDecrypt) {
			t.Fatalf("err = %v, want ErrSponsorCodeDecrypt", err)
		}
	})
	t.Run("密钥长度非法", func(t *testing.T) {
		if _, err := SafeDecryptSponsorCode(code, "00112233"); err == nil {
			t.Fatal("密钥长度非法时应当报错")
		}
	})
}

func TestSponsorCodeV2SignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试密钥对失败: %v", err)
	}
	pubHex := hex.EncodeToString(pub)
	plain := []byte(`{"user":"tester","vipLevel":2,"vipEndTime":"2028-03-25 00:00:00"}`)

	code, err := SignSponsorCodeV2(plain, priv)
	if err != nil {
		t.Fatalf("SignSponsorCodeV2 失败: %v", err)
	}
	if !strings.HasPrefix(code, SponsorCodeV2Prefix) {
		t.Fatalf("期望前缀 %q, got %q", SponsorCodeV2Prefix, code)
	}

	raw, err := VerifySponsorCodeV2(code, pubHex)
	if err != nil {
		t.Fatalf("VerifySponsorCodeV2 失败: %v", err)
	}
	if string(raw) != string(plain) {
		t.Fatalf("payload = %s, want %s", raw, plain)
	}

	// 带换行/零宽字符时也能归一化验签
	dirty := " \n" + code[:12] + "\u200b" + code[12:] + "\r\n"
	if _, err := VerifySponsorCodeV2(dirty, pubHex); err != nil {
		t.Fatalf("归一化后应验签成功, err=%v", err)
	}

	t.Run("篡改payload验签失败", func(t *testing.T) {
		parts := strings.Split(code, ".")
		// 修改 payload 中任意字符
		tampered := parts[0] + ".eyJ1c2VyIjoiaGFja2VyIiwidmlwTGV2ZWwiOjJ9." + parts[2]
		if _, err := VerifySponsorCodeV2(tampered, pubHex); !errors.Is(err, ErrSponsorCodeDecrypt) {
			t.Fatalf("err = %v, want ErrSponsorCodeDecrypt", err)
		}
	})

	t.Run("错误公钥验签失败", func(t *testing.T) {
		otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
		if _, err := VerifySponsorCodeV2(code, hex.EncodeToString(otherPub)); !errors.Is(err, ErrSponsorCodeDecrypt) {
			t.Fatalf("err = %v, want ErrSponsorCodeDecrypt", err)
		}
	})

	t.Run("DecodeSponsorCode统一分流v2与v1", func(t *testing.T) {
		origPub := SponsorVerifyPubKeyHex
		SponsorVerifyPubKeyHex = pubHex
		t.Cleanup(func() { SponsorVerifyPubKeyHex = origPub })

		gotV2, err := DecodeSponsorCode(code, "")
		if err != nil || string(gotV2) != string(plain) {
			t.Fatalf("v2 分流失败: raw=%s err=%v", gotV2, err)
		}

		// v1 老码兼容
		const aesHex = "00112233445566778899aabbccddeeff"
		aesKey, _ := hex.DecodeString(aesHex)
		v1Code := hex.EncodeToString(cryptor.AesEcbEncrypt(plain, aesKey))
		gotV1, err := DecodeSponsorCode(v1Code, aesHex)
		if err != nil || string(gotV1) != string(plain) {
			t.Fatalf("v1 兼容分流失败: raw=%s err=%v", gotV1, err)
		}
	})
}

