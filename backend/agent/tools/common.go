package tools

import (
	"regexp"
	"strings"

	"github.com/duke-git/lancet/v2/strutil"
)

// @Author spark
// @Date 2025/8/5 17:20
// @Desc
//-----------------------------------------------------------------------------------

// htmlTagRe 匹配 HTML 标签（如新闻接口为关键词高亮返回的 <em>）。
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// stripHTMLForAI 移除工具文本中的 HTML 标签。
//
// 上下游新闻/资讯接口会返回带 <em> 高亮标记的正文（供前端渲染），直接写入工具
// 结果会污染模型上下文：既成倍浪费 token，又容易让模型把标签抄进回答。模型侧
// 只需要纯文本，故在工具输出边界统一清洗。
func stripHTMLForAI(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return htmlTagRe.ReplaceAllString(s, "")
}

func GetStockCode(dcCode string) string {
	if strutil.ContainsAny(dcCode, []string{"."}) {
		sp := strings.Split(dcCode, ".")
		return strings.ToLower(sp[1] + sp[0])
	}

	//北京证券交易所	8（83、87、88 等）	创新型中小企业（专精特新为主）
	//上海证券交易所	6（60、688 等）	大盘蓝筹、科创板（高新技术）
	//深圳证券交易所	0、3（000、002、30 等）	中小盘、创业板（成长型创新企业）
	switch dcCode[0:1] {
	case "8":
		return "bj" + dcCode
	case "9":
		return "bj" + dcCode
	case "6":
		return "sh" + dcCode
	case "0":
		return "sz" + dcCode
	case "3":
		return "sz" + dcCode
	}
	return dcCode
}
