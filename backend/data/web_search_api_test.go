package data

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/db"
)

func init() {
	if os.Getenv("TEST_DB_SKIP_INIT") == "1" {
		return
	}
	db.Init("../../data/stock.db")
}

func TestWebSearchApi_Search(t *testing.T) {
	defer GetBrowserManager().ResetBrowser()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	results := api.Search("贵州茅台股票", 5)
	t.Logf("Search results count: %d", len(results))
	for i, r := range results {
		t.Logf("%d. %s - %s", i+1, r.Source, r.Snippet)
	}
	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}
}

func TestWebSearchApi_SearchToMarkdown(t *testing.T) {
	defer GetBrowserManager().ResetBrowser()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	md := api.SearchToMarkdown("贵州茅台股票", 3)
	t.Logf("Markdown result:\n%s", md)
	if md == "" || md == "未找到相关搜索结果" {
		t.Error("Expected search results in markdown format")
	}
}

func TestWebSearchApi_SearchToJson(t *testing.T) {
	defer GetBrowserManager().ResetBrowser()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	json := api.SearchToJson("KDJ指标", 3)
	t.Logf("JSON result:\n%s", json)
	if json == "" || json == "未找到相关搜索结果" {
		t.Error("Expected search results in JSON format")
	}
}

func TestWebSearchApi_Bing(t *testing.T) {
	defer GetBrowserManager().ResetBrowser()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	results := api.Search("A股今日行情", 5)
	bingResults := 0
	for _, r := range results {
		if r.Source == "Bing" {
			bingResults++
		}
	}
	t.Logf("Bing results count: %d", bingResults)
	for i, r := range results {
		if r.Source == "Bing" {
			t.Logf("Result %d: %s - %s", i+1, r.Title, r.Url)
		}
	}
	if bingResults == 0 {
		t.Log("Warning: No Bing search results found")
	}
}

func TestWebSearchApi_Baidu(t *testing.T) {
	defer GetBrowserManager().ResetBrowser()

	path := GetSettingConfig().BrowserPath
	if path == "" {
		t.Skip("BrowserPath not configured")
	}

	api := NewWebSearchApi(30)
	results := api.Search("股票入门基础知识", 10)
	baiduResults := 0
	for _, r := range results {
		if r.Source == "Baidu" {
			baiduResults++
		}
	}
	t.Logf("Baidu results count: %d", baiduResults)
	for i, r := range results {
		if r.Source == "Baidu" {
			t.Logf("Result %d: %s - %s", i+1, r.Title, r.Url)
		}
	}
	if baiduResults == 0 {
		t.Log("Warning: No Baidu search results found")
	}
}

func TestWebSearchApi_NewWebSearchApi(t *testing.T) {
	api1 := NewWebSearchApi(0)
	if api1.timeout != 30 {
		t.Errorf("Expected default timeout 30, got %d", api1.timeout)
	}

	api2 := NewWebSearchApi(60)
	if api2.timeout != 60 {
		t.Errorf("Expected timeout 60, got %d", api2.timeout)
	}

	api3 := NewWebSearchApi(-10)
	if api3.timeout != 30 {
		t.Errorf("Expected default timeout 30 for negative input, got %d", api3.timeout)
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "html entities",
			input:    "&nbsp;Hello&amp;World&quot;",
			expected: "Hello&World\"",
		},
		{
			name:     "multiple spaces",
			input:    "Hello   World",
			expected: "Hello World",
		},
		{
			name:     "carriage returns",
			input:    "Line1\r\nLine2\r\r",
			expected: "Line1\nLine2",
		},
		{
			name:     "multiple newlines",
			input:    "Line1\n\n\n\nLine2",
			expected: "Line1\n\nLine2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanText(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRandomSleep(t *testing.T) {
	for i := 0; i < 10; i++ {
		duration := randomSleep(100, 200)
		if duration < 100*time.Millisecond || duration >= 200*time.Millisecond {
			t.Errorf("randomSleep(100, 200) returned %v, expected between 100ms and 199ms", duration)
		}
	}

	duration := randomSleep(100, 100)
	if duration != 100*time.Millisecond {
		t.Errorf("randomSleep(100, 100) should return 100ms, got %v", duration)
	}
}

func TestParseBingResultsWithGoquery(t *testing.T) {
	html := `
		<ul id="b_results">
			<li class="b_algo">
				<h2><a href="https://example.com/1">Test Title 1</a></h2>
				<div class="b_caption"><p>Test snippet 1</p></div>
			</li>
			<li class="b_algo">
				<h2><a href="https://example.com/2">Test Title 2</a></h2>
				<div class="b_caption"><p>Test snippet 2</p></div>
			</li>
		</ul>
	`

	api := NewWebSearchApi(30)
	results := api.parseBingResultsWithGoquery(html, 5)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
		return
	}

	if results[0].Title != "Test Title 1" {
		t.Errorf("Expected title 'Test Title 1', got %q", results[0].Title)
	}

	if results[0].Url != "https://example.com/1" {
		t.Errorf("Expected URL 'https://example.com/1', got %q", results[0].Url)
	}

	if results[0].Snippet != "Test snippet 1" {
		t.Errorf("Expected snippet 'Test snippet 1', got %q", results[0].Snippet)
	}
}

func TestParseBaiduResultsWithGoquery(t *testing.T) {
	html := `
		<div id="content_left">
			<div class="result">
				<h3 class="t"><a href="https://example.com/1">Test Title 1</a></h3>
				<div class="c-abstract">Test snippet 1</div>
			</div>
			<div class="c-container">
				<h3><a href="https://example.com/2">Test Title 2</a></h3>
				<span class="content-right">Test snippet 2</span>
			</div>
		</div>
	`

	api := NewWebSearchApi(30)
	results := api.parseBaiduResultsWithGoquery(html, 5)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
		return
	}

	if results[0].Title != "Test Title 1" {
		t.Errorf("Expected title 'Test Title 1', got %q", results[0].Title)
	}

	if results[0].Url != "https://example.com/1" {
		t.Errorf("Expected URL 'https://example.com/1', got %q", results[0].Url)
	}
}
