package main

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseReviewItems(t *testing.T) {
	html := `<html><body>
<div id="cm-cr-dp-review-list">
  <div data-hook="review" id="customer_review-R123">
    <span class="a-profile-name">Jane Doe</span>
    <i data-hook="review-star-rating"><span class="a-icon-alt">5.0 out of 5 stars</span></i>
    <a data-hook="review-title" href="/gp/customer-reviews/R123/ref=cm_cr_getr_d_rvw_ttl">
      <span>5.0 out of 5 stars</span><span>Works well</span>
    </a>
    <span data-hook="review-date">Reviewed in the United States on July 1, 2026</span>
    <span data-hook="avp-badge">Verified Purchase</span>
    <span data-hook="review-body"><span>Clear review body text.</span></span>
    <span data-hook="helpful-vote-statement">One person found this helpful</span>
    <div class="review-image-tile-section">
      <img data-hook="review-image-tile"
        src="https://m.media-amazon.com/images/I/abc._SY88.jpg"
        srcset="https://m.media-amazon.com/images/I/abc._SY88.jpg 1x, https://m.media-amazon.com/images/I/abc._SY176.jpg 2x"
        data-a-dynamic-image="{&quot;https://m.media-amazon.com/images/I/full.jpg&quot;:[1200,900]}">
    </div>
  </div>
  <div data-hook="review" id="R456">
    <span class="a-profile-name">Alex</span>
    <i data-hook="review-star-rating"><span class="a-icon-alt">4 out of 5 stars</span></i>
    <a class="a-size-base a-link-normal" href="/portal/customer-reviews/srp/-/R456/ref=cm_cr_dp_d_rvw_ttl">
      <h5 data-hook="reviewTitle">New review title</h5>
    </a>
    <div data-hook="reviewTextContainer">
      <div data-hook="reviewText">
        <div class="a-teaser-describedby-collapsed a-hidden">Brief content visible, double tap to read full content.</div>
        <div data-hook="reviewRichContentContainer"><p><span>New rich review body.</span></p></div>
      </div>
    </div>
  </div>
</div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	reviews := parseReviewItems(doc, "product_page", "https://www.amazon.com/dp/B0FD6LHC23", "B0FD6LHC23", "https://www.amazon.com/dp/B0FD6LHC23")
	if len(reviews) != 2 {
		t.Fatalf("review count = %d", len(reviews))
	}
	review := reviews[0]
	assertEqual(t, "review id", review.ReviewID, "R123")
	assertEqual(t, "reviewer", review.ReviewerName, "Jane Doe")
	assertEqual(t, "rating", review.Rating, "5.0")
	assertEqual(t, "title", review.Title, "Works well")
	assertEqual(t, "body", review.Body, "Clear review body text.")
	assertEqual(t, "detail url", review.DetailURL, "https://www.amazon.com/gp/customer-reviews/R123/ref=cm_cr_getr_d_rvw_ttl")
	if len(review.ImageURLs) != 2 {
		t.Fatalf("image url count = %d, urls=%v", len(review.ImageURLs), review.ImageURLs)
	}
	assertEqual(t, "original review image url", review.ImageURLs[0], "https://m.media-amazon.com/images/I/abc.jpg")

	newReview := reviews[1]
	assertEqual(t, "new title", newReview.Title, "New review title")
	assertEqual(t, "new body", newReview.Body, "New rich review body.")
	assertEqual(t, "new detail url", newReview.DetailURL, "https://www.amazon.com/portal/customer-reviews/srp/-/R456/ref=cm_cr_dp_d_rvw_ttl")
}

func TestEnsureFirstReviewPage(t *testing.T) {
	got := ensureFirstReviewPage("https://www.amazon.com/product-reviews/B0FD6LHC23?sortBy=recent")
	if !strings.Contains(got, "pageNumber=1") {
		t.Fatalf("missing pageNumber: %s", got)
	}
	if !strings.Contains(got, "reviewerType=all_reviews") {
		t.Fatalf("missing reviewerType: %s", got)
	}
}

func TestExtractCustomerPhotos(t *testing.T) {
	html := `<html><body>
<div id="customerReviews-media">
  <button data-mix-operations="CRImageThumbnailOpsClickHandler"
    data-mediatype="IMAGE"
    data-url="https://m.media-amazon.com/images/I/713u-Bv4MRL._AC_UC154,154_CACC,154,154_QL85_.jpg?aicid=community-reviews"
    data-thumbnailurl="https://m.media-amazon.com/images/I/713u-Bv4MRL._SY250_.jpg"
    data-physicalid="713u-Bv4MRL"
    data-reviewid="R2NIYXCUTQBCKR"
    data-extension="jpg"></button>
  <button data-mix-operations="CRImageThumbnailOpsClickHandler"
    data-mediatype="IMAGE"
    data-url="https://m.media-amazon.com/images/I/713u-Bv4MRL._SY500_.jpg"
    data-physicalid="713u-Bv4MRL"
    data-extension="jpg"></button>
  <button data-mix-operations="CRImageThumbnailOpsClickHandler"
    data-mediatype="VIDEO"
    data-url="https://m.media-amazon.com/images/I/video-thumb._SY250_.jpg"
    data-physicalid="video-thumb"
    data-extension="jpg"></button>
</div>
</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	photos := extractCustomerPhotos(doc, "https://www.amazon.com/dp/B0FD6LHC23")
	if len(photos) != 1 {
		t.Fatalf("photo count = %d, photos=%v", len(photos), photos)
	}
	assertEqual(t, "physical id", photos[0].PhysicalID, "713u-Bv4MRL")
	assertEqual(t, "customer photo url", photos[0].ImageURL, "https://m.media-amazon.com/images/I/713u-Bv4MRL.jpg")
	assertEqual(t, "review id", photos[0].ReviewID, "R2NIYXCUTQBCKR")
}

func TestOriginalAmazonImageURL(t *testing.T) {
	tests := map[string]string{
		"https://m.media-amazon.com/images/I/713u-Bv4MRL._SY500_.jpg":                                                  "https://m.media-amazon.com/images/I/713u-Bv4MRL.jpg",
		"https://m.media-amazon.com/images/I/61iLrVNSmrL._AC_UC154,154_CACC,154,154_QL85_.jpg?aicid=community-reviews": "https://m.media-amazon.com/images/I/61iLrVNSmrL.jpg",
		"https://m.media-amazon.com/images/I/713u-Bv4MRL.jpg":                                                          "https://m.media-amazon.com/images/I/713u-Bv4MRL.jpg",
	}
	for input, expected := range tests {
		assertEqual(t, input, originalAmazonImageURL(input), expected)
	}
}
