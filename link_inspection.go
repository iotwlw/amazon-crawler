package main

import (
	"archive/zip"
	"bytes"
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

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

var (
	linkASINRe        = regexp.MustCompile(`(?i)/(?:dp|gp/product)/([A-Z0-9]{10})`)
	bareASINRe        = regexp.MustCompile(`(?i)(^|[^A-Z0-9])([A-Z0-9]{10})([^A-Z0-9]|$)`)
	promoAmountRe     = regexp.MustCompile(`(?i)(\d{1,3}%|\$\d+(?:\.\d+)?)`)
	moneyAmountRe     = regexp.MustCompile(`\$?\s*([0-9][0-9,]*(?:\.\d{1,2})?)`)
	firstNumberRe     = regexp.MustCompile(`\d+`)
	decimalNumberRe   = regexp.MustCompile(`\d+(?:\.\d+)?`)
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
	}
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
	Item            LinkInspectionItem
	Product         string
	ASIN            string
	Price           string
	Coupon          string
	IsDeal          string
	PrimeExclusive  string
	DisplayDiscount string
	Rating          string
	ReviewCount     int
	PromoCheck      string
	Promotion       string
	PromoCode       string
	Keep            string
	Choice          string
	ErrorMessage    string
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
		result := s.inspectItem(item)
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

func (s *LinkInspector) inspectItem(item LinkInspectionItem) LinkInspectionResult {
	result := LinkInspectionResult{
		Item:            item,
		ASIN:            item.ASIN,
		Coupon:          " ",
		IsDeal:          " ",
		PrimeExclusive:  " ",
		DisplayDiscount: " ",
	}

	doc, err := s.fetchDocument(item)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	extracted := extractLinkInspectionFields(doc, item)
	extracted.ErrorMessage = result.ErrorMessage
	return extracted
}

func (s *LinkInspector) fetchDocument(item LinkInspectionItem) (*goquery.Document, error) {
	fp := GetCurrentFingerprint()
	robots, err := s.robotsForDomain(item.Domain)
	if err != nil {
		return nil, err
	}
	if err := robots.IsAllow(fp.UserAgent, item.URL); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		client := get_client()
		req, err := http.NewRequest("GET", item.URL, nil)
		if err != nil {
			return nil, err
		}
		ApplyFingerprint(req, GetRandomReferer(item.Domain))
		if app.cookie != "" {
			req.Header.Set("Cookie", app.cookie)
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
				if err := app.handleCookieInvalid(); err != nil {
					log.Errorf("切换 Cookie 失败: %v", err)
					return nil, lastErr
				}
				continue
			}
			return nil, lastErr
		}
		return doc, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("请求失败")
	}
	return nil, lastErr
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
	reviewCount := extractReviewCountValue(textBySelectors(doc, []string{
		"#acrCustomerReviewText",
		"[data-hook=\"total-review-count\"]",
		"#averageCustomerReviews .a-size-base.a-color-secondary",
	}))

	asin := cleanText(attrBySelectors(doc, []string{"input#ASIN", "input[name=\"ASIN\"]"}, "value"))
	if asin == "" {
		asin = item.ASIN
	}

	rating := extractRatingValue(textBySelectors(doc, []string{
		"#averageCustomerReviews span[aria-hidden=\"true\"]",
		"#averageCustomerReviews .a-icon-alt",
		"#acrPopover .a-icon-alt",
		"[data-hook=\"average-star-rating\"] .a-icon-alt",
	}))
	if reviewCount == 0 {
		rating = ""
	}
	price := extractCurrentPriceValue(doc)
	if strings.TrimSpace(price) == "" {
		price = extractPriceStatusValue(doc)
	}

	result := LinkInspectionResult{
		Item:            item,
		Product:         textBySelectors(doc, []string{"#productTitle"}),
		ASIN:            asin,
		Price:           price,
		Coupon:          defaultSpace(extractCouponValue(doc)),
		IsDeal:          defaultSpace(textBySelectors(doc, []string{"#dealBadgeSupportingText", "#dealBadge_feature_div"})),
		PrimeExclusive:  defaultSpace(textBySelectors(doc, []string{"#primeExclusivePricingMessage .a-size-base", "#primeExclusivePricingMessage"})),
		DisplayDiscount: defaultSpace(calculateDisplayDiscount(extractListPriceValue(doc), price)),
		Rating:          rating,
		ReviewCount:     reviewCount,
		PromoCheck:      extractPromoCheckValue(doc),
		Promotion: textBySelectors(doc, []string{
			"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:nth-child(2) label",
			"label[id^=\"greenBadge\"]",
			"span[id^=\"promotion_title\"]",
		}),
		PromoCode: textBySelectors(doc, []string{
			"#promoPriceBlockMessage_feature_div .promoPriceBlockMessage > div:nth-child(2) span span:nth-child(2)",
			"#promoPriceBlockMessage_feature_div span[id^=\"promoCode\"]",
		}),
		Keep: textBySelectors(doc, []string{
			"#NEW_1_nostos_badge",
		}),
		Choice: extractChoiceValue(doc),
	}
	return result
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
		if value := firstPriceStatusFromSelection(doc.Find(selector)); value != "" {
			return value
		}
	}
	return ""
}

func firstPriceStatusFromSelection(selection *goquery.Selection) string {
	var value string
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		value = priceStatusFromText(selectionTextWithoutScripts(s))
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
			r.Item.Original,
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
	widths := []float64{55, 36, 14, 12, 12, 12, 12, 12, 10, 12, 18, 18, 18, 55, 18}
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
