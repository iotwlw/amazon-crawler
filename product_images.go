package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

const (
	productImageRoleMain  = "main"
	productImageRoleAPlus = "aplus"
)

var (
	imageScriptFieldRe = regexp.MustCompile(`(?is)["'](hiRes|large|lowRes|thumb|imageUrl|mainUrl|largeImageUrl)["']\s*:\s*["']((?:\\.|[^"'])+?)["']`)
	rawAmazonImageRe   = regexp.MustCompile(`(?i)https?:\\?/\\?/[^'"<>\s]+?\.(?:jpe?g|jpg|png|webp)(?:\?[^'"<>\s]*)?`)
	imageModifierRe    = regexp.MustCompile(`(?i)\._[^.]+_\.(jpe?g|jpg|png|webp)$`)
	imageExtensionRe   = regexp.MustCompile(`(?i)\.(jpe?g|jpg|png|webp)$`)
)

type ProductImageExtractionOptions struct {
	Include  string
	MaxMain  int
	MaxAPlus int
}

type ProductImageExtractionResult struct {
	ASIN            string
	ProductTitle    string
	MainCandidates  int
	APlusCandidates int
	Images          []ProductImageCandidate
}

type ProductImageCandidate struct {
	Role   string
	URL    string
	Width  int
	Height int
	Source string
	Order  int
	Key    string
}

type ProductImageExtractor struct {
	defaultDomain string
	robots        map[string]Robots
}

func NewProductImageExtractor(domain string) *ProductImageExtractor {
	return &ProductImageExtractor{
		defaultDomain: normalizeDomain(domain),
		robots: map[string]Robots{
			normalizeDomain(app.Domain): robot,
		},
	}
}

func (s *ProductImageExtractor) Extract(item LinkInspectionItem, options ProductImageExtractionOptions) (ProductImageExtractionResult, error) {
	htmlText, doc, err := s.fetchHTML(item)
	if err != nil {
		return ProductImageExtractionResult{}, err
	}
	result := extractProductImagesFromHTML(htmlText, doc, item.URL, options)
	if result.ASIN == "" {
		result.ASIN = item.ASIN
	}
	return result, nil
}

func (s *ProductImageExtractor) fetchHTML(item LinkInspectionItem) (string, *goquery.Document, error) {
	fp := GetCurrentFingerprint()
	robots, err := s.robotsForDomain(item.Domain)
	if err != nil {
		return "", nil, err
	}
	if err := robots.IsAllow(fp.UserAgent, item.URL); err != nil {
		return "", nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		client := get_client()
		req, err := http.NewRequest("GET", item.URL, nil)
		if err != nil {
			return "", nil, err
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
		body, readErr := readAmazonHTMLResponse(resp)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if resp.StatusCode == http.StatusServiceUnavailable && attempt == 0 {
				RotateFingerprint()
				continue
			}
			continue
		}
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err != nil {
			return "", nil, err
		}
		if isVerificationDocument(doc) {
			lastErr = ERROR_VERIFICATION
			if attempt == 0 {
				if err := app.handleCookieInvalid(); err != nil {
					log.Errorf("切换 Cookie 失败: %v", err)
					return "", nil, lastErr
				}
				continue
			}
			return "", nil, lastErr
		}
		return string(body), doc, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("请求失败")
	}
	return "", nil, lastErr
}

func (s *ProductImageExtractor) robotsForDomain(domain string) (Robots, error) {
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

func readAmazonHTMLResponse(resp *http.Response) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func extractProductImagesFromHTML(htmlText string, doc *goquery.Document, pageURL string, options ProductImageExtractionOptions) ProductImageExtractionResult {
	options = normalizeProductImageOptions(options)
	baseURL, _ := url.Parse(pageURL)
	candidates := make([]ProductImageCandidate, 0)

	addTagProductImageCandidates(&candidates, doc, baseURL)
	addScriptProductImageCandidates(&candidates, htmlText, baseURL)
	addRawAPlusProductImageCandidates(&candidates, htmlText, baseURL)

	mainCandidates := countProductImageRole(candidates, productImageRoleMain)
	aPlusCandidates := countProductImageRole(candidates, productImageRoleAPlus)
	images := selectProductImages(candidates, options)

	return ProductImageExtractionResult{
		ASIN:            extractProductImageASIN(doc, pageURL),
		ProductTitle:    extractProductImageTitle(doc),
		MainCandidates:  mainCandidates,
		APlusCandidates: aPlusCandidates,
		Images:          images,
	}
}

func normalizeProductImageOptions(options ProductImageExtractionOptions) ProductImageExtractionOptions {
	options.Include = strings.ToLower(strings.TrimSpace(options.Include))
	if options.Include == "" {
		options.Include = "all"
	}
	if options.Include != "all" && options.Include != productImageRoleMain && options.Include != productImageRoleAPlus {
		options.Include = "all"
	}
	if options.MaxMain <= 0 {
		options.MaxMain = 8
	}
	if options.MaxAPlus <= 0 {
		options.MaxAPlus = 40
	}
	return options
}

func addTagProductImageCandidates(candidates *[]ProductImageCandidate, doc *goquery.Document, baseURL *url.URL) {
	doc.Find("img").Each(func(index int, img *goquery.Selection) {
		role := classifyProductImageSelection(img)
		if role == "" {
			return
		}

		if payload, ok := img.Attr("data-a-dynamic-image"); ok {
			addDynamicProductImageCandidates(candidates, role, payload, baseURL, index)
		}
		for _, attr := range []string{"srcset", "data-srcset"} {
			if srcset, ok := img.Attr(attr); ok {
				addSrcsetProductImageCandidates(candidates, role, srcset, baseURL, index)
			}
		}
		for _, attr := range []string{"data-old-hires", "data-a-hires", "data-src", "src"} {
			if value, ok := img.Attr(attr); ok {
				addProductImageCandidate(candidates, role, value, 0, 0, attr, index, baseURL)
			}
		}
	})
}

func addDynamicProductImageCandidates(candidates *[]ProductImageCandidate, role, payload string, baseURL *url.URL, order int) {
	payload = html.UnescapeString(payload)
	var parsed map[string][]int
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return
	}
	for imageURL, size := range parsed {
		width, height := 0, 0
		if len(size) >= 2 {
			width = size[0]
			height = size[1]
		}
		addProductImageCandidate(candidates, role, imageURL, width, height, "dynamic", order, baseURL)
	}
}

func addSrcsetProductImageCandidates(candidates *[]ProductImageCandidate, role, srcset string, baseURL *url.URL, order int) {
	for _, entry := range strings.Split(srcset, ",") {
		fields := strings.Fields(strings.TrimSpace(entry))
		if len(fields) == 0 {
			continue
		}
		width := 0
		if len(fields) > 1 && strings.HasSuffix(fields[1], "w") {
			fmt.Sscanf(strings.TrimSuffix(fields[1], "w"), "%d", &width)
		}
		addProductImageCandidate(candidates, role, fields[0], width, 0, "srcset", order, baseURL)
	}
}

func addScriptProductImageCandidates(candidates *[]ProductImageCandidate, htmlText string, baseURL *url.URL) {
	matches := imageScriptFieldRe.FindAllStringSubmatchIndex(htmlText, -1)
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		field := strings.ToLower(htmlText[match[2]:match[3]])
		rawURL := decodeAmazonImageURLText(htmlText[match[4]:match[5]])
		context := productImageContext(htmlText, match[0], 3000, 1000)
		role := productImageRoleFromContext(context, rawURL)
		if role == "" {
			role = productImageRoleMain
		}
		addProductImageCandidate(candidates, role, rawURL, 0, 0, "script-"+field, match[0], baseURL)
	}
}

func addRawAPlusProductImageCandidates(candidates *[]ProductImageCandidate, htmlText string, baseURL *url.URL) {
	matches := rawAmazonImageRe.FindAllStringIndex(htmlText, -1)
	for _, match := range matches {
		rawURL := decodeAmazonImageURLText(htmlText[match[0]:match[1]])
		context := productImageContext(htmlText, match[0], 50000, 1000)
		if productImageRoleFromContext(context, rawURL) != productImageRoleAPlus {
			continue
		}
		addProductImageCandidate(candidates, productImageRoleAPlus, rawURL, 0, 0, "raw-aplus-url", match[0], baseURL)
	}
}

func addProductImageCandidate(candidates *[]ProductImageCandidate, role, rawURL string, width, height int, source string, order int, baseURL *url.URL) {
	imageURL := normalizeProductImageURL(rawURL, baseURL)
	if imageURL == "" || !isAllowedAmazonProductImageURL(imageURL) {
		return
	}
	if shouldSkipProductImageURL(imageURL) {
		return
	}
	*candidates = append(*candidates, ProductImageCandidate{
		Role:   role,
		URL:    imageURL,
		Width:  width,
		Height: height,
		Source: source,
		Order:  order,
		Key:    amazonImageKey(imageURL),
	})
}

func classifyProductImageSelection(img *goquery.Selection) string {
	if selectionHasAPlusMarker(img) {
		return productImageRoleAPlus
	}
	if selectionHasMainImageMarker(img) {
		return productImageRoleMain
	}
	if _, ok := img.Attr("data-a-dynamic-image"); ok {
		return productImageRoleMain
	}
	return ""
}

func selectionHasAPlusMarker(selection *goquery.Selection) bool {
	if selectionHasMarker(selection, "aplus") || selectionHasMarker(selection, "aplus_feature_div") ||
		selectionHasMarker(selection, "aplus-media") || selectionHasMarker(selection, "premium-aplus") {
		return true
	}
	return false
}

func selectionHasMainImageMarker(selection *goquery.Selection) bool {
	for _, marker := range []string{"imageBlock", "main-image-container", "altImages", "imgTagWrapperId", "landingImage", "imageThumbnail"} {
		if selectionHasMarker(selection, marker) {
			return true
		}
	}
	return false
}

func selectionHasMarker(selection *goquery.Selection, marker string) bool {
	found := false
	selection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for current := s; current != nil && current.Length() > 0; current = current.Parent() {
			if id, ok := current.Attr("id"); ok && strings.Contains(strings.ToLower(id), strings.ToLower(marker)) {
				found = true
				return false
			}
			if className, ok := current.Attr("class"); ok && strings.Contains(strings.ToLower(className), strings.ToLower(marker)) {
				found = true
				return false
			}
			if current.Is("html") {
				break
			}
		}
		return true
	})
	return found
}

func productImageRoleFromContext(context, imageURL string) string {
	text := strings.ToLower(context + " " + imageURL)
	if strings.Contains(text, "aplus") || strings.Contains(text, "aplus_feature_div") ||
		strings.Contains(text, "aplus-media") || strings.Contains(text, "premium-aplus") {
		return productImageRoleAPlus
	}
	if strings.Contains(text, "imageblock") || strings.Contains(text, "main-image-container") ||
		strings.Contains(text, "altimages") || strings.Contains(text, "imgtagwrapperid") ||
		strings.Contains(text, "landingimage") || strings.Contains(text, "data-a-dynamic-image") {
		return productImageRoleMain
	}
	return ""
}

func productImageContext(text string, index, before, after int) string {
	start := index - before
	if start < 0 {
		start = 0
	}
	end := index + after
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

func normalizeProductImageURL(raw string, baseURL *url.URL) string {
	raw = decodeAmazonImageURLText(raw)
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") || strings.HasPrefix(strings.ToLower(raw), "blob:") {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if baseURL != nil {
		parsed = baseURL.ResolveReference(parsed)
	}
	return parsed.String()
}

func decodeAmazonImageURLText(raw string) string {
	raw = html.UnescapeString(raw)
	raw = strings.ReplaceAll(raw, `\/`, `/`)
	raw = strings.ReplaceAll(raw, `\u0026`, `&`)
	raw = strings.ReplaceAll(raw, `\u002F`, `/`)
	raw = strings.ReplaceAll(raw, `\u003D`, `=`)
	return strings.TrimSpace(raw)
}

func isAllowedAmazonProductImageURL(imageURL string) bool {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	if !imageExtensionRe.MatchString(path) {
		return false
	}
	return strings.HasSuffix(host, "media-amazon.com") ||
		strings.HasSuffix(host, "ssl-images-amazon.com") ||
		(strings.Contains(host, "amazon.") && strings.Contains(path, "/images/"))
}

func shouldSkipProductImageURL(imageURL string) bool {
	lower := strings.ToLower(imageURL)
	skipParts := []string{
		"/sash/",
		"inlineplayer",
		"/play.png",
		"/pause.png",
		"audiooff.png",
		"audioon.png",
		"sprite",
		"grey-pixel",
		"transparent-pixel",
		"avatar",
		"profile",
	}
	for _, part := range skipParts {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

func selectProductImages(candidates []ProductImageCandidate, options ProductImageExtractionOptions) []ProductImageCandidate {
	selected := make([]ProductImageCandidate, 0)
	if options.Include == "all" || options.Include == productImageRoleMain {
		selected = append(selected, selectProductImageRole(candidates, productImageRoleMain, options.MaxMain)...)
	}
	if options.Include == "all" || options.Include == productImageRoleAPlus {
		selected = append(selected, selectProductImageRole(candidates, productImageRoleAPlus, options.MaxAPlus)...)
	}
	return selected
}

func selectProductImageRole(candidates []ProductImageCandidate, role string, limit int) []ProductImageCandidate {
	groups := map[string][]ProductImageCandidate{}
	order := map[string]int{}
	for _, candidate := range candidates {
		if candidate.Role != role {
			continue
		}
		key := candidate.Key
		if key == "" {
			key = candidate.URL
		}
		groups[key] = append(groups[key], candidate)
		if _, ok := order[key]; !ok || candidate.Order < order[key] {
			order[key] = candidate.Order
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return order[keys[i]] < order[keys[j]]
	})

	selected := make([]ProductImageCandidate, 0, len(keys))
	for _, key := range keys {
		best := bestProductImageCandidate(groups[key])
		selected = append(selected, best)
		if limit > 0 && len(selected) >= limit {
			break
		}
	}
	return selected
}

func bestProductImageCandidate(candidates []ProductImageCandidate) ProductImageCandidate {
	sort.SliceStable(candidates, func(i, j int) bool {
		return productImageCandidateScore(candidates[i]) > productImageCandidateScore(candidates[j])
	})
	return candidates[0]
}

func productImageCandidateScore(candidate ProductImageCandidate) int {
	score := candidate.Width * candidate.Height
	lowerURL := strings.ToLower(candidate.URL)
	lowerSource := strings.ToLower(candidate.Source)
	if strings.Contains(lowerSource, "hires") || strings.Contains(lowerSource, "large") {
		score += 3000000
	}
	if strings.Contains(lowerURL, "._sl1500_.") || strings.Contains(lowerURL, "._sl2000_.") || strings.Contains(lowerURL, "._sx1500_.") {
		score += 2000000
	}
	if imageModifierRe.MatchString(lowerURL) {
		score += 500000
	}
	return score
}

func countProductImageRole(candidates []ProductImageCandidate, role string) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Role == role {
			count++
		}
	}
	return count
}

func extractProductImageASIN(doc *goquery.Document, pageURL string) string {
	if value, ok := doc.Find("#ASIN, input[name='ASIN']").First().Attr("value"); ok {
		return strings.ToUpper(strings.TrimSpace(value))
	}
	if href, ok := doc.Find("link[rel='canonical']").First().Attr("href"); ok {
		if asin := extractASINFromString(href); asin != "" {
			return asin
		}
	}
	return extractASINFromString(pageURL)
}

func extractProductImageTitle(doc *goquery.Document) string {
	title := cleanText(doc.Find("#productTitle").First().Text())
	if title != "" {
		return title
	}
	return cleanText(strings.TrimSuffix(doc.Find("title").First().Text(), "- Amazon"))
}

func amazonImageKey(imageURL string) string {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(parsed.Path, "/")
	file := ""
	if len(segments) > 0 {
		file = segments[len(segments)-1]
	}
	file, _ = url.PathUnescape(file)
	file = imageModifierRe.ReplaceAllString(file, "")
	file = imageExtensionRe.ReplaceAllString(file, "")
	if dot := strings.Index(file, "."); dot >= 0 {
		file = file[:dot]
	}
	file = cleanText(file)
	if len(file) > 64 {
		return file[:64]
	}
	return file
}
