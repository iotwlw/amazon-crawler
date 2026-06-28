package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildASINInspectionItems(t *testing.T) {
	req := ASINInspectionRequest{
		Domain: "www.amazon.com.mx",
		Items: []ASINInspectionRequestItem{
			{ASIN: "B0FNMPQSJC"},
			{URL: "https://www.amazon.com/dp/B0DWWWP4FF?ref_=x&th=1"},
			{Original: "B0FNMPQSJC"},
		},
	}

	items, err := buildASINInspectionItems(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	assertEqual(t, "first domain", items[0].Domain, "www.amazon.com.mx")
	assertEqual(t, "first url", items[0].URL, "https://www.amazon.com.mx/dp/B0FNMPQSJC")
	assertEqual(t, "second domain", items[1].Domain, "www.amazon.com")
	assertEqual(t, "second url", items[1].URL, "https://www.amazon.com/dp/B0DWWWP4FF?th=1")
}

func TestBuildASINInspectionItemsRejectsInvalidInput(t *testing.T) {
	_, err := buildASINInspectionItems(ASINInspectionRequest{
		Domain: "www.amazon.com",
		Items:  []ASINInspectionRequestItem{{ASIN: "not-an-asin"}},
	})
	if err == nil {
		t.Fatal("expected invalid asin error")
	}
	if !strings.Contains(err.Error(), "无法识别") {
		t.Fatalf("err = %v", err)
	}
}

func TestLinkInspectionResultToAPIItemIncludesOriginalAndActualASIN(t *testing.T) {
	apiItem := linkInspectionResultToAPIItem(LinkInspectionResult{
		Item: LinkInspectionItem{
			Original: "https://www.amazon.com/dp/B0B6FZ1R2L",
			URL:      "https://www.amazon.com/dp/B0B6FZ1R2L",
			ASIN:     "B0B6FZ1R2L",
			Domain:   "www.amazon.com",
		},
		Product:         "Lightdot 2 Pack 150W Wall Pack LED Exterior Light",
		ASIN:            "B0DKF7HNZX",
		Price:           variantPriceStatus,
		Coupon:          " ",
		IsDeal:          "Deal",
		PrimeExclusive:  " ",
		DisplayDiscount: " ",
		Rating:          "4.3",
		ReviewCount:     7,
		Choice:          "Amazon's  Choice",
	}, "2026-06-06T00:00:00Z")

	assertEqual(t, "status", apiItem.Status, "success")
	assertEqual(t, "original asin", apiItem.OriginalASIN, "B0B6FZ1R2L")
	assertEqual(t, "actual asin", apiItem.ASIN, "B0DKF7HNZX")
	assertEqual(t, "price", apiItem.Price, variantPriceStatus)
	assertEqual(t, "choice", apiItem.ChoiceBadge, "Amazon's  Choice")
	if apiItem.ReviewCount != 7 {
		t.Fatalf("review count = %d", apiItem.ReviewCount)
	}
}

func TestCheckCrawlerToken(t *testing.T) {
	t.Setenv("CRAWLER_API_TOKEN", "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/asin-inspection", nil)
	rr := httptest.NewRecorder()
	if checkCrawlerToken(rr, req) {
		t.Fatal("expected request without token to fail")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/asin-inspection", nil)
	req.Header.Set("X-Crawler-Token", "secret")
	rr = httptest.NewRecorder()
	if !checkCrawlerToken(rr, req) {
		t.Fatal("expected request with token to pass")
	}
}
