package data

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/cryptor"
	"go-stock/backend/logger"
)

// DefaultSponsorAESKeyHex 占位密钥，仅用于社区自行编译、未注入正式密钥的场景：
// 正式密钥必须通过构建期 ldflags（-X main.BuildKey=...）注入，不落在仓库里。
// 未注入时赞助码解密必然失败，因此启动日志会给出明确告警（见 main.checkDir）。
const DefaultSponsorAESKeyHex = "cc1e0d684e32f176c56ff1fcf384dcd9"

// SponsorDecryptKeyHex 由主程序在启动时同步为 ldflags 注入的 BuildKey；为空则使用 DefaultSponsorAESKeyHex。
var SponsorDecryptKeyHex string

// 赞助码校验失败的原因分类，便于上层给出可自行排查的提示：
// 格式问题（复制带入换行/不可见字符）与密钥不匹配需要用户做完全不同的事。
var (
	ErrSponsorCodeEmpty   = errors.New("赞助码为空")
	ErrSponsorCodeFormat  = errors.New("赞助码格式不正确（含非 hex 字符）")
	ErrSponsorCodeDecrypt = errors.New("赞助码解密失败（密钥不匹配）")
)

// sponsorInvisibleRunes 复制粘贴时容易被带进来、又看不出来的字符：
// 零宽空格/连接符/断行符、BOM、词连接符。换行与各种空白另有 unicode.IsSpace 处理。
var sponsorInvisibleRunes = map[rune]bool{
	'\u200b': true, // 零宽空格
	'\u200c': true, // 零宽不连字
	'\u200d': true, // 零宽连字
	'\u2060': true, // 词连接符
	'\ufeff': true, // BOM / 零宽不换行空格
}

// NormalizeSponsorCode 去掉赞助码中的全部空白与不可见字符。
// 不同平台/不同聊天工具的复制结果不一样（macOS 上尤其容易带上硬换行或零宽字符），
// 只要有一个字符残留，hex 解码就会失败并表现为"同一个码 Windows 能过、mac 不能"。
func NormalizeSponsorCode(sponsorCode string) string {
	var b strings.Builder
	b.Grow(len(sponsorCode))
	for _, r := range sponsorCode {
		if unicode.IsSpace(r) || sponsorInvisibleRunes[r] {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SafeDecryptSponsorCode 解密赞助码（hex 解码 + AES-ECB）。
// 先归一化剔除空白/不可见字符，再预校验密钥长度（16/24/32 字节）与密文块长度（16 字节整数倍），
// 并 defer recover 兜底 lancet AesEcbDecrypt 对非法输入（如密钥不匹配导致 PKCS#7 填充非法）的直接 panic；
// 任何失败以 error 返回，绝不使调用方进程崩溃。
func SafeDecryptSponsorCode(sponsorCode, keyHex string) (raw []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			raw, err = nil, fmt.Errorf("%w: %v", ErrSponsorCodeDecrypt, r)
		}
	}()
	sponsorCode = NormalizeSponsorCode(sponsorCode)
	if sponsorCode == "" {
		return nil, ErrSponsorCodeEmpty
	}
	encrypted, err := hex.DecodeString(sponsorCode)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSponsorCodeFormat, err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, fmt.Errorf("赞助码密钥 hex 解码失败: %w", err)
	}
	if l := len(key); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("赞助码密钥长度非法: %d 字节", l)
	}
	if len(encrypted) == 0 || len(encrypted)%16 != 0 {
		return nil, fmt.Errorf("%w: 密文长度 %d 字节（须为 16 字节整数倍）", ErrSponsorCodeFormat, len(encrypted))
	}
	return cryptor.AesEcbDecrypt(encrypted, key), nil
}

// SponsorVipStatus 赞助权益的权威判定结果。
// 展示端（关于页）与功能门控（K线分析 / AI 助手 / 信号监控）必须共用同一份结果，
// 否则会出现「关于页显示 VIP2，功能却提示权限不足」的展示与门控不一致。
type SponsorVipStatus struct {
	// Level 赞助码声明的 VIP 等级（未配置或解析失败时为 0）
	Level int `json:"level"`
	// Active 是否处于有效期内
	Active bool `json:"active"`
	// Reason 未生效原因（Active=true 时为空），用于向用户解释为什么用不了
	Reason string `json:"reason"`
	User   string `json:"user"`
	// 赞助码中的原始时间字符串，便于界面展示与问题排查
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	AuthTime  string `json:"authTime"`
}

// sponsorTimeLayouts 赞助码时间字段的历史写法（服务端不同版本导出格式并不完全统一）。
var sponsorTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05",
	"2006/01/02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseSponsorTime 解析赞助码中的时间字段，兼容历史多种格式与 Unix 秒/毫秒时间戳。
// ok=false 表示该字段无法识别（空字符串同样返回 false，由调用方区分「缺失」与「非法」）。
func parseSponsorTime(v any) (t time.Time, ok bool) {
	s := strings.TrimSpace(convertor.ToString(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range sponsorTimeLayouts {
		if parsed, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return parsed, true
		}
	}
	if n, err := strconv.ParseInt(strings.SplitN(s, ".", 2)[0], 10, 64); err == nil && n > 0 {
		if n > 1e12 {
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}

// EvaluateSponsorVipInfo 判断已解密的赞助码内容在当前时刻是否有效。
// 判定规则：等级 > 0 且 当前时间早于 vipEndTime。
// vipStartTime / vipAuthTime 只作展示，不参与放行判断：这两个字段是不带时区的字符串，
// 服务端按东八区写入，若按 time.Local 解析，非东八区的机器（如时区为 UTC / 美西的 macOS）
// 会把发码时刻算到未来，误判为"尚未生效"，出现"验证通过但 VIP 用不了"。
func EvaluateSponsorVipInfo(info map[string]any) SponsorVipStatus {
	return evaluateSponsorVip(info, time.Now())
}

func evaluateSponsorVip(info map[string]any, now time.Time) SponsorVipStatus {
	lvl64, _ := convertor.ToInt(info["vipLevel"])
	status := SponsorVipStatus{
		Level:     int(lvl64),
		User:      strings.TrimSpace(convertor.ToString(info["user"])),
		StartTime: strings.TrimSpace(convertor.ToString(info["vipStartTime"])),
		EndTime:   strings.TrimSpace(convertor.ToString(info["vipEndTime"])),
		AuthTime:  strings.TrimSpace(convertor.ToString(info["vipAuthTime"])),
	}
	if status.Level <= 0 {
		status.Reason = "赞助码未包含有效的 VIP 等级"
		return status
	}
	end, ok := parseSponsorTime(status.EndTime)
	if !ok {
		if status.EndTime == "" {
			status.Reason = "赞助码缺少到期时间（vipEndTime）"
		} else {
			status.Reason = fmt.Sprintf("赞助码到期时间无法识别：%s", status.EndTime)
		}
		return status
	}
	if !now.Before(end) {
		status.Reason = fmt.Sprintf("VIP 已到期（到期时间 %s）", end.Format("2006-01-02 15:04:05"))
		return status
	}
	status.Active = true
	return status
}

// EffectiveSponsorVipStatus 解密本地配置中的赞助码并返回完整判定结果（含未生效原因）。
func EffectiveSponsorVipStatus() SponsorVipStatus {
	sponsorCode := strings.TrimSpace(GetSettingConfig().SponsorCode)
	if sponsorCode == "" {
		return SponsorVipStatus{Reason: "未配置赞助码"}
	}
	keyHex := strings.TrimSpace(SponsorDecryptKeyHex)
	if keyHex == "" {
		keyHex = DefaultSponsorAESKeyHex
	}
	raw, err := SafeDecryptSponsorCode(sponsorCode, keyHex)
	if err != nil || len(raw) == 0 {
		return SponsorVipStatus{Reason: fmt.Sprintf("赞助码解密失败：%v", err)}
	}
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return SponsorVipStatus{Reason: fmt.Sprintf("赞助码内容解析失败：%v", err)}
	}
	status := EvaluateSponsorVipInfo(info)
	if !status.Active {
		// 只记录等级与原因，不记录赞助码本身
		logger.SugaredLogger.Warnf("赞助码权益未生效: level=%d reason=%s", status.Level, status.Reason)
	}
	return status
}

// EffectiveSponsorVipLevel 根据设置中的 sponsorCode 解析 VIP 等级并判断当前是否有效。
// 时间判定与展示端共用 EvaluateSponsorVipInfo，避免两处规则漂移。
func EffectiveSponsorVipLevel() (level int, active bool) {
	status := EffectiveSponsorVipStatus()
	return status.Level, status.Active
}
