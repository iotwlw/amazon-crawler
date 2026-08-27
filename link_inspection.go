package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
	"golang.org/x/net/html"
)

var (
	linkASINRe             = regexp.MustCompile(`(?i)/(?:dp|gp/product)/([A-Z0-9]{10})`)
	bareASINRe             = regexp.MustCompile(`(?i)(^|[^A-Z0-9])([A-Z0-9]{10})([^A-Z0-9]|$)`)
	promoAmountRe          = regexp.MustCompile(`(?i)(\d{1,3}%|\$\d+(?:\.\d+)?)`)
	moneyAmountRe          = regexp.MustCompile(`\$?\s*([0-9][0-9,]*(?:\.\d{1,2})?)`)
	localizedMoneyAmountRe = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)
	firstNumberRe          = regexp.MustCompile(`\d+`)
	decimalNumberRe        = regexp.MustCompile(`\d+(?:\.\d+)?`)
	soldByNameRe           = regexp.MustCompile(`(?i)\bsold by\s+(.+?)(?:\s+(?:ships from|(?:and\s+)?fulfilled by|returns|payment|secure transaction)\b|$)`)
	parentASINJSONRe       = regexp.MustCompile(`\\?"parentAsin\\?"\s*:\s*\\?"([A-Z0-9]{10})\\?"`)
	twisterParentRefRe     = regexp.MustCompile(`(?i)ref=twister_([A-Z0-9]{10})`)

	featuredOfferEvidenceSelectors = []string{
		"#desktop_buybox",
		"#buybox",
		"#buybox_feature_div",
		"#qualifiedBuyBox",
		"#unqualifiedBuyBox_feature_div",
		"#apex_desktop_buybox",
		"#offerDisplayGroup",
		"#rightCol",
		"[id*=\"BuyBox\"]",
		"[id*=\"buybox\"]",
	}
	availabilityEvidenceSelectors = []string{
		"#availability",
		"#availabilityInsideBuyBox_feature_div",
		"#outOfStock",
		"#desktop_buybox",
		"#buybox",
		"#buybox_feature_div",
		"#rightCol",
	}
	inspectionHeaders = []string{
		"产品",
		"原ASIN",
		"ASIN",
		"价格",
		"优惠券",
		"是否秒杀",
		"会员专享",
		"显示折扣",
		"评级",
		"评价数量",
		"PromoCheck",
		"Promotion",
		"PromoCode",
		"Keep",
		"Choice",
		"Frequently returned item",
		"Newer model",
		"ParentASIN",
	}
)

const (
	availabilityStatusAvailable   = "available"
	availabilityStatusUnavailable = "unavailable"
	availabilityStatusUnknown     = "unknown"

	featuredOfferStatusPresent  = "present"
	featuredOfferStatusMissing  = "missing"
	featuredOfferStatusUsedOnly = "used_only"
	featuredOfferStatusUnknown  = "unknown"

	variantPriceStatus                = "不可售-变体"
	usedOfferContainerSelector        = "#usedBuyBox, #usedBuyBox_feature_div, #usedAccordionRow"
	unrelatedOfferContainerSelector   = "#productQuickView_feature_div, #a-popover-pqvOverlay"
	nonFeaturedOfferContainerSelector = usedOfferContainerSelector + ", " + unrelatedOfferContainerSelector
)

// LinkInspector implements the EasySpider-compatible product link inspection mode.
type LinkInspector struct {
	inputFile     string
	defaultDomain string
	outputFile    string
	results       []LinkInspectionResult
	robots        map[string]Robots
}

type LinkInspectionItem struct {
	Original string
	URL      string
	ASIN     string
	Domain   string
}

type LinkInspectionResult struct {
	Item                LinkInspectionItem
	Product             string
	ASIN                string
	ActualASIN          string
	ParentASIN          string
	FinalURL            string
	Price               string
	PriceValue          *float64
	Currency            string
	AvailabilityStatus  string
	FeaturedOfferStatus string
	FeaturedOfferText   string
	SellerID            string
	SellerName          string
	Coupon              string
	IsDeal              string
	PrimeExclusive      string
	DisplayDiscount     string
	Rating              string
	ReviewCount         int
	PromoCheck          string
	Promotion           string
	PromoCode           string
	Keep                string
	Choice              string
	FrequentReturn      string
	NewerModel          string
	ErrorMessage        string
}

type inspectionPage struct {
	Document *goquery.Document
	FinalURL string
}

type inspectionContractFields struct {
	AvailabilityStatus  string
	FeaturedOfferStatus string
	FeaturedOfferText   string
	SellerID            string
	SellerName          string
	PriceValue          *float64
	Currency            string
}

func NewLinkInspector(inputFile, domain, outputFile string) *LinkInspector {
	return &LinkInspector{
		inputFile:     inputFile,
		defaultDomain: normalizeDomain(domain),
		outputFile:    outputFile,
		results:       make([]LinkInspectionResult, 0),
		robots: map[string]Robots{
			normalizeDomain(app.Domain): robot,
		},
	}
}

func (s *LinkInspector) Run() error {
	items, err := loadLinkInspectionItems(s.inputFile, s.defaultDomain)
	if err != nil {
		return err
	}

	log.Infof("开始链接巡检，共 %d 条链接/ASIN", len(items))
	if _, err := app.get_cookie(); err != nil {
		log.Warnf("获取 Cookie 失败: %v，将不使用 Cookie", err)
	}

	successCount := 0
	for i, item := range items {
		log.Infof("进度: %d/%d - 巡检: %s", i+1, len(items), item.Original)
		result := s.inspectItem(context.Background(), item)
		if result.ErrorMessage == "" {
			successCount++
		} else {
			log.Warnf("巡检失败 ASIN:%s URL:%s 错误:%s", item.ASIN, item.URL, result.ErrorMessage)
		}
		s.results = append(s.results, result)

		if i < len(items)-1 {
			delay := 2 + rangdom_range(2)
			log.Infof("等待 %d 秒后继续...", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	outputFile := s.outputFile
	if outputFile == "" {
		outputFile = filepath.Join("output", fmt.Sprintf("link_inspection_%s.xlsx", time.Now().Format("20060102_150405")))
	}
	if err := writeInspectionXLSX(outputFile, inspectionRows(s.results)); err != nil {
		return err
	}

	log.Infof("链接巡检完成: 成功=%d 失败=%d 输出=%s", successCount, len(items)-successCount, outputFile)
	return nil
}

func (s *LinkInspector) inspectItem(ctx context.Context, item LinkInspectionItem) LinkInspectionResult {
	result := LinkInspectionResult{
		Item:                item,
		ASIN:                item.ASIN,
		ActualASIN:          item.ASIN,
		FinalURL:            item.URL,
		AvailabilityStatus:  availabilityStatusUnknown,
		FeaturedOfferStatus: featuredOfferStatusUnknown,
		Coupon:              " ",
		IsDeal:              " ",
		PrimeExclusive:      " ",
		DisplayDiscount:     " ",
	}

	page, err := s.fetchDocument(ctx, item)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	extracted := extractLinkInspectionPageFields(page.Document, item, page.FinalURL)
	extracted.ErrorMessage = result.ErrorMessage
	return extracted
}

func (s *LinkInspector) fetchDocument(ctx context.Context, item LinkInspectionItem) (inspectionPage, error) {
	fp := GetCurrentFingerprint()
	robots, err := s.robotsForDomain(item.Domain)
	if err != nil {
		return inspectionPage{}, err
	}
	if err := robots.IsAllow(fp.UserAgent, item.URL); err != nil {
		return inspectionPage{}, err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		client := get_client()
		req, err := http.NewRequestWithContext(ctx, "GET", item.URL, nil)
		if err != nil {
			return inspectionPage{}, err
		}
		ApplyFingerprint(req, GetRandomReferer(item.Domain))
		cookie := inspectionCookieSnapshot()
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		doc, readErr := documentFromResponse(resp)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if resp.StatusCode == http.StatusServiceUnavailable && attempt == 0 {
				RotateFingerprint()
				continue
			}
			continue
		}
		if isVerificationDocument(doc) {
			lastErr = ERROR_VERIFICATION
			if attempt == 0 {
				if err := refreshInspectionCookie(cookie); err != nil {
					log.Errorf("切换 Cookie 失败: %v", err)
					return inspectionPage{}, lastErr
				}
				continue
			}
			return inspectionPage{}, lastErr
		}
		finalURL := item.URL
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = resp.Request.URL.String()
		}
		return inspectionPage{Document: doc, FinalURL: finalURL}, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("请求失败")
	}
	return inspectionPage{}, lastErr
}

func (s *LinkInspector) robotsForDomain(domain string) (Robots, error) {
	domain = normalizeDomain(domain)
	if r, ok := s.robots[domain]; ok {
		return r, nil
	}

	fp := GetCurrentFingerprint()
	robotTxt := fmt.Sprintf("https://%s/robots.txt", domain)
	log.Infof("加载文件: %s", robotTxt)
	txt, err := request_get(robotTxt, fp.UserAgent)
	if err != nil {
		return Robots{}, fmt.Errorf("加载 robots.txt 失败: %w", err)
	}

	r := GetRobotFromTxt(txt)
	s.robots[domain] = r
	return r, nil
}

func documentFromResponse(resp *http.Response) (*goquery.Document, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func extractLinkInspectionFields(doc *goquery.Document, item LinkInspectionItem) LinkInspectionResult {
	return extractLinkInspectionPageFields(doc, item, item.URL)
}

func extractLinkInspectionPageFields(doc *goquery.Document, item LinkInspectionItem, finalURL string) LinkInspectionResult {
	reviewCount := extractReviewCountValue(textBySelectors(doc, []string{
		"#acrCustomerReviewText",
		"[data-hook=\"total-review-count\"]",
		"#averageCustomerReviews .a-size-base.a-color-secondary",
	}))

	asin := extractActualASINValueWithFinalURL(doc, item, finalURL)
	parentASIN := extractParentASINValue(doc, asin, item.ASIN)

	rating := extractRatingValue(textBySelectors(doc, []string{
		"#averageCustomerReviews span[aria-hidden=\"true\"]",
		"#averageCustomerReviews .a-icon-alt",
		"#acrPopover .a-icon-alt",
		"[data-hook=\"average-star-rating\"] .a-icon-alt",
	}))
	if reviewCount == 0 {
		rating = ""
	}
	currentPrice := extractCurrentPriceValue(doc)
	contract := extractInspectionContractFields(doc, item, currentPrice)
	price := currentPrice
	if isVariantASIN(item.ASIN, asin) {
		price = variantPriceStatus
	} else if strings.TrimSpace(price) == "" {
		price = extractPriceStatusValue(doc)
	}

	result := LinkInspectionResult{
		Item:                item,
		Product:             textBySelectors(doc, []string{"#productTitle"}),
		ASIN:                asin,
		ActualASIN:          asin,
		ParentASIN:          parentASIN,
		FinalURL:            inspectionFinalURL(finalURL, item.URL),
		Price:               price,
		PriceValue:          contract.PriceValue,
		Currency:            contract.Currency,
		AvailabilityStatus:  contract.AvailabilityStatus,
		FeaturedOfferStatus: contract.FeaturedOfferStatus,
		FeaturedOfferText:   contract.FeaturedOfferText,
		SellerID:            contract.SellerID,
		SellerName:          contract.SellerName,
		Coupon:              defaultSpace(extractCouponValue(doc)),
		IsDeal:              defaultSpace(textBySelectors(doc, []string{"#dealBadgeSupportingText", "#dealBadge_feature_div"})),
		PrimeExclusive:      defaultSpace(textBySelectors(doc, []string{"#primeExclusivePricingMessage .a-size-base", "#primeExclusivePricingMessage"})),
		DisplayDiscount:     defaultSpace(calculateDisplayDiscount(extractListPriceValue(doc), price)),
		Rating:              rating,
		ReviewCount:         reviewCount,
		PromoCheck:          extractPromoCheckValue(doc),
		Promotion:           extractPromotionValue(doc),
		PromoCode: textBySelectors(doc, []string{
			"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:nth-child(2) span span:nth-child(2)",
			"#promoPriceBlockMessage_feature_div span[id^=\"promoCode\"]",
		}),
		Keep: textBySelectors(doc, []string{
			"#NEW_1_nostos_badge",
		}),
		Choice:         extractChoiceValue(doc),
		FrequentReturn: extractFrequentlyReturnedValue(doc),
		NewerModel:     extractNewerModelValue(doc),
	}
	return result
}

func extractActualASINValue(doc *goquery.Document, item LinkInspectionItem) string {
	return extractActualASINValueWithFinalURL(doc, item, "")
}

func extractActualASINValueWithFinalURL(doc *goquery.Document, item LinkInspectionItem, finalURL string) string {
	asin := cleanText(attrBySelectors(doc, []string{"input#ASIN", "input[name=\"ASIN\"]"}, "value"))
	if asin != "" {
		return strings.ToUpper(asin)
	}
	if canonicalASIN := extractASINFromString(attrBySelectors(doc, []string{"link[rel=\"canonical\"]"}, "href")); canonicalASIN != "" {
		return canonicalASIN
	}
	if finalURLASIN := extractASINFromString(finalURL); finalURLASIN != "" {
		return finalURLASIN
	}
	return item.ASIN
}

func extractParentASINValue(doc *goquery.Document, currentASINs ...string) string {
	raw, err := goquery.OuterHtml(doc.Selection)
	if err != nil {
		return ""
	}
	current := make([]string, 0, len(currentASINs))
	for _, value := range currentASINs {
		if value = strings.ToUpper(strings.TrimSpace(value)); value != "" {
			current = append(current, value)
		}
	}

	text := html.UnescapeString(raw)
	if value := firstForeignParentASIN(parentASINJSONRe.FindAllStringSubmatch(text, -1), current); value != "" {
		return value
	}
	return firstForeignParentASIN(twisterParentRefRe.FindAllStringSubmatch(text, -1), current)
}

// firstForeignParentASIN returns the first captured ASIN that does not belong to
// the inspected page itself. Standalone products embed a parentAsin equal to
// their own ASIN in the media JSON, which counts as no merged parent.
func firstForeignParentASIN(matches [][]string, currentASINs []string) string {
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := strings.ToUpper(match[1])
		foreign := true
		for _, known := range currentASINs {
			if known == value {
				foreign = false
				break
			}
		}
		if foreign {
			return value
		}
	}
	return ""
}

func isVariantASIN(originalASIN, actualASIN string) bool {
	originalASIN = strings.ToUpper(strings.TrimSpace(originalASIN))
	actualASIN = strings.ToUpper(strings.TrimSpace(actualASIN))
	return originalASIN != "" && actualASIN != "" && originalASIN != actualASIN
}

func extractInspectionContractFields(doc *goquery.Document, item LinkInspectionItem, currentPrice string) inspectionContractFields {
	sellerID, sellerName := extractFeaturedOfferSeller(doc)
	featuredOfferStatus, featuredOfferText := extractFeaturedOfferState(doc, currentPrice, sellerID, sellerName)
	if featuredOfferStatus != featuredOfferStatusPresent {
		sellerID = ""
		sellerName = ""
	}

	priceValue, currency := extractStructuredPrice(currentPrice, item.Domain)
	return inspectionContractFields{
		AvailabilityStatus:  extractAvailabilityStatus(doc, featuredOfferStatus),
		FeaturedOfferStatus: featuredOfferStatus,
		FeaturedOfferText:   featuredOfferText,
		SellerID:            sellerID,
		SellerName:          sellerName,
		PriceValue:          priceValue,
		Currency:            currency,
	}
}

func extractFeaturedOfferState(doc *goquery.Document, currentPrice, sellerID, sellerName string) (string, string) {
	if evidence := extractPrimaryUsedOnlyEvidence(doc, currentPrice, sellerID, sellerName); evidence != "" {
		return featuredOfferStatusUsedOnly, evidence
	}

	missingPhrases := []string{
		"no featured offers available",
		"no featured offer available",
	}
	missingEvidence := scopedEvidenceTextContainingAny(
		doc,
		featuredOfferEvidenceSelectors,
		missingPhrases,
		800,
		nonFeaturedOfferContainerSelector,
	)

	usedPhrases := []string{
		"buy used",
		"used:",
		"condition: used",
		"sold by amazon resale",
		"pre-owned",
	}
	usedEvidence := scopedEvidenceTextContainingAny(
		doc,
		featuredOfferEvidenceSelectors,
		usedPhrases,
		800,
		unrelatedOfferContainerSelector,
	)
	hasPresentSignal := hasFeaturedOfferSignal(doc, currentPrice, sellerID, sellerName)
	if usedEvidence != "" && !hasPresentSignal {
		return featuredOfferStatusUsedOnly, usedEvidence
	}
	if missingEvidence != "" {
		return featuredOfferStatusMissing, missingEvidence
	}
	if hasPresentSignal {
		return featuredOfferStatusPresent, buildFeaturedOfferEvidence(doc, currentPrice, sellerName)
	}
	if extractUnavailableEvidence(doc) != "" {
		return featuredOfferStatusUnknown, ""
	}
	if usedEvidence != "" {
		return featuredOfferStatusUsedOnly, usedEvidence
	}

	return featuredOfferStatusUnknown, ""
}

func extractPrimaryUsedOnlyEvidence(doc *goquery.Document, currentPrice, sellerID, sellerName string) string {
	dedicatedUsedPhrases := []string{
		"buy used",
		"used:",
		"condition: used",
		"used - like",
		"used - good",
		"amazon resale",
		"pre-owned",
	}
	if evidence := selectionEvidenceContainingAny(doc, []string{
		"#usedBuyBox",
		"#usedBuyBox_feature_div",
		"#usedAccordionRow",
	}, dedicatedUsedPhrases, 800); evidence != "" && !hasFeaturedOfferSignal(doc, currentPrice, sellerID, sellerName) {
		return evidence
	}

	primaryConditionPhrases := []string{
		"condition: used",
		"sold by amazon resale",
		"pre-owned",
	}
	return selectionEvidenceContainingAnyExcluding(doc, []string{
		"#desktop_buybox",
		"#buybox",
		"#buybox_feature_div",
		"#qualifiedBuyBox",
	}, primaryConditionPhrases, 800, usedOfferContainerSelector)
}

func extractAvailabilityStatus(doc *goquery.Document, featuredOfferStatus string) string {
	if featuredOfferStatus == featuredOfferStatusPresent || featuredOfferStatus == featuredOfferStatusUsedOnly {
		return availabilityStatusAvailable
	}
	if extractUnavailableEvidence(doc) != "" {
		return availabilityStatusUnavailable
	}
	if hasFeaturedOfferPurchaseControl(doc) {
		return availabilityStatusAvailable
	}

	availablePhrases := []string{
		"in stock",
		"left in stock",
		"available to ship",
		"see all buying options",
	}
	if scopedEvidenceTextContainingAny(
		doc,
		availabilityEvidenceSelectors,
		availablePhrases,
		800,
		nonFeaturedOfferContainerSelector,
	) != "" {
		return availabilityStatusAvailable
	}

	return availabilityStatusUnknown
}

func extractUnavailableEvidence(doc *goquery.Document) string {
	return scopedEvidenceTextContainingAny(
		doc,
		availabilityEvidenceSelectors,
		[]string{
			"currently unavailable",
			"we don't know when or if this item will be back in stock",
			"temporarily out of stock",
			"not available for purchase",
		},
		800,
		nonFeaturedOfferContainerSelector,
	)
}

func hasFeaturedOfferSignal(doc *goquery.Document, currentPrice, sellerID, sellerName string) bool {
	if hasFeaturedOfferPurchaseControl(doc) {
		return true
	}
	if strings.TrimSpace(currentPrice) == "" {
		return false
	}
	return sellerID != "" || sellerName != ""
}

func hasFeaturedOfferPurchaseControl(doc *goquery.Document) bool {
	for _, selector := range purchaseControlSelectors() {
		found := false
		doc.Find(selector).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			if selectionInsideContainer(selection, nonFeaturedOfferContainerSelector) {
				return true
			}
			found = true
			return false
		})
		if found {
			return true
		}
	}
	return false
}

func purchaseControlSelectors() []string {
	return []string{
		"#add-to-cart-button",
		"#buy-now-button",
		"input[name=\"submit.add-to-cart\"]",
		"input[name=\"submit.buy-now\"]",
		"form#addToCart",
	}
}

func scopedEvidenceTextContainingAny(
	doc *goquery.Document,
	selectors, phrases []string,
	maxLen int,
	excludedSelector string,
) string {
	best := ""
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			best = shorterEvidenceTextExcluding(best, selection, phrases, maxLen, excludedSelector)
			selection.Find("div, section, aside, table, tr, td, li, p, span").Each(func(_ int, descendant *goquery.Selection) {
				best = shorterEvidenceTextExcluding(best, descendant, phrases, maxLen, excludedSelector)
			})
		})
	}
	return best
}

func selectionEvidenceContainingAny(doc *goquery.Document, selectors, phrases []string, maxLen int) string {
	best := ""
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			best = shorterEvidenceText(best, selection, phrases, maxLen)
		})
	}
	return best
}

func shorterEvidenceText(best string, selection *goquery.Selection, phrases []string, maxLen int) string {
	text := selectionTextWithoutScripts(selection)
	if text == "" || (maxLen > 0 && len(text) > maxLen) || !containsAnyPhrase(text, phrases) {
		return best
	}
	if best == "" || len(text) < len(best) {
		return text
	}
	return best
}

func shorterEvidenceTextExcluding(
	best string,
	selection *goquery.Selection,
	phrases []string,
	maxLen int,
	excludedSelector string,
) string {
	if selectionInsideContainer(selection, excludedSelector) {
		return best
	}
	if excludedSelector == "" || selection.Find(excludedSelector).Length() == 0 {
		return shorterEvidenceText(best, selection, phrases, maxLen)
	}

	clone := selection.Clone()
	clone.Find(excludedSelector).Remove()
	return shorterEvidenceText(best, clone, phrases, maxLen)
}

func selectionEvidenceContainingAnyExcluding(
	doc *goquery.Document,
	selectors []string,
	phrases []string,
	maxLen int,
	excludedSelector string,
) string {
	best := ""
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			clone := selection.Clone()
			clone.Find(excludedSelector).Remove()
			text := selectionTextWithoutScripts(clone)
			if text == "" || (maxLen > 0 && len(text) > maxLen) || !containsAnyPhrase(text, phrases) {
				return
			}
			if best == "" || len(text) < len(best) {
				best = text
			}
		})
	}
	return best
}

func containsAnyPhrase(text string, phrases []string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range phrases {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func buildFeaturedOfferEvidence(doc *goquery.Document, currentPrice, sellerName string) string {
	parts := make([]string, 0, 4)
	parts = appendEvidencePart(parts, currentPrice)
	parts = appendEvidencePart(parts, textByFeaturedOfferSelectors(doc, []string{
		"#availabilityInsideBuyBox_feature_div",
		"#availability",
	}))
	parts = appendEvidencePart(parts, textByFeaturedOfferSelectors(doc, []string{
		"#offerDisplayFeatures_desktop",
		"#merchantInfoFeature_feature_div",
		"#fulfillerInfoFeature_feature_div",
		"#merchant-info",
		"#tabular-buybox",
		"#tabular-buybox-truncate-1",
	}))
	parts = appendEvidencePart(parts, sellerName)
	return cleanText(strings.Join(parts, " "))
}

func appendEvidencePart(parts []string, value string) []string {
	value = cleanText(value)
	if value == "" {
		return parts
	}
	for _, existing := range parts {
		if existing == value || strings.Contains(existing, value) {
			return parts
		}
	}
	return append(parts, value)
}

func extractFeaturedOfferSeller(doc *goquery.Document) (string, string) {
	sellerID := ""
	sellerName := ""
	selectors := []string{
		"#sellerProfileTriggerId",
		"#vse-seller-link",
		"#merchant-info a[href*=\"seller=\"]",
		"#merchant-info a[href*=\"/sp\"]",
		"#tabular-buybox a[href*=\"seller=\"]",
		"#tabular-buybox-container a[href*=\"seller=\"]",
		"span.tabular-buybox-text a[href*=\"seller=\"]",
		"div.tabular-buybox-container a[href*=\"seller=\"]",
		"#desktop_buybox a[href*=\"seller=\"]",
		"#buybox a[href*=\"seller=\"]",
	}
	for _, selector := range selectors {
		doc.Find(selector).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			if selectionInsideContainer(selection, nonFeaturedOfferContainerSelector) {
				return true
			}
			name := cleanText(selection.Text())
			if sellerName == "" && name != "" {
				sellerName = name
			}
			href, _ := selection.Attr("href")
			if id := extractSellerIDFromHref(href); id != "" {
				sellerID = id
				if name != "" {
					sellerName = name
				}
				return false
			}
			return true
		})
		if sellerID != "" {
			break
		}
	}

	if sellerID == "" {
		sellerID = cleanText(attrByFeaturedOfferSelectors(doc, []string{
			"input#merchantID",
			"input[name=\"merchantID\"]",
			"input[name=\"merchantId\"]",
			"input[name=\"sellerID\"]",
		}, "value"))
	}
	if sellerName == "" {
		sellerName = textByFeaturedOfferSelectors(doc, []string{
			"#sellerProfileTriggerId",
			"#vse-seller-link",
			"#tabular-buybox-truncate-1 .tabular-buybox-text",
			"#tabular-buybox .tabular-buybox-text",
		})
	}
	if sellerName == "" {
		sellerName = extractSellerNameFromMerchantText(textByFeaturedOfferSelectors(doc, []string{
			"#merchant-info",
			"#merchantInfoFeature_feature_div",
			"#tabular-buybox",
			"#tabular-buybox-container",
		}))
	}
	return sellerID, sellerName
}

func selectionInsideContainer(selection *goquery.Selection, selector string) bool {
	return selector != "" && (selection.Is(selector) || selection.ParentsFiltered(selector).Length() > 0)
}

func attrByFeaturedOfferSelectors(doc *goquery.Document, selectors []string, attrName string) string {
	for _, selector := range selectors {
		value := ""
		doc.Find(selector).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			if selectionInsideContainer(selection, nonFeaturedOfferContainerSelector) {
				return true
			}
			if candidate, ok := selection.Attr(attrName); ok {
				value = candidate
				return false
			}
			return true
		})
		if value != "" {
			return value
		}
	}
	return ""
}

func textByFeaturedOfferSelectors(doc *goquery.Document, selectors []string) string {
	for _, selector := range selectors {
		value := ""
		doc.Find(selector).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			if selectionInsideContainer(selection, nonFeaturedOfferContainerSelector) {
				return true
			}
			value = cleanText(selection.Text())
			return value == ""
		})
		if value != "" {
			return value
		}
	}
	return ""
}

func extractSellerIDFromHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if parsed, err := url.Parse(href); err == nil {
		for _, key := range []string{"seller", "me", "smid"} {
			if value := strings.TrimSpace(parsed.Query().Get(key)); value != "" {
				return value
			}
		}
	}

	for _, marker := range []string{"seller=", "me=", "smid="} {
		index := strings.Index(strings.ToLower(href), marker)
		if index < 0 {
			continue
		}
		value := href[index+len(marker):]
		if end := strings.IndexAny(value, "&#"); end >= 0 {
			value = value[:end]
		}
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func extractSellerNameFromMerchantText(text string) string {
	match := soldByNameRe.FindStringSubmatch(cleanText(text))
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(cleanText(match[1]), " .,:;-")
}

func extractStructuredPrice(priceText, domain string) (*float64, string) {
	value, ok := extractLocalizedMoneyValue(priceText)
	if !ok {
		return nil, ""
	}
	return &value, currencyCodeForPrice(priceText, domain)
}

func extractLocalizedMoneyValue(text string) (float64, bool) {
	compactText := strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) {
			return -1
		}
		return char
	}, text)
	match := localizedMoneyAmountRe.FindString(compactText)
	if match == "" {
		return 0, false
	}

	lastComma := strings.LastIndex(match, ",")
	lastDot := strings.LastIndex(match, ".")
	lastSeparator := lastComma
	if lastDot > lastSeparator {
		lastSeparator = lastDot
	}
	decimalSeparator := -1
	if lastSeparator >= 0 {
		digitsAfter := len(match) - lastSeparator - 1
		if digitsAfter == 1 || digitsAfter == 2 {
			decimalSeparator = lastSeparator
		}
	}

	var normalized strings.Builder
	for index, char := range match {
		switch {
		case char >= '0' && char <= '9':
			normalized.WriteRune(char)
		case index == decimalSeparator:
			normalized.WriteByte('.')
		}
	}
	value, err := strconv.ParseFloat(normalized.String(), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func currencyCodeForPrice(priceText, domain string) string {
	upper := strings.ToUpper(strings.ReplaceAll(cleanText(priceText), " ", ""))
	switch {
	case strings.Contains(upper, "US$") || strings.Contains(upper, "USD"):
		return "USD"
	case strings.Contains(upper, "MX$") || strings.Contains(upper, "MXN"):
		return "MXN"
	case strings.Contains(upper, "CA$") || strings.Contains(upper, "C$") || strings.Contains(upper, "CAD"):
		return "CAD"
	case strings.Contains(upper, "AU$") || strings.Contains(upper, "A$") || strings.Contains(upper, "AUD"):
		return "AUD"
	case strings.Contains(upper, "S$") || strings.Contains(upper, "SGD"):
		return "SGD"
	case strings.Contains(upper, "R$") || strings.Contains(upper, "BRL"):
		return "BRL"
	case strings.Contains(upper, "£") || strings.Contains(upper, "GBP"):
		return "GBP"
	case strings.Contains(upper, "€") || strings.Contains(upper, "EUR"):
		return "EUR"
	case strings.Contains(upper, "₹") || strings.Contains(upper, "INR"):
		return "INR"
	case strings.Contains(upper, "¥") || strings.Contains(upper, "￥") || strings.Contains(upper, "JPY"):
		return "JPY"
	case strings.Contains(upper, "AED"):
		return "AED"
	case strings.Contains(upper, "SAR"):
		return "SAR"
	case strings.Contains(upper, "SEK"):
		return "SEK"
	case strings.Contains(upper, "PLN"):
		return "PLN"
	case strings.Contains(upper, "TRY") || strings.Contains(upper, "₺"):
		return "TRY"
	}
	return domainCurrencyCode(domain)
}

func domainCurrencyCode(domain string) string {
	switch normalizeDomain(domain) {
	case "amazon.com", "www.amazon.com":
		return "USD"
	case "amazon.com.mx", "www.amazon.com.mx":
		return "MXN"
	case "amazon.ca", "www.amazon.ca":
		return "CAD"
	case "amazon.co.uk", "www.amazon.co.uk":
		return "GBP"
	case "amazon.de", "www.amazon.de", "amazon.fr", "www.amazon.fr", "amazon.it", "www.amazon.it",
		"amazon.es", "www.amazon.es", "amazon.nl", "www.amazon.nl", "amazon.com.be", "www.amazon.com.be",
		"amazon.ie", "www.amazon.ie":
		return "EUR"
	case "amazon.co.jp", "www.amazon.co.jp":
		return "JPY"
	case "amazon.com.au", "www.amazon.com.au":
		return "AUD"
	case "amazon.in", "www.amazon.in":
		return "INR"
	case "amazon.com.br", "www.amazon.com.br":
		return "BRL"
	case "amazon.sg", "www.amazon.sg":
		return "SGD"
	case "amazon.ae", "www.amazon.ae":
		return "AED"
	case "amazon.sa", "www.amazon.sa":
		return "SAR"
	case "amazon.se", "www.amazon.se":
		return "SEK"
	case "amazon.pl", "www.amazon.pl":
		return "PLN"
	case "amazon.com.tr", "www.amazon.com.tr":
		return "TRY"
	default:
		return ""
	}
}

func inspectionFinalURL(finalURL, requestedURL string) string {
	if finalURL = strings.TrimSpace(finalURL); finalURL != "" {
		return finalURL
	}
	return strings.TrimSpace(requestedURL)
}

func loadLinkInspectionItems(inputFile, defaultDomain string) ([]LinkInspectionItem, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("读取链接文件失败: %w", err)
	}

	defaultDomain = normalizeDomain(defaultDomain)
	items := make([]LinkInspectionItem, 0)
	for lineNo, line := range strings.Split(string(data), "\n") {
		item, ok := parseLinkInspectionItem(line, defaultDomain)
		if !ok {
			if strings.TrimSpace(line) != "" {
				log.Warnf("跳过无法识别的链接/ASIN 第%d行: %s", lineNo+1, strings.TrimSpace(line))
			}
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("链接文件没有可识别的 ASIN 或商品链接")
	}
	return items, nil
}

func parseLinkInspectionItem(raw, defaultDomain string) (LinkInspectionItem, bool) {
	original := strings.TrimSpace(raw)
	if original == "" {
		return LinkInspectionItem{}, false
	}
	original = strings.TrimPrefix(original, "\uFEFF")

	candidate := original
	if strings.HasPrefix(strings.ToLower(candidate), "www.") {
		candidate = "https://" + candidate
	}

	domain := normalizeDomain(defaultDomain)
	if domain == "" {
		domain = "www.amazon.com"
	}

	var parsedURL *url.URL
	if u, err := url.Parse(candidate); err == nil && u.Host != "" {
		domain = normalizeDomain(u.Host)
		parsedURL = u
	}

	asin := extractASINFromString(candidate)
	if asin == "" {
		asin = extractASINFromString(original)
	}
	if asin == "" {
		return LinkInspectionItem{}, false
	}

	return LinkInspectionItem{
		Original: original,
		URL:      buildLinkInspectionProductURL(domain, asin, parsedURL),
		ASIN:     asin,
		Domain:   domain,
	}, true
}

func buildLinkInspectionProductURL(domain, asin string, parsedURL *url.URL) string {
	productURL := fmt.Sprintf("https://%s/dp/%s", domain, asin)
	if parsedURL == nil {
		return productURL
	}

	query := url.Values{}
	for _, key := range []string{"th", "psc", "smid", "m"} {
		for _, value := range parsedURL.Query()[key] {
			if strings.TrimSpace(value) != "" {
				query.Add(key, value)
			}
		}
	}
	if encoded := query.Encode(); encoded != "" {
		productURL += "?" + encoded
	}
	return productURL
}

func detectDomainFromLinkFile(inputFile string) (string, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		item, ok := parseLinkInspectionItem(line, "")
		if ok && item.Domain != "" {
			return item.Domain, nil
		}
	}
	return "", nil
}

func extractASINFromString(s string) string {
	if match := linkASINRe.FindStringSubmatch(s); len(match) > 1 {
		return strings.ToUpper(match[1])
	}
	if match := bareASINRe.FindStringSubmatch(s); len(match) > 2 {
		return strings.ToUpper(match[2])
	}
	return ""
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return strings.ToLower(domain)
}

func textBySelectors(doc *goquery.Document, selectors []string) string {
	for _, selector := range selectors {
		text := cleanText(doc.Find(selector).First().Text())
		if text != "" {
			return text
		}
	}
	return ""
}

func textByContentSelectors(doc *goquery.Document, selectors []string, requiredPhrases []string, maxLen int) string {
	for _, selector := range selectors {
		if text := firstSelectionTextContaining(doc.Find(selector), requiredPhrases, maxLen); text != "" {
			return text
		}
	}
	return fallbackModuleTextByPhrase(doc, requiredPhrases, maxLen)
}

func firstSelectionTextContaining(selection *goquery.Selection, requiredPhrases []string, maxLen int) string {
	best := ""
	selection.Each(func(_ int, s *goquery.Selection) {
		text := moduleTextWithoutScripts(s)
		if text == "" || !containsAllPhrases(text, requiredPhrases) {
			return
		}
		if maxLen > 0 && len(text) > maxLen {
			return
		}
		if best == "" || len(text) < len(best) {
			best = text
		}
	})
	return best
}

func fallbackModuleTextByPhrase(doc *goquery.Document, requiredPhrases []string, maxLen int) string {
	best := ""
	bestWithDetails := ""
	doc.Find("div, section, aside, table, tr, td, li").Each(func(_ int, s *goquery.Selection) {
		text := moduleTextWithoutScripts(s)
		if text == "" || !containsAllPhrases(text, requiredPhrases) {
			return
		}
		if maxLen > 0 && len(text) > maxLen {
			return
		}
		if moduleHasDetails(s, text) {
			if bestWithDetails == "" || len(text) < len(bestWithDetails) {
				bestWithDetails = text
			}
			return
		}
		if best == "" || len(text) < len(best) {
			best = text
		}
	})
	if bestWithDetails != "" {
		return bestWithDetails
	}
	return best
}

func moduleHasDetails(selection *goquery.Selection, text string) bool {
	lower := strings.ToLower(text)
	return selection.Find("a").Length() > 0 ||
		strings.Contains(text, "$") ||
		strings.Contains(lower, "left in stock") ||
		strings.Contains(lower, "customer reviews")
}

func moduleTextWithoutScripts(selection *goquery.Selection) string {
	clone := selection.Clone()
	clone.Find("script, style").Remove()
	parts := make([]string, 0)
	collectTextParts(clone, &parts)
	return cleanText(strings.Join(parts, " "))
}

func collectTextParts(selection *goquery.Selection, parts *[]string) {
	selection.Contents().Each(func(_ int, child *goquery.Selection) {
		node := child.Get(0)
		if node == nil {
			return
		}
		switch node.Type {
		case html.TextNode:
			text := strings.TrimSpace(node.Data)
			if text != "" {
				*parts = append(*parts, text)
			}
		case html.ElementNode:
			if node.Data == "script" || node.Data == "style" {
				return
			}
			collectTextParts(child, parts)
		}
	})
}

func containsAllPhrases(text string, phrases []string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range phrases {
		if !strings.Contains(lower, strings.ToLower(phrase)) {
			return false
		}
	}
	return true
}

func attrBySelectors(doc *goquery.Document, selectors []string, attrName string) string {
	for _, selector := range selectors {
		if val, ok := doc.Find(selector).First().Attr(attrName); ok {
			return val
		}
	}
	return ""
}

func cleanText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	return strings.Join(strings.Fields(text), " ")
}

func defaultSpace(text string) string {
	if strings.TrimSpace(text) == "" {
		return " "
	}
	return text
}

func extractPromoAmount(text string) string {
	match := promoAmountRe.FindString(text)
	return cleanText(match)
}

func extractCurrentPriceValue(doc *goquery.Document) string {
	return textBySelectors(doc, []string{
		"#corePriceDisplay_desktop_feature_div .priceToPay .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .apexPriceToPay .a-offscreen",
		"#corePrice_feature_div .priceToPay .a-offscreen",
		"#corePrice_feature_div .apexPriceToPay .a-offscreen",
		"#corePriceDisplay_desktop_feature_div [data-a-color=\"price\"] .a-offscreen",
		"#corePrice_feature_div [data-a-color=\"price\"] .a-offscreen",
		"#corePrice_feature_div .a-price:not(.a-text-price) .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .a-price:not(.a-text-price) .a-offscreen",
		"#corePrice_feature_div .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .a-offscreen",
	})
}

func extractListPriceValue(doc *goquery.Document) string {
	return textBySelectors(doc, []string{
		"#corePriceDisplay_desktop_feature_div .basisPrice .a-offscreen",
		"#corePrice_feature_div .basisPrice .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .a-text-price .a-offscreen",
		"#corePrice_feature_div .a-text-price .a-offscreen",
		"#corePriceDisplay_desktop_feature_div [data-a-strike=\"true\"] .a-offscreen",
		"#corePrice_feature_div [data-a-strike=\"true\"] .a-offscreen",
	})
}

func extractPriceStatusValue(doc *goquery.Document) string {
	selectors := []string{
		"#rightCol",
		"#availability",
		"#availabilityInsideBuyBox_feature_div",
		"#outOfStock",
		"#unqualifiedBuyBox_feature_div",
		"#desktop_buybox",
		"#buybox",
		"#buybox_feature_div",
		"#apex_desktop",
		"#apex_desktop_buybox",
		"#qualifiedBuyBox",
		"#offerDisplayGroup",
		"[id*=\"BuyBox\"]",
		"[id*=\"buybox\"]",
		"[id*=\"offer\"]",
		"[id*=\"Offer\"]",
	}
	for _, selector := range selectors {
		if value := firstPriceStatusFromSelectionExcluding(
			doc.Find(selector),
			unrelatedOfferContainerSelector,
		); value != "" {
			return value
		}
	}
	return ""
}

func firstPriceStatusFromSelection(selection *goquery.Selection) string {
	return firstPriceStatusFromSelectionExcluding(selection, "")
}

func firstPriceStatusFromSelectionExcluding(selection *goquery.Selection, excludedSelector string) string {
	var value string
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if selectionInsideContainer(s, excludedSelector) {
			return true
		}
		if excludedSelector == "" || s.Find(excludedSelector).Length() == 0 {
			value = priceStatusFromText(selectionTextWithoutScripts(s))
			return value == ""
		}

		clone := s.Clone()
		clone.Find(excludedSelector).Remove()
		value = priceStatusFromText(selectionTextWithoutScripts(clone))
		return value == ""
	})
	return value
}

func priceStatusFromText(text string) string {
	normalized := strings.ToLower(cleanText(text))
	switch {
	case strings.Contains(normalized, "currently unavailable") ||
		strings.Contains(normalized, "we don't know when or if this item will be back in stock") ||
		strings.Contains(normalized, "temporarily out of stock"):
		return "不可售"
	case strings.Contains(normalized, "buy used") ||
		strings.Contains(normalized, "used:") ||
		strings.Contains(normalized, "sold by amazon resale"):
		return "二手跟卖"
	case strings.Contains(normalized, "no featured offers available"):
		return "没有购物车"
	default:
		return ""
	}
}

func calculateDisplayDiscount(listPriceText, priceText string) string {
	listPrice, ok := extractMoneyValue(listPriceText)
	if !ok {
		return ""
	}
	price, ok := extractMoneyValue(priceText)
	if !ok || listPrice <= 0 || price <= 0 || price >= listPrice {
		return ""
	}

	discount := int(math.Round((listPrice - price) / listPrice * 100))
	if discount <= 0 {
		return ""
	}
	return fmt.Sprintf("-%d%%", discount)
}

func extractMoneyValue(text string) (float64, bool) {
	match := moneyAmountRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func extractCouponValue(doc *goquery.Document) string {
	selectors := []string{
		"#promoPriceBlockMessage_feature_div [id^=\"couponText\"]",
		"#promoPriceBlockMessage_feature_div .couponLabelText",
		"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:first-child > span:first-child label",
		"#promoPriceBlockMessage_feature_div label[id^=\"couponText\"]",
	}
	for _, selector := range selectors {
		if amount := firstPromoAmountFromSelection(doc.Find(selector)); amount != "" {
			return amount
		}
	}

	return firstPromoAmountFromSelection(doc.Find("#promoPriceBlockMessage_feature_div .promoPriceBlockMessage"))
}

func firstPromoAmountFromSelection(selection *goquery.Selection) string {
	var amount string
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := selectionTextWithoutScripts(s)
		if text == "" {
			return true
		}
		amount = extractPromoAmount(text)
		return amount == ""
	})
	return amount
}

func selectionTextWithoutScripts(selection *goquery.Selection) string {
	clone := selection.Clone()
	clone.Find("script, style").Remove()
	return cleanText(clone.Text())
}

func extractPromoCheckValue(doc *goquery.Document) string {
	text := textBySelectors(doc, []string{
		"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:first-child > span:first-child > div > div",
	})

	// EasySpider's saved sample leaves PromoCheck blank for ordinary coupon text;
	// keep this field for non-coupon promo markers only.
	lower := strings.ToLower(text)
	if lower == "" || strings.Contains(lower, "coupon") || strings.Contains(lower, "terms") || strings.Contains(lower, "shop items") {
		return ""
	}
	return text
}

func extractPromotionValue(doc *goquery.Document) string {
	if text := textBySelectors(doc, []string{
		"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:nth-child(2) label",
		"label[id^=\"greenBadge\"]",
		"span[id^=\"promotion_title\"]",
	}); text != "" {
		return text
	}

	selectors := []string{
		"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage",
		"#promoPriceBlockMessage_feature_div",
		"[id*=\"brandPromotion\"]",
		"[id*=\"brand-promotion\"]",
		"[cel_widget_id*=\"brandPromotion\"]",
		"[data-feature-name*=\"brandPromotion\"]",
	}
	for _, selector := range selectors {
		if text := firstPromotionTextFromSelection(doc.Find(selector)); text != "" {
			return text
		}
	}
	if hasSponsoredBrandPromotion(doc) && extractCouponValue(doc) == "" && !hasOrdinaryPromotionMarker(doc) {
		return "Brand Promotion"
	}
	return ""
}

func firstPromotionTextFromSelection(selection *goquery.Selection) string {
	var promotion string
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := selectionTextWithoutScripts(s)
		if text == "" {
			return true
		}
		promotion = normalizePromotionText(text)
		return promotion == ""
	})
	return promotion
}

func normalizePromotionText(text string) string {
	text = cleanText(text)
	lower := strings.ToLower(text)
	if lower == "" {
		return ""
	}
	if strings.Contains(lower, "brand promotion") {
		return "Brand Promotion"
	}
	if strings.Contains(lower, "coupon") || strings.Contains(lower, "subscribe & save") {
		return ""
	}
	if strings.Contains(lower, "save") || strings.Contains(lower, "promotion") {
		return text
	}
	return ""
}

func hasOrdinaryPromotionMarker(doc *goquery.Document) bool {
	return doc.Find("[data-csa-c-item-id*=\"amzn1.promotion\"], label[id^=\"greenBadge\"], span[id^=\"promotion_title\"]").Length() > 0
}

func hasSponsoredBrandPromotion(doc *goquery.Document) bool {
	selectors := []string{
		"[data-slot=\"desktop-arbies\"]",
		".sbx-desktop",
		"[data-csa-c-owner=\"sponsored-brands-video\"]",
		"[data-card-metrics-id*=\"multi-brand\"]",
		"[class*=\"multi-brand\"]",
	}
	for _, selector := range selectors {
		if selectionHasSponsoredBrandSignal(doc.Find(selector)) {
			return true
		}
	}
	return false
}

func selectionHasSponsoredBrandSignal(selection *goquery.Selection) bool {
	found := false
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.ToLower(selectionTextWithoutScripts(s))
		attrs := strings.ToLower(selectionAttributes(s))
		haystack := text + " " + attrs
		found = strings.Contains(haystack, "brand in this category on amazon") ||
			strings.Contains(haystack, "sponsored ad from") ||
			strings.Contains(haystack, "sponsored-brands") ||
			strings.Contains(haystack, "desktop-arbies")
		return !found
	})
	return found
}

func selectionAttributes(selection *goquery.Selection) string {
	node := selection.Get(0)
	if node == nil {
		return ""
	}
	parts := make([]string, 0, len(node.Attr))
	for _, attr := range node.Attr {
		parts = append(parts, attr.Key, attr.Val)
	}
	return strings.Join(parts, " ")
}

func extractChoiceValue(doc *goquery.Document) string {
	text := textBySelectors(doc, []string{
		"#acBadge_feature_div > div > span > span > span",
		"#acBadge_feature_div span.a-size-small",
	})
	if text == "" {
		containerText := textBySelectors(doc, []string{"#acBadge_feature_div"})
		if strings.Contains(strings.ToLower(containerText), "amazon") && strings.Contains(strings.ToLower(containerText), "choice") {
			text = containerText
		}
	}

	normalized := strings.ToLower(strings.ReplaceAll(text, " ", ""))
	if strings.Contains(normalized, "amazon'schoice") || strings.Contains(normalized, "amazonschoice") {
		return "Amazon's  Choice"
	}
	return ""
}

func extractFrequentlyReturnedValue(doc *goquery.Document) string {
	selectors := []string{
		"#product-alert-grid_feature_div .a-alert-content",
		"#product-alert-grid_feature_div",
		"[id*=\"product-alert\"] .a-alert-content",
		"[id*=\"product-alert\"]",
		"[data-feature-name=\"product-alert-grid\"]",
		"[cel_widget_id*=\"product-alert-grid\"]",
	}
	if text := textByContentSelectors(doc, selectors, []string{"frequently returned item", "customer reviews"}, 800); text != "" {
		return text
	}
	return textByContentSelectors(doc, selectors, []string{"frequently returned item"}, 800)
}

func extractNewerModelValue(doc *goquery.Document) string {
	selectors := []string{
		"#newerVersion_feature_div",
		"#newer-version",
		"#newerVersion",
		"[id*=\"newerVersion\"]",
		"[id*=\"newer-version\"]",
		"[cel_widget_id*=\"newerVersion\"]",
	}
	if text := textByContentSelectors(doc, selectors, []string{"newer model of this item"}, 1600); text != "" {
		return text
	}
	return textByContentSelectors(doc, selectors, []string{"newer version of this item"}, 1600)
}

func extractReviewCountValue(text string) int {
	text = strings.ReplaceAll(text, ",", "")
	match := firstNumberRe.FindString(text)
	if match == "" {
		return 0
	}
	count, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return count
}

func extractRatingValue(text string) string {
	match := decimalNumberRe.FindString(text)
	if match == "" {
		return ""
	}
	rating, err := strconv.ParseFloat(match, 64)
	if err != nil || rating <= 0 || rating > 5 {
		return ""
	}
	return strconv.FormatFloat(rating, 'f', 1, 64)
}

func isVerificationDocument(doc *goquery.Document) bool {
	title := doc.Find("title").First().Text()
	h4 := doc.Find("h4").First().Text()
	return strings.Contains(title, "Enter the characters") ||
		strings.Contains(title, "Type the characters") ||
		strings.Contains(title, "Robot check") ||
		strings.Contains(h4, "Enter the characters")
}

func inspectionRows(results []LinkInspectionResult) [][]string {
	rows := make([][]string, 0, len(results)+1)
	rows = append(rows, inspectionHeaders)
	for _, r := range results {
		rows = append(rows, []string{
			r.Product,
			r.Item.ASIN,
			r.ASIN,
			r.Price,
			r.Coupon,
			r.IsDeal,
			r.PrimeExclusive,
			r.DisplayDiscount,
			r.Rating,
			strconv.Itoa(r.ReviewCount),
			r.PromoCheck,
			r.Promotion,
			r.PromoCode,
			r.Keep,
			r.Choice,
			r.FrequentReturn,
			r.NewerModel,
			r.ParentASIN,
		})
	}
	return rows
}

func writeInspectionXLSX(filename string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil && filepath.Dir(filename) != "." {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建 xlsx 文件失败: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	files := map[string]string{
		"[Content_Types].xml":        contentTypesXML,
		"_rels/.rels":                rootRelsXML,
		"xl/workbook.xml":            workbookXML,
		"xl/_rels/workbook.xml.rels": workbookRelsXML,
		"xl/styles.xml":              stylesXML,
		"docProps/core.xml":          corePropertiesXML(),
		"docProps/app.xml":           appPropertiesXML,
		"xl/worksheets/sheet1.xml":   worksheetXML(rows),
	}

	order := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"docProps/app.xml",
		"docProps/core.xml",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/styles.xml",
		"xl/worksheets/sheet1.xml",
	}
	for _, name := range order {
		if err := addZipFile(zipWriter, name, files[name]); err != nil {
			return err
		}
	}
	return nil
}

func addZipFile(zipWriter *zip.Writer, name, content string) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer, content)
	return err
}

func worksheetXML(rows [][]string) string {
	var builder strings.Builder
	lastCol := columnName(len(inspectionHeaders))
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	builder.WriteString(fmt.Sprintf(`<dimension ref="A1:%s%d"/>`, lastCol, len(rows)))
	builder.WriteString(`<sheetViews><sheetView workbookViewId="0"/></sheetViews>`)
	builder.WriteString(`<sheetFormatPr defaultRowHeight="15"/>`)
	builder.WriteString(`<cols>`)
	widths := []float64{55, 36, 14, 12, 12, 12, 12, 12, 10, 12, 18, 18, 18, 55, 18, 32, 70, 14}
	for i, width := range widths {
		builder.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%.2f" customWidth="1"/>`, i+1, i+1, width))
	}
	builder.WriteString(`</cols>`)
	builder.WriteString(`<sheetData>`)
	for rowIndex, row := range rows {
		excelRow := rowIndex + 1
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, excelRow))
		for colIndex := 0; colIndex < len(inspectionHeaders); colIndex++ {
			value := ""
			if colIndex < len(row) {
				value = row[colIndex]
			}
			ref := fmt.Sprintf("%s%d", columnName(colIndex+1), excelRow)
			if excelRow > 1 && colIndex == 9 {
				if value == "" {
					value = "0"
				}
				builder.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, escapeXML(value)))
				continue
			}
			builder.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, escapeXML(value)))
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData>`)
	builder.WriteString(`</worksheet>`)
	return builder.String()
}

func columnName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}

func escapeXML(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 {
			return r
		}
		return -1
	}, value)
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const workbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="链接巡检" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
  <fills count="1"><fill><patternFill patternType="none"/></fill></fills>
  <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`

const appPropertiesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>amazon-crawler</Application>
</Properties>`

func corePropertiesXML() string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:creator>amazon-crawler</dc:creator>
  <cp:lastModifiedBy>amazon-crawler</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, now, now)
}
