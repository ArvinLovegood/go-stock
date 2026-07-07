package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

var (
	upPreRegex    = regexp.MustCompile(`涨([0-9])`)
	downPreRegex  = regexp.MustCompile(`跌([0-9])`)
	codeSpanRegex = regexp.MustCompile("`([^`]+)`")
	boldRegex     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex   = regexp.MustCompile(`\*([^*]+)\*`)
	upRegex       = regexp.MustCompile(`(\+[0-9]+\.?[0-9]*%|\+[0-9]+\.?[0-9]*pp|涨[0-9]+\.?[0-9]*%?|涨停|超预期|新高|突破|放量|多头|金叉|强势|偏强)`)
	downRegex     = regexp.MustCompile(`(\-[0-9]+\.?[0-9]*%|\-[0-9]+\.?[0-9]*pp|跌[0-9]+\.?[0-9]*%?|跌停|低于预期|新低|破位|缩量|空头|死叉|弱势|偏弱)`)
	tagBuyRegex   = regexp.MustCompile(`(买入|强烈推荐|推荐|增持)`)
	tagSellRegex  = regexp.MustCompile(`(卖出|减持|回避)`)
	tagHoldRegex  = regexp.MustCompile(`(持有|中性|观望|持有观望)`)
	tagScaleRegex = regexp.MustCompile(`(规模量产|量产|已突破|完成|成功|投产|上线)`)
	tagDevRegex   = regexp.MustCompile(`(研发|预研|试产|认证中|小批量|N/A|规划中)`)
	appIconCache  string
)

func SetAppIcon(data []byte) {
	if len(data) > 0 {
		appIconCache = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
		logger.SugaredLogger.Infof("SetAppIcon成功, 大小: %d", len(data))
	}
}

const mdImageHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
    font-family: "Inter", -apple-system, BlinkMacSystemFont, "SF Pro Display", "Segoe UI", Roboto, "PingFang SC", "Microsoft YaHei", sans-serif;
    font-size: 14px;
    line-height: 1.75;
    color: #d1d5db;
    background: #080c18;
    padding: 24px 20px 18px;
    width: 480px;
    word-wrap: break-word;
    overflow-wrap: break-word;
    background-image:
        radial-gradient(ellipse at 20% 0%, rgba(56,189,248,0.08) 0%, transparent 50%),
        radial-gradient(ellipse at 80% 0%, rgba(168,85,247,0.06) 0%, transparent 50%),
        linear-gradient(rgba(56,189,248,0.02) 1px, transparent 1px),
        linear-gradient(90deg, rgba(56,189,248,0.02) 1px, transparent 1px);
    background-size: 100% 100%, 100% 100%, 24px 24px, 24px 24px;
}
body::before {
    content: "";
    position: fixed;
    top: 0; left: 0; right: 0;
    height: 3px;
    background: linear-gradient(90deg, #38bdf8, #818cf8, #c084fc, #f472b6, #fb923c, #38bdf8);
    background-size: 300% 100%;
}
body.light {
    color: #334155;
    background: #f8fafc;
    background-image:
        radial-gradient(ellipse at 20% 0%, rgba(59,130,246,0.05) 0%, transparent 50%),
        radial-gradient(ellipse at 80% 0%, rgba(139,92,246,0.04) 0%, transparent 50%),
        linear-gradient(rgba(59,130,246,0.03) 1px, transparent 1px),
        linear-gradient(90deg, rgba(59,130,246,0.03) 1px, transparent 1px);
    background-size: 100% 100%, 100% 100%, 24px 24px, 24px 24px;
}
body.light::before {
    background: linear-gradient(90deg, #3b82f6, #8b5cf6, #ec4899, #f97316, #3b82f6);
    background-size: 300% 100%;
}
.header {
    display: flex;
    align-items: center;
    margin-bottom: 18px;
    padding-bottom: 14px;
    border-bottom: 1px solid rgba(56,189,248,0.12);
    position: relative;
}
.header::after {
    content: "";
    position: absolute;
    bottom: -1px;
    left: 0;
    width: 80px;
    height: 2px;
    background: linear-gradient(90deg, #38bdf8, #818cf8, transparent);
}
body.light .header {
    border-bottom-color: rgba(59,130,246,0.15);
}
body.light .header::after {
    background: linear-gradient(90deg, #3b82f6, #8b5cf6, transparent);
}
.header .avatar {
    width: 42px;
    height: 42px;
    border-radius: 8px;
    overflow: hidden;
    margin-right: 12px;
    flex-shrink: 0;
    box-shadow: 0 0 16px rgba(56,189,248,0.25);
    border: 1px solid rgba(56,189,248,0.25);
}
body.light .header .avatar {
    box-shadow: 0 2px 8px rgba(59,130,246,0.15);
    border-color: rgba(59,130,246,0.15);
}
.header .avatar img {
    width: 100%;
    height: 100%;
    object-fit: contain;
}
.header .info .name {
    font-size: 16px;
    font-weight: 700;
    background: linear-gradient(90deg, #e2e8f0, #38bdf8);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    letter-spacing: 0.5px;
}
body.light .header .info .name {
    background: linear-gradient(90deg, #0f172a, #3b82f6);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
}
.header .info .time {
    font-size: 11px;
    color: #64748b;
    font-family: "SF Mono", Consolas, "Liberation Mono", Menlo, monospace;
    margin-top: 2px;
}
.content { word-break: break-word; }
.content h1 {
    font-size: 19px; margin: 18px 0 10px; font-weight: 700;
    background: linear-gradient(90deg, #38bdf8, #818cf8);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    padding-bottom: 8px;
    border-bottom: 1px solid rgba(56,189,248,0.12);
}
body.light .content h1 {
    background: linear-gradient(90deg, #1e40af, #7c3aed);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    border-bottom-color: rgba(59,130,246,0.15);
}
.content h2 {
    font-size: 16px; margin: 16px 0 8px; font-weight: 700;
    color: #e2e8f0;
    padding: 6px 12px;
    background: linear-gradient(90deg, rgba(56,189,248,0.1), rgba(129,140,248,0.05), transparent);
    border-radius: 6px;
    border-left: 3px solid #38bdf8;
}
body.light .content h2 {
    color: #1e293b;
    background: linear-gradient(90deg, rgba(59,130,246,0.08), transparent);
    border-left-color: #3b82f6;
}
.content h3 {
    font-size: 15px; margin: 14px 0 6px; font-weight: 600;
    color: #c084fc;
    padding-left: 10px;
    border-left: 2px solid #c084fc;
}
body.light .content h3 {
    color: #7c3aed;
    border-left-color: #8b5cf6;
}
.content p { margin: 8px 0; }
.content ul, .content ol { margin: 8px 0; padding-left: 20px; }
.content li {
    margin: 5px 0;
    padding-left: 4px;
    line-height: 1.7;
}
.content ul li::marker { color: #38bdf8; font-size: 1.1em; }
body.light .content ul li::marker { color: #3b82f6; }
.content strong { font-weight: 600; color: #38bdf8; }
body.light .content strong { color: #1d4ed8; }
.content em { font-style: italic; color: #c084fc; }
body.light .content em { color: #7c3aed; }
.up { color: #ef4444 !important; font-weight: 500; }
body.light .up { color: #dc2626 !important; }
.down { color: #22c55e !important; font-weight: 500; }
body.light .down { color: #16a34a !important; }
.tag {
    display: inline-block;
    padding: 1px 7px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    line-height: 1.5;
    vertical-align: middle;
}
.tag-green { background: rgba(34,197,94,0.15); color: #4ade80; border: 1px solid rgba(34,197,94,0.25); }
body.light .tag-green { background: rgba(22,163,74,0.1); color: #15803d; border-color: rgba(22,163,74,0.2); }
.tag-red { background: rgba(239,68,68,0.15); color: #f87171; border: 1px solid rgba(239,68,68,0.25); }
body.light .tag-red { background: rgba(220,38,38,0.08); color: #b91c1c; border-color: rgba(220,38,38,0.15); }
.tag-blue { background: rgba(56,189,248,0.15); color: #38bdf8; border: 1px solid rgba(56,189,248,0.25); }
body.light .tag-blue { background: rgba(59,130,246,0.1); color: #1d4ed8; border-color: rgba(59,130,246,0.2); }
.tag-orange { background: rgba(251,146,60,0.15); color: #fb923c; border: 1px solid rgba(251,146,60,0.25); }
body.light .tag-orange { background: rgba(234,88,12,0.1); color: #c2410c; border-color: rgba(234,88,12,0.2); }
.tag-purple { background: rgba(192,132,252,0.15); color: #c084fc; border: 1px solid rgba(192,132,252,0.25); }
body.light .tag-purple { background: rgba(139,92,246,0.1); color: #7c3aed; border-color: rgba(139,92,246,0.2); }
.content code {
    background: rgba(56,189,248,0.1);
    padding: 1px 6px;
    border-radius: 4px;
    font-family: "SF Mono", Consolas, "Liberation Mono", Menlo, monospace;
    font-size: 12px;
    color: #7dd3fc;
    border: 1px solid rgba(56,189,248,0.18);
}
body.light .content code {
    background: rgba(59,130,246,0.08);
    color: #2563eb;
    border-color: rgba(59,130,246,0.15);
}
.content pre {
    background: #0c1021;
    color: #7ee787;
    padding: 14px 14px 14px;
    border-radius: 8px;
    overflow-x: auto;
    margin: 10px 0;
    font-size: 12px;
    line-height: 1.5;
    border: 1px solid rgba(56,189,248,0.1);
    box-shadow: 0 0 24px rgba(56,189,248,0.04), inset 0 1px 0 rgba(255,255,255,0.03);
    position: relative;
}
.content pre::before {
    content: "";
    position: absolute;
    top: 10px; left: 14px;
    width: 6px; height: 6px;
    border-radius: 50%;
    background: #ff5f57;
    box-shadow: 10px 0 0 #febc2e, 20px 0 0 #28c840;
}
.content pre code {
    background: none;
    padding: 0;
    color: inherit;
    font-size: inherit;
    border: none;
    display: block;
    margin-top: 14px;
}
body.light .content pre {
    background: #1e293b;
    border-color: rgba(59,130,246,0.12);
    box-shadow: 0 2px 12px rgba(0,0,0,0.08);
}
.content blockquote {
    border-left: 3px solid #f59e0b;
    padding: 8px 14px;
    margin: 10px 0;
    background: linear-gradient(90deg, rgba(245,158,11,0.08), rgba(245,158,11,0.02));
    color: #fbbf24;
    border-radius: 0 8px 8px 0;
    font-size: 13px;
}
body.light .content blockquote {
    background: linear-gradient(90deg, rgba(245,158,11,0.08), transparent);
    color: #b45309;
    border-left-color: #f59e0b;
}
.content table {
    border-collapse: separate;
    border-spacing: 0;
    width: 100%;
    margin: 12px 0;
    font-size: 12px;
    table-layout: fixed;
    word-break: break-all;
    border-radius: 8px;
    overflow: hidden;
    border: 1px solid rgba(56,189,248,0.12);
    box-shadow: 0 4px 16px rgba(0,0,0,0.2);
}
body.light .content table {
    border-color: rgba(59,130,246,0.12);
    box-shadow: 0 2px 8px rgba(59,130,246,0.06);
}
.content th, .content td {
    padding: 7px 8px;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    border-bottom: 1px solid rgba(56,189,248,0.06);
    border-right: 1px solid rgba(56,189,248,0.06);
}
.content th:last-child, .content td:last-child {
    border-right: none;
}
body.light .content th, body.light .content td {
    border-bottom-color: rgba(59,130,246,0.06);
    border-right-color: rgba(59,130,246,0.06);
}
.content th {
    background: linear-gradient(135deg, rgba(56,189,248,0.15), rgba(129,140,248,0.1));
    font-weight: 600;
    white-space: nowrap;
    color: #7dd3fc;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.3px;
}
body.light .content th {
    background: linear-gradient(135deg, rgba(59,130,246,0.1), rgba(139,92,246,0.06));
    color: #1e40af;
    text-shadow: none;
}
.content td {
    background: rgba(12,16,33,0.6);
}
.content tbody tr:nth-child(odd) td {
    background: rgba(12,16,33,0.4);
}
.content tbody tr:nth-child(even) td {
    background: rgba(56,189,248,0.04);
}
body.light .content td {
    background: rgba(255,255,255,0.8);
}
body.light .content tbody tr:nth-child(even) td {
    background: rgba(59,130,246,0.04);
}
.content tbody tr:hover td {
    background: rgba(56,189,248,0.08);
}
.content tbody tr:last-child td {
    border-bottom: none;
}
.content hr {
    border: none;
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(129,140,248,0.3), rgba(56,189,248,0.3), rgba(192,132,252,0.3), transparent);
    margin: 14px 0;
}
body.light .content hr {
    background: linear-gradient(90deg, transparent, rgba(59,130,246,0.2), rgba(139,92,246,0.2), transparent);
}
.disclaimer {
    margin-top: 16px;
    padding-top: 10px;
    border-top: 1px solid rgba(129,140,248,0.1);
    font-size: 10px;
    color: #475569;
    text-align: center;
    letter-spacing: 0.5px;
}
body.light .disclaimer {
    border-top-color: rgba(59,130,246,0.1);
    color: #94a3b8;
}
</style>
</head>
<body class="{{.ThemeClass}}">
<div class="header">
    <div class="avatar"><img src="{{.AvatarSrc}}" alt="Go-Stock"></div>
    <div class="info">
        <div class="name">Go-Stock AI</div>
        <div class="time">{{.Time}}</div>
    </div>
</div>
<div class="content">{{.Content}}</div>
<div class="disclaimer">以上内容仅供参考 · 不构成任何投资建议</div>
</body>
</html>`

func loadAppIcon() string {
	if appIconCache != "" {
		return appIconCache
	}
	logger.SugaredLogger.Warn("appicon未通过SetAppIcon设置，使用默认SVG头像")
	return ""
}

func isDarkTheme() bool {
	cfg := GetSettingConfig()
	return cfg.DarkTheme
}

func getThemeClass() string {
	if isDarkTheme() {
		return ""
	}
	return "light"
}

// MarkdownToImageBytes 将 markdown 文本渲染为 PNG 图片字节（原始字节，未做 base64 包装）。
// 使用 chromedp 截图精心设计的 HTML 模板（深色/浅色主题、语法高亮、彩色标签、表格样式）。
// 供飞书机器人等需要上传原始图片字节到云端的场景使用。
func MarkdownToImageBytes(text string) ([]byte, error) {
	htmlContent := simpleMarkdownToHTML(text)
	now := time.Now().Format("2006-01-02 15:04")

	avatarSrc := loadAppIcon()
	if avatarSrc == "" {
		avatarSrc = "data:image/svg+xml," + urlEscapeSimpleSVG()
	}

	themeClass := getThemeClass()

	tpl := strings.ReplaceAll(mdImageHTMLTemplate, "{{.AvatarSrc}}", avatarSrc)
	tpl = strings.ReplaceAll(tpl, "{{.Time}}", now)
	tpl = strings.ReplaceAll(tpl, "{{.Content}}", htmlContent)
	tpl = strings.ReplaceAll(tpl, "{{.ThemeClass}}", themeClass)

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	isDark := themeClass == ""

	tplJSON, _ := json.Marshal(tpl)

	var buf []byte
	actions := []chromedp.Action{
		chromedp.Navigate("about:blank"),
		emulation.SetDeviceMetricsOverride(480, 800, 1.0, false),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if isDark {
				return emulation.SetEmulatedMedia().
					WithMedia("screen").
					WithFeatures([]*emulation.MediaFeature{
						{Name: "prefers-color-scheme", Value: "dark"},
					}).Do(ctx)
			}
			return nil
		}),
		chromedp.Evaluate(fmt.Sprintf(`document.open(); document.write(%s); document.close();`, string(tplJSON)), nil),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var height int64
			if err := chromedp.Evaluate(`Math.max(document.documentElement.scrollHeight, document.body.scrollHeight)`, &height).Do(ctx); err != nil {
				return err
			}
			if height < 800 {
				height = 800
			}
			return emulation.SetDeviceMetricsOverride(480, height, 1.0, false).Do(ctx)
		}),
		chromedp.Sleep(100 * time.Millisecond),
		chromedp.Screenshot("body", &buf, chromedp.ByQuery, chromedp.NodeVisible),
	}

	if err := chromedp.Run(ctx, actions...); err != nil {
		return nil, fmt.Errorf("chromedp截图失败: %w", err)
	}

	if len(buf) == 0 {
		return nil, fmt.Errorf("截图结果为空")
	}

	logger.SugaredLogger.Infof("生成图片成功, 主题=%s, 大小: %d bytes", themeClass, len(buf))
	return buf, nil
}

// markdownToImage 将 markdown 文本渲染为 base64 编码的 PNG 图片字符串（QQ Bot 约定格式）。
// 保留原签名以兼容 QQ bot 的 sendGroupImageMessage/sendPrivateImageMessage。
func markdownToImage(text string) (string, error) {
	buf, err := MarkdownToImageBytes(text)
	if err != nil {
		return "", err
	}
	return "base64://" + base64.StdEncoding.EncodeToString(buf), nil
}

func urlEscapeSimpleSVG() string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36"><rect width="36" height="36" rx="18" fill="url(#g)"/><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="%23667eea"/><stop offset="100%" stop-color="%23764ba2"/></linearGradient></defs><text x="18" y="24" text-anchor="middle" font-size="18" fill="white">📈</text></svg>`
	return svg
}

func simpleMarkdownToHTML(text string) string {
	text = html.EscapeString(text)

	text = strings.ReplaceAll(text, "&#34;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&lt;", "<")

	var result strings.Builder
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	inList := false
	var codeBlock strings.Builder

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				inCodeBlock = false
				result.WriteString("<pre><code>")
				result.WriteString(html.EscapeString(codeBlock.String()))
				result.WriteString("</code></pre>")
				codeBlock.Reset()
			} else {
				if inList {
					result.WriteString("</ul>")
					inList = false
				}
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			if codeBlock.Len() > 0 {
				codeBlock.WriteString("\n")
			}
			codeBlock.WriteString(line)
			continue
		}

		stripped := strings.TrimSpace(line)
		if stripped == "" {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			continue
		}

		if strings.HasPrefix(stripped, "### ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h3>")
			result.WriteString(inlineFormat(stripped[4:]))
			result.WriteString("</h3>")
		} else if strings.HasPrefix(stripped, "## ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h2>")
			result.WriteString(inlineFormat(stripped[3:]))
			result.WriteString("</h2>")
		} else if strings.HasPrefix(stripped, "# ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h1>")
			result.WriteString(inlineFormat(stripped[2:]))
			result.WriteString("</h1>")
		} else if strings.HasPrefix(stripped, "- ") || strings.HasPrefix(stripped, "* ") {
			if !inList {
				result.WriteString("<ul>")
				inList = true
			}
			result.WriteString("<li>")
			result.WriteString(inlineFormat(stripped[2:]))
			result.WriteString("</li>")
		} else if strings.HasPrefix(stripped, "> ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<blockquote>")
			result.WriteString(inlineFormat(stripped[2:]))
			result.WriteString("</blockquote>")
		} else if stripped == "---" || stripped == "***" {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<hr>")
		} else if isTableRow(stripped) {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			var tableRows []string
			tableRows = append(tableRows, stripped)
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if isTableRow(next) {
					tableRows = append(tableRows, next)
					i = j
				} else {
					break
				}
			}
			if len(tableRows) >= 2 {
				result.WriteString(renderTableHTML(tableRows))
			} else {
				result.WriteString("<p>")
				result.WriteString(inlineFormat(stripped))
				result.WriteString("</p>")
			}
		} else {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<p>")
			result.WriteString(inlineFormat(stripped))
			result.WriteString("</p>")
		}
	}

	if inList {
		result.WriteString("</ul>")
	}
	if inCodeBlock {
		result.WriteString("<pre><code>")
		result.WriteString(html.EscapeString(codeBlock.String()))
		result.WriteString("</code></pre>")
	}

	return result.String()
}

func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	parts := strings.Split(trimmed, "|")
	nonEmpty := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty++
		}
	}
	return nonEmpty >= 1
}

func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := strings.Split(trimmed, "|")
	for _, cell := range cells {
		c := strings.TrimSpace(cell)
		if c == "" {
			continue
		}
		for _, ch := range c {
			if ch != '-' && ch != ':' {
				return false
			}
		}
	}
	return true
}

func parseTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := strings.Split(trimmed, "|")
	result := make([]string, 0, len(cells))
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}
	return result
}

func renderTableHTML(rows []string) string {
	var sb strings.Builder
	sb.WriteString("<table>")

	headerWritten := false
	for _, row := range rows {
		if isTableSeparator(row) {
			continue
		}
		cells := parseTableRow(row)
		if !headerWritten {
			sb.WriteString("<thead><tr>")
			for _, cell := range cells {
				sb.WriteString("<th>")
				sb.WriteString(cellFormat(cell))
				sb.WriteString("</th>")
			}
			sb.WriteString("</tr></thead>")
			sb.WriteString("<tbody>")
			headerWritten = true
			continue
		}
		sb.WriteString("<tr>")
		for _, cell := range cells {
			sb.WriteString("<td>")
			sb.WriteString(cellFormat(cell))
			sb.WriteString("</td>")
		}
		sb.WriteString("</tr>")
	}
	if headerWritten {
		sb.WriteString("</tbody>")
	}
	sb.WriteString("</table>")
	return sb.String()
}

func inlineFormat(text string) string {
	text = codeSpanRegex.ReplaceAllString(text, `<code>$1</code>`)
	text = boldRegex.ReplaceAllString(text, `<strong>$1</strong>`)
	text = italicRegex.ReplaceAllString(text, `<em>$1</em>`)
	return text
}

func cellFormat(text string) string {
	text = upPreRegex.ReplaceAllString(text, `+$1`)
	text = downPreRegex.ReplaceAllString(text, `-$1`)
	text = codeSpanRegex.ReplaceAllString(text, `<code>$1</code>`)
	text = boldRegex.ReplaceAllString(text, `<strong>$1</strong>`)
	text = italicRegex.ReplaceAllString(text, `<em>$1</em>`)
	text = tagBuyRegex.ReplaceAllString(text, `<span class="tag tag-red">$1</span>`)
	text = tagSellRegex.ReplaceAllString(text, `<span class="tag tag-green">$1</span>`)
	text = tagHoldRegex.ReplaceAllString(text, `<span class="tag tag-orange">$1</span>`)
	text = tagScaleRegex.ReplaceAllString(text, `<span class="tag tag-blue">$1</span>`)
	text = tagDevRegex.ReplaceAllString(text, `<span class="tag tag-purple">$1</span>`)
	text = upRegex.ReplaceAllString(text, `<span class="up">$1</span>`)
	text = downRegex.ReplaceAllString(text, `<span class="down">$1</span>`)
	return text
}
