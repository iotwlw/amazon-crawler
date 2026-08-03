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
  <div id="product-alert-grid_feature_div">
    <div class="a-alert-content">
      <span>Frequently returned item</span>
      <span>Check the product details and customer reviews to learn more about this item.</span>
    </div>
  </div>
  <div id="newerVersion_feature_div">
    <h1>There is a newer model of this item:</h1>
    <a>Lightdot 320W LED Parking Lot Light 80000Lumens 5000K LED Pole Lights Outdoor</a>
    <span>$369.99</span>
    <span>Only 10 left in stock - order soon.</span>
  </div>
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
	assertEqual(t, "frequently returned", result.FrequentReturn, "Frequently returned item Check the product details and customer reviews to learn more about this item.")
	assertEqual(t, "newer model", result.NewerModel, "There is a newer model of this item: Lightdot 320W LED Parking Lot Light 80000Lumens 5000K LED Pole Lights Outdoor $369.99 Only 10 left in stock - order soon.")
}

func TestVariantMergedASINMarksPriceAndOriginalASIN(t *testing.T) {
	html := `
<html>
<head>
  <link rel="canonical" href="https://www.amazon.com/Lightdot-100-277v-Photocell-Exterior-Lighting/dp/B0DKF7HNZX"/>
</head>
<body>
  <input id="ASIN" value="B0DKF7HNZX"/>
  <span id="productTitle"> Lightdot 2 Pack 150W Wall Pack LED Exterior Light </span>
  <div id="corePrice_feature_div"><span class="a-offscreen">$95.99</span></div>
</body>
</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0B6FZ1R2L",
		URL:      "https://www.amazon.com/dp/B0B6FZ1R2L",
		ASIN:     "B0B6FZ1R2L",
		Domain:   "www.amazon.com",
	}
	result := extractLinkInspectionFields(doc, item)

	assertEqual(t, "actual asin", result.ASIN, "B0DKF7HNZX")
	assertEqual(t, "price", result.Price, "不可售-变体")

	rows := inspectionRows([]LinkInspectionResult{result})
	assertEqual(t, "original asin column", rows[1][1], "B0B6FZ1R2L")
	assertEqual(t, "actual asin column", rows[1][2], "B0DKF7HNZX")
	assertEqual(t, "price column", rows[1][3], "不可售-变体")
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

func TestExtractInspectionContractStatuses(t *testing.T) {
	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0FNMPQSJC",
		URL:      "https://www.amazon.com/dp/B0FNMPQSJC",
		ASIN:     "B0FNMPQSJC",
		Domain:   "www.amazon.com",
	}
	cases := []struct {
		name             string
		html             string
		wantPrice        string
		wantAvailability string
		wantOffer        string
	}{
		{
			name:             "unavailable",
			html:             `<html><body><div id="availability"><span>Currently unavailable.</span><span>We don't know when or if this item will be back in stock.</span></div></body></html>`,
			wantPrice:        "不可售",
			wantAvailability: availabilityStatusUnavailable,
			wantOffer:        featuredOfferStatusUnknown,
		},
		{
			name:             "used only",
			html:             `<html><body><div id="usedBuyBox"><span>No featured offers available</span><span>Buy used: $305.01</span><span>Used: Like New</span><span>Sold by Amazon Resale</span></div></body></html>`,
			wantPrice:        "二手跟卖",
			wantAvailability: availabilityStatusAvailable,
			wantOffer:        featuredOfferStatusUsedOnly,
		},
		{
			name: "featured offer present",
			html: `<html><body>
			  <div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div>
			  <div id="desktop_buybox">
			    <div id="availability">In Stock</div>
			    <div id="merchant-info">Ships from Amazon.com Sold by <a id="sellerProfileTriggerId" href="/sp?seller=A1LIGHTDOT">Lightdot</a></div>
			    <input id="add-to-cart-button" name="submit.add-to-cart"/>
			  </div>
			</body></html>`,
			wantPrice:        "$199.99",
			wantAvailability: availabilityStatusAvailable,
			wantOffer:        featuredOfferStatusPresent,
		},
		{
			name:             "unknown page",
			html:             `<html><body><span id="productTitle">Product without commerce modules</span></body></html>`,
			wantPrice:        "",
			wantAvailability: availabilityStatusUnknown,
			wantOffer:        featuredOfferStatusUnknown,
		},
		{
			name:             "price alone does not imply availability",
			html:             `<html><body><div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div></body></html>`,
			wantPrice:        "$199.99",
			wantAvailability: availabilityStatusUnknown,
			wantOffer:        featuredOfferStatusUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
			if err != nil {
				t.Fatal(err)
			}
			result := extractLinkInspectionFields(doc, item)
			assertEqual(t, "legacy price", result.Price, tc.wantPrice)
			assertEqual(t, "availability status", result.AvailabilityStatus, tc.wantAvailability)
			assertEqual(t, "featured offer status", result.FeaturedOfferStatus, tc.wantOffer)
		})
	}
}

func TestFeaturedOfferPresentWinsOverSecondaryUsedOffer(t *testing.T) {
	html := `<html><body>
	  <input id="ASIN" value="B0FNMPQSJC"/>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div>
	  <div id="desktop_buybox">
	    <div id="availability">In Stock</div>
	    <div id="merchant-info">Ships from Amazon.com Sold by <a id="sellerProfileTriggerId" href="/sp?seller=A1LIGHTDOT">Lightdot Direct</a></div>
	  </div>
	  <div id="usedBuyBox"><span>Used - Like New from $149.99</span><span>Sold by Amazon Resale</span></div>
	</body></html>`
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
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusPresent)
	assertEqual(t, "seller id", result.SellerID, "A1LIGHTDOT")
	assertEqual(t, "seller name", result.SellerName, "Lightdot Direct")
}

func TestUnavailablePageWithPriceAndBuyboxIsNotFeaturedOfferPresent(t *testing.T) {
	html := `<html><body>
	  <input id="ASIN" value="B0FNMPQSJC"/>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div>
	  <div id="desktop_buybox"><div id="availability">Currently unavailable.</div></div>
	</body></html>`
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
	assertEqual(t, "legacy price remains unchanged", result.Price, "$199.99")
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusUnavailable)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusUnknown)
	assertEqual(t, "seller id", result.SellerID, "")
	assertEqual(t, "seller name", result.SellerName, "")
}

func TestUnrelatedQuickViewUnavailableIsIgnored(t *testing.T) {
	html := `<html><body>
	  <span id="productTitle">Current product without commerce modules</span>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$187.99</span></div>
	  <div id="productQuickView_feature_div">
	    <div id="a-popover-pqvOverlay" class="a-popover-preload">
	      <div id="pqv-newer-version">
	        <div id="availability"><p>Currently unavailable.</p></div>
	        <a id="sellerProfileTriggerId" href="/sp?seller=A1QUICKVIEW">Other seller</a>
	        <input id="add-to-cart-button" name="submit.add-to-cart"/>
	        <input id="buy-now-button" name="submit.buy-now"/>
	      </div>
	    </div>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "B0B87GXWLM",
		URL:      "https://www.amazon.com/dp/B0B87GXWLM",
		ASIN:     "B0B87GXWLM",
		Domain:   "www.amazon.com",
	}

	result := extractLinkInspectionFields(doc, item)
	assertEqual(t, "price", result.Price, "$187.99")
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusUnknown)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusUnknown)
	assertEqual(t, "seller id", result.SellerID, "")
	assertEqual(t, "seller name", result.SellerName, "")
}

func TestUnrelatedQuickViewUnavailableDoesNotBackfillLegacyPrice(t *testing.T) {
	html := `<html><body>
	  <span id="productTitle">Current product without commerce modules</span>
	  <div id="productQuickView_feature_div">
	    <div id="a-popover-pqvOverlay" class="a-popover-preload">
	      <div id="pqv-newer-version"><div id="availability">Currently unavailable.</div></div>
	    </div>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "B0B87GXWLM",
		URL:      "https://www.amazon.com/dp/B0B87GXWLM",
		ASIN:     "B0B87GXWLM",
		Domain:   "www.amazon.com",
	}

	result := extractLinkInspectionFields(doc, item)
	assertEqual(t, "price", result.Price, "")
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusUnknown)
}

func TestConfirmedFeaturedOfferWinsConflictingAvailabilityEvidence(t *testing.T) {
	for _, asin := range []string{"B0B87GXWLM", "B0BCL4SH3W"} {
		t.Run(asin, func(t *testing.T) {
			html := `<html><body>
			  <input id="ASIN" value="` + asin + `"/>
			  <div id="corePrice_feature_div"><span class="a-offscreen">$187.99</span></div>
			  <div id="desktop_buybox">
			    <div id="desktop_qualifiedBuyBox">
			      <div id="availabilityInsideBuyBox_feature_div">
			        <div id="availability">Only 7 left in stock - order soon.</div>
			      </div>
			      <div id="merchantInfoFeature_feature_div">
			        <span>Ships from Amazon.com</span>
			        <span>Sold by <a id="sellerProfileTriggerId" href="/gp/help/seller/at-a-glance.html?ie=UTF8&amp;seller=A31LEUCZ131Y7E&amp;isAmazonFulfilled=1">Lightdot</a></span>
			      </div>
			      <form id="addToCart">
			        <input id="merchantID" value="A31LEUCZ131Y7E"/>
			        <input id="add-to-cart-button" name="submit.add-to-cart"/>
			      </form>
			      <input id="buy-now-button" name="submit.buy-now"/>
			      <div id="outOfStock">Currently unavailable.</div>
			    </div>
			  </div>
			</body></html>`
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			if err != nil {
				t.Fatal(err)
			}
			item := LinkInspectionItem{
				Original: asin,
				URL:      "https://www.amazon.com/dp/" + asin,
				ASIN:     asin,
				Domain:   "www.amazon.com",
			}

			result := extractLinkInspectionFields(doc, item)
			assertEqual(t, "price", result.Price, "$187.99")
			assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusAvailable)
			assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusPresent)
			assertEqual(t, "seller id", result.SellerID, "A31LEUCZ131Y7E")
			assertEqual(t, "seller name", result.SellerName, "Lightdot")
		})
	}
}

func TestHiddenUsedAccordionDoesNotOverrideFeaturedOffer(t *testing.T) {
	html := `<html><body>
	  <input id="ASIN" value="B0BCL4SH3W"/>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$257.99</span></div>
	  <div id="desktop_buybox">
	    <div id="usedAccordionRow">
	      <div id="availability">Only 1 left in stock - order soon.</div>
	      <a id="sellerProfileTriggerId" href="/sp?seller=A1USEDSELLER">Used Seller</a>
	      <input id="merchantID" value="A1USEDSELLER"/>
	      <input id="add-to-cart-button" name="submit.add-to-cart"/>
	    </div>
	    <div id="newAccordionRow_0">
	      <div id="desktop_qualifiedBuyBox">
	        <div id="availability">In Stock</div>
	        <div id="offerDisplayFeatures_desktop">
	          <div id="fulfillerInfoFeature_feature_div"><span>Ships from Amazon</span></div>
	          <div id="merchantInfoFeature_feature_div">
	            <span>Sold by <a id="sellerProfileTriggerId" href="/sp?seller=A31LEUCZ131Y7E">Lightdot</a></span>
	          </div>
	        </div>
	        <form id="addToCart">
	          <input id="merchantID" value="A31LEUCZ131Y7E"/>
	          <input id="add-to-cart-button" name="submit.add-to-cart"/>
	        </form>
	        <input id="buy-now-button" name="submit.buy-now"/>
	      </div>
	    </div>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "B0BCL4SH3W",
		URL:      "https://www.amazon.com/dp/B0BCL4SH3W",
		ASIN:     "B0BCL4SH3W",
		Domain:   "www.amazon.com",
	}

	result := extractLinkInspectionFields(doc, item)
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusPresent)
	assertEqual(t, "seller id", result.SellerID, "A31LEUCZ131Y7E")
	assertEqual(t, "seller name", result.SellerName, "Lightdot")
	if strings.Contains(result.FeaturedOfferText, "Used Seller") {
		t.Fatalf("featured offer text included hidden used seller: %q", result.FeaturedOfferText)
	}
}

func TestUsedAccordionOnlyIsUsedOffer(t *testing.T) {
	html := `<html><body>
	  <div id="desktop_buybox">
	    <div id="usedAccordionRow">
	      <span>No featured offers available</span>
	      <span>Condition: Used - Like New</span>
	      <a id="sellerProfileTriggerId" href="/sp?seller=A1USEDSELLER">Amazon Resale</a>
	      <input id="merchantID" value="A1USEDSELLER"/>
	      <input id="add-to-cart-button" name="submit.add-to-cart"/>
	    </div>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "B0BCL4SH3W",
		URL:      "https://www.amazon.com/dp/B0BCL4SH3W",
		ASIN:     "B0BCL4SH3W",
		Domain:   "www.amazon.com",
	}

	result := extractLinkInspectionFields(doc, item)
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusUsedOnly)
	assertEqual(t, "seller id", result.SellerID, "")
	assertEqual(t, "seller name", result.SellerName, "")
}

func TestUsedOnlyDoesNotExposeFeaturedOfferSeller(t *testing.T) {
	html := `<html><body>
	  <div id="usedBuyBox">
	    <span>No featured offers available</span>
	    <span>Condition: Used - Like New</span>
	    <a id="sellerProfileTriggerId" href="/sp?seller=A1USEDSELLER">Amazon Resale</a>
	    <input id="merchantID" value="A1USEDSELLER"/>
	    <input id="add-to-cart-button"/>
	  </div>
	</body></html>`
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
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusUsedOnly)
	assertEqual(t, "seller id", result.SellerID, "")
	assertEqual(t, "seller name", result.SellerName, "")
}

func TestNoFeaturedOfferOverridesPriceForStructuredStatus(t *testing.T) {
	html := `<html><body>
	  <input id="ASIN" value="B0FNMPQSJC"/>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$199.99</span></div>
	  <div id="rightCol"><span>No featured offers available</span><a>See All Buying Options</a><input id="add-to-cart-button"/></div>
	</body></html>`
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
	assertEqual(t, "legacy price remains unchanged", result.Price, "$199.99")
	assertEqual(t, "availability status", result.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer status", result.FeaturedOfferStatus, featuredOfferStatusMissing)
	if !strings.Contains(result.FeaturedOfferText, "No featured offers available") {
		t.Fatalf("featured offer text = %q", result.FeaturedOfferText)
	}
	if result.PriceValue == nil || *result.PriceValue != 199.99 {
		t.Fatalf("price value = %v", result.PriceValue)
	}
	assertEqual(t, "currency", result.Currency, "USD")
}

func TestExtractStructuredOfferSellerAndRedirectFields(t *testing.T) {
	html := `<html><body>
	  <input id="ASIN" value="B0DKF7HNZX"/>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$95.99</span></div>
	  <div id="desktop_buybox">
	    <div id="availability">Only 3 left in stock - order soon.</div>
	    <div id="merchant-info">Ships from Amazon.com Sold by <a id="sellerProfileTriggerId" href="/sp?seller=A1LIGHTDOT&amp;ref_=dp_merchant_link">Lightdot Direct</a></div>
	    <input id="add-to-cart-button"/>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0B6FZ1R2L",
		URL:      "https://www.amazon.com/dp/B0B6FZ1R2L",
		ASIN:     "B0B6FZ1R2L",
		Domain:   "www.amazon.com",
	}
	finalURL := "https://www.amazon.com/dp/B0DKF7HNZX?th=1"

	result := extractLinkInspectionPageFields(doc, item, finalURL)
	assertEqual(t, "legacy asin", result.ASIN, "B0DKF7HNZX")
	assertEqual(t, "actual asin", result.ActualASIN, "B0DKF7HNZX")
	assertEqual(t, "final url", result.FinalURL, finalURL)
	assertEqual(t, "legacy variant price", result.Price, variantPriceStatus)
	if result.PriceValue == nil || *result.PriceValue != 95.99 {
		t.Fatalf("price value = %v", result.PriceValue)
	}
	assertEqual(t, "currency", result.Currency, "USD")
	assertEqual(t, "availability", result.AvailabilityStatus, availabilityStatusAvailable)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusPresent)
	assertEqual(t, "seller id", result.SellerID, "A1LIGHTDOT")
	assertEqual(t, "seller name", result.SellerName, "Lightdot Direct")
	if !strings.Contains(result.FeaturedOfferText, "Sold by Lightdot Direct") {
		t.Fatalf("featured offer text = %q", result.FeaturedOfferText)
	}
}

func TestActualASINFallsBackToFinalURL(t *testing.T) {
	html := `<html><body><span id="productTitle">Redirected product</span></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "https://www.amazon.com/dp/B0B6FZ1R2L",
		URL:      "https://www.amazon.com/dp/B0B6FZ1R2L",
		ASIN:     "B0B6FZ1R2L",
		Domain:   "www.amazon.com",
	}
	finalURL := "https://www.amazon.com/gp/product/B0DKF7HNZX?th=1"

	result := extractLinkInspectionPageFields(doc, item, finalURL)
	assertEqual(t, "actual asin", result.ActualASIN, "B0DKF7HNZX")
	assertEqual(t, "legacy asin", result.ASIN, "B0DKF7HNZX")
	assertEqual(t, "final url", result.FinalURL, finalURL)
	assertEqual(t, "variant price", result.Price, variantPriceStatus)
}

func TestExtractFeaturedOfferSellerSupportsKnownSelectorVariant(t *testing.T) {
	html := `<html><body>
	  <div id="corePrice_feature_div"><span class="a-offscreen">$95.99</span></div>
	  <div id="desktop_buybox">
	    <a id="vse-seller-link" href="/sp?seller=A1VSESELLER">VSE Seller</a>
	    <input id="add-to-cart-button"/>
	  </div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	item := LinkInspectionItem{
		Original: "B0FNMPQSJC",
		URL:      "https://www.amazon.com/dp/B0FNMPQSJC",
		ASIN:     "B0FNMPQSJC",
		Domain:   "www.amazon.com",
	}

	result := extractLinkInspectionFields(doc, item)
	assertEqual(t, "featured offer", result.FeaturedOfferStatus, featuredOfferStatusPresent)
	assertEqual(t, "seller id", result.SellerID, "A1VSESELLER")
	assertEqual(t, "seller name", result.SellerName, "VSE Seller")
}

func TestExtractStructuredPriceSupportsMarketplaceFormats(t *testing.T) {
	cases := []struct {
		name         string
		price        string
		domain       string
		wantValue    float64
		wantCurrency string
	}{
		{name: "usd thousands", price: "$1,299.99", domain: "www.amazon.com", wantValue: 1299.99, wantCurrency: "USD"},
		{name: "mxn domain fallback", price: "$1,299.00", domain: "www.amazon.com.mx", wantValue: 1299, wantCurrency: "MXN"},
		{name: "eur decimal comma", price: "199,99 €", domain: "www.amazon.de", wantValue: 199.99, wantCurrency: "EUR"},
		{name: "eur space thousands", price: "1 299,99 €", domain: "www.amazon.de", wantValue: 1299.99, wantCurrency: "EUR"},
		{name: "eur narrow no-break space thousands", price: "1\u202f299,99 €", domain: "www.amazon.fr", wantValue: 1299.99, wantCurrency: "EUR"},
		{name: "jpy thousands", price: "￥1,299", domain: "www.amazon.co.jp", wantValue: 1299, wantCurrency: "JPY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, currency := extractStructuredPrice(tc.price, tc.domain)
			if value == nil || *value != tc.wantValue {
				t.Fatalf("price value = %v, want %v", value, tc.wantValue)
			}
			assertEqual(t, "currency", currency, tc.wantCurrency)
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

func TestExtractPromotionValueSupportsBrandPromotion(t *testing.T) {
	html := `<html><body>
  <div id="promoPriceBlockMessage_feature_div">
    <span class="promoPriceBlockMessage">
      <div><span><span class="a-size-base a-color-success">Brand Promotion</span></span></div>
    </span>
  </div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "brand promotion", extractPromotionValue(doc), "Brand Promotion")
}

func TestExtractPromotionValueIgnoresCouponOnly(t *testing.T) {
	html := `<html><body>
  <div id="promoPriceBlockMessage_feature_div">
    <span class="promoPriceBlockMessage">
      <div><span><label>Apply 10% coupon</label><a>Shop items</a><a>Terms</a></span></div>
    </span>
  </div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "coupon-only promotion", extractPromotionValue(doc), "")
}

func TestExtractPromotionValueFallsBackToSponsoredBrand(t *testing.T) {
	html := `<html><body>
  <div class="sbx-desktop" data-slot="desktop-arbies">
    <h2>Brand in this category on Amazon</h2>
    <a aria-label="Sponsored ad from Lighting Your Business Future. Shop Lighting Your Business Future."></a>
  </div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "sponsored brand promotion", extractPromotionValue(doc), "Brand Promotion")
}

func TestExtractPromotionValueDoesNotUseSponsoredBrandWhenCouponExists(t *testing.T) {
	html := `<html><body>
  <div id="promoPriceBlockMessage_feature_div">
    <span class="promoPriceBlockMessage">
      <span id="couponTextpctch123" class="couponLabelText">Apply 10% coupon</span>
    </span>
  </div>
  <div class="sbx-desktop" data-slot="desktop-arbies">
    <h2>Brand in this category on Amazon</h2>
  </div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "sponsored brand with coupon", extractPromotionValue(doc), "")
}

func TestExtractFrequentlyReturnedValueFallsBackByText(t *testing.T) {
	html := `<html><body><div class="a-alert"><span>Frequently returned item</span><span>Check the product details and customer reviews to learn more about this item.</span></div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "frequently returned", extractFrequentlyReturnedValue(doc), "Frequently returned item Check the product details and customer reviews to learn more about this item.")
}

func TestExtractNewerModelValueFallsBackByText(t *testing.T) {
	html := `<html><body><div class="a-box"><div>There is a newer model of this item:</div><a>Replacement product title</a><span>$12.34</span></div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, "newer model", extractNewerModelValue(doc), "There is a newer model of this item: Replacement product title $12.34")
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
	if inspectionHeaders[len(inspectionHeaders)-1] != "Newer model" {
		t.Fatalf("last header = %q, want Newer model", inspectionHeaders[len(inspectionHeaders)-1])
	}
	rows := [][]string{
		inspectionHeaders,
		{"Product", "https://www.amazon.com/dp/B0FNMPQSJC", "B0FNMPQSJC", "$199.99", "10%", " ", " ", "-32%", "4.3", "7", "", "", "", "", "Amazon's Choice", "Frequently returned item", "There is a newer model of this item: Product"},
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
