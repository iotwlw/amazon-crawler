package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

var reviewFilePartRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

type ReviewScraper struct {
	rawInput   string
	outputFile string
	imageDir   string
	productURL string
	domain     string
	asin       string
	robots     map[string]Robots
	photos     []CustomerPhoto
}

type ReviewItem struct {
	ASIN             string
	ProductURL       string
	SourcePage       string
	SourceURL        string
	ReviewID         string
	ReviewerName     string
	Rating           string
	Title            string
	Date             string
	VerifiedPurchase string
	HelpfulText      string
	Body             string
	DetailURL        string
	ImageURLs        []string
	ImageFiles       []string
}

type CustomerPhoto struct {
	PhysicalID string
	ReviewID   string
	MediaType  string
	ImageURL   string
	ImageFile  string
}

func NewReviewScraper(productURL, outputFile, imageDir string) *ReviewScraper {
	return &ReviewScraper{
		rawInput:   productURL,
		outputFile: outputFile,
		imageDir:   imageDir,
		robots: map[string]Robots{
			normalizeDomain(app.Domain): robot,
		},
	}
}

func (s *ReviewScraper) Run() error {
	item, ok := parseLinkInspectionItem(s.rawInput, app.Domain)
	if !ok {
		return fmt.Errorf("无法识别评价抓取链接或 ASIN: %s", s.rawInput)
	}

	s.productURL = item.URL
	s.domain = item.Domain
	s.asin = item.ASIN
	if _, err := app.get_cookie(); err != nil {
		log.Warnf("获取 Cookie 失败: %v，将不使用 Cookie", err)
	}

	reviews, err := s.scrapeFirstPage()
	if err != nil {
		return err
	}

	outputFile := s.outputFile
	if outputFile == "" {
		outputFile = filepath.Join("output", fmt.Sprintf("reviews_%s_%s.csv", s.asin, time.Now().Format("20060102_150405")))
	}
	s.outputFile = outputFile
	if s.imageDir == "" {
		ext := filepath.Ext(outputFile)
		s.imageDir = strings.TrimSuffix(outputFile, ext) + "_images"
	}

	s.downloadReviewImages(reviews)
	if err := s.exportCSV(reviews); err != nil {
		return err
	}
	if err := s.downloadCustomerPhotos(); err != nil {
		return err
	}

	log.Infof("评价抓取完成: ASIN=%s 评价=%d 顾客图片区图片=%d 输出=%s 图片目录=%s", s.asin, len(reviews), len(s.photos), s.outputFile, s.imageDir)
	return nil
}

func (s *ReviewScraper) scrapeFirstPage() ([]ReviewItem, error) {
	productDoc, err := s.fetchDocument(s.productURL, "")
	if err != nil {
		return nil, fmt.Errorf("抓取商品页失败: %w", err)
	}

	item := LinkInspectionItem{URL: s.productURL, ASIN: s.asin, Domain: s.domain}
	if actualASIN := extractActualASINValue(productDoc, item); actualASIN != "" {
		s.asin = actualASIN
	}

	reviews := parseReviewItems(productDoc, "product_page", s.productURL, s.asin, s.productURL)
	s.photos = extractCustomerPhotos(productDoc, s.productURL)
	reviewPageURL := s.findFirstReviewPageURL(productDoc)
	if reviewPageURL != "" {
		reviewDoc, err := s.fetchDocument(reviewPageURL, s.productURL)
		if err != nil {
			log.Warnf("抓取评价第一页失败，将仅导出商品页可见评价: %v", err)
		} else {
			reviews = append(reviews, parseReviewItems(reviewDoc, "reviews_page", reviewPageURL, s.asin, s.productURL)...)
		}
	}

	return dedupeReviewItems(reviews), nil
}

func (s *ReviewScraper) fetchDocument(pageURL, referer string) (*goquery.Document, error) {
	fp := GetCurrentFingerprint()
	robots, err := s.robotsForURL(pageURL)
	if err != nil {
		return nil, err
	}
	if err := robots.IsAllow(fp.UserAgent, pageURL); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		client := get_client()
		req, err := http.NewRequest("GET", pageURL, nil)
		if err != nil {
			return nil, err
		}
		if referer == "" {
			referer = GetRandomReferer(s.domain)
		}
		ApplyFingerprint(req, referer)
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

func (s *ReviewScraper) robotsForURL(rawURL string) (Robots, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return Robots{}, err
	}
	domain := normalizeDomain(parsed.Host)
	if domain == "" {
		domain = s.domain
	}
	if r, ok := s.robots[domain]; ok {
		return r, nil
	}

	fp := GetCurrentFingerprint()
	robotURL := fmt.Sprintf("https://%s/robots.txt", domain)
	log.Infof("加载文件: %s", robotURL)
	txt, err := request_get(robotURL, fp.UserAgent)
	if err != nil {
		return Robots{}, fmt.Errorf("加载 robots.txt 失败: %w", err)
	}

	r := GetRobotFromTxt(txt)
	s.robots[domain] = r
	return r, nil
}

func (s *ReviewScraper) findFirstReviewPageURL(doc *goquery.Document) string {
	selectors := []string{
		`a[data-hook="see-all-reviews-link-foot"]`,
		`a[href*="/product-reviews/"]`,
		`a[href*="cm_cr_arp"]`,
	}
	for _, selector := range selectors {
		var found string
		doc.Find(selector).EachWithBreak(func(_ int, a *goquery.Selection) bool {
			href, ok := a.Attr("href")
			if !ok || strings.TrimSpace(href) == "" {
				return true
			}
			if strings.Contains(href, "/product-reviews/") || strings.Contains(href, "cm_cr_arp") {
				found = absoluteReviewURL(href, s.productURL)
				return false
			}
			return true
		})
		if found != "" {
			return ensureFirstReviewPage(found)
		}
	}
	if s.asin == "" || s.domain == "" {
		return ""
	}
	return fmt.Sprintf("https://%s/product-reviews/%s?reviewerType=all_reviews&pageNumber=1", s.domain, s.asin)
}

func parseReviewItems(doc *goquery.Document, sourcePage, sourceURL, asin, productURL string) []ReviewItem {
	reviews := make([]ReviewItem, 0)
	doc.Find(`div[data-hook="review"], div.review[id^="customer_review-"], #cm-cr-dp-review-list div.review`).Each(func(_ int, card *goquery.Selection) {
		review := ReviewItem{
			ASIN:       asin,
			ProductURL: productURL,
			SourcePage: sourcePage,
			SourceURL:  sourceURL,
		}
		review.ReviewID = extractReviewID(card)
		review.ReviewerName = cleanText(card.Find(`.a-profile-name`).First().Text())
		review.Rating = extractRatingValue(cleanText(card.Find(`[data-hook="review-star-rating"] .a-icon-alt, [data-hook="cmps-review-star-rating"] .a-icon-alt, i.review-rating .a-icon-alt, .review-rating .a-icon-alt`).First().Text()))
		review.Title = extractReviewTitle(card)
		review.Date = cleanText(card.Find(`[data-hook="review-date"], .review-date`).First().Text())
		review.VerifiedPurchase = cleanText(card.Find(`[data-hook="avp-badge"], .a-size-mini.a-color-state`).First().Text())
		review.HelpfulText = cleanText(card.Find(`[data-hook="helpful-vote-statement"], .cr-vote-text`).First().Text())
		review.Body = extractReviewBody(card)
		review.DetailURL = extractReviewDetailURL(card, sourceURL)
		review.ImageURLs = extractReviewImageURLs(card, sourceURL)
		if review.ReviewID == "" && review.Title == "" && review.Body == "" && len(review.ImageURLs) == 0 {
			return
		}
		reviews = append(reviews, review)
	})
	return reviews
}

func extractReviewID(card *goquery.Selection) string {
	for _, attr := range []string{"id", "data-review-id"} {
		if val, ok := card.Attr(attr); ok {
			val = strings.TrimSpace(strings.TrimPrefix(val, "customer_review-"))
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func extractReviewTitle(card *goquery.Selection) string {
	titleNode := card.Find(`[data-hook="review-title"], [data-hook="reviewTitle"], a.review-title, .review-title`).First()
	if titleNode.Length() == 0 {
		return ""
	}

	parts := make([]string, 0)
	titleNode.Find("span").Each(func(_ int, span *goquery.Selection) {
		text := cleanText(span.Text())
		lower := strings.ToLower(text)
		if text == "" || (strings.Contains(lower, "out of") && strings.Contains(lower, "star")) {
			return
		}
		parts = append(parts, text)
	})
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	text := cleanText(titleNode.Text())
	for _, marker := range []string{"out of 5 stars", "out of 5"} {
		if idx := strings.LastIndex(strings.ToLower(text), marker); idx >= 0 {
			text = strings.TrimSpace(text[idx+len(marker):])
			break
		}
	}
	return cleanText(text)
}

func extractReviewBody(card *goquery.Selection) string {
	selectors := []string{
		`[data-hook="review-body"]`,
		`[data-hook="reviewRichContentContainer"]`,
		`[data-hook="reviewText"] [data-hook="reviewRichContentContainer"]`,
		`[data-hook="reviewTextContainer"] [data-hook="reviewRichContentContainer"]`,
		`.review-text-content`,
		`.review-text`,
	}
	for _, selector := range selectors {
		text := cleanText(card.Find(selector).First().Text())
		text = cleanReviewBodyText(text)
		if text != "" {
			return text
		}
	}
	return ""
}

func cleanReviewBodyText(text string) string {
	for _, noise := range []string{
		"Brief content visible, double tap to read full content.",
		"Full content visible, double tap to read brief content.",
		"Read more",
		"Read less",
	} {
		text = strings.ReplaceAll(text, noise, "")
	}
	return cleanText(text)
}

func extractReviewDetailURL(card *goquery.Selection, baseURL string) string {
	if href, ok := card.Find(`[data-hook="review-title"], a.review-title`).First().Attr("href"); ok {
		return absoluteReviewURL(href, baseURL)
	}
	titleNode := card.Find(`[data-hook="reviewTitle"]`).First()
	if titleNode.Length() > 0 {
		if href, ok := titleNode.ParentsFiltered("a").First().Attr("href"); ok {
			return absoluteReviewURL(href, baseURL)
		}
	}
	return ""
}

func extractReviewImageURLs(card *goquery.Selection, baseURL string) []string {
	urls := make([]string, 0)
	seen := make(map[string]bool)
	addURL := func(raw string) {
		normalized := normalizeImageURL(raw, baseURL)
		if normalized == "" || seen[normalized] {
			return
		}
		seen[normalized] = true
		urls = append(urls, normalized)
	}

	card.Find(`img[data-hook="review-image-tile"], img.review-image-tile, .review-image-tile-section img, [data-hook="review-image"] img`).Each(func(_ int, img *goquery.Selection) {
		for _, attr := range []string{"data-a-hires", "data-old-hires", "data-src", "src"} {
			if val, ok := img.Attr(attr); ok {
				addURL(val)
			}
		}
		for _, attr := range []string{"srcset", "data-srcset"} {
			if val, ok := img.Attr(attr); ok {
				addURL(bestSrcsetURL(val))
			}
		}
		if val, ok := img.Attr("data-a-dynamic-image"); ok {
			for _, dynamicURL := range dynamicImageURLs(val) {
				addURL(dynamicURL)
			}
		}
		if href, ok := img.ParentFiltered("a").Attr("href"); ok && strings.Contains(href, "media-amazon.com") {
			addURL(href)
		}
	})
	return urls
}

func extractCustomerPhotos(doc *goquery.Document, baseURL string) []CustomerPhoto {
	photos := make([]CustomerPhoto, 0)
	seen := make(map[string]bool)
	selectors := []string{
		`button[data-mix-operations*="CRImageThumbnailOpsClickHandler"][data-physicalid]`,
		`button[data-csa-c-slot-id^="cm_cr_image_carousel_"][data-physicalid]`,
	}
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, button *goquery.Selection) {
			mediaType := strings.ToUpper(cleanText(attrFromSelection(button, "data-mediatype")))
			if mediaType != "" && mediaType != "IMAGE" {
				return
			}

			physicalID := cleanText(attrFromSelection(button, "data-physicalid"))
			extension := strings.TrimPrefix(strings.ToLower(cleanText(attrFromSelection(button, "data-extension"))), ".")
			imageURL := originalCustomerPhotoURL(physicalID, extension)
			if imageURL == "" {
				imageURL = attrFromSelection(button, "data-url")
			}
			if imageURL == "" {
				imageURL = attrFromSelection(button, "data-thumbnailurl")
			}
			imageURL = normalizeImageURL(imageURL, baseURL)
			if imageURL == "" || seen[imageURL] {
				return
			}
			seen[imageURL] = true

			photos = append(photos, CustomerPhoto{
				PhysicalID: physicalID,
				ReviewID:   cleanText(attrFromSelection(button, "data-reviewid")),
				MediaType:  defaultSpace(mediaType),
				ImageURL:   imageURL,
			})
		})
	}
	return photos
}

func attrFromSelection(selection *goquery.Selection, attr string) string {
	if val, ok := selection.Attr(attr); ok {
		return strings.TrimSpace(val)
	}
	return ""
}

func originalCustomerPhotoURL(physicalID, extension string) string {
	if physicalID == "" {
		return ""
	}
	if extension == "" {
		extension = "jpg"
	}
	return fmt.Sprintf("https://m.media-amazon.com/images/I/%s.%s", physicalID, extension)
}

func dynamicImageURLs(raw string) []string {
	var payload map[string][]int
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	urls := make([]string, 0, len(payload))
	for imageURL := range payload {
		urls = append(urls, imageURL)
	}
	sort.Strings(urls)
	return urls
}

func bestSrcsetURL(raw string) string {
	parts := strings.Split(raw, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		fields := strings.Fields(strings.TrimSpace(parts[i]))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func normalizeImageURL(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "data:") || strings.Contains(raw, "transparent-pixel") {
		return ""
	}
	return originalAmazonImageURL(absoluteReviewURL(raw, baseURL))
}

func originalAmazonImageURL(raw string) string {
	parsed, err := neturl.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	if !strings.Contains(parsed.Host, "media-amazon.com") || !strings.Contains(parsed.Path, "/images/I/") {
		return raw
	}

	ext := path.Ext(parsed.Path)
	if ext == "" {
		return raw
	}
	filename := path.Base(parsed.Path)
	stem := strings.TrimSuffix(filename, ext)
	if idx := strings.Index(stem, "._"); idx >= 0 {
		stem = stem[:idx]
	}
	if stem == "" {
		return raw
	}

	parsed.Path = path.Join(path.Dir(parsed.Path), stem+ext)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func absoluteReviewURL(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := neturl.Parse(baseURL)
	if err != nil {
		return raw
	}
	return base.ResolveReference(parsed).String()
}

func ensureFirstReviewPage(raw string) string {
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	if query.Get("pageNumber") == "" {
		query.Set("pageNumber", "1")
	}
	if query.Get("reviewerType") == "" {
		query.Set("reviewerType", "all_reviews")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func dedupeReviewItems(items []ReviewItem) []ReviewItem {
	result := make([]ReviewItem, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		key := item.ReviewID
		if key == "" {
			key = item.Title + "\n" + item.Body
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func (s *ReviewScraper) downloadReviewImages(reviews []ReviewItem) {
	hasImage := false
	for _, review := range reviews {
		if len(review.ImageURLs) > 0 {
			hasImage = true
			break
		}
	}
	if !hasImage {
		return
	}
	if err := os.MkdirAll(s.imageDir, 0755); err != nil {
		log.Errorf("创建评价图片目录失败: %v", err)
		return
	}

	for reviewIndex := range reviews {
		for imageIndex, imageURL := range reviews[reviewIndex].ImageURLs {
			filePath, err := s.downloadImage(imageURL, reviews[reviewIndex], reviewIndex, imageIndex)
			if err != nil {
				log.Warnf("评价图片下载失败: %s 错误:%v", imageURL, err)
				continue
			}
			reviews[reviewIndex].ImageFiles = append(reviews[reviewIndex].ImageFiles, filePath)
		}
	}
}

func (s *ReviewScraper) downloadImage(imageURL string, review ReviewItem, reviewIndex, imageIndex int) (string, error) {
	filePath := filepath.Join(s.imageDir, reviewImageFilename(imageURL, review, reviewIndex, imageIndex))
	return filePath, s.downloadImageFile(imageURL, review.SourceURL, filePath)
}

func (s *ReviewScraper) downloadImageFile(imageURL, referer, filePath string) error {
	fp := GetCurrentFingerprint()
	robots, err := s.robotsForURL(imageURL)
	if err != nil {
		return err
	}
	if err := robots.IsAllow(fp.UserAgent, imageURL); err != nil {
		return err
	}

	client := get_client()
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return err
	}
	ApplyFingerprint(req, referer)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	if app.cookie != "" {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}

func reviewImageFilename(imageURL string, review ReviewItem, reviewIndex, imageIndex int) string {
	parsed, _ := neturl.Parse(imageURL)
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}
	id := review.ReviewID
	if id == "" {
		id = fmt.Sprintf("review_%03d", reviewIndex+1)
	}
	id = safeReviewFilePart(id)
	return fmt.Sprintf("%03d_%s_%02d%s", reviewIndex+1, id, imageIndex+1, ext)
}

func customerPhotoFilename(photo CustomerPhoto, index int) string {
	ext := ".jpg"
	if parsed, err := neturl.Parse(photo.ImageURL); err == nil {
		if parsedExt := strings.ToLower(path.Ext(parsed.Path)); parsedExt != "" && len(parsedExt) <= 5 {
			ext = parsedExt
		}
	}
	id := photo.PhysicalID
	if id == "" {
		id = fmt.Sprintf("photo_%03d", index+1)
	}
	return fmt.Sprintf("%03d_%s%s", index+1, safeReviewFilePart(id), ext)
}

func safeReviewFilePart(text string) string {
	text = reviewFilePartRe.ReplaceAllString(text, "_")
	text = strings.Trim(text, "_")
	if text == "" {
		return "review"
	}
	if len(text) > 80 {
		return text[:80]
	}
	return text
}

func (s *ReviewScraper) downloadCustomerPhotos() error {
	if len(s.photos) == 0 {
		return nil
	}
	photoDir := filepath.Join(s.imageDir, "customer_photos")
	if err := os.MkdirAll(photoDir, 0755); err != nil {
		return fmt.Errorf("创建顾客图片区目录失败: %w", err)
	}
	for i := range s.photos {
		filePath := filepath.Join(photoDir, customerPhotoFilename(s.photos[i], i))
		if err := s.downloadImageFile(s.photos[i].ImageURL, s.productURL, filePath); err != nil {
			log.Warnf("顾客图片区原图下载失败: %s 错误:%v", s.photos[i].ImageURL, err)
			continue
		}
		s.photos[i].ImageFile = filePath
	}
	return s.exportCustomerPhotosCSV()
}

func (s *ReviewScraper) exportCustomerPhotosCSV() error {
	if len(s.photos) == 0 {
		return nil
	}
	ext := filepath.Ext(s.outputFile)
	manifestFile := strings.TrimSuffix(s.outputFile, ext) + "_customer_photos.csv"
	dir := filepath.Dir(manifestFile)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建顾客图片区清单目录失败: %w", err)
		}
	}

	file, err := os.Create(manifestFile)
	if err != nil {
		return fmt.Errorf("创建顾客图片区清单失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"PhysicalID", "ReviewID", "MediaType", "ImageURL", "ImageFile"}); err != nil {
		return err
	}
	for _, photo := range s.photos {
		if err := writer.Write([]string{photo.PhysicalID, photo.ReviewID, photo.MediaType, photo.ImageURL, photo.ImageFile}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func (s *ReviewScraper) exportCSV(reviews []ReviewItem) error {
	dir := filepath.Dir(s.outputFile)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建输出目录失败: %w", err)
		}
	}

	file, err := os.Create(s.outputFile)
	if err != nil {
		return fmt.Errorf("创建 CSV 文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"ASIN",
		"SourcePage",
		"ReviewID",
		"ReviewerName",
		"Rating",
		"Title",
		"Date",
		"VerifiedPurchase",
		"HelpfulText",
		"Body",
		"ImageURLs",
		"ImageFiles",
		"DetailURL",
		"ProductURL",
		"SourceURL",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, review := range reviews {
		record := []string{
			review.ASIN,
			review.SourcePage,
			review.ReviewID,
			review.ReviewerName,
			review.Rating,
			review.Title,
			review.Date,
			review.VerifiedPurchase,
			review.HelpfulText,
			review.Body,
			strings.Join(review.ImageURLs, "\n"),
			strings.Join(review.ImageFiles, "\n"),
			review.DetailURL,
			review.ProductURL,
			review.SourceURL,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return writer.Error()
}
