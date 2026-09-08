package data

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"

	"github.com/tidwall/gjson"
)

// imageBedUploadURL 免费公共图床上传端点（img.scdn.io）。
// 参考 https://img.scdn.io/api_docs.php ：POST multipart/form-data，文件字段名 image，
// 成功响应 {"success":true,"url":"https://img.scdn.io/i/xxx.webp"}。
const imageBedUploadURL = "https://img.scdn.io/api/v1.php"

// imageBedUploadMaxSize 图床单图大小上限（服务端会自动压缩转 WebP）
const imageBedUploadMaxSize = 10 * 1024 * 1024

// UploadImageToImageBed 将 base64 编码图片（data URL 或纯 base64）上传到免费图床，
// 返回可直接访问的外链 URL。AI 助手视觉对话默认走外链 URL 模式：
// 请求体小、不占用 base64 内联体积，且多轮对话/会话存储不膨胀。
// HTTP 客户端复用共享 transport（自动跟随应用代理设置）。
func UploadImageToImageBed(base64Data string, filename string) (string, error) {
	b64 := base64Data
	// 剥离 data URL 前缀（data:image/png;base64,xxx）
	if strings.HasPrefix(base64Data, "data:") {
		if idx := strings.Index(base64Data, ","); idx >= 0 {
			b64 = base64Data[idx+1:]
		}
	}
	fileBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("图片 base64 解码失败: %w", err)
	}
	if len(fileBytes) == 0 {
		return "", fmt.Errorf("图片内容为空")
	}
	if len(fileBytes) > imageBedUploadMaxSize {
		return "", fmt.Errorf("图片超过 %dMB 上限", imageBedUploadMaxSize/1024/1024)
	}

	if filename == "" {
		filename = "image.png"
	}
	// 图床按文件真实内容（magic bytes）识别类型；修正扩展名便于服务端处理与展示
	if ext := sniffImageExt(fileBytes); ext != "" && !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename += ext
	}

	client := CreateHTTPClientWithTimeout(60 * time.Second)
	resp, err := client.R().
		SetFileReader("image", filename, bytes.NewReader(fileBytes)).
		Post(imageBedUploadURL)
	if err != nil {
		logger.SugaredLogger.Errorf("图床上传请求失败: %s", err.Error())
		return "", fmt.Errorf("图床上传请求失败: %w", err)
	}
	body := resp.String()
	if resp.StatusCode() != 200 {
		logger.SugaredLogger.Errorf("图床返回 HTTP %d: %s", resp.StatusCode(), truncateRespBody(body))
		return "", fmt.Errorf("图床返回 HTTP %d: %s", resp.StatusCode(), truncateRespBody(body))
	}
	url := gjson.Get(body, "url").String()
	if !gjson.Get(body, "success").Bool() || url == "" {
		errMsg := gjson.Get(body, "message").String()
		if errMsg == "" {
			errMsg = gjson.Get(body, "error").String()
		}
		if errMsg == "" {
			errMsg = truncateRespBody(body)
		}
		logger.SugaredLogger.Errorf("图床上传失败: %s", errMsg)
		return "", fmt.Errorf("图床上传失败: %s", errMsg)
	}
	return url, nil
}

// sniffImageExt 通过 magic bytes 识别图片格式并返回对应扩展名。
func sniffImageExt(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G'}):
		return ".png"
	case len(b) >= 3 && bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case len(b) >= 6 && (bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a"))):
		return ".gif"
	case len(b) >= 12 && bytes.HasPrefix(b, []byte("RIFF")) && bytes.HasSuffix(b[:12], []byte("WEBP")):
		return ".webp"
	case len(b) >= 2 && (bytes.HasPrefix(b, []byte("BM"))):
		return ".bmp"
	case len(b) >= 4 && (bytes.HasPrefix(b, []byte("II*\x00")) || bytes.HasPrefix(b, []byte("MM\x00*"))):
		return ".tiff"
	default:
		return ""
	}
}

// truncateRespBody 截断响应体用于错误展示与日志。
func truncateRespBody(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}
