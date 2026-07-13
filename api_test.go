package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthIdentifiesInspectionServiceAndContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var response APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("code = %d, want 0", response.Code)
	}
	assertEqual(t, "inspection schema version", response.InspectionSchemaVersion, "2.0")
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T", response.Data)
	}
	service, ok := data["service"].(string)
	if !ok {
		t.Fatalf("service type = %T", data["service"])
	}
	assertEqual(t, "service", service, "amazon-crawler")
}

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
	priceValue := 95.99
	apiItem := linkInspectionResultToAPIItem(LinkInspectionResult{
		Item: LinkInspectionItem{
			Original: "https://www.amazon.com/dp/B0B6FZ1R2L",
			URL:      "https://www.amazon.com/dp/B0B6FZ1R2L",
			ASIN:     "B0B6FZ1R2L",
			Domain:   "www.amazon.com",
		},
		Product:             "Lightdot 2 Pack 150W Wall Pack LED Exterior Light",
		ASIN:                "B0DKF7HNZX",
		ActualASIN:          "B0DKF7HNZX",
		FinalURL:            "https://www.amazon.com/dp/B0DKF7HNZX",
		Price:               variantPriceStatus,
		PriceValue:          &priceValue,
		Currency:            "USD",
		AvailabilityStatus:  availabilityStatusAvailable,
		FeaturedOfferStatus: featuredOfferStatusPresent,
		FeaturedOfferText:   "$95.99 In Stock Sold by Lightdot Direct",
		SellerID:            "A1LIGHTDOT",
		SellerName:          "Lightdot Direct",
		Coupon:              " ",
		IsDeal:              "Deal",
		PrimeExclusive:      " ",
		DisplayDiscount:     " ",
		Rating:              "4.3",
		ReviewCount:         7,
		Choice:              "Amazon's  Choice",
	}, "2026-06-06T00:00:00Z")

	assertEqual(t, "status", apiItem.Status, "success")
	assertEqual(t, "original asin", apiItem.OriginalASIN, "B0B6FZ1R2L")
	assertEqual(t, "actual asin", apiItem.ASIN, "B0DKF7HNZX")
	assertEqual(t, "explicit actual asin", apiItem.ActualASIN, "B0DKF7HNZX")
	assertEqual(t, "final url", apiItem.FinalURL, "https://www.amazon.com/dp/B0DKF7HNZX")
	assertEqual(t, "price", apiItem.Price, variantPriceStatus)
	if apiItem.PriceValue == nil || *apiItem.PriceValue != 95.99 {
		t.Fatalf("price value = %v", apiItem.PriceValue)
	}
	assertEqual(t, "currency", apiItem.Currency, "USD")
	assertEqual(t, "availability status", apiItem.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer status", apiItem.FeaturedOfferStatus, featuredOfferStatusPresent)
	assertEqual(t, "seller id", apiItem.SellerID, "A1LIGHTDOT")
	assertEqual(t, "seller name", apiItem.SellerName, "Lightdot Direct")
	assertEqual(t, "choice", apiItem.ChoiceBadge, "Amazon's  Choice")
	if apiItem.ReviewCount != 7 {
		t.Fatalf("review count = %d", apiItem.ReviewCount)
	}
}

func TestASINInspectionResponseJSONV2Contract(t *testing.T) {
	priceValue := 199.99
	response := APIResponse{
		Code:                    0,
		Message:                 "ok",
		InspectionSchemaVersion: inspectionSchemaVersion,
		Data: ASINInspectionResponseData{
			JobID: "job-1",
			Items: []ASINInspectionResponseItem{
				{
					Input:               "B0FNMPQSJC",
					URL:                 "https://www.amazon.com/dp/B0FNMPQSJC",
					FinalURL:            "https://www.amazon.com/dp/B0FNMPQSJC?th=1",
					Domain:              "www.amazon.com",
					OriginalASIN:        "B0FNMPQSJC",
					ASIN:                "B0FNMPQSJC",
					ActualASIN:          "B0FNMPQSJC",
					Status:              "success",
					Price:               "$199.99",
					PriceValue:          &priceValue,
					Currency:            "USD",
					AvailabilityStatus:  availabilityStatusAvailable,
					FeaturedOfferStatus: featuredOfferStatusMissing,
					FeaturedOfferText:   "No featured offers available",
					SellerID:            "",
					SellerName:          "",
				},
			},
		},
	}

	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "schema version", payload["inspection_schema_version"].(string), "2.0")

	data := payload["data"].(map[string]interface{})
	if _, exists := data["inspection_schema_version"]; exists {
		t.Fatal("inspection_schema_version must be at the response root")
	}
	items := data["items"].([]interface{})
	item := items[0].(map[string]interface{})

	stringFields := map[string]string{
		"availability_status":   availabilityStatusAvailable,
		"featured_offer_status": featuredOfferStatusMissing,
		"featured_offer_text":   "No featured offers available",
		"seller_id":             "",
		"seller_name":           "",
		"currency":              "USD",
		"actual_asin":           "B0FNMPQSJC",
		"final_url":             "https://www.amazon.com/dp/B0FNMPQSJC?th=1",
		"price":                 "$199.99",
	}
	for field, want := range stringFields {
		got, exists := item[field]
		if !exists {
			t.Fatalf("missing JSON field %q: %s", field, string(raw))
		}
		assertEqual(t, field, got.(string), want)
	}
	if got := item["price_value"].(float64); got != 199.99 {
		t.Fatalf("price_value = %v", got)
	}
}

func TestLinkInspectionResultToAPIItemDefaultsUnknownV2Fields(t *testing.T) {
	apiItem := linkInspectionResultToAPIItem(LinkInspectionResult{
		Item: LinkInspectionItem{
			Original: "B0FNMPQSJC",
			URL:      "https://www.amazon.com/dp/B0FNMPQSJC",
			ASIN:     "B0FNMPQSJC",
			Domain:   "www.amazon.com",
		},
		ASIN:         "B0FNMPQSJC",
		ErrorMessage: "parse failed",
	}, "2026-07-12T00:00:00Z")

	assertEqual(t, "status", apiItem.Status, "failed")
	assertEqual(t, "availability status", apiItem.AvailabilityStatus, availabilityStatusUnknown)
	assertEqual(t, "featured offer status", apiItem.FeaturedOfferStatus, featuredOfferStatusUnknown)
	assertEqual(t, "actual asin fallback", apiItem.ActualASIN, "B0FNMPQSJC")
	assertEqual(t, "final url fallback", apiItem.FinalURL, "https://www.amazon.com/dp/B0FNMPQSJC")
	if apiItem.PriceValue != nil {
		t.Fatalf("price value = %v, want nil", apiItem.PriceValue)
	}

	raw, err := json.Marshal(apiItem)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	value, exists := payload["price_value"]
	if !exists || value != nil {
		t.Fatalf("price_value must be present as null: %s", string(raw))
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
