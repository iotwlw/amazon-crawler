package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseLinkInspectionItem(t *testing.T) {
	item, ok := parseLinkInspectionItem("https://www.amazon.com/dp/B0DWWWP4FF?ref_=x&th=1", "www.amazon.com.mx")
	if !ok {
		t.Fatal("expected URL to parse")
	}
	if item.ASIN != "B0DWWWP4FF" {
		t.Fatalf("asin = %q", item.ASIN)
	}
	if item.Domain != "www.amazon.com" {
		t.Fatalf("domain = %q", item.Domain)
	}
	if item.URL != "https://www.amazon.com/dp/B0DWWWP4FF?th=1" {
		t.Fatalf("url = %q", item.URL)
	}

	item, ok = parseLinkInspectionItem("B0FNMPQSJC", "www.amazon.com")
	if !ok {
		t.Fatal("expected bare ASIN to parse")
	}
	if item.URL != "https://www.amazon.com/dp/B0FNMPQSJC" {
		t.Fatalf("bare asin url = %q", item.URL)
	}
}

func TestExtractLinkInspectionFields(t *testing.T) {
	html := `
<html>
<body>
  <input id="ASIN" value="B0FNMPQSJC"/>
  <span id="productTitle"> Lightdot 320W LED Wall Pack Lights </span>
  <div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div>
  <div id="corePriceDisplay_desktop_feature_div">
    <span class="savingsPercentage">-99%</span>
    <span class="basisPrice"><span class="a-offscreen">$294.10</span></span>
  </div>
  <div id="averageCustomerReviews"><span aria-hidden="true">4.3</span></div>
  <span id="acrCustomerReviewText">7 ratings</span>
  <div id="promoPriceBlockMessage_feature_div">
    <span class="promoPriceBlockMessage">
      <div><span><div><div>Apply 10% coupon</div></div></span></div>
      <div><span><label>Save 50%</label><span>Code: ABC123</span></span></div>
    </span>
  </div>
  <span id="dealBadgeSupportingText">Deal</span>
  <span id="primeExclusivePricingMessage"><span class="a-size-base">$49.99</span></span>
  <div id="NEW_1_nostos_badge">Customers usually keep this item.</div>
  <div id="acBadge_feature_div"><div><span><span><span>    Amazon's  Choice   </span></span></span></div><span>Overall Pick</span></div>
</body>
</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0FNMPQSJC",
		URL:      "https://www.amazon.com/dp/B0FNMPQSJC",
		ASIN:     "B0FNMPQSJC",
		Domain:   "www.amazon.com",
	}
	result := extractLinkInspectionFields(doc, item)

	assertEqual(t, "product", result.Product, "Lightdot 320W LED Wall Pack Lights")
	assertEqual(t, "asin", result.ASIN, "B0FNMPQSJC")
	assertEqual(t, "price", result.Price, "$199.99")
	assertEqual(t, "coupon", result.Coupon, "10%")
	assertEqual(t, "deal", result.IsDeal, "Deal")
	assertEqual(t, "prime", result.PrimeExclusive, "$49.99")
	assertEqual(t, "discount", result.DisplayDiscount, "-32%")
	assertEqual(t, "rating", result.Rating, "4.3")
	if result.ReviewCount != 7 {
		t.Fatalf("review count = %d", result.ReviewCount)
	}
	assertEqual(t, "promotion", result.Promotion, "Save 50%")
	assertEqual(t, "promo code", result.PromoCode, "Code: ABC123")
	assertEqual(t, "promo check", result.PromoCheck, "")
	assertEqual(t, "keep", result.Keep, "Customers usually keep this item.")
	assertEqual(t, "choice", result.Choice, "Amazon's  Choice")
}

func TestExtractPriceStatusValue(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "unavailable",
			html: `<html><body><div id="availability"><span>Currently unavailable.</span><span>We don't know when or if this item will be back in stock.</span></div></body></html>`,
			want: "不可售",
		},
		{
			name: "buy used",
			html: `<html><body><div id="rightCol"><span>Buy used: $305.01</span><span>Used: Like New</span><span>Sold by Amazon Resale</span></div></body></html>`,
			want: "二手跟卖",
		},
		{
			name: "no featured offer",
			html: `<html><body><div id="rightCol"><span>No featured offers available</span><a>See All Buying Options</a></div></body></html>`,
			want: "没有购物车",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
			if err != nil {
				t.Fatal(err)
			}
			assertEqual(t, tc.name, extractPriceStatusValue(doc), tc.want)
		})
	}
}

func TestPriceStatusBackfillsOnlyMissingPrice(t *testing.T) {
	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0FNMPQSJC",
		URL:      "https://www.amazon.com/dp/B0FNMPQSJC",
		ASIN:     "B0FNMPQSJC",
		Domain:   "www.amazon.com",
	}
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "keeps current price",
			html: `<html><body><input id="ASIN" value="B0FNMPQSJC"/><div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div><div id="availability">Currently unavailable.</div></body></html>`,
			want: "$199.99",
		},
		{
			name: "fills missing price from status",
			html: `<html><body><input id="ASIN" value="B0FNMPQSJC"/><div id="availability">Currently unavailable.</div></body></html>`,
			want: "不可售",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
			if err != nil {
				t.Fatal(err)
			}
			result := extractLinkInspectionFields(doc, item)
			assertEqual(t, "price", result.Price, tc.want)
		})
	}
}

func TestChoiceFallsBackToContainerButNormalizes(t *testing.T) {
	html := `<html><body><div id="acBadge_feature_div">Amazon's Choice in Outdoor Wall Lights by Lightdot</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "choice", extractChoiceValue(doc), "Amazon's  Choice")
}

func TestCalculateDisplayDiscountFromListPriceAndPrice(t *testing.T) {
	assertEqual(t, "discount", calculateDisplayDiscount("List Price: $269.99", "$242.99"), "-10%")
	assertEqual(t, "discount missing list price", calculateDisplayDiscount("", "$242.99"), "")
	assertEqual(t, "discount no markdown", calculateDisplayDiscount("$242.99", "$242.99"), "")
}

func TestExtractCouponValueIgnoresPromoScripts(t *testing.T) {
	html := `
<html>
<body>
  <div id="promoPriceBlockMessage_feature_div">
    <span class="promoPriceBlockMessage">
      <script>window.location.href = '/promotion?token=abc5%2Fdef&asin=B0C73DTJLQ';</script>
      <style>.couponLabelText { display: inline; }</style>
      <span id="couponTextpctch123" class="a-color-success couponLabelText">
        Apply 10% coupon
        <a>Shop items</a>
        <style>.cxcwEmphasisLink { padding-left: 6px; }</style>
        <a>Terms</a>
      </span>
    </span>
  </div>
</body>
</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "coupon", extractCouponValue(doc), "10%")
}

func TestWriteInspectionXLSX(t *testing.T) {
	out := filepath.Join(t.TempDir(), "inspection.xlsx")
	if containsString(inspectionHeaders, "Ask Rufus问题") {
		t.Fatal("Ask Rufus column should not be present")
	}
	if containsString(inspectionHeaders, "价格状态") {
		t.Fatal("价格状态 column should not be present")
	}
	if inspectionHeaders[len(inspectionHeaders)-1] != "Choice" {
		t.Fatalf("last header = %q, want Choice", inspectionHeaders[len(inspectionHeaders)-1])
	}
	rows := [][]string{
		inspectionHeaders,
		{"Product", "https://www.amazon.com/dp/B0FNMPQSJC", "B0FNMPQSJC", "$199.99", "10%", " ", " ", "-32%", "4.3", "7", "", "", "", "", "Amazon's Choice"},
	}
	if err := writeInspectionXLSX(out, rows); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		t.Fatalf("xlsx not written: info=%v err=%v", info, err)
	}

	reader, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	foundSheet := false
	for _, file := range reader.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			foundSheet = true
			rc, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(rc)
			if err != nil {
				rc.Close()
				t.Fatal(err)
			}
			rc.Close()
			if !strings.Contains(string(data), "Amazon&#39;s Choice") {
				t.Fatalf("sheet did not contain escaped choice value: %s", string(data))
			}
		}
	}
	if !foundSheet {
		t.Fatal("sheet1.xml not found")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertEqual(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}
