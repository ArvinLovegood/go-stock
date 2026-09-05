package data

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/html/charset"
)

// @Author spark
// @Date 2026/9/4 21:00
// @Desc 政策新闻模块：从中国政府网国务院部门网站列表页获取各部门官网链接，
//	按部门抓取其官网最新政策新闻——只保留发布在该部门自己域名下的条目（链接到
//	gov.cn/新华社等的转载内容会被过滤），并按 URL+标题双维度去重避免重复资讯。
//	抓取结果持久化到 SQLite（models.PolicyNews，URL 唯一索引），支持历史回溯
//	（GetStoredPolicyNews）。通用解析基于"链接+相邻日期"启发式（各部委 CMS
//	列表页均为 <li>/<td> 内 <a>标题</a>+日期 结构）；对 JS 动态渲染站点自动
//	降级为"栏目页发现"二次抓取。

// GovDepartment 政府部门（含官网链接）
type GovDepartment struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

// PolicyNewsItem 政策新闻条目
type PolicyNewsItem struct {
	Title  string `json:"title"`
	Url    string `json:"url"`
	Date   string `json:"date"`   // yyyy-MM-dd
	Source string `json:"source"` // 部门名
}

type PolicyNewsApi struct {
}

func NewPolicyNewsApi() *PolicyNewsApi {
	return &PolicyNewsApi{}
}

// 国务院部门网站列表页（中国政府网）
const govDeptListURL = "https://www.gov.cn/home/2023-03/29/content_5748953.htm"

// policyPageOverrides 部门 -> 政策/新闻列表页（人工核验过的重点部门，绕过首页通用解析）。
// 注：海关总署/公安部/人社部/卫健委四站为瑞数反爬（HTTP 客户端返回 JS 挑战页），
// 国合署为 Vue 渲染框架页——当前抓不到数据但保留正确入口，站点放开限制后即生效。
var policyPageOverrides = map[string]string{
	"国家发展和改革委员会":  "https://www.ndrc.gov.cn/",
	"中国人民银行":      "http://www.pbc.gov.cn/goutongjiaoliu/113456/113469/index.html",
	"中国证券监督管理委员会": "http://www.csrc.gov.cn/csrc/c100028/common_xq_list.shtml",
	"财政部":         "https://www.mof.gov.cn/zhengwuxinxi/caizhengxinwen/",
	"国家统计局":       "https://www.stats.gov.cn/sj/zxfb/",
	"国家外汇管理局":     "https://www.safe.gov.cn/safe/whxw/index.html",
	"海关总署":        "http://www.customs.gov.cn/customs/xwfb34/index.html",
	"公安部":         "https://www.mps.gov.cn/n2253534/n2253535/index.html",
	"人力资源和社会保障部":  "https://www.mohrss.gov.cn/SYrlzyhshbzb/dongtaixinwen/buneiyaowen/",
	"国家卫生健康委员会":   "https://www.nhc.gov.cn/wjw/wnsj/list.shtml",
	"国家航天局":       "https://www.cnsa.gov.cn/n6758823/n6758838/index.html",
	"国家原子能机构":     "https://www.caea.gov.cn/n6760338/n6760342/index.html",
	"国家国际发展合作署":   "http://www.cidca.gov.cn/hzdt2.htm",
	"国家国防科技工业局":   "http://www.sastind.gov.cn/n10086200/n10086319/index.html",
	"国家矿山安全监察局":   "https://www.chinamine-safety.gov.cn/zfxxgk/fdzdgknr/tzgg/",
	"全国人大":        "http://www.npc.gov.cn/c2/c10134/",
	"全国政协":        "http://www.cppcc.gov.cn/zxxw/yw/",
	"中国气象局":       "https://www.cma.gov.cn/2011xwzx/2011xqxxw/2011xqxyw/",
	"中国社会科学院":     "http://www.cass.cn/yaowen/",
	"国家疾病预防控制局":   "https://www.ndcpa.gov.cn/jbkzzx/c100014/common/list.html",
	"国家数据局":       "https://www.nda.gov.cn/sjj/swdt/list/index_pc_1.html",
	"国家能源局":       "https://www.nea.gov.cn/policy/zxwj.htm",
}

// apiFetchers 部门 -> 专用 JSON API 抓取器（页面为 JS 动态渲染、但后端 API 可直接访问的站点）
var apiFetchers = map[string]func(limit int) []PolicyNewsItem{
	"国家金融监督管理总局":  fetchNfraPolicyNews,
	"中国证券监督管理委员会": fetchCsrcPolicyNews,
	"国家疾病预防控制局":   fetchNdcpaPolicyNews,
	"国家能源局":       fetchNeaPolicyNews,
}

// defaultKeyDepartments 默认重点部门（前端左侧快捷入口 + 聚合视图并发抓取）。
// 用户可通过 SaveKeyDepartments 自定义（持久化 data/key_departments.json），空列表恢复默认。
var defaultKeyDepartments = []string{
	"国家发展和改革委员会",
	"中国人民银行",
	"中国证券监督管理委员会",
	"财政部",
	"国家统计局",
	"国家外汇管理局",
	"商务部",
	"工业和信息化部",
	"住房和城乡建设部",
	"交通运输部",
	"国家数据局",
	"国家能源局",
}

// keyDepartmentsFile 重点部门外置文件（用户自定义，可编辑）
const keyDepartmentsFile = "data/key_departments.json"

// keyDepartmentFile 外置文件结构
type keyDepartmentFile struct {
	Departments []string `json:"departments"`
}

// ---- 缓存 ----
var (
	govDeptCache      []GovDepartment
	govDeptCacheAt    time.Time
	govDeptCacheMutex sync.RWMutex

	policyNewsCache      = map[string][]PolicyNewsItem{}
	policyNewsCacheAt    = map[string]time.Time{}
	policyNewsCacheMutex sync.RWMutex
)

const (
	govDeptCacheTTL    = 24 * time.Hour
	policyNewsCacheTTL = 5 * time.Minute // 与后台 5 分钟定时抓取周期匹配，保证定时任务不命中缓存空转
)

// GetGovDepartments 从中国政府网获取国务院各部门网站列表（缓存 24 小时）
func (p PolicyNewsApi) GetGovDepartments() *[]GovDepartment {
	govDeptCacheMutex.RLock()
	if govDeptCache != nil && time.Since(govDeptCacheAt) < govDeptCacheTTL {
		defer govDeptCacheMutex.RUnlock()
		return &govDeptCache
	}
	govDeptCacheMutex.RUnlock()

	var depts []GovDepartment
	seen := map[string]bool{}
	doc, err := fetchGovPage(govDeptListURL)
	if err != nil {
		logger.SugaredLogger.Errorf("GetGovDepartments 获取部门列表失败:%v", err)
		govDeptCacheMutex.RLock()
		cached := govDeptCache
		govDeptCacheMutex.RUnlock()
		return &cached
	}
	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		name := strings.TrimSpace(a.Text())
		href, exists := a.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)
		if name == "" || href == "" || strings.HasPrefix(href, "javascript") {
			return
		}
		r := []rune(name)
		if len(r) < 3 || len(r) > 20 {
			return
		}
		// 排除备案号等页脚链接（京ICP备xxx号 / 京公网安备xxx号 / 网站标识码）
		for _, kw := range []string{"ICP", "备案", "公网安备", "标识码", "无障碍", "手机版"} {
			if strings.Contains(name, kw) {
				return
			}
		}
		u, err := url.Parse(href)
		if err != nil || u.Host == "" {
			return
		}
		// 排除 gov.cn 站内导航链接（首页/繁体/英文/邮箱）及备案系统域名
		if strings.Contains(u.Host, "www.gov.cn") || strings.Contains(u.Host, "beian.") {
			return
		}
		if seen[u.Host] {
			return
		}
		seen[u.Host] = true
		depts = append(depts, GovDepartment{Name: name, Url: u.String()})
	})

	if len(depts) > 0 {
		govDeptCacheMutex.Lock()
		govDeptCache = depts
		govDeptCacheAt = time.Now()
		govDeptCacheMutex.Unlock()
	}
	return &depts
}

// GetPolicyNews 获取指定部门发布的最新政策新闻（内存缓存 30 分钟）
func (p PolicyNewsApi) GetPolicyNews(department string, limit int) *[]PolicyNewsItem {
	if limit <= 0 {
		limit = 30
	}
	policyNewsCacheMutex.RLock()
	if cache, ok := policyNewsCache["dept:"+department]; ok && time.Since(policyNewsCacheAt["dept:"+department]) < policyNewsCacheTTL {
		defer policyNewsCacheMutex.RUnlock()
		return &cache
	}
	policyNewsCacheMutex.RUnlock()

	items := p.crawlDepartment(department, limit)
	savePolicyNews(items)
	policyNewsCacheMutex.Lock()
	policyNewsCache["dept:"+department] = items
	policyNewsCacheAt["dept:"+department] = time.Now()
	policyNewsCacheMutex.Unlock()
	return &items
}

// GetKeyDeptPolicyNews 聚合重点部门最新政策新闻，按日期倒序、跨部门标题去重（缓存 30 分钟）
func (p PolicyNewsApi) GetKeyDeptPolicyNews(limit int) *[]PolicyNewsItem {
	if limit <= 0 {
		limit = 50
	}
	policyNewsCacheMutex.RLock()
	if cache, ok := policyNewsCache["keydept:all"]; ok && time.Since(policyNewsCacheAt["keydept:all"]) < policyNewsCacheTTL {
		defer policyNewsCacheMutex.RUnlock()
		return &cache
	}
	policyNewsCacheMutex.RUnlock()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		all     []PolicyNewsItem
		sem     = make(chan struct{}, 5)
		perDept = 10
	)
	for _, dept := range loadKeyDepartments() {
		wg.Add(1)
		go func(deptName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items := p.crawlDepartment(deptName, perDept)
			if len(items) == 0 {
				return
			}
			mu.Lock()
			all = append(all, items...)
			mu.Unlock()
		}(dept)
	}
	wg.Wait()

	all = dedupeAndSortPolicyNews(all, limit)
	savePolicyNews(all)
	policyNewsCacheMutex.Lock()
	policyNewsCache["keydept:all"] = all
	policyNewsCacheAt["keydept:all"] = time.Now()
	policyNewsCacheMutex.Unlock()
	return &all
}

// GetKeyDepartments 获取重点部门列表（用户自定义优先，未自定义时返回默认列表）
func (p PolicyNewsApi) GetKeyDepartments() *[]string {
	depts := loadKeyDepartments()
	return &depts
}

// SaveKeyDepartments 保存用户自定义重点部门列表（去空白/去重；空列表=恢复默认），
// 保存后立即清除聚合缓存使新配置生效。返回空串表示成功，否则为错误信息。
func (p PolicyNewsApi) SaveKeyDepartments(departments []string) string {
	cleaned := make([]string, 0, len(departments))
	seen := map[string]bool{}
	for _, d := range departments {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		cleaned = append(cleaned, d)
	}
	if len(cleaned) == 0 {
		cleaned = append(cleaned, defaultKeyDepartments...)
	}
	if err := os.MkdirAll(filepath.Dir(keyDepartmentsFile), 0755); err != nil {
		logger.SugaredLogger.Errorf("保存重点部门失败(创建目录):%v", err)
		return "保存失败：" + err.Error()
	}
	b, _ := json.MarshalIndent(keyDepartmentFile{Departments: cleaned}, "", "    ")
	if err := os.WriteFile(keyDepartmentsFile, b, 0644); err != nil {
		logger.SugaredLogger.Errorf("保存重点部门失败:%v", err)
		return "保存失败：" + err.Error()
	}
	// 清空聚合缓存，让新配置下次抓取立即生效
	policyNewsCacheMutex.Lock()
	delete(policyNewsCache, "keydept:all")
	policyNewsCacheMutex.Unlock()
	logger.SugaredLogger.Infof("重点部门已更新（%d 个）", len(cleaned))
	return ""
}

// loadKeyDepartments 读取重点部门：优先外置 data/key_departments.json，
// 文件不存在/为空/解析失败时回退默认列表（返回副本避免调用方污染默认值）。
func loadKeyDepartments() []string {
	if raw, err := os.ReadFile(keyDepartmentsFile); err == nil {
		var f keyDepartmentFile
		if json.Unmarshal(raw, &f) == nil && len(f.Departments) > 0 {
			return f.Departments
		}
	}
	return append([]string{}, defaultKeyDepartments...)
}

// GetAllDeptPolicyNews 聚合全部部门最新政策新闻（默认视图），按日期倒序、跨部门标题去重（缓存 30 分钟）。
// 使用浅抓取（每部门最多二次发现 1 个栏目页）+ 10 并发控制总耗时；结果同步落库。
func (p PolicyNewsApi) GetAllDeptPolicyNews(limit int) *[]PolicyNewsItem {
	if limit <= 0 {
		limit = 100
	}
	policyNewsCacheMutex.RLock()
	if cache, ok := policyNewsCache["alldept:all"]; ok && time.Since(policyNewsCacheAt["alldept:all"]) < policyNewsCacheTTL {
		defer policyNewsCacheMutex.RUnlock()
		return &cache
	}
	policyNewsCacheMutex.RUnlock()

	depts := p.GetGovDepartments()
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		all     []PolicyNewsItem
		sem     = make(chan struct{}, 10)
		perDept = 5
	)
	for _, dept := range *depts {
		wg.Add(1)
		go func(deptName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items := p.crawlDepartmentShallow(deptName, perDept)
			if len(items) == 0 {
				return
			}
			mu.Lock()
			all = append(all, items...)
			mu.Unlock()
		}(dept.Name)
	}
	wg.Wait()

	all = dedupeAndSortPolicyNews(all, limit)
	savePolicyNews(all)
	policyNewsCacheMutex.Lock()
	policyNewsCache["alldept:all"] = all
	policyNewsCacheAt["alldept:all"] = time.Now()
	policyNewsCacheMutex.Unlock()
	return &all
}

// savePolicyNews 批量持久化到数据库（按 URL 唯一索引去重，已存在则跳过；失败仅记日志不影响主流程）
func savePolicyNews(items []PolicyNewsItem) {
	if len(items) == 0 || db.Dao == nil {
		return
	}
	var saved int
	for _, item := range items {
		if item.Url == "" {
			continue
		}
		record := models.PolicyNews{
			Title:  item.Title,
			Url:    item.Url,
			Date:   item.Date,
			Source: item.Source,
		}
		created := models.PolicyNews{}
		res := db.Dao.Where("url = ?", record.Url).FirstOrCreate(&created, &record)
		if res.Error != nil {
			logger.SugaredLogger.Warnf("政策新闻落库失败[%s]:%v", record.Url, res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			saved++
		}
	}
	if saved > 0 {
		logger.SugaredLogger.Infof("政策新闻落库新增 %d 条", saved)
	}
}

// GetStoredPolicyNews 从数据库读取已持久化的政策新闻（支持按部门/关键词过滤，日期倒序分页）
func (p PolicyNewsApi) GetStoredPolicyNews(department, keyword string, page, pageSize int) *[]PolicyNewsItem {
	if db.Dao == nil {
		return &[]PolicyNewsItem{}
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	if page < 1 {
		page = 1
	}
	query := db.Dao.Model(&models.PolicyNews{})
	if department != "" {
		// 部门支持名称关键词模糊匹配（"能源"命中"国家能源局"，精确全名同样命中）
		query = query.Where("source like ?", "%"+department+"%")
	}
	if keyword != "" {
		query = query.Where("title like ?", "%"+keyword+"%")
	}
	records := &[]models.PolicyNews{}
	query.Order("date desc, created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(records)
	items := make([]PolicyNewsItem, 0, len(*records))
	for _, r := range *records {
		items = append(items, PolicyNewsItem{
			Title:  r.Title,
			Url:    r.Url,
			Date:   r.Date,
			Source: r.Source,
		})
	}
	return &items
}

// crawlDepartment 抓取单个部门：curated 覆盖页 -> 官网首页通用解析 -> 栏目页二次发现（最多 5 个）
func (p PolicyNewsApi) crawlDepartment(department string, limit int) []PolicyNewsItem {
	return p.crawlDepartmentDepth(department, limit, 5)
}

// crawlDepartmentShallow 浅抓取：最多二次发现 1 个栏目页（用于全部门聚合，控制总耗时）
func (p PolicyNewsApi) crawlDepartmentShallow(department string, limit int) []PolicyNewsItem {
	return p.crawlDepartmentDepth(department, limit, 1)
}

// crawlDepartmentDepth 抓取单个部门，maxDiscover 为栏目页二次发现上限（0 表示不二次发现）。
// 优先级：专用 JSON API（JS 渲染站）-> curated 覆盖页/官网首页 HTML 解析 -> 栏目页二次发现。
func (p PolicyNewsApi) crawlDepartmentDepth(department string, limit, maxDiscover int) []PolicyNewsItem {
	// 专用 JSON API（页面 JS 渲染但后端 API 可直连的站点，如金监总局/证监会）
	if fetcher, ok := apiFetchers[department]; ok {
		if items := fetcher(limit); len(items) > 0 {
			return items
		}
	}
	depts := p.GetGovDepartments()
	homeURL := ""
	for _, d := range *depts {
		if d.Name == department {
			homeURL = d.Url
			break
		}
	}
	targetURL := homeURL
	if override, ok := policyPageOverrides[department]; ok {
		targetURL = override
	}
	if targetURL == "" {
		logger.SugaredLogger.Warnf("政策新闻：未找到部门[%s]的官网链接", department)
		return []PolicyNewsItem{}
	}

	base, err := url.Parse(targetURL)
	if err != nil {
		return []PolicyNewsItem{}
	}
	doc, err := fetchGovPageRobust(targetURL)
	if err != nil {
		logger.SugaredLogger.Warnf("政策新闻：抓取[%s]%s 失败:%v", department, targetURL, err)
		return []PolicyNewsItem{}
	}
	items := extractPolicyListItems(doc, base)

	// 首页解析结果太少（JS 动态渲染站点），尝试发现新闻/政策栏目页二次抓取
	if maxDiscover > 0 && len(items) < 5 && homeURL != "" {
		discovered := discoverGovListPages(doc, base)
		if len(discovered) > maxDiscover {
			discovered = discovered[:maxDiscover]
		}
		for _, subURL := range discovered {
			subBase, err := url.Parse(subURL)
			if err != nil {
				continue
			}
			doc2, err := fetchGovPageRobust(subURL)
			if err != nil {
				continue
			}
			items = append(items, extractPolicyListItems(doc2, subBase)...)
			if len(items) >= limit {
				break
			}
		}
	}

	for i := range items {
		items[i].Source = department
	}
	return dedupeAndSortPolicyNews(items, limit)
}

// ---- 通用解析 ----

var (
	fullDateRe  = regexp.MustCompile(`(?:^|[^\d])(\d{4})([-/.年])(\d{1,2})([-/.月])(\d{1,2})[日]?(?:$|[^\d])`)
	shortDateRe = regexp.MustCompile(`(?:^|[^\d])(\d{1,2})[-/](\d{1,2})(?:$|[^\d])`)
)

// fetchGovPage 抓取页面并按 charset（GB2312/GBK/UTF-8）解码为 goquery 文档
func fetchGovPage(rawurl string) (*goquery.Document, error) {
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
		Get(rawurl)
	if err != nil {
		return nil, err
	}
	reader, err := charset.NewReader(bytes.NewReader(resp.Body()), resp.Header().Get("Content-Type"))
	if err != nil {
		return goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	}
	return goquery.NewDocumentFromReader(reader)
}

// fetchGovPageRobust 健壮抓取：普通客户端 -> TLS1.2 客户端 -> 延迟后双重试。
// 覆盖两类失败：TLS 版本不兼容（如国家航天局）与并发压力下的偶发超时/限流。
func fetchGovPageRobust(rawurl string) (*goquery.Document, error) {
	doc, err := fetchGovPage(rawurl)
	if err == nil {
		return doc, nil
	}
	// TLS 1.3 握手失败的站点（如国家航天局）用 TLS 1.2 客户端重试
	if doc, err2 := fetchGovPageTLS12(rawurl); err2 == nil {
		return doc, nil
	}
	// 偶发超时/限流：延迟后依次重试两种客户端
	time.Sleep(500 * time.Millisecond)
	if doc, err2 := fetchGovPage(rawurl); err2 == nil {
		return doc, nil
	}
	return fetchGovPageTLS12(rawurl)
}

// tls12Client 兼容仅支持 TLS 1.2 的部委站点（如国家航天局 cnsa.gov.cn，
// 其网关对 Go 默认的 TLS 1.3 ClientHello 处理异常，强制 1.2 握手即可）。
// 惰性初始化、全局复用，代理跟随共享 transport。
var tls12Client *resty.Client
var tls12Once sync.Once

func fetchGovPageTLS12(rawurl string) (*goquery.Document, error) {
	tls12Once.Do(func() {
		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          4,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ForceAttemptHTTP2:     false,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12},
			Proxy:                 GetSharedTransport().Proxy,
		}
		tls12Client = resty.NewWithClient(&http.Client{Transport: transport, Timeout: 30 * time.Second})
	})
	resp, err := tls12Client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
		Get(rawurl)
	if err != nil {
		return nil, err
	}
	reader, err := charset.NewReader(bytes.NewReader(resp.Body()), resp.Header().Get("Content-Type"))
	if err != nil {
		return goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	}
	return goquery.NewDocumentFromReader(reader)
}

// extractPolicyListItems 从页面提取"标题+链接+相邻日期"列表项（各部委 CMS 通用结构）。
// 仅保留发布在该部门自己域名（www 前缀忽略）下的条目，过滤转载的 gov.cn/新华社等外链。
func extractPolicyListItems(doc *goquery.Document, base *url.URL) []PolicyNewsItem {
	var items []PolicyNewsItem
	seen := map[string]bool{}
	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "javascript") || strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "mailto:") {
			return
		}
		title := strings.TrimSpace(a.Text())
		if t, ok := a.Attr("title"); ok {
			t = strings.TrimSpace(t)
			if t != "" {
				if len([]rune(title)) > 80 {
					// a 文本混入日期/摘要等杂质（图片卡片式列表，如国家航天局），title 属性是纯标题
					title = t
				} else if len([]rune(t)) > len([]rune(title)) {
					title = t // title 属性中的完整标题（正文常被截断）
				}
			}
		}
		title = trimTrailingDate(title) // 去掉混入标题文本的尾部日期（矿山安全局等 <a>内嵌日期的站点）
		if !isPolicyTitle(title) {
			return
		}
		u, err := base.Parse(href)
		if err != nil || u.Host == "" {
			return
		}
		// 只保留该部门自己域名下发布的条目
		if normalizeGovHost(u.Host) != normalizeGovHost(base.Host) {
			return
		}
		if seen[u.String()] {
			return
		}
		// 在父级节点文本中查找相邻日期（<li><a>标题</a><span>日期</span></li> / <td><a>标题</a> 日期</td> / <li><dt><a/></dt><dd>日期</dd></li>）
		dateStr := ""
		parent := a.Parent()
		for i := 0; i < 3 && parent.Length() > 0; i++ {
			txt := strings.TrimSpace(parent.Text())
			if len([]rune(txt)) > 300 {
				break // 父级容器过大，日期可能来自无关区块
			}
			if d := findDateInText(txt); d != "" {
				dateStr = d
				break
			}
			parent = parent.Parent()
		}
		// 父级无日期时，尝试从链接 URL 中提取（如 /202609/t20260904_xxx.shtml，社科院等站点）
		if dateStr == "" {
			dateStr = findDateInURL(u.Path)
		}
		if dateStr == "" {
			return
		}
		seen[u.String()] = true
		items = append(items, PolicyNewsItem{
			Title: title,
			Url:   u.String(),
			Date:  dateStr,
		})
	})
	return items
}

// normalizeGovHost 归一化域名（去 www. 前缀）
func normalizeGovHost(host string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
}

// isPolicyTitle 校验是否为有效标题（长度、中文占比、排除导航词）
func isPolicyTitle(t string) bool {
	r := []rune(t)
	if len(r) < 8 || len(r) > 80 {
		return false
	}
	chinese := 0
	for _, c := range r {
		if unicode.Is(unicode.Han, c) {
			chinese++
		}
	}
	if chinese < 4 {
		return false
	}
	navWords := []string{"更多", "详情", "点击", "登录", "注册", "搜索", "首页", "上一页", "下一页",
		"关于我们", "联系方式", "网站地图", "无障碍", "收藏本站", "主办单位", "承办单位",
		"常见问题", "使用帮助", "相关链接", "友情链接", "网站标识", "政府网站", "智能问答"}
	for _, w := range navWords {
		if strings.Contains(t, w) {
			return false
		}
	}
	return true
}

// trailingDateRe 标题文本尾部混入的日期（<a>标题 2026-08-26</a> 结构，矿山安全局等站点）
var trailingDateRe = regexp.MustCompile(`[\s\x{00a0}\x{3000}]*20\d{2}[-/.年]\d{1,2}[-/.月]\d{1,2}日?[\s\x{00a0}\x{3000}]*$`)

// trimTrailingDate 去掉标题尾部混入的日期文本
func trimTrailingDate(title string) string {
	return strings.TrimSpace(trailingDateRe.ReplaceAllString(title, ""))
}

// findDateInURL 从链接路径中提取发布日期（社科院等站点日期只出现在 URL 里）。
// 支持模式：/202609/t20260904_xxx.shtml、/2026/09/04/、2026-09-04、t20260904_
func findDateInURL(path string) string {
	now := time.Now()
	for _, m := range urlDateRe.FindAllStringSubmatch(path, -1) {
		date := fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}
		// 仅接受近两年的日期（URL 中旧文章链接很多，过老的视为无效）
		if t.After(now.AddDate(-2, 0, 0)) && !t.After(now.Add(24*time.Hour)) {
			return date
		}
	}
	return ""
}

// urlDateRe URL 中的日期模式：20260904 / 2026-09-04 / 2026/09/04
var urlDateRe = regexp.MustCompile(`(20\d{2})[-/]?(\d{2})[-/]?(\d{2})`)

// findDateInText 从文本中提取日期（支持 2026-09-01 / 2026/9/1 / 2026.09.01 / 2026年9月1日 / 09-01）。
// 标题中常含未来日期（如"XX法2027年1月1日起施行"），非发布日期——遍历全部匹配取第一个不早于明天之前的。
func findDateInText(txt string) string {
	now := time.Now()
	for _, m := range fullDateRe.FindAllStringSubmatch(txt, -1) {
		date := fmt.Sprintf("%s-%s-%s", m[1], pad2(m[3]), pad2(m[5]))
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}
		// 超过明天的完整日期视为标题中的未来日期，非列表发布日期
		if t.After(now.Add(24 * time.Hour)) {
			continue
		}
		return date
	}
	for _, m := range shortDateRe.FindAllStringSubmatch(txt, -1) {
		date := fmt.Sprintf("%d-%s-%s", now.Year(), pad2(m[1]), pad2(m[2]))
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}
		if t.After(now.Add(24 * time.Hour)) {
			// 未来日期视为去年（跨年 12 月发布的新闻）
			date = fmt.Sprintf("%d-%s-%s", now.Year()-1, pad2(m[1]), pad2(m[2]))
		}
		return date
	}
	return ""
}

func pad2(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// discoverGovListPages 从首页发现新闻/政策栏目页链接（用于 JS 渲染首页的二次抓取）
func discoverGovListPages(doc *goquery.Document, base *url.URL) []string {
	keywords := []string{"新闻", "要闻", "政策", "公告", "发布", "文件", "动态"}
	var urls []string
	seen := map[string]bool{}
	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		name := strings.TrimSpace(a.Text())
		if name == "" || len([]rune(name)) > 12 {
			return
		}
		matched := false
		for _, kw := range keywords {
			if strings.Contains(name, kw) {
				matched = true
				break
			}
		}
		if !matched {
			return
		}
		href, exists := a.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "javascript") || strings.HasPrefix(href, "#") {
			return
		}
		u, err := base.Parse(href)
		if err != nil || u.Host == "" {
			return
		}
		// 仅同站点栏目页
		if normalizeGovHost(u.Host) != normalizeGovHost(base.Host) {
			return
		}
		if seen[u.String()] {
			return
		}
		seen[u.String()] = true
		urls = append(urls, u.String())
	})
	if len(urls) > 5 {
		urls = urls[:5]
	}
	return urls
}

// dedupeAndSortPolicyNews 按 URL+标题双维度去重（避免同一政策重复展示）、按日期倒序，截取前 limit 条
func dedupeAndSortPolicyNews(items []PolicyNewsItem, limit int) []PolicyNewsItem {
	seenURL := map[string]bool{}
	seenTitle := map[string]bool{}
	result := make([]PolicyNewsItem, 0, len(items))
	for _, item := range items {
		if item.Url == "" {
			continue
		}
		normalizedTitle := normalizePolicyTitle(item.Title)
		if seenURL[item.Url] || (normalizedTitle != "" && seenTitle[normalizedTitle]) {
			continue
		}
		seenURL[item.Url] = true
		if normalizedTitle != "" {
			seenTitle[normalizedTitle] = true
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})
	if len(result) > limit {
		result = result[:limit]
	}
	if result == nil {
		result = []PolicyNewsItem{}
	}
	return result
}

// normalizePolicyTitle 标题归一化（去空白/书名号/标点），用于跨来源重复识别
func normalizePolicyTitle(t string) string {
	puncts := " \t\n\r\u300a\u300b\u3008\u3009\u201c\u201d\u2018\u2019\u3010\u3011[]\uff08\uff08()\uff0c,\u3002.\u3001\uff1a:\uff01!\uff1f?\u2014-\uff0d\u00b7"
	var sb strings.Builder
	for _, c := range t {
		if strings.ContainsRune(puncts, c) {
			continue
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

// ---- 专用 JSON API 抓取器（JS 渲染站点）----

// nfraDoc NFRA 文档记录（接口字段）
type nfraDoc struct {
	DocId       int    `json:"docId"`
	DocTitle    string `json:"docTitle"`
	PublishDate string `json:"publishDate"` // "2026-08-31 18:50:21"
	IsTitleLink string `json:"isTitleLink"`
	TitleLink   string `json:"titleLink"` // 外链（如指向 gov.cn）时非空
}

// nfraResp NFRA 栏目接口响应
type nfraResp struct {
	RptCode int `json:"rptCode"`
	Data    []struct {
		ItemName string    `json:"itemName"`
		ItemUrl  string    `json:"itemUrl"`
		ItemId   int       `json:"itemId"`
		Docs     []nfraDoc `json:"docInfoVOList"`
	} `json:"data"`
}

// fetchNfraPolicyNews 国家金融监督管理总局新闻（页面 Angular 渲染，后端 API 可直连）。
// itemId=914 为"新闻资讯"父栏目，返回各子栏目（时政要闻/监管动态/政策解读等）及各自最新文档。
func fetchNfraPolicyNews(limit int) []PolicyNewsItem {
	apiURL := "https://www.nfra.gov.cn/cbircweb/DocInfo/SelectItemAndDocByItemPId?itemId=914&pageSize=20"
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("Referer", "https://www.nfra.gov.cn/cn/view/pages/xinwenzixun/xinwenzixun.html").
		Get(apiURL)
	if err != nil {
		logger.SugaredLogger.Warnf("政策新闻：金监总局 API 请求失败:%v", err)
		return nil
	}
	body := resp.Body()
	var nr nfraResp
	if err := json.Unmarshal(body, &nr); err != nil || nr.RptCode != 200 {
		logger.SugaredLogger.Warnf("政策新闻：金监总局 API 解析失败:%v", err)
		return nil
	}
	// 外链文档（转 gov.cn/新华社等的时政要闻）不算金监总局自产新闻，跳过
	var items []PolicyNewsItem
	for _, sub := range nr.Data {
		for _, doc := range sub.Docs {
			if doc.DocTitle == "" || (doc.IsTitleLink == "1" && doc.TitleLink != "") {
				continue
			}
			link := fmt.Sprintf("https://www.nfra.gov.cn/cn/view/pages/ItemDetail.html?docId=%d&itemId=%d", doc.DocId, sub.ItemId)
			date := ""
			if t, err := time.Parse("2006-01-02 15:04:05", doc.PublishDate); err == nil {
				date = t.Format("2006-01-02")
			}
			items = append(items, PolicyNewsItem{
				Title:  doc.DocTitle,
				Url:    link,
				Date:   date,
				Source: "国家金融监督管理总局",
			})
		}
	}
	return dedupeAndSortPolicyNews(items, limit)
}

// csrcResp 证监会 searchList 接口响应
type csrcResp struct {
	Data struct {
		Total   int `json:"total"`
		Results []struct {
			Title         string `json:"title"`
			Url           string `json:"url"`           // 协议相对 "//www.csrc.gov.cn/..."
			PublishedTime int64  `json:"publishedTime"` // 毫秒时间戳
			ChannelName   string `json:"channelName"`
		} `json:"results"`
	} `json:"data"`
}

// fetchCsrcPolicyNews 中国证监会（页面 JS 渲染，searchList API 可直连）。
// 合并两个栏目：
//   - a1a078ee0bc54721ab6b148884c784a8 "证监会要闻"（c100028）
//   - 8d1c236a98924e38a854bbb9f215efb9 "政府信息公开-主动公开/行政许可批复"（c101955/zfxxgk_zdgk.shtml，注册批复/备案公示等）
func fetchCsrcPolicyNews(limit int) []PolicyNewsItem {
	if limit <= 0 {
		limit = 20
	}
	channels := []struct{ id, referer string }{
		{"a1a078ee0bc54721ab6b148884c784a8", "https://www.csrc.gov.cn/csrc/c100028/common_xq_list.shtml"},
		{"8d1c236a98924e38a854bbb9f215efb9", "https://www.csrc.gov.cn/csrc/c101955/zfxxgk_zdgk.shtml"},
	}
	var items []PolicyNewsItem
	for _, ch := range channels {
		apiURL := fmt.Sprintf("https://www.csrc.gov.cn/searchList/%s?_isAgg=true&_isJson=true&_pageSize=%d&_template=index&_rangeTimeGte=&_channelName=&page=1", ch.id, limit)
		resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
			SetHeader("Referer", ch.referer).
			Get(apiURL)
		if err != nil {
			logger.SugaredLogger.Warnf("政策新闻：证监会 API 请求失败:%v", err)
			continue
		}
		var cr csrcResp
		if err := json.Unmarshal(resp.Body(), &cr); err != nil {
			logger.SugaredLogger.Warnf("政策新闻：证监会 API 解析失败:%v", err)
			continue
		}
		for _, r := range cr.Data.Results {
			if r.Title == "" || r.Url == "" {
				continue
			}
			link := r.Url
			if strings.HasPrefix(link, "//") {
				link = "https:" + link
			}
			// 附件类条目（.xlsx/.docx/.pdf 等下载链接）跳过，只保留政策文档页
			if regexp.MustCompile(`\.(xlsx?|docx?|pdf|zip|rar|wps)($|\?)`).MatchString(strings.ToLower(link)) {
				continue
			}
			date := ""
			if r.PublishedTime > 0 {
				date = time.Unix(r.PublishedTime/1000, 0).Format("2006-01-02")
			}
			items = append(items, PolicyNewsItem{
				Title:  r.Title,
				Url:    link,
				Date:   date,
				Source: "中国证券监督管理委员会",
			})
		}
	}
	return dedupeAndSortPolicyNews(items, limit)
}

// ---- 内嵌 JSON 数据解析（政府网站集约化平台 Vue 站点）----

// ndcpaJSONRe 集约化平台内嵌数据字段：aT=标题 aPd=发布时间 aU=链接（嵌套转义 JSON）
var (
	ndcpaTitleRe = regexp.MustCompile(`"aT":"((?:[^"\\]|\\.)*)"`)
	ndcpaDateRe  = regexp.MustCompile(`"aPd":"(\d{4}-\d{2}-\d{2})[ ]?[\d:]*"`)
	ndcpaURLRe   = regexp.MustCompile(`"aU":"\{\\"common\\":\\"([^"\\]+)`)
)

// fetchNdcpaPolicyNews 国家疾病预防控制局（Vue 渲染，文章数据以转义 JSON 内嵌在列表页 script 中）。
// 每条记录形如 "aT":"标题","aPd":"2026-07-31 14:33","aU":"{\"common\":\"/jbkzzx/...html\"}"，
// 按 aT 锚点切分记录后逐条提取。
func fetchNdcpaPolicyNews(limit int) []PolicyNewsItem {
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		Get("https://www.ndcpa.gov.cn/jbkzzx/c100014/common/list.html")
	if err != nil {
		logger.SugaredLogger.Warnf("政策新闻：疾控局列表页请求失败:%v", err)
		return nil
	}
	html := string(resp.Body())
	// 按 "aT" 出现位置切分记录块（每个 aT 与其后的 aPd/aU 属同一条）
	var items []PolicyNewsItem
	matches := ndcpaTitleRe.FindAllStringSubmatchIndex(html, -1)
	for i, loc := range matches {
		title := strings.ReplaceAll(html[loc[2]:loc[3]], `\"`, `"`)
		if !isPolicyTitle(title) {
			continue
		}
		// 记录块：当前 aT 到下一个 aT（或文末）之间
		end := len(html)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		block := html[loc[0]:end]
		date := ""
		if m := ndcpaDateRe.FindStringSubmatch(block); m != nil {
			date = m[1]
		}
		link := ""
		if m := ndcpaURLRe.FindStringSubmatch(block); m != nil {
			link = "https://www.ndcpa.gov.cn" + m[1]
		}
		if date == "" || link == "" {
			continue
		}
		items = append(items, PolicyNewsItem{
			Title:  title,
			Url:    link,
			Date:   date,
			Source: "国家疾病预防控制局",
		})
	}
	return dedupeAndSortPolicyNews(items, limit)
}

// neaDoc 国家能源局集约化平台 datasource 记录（Xhwpage 前端组件的数据接口字段）
type neaDoc struct {
	Title       string `json:"title"`
	ShowTitle   string `json:"showTitle"`
	PublishUrl  string `json:"publishUrl"`  // 绝对或相对（../20260904/xxx/c.html）链接
	PublishTime string `json:"publishTime"` // "2026-09-04 15:53:27"
	ContentType string `json:"contentType"`
}

// neaResp 国家能源局集约化平台 datasource JSON 响应
type neaResp struct {
	CategoryName string   `json:"categoryName"`
	Datasource   []neaDoc `json:"datasource"`
}

// neaHTMLTagRe 标题中内嵌的 HTML 标签（能源局 datasource 的 title/showTitle 常含 <a href=...>）
var neaHTMLTagRe = regexp.MustCompile(`<[^>]+>`)

// stripHTMLTags 去掉字符串中的 HTML 标签并压缩空白
func stripHTMLTags(s string) string {
	s = neaHTMLTagRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// fetchNeaPolicyNews 国家能源局（页面 Vue+Xhwpage 渲染拿不到列表 HTML，数据走集约化平台
// datasource JSON 接口：列表页 <ul data="datasource:{id}"> → 同目录 ds_{id}.json）。
// 合并两个栏目：最新文件（policy/zxwj.htm，正式政策文件）与能源要闻（xwzx/nyyw.htm，新闻动态）。
func fetchNeaPolicyNews(limit int) []PolicyNewsItem {
	if limit <= 0 {
		limit = 20
	}
	channels := []struct{ jsonURL, referer string }{
		{"https://www.nea.gov.cn/policy/ds_40d365c13659452aa06cdb7268d6192e.json", "https://www.nea.gov.cn/policy/zxwj.htm"},
		{"https://www.nea.gov.cn/xwzx/ds_8839d76f7cb542ca8cbaab7122cc9b83.json", "https://www.nea.gov.cn/xwzx/nyyw.htm"},
	}
	var items []PolicyNewsItem
	for _, ch := range channels {
		base, err := url.Parse(ch.jsonURL)
		if err != nil {
			continue
		}
		resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
			SetHeader("Referer", ch.referer).
			Get(ch.jsonURL)
		if err != nil {
			logger.SugaredLogger.Warnf("政策新闻：能源局 API 请求失败:%v", err)
			continue
		}
		var nr neaResp
		if err := json.Unmarshal(resp.Body(), &nr); err != nil {
			logger.SugaredLogger.Warnf("政策新闻：能源局 API 解析失败:%v", err)
			continue
		}
		for _, doc := range nr.Datasource {
			title := stripHTMLTags(doc.Title)
			if title == "" {
				title = stripHTMLTags(doc.ShowTitle)
			}
			if !isPolicyTitle(title) {
				continue
			}
			link := strings.TrimSpace(doc.PublishUrl)
			if link == "" {
				continue
			}
			// 相对链接按 JSON 所在目录解析（../20260904/xxx/c.html → www.nea.gov.cn/20260904/xxx/c.html）
			if !strings.HasPrefix(link, "http") {
				if abs, err := base.Parse(link); err == nil {
					link = abs.String()
				}
			}
			// 只保留能源局自己域名下的条目
			if !strings.Contains(link, "nea.gov.cn") {
				continue
			}
			date := ""
			if len(doc.PublishTime) >= 10 {
				date = doc.PublishTime[:10]
			}
			items = append(items, PolicyNewsItem{
				Title:  title,
				Url:    link,
				Date:   date,
				Source: "国家能源局",
			})
		}
	}
	return dedupeAndSortPolicyNews(items, limit)
}

// ---- AI 工具输出 ----

// crawlDepartmentsByKeyword 按部门名称关键词实时抓取（库为空时的回退路径）：
// "能源"解析为"国家能源局"，"央行"等简称未收录时无命中。精确部门名直接走单部门
// 缓存路径；关键词可命中多个部门时并发抓取（上限 8 个，防止"局"这类宽泛词拖垮耗时）。
func (p PolicyNewsApi) crawlDepartmentsByKeyword(keyword string, limit int) *[]PolicyNewsItem {
	depts := p.GetGovDepartments()
	for _, d := range *depts {
		if d.Name == keyword {
			return p.GetPolicyNews(keyword, limit)
		}
	}
	var matched []string
	for _, d := range *depts {
		if strings.Contains(d.Name, keyword) {
			matched = append(matched, d.Name)
			if len(matched) >= 8 {
				break
			}
		}
	}
	if len(matched) == 0 {
		logger.SugaredLogger.Warnf("政策新闻：部门关键词[%s]未命中任何部门", keyword)
		return &[]PolicyNewsItem{}
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		all []PolicyNewsItem
		sem = make(chan struct{}, 5)
	)
	for _, name := range matched {
		wg.Add(1)
		go func(deptName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items := p.crawlDepartment(deptName, limit)
			mu.Lock()
			all = append(all, items...)
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	all = dedupeAndSortPolicyNews(all, len(matched)*limit)
	savePolicyNews(all)
	return &all
}

// GetPolicyNewsToMarkdown 政策新闻列表渲染为 Markdown（AI 工具输出用）。
// department 为空=全部部门聚合；keyword 非空时检索已入库的历史政策（标题模糊匹配）。
func (p PolicyNewsApi) GetPolicyNewsToMarkdown(department, keyword string, limit int) string {
	if limit <= 0 {
		limit = 20
	}
	var items []PolicyNewsItem
	var title string
	if keyword != "" {
		items = *p.GetStoredPolicyNews(department, keyword, 1, limit)
		title = fmt.Sprintf("政策新闻搜索：%s", keyword)
		if department != "" {
			title += "（" + department + "）"
		}
	} else {
		// 最新列表：优先读库（毫秒级，后台 5 分钟定时任务持续入库，新鲜度足够），
		// 库为空（首次使用/该部门未入库）才回退实时抓取，避免 AI 工具同步等待 82 站点
		stored := *p.GetStoredPolicyNews(department, "", 1, limit)
		if len(stored) > 0 {
			items = stored
		} else if department != "" {
			items = *p.crawlDepartmentsByKeyword(department, limit)
		} else {
			items = *p.GetAllDeptPolicyNews(limit)
		}
		if department != "" {
			title = department + " 最新政策新闻"
		} else {
			title = "全部部门最新政策新闻"
		}
	}
	if len(items) == 0 {
		if keyword != "" {
			return fmt.Sprintf("## %s\n\n未找到匹配的政策新闻，建议先不带关键词获取最新政策列表，或更换关键词", title)
		}
		return fmt.Sprintf("## %s\n\n暂未抓取到政策新闻", title)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（共 %d 条）\n\n", title, len(items)))
	sb.WriteString("| 日期 | 部门 | 标题 | 链接 |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", item.Date, item.Source, item.Title, item.Url))
	}
	sb.WriteString("\n> 提示：可调用 GetPolicyNewsDetail 工具并传入链接获取某条政策的全文内容\n")
	return sb.String()
}

// GetPolicyNewsDetail 抓取政策详情页并提取正文，返回 Markdown（AI 工具输出用）。
// url 为政策新闻列表中的原文链接；正文截断至约 8000 字防止超上下文。
func (p PolicyNewsApi) GetPolicyNewsDetail(rawurl string) string {
	if rawurl == "" {
		return "参数 url 不能为空"
	}
	u, err := url.Parse(rawurl)
	if err != nil || u.Host == "" || !strings.Contains(u.Host, ".gov.cn") {
		return "仅支持政府部门网站（*.gov.cn）的政策详情链接"
	}
	doc, err := fetchGovPage(rawurl)
	if err != nil {
		return fmt.Sprintf("抓取政策详情失败：%v", err)
	}

	// 标题：优先 h1/article 标题，退化到 <title>
	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
		if i := strings.Index(title, "-"); i > 8 { // 去掉"XX部-标题"站名前缀
			title = strings.TrimSpace(title[:i])
		}
	}

	// 正文：政府网站常见正文容器优先，退化到文本最长的 div/td
	content := ""
	for _, sel := range []string{"#UCAP-CONTENT", ".pages_content", "#content", ".article", ".article-content", ".TRS_Editor", ".content", "#zoom", ".detail"} {
		if s := doc.Find(sel).First(); s.Length() > 0 {
			if txt := strings.TrimSpace(s.Text()); len([]rune(txt)) > 200 {
				content = txt
				break
			}
		}
	}
	if content == "" {
		doc.Find("div,td").Each(func(_ int, s *goquery.Selection) {
			if content != "" {
				return
			}
			if txt := strings.TrimSpace(s.Text()); len([]rune(txt)) > 500 {
				// 排除整页容器（链接过多的是导航/页脚区块）
				if s.Find("a").Length() < 20 {
					content = txt
				}
			}
		})
	}
	if content == "" {
		return fmt.Sprintf("## %s\n\n未能提取到政策正文（页面可能为 JS 动态渲染）\n\n原文链接：%s", title, rawurl)
	}

	// 清理正文：压缩连续空白、去页脚噪声
	content = cleanPolicyContent(content)
	if r := []rune(content); len(r) > 8000 {
		content = string(r[:8000]) + "\n\n...（正文过长已截断）"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))
	sb.WriteString(fmt.Sprintf("- 原文链接：%s\n\n", rawurl))
	sb.WriteString(content)
	return sb.String()
}

// cleanPolicyContent 压缩连续空白并移除常见页脚/工具栏噪声行
func cleanPolicyContent(s string) string {
	s = regexp.MustCompile(`[ \t\x{00a0}\x{3000}]+`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\n\s*\n+`).ReplaceAllString(s, "\n")
	// 页面工具栏噪声：【字体： 大 中 小 】/【中大小】/ 打印 / 收藏 / 分享 / 面包屑导航 等
	s = regexp.MustCompile(`【(字体[^】]*|中大小|大中小|打印|收藏)】`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?m)^[\s\x{00a0}\x{3000}]*(打印|收藏|分享.*|打开微信.*|使用"扫一扫".*|扫描二维码.*|首页\s*>\s*.*)[\s\x{00a0}\x{3000}]*$`).ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	noisePrefixes := []string{"主办单位", "承办单位", "网站标识码", "ICP备", "公网安备", "版权所有", "京ICP", "网站地图", "联系我们", "相关链接"}
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		skip := false
		for _, p := range noisePrefixes {
			if strings.HasPrefix(line, p) || strings.Contains(line, p) && len([]rune(line)) < 40 {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
