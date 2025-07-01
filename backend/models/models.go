package models

import (
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
	"time"
)

// @Author spark
// @Date 2025/2/6 15:25
// @Desc
//-----------------------------------------------------------------------------------

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

type VersionInfo struct {
	gorm.Model
	Version        string                `json:"version"`
	Content        string                `json:"content"`
	Icon           string                `json:"icon"`
	Alipay         string                `json:"alipay"`
	Wxpay          string                `json:"wxpay"`
	BuildTimeStamp int64                 `json:"buildTimeStamp"`
	IsDel          soft_delete.DeletedAt `gorm:"softDelete:flag"`
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
}

func (receiver StockInfoUS) TableName() string {
	return "stock_base_info_us"
}

type Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
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

type Prompt struct {
	ID      int    `json:"ID"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

type Telegraph struct {
	gorm.Model
	Time            string          `json:"time"`
	Content         string          `json:"content"`
	SubjectTags     []string        `json:"subjects" gorm:"-:all"`
	StocksTags      []string        `json:"stocks" gorm:"-:all"`
	IsRed           bool            `json:"isRed"`
	Url             string          `json:"url"`
	Source          string          `json:"source"`
	TelegraphTags   []TelegraphTags `json:"tags" gorm:"-:migration;foreignKey:TelegraphId"`
	SentimentResult string          `json:"sentimentResult" gorm:"-:all"`
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

type XUEQIUHot struct {
	Data struct {
		Items     []HotItem `json:"items"`
		ItemsSize int       `json:"items_size"`
	} `json:"data"`
	ErrorCode        int    `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

type HotItem struct {
	Type         int         `json:"type"`
	Code         string      `json:"code"`
	Name         string      `json:"name"`
	Value        float64     `json:"value"`
	Increment    int         `json:"increment"`
	RankChange   int         `json:"rank_change"`
	HasExist     interface{} `json:"has_exist"`
	Symbol       string      `json:"symbol"`
	Percent      float64     `json:"percent"`
	Current      float64     `json:"current"`
	Chg          float64     `json:"chg"`
	Exchange     string      `json:"exchange"`
	StockType    int         `json:"stock_type"`
	SubType      string      `json:"sub_type"`
	Ad           int         `json:"ad"`
	AdId         interface{} `json:"ad_id"`
	ContentId    interface{} `json:"content_id"`
	Page         interface{} `json:"page"`
	Model        interface{} `json:"model"`
	Location     interface{} `json:"location"`
	TradeSession interface{} `json:"trade_session"`
	CurrentExt   interface{} `json:"current_ext"`
	PercentExt   interface{} `json:"percent_ext"`
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
