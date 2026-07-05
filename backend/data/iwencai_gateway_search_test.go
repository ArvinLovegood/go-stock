package data

import (
	"testing"
)

func TestIwencaiGatewaySearchNoAuth(t *testing.T) {
	api := NewIwencaiAPI()
	result, err := api.searchComprehensiveGateway("news", "最新资讯", 20)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected non-empty data")
	}

	for _, item := range result.Data {
		if item.Title == "" || item.URL == "" {
			t.Fatalf("item missing title or url: %+v", item)
		}
	}
	t.Logf("news gateway: count=%d", len(result.Data))

	for i, item := range result.Data {
		if i >= 3 {
			break
		}
		t.Logf("[%d] %s | %s \n%s", i+1, item.Title, item.PublishDate, item.Summary)
	}
}

func TestIwencaiSearchNewsViaGateway(t *testing.T) {
	api := NewIwencaiAPI()
	result, err := api.SearchNews("机器人龙头")
	if err != nil {
		t.Fatalf("SearchNews failed: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected non-empty data")
	}
	t.Logf("SearchNews: count=%d first=%q", len(result.Data), result.Data[0].Title)
}

func TestIwencaiSearchReportViaGateway(t *testing.T) {
	api := NewIwencaiAPI()
	result, err := api.SearchReport("机器人龙头")
	if err != nil {
		t.Fatalf("SearchReport failed: %v", err)
	}
	t.Logf("SearchReport: count=%d", len(result.Data))
}

func TestIwencaiSearchAnnouncementViaGateway(t *testing.T) {
	api := NewIwencaiAPI()
	result, err := api.SearchAnnouncement("机器人龙头")
	if err != nil {
		t.Fatalf("SearchAnnouncement failed: %v", err)
	}
	t.Logf("SearchAnnouncement: count=%d", len(result.Data))
}

func TestIwencaiSearchInvestorViaGateway(t *testing.T) {
	api := NewIwencaiAPI()
	result, err := api.SearchInvestor("机器人龙头")
	if err != nil {
		t.Fatalf("SearchInvestor failed: %v", err)
	}
	t.Logf("SearchInvestor: count=%d", len(result.Data))
}

func TestIwencaiGatewaySearchEmptyQueryRejected(t *testing.T) {
	api := NewIwencaiAPI()
	_, err := api.searchComprehensiveGateway("news", "", 5)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestIwencaiSearchNewsToMarkdownViaGateway(t *testing.T) {
	api := NewIwencaiAPI()
	md := api.SearchNewsToMarkdown("机器人龙头")
	if md == "" {
		t.Fatal("expected markdown output")
	}
	t.Logf("markdown prefix: %.200s", md)
}
