package data

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
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
		// extendresult.facetMap.bmfl：部门名 -> "N条"（文件库标准部门名列表，用于关键词解析）
		ExtendResult struct {
			FacetMap struct {
				Bmfl map[string]string `json:"bmfl"`
			} `json:"facetMap"`
		} `json:"extendresult"`
	} `json:"searchVO"`
}

// 部门 facet 名单内存缓存（标准部门名列表，24 小时刷新）
var (
	govPolicyDeptNames      []string
	govPolicyDeptNamesAt    time.Time
	govPolicyDeptNamesMutex sync.Mutex
)

const govPolicyDeptNamesTTL = 24 * time.Hour

// loadGovPolicyDeptNames 获取文件库标准部门名列表（走一次 n=1 的空查询取 facetMap.bmfl 键，带内存缓存）
func (g GovPolicyLibApi) loadGovPolicyDeptNames() []string {
	govPolicyDeptNamesMutex.Lock()
	defer govPolicyDeptNamesMutex.Unlock()
	if len(govPolicyDeptNames) > 0 && time.Since(govPolicyDeptNamesAt) < govPolicyDeptNamesTTL {
		return govPolicyDeptNames
	}
	params := url.Values{}
	params.Set("t", "zhengcelibrary")
	params.Set("type", "gwyzcwjk")
	params.Set("q", "")
	params.Set("sort", "pubtime")
	params.Set("sortType", "1")
	params.Set("p", "1")
	params.Set("n", "1")
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("Referer", "https://sousuo.www.gov.cn/zcwjk/policyDocumentLibrary").
		SetQueryParamsFromValues(params).
		Get(govPolicySearchURL)
	if err != nil {
		logger.SugaredLogger.Warnf("政策文件库部门名单获取失败:%v", err)
		return govPolicyDeptNames
	}
	var pr govPolicyResp
	if err := json.Unmarshal(resp.Body(), &pr); err != nil || pr.Code != 200 {
		logger.SugaredLogger.Warnf("政策文件库部门名单解析失败 code=%d err=%v", pr.Code, err)
		return govPolicyDeptNames
	}
	names := make([]string, 0, len(pr.SearchVO.ExtendResult.FacetMap.Bmfl))
	for name := range pr.SearchVO.ExtendResult.FacetMap.Bmfl {
		names = append(names, name)
	}
	if len(names) > 0 {
		govPolicyDeptNames = names
		govPolicyDeptNamesAt = time.Now()
	}
	return govPolicyDeptNames
}

// resolveGovPolicyDepartment 部门关键词解析为文件库标准部门名：
// 精确匹配优先，其次包含匹配（"能源"->"国家能源局"；多个命中取第一个）。
// 返回空串表示无命中。注：国务院本级机关（国务院/国务院办公厅等）不在 bmfl 名单，
// 走 zhengcelibrary_gw + puborg 通道，由调用方单独处理。
func (g GovPolicyLibApi) resolveGovPolicyDepartment(keyword string) string {
	if keyword == "" {
		return ""
	}
	names := g.loadGovPolicyDeptNames()
	for _, n := range names {
		if n == keyword {
			return n
		}
	}
	for _, n := range names {
		if strings.Contains(n, keyword) {
			return n
		}
	}
	return ""
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
//   - department：发文机关过滤，支持全名或名称关键词（"能源"->国家能源局）；命中部委时检索
//     其部门文件，含"国务院"时走国务院本级文件通道（puborg 精确匹配，如 国务院办公厅）
//   - category：gongwen=国务院文件 / bumenfile=部门文件 / otherfile=其他文件 / gongbao=国务院公报，
//     为空合并全部（指定 department 委部时忽略，自动限定对应通道）
//   - sortBy：score=相关度（默认）/ pubtime=发布时间倒序
//   - page：页码（1 起）
//   - pageSize：每页条数（每类分别取，默认 20，最大 50）
func (g GovPolicyLibApi) SearchGovPolicyLibrary(keyword, searchField, department, category, sortBy string, page, pageSize int) *[]GovPolicyDoc {
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
	params.Set("q", keyword)
	params.Set("searchfield", searchField)
	params.Set("sort", sortBy)
	params.Set("sortType", "1")
	params.Set("p", fmt.Sprintf("%d", page))
	params.Set("n", fmt.Sprintf("%d", pageSize))

	// 发文机关过滤（部委走 _bm+bmfl，国务院本级走 _gw+puborg；未命中则不加过滤）
	if department != "" {
		if strings.Contains(department, "国务院") {
			// 国务院本级机关（国务院/国务院办公厅/国务院、中央军委 等），puborg 需精确名
			params.Set("t", "zhengcelibrary_gw")
			params.Set("puborg", department)
			category = "" // _gw 通道只返回国务院文件
		} else if resolved := g.resolveGovPolicyDepartment(department); resolved != "" {
			// 部委：bmfl 需 facetMap 标准部门名（含"其他文件"类，如政策解读）
			params.Set("t", "zhengcelibrary_bm")
			params.Set("bmfl", resolved)
		} else {
			logger.SugaredLogger.Warnf("政策文件库：部门[%s]未命中标准部门名单，忽略部门过滤", department)
		}
	}
	if params.Get("t") == "" {
		params.Set("t", "zhengcelibrary")
		params.Set("type", "gwyzcwjk")
	}

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
func (g GovPolicyLibApi) SearchGovPolicyLibraryToMarkdown(keyword, searchField, department, category, sortBy string, page, pageSize int) string {
	items := *g.SearchGovPolicyLibrary(keyword, searchField, department, category, sortBy, page, pageSize)

	var title string
	catName := "全部类别"
	for _, c := range govPolicyCategoryNames {
		if c.code == category {
			catName = c.name
			break
		}
	}
	// 部门过滤：标题中体现解析后的标准部门名（解析走内存缓存，无额外请求）
	var deptName string
	if department != "" {
		if strings.Contains(department, "国务院") {
			deptName = department
		} else {
			deptName = g.resolveGovPolicyDepartment(department)
		}
	}
	if keyword != "" {
		title = fmt.Sprintf("国务院政策文件库检索：%s（%s，%s，%s）", keyword, catName, deptName,
			map[bool]string{true: "按发布时间", false: "按相关度"}[sortBy == "pubtime"])
	} else if deptName != "" {
		title = fmt.Sprintf("%s 政策文件（%s）", deptName,
			map[bool]string{true: "按发布时间", false: "按相关度"}[sortBy == "pubtime"])
	} else {
		title = fmt.Sprintf("国务院政策文件库最新文件（%s）", catName)
	}
	if page > 1 {
		title += fmt.Sprintf(" 第%d页", page)
	}

	if len(items) == 0 {
		hint := "建议更换关键词、把 searchField 换成 content 按正文检索，或改用 GetPolicyNewsList 获取部委官网政策新闻"
		if department != "" && deptName == "" {
			hint = fmt.Sprintf("部门[%s]未命中文件库发文机关名单，建议改用部门全名或去掉 department 参数；也可改用 GetPolicyNewsList 获取部委官网政策新闻", department)
		}
		return fmt.Sprintf("## %s\n\n未检索到匹配的政策文件，%s", title, hint)
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
