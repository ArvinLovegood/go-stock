package models

import (
	"encoding/json"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// @Author spark
// @Date 2025/2/6 15:25
// @Desc
//-----------------------------------------------------------------------------------

type StockChangeHistory struct {
	ID         uint      `json:"id" gorm:"primarykey" md:"-"`
	ChangeTime string    `json:"changeTime" gorm:"uniqueIndex:idx_unique_change;size:10" md:"异动时间"`       // 异动时间 HH:MM:SS
	ChangeDate string    `json:"changeDate" gorm:"uniqueIndex:idx_unique_change;index;size:10" md:"异动日期"` // 异动日期 YYYY-MM-DD
	StockCode  string    `json:"stockCode" gorm:"uniqueIndex:idx_unique_change;index;size:20" md:"股票代码"`  // 股票代码
	StockName  string    `json:"stockName" gorm:"size:50" md:"股票名称"`                                      // 股票名称
	Market     int       `json:"market" md:"-"`                                                           // 市场
	ChangeType int       `json:"changeType" gorm:"uniqueIndex:idx_unique_change;index" md:"-"`            // 异动类型代码
	TypeName   string    `json:"typeName" gorm:"size:20" md:"异动类型名称"`                                     // 异动类型名称
	Volume     int64     `json:"volume" gorm:"uniqueIndex:idx_unique_change" md:"成交量(股)"`                 // 成交量(股)
	Price      float64   `json:"price" gorm:"uniqueIndex:idx_unique_change" md:"价格"`                      // 价格
	ChangeRate float64   `json:"changeRate" gorm:"uniqueIndex:idx_unique_change" md:" 涨跌幅(%)"`            // 涨跌幅(%)
	Amount     float64   `json:"amount" gorm:"uniqueIndex:idx_unique_change" md:"金额"`                     // 金额
	Industry   string    `json:"industry" gorm:"size:100" md:"所属行业"`                                      // 所属行业
	Concept    string    `json:"concept" gorm:"size:500" md:"所属概念"`                                       // 所属概念
	CreatedAt  time.Time `json:"createdAt" gorm:"autoCreateTime" md:"-"`
}

func (StockChangeHistory) TableName() string {
	return "stock_change_history"
}

type StockChangeHistoryQuery struct {
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	ChangeType    int     `json:"changeType"`
	ChangeTypes   []int   `json:"changeTypes"`
	TypeName      string  `json:"typeName"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	StartTime     string  `json:"startTime"`
	EndTime       string  `json:"endTime"`
	MinVolume     int64   `json:"minVolume"`
	MinAmount     float64 `json:"minAmount"`
	MinChangeRate float64 `json:"minChangeRate"`
	MaxChangeRate float64 `json:"maxChangeRate"`
	Industry      string  `json:"industry"`
	Concept       string  `json:"concept"`
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
}

type StockChangeHistoryPageData struct {
	List       []StockChangeHistory `json:"list"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	TotalPages int                  `json:"totalPages"`
}

type GitHubReleaseVersion struct {
	Url       string `json:"url"`
	AssetsUrl string `json:"assets_url"`
	UploadUrl string `json:"upload_url"`
	HtmlUrl   string `json:"html_url"`
	Id        int    `json:"id"`
	Author    struct {
		Login             string `json:"login"`
		Id                int    `json:"id"`
		NodeId            string `json:"node_id"`
		AvatarUrl         string `json:"avatar_url"`
		GravatarId        string `json:"gravatar_id"`
		Url               string `json:"url"`
		HtmlUrl           string `json:"html_url"`
		FollowersUrl      string `json:"followers_url"`
		FollowingUrl      string `json:"following_url"`
		GistsUrl          string `json:"gists_url"`
		StarredUrl        string `json:"starred_url"`
		SubscriptionsUrl  string `json:"subscriptions_url"`
		OrganizationsUrl  string `json:"organizations_url"`
		ReposUrl          string `json:"repos_url"`
		EventsUrl         string `json:"events_url"`
		ReceivedEventsUrl string `json:"received_events_url"`
		Type              string `json:"type"`
		UserViewType      string `json:"user_view_type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	NodeId          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	Assets          []struct {
		Url      string `json:"url"`
		Id       int    `json:"id"`
		NodeId   string `json:"node_id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			Id                int    `json:"id"`
			NodeId            string `json:"node_id"`
			AvatarUrl         string `json:"avatar_url"`
			GravatarId        string `json:"gravatar_id"`
			Url               string `json:"url"`
			HtmlUrl           string `json:"html_url"`
			FollowersUrl      string `json:"followers_url"`
			FollowingUrl      string `json:"following_url"`
			GistsUrl          string `json:"gists_url"`
			StarredUrl        string `json:"starred_url"`
			SubscriptionsUrl  string `json:"subscriptions_url"`
			OrganizationsUrl  string `json:"organizations_url"`
			ReposUrl          string `json:"repos_url"`
			EventsUrl         string `json:"events_url"`
			ReceivedEventsUrl string `json:"received_events_url"`
			Type              string `json:"type"`
			UserViewType      string `json:"user_view_type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadUrl string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballUrl string `json:"tarball_url"`
	ZipballUrl string `json:"zipball_url"`
	Body       string `json:"body"`
	Tag        Tag    `json:"tag"`
	Commit     Commit `json:"commit"`
}

type Tag struct {
	Ref    string `json:"ref"`
	NodeId string `json:"node_id"`
	Url    string `json:"url"`
	Object struct {
		Sha  string `json:"sha"`
		Type string `json:"type"`
		Url  string `json:"url"`
	} `json:"object"`
}

type Commit struct {
	Sha     string `json:"sha"`
	NodeId  string `json:"node_id"`
	Url     string `json:"url"`
	HtmlUrl string `json:"html_url"`
	Author  struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"author"`
	Committer struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"committer"`
	Tree struct {
		Sha string `json:"sha"`
		Url string `json:"url"`
	} `json:"tree"`
	Message string `json:"message"`
	Parents []struct {
		Sha     string `json:"sha"`
		Url     string `json:"url"`
		HtmlUrl string `json:"html_url"`
	} `json:"parents"`
	Verification struct {
		Verified   bool        `json:"verified"`
		Reason     string      `json:"reason"`
		Signature  interface{} `json:"signature"`
		Payload    interface{} `json:"payload"`
		VerifiedAt interface{} `json:"verified_at"`
	} `json:"verification"`
}

type AIResponseResult struct {
	gorm.Model
	ChatId    string                `json:"chatId"`
	ModelName string                `json:"modelName"`
	StockCode string                `json:"stockCode"`
	StockName string                `json:"stockName"`
	Question  string                `json:"question"`
	Content   string                `json:"content"`
	IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (receiver AIResponseResult) TableName() string {
	return "ai_response_result"
}

// AIResponseResultQuery 分页查询参数
type AIResponseResultQuery struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页大小
	ChatId    string `form:"chatId" json:"chatId"`       // 聊天ID筛选
	ModelName string `form:"modelName" json:"modelName"` // 模型名称筛选
	StockCode string `form:"stockCode" json:"stockCode"` // 股票代码筛选
	StockName string `form:"stockName" json:"stockName"` // 股票名称筛选
	Question  string `form:"question" json:"question"`   // 问题内容模糊搜索
	StartDate string `form:"startDate" json:"startDate"` // 开始日期
	EndDate   string `form:"endDate" json:"endDate"`     // 结束日期
}

// AIResponseResultPageResp 分页查询响应
type AIResponseResultPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    AIResponseResultPageData `json:"data"`
}

type AIResponseResultPageData struct {
	List       []AIResponseResult `json:"list"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

type VersionInfo struct {
	gorm.Model
	Version           string                `json:"version"`
	Content           string                `json:"content"`
	Icon              string                `json:"icon"`
	Alipay            string                `json:"alipay"`
	Wxpay             string                `json:"wxpay"`
	Wxgzh             string                `json:"wxgzh"`
	BuildTimeStamp    int64                 `json:"buildTimeStamp"`
	OfficialStatement string                `json:"officialStatement"`
	IsDel             soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (receiver VersionInfo) TableName() string {
	return "version_info"
}

type StockInfoHK struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (receiver StockInfoHK) TableName() string {
	return "stock_base_info_hk"
}

type StockInfoUS struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	Exchange string                `json:"exchange"`
	Type     string                `json:"type"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (receiver StockInfoUS) TableName() string {
	return "stock_base_info_us"
}

type Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
		Type    string `json:"type"`
	} `json:"error"`
}

type PromptTemplate struct {
	ID        int `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `json:"name"`
	Content   string `json:"content"`
	Type      string `json:"type"`
}

func (p PromptTemplate) TableName() string {
	return "prompt_templates"
}

// PromptTemplateQuery 分页查询参数
type PromptTemplateQuery struct {
	Page     int    `form:"page" json:"page"`         // 页码
	PageSize int    `form:"pageSize" json:"pageSize"` // 每页大小
	Name     string `form:"name" json:"name"`         // 模板名称筛选
	Type     string `form:"type" json:"type"`         // 模板类型筛选
	Content  string `form:"content" json:"content"`   // 内容模糊搜索
}

// PromptTemplatePageResp 分页查询响应
type PromptTemplatePageResp struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    PromptTemplatePageData `json:"data"`
}

type PromptTemplatePageData struct {
	List       []PromptTemplate `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

type Prompt struct {
	ID      int    `json:"ID"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

type Telegraph struct {
	gorm.Model
	Time            string          `json:"time"`
	DataTime        *time.Time      `json:"dataTime" gorm:"index"`
	Title           string          `json:"title" gorm:"index"`
	Content         string          `json:"content" gorm:"index"`
	SubjectTags     []string        `json:"subjects" gorm:"-:all"`
	StocksTags      []string        `json:"stocks" gorm:"-:all"`
	IsRed           bool            `json:"isRed" gorm:"index"`
	Url             string          `json:"url"`
	Source          string          `json:"source" gorm:"index"`
	TelegraphTags   []TelegraphTags `json:"tags" gorm:"-:migration;foreignKey:TelegraphId"`
	SentimentResult string          `json:"sentimentResult" gorm:"index"`
}
type TelegraphTags struct {
	gorm.Model
	TagId       uint `json:"tagId"`
	TelegraphId uint `json:"telegraphId"`
}

func (t TelegraphTags) TableName() string {
	return "telegraph_tags"
}

type Tags struct {
	gorm.Model
	Name string `json:"name"`
	Type string `json:"type"`
}

func (p Tags) TableName() string {
	return "tags"
}

func (p Telegraph) TableName() string {
	return "telegraph_list"
}

type SinaStockInfo struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Engname       string `json:"engname"`
	Tradetype     string `json:"tradetype"`
	Lasttrade     string `json:"lasttrade"`
	Prevclose     string `json:"prevclose"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Volume        string `json:"volume"`
	Currentvolume string `json:"currentvolume"`
	Amount        string `json:"amount"`
	Ticktime      string `json:"ticktime"`
	Buy           string `json:"buy"`
	Sell          string `json:"sell"`
	High52Week    string `json:"high_52week"`
	Low52Week     string `json:"low_52week"`
	Eps           string `json:"eps"`
	Dividend      string `json:"dividend"`
	StocksSum     string `json:"stocks_sum"`
	Pricechange   string `json:"pricechange"`
	Changepercent string `json:"changepercent"`
	MarketValue   string `json:"market_value"`
	PeRatio       string `json:"pe_ratio"`
}

type LongTigerRankData struct {
	ACCUMAMOUNT      float64 `json:"ACCUM_AMOUNT"`
	BILLBOARDBUYAMT  float64 `json:"BILLBOARD_BUY_AMT"`
	BILLBOARDDEALAMT float64 `json:"BILLBOARD_DEAL_AMT"`
	BILLBOARDNETAMT  float64 `json:"BILLBOARD_NET_AMT"`
	BILLBOARDSELLAMT float64 `json:"BILLBOARD_SELL_AMT"`
	CHANGERATE       float64 `json:"CHANGE_RATE"`
	CLOSEPRICE       float64 `json:"CLOSE_PRICE"`
	DEALAMOUNTRATIO  float64 `json:"DEAL_AMOUNT_RATIO"`
	DEALNETRATIO     float64 `json:"DEAL_NET_RATIO"`
	EXPLAIN          string  `json:"EXPLAIN"`
	EXPLANATION      string  `json:"EXPLANATION"`
	FREEMARKETCAP    float64 `json:"FREE_MARKET_CAP"`
	SECUCODE         string  `json:"SECUCODE" gorm:"index"`
	SECURITYCODE     string  `json:"SECURITY_CODE"`
	SECURITYNAMEABBR string  `json:"SECURITY_NAME_ABBR"`
	SECURITYTYPECODE string  `json:"SECURITY_TYPE_CODE"`
	TRADEDATE        string  `json:"TRADE_DATE" gorm:"index"`
	TURNOVERRATE     float64 `json:"TURNOVERRATE"`
}

func (l LongTigerRankData) TableName() string {
	return "long_tiger_rank"
}

type TVNews struct {
	Id         string `json:"id"`
	Title      string `json:"title"`
	Published  int    `json:"published"`
	Urgency    int    `json:"urgency"`
	Permission string `json:"permission"`
	StoryPath  string `json:"storyPath"`
	Provider   struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		LogoId string `json:"logo_id"`
	} `json:"provider"`
}
type TVNewsDetail struct {
	ShortDescription string `json:"shortDescription"`
	Tags             []struct {
		Title string `json:"title"`
		Args  []struct {
			Id    string `json:"id"`
			Value string `json:"value"`
		} `json:"args"`
	} `json:"tags"`
	Copyright string `json:"copyright"`
	Id        string `json:"id"`
	Title     string `json:"title"`
	Published int    `json:"published"`
	Urgency   int    `json:"urgency"`
	StoryPath string `json:"storyPath"`
}

type XUEQIUHot struct {
	Data struct {
		Items     []HotItem `json:"items"`
		ItemsSize int       `json:"items_size"`
	} `json:"data"`
	ErrorCode        int    `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

type HotItem struct {
	//Type         int         `json:"type" md:"-"`
	Code       string  `json:"code" md:"股票代码"`
	Name       string  `json:"name" md:"股票名称"`
	Value      float64 `json:"value" md:"热度"`
	Increment  int     `json:"increment" md:"热度变化"`
	RankChange int     `json:"rank_change" md:"排名变化"`
	//HasExist     interface{} `json:"has_exist" md:"-"`
	//Symbol       string      `json:"symbol" md:"-"`
	Percent  float64 `json:"percent" md:"涨跌幅(%)"`
	Current  float64 `json:"current" md:"股价"`
	Chg      float64 `json:"chg" md:"股价变化"`
	Exchange string  `json:"exchange" md:"交易所代码"`
	//StockType    int         `json:"stock_type" md:"-"`
	//SubType      string      `json:"sub_type" md:"-"`
	//Ad           int         `json:"ad" md:"-"`
	//AdId         interface{} `json:"ad_id" md:"-"`
	//ContentId    interface{} `json:"content_id" md:"-"`
	//Page         interface{} `json:"page" md:"-"`
	//Model        interface{} `json:"model" md:"-"`
	//Location     interface{} `json:"location" md:"-"`
	//TradeSession interface{} `json:"trade_session" md:"-"`
	//CurrentExt   interface{} `json:"current_ext" md:"-"`
	//PercentExt   interface{} `json:"percent_ext" md:"-"`
}

type HotEvent struct {
	PicSize     interface{} `json:"pic_size"`
	Tag         string      `json:"tag"`
	Id          int         `json:"id"`
	Pic         string      `json:"pic"`
	Hot         int         `json:"hot"`
	StatusCount int         `json:"status_count"`
	Content     string      `json:"content"`
}

type GDP struct {
	REPORTDATE           string  `json:"REPORT_DATE" md:"报告时间"`
	TIME                 string  `json:"TIME" md:"报告期"`
	DOMESTICLPRODUCTBASE float64 `json:"DOMESTICL_PRODUCT_BASE" md:"国内生产总值(亿元)"`
	SUMSAME              float64 `json:"SUM_SAME" md:"国内生产总值同比增长(%)"`
	FIRSTPRODUCTBASE     float64 `json:"FIRST_PRODUCT_BASE" md:"第一产业(亿元)"`
	FIRSTSAME            int     `json:"FIRST_SAME" md:"第一产业同比增长(%)"`
	SECONDPRODUCTBASE    float64 `json:"SECOND_PRODUCT_BASE" md:"第二产业(亿元)"`
	SECONDSAME           float64 `json:"SECOND_SAME" md:"第二产业同比增长(%)"`
	THIRDPRODUCTBASE     float64 `json:"THIRD_PRODUCT_BASE" md:"第三产业(亿元)"`
	THIRDSAME            float64 `json:"THIRD_SAME" md:"第三产业同比增长(%)"`
}
type CPI struct {
	REPORTDATE         string  `json:"REPORT_DATE" md:"报告时间"`
	TIME               string  `json:"TIME" md:"报告期"`
	NATIONALBASE       float64 `json:"NATIONAL_BASE" md:"全国当月"`
	NATIONALSAME       float64 `json:"NATIONAL_SAME" md:"全国当月同比增长(%)"`
	NATIONALSEQUENTIAL float64 `json:"NATIONAL_SEQUENTIAL" md:"全国当月环比增长(%)"`
	NATIONALACCUMULATE float64 `json:"NATIONAL_ACCUMULATE" md:"全国当月累计"`
	CITYBASE           float64 `json:"CITY_BASE" md:"城市当月"`
	CITYSAME           float64 `json:"CITY_SAME" md:"城市当月同比增长(%)"`
	CITYSEQUENTIAL     float64 `json:"CITY_SEQUENTIAL" md:"城市当月环比增长(%)"`
	CITYACCUMULATE     int     `json:"CITY_ACCUMULATE" md:"城市当月累计"`
	RURALBASE          float64 `json:"RURAL_BASE" md:"农村当月"`
	RURALSAME          float64 `json:"RURAL_SAME" md:"农村当月同比增长(%)"`
	RURALSEQUENTIAL    int     `json:"RURAL_SEQUENTIAL" md:"农村当月环比增长(%)"`
	RURALACCUMULATE    float64 `json:"RURAL_ACCUMULATE" md:"农村当月累计"`
}
type PPI struct {
	REPORTDATE     string  `json:"REPORT_DATE" md:"报告时间"`
	TIME           string  `json:"TIME" md:"报告期"`
	BASE           float64 `json:"BASE" md:"当月"`
	BASESAME       float64 `json:"BASE_SAME" md:"当月同比增长(%)"`
	BASEACCUMULATE float64 `json:"BASE_ACCUMULATE" md:"累计"`
}
type PMI struct {
	REPORTDATE string  `md:"报告时间" json:"REPORT_DATE"`
	TIME       string  `md:"报告期" json:"TIME"`
	MAKEINDEX  float64 `md:"制造业指数" json:"MAKE_INDEX"`
	MAKESAME   float64 `md:"制造业指数同比增长(%)" json:"MAKE_SAME"`
	NMAKEINDEX float64 `md:"非制造业" json:"NMAKE_INDEX"`
	NMAKESAME  float64 `md:"非制造业同比增长(%)" json:"NMAKE_SAME"`
}

type DCResp struct {
	Version string `json:"version"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type GDPResult struct {
	Pages int   `json:"pages"`
	Data  []GDP `json:"data"`
	Count int   `json:"count"`
}
type CPIResult struct {
	Pages int   `json:"pages"`
	Data  []CPI `json:"data"`
	Count int   `json:"count"`
}

type PPIResult struct {
	Pages int   `json:"pages"`
	Data  []PPI `json:"data"`
	Count int   `json:"count"`
}
type PMIResult struct {
	Pages int   `json:"pages"`
	Data  []PMI `json:"data"`
	Count int   `json:"count"`
}
type GDPResp struct {
	DCResp
	GDPResult GDPResult `json:"result"`
}

type CPIResp struct {
	DCResp
	CPIResult CPIResult `json:"result"`
}

type PPIResp struct {
	DCResp
	PPIResult PPIResult `json:"result"`
}
type PMIResp struct {
	DCResp
	PMIResult PMIResult `json:"result"`
}

type OldSettings struct {
	gorm.Model
	TushareToken           string `json:"tushareToken"`
	LocalPushEnable        bool   `json:"localPushEnable"`
	DingPushEnable         bool   `json:"dingPushEnable"`
	DingRobot              string `json:"dingRobot"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`

	OpenAiEnable      bool    `json:"openAiEnable"`
	OpenAiBaseUrl     string  `json:"openAiBaseUrl"`
	OpenAiApiKey      string  `json:"openAiApiKey"`
	OpenAiModelName   string  `json:"openAiModelName"`
	OpenAiMaxTokens   int     `json:"openAiMaxTokens"`
	OpenAiTemperature float64 `json:"openAiTemperature"`
	OpenAiApiTimeOut  int     `json:"openAiApiTimeOut"`
	Prompt            string  `json:"prompt"`
	CheckUpdate       bool    `json:"checkUpdate"`
	QuestionTemplate  string  `json:"questionTemplate"`
	CrawlTimeOut      int64   `json:"crawlTimeOut"`
	KDays             int64   `json:"kDays"`
	EnableDanmu       bool    `json:"enableDanmu"`
	BrowserPath       string  `json:"browserPath"`
	EnableNews        bool    `json:"enableNews"`
	DarkTheme         bool    `json:"darkTheme"`
	BrowserPoolSize   int     `json:"browserPoolSize"`
	EnableFund        bool    `json:"enableFund"`
	EnablePushNews    bool    `json:"enablePushNews"`
	SponsorCode       string  `json:"sponsorCode"`
}

func (receiver OldSettings) TableName() string {
	return "settings"
}

type ReutersNews struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Result     struct {
		ParentSectionName string `json:"parent_section_name"`
		Pagination        struct {
			Size         int    `json:"size"`
			ExpectedSize int    `json:"expected_size"`
			TotalSize    int    `json:"total_size"`
			Orderby      string `json:"orderby"`
		} `json:"pagination"`
		DateModified time.Time `json:"date_modified"`
		FetchType    string    `json:"fetch_type"`
		Articles     []struct {
			Id                          string    `json:"id"`
			CanonicalUrl                string    `json:"canonical_url"`
			Website                     string    `json:"website"`
			Web                         string    `json:"web"`
			Native                      string    `json:"native"`
			UpdatedTime                 time.Time `json:"updated_time"`
			PublishedTime               time.Time `json:"published_time"`
			ArticleType                 string    `json:"article_type"`
			DisplayMyNews               bool      `json:"display_my_news"`
			DisplayNewsletterSignup     bool      `json:"display_newsletter_signup"`
			DisplayNotifications        bool      `json:"display_notifications"`
			DisplayRelatedMedia         bool      `json:"display_related_media"`
			DisplayRelatedOrganizations bool      `json:"display_related_organizations"`
			ContentCode                 string    `json:"content_code"`
			Source                      struct {
				Name         string `json:"name"`
				OriginalName string `json:"original_name"`
			} `json:"source"`
			Title            string `json:"title"`
			BasicHeadline    string `json:"basic_headline"`
			Distributor      string `json:"distributor"`
			Description      string `json:"description"`
			PrimaryMediaType string `json:"primary_media_type,omitempty"`
			PrimaryTag       struct {
				ShortBio    string `json:"short_bio"`
				Description string `json:"description"`
				Slug        string `json:"slug"`
				Text        string `json:"text"`
				TopicUrl    string `json:"topic_url"`
				CanFollow   bool   `json:"can_follow,omitempty"`
				IsTopic     bool   `json:"is_topic,omitempty"`
			} `json:"primary_tag"`
			WordCount   int `json:"word_count"`
			ReadMinutes int `json:"read_minutes"`
			Kicker      struct {
				Path  string   `json:"path"`
				Names []string `json:"names"`
				Name  string   `json:"name,omitempty"`
			} `json:"kicker"`
			AdTopics  []string `json:"ad_topics"`
			Thumbnail struct {
				Url                   string    `json:"url"`
				Caption               string    `json:"caption,omitempty"`
				Type                  string    `json:"type"`
				ResizerUrl            string    `json:"resizer_url"`
				Location              string    `json:"location,omitempty"`
				Id                    string    `json:"id"`
				Authors               string    `json:"authors,omitempty"`
				AltText               string    `json:"alt_text"`
				Width                 int       `json:"width"`
				Height                int       `json:"height"`
				Subtitle              string    `json:"subtitle"`
				Slug                  string    `json:"slug,omitempty"`
				UpdatedAt             time.Time `json:"updated_at"`
				Company               string    `json:"company,omitempty"`
				PurchaseLicensingPath string    `json:"purchase_licensing_path,omitempty"`
			} `json:"thumbnail"`
			Authors []struct {
				Id        string `json:"id,omitempty"`
				Name      string `json:"name"`
				FirstName string `json:"first_name,omitempty"`
				LastName  string `json:"last_name,omitempty"`
				Company   string `json:"company"`
				Thumbnail struct {
					Url        string `json:"url"`
					Type       string `json:"type"`
					ResizerUrl string `json:"resizer_url"`
				} `json:"thumbnail"`
				SocialLinks []struct {
					Site string `json:"site"`
					Url  string `json:"url"`
				} `json:"social_links,omitempty"`
				Byline      string `json:"byline"`
				Description string `json:"description,omitempty"`
				TopicUrl    string `json:"topic_url,omitempty"`
				Role        string `json:"role,omitempty"`
			} `json:"authors"`
			DisplayTime   time.Time `json:"display_time"`
			ThumbnailDark struct {
				Url        string    `json:"url"`
				Type       string    `json:"type"`
				ResizerUrl string    `json:"resizer_url"`
				Id         string    `json:"id"`
				AltText    string    `json:"alt_text"`
				Width      int       `json:"width"`
				Height     int       `json:"height"`
				Subtitle   string    `json:"subtitle"`
				UpdatedAt  time.Time `json:"updated_at"`
			} `json:"thumbnail_dark,omitempty"`
		} `json:"articles"`
		Section struct {
			Id          string `json:"id"`
			AdUnitCode  string `json:"ad_unit_code"`
			Website     string `json:"website"`
			Name        string `json:"name"`
			PageTitle   string `json:"page_title"`
			CanFollow   bool   `json:"can_follow"`
			Language    string `json:"language"`
			Type        string `json:"type"`
			Advertising struct {
				Sponsored string `json:"sponsored"`
			} `json:"advertising"`
			VideoPlaylistId  string `json:"video_playlistId"`
			MobileAdUnitPath string `json:"mobile_ad_unit_path"`
			AdUnitPath       string `json:"ad_unit_path"`
			CollectionAlias  string `json:"collection_alias"`
			SectionAbout     string `json:"section_about"`
			Title            string `json:"title"`
			Personalization  struct {
				Id        string `json:"id"`
				Type      string `json:"type"`
				ShowTags  bool   `json:"show_tags"`
				CanFollow bool   `json:"can_follow"`
			} `json:"personalization"`
		} `json:"section"`
		AdUnitPath   string `json:"ad_unit_path"`
		ResponseTime int64  `json:"response_time"`
	} `json:"result"`
	Id string `json:"_id"`
}

type InteractiveAnswer struct {
	PageNo      int                        `json:"pageNo"`
	PageSize    int                        `json:"pageSize"`
	TotalRecord int                        `json:"totalRecord"`
	TotalPage   int                        `json:"totalPage"`
	Results     []InteractiveAnswerResults `json:"results"`
	Count       bool                       `json:"count"`
}

type InteractiveAnswerResults struct {
	EsId             string   `json:"esId" md:"-"`
	IndexId          string   `json:"indexId" md:"-"`
	ContentType      int      `json:"contentType" md:"-"`
	Trade            []string `json:"trade"  md:"行业名称"`
	MainContent      string   `json:"mainContent" md:"投资者提问"`
	StockCode        string   `json:"stockCode" md:"股票代码"`
	Secid            string   `json:"secid" md:"-"`
	CompanyShortName string   `json:"companyShortName" md:"股票名称"`
	CompanyLogo      string   `json:"companyLogo,omitempty" md:"-"`
	BoardType        []string `json:"boardType" md:"-"`
	PubDate          string   `json:"pubDate" md:"发布时间"`
	UpdateDate       string   `json:"updateDate" md:"-"`
	Author           string   `json:"author" md:"-"`
	AuthorName       string   `json:"authorName" md:"-"`
	PubClient        string   `json:"pubClient" md:"-"`
	AttachedId       string   `json:"attachedId" md:"-"`
	AttachedContent  string   `json:"attachedContent" md:"上市公司回复"`
	AttachedAuthor   string   `json:"attachedAuthor" md:"-"`
	AttachedPubDate  string   `json:"attachedPubDate" md:"回复时间"`
	Score            float64  `json:"score" md:"-"`
	TopStatus        int      `json:"topStatus" md:"-"`
	PraiseCount      int      `json:"praiseCount" md:"-"`
	PraiseStatus     bool     `json:"praiseStatus" md:"-"`
	FavoriteStatus   bool     `json:"favoriteStatus" md:"-"`
	AttentionCompany bool     `json:"attentionCompany" md:"-"`
	IsCheck          string   `json:"isCheck" md:"-"`
	QaStatus         int      `json:"qaStatus" md:"-"`
	PackageDate      string   `json:"packageDate" md:"-"`
	RemindStatus     bool     `json:"remindStatus" md:"-"`
	InterviewLive    bool     `json:"interviewLive" md:"-"`
}

type CailianpressWeb struct {
	Total int `json:"total"`
	List  []struct {
		Title   string `json:"title" md:"资讯标题"`
		Ctime   int    `json:"ctime" md:"资讯时间"`
		Content string `json:"content" md:"资讯内容"`
		Author  string `json:"author" md:"资讯发布者"`
	} `json:"list"`
}

type BKDict struct {
	gorm.Model  `md:"-"`
	BkCode      string `json:"bkCode" md:"行业/板块代码"`
	BkName      string `json:"bkName" md:"行业/板块名称"`
	FirstLetter string `json:"firstLetter" md:"first_letter"`
	FubkCode    string `json:"fubkCode" md:"fubk_code"`
	PublishCode string `json:"publishCode" md:"publish_code"`
}

func (b BKDict) TableName() string {
	return "bk_dict"
}

type WordAnalyze struct {
	gorm.Model
	DataTime *time.Time `json:"dataTime" gorm:"index;autoCreateTime"`
	WordFreqWithWeight
}

// WordFreqWithWeight 词频统计结果，包含权重信息
type WordFreqWithWeight struct {
	Word      string
	Frequency int
	Weight    float64
	Score     float64
}

// SentimentResult 情感分析结果类型
type SentimentResult struct {
	Score         float64       // 情感得分
	Category      SentimentType // 情感类别
	PositiveCount int           // 正面词数量
	NegativeCount int           // 负面词数量
	Description   string        // 情感描述
}

type SentimentResultAnalyze struct {
	gorm.Model
	DataTime *time.Time `json:"dataTime" gorm:"index;autoCreateTime"`
	SentimentResult
}

// SentimentType 情感类型枚举
type SentimentType int

type HotStrategy struct {
	ChgEffect bool               `json:"chgEffect"`
	Code      int                `json:"code"`
	Data      []*HotStrategyData `json:"data"`
	Message   string             `json:"message"`
}

type HotStrategyData struct {
	Chg       float64 `json:"chg" md:"平均涨幅(%)"`
	Code      string  `json:"code" md:"-"`
	HeatValue int     `json:"heatValue" md:"热度值"`
	Market    string  `json:"market" md:"-"`
	Question  string  `json:"question" md:"选股策略"`
	Rank      int     `json:"rank" md:"-"`
}

type NtfyNews struct {
	Id      string   `json:"id"`
	Time    int      `json:"time"`
	Expires int      `json:"expires"`
	Event   string   `json:"event"`
	Topic   string   `json:"topic"`
	Title   string   `json:"title"`
	Message string   `json:"message"`
	Tags    []string `json:"tags"`
	Icon    string   `json:"icon"`
}

type THSHotStrategy struct {
	Result struct {
		Num  int `json:"num"`
		List []struct {
			Author struct {
				Avatar   string `json:"avatar"`
				UserName string `json:"userName"`
				UserId   int    `json:"userId"`
			} `json:"author"`
			Property struct {
				Id          int         `json:"id"`
				Name        string      `json:"name"`
				Query       string      `json:"query"`
				Logic       string      `json:"logic"`
				BuyPosition interface{} `json:"buyPosition"`
				Ctime       string      `json:"ctime"`
				Tags        []string    `json:"tags"`
				WinRate     string      `json:"winRate"`
				AnnualYield string      `json:"annualYield"`
				Type        int         `json:"type"`
			} `json:"property"`
			Interaction struct {
				CommentNum  int  `json:"commentNum"`
				CollectNum  int  `json:"collectNum"`
				IsCollected bool `json:"isCollected"`
				IsSubscribe int  `json:"isSubscribe"`
				IsPublish   int  `json:"isPublish"`
				Pid         int  `json:"pid"`
			} `json:"interaction"`
		} `json:"list"`
	} `json:"result"`
}

type StockMoneyDataResp struct {
	Rc     int            `json:"rc"`
	Rt     int            `json:"rt"`
	Svr    int            `json:"svr"`
	Lt     int            `json:"lt"`
	Full   int            `json:"full"`
	Dlmkts string         `json:"dlmkts"`
	Data   StockMoneyData `json:"data"`
}

type StockMoneyData struct {
	Total int                  `json:"total"`
	Diff  []StockMoneyDataDiff `json:"diff"`
}

type StockMoneyDataDiff struct {
	//F1   int     `json:"f1" md:"-"`
	F12 string `json:"f12" md:"股票代码"`
	//F13  int     `json:"f13" md:"-"`
	F14  string  `json:"f14" md:"股票名称"`
	F2   float64 `json:"f2" md:"最新价"`
	F3   float64 `json:"f3" md:"今日涨跌幅(%)"`
	F62  float64 `json:"f62" md:"今日主力净额(元)"`
	F184 float64 `json:"f184" md:"今日主力净占比(%)"`
	F66  float64 `json:"f66" md:"今日超大单净额(元)"`
	F69  float64 `json:"f69" md:"今日超大单净占比(%)"`
	F72  float64 `json:"f72" md:"今日大单净额(元)"`
	F75  float64 `json:"f75" md:"今日大单净占比(%)"`
	F78  float64 `json:"f78" md:"今日中单净额(元)"`
	F81  float64 `json:"f81" md:"今日中单净占比(%)"`
	F84  float64 `json:"f84" md:"今日小单净额(元)"`
	F87  float64 `json:"f87" md:"今日小单净占比(%)"`
	//F124 int     `json:"f124" md:"f124"`
	F100 string `json:"f100" md:"所属板块"`
	F265 string `json:"f265" md:"板块代码"`
}

// MutualTop10DealResp 沪深港通十大成交股响应
type MutualTop10DealResp struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Result  MutualTop10DealData `json:"result"`
	Success bool                `json:"success"`
	Version string              `json:"version"`
}

// MutualTop10DealData 结果集
type MutualTop10DealData struct {
	Pages int                 `json:"pages"`
	Data  []MutualTop10Record `json:"data"`
	Count int                 `json:"count"`
}

// MutualTop10Record 单条互联互通十大成交股记录
// MUTUAL_TYPE 含义：001=沪股通十大成交股, 002=港股通(沪)十大成交股, 003=深股通十大成交股, 004=港股通(深)十大成交股
// 字段名参考东方财富 datacenter RPT_MUTUAL_TOP10DEAL 报表
type MutualTop10Record struct {
	MUTUALTYPE         string      `json:"MUTUAL_TYPE" md:"通道类型"`
	SECURITYCODE       string      `json:"SECURITY_CODE" md:"股票代码"`
	DERIVESECURITYCODE string      `json:"DERIVE_SECURITY_CODE" md:"完整股票代码"`
	SECURITYNAME       string      `json:"SECURITY_NAME" md:"股票名称"`
	TRADEDATE          string      `json:"TRADE_DATE" md:"交易日期"`
	CLOSEPRICE         float64     `json:"CLOSE_PRICE" md:"收盘价"`
	CHANGERATE         float64     `json:"CHANGE_RATE" md:"涨跌幅"`
	NETBUYAMT          interface{} `json:"NET_BUY_AMT" md:"净买入额"`
	RANK               int         `json:"RANK" md:"成交金额排名"`
	BUYAMT             interface{} `json:"BUY_AMT" md:"买入额"`
	SELLAMT            interface{} `json:"SELL_AMT" md:"卖出额"`
	DEALAMT            int64       `json:"DEAL_AMT" md:"成交金额"`
	DEALAMOUNT         float64     `json:"DEAL_AMOUNT" md:"总成交金额"`
	MUTUALRATIO        float64     `json:"MUTUAL_RATIO" md:"占比成交"`
	TURNOVERRATE       float64     `json:"TURNOVERRATE" md:"换手率"`
	CHANGE             float64     `json:"CHANGE" md:"涨跌额"`
}

type StockConceptInfoResp struct {
	Version string                 `json:"version"`
	Result  StockConceptInfoResult `json:"result"`
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Code    int                    `json:"code"`
}

type StockConceptInfoResult struct {
	Pages int                `json:"pages"`
	Data  []StockConceptInfo `json:"data"`
	Count int                `json:"count"`
}

type StockConceptInfo struct {
	//SECUCODE string `json:"SECUCODE" md:"完整股票代码"`
	//SECURITYCODE        string  `json:"SECURITY_CODE" md:"股票代码"`
	//SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	//NEWBOARDCODE        string  `json:"NEW_BOARD_CODE" md:"板块/概念代码"`
	BOARDNAME           string `json:"BOARD_NAME" md:"板块/概念名称"`
	SELECTEDBOARDREASON string `json:"SELECTED_BOARD_REASON" md:"板块/概念描述"`
	//ISPRECISE           string  `json:"IS_PRECISE" md:"-"`
	//BOARDRANK           int     `json:"BOARD_RANK" md:"-"`
	BOARDYIELD float64 `json:"BOARD_YIELD" md:"板块/概念涨跌幅(%)"`
	//DERIVEBOARDCODE     string  `json:"DERIVE_BOARD_CODE" md:"-"`
}

type AiRecommendStocks struct {
	gorm.Model                  `md:"-"`
	DataTime                    *time.Time `json:"dataTime" gorm:"index;autoCreateTime" md:"推荐时间"`
	ModelName                   string     `json:"modelName" md:"模型名称"`
	Rating                      string     `json:"rating" md:"评级"`
	StockCode                   string     `json:"stockCode" md:"股票代码"`
	StockName                   string     `json:"stockName" md:"股票名称"`
	BkCode                      string     `json:"bkCode" md:"行业/板块代码"`
	BkName                      string     `json:"bkName" md:"行业/板块名称"`
	StockPrice                  string     `json:"stockPrice" md:"推荐时股票价格"`
	StockCurrentPrice           string     `json:"stockCurrentPrice" md:"当前价格"`
	StockCurrentPriceTime       string     `json:"stockCurrentPriceTime" md:"当前价格时间"`
	StockClosePrice             string     `json:"stockClosePrice" md:"推荐时股票收盘价格"`
	StockPrePrice               string     `json:"stockPrePrice" md:"前一交易日股票价格"`
	RecommendReason             string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
	RecommendBuyPrice           string     `json:"recommendBuyPrice" md:"ai建议买入价范围"`
	RecommendBuyPriceMin        float64    `json:"recommendBuyPriceMin" md:"ai建议最低买入价"`
	RecommendBuyPriceMax        float64    `json:"recommendBuyPriceMax" md:"ai建议最高买入价"`
	RecommendStopProfitPrice    string     `json:"recommendStopProfitPrice" md:"ai建议止盈价范围"`
	RecommendStopProfitPriceMin float64    `json:"recommendStopProfitPriceMin" md:"ai建议最低止盈价"`
	RecommendStopProfitPriceMax float64    `json:"recommendStopProfitPriceMax" md:"ai建议最高止盈价"`
	RecommendStopLossPrice      string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
	RiskRemarks                 string     `json:"riskRemarks" md:"风险提示"`
	Remarks                     string     `json:"remarks" md:"备注"`
	EnableAlert                 bool       `json:"enableAlert" gorm:"default:false" md:"开启预警"`
}

type AiRecommendStocksMdExport struct {
	DataTime                    string  `json:"dataTime"  md:"推荐时间"`
	ModelName                   string  `json:"modelName" md:"模型名称"`
	Rating                      string  `json:"rating" md:"评级"`
	StockCode                   string  `json:"stockCode" md:"股票代码"`
	StockName                   string  `json:"stockName" md:"股票名称"`
	BkCode                      string  `json:"bkCode" md:"行业/板块代码"`
	BkName                      string  `json:"bkName" md:"行业/板块名称"`
	StockPrice                  string  `json:"stockPrice" md:"推荐时股票价格"`
	StockCurrentPrice           string  `json:"stockCurrentPrice" md:"当前价格"`
	StockCurrentPriceTime       string  `json:"stockCurrentPriceTime" md:"当前价格时间"`
	StockClosePrice             string  `json:"stockClosePrice" md:"推荐时股票收盘价格"`
	StockPrePrice               string  `json:"stockPrePrice" md:"前一交易日股票价格"`
	RecommendReason             string  `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
	RecommendBuyPrice           string  `json:"recommendBuyPrice" md:"ai建议买入价范围"`
	RecommendBuyPriceMin        float64 `json:"recommendBuyPriceMin" md:"ai建议最低买入价"`
	RecommendBuyPriceMax        float64 `json:"recommendBuyPriceMax" md:"ai建议最高买入价"`
	RecommendStopProfitPrice    string  `json:"recommendStopProfitPrice" md:"ai建议止盈价范围"`
	RecommendStopProfitPriceMin float64 `json:"recommendStopProfitPriceMin" md:"ai建议最低止盈价"`
	RecommendStopProfitPriceMax float64 `json:"recommendStopProfitPriceMax" md:"ai建议最高止盈价"`
	RecommendStopLossPrice      string  `json:"recommendStopLossPrice" md:"ai建议止损价"`
	RiskRemarks                 string  `json:"riskRemarks" md:"风险提示"`
	Remarks                     string  `json:"remarks" md:"备注"`
}

func (receiver AiRecommendStocks) TableName() string { return "ai_recommend_stocks" }

func (receiver AiRecommendStocks) ToMdExportStruct() AiRecommendStocksMdExport {
	return AiRecommendStocksMdExport{
		DataTime:                    receiver.DataTime.Format("2006-01-02 15:04:05"),
		ModelName:                   receiver.ModelName,
		Rating:                      receiver.Rating,
		StockCode:                   receiver.StockCode,
		StockName:                   receiver.StockName,
		BkCode:                      receiver.BkCode,
		BkName:                      receiver.BkName,
		StockPrice:                  receiver.StockPrice,
		StockCurrentPrice:           receiver.StockCurrentPrice,
		StockCurrentPriceTime:       receiver.StockCurrentPriceTime,
		StockClosePrice:             receiver.StockClosePrice,
		StockPrePrice:               receiver.StockPrePrice,
		RecommendReason:             receiver.RecommendReason,
		RecommendBuyPrice:           receiver.RecommendBuyPrice,
		RecommendBuyPriceMin:        receiver.RecommendBuyPriceMin,
		RecommendBuyPriceMax:        receiver.RecommendBuyPriceMax,
		RecommendStopProfitPrice:    receiver.RecommendStopProfitPrice,
		RecommendStopProfitPriceMin: receiver.RecommendStopProfitPriceMin,
		RecommendStopProfitPriceMax: receiver.RecommendStopProfitPriceMax,
		RecommendStopLossPrice:      receiver.RecommendStopLossPrice,
		RiskRemarks:                 receiver.RiskRemarks,
		Remarks:                     receiver.Remarks,
	}

}

type AiRecommendStocksQuery struct {
	Page        int    `form:"page" json:"page"`               // 页码
	PageSize    int    `form:"pageSize" json:"pageSize"`       // 每页大小
	ModelName   string `form:"modelName" json:"modelName"`     // 模型名称筛选
	StockCode   string `form:"stockCode" json:"stockCode"`     // 股票代码筛选
	StockName   string `form:"stockName" json:"stockName"`     // 股票名称筛选
	BkCode      string `form:"bkCode" json:"bkCode"`           // 板块代码筛选
	BkName      string `form:"bkName" json:"bkName"`           // 板块名称筛选
	StartDate   string `form:"startDate" json:"startDate"`     // 开始日期
	EndDate     string `form:"endDate" json:"endDate"`         // 结束日期
	EnableAlert *bool  `form:"enableAlert" json:"enableAlert"` // 预警状态筛选
}

type AiRecommendStocksPageResp struct {
	Code    int                       `json:"code"`
	Message string                    `json:"message"`
	Data    AiRecommendStocksPageData `json:"data"`
}

type AiRecommendStocksPageData struct {
	List       []AiRecommendStocks `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}

// StockFinancialInfoResp
type StockFinancialInfoResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int                            `json:"pages"`
		Data  []StockFinancialInfoRespResult `json:"data"`
		Count int                            `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// StockFinancialInfoRespResult
type StockFinancialInfoRespResult struct {
	//SECUCODE         string `json:"SECUCODE" md:"股票代码"`
	//SECURITYCODE     string `json:"SECURITY_CODE" md:"-"`
	//SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	//ORGCODE                string  `json:"ORG_CODE" md:"-"`
	//ORGTYPE                string  `json:"ORG_TYPE" md:"-"`
	REPORTDATE string `json:"REPORT_DATE" md:"报告日期"`
	//REPORTTYPE             string  `json:"REPORT_TYPE" md:"报告类型"`
	//REPORTDATENAME         string  `json:"REPORT_DATE_NAME" md:"报告类型"`
	//SECURITYTYPECODE       string  `json:"SECURITY_TYPE_CODE" md:"-"`
	//NOTICEDATE             string  `json:"NOTICE_DATE" md:"提醒日期"`
	//UPDATEDATE             string  `json:"UPDATE_DATE" md:"更新日期"`
	//CURRENCY               string  `json:"CURRENCY" md:"货币单位"`
	NETPROFIT             float64 `json:"NETPROFIT" md:"净利润(元)"`
	TOTALOPERATEINCOME    float64 `json:"TOTAL_OPERATE_INCOME" md:"营业总收入(元)"`
	TOTALASSETS           float64 `json:"TOTAL_ASSETS" md:"总资产(元)"`
	TOTALLIABILITIES      float64 `json:"TOTAL_LIABILITIES" md:"总负债(元)"`
	TOTALCURRENTASSETS    float64 `json:"TOTAL_CURRENT_ASSETS" md:"总流动资产(元)"`
	TOTALNONCURRENTASSETS float64 `json:"TOTAL_NONCURRENT_ASSETS" md:"总非流动资产(元)"`
	PARENTNETPROFIT       float64 `json:"PARENT_NETPROFIT" md:"归属于母公司股东的净利润(元)"`
	SALENPR               float64 `json:"SALE_NPR" md:"销售净利率(%)"`
	TOTALASSETSTR         float64 `json:"TOTAL_ASSETS_TR" md:"总资产周转率(%)"`
	JROA                  float64 `json:"JROA" md:"总资产收益率(加权)(%)"`
	PARENTNETPROFITRATIO  float64 `json:"PARENT_NETPROFIT_RATIO" md:"母公司净利润占比(%)"`
	//EQUITYMULTIPLIER      float64 `json:"EQUITY_MULTIPLIER" md:"权益乘数(%)"`
	ROE            float64 `json:"ROE" md:"ROE(%)"`
	DEBTASSETRATIO float64 `json:"DEBT_ASSET_RATIO" md:"负债资产比率(%)"`
	TOTALINCOME    float64 `json:"TOTAL_INCOME" md:"总收入(元)"`
	TOTALCOST      float64 `json:"TOTAL_COST" md:"总成本(元)"`
	TOTALEXPENSE   float64 `json:"TOTAL_EXPENSE" md:"总费用(元)"`
	MONETARYFUNDS  float64 `json:"MONETARYFUNDS" md:"货币资金(元)"`
	TRADEFINASSET  float64 `json:"TRADE_FINASSET" md:"交易性金融资产(元)"`
	NOTERECE       float64 `json:"NOTE_RECE" md:"应收票据(元)"`
	ACCOUNTSRECE   float64 `json:"ACCOUNTS_RECE" md:"应收账款(元)"`
	FINANCERECE    float64 `json:"FINANCE_RECE" md:"应收款项融资(元)"`
	OTHERRECE      float64 `json:"OTHER_RECE" md:"其他应收款项(元)"`
	INVENTORY      float64 `json:"INVENTORY" md:"存货(元)"`
	//CREDITORINVEST        float64 `json:"CREDITOR_INVEST" md:"CREDITORINVEST"`
	LONGEQUITYINVEST   float64 `json:"LONG_EQUITY_INVEST" md:"长期股权投资(元)"`
	INVESTREALESTATE   float64 `json:"INVEST_REALESTATE" md:"投资性房地产(元)"`
	FIXEDASSET         float64 `json:"FIXED_ASSET" md:"固定资产(元)"`
	CIP                float64 `json:"CIP" md:"在建工程(元)"`
	USERIGHTASSET      float64 `json:"USERIGHT_ASSET" md:"使用权资产(元)"`
	INTANGIBLEASSET    float64 `json:"INTANGIBLE_ASSET" md:"无形资产(元)"`
	DEVELOPEXPENSE     float64 `json:"DEVELOP_EXPENSE" md:"开发支出(元)"`
	GOODWILL           float64 `json:"GOODWILL" md:"商誉(元)"`
	LONGPREPAIDEXPENSE float64 `json:"LONG_PREPAID_EXPENSE" md:"长期待摊费用(元)"`
	DEFERTAXASSET      float64 `json:"DEFER_TAX_ASSET" md:"递延所得税资产(元)"`
	INVESTINCOME       float64 `json:"INVEST_INCOME" md:"投资收益(元)"`
	//EXCHANGEINCOME         float64 `json:"EXCHANGE_INCOME" md:"EXCHANGEINCOME"`
	FAIRVALUECHANGEINCOME float64 `json:"FAIRVALUE_CHANGE_INCOME" md:"公允价值变动收益(元)"`
	ASSETDISPOSALINCOME   float64 `json:"ASSET_DISPOSAL_INCOME" md:"资产处置收益(元)"`
	OPERATECOST           float64 `json:"OPERATE_COST" md:"经营成本(元)"`
	//SURRENDERVALUE         float64 `json:"SURRENDER_VALUE" md:"SURRENDERVALUE"`
	//NETCOMPENSATEEXPENSE   float64 `json:"NET_COMPENSATE_EXPENSE" md:"NETCOMPENSATEEXPENSE"`
	//NETCONTRACTRESERVE     float64 `json:"NET_CONTRACT_RESERVE" md:"NETCONTRACTRESERVE"`
	//POLICYBONUSEXPENSE     float64 `json:"POLICY_BONUS_EXPENSE" md:"POLICYBONUSEXPENSE"`
	OPERATETAXADD          float64 `json:"OPERATE_TAX_ADD" md:"营业税金及附加(元)"`
	INCOMETAX              float64 `json:"INCOME_TAX" md:"所得税(元)"`
	ASSETIMPAIRMENTINCOME  float64 `json:"ASSET_IMPAIRMENT_INCOME" md:"资产减值损失(新)"`
	CREDITIMPAIRMENTINCOME float64 `json:"CREDIT_IMPAIRMENT_INCOME" md:"信用减值损失(新)"`
	NONBUSINESSEXPENSE     float64 `json:"NONBUSINESS_EXPENSE" md:"营业外支出(元)"`
	FINANCEEXPENSE         float64 `json:"FINANCE_EXPENSE" md:"财务费用(元)"`
	SALEEXPENSE            float64 `json:"SALE_EXPENSE" md:"销售费用(元)"`
	MANAGEEXPENSE          float64 `json:"MANAGE_EXPENSE" md:"管理费用(元)"`
	RESEARCHEXPENSE        float64 `json:"RESEARCH_EXPENSE" md:"研发费用(元)"`
	//INTERESTNI             float64 `json:"INTEREST_NI" md:"INTERESTNI"`
	//FEECOMMISSIONNI        float64 `json:"FEE_COMMISSION_NI" md:"FEECOMMISSIONNI"`
	//EARNEDPREMIUM          float64 `json:"EARNED_PREMIUM" md:"EARNEDPREMIUM"`
	//BUSINESSMANAGEEXPENSE  float64 `json:"BUSINESS_MANAGE_EXPENSE" md:"BUSINESSMANAGEEXPENSE"`
	//OTHERCREDITORINVEST    float64 `json:"OTHER_CREDITOR_INVEST" md:"OTHERCREDITORINVEST"`
	//OTHEREQUITYINVEST     float64 `json:"OTHER_EQUITY_INVEST" md:"其他权益工具投资(元)"`
	//LONGRECE              float64 `json:"LONG_RECE" md:"长期应收款(元)"`
	//AVAILABLESALEFINASSET float64 `json:"AVAILABLE_SALE_FINASSET" md:"可售金融资产(元)"`
	//HOLDMATURITYINVEST     float64 `json:"HOLD_MATURITY_INVEST" md:"HOLDMATURITYINVEST"`
	//FEECOMMISSIONEXPENSE   float64 `json:"FEE_COMMISSION_EXPENSE" md:"FEECOMMISSIONEXPENSE"`
}

type StockHolderNumResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int                        `json:"pages"`
		Data  []StockHolderNumRespResult `json:"data"`
		Count int                        `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
type StockHolderNumRespResult struct {
	//SECUCODE           string  `json:"SECUCODE" md:"股票代码"`
	SECURITYCODE       string  `json:"SECURITY_CODE" md:"-"`
	ENDDATE            string  `json:"END_DATE" md:"报告结束日期"`
	HOLDERTOTALNUM     int     `json:"HOLDER_TOTAL_NUM" md:"股东人数(户)"`
	TOTALNUMRATIO      float64 `json:"TOTAL_NUM_RATIO" md:"较上期变化(%)"`
	AVGFREESHARES      int     `json:"AVG_FREE_SHARES" md:"人均流通股(股)"`
	AVGFREESHARESRATIO float64 `json:"AVG_FREESHARES_RATIO" md:"较上期变化(%)"`
	HOLDFOCUS          string  `json:"HOLD_FOCUS" md:"筹码集中度"`
	PRICE              float64 `json:"PRICE" md:"股价(元)"`
	AVGHOLDAMT         float64 `json:"AVG_HOLD_AMT" md:"人均持股金额(元)"`
	HOLDRATIOTOTAL     float64 `json:"HOLD_RATIO_TOTAL" md:"十大股东持股合计(%)"`
	FREEHOLDRATIOTOTAL float64 `json:"FREEHOLD_RATIO_TOTAL" md:"十大流通股东持股合计(%) "`
}

type StockHistoryMoneyDataResp struct {
	Rc     int    `json:"rc"`
	Rt     int    `json:"rt"`
	Svr    int    `json:"svr"`
	Lt     int    `json:"lt"`
	Full   int    `json:"full"`
	Dlmkts string `json:"dlmkts"`
	Data   struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

type StockMoneyDataHis struct {
	Date string `json:"date" md:"日期"`
	F2   string `json:"f2" md:"最新价"`
	F3   string `json:"f3" md:"涨跌幅(%)"`
	F62  string `json:"f62" md:"主力净额(元)"`
	F184 string `json:"f184" md:"主力净占比(%)"`
	F66  string `json:"f66" md:"超大单净额(元)"`
	F69  string `json:"f69" md:"超大单净占比(%)"`
	F72  string `json:"f72" md:"大单净额(元)"`
	F75  string `json:"f75" md:"大单净占比(%)"`
	F78  string `json:"f78" md:"中单净额(元)"`
	F81  string `json:"f81" md:"中单净占比(%)"`
	F84  string `json:"f84" md:"小单净额(元)"`
	F87  string `json:"f87" md:"小单净占比(%)"`
}

type IndustryValuationResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Count int                     `json:"count"`
		Data  []IndustryValuationData `json:"data"`
		Pages int                     `json:"pages"`
	} `json:"result"`
	Success bool   `json:"success"`
	Version string `json:"version"`
}

type IndustryValuationData struct {
	BOARDCODE            string  `json:"BOARD_CODE" md:"行业/板块代码"`
	BOARDNAME            string  `json:"BOARD_NAME" md:"行业/板块名称"`
	INDUSTRYTYPE         string  `json:"INDUSTRY_TYPE" md:"估值类型"`
	MARKETCAPVAG         float64 `json:"MARKET_CAP_VAG" md:"总市值(元)"`
	NOMARKETCAPAVAG      float64 `json:"NOMARKETCAP_A_VAG" md:"流通市值(元)"`
	NOTLIMITEDMARKETCAPA float64 `json:"NOTLIMITED_MARKETCAP_A" md:"-"`
	PBMRQ                float64 `json:"PB_MRQ" md:"市净率"`
	PCFOCFTTM            float64 `json:"PCF_OCF_TTM" md:"市现率"`
	PEGCAR               float64 `json:"PEG_CAR" md:"PEG值"`
	PELAR                float64 `json:"PE_LAR" md:"PE(静)"`
	PETTM                float64 `json:"PE_TTM" md:"PE(TTM)"`
	TOTALMARKETCAP       float64 `json:"TOTAL_MARKET_CAP" md:"-"`
	TOTALSHARES          float64 `json:"TOTAL_SHARES" md:"-"`
	TOTALSHARESVAG       float64 `json:"TOTAL_SHARES_VAG" md:"总股本(股)"`
	TRADEDATE            string  `json:"TRADE_DATE" md:"日期"`
}

type AllStocksResp struct {
	Version interface{} `json:"version"`
	Result  struct {
		Nextpage    bool          `json:"nextpage"`
		Currentpage int           `json:"currentpage"`
		Data        []StockInfo   `json:"data"`
		Config      []interface{} `json:"config"`
		Count       int           `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Url     string `json:"url"`
}

type StockInfo struct {
	SECUCODE         string `json:"SECUCODE" md:"股票代码" gorm:"index;secucode"`
	SECURITYCODE     string `json:"SECURITY_CODE" md:"股票代码(精简)" gorm:"index;securitycode"`
	SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称" gorm:"index;securitynameabbr"`
	NEWPRICE         any    `json:"NEW_PRICE" md:"最新价" gorm:"newprice"`
	CHANGERATE       any    `json:"CHANGE_RATE" md:"涨跌幅(%)" gorm:"changerate"`
	VOLUMERATIO      any    `json:"VOLUME_RATIO" md:"量比" gorm:"volumeratio"`
	HIGHPRICE        any    `json:"HIGH_PRICE" md:"最高价" gorm:"highprice"`
	LOWPRICE         any    `json:"LOW_PRICE" md:"最低价" gorm:"lowprice"`
	PRECLOSEPRICE    any    `json:"PRE_CLOSE_PRICE" md:"前一交易日收盘价" gorm:"precloseprice"`
	VOLUME           any    `json:"VOLUME" md:"成交量" gorm:"volume"`
	DEALAMOUNT       any    `json:"DEAL_AMOUNT" md:"成交额（元）" gorm:"dealamount"`
	TURNOVERRATE     any    `json:"TURNOVERRATE" md:"换手率(%)" gorm:"turnoverrate"`
	MARKET           string `json:"MARKET" md:"交易所" gorm:"index;market"`
	CONCEPT          any    `json:"CONCEPT" md:"所属概念" gorm:"index;concept"`
	INDUSTRY         string `json:"INDUSTRY" md:"所属行业" gorm:"index;industry"`
	MAXTRADEDATE     string `json:"MAX_TRADE_DATE" md:"数据日期" gorm:"index;maxtradedate"`
}

func (receiver StockInfo) ToAllStockInfo() AllStockInfo {
	return AllStockInfo{
		SECUCODE:         receiver.SECUCODE,
		SECURITYCODE:     receiver.SECURITYCODE,
		SECURITYNAMEABBR: receiver.SECURITYNAMEABBR,
		NEWPRICE:         convertor.ToString(receiver.NEWPRICE),
		CHANGERATE:       convertor.ToString(receiver.CHANGERATE),
		VOLUMERATIO:      convertor.ToString(receiver.VOLUMERATIO),
		HIGHPRICE:        convertor.ToString(receiver.HIGHPRICE),
		LOWPRICE:         convertor.ToString(receiver.LOWPRICE),
		PRECLOSEPRICE:    convertor.ToString(receiver.PRECLOSEPRICE),
		VOLUME:           convertor.ToString(receiver.VOLUME),
		DEALAMOUNT:       convertor.ToString(receiver.DEALAMOUNT),
		TURNOVERRATE:     convertor.ToString(receiver.TURNOVERRATE),
		MARKET:           receiver.MARKET,
		CONCEPT:          convertor.ToString(receiver.CONCEPT),
		INDUSTRY:         receiver.INDUSTRY,
		MAXTRADEDATE:     receiver.MAXTRADEDATE,
	}

}

type AllStockInfo struct {
	gorm.Model
	SECUCODE         string `json:"SECUCODE" md:"股票代码" gorm:"index;secucode"`
	SECURITYCODE     string `json:"SECURITY_CODE" md:"股票代码(精简)" gorm:"index;securitycode"`
	SECURITYNAMEABBR string `json:"SECURITY_NAME_ABBR" md:"股票名称" gorm:"index;securitynameabbr"`
	NEWPRICE         string `json:"NEW_PRICE" md:"最新价" gorm:"newprice"`
	CHANGERATE       string `json:"CHANGE_RATE" md:"涨跌幅(%)" gorm:"changerate"`
	VOLUMERATIO      string `json:"VOLUME_RATIO" md:"量比" gorm:"volumeratio"`
	HIGHPRICE        string `json:"HIGH_PRICE" md:"最高价" gorm:"highprice"`
	LOWPRICE         string `json:"LOW_PRICE" md:"最低价" gorm:"lowprice"`
	PRECLOSEPRICE    string `json:"PRE_CLOSE_PRICE" md:"前一交易日收盘价" gorm:"precloseprice"`
	VOLUME           string `json:"VOLUME" md:"成交量" gorm:"volume"`
	DEALAMOUNT       string `json:"DEAL_AMOUNT" md:"成交额（元）" gorm:"dealamount"`
	TURNOVERRATE     string `json:"TURNOVERRATE" md:"换手率(%)" gorm:"turnoverrate"`
	MARKET           string `json:"MARKET" md:"交易所" gorm:"index;market"`
	CONCEPT          string `json:"CONCEPT" md:"所属概念" gorm:"index;concept"`
	INDUSTRY         string `json:"INDUSTRY" md:"所属行业" gorm:"index;industry"`
	MAXTRADEDATE     string `json:"MAX_TRADE_DATE" md:"数据日期" gorm:"index;maxtradedate"`
}

func (s AllStockInfo) TableName() string {
	return "all_stock_info"
}

type TechnicalIndicators struct {
	MACDGOLDENFORK     bool `json:"MACD_GOLDEN_FORK" operator:"="`
	KDJGOLDENFORK      bool `json:"KDJ_GOLDEN_FORK" operator:"="`
	BREAKTHROUGH       bool `json:"BREAK_THROUGH" operator:"="`
	LOWFUNDSINFLOW     bool `json:"LOW_FUNDS_INFLOW" operator:"="`
	HIGHFUNDSOUTFLOW   bool `json:"HIGH_FUNDS_OUTFLOW" operator:"="`
	BREAKUPMA5DAYS     bool `json:"BREAKUP_MA_5DAYS" operator:"="`
	LONGAVGARRAY       bool `json:"LONG_AVG_ARRAY" operator:"="`
	SHORTAVGARRAY      bool `json:"SHORT_AVG_ARRAY" operator:"="`
	UPPERLARGEVOLUME   bool `json:"UPPER_LARGE_VOLUME" operator:"="`
	DOWNNARROWVOLUME   bool `json:"DOWN_NARROW_VOLUME" operator:"="`
	ONEDAYANGLINE      bool `json:"ONE_DAYANG_LINE" operator:"="`
	TWODAYANGLINES     bool `json:"TWO_DAYANG_LINES" operator:"="`
	RISESUN            bool `json:"RISE_SUN" operator:"="`
	POWERFULGUN        bool `json:"POWER_FULGUN" operator:"="`
	RESTOREJUSTICE     bool `json:"RESTORE_JUSTICE" operator:"="`
	DOWN7DAYS          bool `json:"DOWN_7DAYS" operator:"="`
	UPPER8DAYS         bool `json:"UPPER_8DAYS" operator:"="`
	UPPER9DAYS         bool `json:"UPPER_9DAYS" operator:"="`
	UPPER4DAYS         bool `json:"UPPER_4DAYS" operator:"="`
	HEAVENRULE         bool `json:"HEAVEN_RULE" operator:"="`
	UPSIDEVOLUME       bool `json:"UPSIDE_VOLUME" operator:"="`
	BEARISHENGULFING   bool `json:"BEARISH_ENGULFING" operator:"="`
	REVERSINGHAMMER    bool `json:"REVERSING_HAMMER" operator:"="`
	SHOOTINGSTAR       bool `json:"SHOOTING_STAR" operator:"="`
	EVENINGSTAR        bool `json:"EVENING_STAR" operator:"="`
	FIRSTDAWN          bool `json:"FIRST_DAWN" operator:"="`
	PREGNANT           bool `json:"PREGNANT" operator:"="`
	BLACKCLOUDTOPS     bool `json:"BLACK_CLOUD_TOPS" operator:"="`
	MORNINGSTAR        bool `json:"MORNING_STAR" operator:"="`
	NARROWFINISH       bool `json:"NARROW_FINISH" operator:"="`
	UPP_DAYS           int  `json:"UPP_DAYS" operator:">="`
	CONCERN_RANK_7DAYS int  `json:"CONCERN_RANK_7DAYS" operator:"<="`
	UPNDAY             int  `json:"UPNDAY" operator:">="`
	DOWNNDAY           int  `json:"DOWNNDAY" operator:">="`
}

type SecuritiesCompanyOpinionResp struct {
	Hits        int                             `json:"hits"`
	Size        int                             `json:"size"`
	Data        []*SecuritiesCompanyOpinionData `json:"data"`
	TotalPage   int                             `json:"TotalPage"`
	AuthorName  string                          `json:"authorName"`
	PageNo      int                             `json:"pageNo"`
	OrgSName    string                          `json:"orgSName"`
	CurrentYear int                             `json:"currentYear"`
}

type SecuritiesCompanyOpinionData struct {
	Id           int64       `json:"id"`
	Title        string      `json:"title"`
	Author       interface{} `json:"author"`
	OrgName      string      `json:"orgName"`
	OrgCode      string      `json:"orgCode"`
	OrgSName     string      `json:"orgSName"`
	PublishDate  string      `json:"publishDate"`
	EncodeUrl    string      `json:"encodeUrl"`
	Researcher   string      `json:"researcher"`
	Market       string      `json:"market"`
	IndustryCode string      `json:"industryCode"`
	IndustryName string      `json:"industryName"`
	AuthorID     interface{} `json:"authorID"`
	Count        int         `json:"count"`
	OrgType      string      `json:"orgType"`
	OpinionData  string      `json:"opinionData"`
}

type CronTask struct {
	ID            uint       `json:"id" gorm:"primarykey"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Name          string     `json:"name" gorm:"size:255;not null"`
	CronExpr      string     `json:"cronExpr" gorm:"size:100;not null"`
	TaskType      string     `json:"taskType" gorm:"size:50;not null"` // stock_analysis, fund_analysis, news_fetch, custom
	Target        string     `json:"target" gorm:"size:255"`           // 股票代码或其他目标
	Params        string     `json:"params" gorm:"type:text"`          // JSON 格式的任务参数
	Enable        bool       `json:"enable" gorm:"default:true"`
	LastRunAt     *time.Time `json:"lastRunAt"`
	NextRunAt     *time.Time `json:"nextRunAt"`
	RunCount      int64      `json:"runCount" gorm:"default:0"`
	Status        string     `json:"status" gorm:"size:20;default:active"` // active, paused, error
	Description   string     `json:"description" gorm:"size:500"`
	LastRunResult string     `json:"lastRunResult" gorm:"size:500"`
}

func (CronTask) TableName() string {
	return "cron_tasks"
}

type CronTaskQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	TaskType string `json:"taskType"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

type CronTaskPageResp struct {
	Total int        `json:"total"`
	Data  []CronTask `json:"data"`
}

type CronTaskPageData struct {
	List       []CronTask `json:"list"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// AiAssistantSession 悬浮 AI 助手会话表，保存最近一次对话
type AiAssistantSession struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	SessionId string    `json:"sessionId" gorm:"index;size:64"` // 会话ID，时间戳格式
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Messages  string    `json:"messages" gorm:"type:text"` // JSON 数组，每项 { role, content, reasoning, time, modelName }
}

func (AiAssistantSession) TableName() string {
	return "ai_assistant_sessions"
}

// AiAssistantMessage 单条消息，供前后端 JSON 序列化
type AiAssistantMessage struct {
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	Reasoning   string          `json:"reasoning"`
	Time        string          `json:"time"`                // 消息时间，格式如 "2006-01-02 15:04:05"
	ModelName   string          `json:"modelName,omitempty"` // 助手回复所用模型展示名（如配置名称 + 模型名）
	ToolCalls   json.RawMessage `json:"toolCalls,omitempty"`
	ToolResults json.RawMessage `json:"toolResults,omitempty"`
	Timeline    json.RawMessage `json:"timeline,omitempty"` // 前端按时间序存储的正文/工具块
}

// AiAssistantSessionResp 会话查询响应
type AiAssistantSessionResp struct {
	Messages  []AiAssistantMessage `json:"messages"`
	SessionId string               `json:"sessionId"`
}

type StockRZRQInfoResp struct {
	Version string `json:"version"`
	Result  struct {
		Pages int             `json:"pages"`
		Data  []StockRZRQInfo `json:"data"`
		Count int             `json:"count"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type StockRZRQInfo struct {
	//MARKETNAME string `json:"MARKET_NAME" md:"交易所"`
	//MARKETCODE string `json:"MARKET_CODE" md:"交易所代码"`
	TRADEDATE string `json:"TRADE_DATE" md:"交易日期"`
	//SECURITYCODE     string  `json:"SECURITY_CODE" md:"股票代码"`
	//SECUCODE         string  `json:"SECUCODE" md:"股票代码"`
	//SECURITYNAMEABBR string  `json:"SECURITY_NAME_ABBR" md:"股票名称"`
	FINBALANCE     int64   `json:"FIN_BALANCE" md:"融资余额(元)"`
	FINBUYAMT      int     `json:"FIN_BUY_AMT" md:"融资买入额(元)"`
	FINREPAYAMT    int     `json:"FIN_REPAY_AMT" md:"融资还款额(元)"`
	LOANBALANCE    float64 `json:"LOAN_BALANCE" md:"融券余额(元)"`
	LOANSELLVOL    int     `json:"LOAN_SELL_VOL" md:"融券卖出量(股)"`
	LOANREPAYVOL   int     `json:"LOAN_REPAY_VOL" md:"融券还款量(股)"`
	MARGINBALANCE  float64 `json:"MARGIN_BALANCE" md:"两融余额(股)"`
	LOANBALANCEVOL int     `json:"LOAN_BALANCE_VOL" md:"融券余量(股)"`
	FINNETBUYAMT   int     `json:"FIN_NETBUY_AMT" md:"融资净买入额(元)"`
}

// GlobalStockIndex 全球股市指数缓存表
type GlobalStockIndex struct {
	gorm.Model
	Code       string `json:"code" gorm:"index"`   // 指数代码，如 DJI, IXIC, 000001
	Name       string `json:"name" md:"指数名称"`      // 指数名称，如 道琼斯, 上证指数
	Location   string `json:"location" md:"地区"`    // 地区，如 纽约, 上海
	Qtcode     string `json:"qtcode" gorm:"index"` // 行情代码，如 s_usDJI, sh000001
	State      string `json:"state" md:"状态"`       // 状态：open(开盘), close(收盘), break(休市)
	Zdf        string `json:"zdf" md:"涨跌幅(%)"`     // 涨跌幅百分比
	Zxj        string `json:"zxj" md:"最新点位"`       // 最新点位/价格
	Img        string `json:"img" md:"图标URL"`      // 图标URL
	Region     string `json:"region" gorm:"index"` // 区域：america(美洲), asia(亚洲), europe(欧洲), common(重点关注), other(其他)
	RegionName string `json:"regionName" md:"区域名称"`
}

func (GlobalStockIndex) TableName() string {
	return "global_stock_index"
}

type MarketStatistic struct {
	ID            uint      `json:"id" gorm:"primarykey"`
	DataDate      string    `json:"dataDate" gorm:"index;size:10"` // 日期 YYYY-MM-DD
	DataTime      string    `json:"dataTime" gorm:"index;size:8"`  // 时间 HH:MM
	UpCount       int       `json:"upCount"`                       // 上涨家数
	DownCount     int       `json:"downCount"`                     // 下跌家数
	UpRatio       float64   `json:"upRatio"`                       // 涨跌比(上涨家数占比)
	UpDownRatio   float64   `json:"upDownRatio"`                   // 市场情绪指标(上涨家数/下跌家数)
	SentimentDesc string    `json:"sentimentDesc" gorm:"size:20"`  // 市场情绪描述
	LimitUp       int       `json:"limitUp"`                       // 涨停家数
	LimitDown     int       `json:"limitDown"`                     // 跌停家数
	LimitRatio    float64   `json:"limitRatio"`                    // 涨跌停比(涨停/跌停)
	ShUpCount     int       `json:"shUpCount"`                     // 上海上涨家数
	ShDownCount   int       `json:"shDownCount"`                   // 上海下跌家数
	SzUpCount     int       `json:"szUpCount"`                     // 深圳上涨家数
	SzDownCount   int       `json:"szDownCount"`                   // 深圳下跌家数
	CreatedAt     time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (MarketStatistic) TableName() string {
	return "market_statistic"
}

type MCPServer struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Description string    `json:"description" gorm:"size:500"`
	URL         string    `json:"url" gorm:"size:500"`
	Type        string    `json:"type" gorm:"size:20;default:streamable-http"`
	Headers     string    `json:"headers" gorm:"type:text"`
	Command     string    `json:"command" gorm:"size:500"`
	Args        string    `json:"args" gorm:"type:text"`
	Env         string    `json:"env" gorm:"type:text"`
	Enable      bool      `json:"enable" gorm:"default:true"`
	Status      string    `json:"status" gorm:"size:20;default:stopped"`
	TestResult  string    `json:"testResult" gorm:"size:500"`
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

type MCPServerQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

type MCPServerPageResp struct {
	Total int         `json:"total"`
	Data  []MCPServer `json:"data"`
}

type MCPServerTool struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MCPServerID  uint      `json:"mcpServerId" gorm:"index;not null"`
	ToolName     string    `json:"toolName" gorm:"size:255;not null"`
	Description  string    `json:"description" gorm:"type:text"`
	ParamsSchema string    `json:"paramsSchema" gorm:"type:text"`
}

func (MCPServerTool) TableName() string {
	return "mcp_server_tools"
}

type Skill struct {
	ID              uint      `json:"id" gorm:"primarykey"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Name            string    `json:"name" gorm:"size:255;not null"`
	Description     string    `json:"description" gorm:"size:500"`
	Category        string    `json:"category" gorm:"size:50"`
	SystemPrompt    string    `json:"systemPrompt" gorm:"type:text"`
	Examples        string    `json:"examples" gorm:"type:text"`
	TriggerKeywords string    `json:"triggerKeywords" gorm:"size:500"`
	MCPServerIDs    string    `json:"mcpServerIds" gorm:"size:500"`
	Enable          bool      `json:"enable" gorm:"default:true"`
	SortOrder       int       `json:"sortOrder" gorm:"default:0"`
}

func (Skill) TableName() string {
	return "skills"
}

type SkillQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Enable   *bool  `json:"enable"`
}

type SkillPageResp struct {
	Total int     `json:"total"`
	Data  []Skill `json:"data"`
}

type CustomStrategy struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Query       string    `json:"query" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"size:500"`
	SortOrder   int       `json:"sortOrder" gorm:"default:0"`
}

func (CustomStrategy) TableName() string {
	return "custom_strategies"
}

type CustomStrategyQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
}

type CustomStrategyPageData struct {
	List       []CustomStrategy `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

// BKFundFlow 板块资金流向数据
type BKFundFlow struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Code      string    `json:"code" gorm:"size:20;index:idx_bk_code_time"`     // 板块代码 BK0475
	Name      string    `json:"name" gorm:"size:50"`                            // 板块名称
	NetInflow int64     `json:"netInflow"`                                      // 主力净流入金额（元）
	SnapTime  string    `json:"snapTime" gorm:"size:19;index:idx_bk_code_time"` // 快照时间 YYYY-MM-DD HH:MM:SS
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (BKFundFlow) TableName() string {
	return "bk_fund_flow"
}

// BKFundFlowPoint 板块资金流向数据点（前端折线图用）
type BKFundFlowPoint struct {
	SnapTime  string `json:"snapTime"`
	NetInflow int64  `json:"netInflow"`
}

// ConceptFundFlow 概念资金流向数据
type ConceptFundFlow struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Code      string    `json:"code" gorm:"size:20;index:idx_concept_code_time"`     // 概念代码
	Name      string    `json:"name" gorm:"size:50"`                                 // 概念名称
	NetInflow int64     `json:"netInflow"`                                           // 主力净流入金额（元）
	SnapTime  string    `json:"snapTime" gorm:"size:19;index:idx_concept_code_time"` // 快照时间 YYYY-MM-DD HH:MM:SS
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (ConceptFundFlow) TableName() string {
	return "concept_fund_flow"
}

// ConceptFundFlowPoint 概念资金流向数据点（前端折线图用）
type ConceptFundFlowPoint struct {
	SnapTime  string `json:"snapTime"`
	NetInflow int64  `json:"netInflow"`
}
