package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractProductImagesFromHTMLClassifiesMainAndAPlus(t *testing.T) {
	htmlText := `<!doctype html><html><head>
<title>Fixture Product - Amazon.ca</title>
<link rel="canonical" href="https://www.amazon.ca/dp/B0D6XW3ZR7" />
</head><body>
<input id="ASIN" value="B0D6XW3ZR7" />
<span id="productTitle">ESVIENS Button Replacement Adjustable Tightener</span>
<div id="imageBlock"><img id="landingImage" data-a-dynamic-image='{"https://m.media-amazon.com/images/I/71abc._AC_SL1500_.jpg":[1500,1500],"https://m.media-amazon.com/images/I/61abc._AC_SX679_.jpg":[679,679]}' src="https://m.media-amazon.com/images/I/41abc._AC_US40_.jpg" /></div>
<div id="altImages"><img src="https://m.media-amazon.com/images/I/51def._AC_US40_.jpg" data-old-hires="https://m.media-amazon.com/images/I/71def._AC_SL1500_.jpg" /></div>
<div id="aplus_feature_div"><div id="aplus"><img data-src="https://m.media-amazon.com/images/S/aplus-media-library-service-media/abc123._CR0,0,970,600_.jpg" /><img srcset="https://m.media-amazon.com/images/I/81ghi._AC_SX970_.jpg 970w" /></div></div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		t.Fatal(err)
	}

	result := extractProductImagesFromHTML(htmlText, doc, "https://www.amazon.ca/dp/B0D6XW3ZR7", ProductImageExtractionOptions{})

	assertEqual(t, "asin", result.ASIN, "B0D6XW3ZR7")
	assertEqual(t, "title", result.ProductTitle, "ESVIENS Button Replacement Adjustable Tightener")
	if result.MainCandidates == 0 {
		t.Fatal("expected main candidates")
	}
	if result.APlusCandidates == 0 {
		t.Fatal("expected A+ candidates")
	}
	mainCount := 0
	aplusCount := 0
	for _, image := range result.Images {
		switch image.Role {
		case productImageRoleMain:
			mainCount++
		case productImageRoleAPlus:
			aplusCount++
		}
		if strings.HasSuffix(image.URL, ".svg") {
			t.Fatalf("unexpected svg image: %s", image.URL)
		}
	}
	if mainCount == 0 || aplusCount == 0 {
		t.Fatalf("main=%d aplus=%d, want both roles", mainCount, aplusCount)
	}
}

func TestExtractProductImagesHonorsIncludeAndLimits(t *testing.T) {
	htmlText := `<!doctype html><html><body>
<input id="ASIN" value="B0D6XW3ZR7" />
<div id="imageBlock">
<img src="https://m.media-amazon.com/images/I/51aaa._AC_US40_.jpg" />
<img src="https://m.media-amazon.com/images/I/51bbb._AC_US40_.jpg" />
</div>
<div id="aplus"><img src="https://m.media-amazon.com/images/I/51ccc._AC_US40_.jpg" /></div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		t.Fatal(err)
	}

	result := extractProductImagesFromHTML(htmlText, doc, "https://www.amazon.ca/dp/B0D6XW3ZR7", ProductImageExtractionOptions{
		Include: productImageRoleMain,
		MaxMain: 1,
	})

	if len(result.Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(result.Images))
	}
	assertEqual(t, "role", result.Images[0].Role, productImageRoleMain)
}

func TestHandleProductImagesRejectsInvalidInput(t *testing.T) {
	body, _ := json.Marshal(ProductImagesRequest{URL: "not-a-link"})
	req := httptest.NewRequest(http.MethodPost, "/api/product-images", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handleProductImages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
