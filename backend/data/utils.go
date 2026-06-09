package data

import (
	"go-stock/backend/logger"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
)

// @Author spark
// @Date 2025/2/13 13:08
// @Desc
//-----------------------------------------------------------------------------------

// SensitiveWords 敏感词
var SensitiveWords = strings.Split("戊边、戍边")

// ReplaceSensitiveWords 过滤敏感词
func ReplaceSensitiveWords(text string) string {
	for _, word := range SensitiveWords {
		if strings.Contains(text, word) {
			text = strings.ReplaceAll(text, word, "")
		}
	}
	return text
}

// RemoveAllBlankChar  使用正则表达式移除字符串中的空白字符
func RemoveAllBlankChar(s string) string {
	return removeAllSpaces(s)
}
func removeAllSpaces(s string) string {
	re := regexp.MustCompile(`\s`)
	return re.ReplaceAllString(s, "")
}

// RemoveAllNonDigitChar 去除所有非数字字符
func RemoveAllNonDigitChar(s string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(s, "")
}

// RemoveAllDigitChar 去除所有数字字符
func RemoveAllDigitChar(s string) string {
	re := regexp.MustCompile(`\d`)
	return re.ReplaceAllString(s, "")
}

// ConvertStockCodeToTushareCode 将股票代码转换为tushare的股票代码
func ConvertStockCodeToTushareCode(stockCode string) string {
	//提取非数字
	if strutil.HasPrefixAny(stockCode, []string{"SH", "sh", "SZ", "sz", "BJ", "bj", "HK", "hk"}) {
		stockCode = RemoveAllNonDigitChar(stockCode) + "." + strings.ToUpper(RemoveAllDigitChar(stockCode))
	}
	return stockCode
}

// ConvertTushareCodeToStockCode 将tushare股票代码转换为的普通股票代码
func ConvertTushareCodeToStockCode(stockCode string) string {
	//提取非数字
	stockCode = strings.ToLower(RemoveAllDigitChar(stockCode)) + RemoveAllNonDigitChar(stockCode)
	return strings.ReplaceAll(stockCode, ".", "")
}
func ConvertTushareCodeToStockCodes(a []string) []string {
	return slice.Map(a, func(i int, s string) string {
		return ConvertTushareCodeToStockCode(s)
	})
}

func GetTableMarkdown(document *goquery.Document, waitVisible string, markdown *strings.Builder) {
	document.Find(waitVisible).First().Find("tr").Each(func(index int, item *goquery.Selection) {
		row := ""
		item.Find("th, td").Each(func(i int, cell *goquery.Selection) {
			text := cell.Children().FilterFunction(func(i int, s *goquery.Selection) bool {
				return isVisible(s)
			}).Text()
			if text == "" {
				text = cell.Text()
			}

			row += "|" + text
		})
		row += "|"

		if index == 0 {
			// Header row
			markdown.WriteString(row + "\n")
			// Separator row
			separator := ""
			item.Find("th, td").Each(func(i int, cell *goquery.Selection) {
				separator += "|---"
			})
			markdown.WriteString(separator + "|\n")
		} else {
			// Data row
			markdown.WriteString(row + "\n")
		}
	})
	logger.SugaredLogger.Infof("\n%s", markdown.String())
}

// isVisible 函数用于判断元素是否可见
func isVisible(s *goquery.Selection) bool {
	// 检查 display 属性
	display, _ := s.Attr("style")
	if strings.Contains(strings.ToLower(display), "display: none") {
		return false
	}
	// 检查 visibility 属性
	if strings.Contains(strings.ToLower(display), "visibility: hidden") {
		return false
	}
	// 检查 opacity 属性
	if strings.Contains(strings.ToLower(display), "opacity: 0") {
		return false
	}
	return true
}
