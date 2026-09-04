package data

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"go-stock/backend/logger"
)

// @Author spark
// @Date 2026/9/5
// @Desc 国务院政策文件库数据源（https://sousuo.www.gov.cn/zcwjk/policyDocumentLibrary）。
//	页面为 Vue SPA，数据接口 GET https://sousuo.www.gov.cn/search-gov/data?t=zhengcelibrary&type=gwyzcwjk&...，
//	响应 searchVO.catMap 分四类：gongwen(国务院文件)/bumenfile(部门文件)/otherfile(其他文件)/gongbao(国务院公报)。
//	检索结果同步落库 PolicyNews 表（Source=发文机关），供历史关键词检索复用。

// GovPolicyDoc 国务院政策文件库条目
type GovPolicyDoc struct {
	Title        string `json:"title"`
	Url          string `json:"url"`
	Pcode        string `json:"pcode"` // 文号，如 国发〔2025〕11号
	Pubtime      string `json:"pubtime"`
	Puborg       string `json:"puborg"` // 发文机关，如 国务院/商务部 国家发展改革委...
	Summary      string `json:"summary"`
	Category     string `json:"category"`     // gongwen/bumenfile/otherfile/gongbao
	CategoryName string `json:"categoryName"` // 国务院文件/部门文件/其他文件/国务院公报
}

type GovPolicyLibApi struct {
}

func NewGovPolicyLibApi() *GovPolicyLibApi {
	return &GovPolicyLibApi{}
}

// govPolicyCategoryNames 类别代码 -> 展示名（顺序即合并输出顺序）
var govPolicyCategoryNames = []struct{ code, name string }{
	{"gongwen", "国务院文件"},
	{"bumenfile", "部门文件"},
	{"otherfile", "其他文件"},
	{"gongbao", "国务院公报"},
}

// govPolicySearchURL 政策文件库检索接口
const govPolicySearchURL = "https://sousuo.www.gov.cn/search-gov/data"

// govPolicyResp search-gov/data 接口响应（仅提取用到的字段；searchVO 位于顶层，data 恒为 null）
type govPolicyResp struct {
	Code     int    `json:"code"`
	Msg      string `json:"msg"`
	SearchVO struct {
		TotalCount int `json:"totalCount"`
		CatMap     map[string]struct {
			TotalCount int `json:"totalCount"`
			ListVO     []struct {
				Title      string `json:"title"`
				Url        string `json:"url"`
				Pcode      string `json:"pcode"`
				PubtimeStr string `json:"pubtimeStr"` // 2026.09.04
				Puborg     string `json:"puborg"`
				Summary    string `json:"summary"`
			} `json:"listVO"`
		} `json:"catMap"`
	} `json:"searchVO"`
}

// govPolicyEmRe 标题/摘要中的搜索高亮 <em> 标签
var govPolicyEmRe = regexp.MustCompile(`</?em>`)

// govPolicyFillerRe 公报标题中的填充省略号（………，标题混入正文首段时用长省略号占位）
var govPolicyFillerRe = regexp.MustCompile(`[…\.]{6,}`)

// cleanGovPolicyTitle 清理标题：去 HTML 标签（<em>高亮/<br/>换行）、去填充省略号、压缩空白
func cleanGovPolicyTitle(s string) string {
	s = stripHTMLTags(s)
	s = govPolicyFillerRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// SearchGovPolicyLibrary 检索国务院政策文件库。
//   - keyword：检索词（标题或正文，由 searchField 决定），为空则按发布时间倒序返回最新文件
//   - searchField：title=按标题检索（默认）/ content=按正文检索
//   - category：gongwen=国务院文件 / bumenfile=部门文件 / otherfile=其他文件 / gongbao=国务院公报，为空合并全部
//   - sort：score=相关度（默认）/ pubtime=发布时间倒序
//   - page：页码（1 起）
//   - pageSize：每页条数（每类分别取，默认 20，最大 50）
func (g GovPolicyLibApi) SearchGovPolicyLibrary(keyword, searchField, category, sortBy string, page, pageSize int) *[]GovPolicyDoc {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	if searchField != "content" {
		searchField = "title"
	}
	if sortBy != "pubtime" {
		sortBy = "score"
	}
	// 类别校验：非法值按全部处理
	if category != "" {
		valid := false
		for _, c := range govPolicyCategoryNames {
			if c.code == category {
				valid = true
				break
			}
		}
		if !valid {
			category = ""
		}
	}

	params := url.Values{}
	params.Set("t", "zhengcelibrary")
	params.Set("type", "gwyzcwjk")
	params.Set("q", keyword)
	params.Set("searchfield", searchField)
	params.Set("sort", sortBy)
	params.Set("sortType", "1")
	params.Set("p", fmt.Sprintf("%d", page))
	params.Set("n", fmt.Sprintf("%d", pageSize))

	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("Referer", "https://sousuo.www.gov.cn/zcwjk/policyDocumentLibrary").
		SetQueryParamsFromValues(params).
		Get(govPolicySearchURL)
	if err != nil {
		logger.SugaredLogger.Warnf("政策文件库检索失败:%v", err)
		return &[]GovPolicyDoc{}
	}
	var pr govPolicyResp
	if err := json.Unmarshal(resp.Body(), &pr); err != nil || pr.Code != 200 {
		logger.SugaredLogger.Warnf("政策文件库响应解析失败 code=%d msg=%s err=%v", pr.Code, pr.Msg, err)
		return &[]GovPolicyDoc{}
	}

	items := make([]GovPolicyDoc, 0, pageSize*4)
	seenURL := map[string]bool{}
	seenTitle := map[string]bool{}
	for _, cat := range govPolicyCategoryNames {
		if category != "" && cat.code != category {
			continue
		}
		c, ok := pr.SearchVO.CatMap[cat.code]
		if !ok {
			continue
		}
		for _, d := range c.ListVO {
			title := cleanGovPolicyTitle(d.Title)
			if title == "" || d.Url == "" {
				continue
			}
			// 跨类别去重（同一文件会同时出现在"国务院文件"与"国务院公报"）
			normalizedTitle := normalizePolicyTitle(title)
			if seenURL[d.Url] || (normalizedTitle != "" && seenTitle[normalizedTitle]) {
				continue
			}
			pubtime := strings.ReplaceAll(d.PubtimeStr, ".", "-") // 2026.09.04 -> 2026-09-04
			if !fullDateRe.MatchString(pubtime) {
				pubtime = ""
			}
			summary := strings.TrimSpace(govPolicyEmRe.ReplaceAllString(d.Summary, ""))
			if r := []rune(summary); len(r) > 120 { // 摘要截断，Markdown 表格不宜过长
				summary = string(r[:120]) + "..."
			}
			seenURL[d.Url] = true
			if normalizedTitle != "" {
				seenTitle[normalizedTitle] = true
			}
			items = append(items, GovPolicyDoc{
				Title:        title,
				Url:          d.Url,
				Pcode:        strings.TrimSpace(d.Pcode),
				Pubtime:      pubtime,
				Puborg:       strings.TrimSpace(d.Puborg),
				Summary:      summary,
				Category:     cat.code,
				CategoryName: cat.name,
			})
		}
	}

	// 合并全部类别时按发布日期倒序；单类别接口已有序
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Pubtime > items[j].Pubtime
	})

	// 检索结果落库（Source=发文机关），供历史关键词检索复用
	g.saveToPolicyNews(items)
	return &items
}

// saveToPolicyNews 将政策文件库结果同步写入 PolicyNews 表（URL 唯一索引去重）
func (g GovPolicyLibApi) saveToPolicyNews(items []GovPolicyDoc) {
	if len(items) == 0 {
		return
	}
	pi := make([]PolicyNewsItem, 0, len(items))
	for _, d := range items {
		source := d.Puborg
		if source == "" {
			source = "国务院政策文件库"
		}
		pi = append(pi, PolicyNewsItem{
			Title:  d.Title,
			Url:    d.Url,
			Date:   d.Pubtime,
			Source: source,
		})
	}
	savePolicyNews(pi)
}

// SearchGovPolicyLibraryToMarkdown 检索结果渲染为 Markdown（AI 工具输出用）
func (g GovPolicyLibApi) SearchGovPolicyLibraryToMarkdown(keyword, searchField, category, sortBy string, page, pageSize int) string {
	items := *g.SearchGovPolicyLibrary(keyword, searchField, category, sortBy, page, pageSize)

	var title string
	catName := "全部类别"
	for _, c := range govPolicyCategoryNames {
		if c.code == category {
			catName = c.name
			break
		}
	}
	if keyword != "" {
		title = fmt.Sprintf("国务院政策文件库检索：%s（%s，%s）", keyword, catName,
			map[bool]string{true: "按发布时间", false: "按相关度"}[sortBy == "pubtime"])
	} else {
		title = fmt.Sprintf("国务院政策文件库最新文件（%s）", catName)
	}
	if page > 1 {
		title += fmt.Sprintf(" 第%d页", page)
	}

	if len(items) == 0 {
		return fmt.Sprintf("## %s\n\n未检索到匹配的政策文件，建议更换关键词、把 searchField 换成 content 按正文检索，或改用 GetPolicyNewsList 获取部委官网政策新闻", title)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（本页 %d 条）\n\n", title, len(items)))
	sb.WriteString("| 日期 | 类别 | 发文机关 | 文号 | 标题 | 链接 |\n")
	sb.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, d := range items {
		pcode := d.Pcode
		if pcode == "" {
			pcode = "-"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			d.Pubtime, d.CategoryName, d.Puborg, pcode, d.Title, d.Url))
	}
	sb.WriteString("\n> 提示：可调用 GetPolicyNewsDetail 工具并传入链接获取政策全文；可用 page 参数翻页（每页默认 20 条）\n")
	return sb.String()
}
