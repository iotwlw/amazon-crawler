package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

// ASINScraper ASIN 评论爬虫结构体
type ASINScraper struct {
	asinList  []string     // ASIN 列表
	domain    string       // 目标域名
	results   []ASINResult // 抓取结果
	outputDir string       // 输出目录
}

// ASINResult 单个 ASIN 的抓取结果
type ASINResult struct {
	ASIN         string  // ASIN
	Rating       float64 // 评分
	ReviewCount  int     // 评论数
	Status       string  // 状态: success/error
	ErrorMessage string  // 错误信息
}

// NewASINScraper 创建 ASIN 爬虫实例
func NewASINScraper(asinStr, domain string) *ASINScraper {
	// 解析 ASIN 列表
	asinList := strings.Split(asinStr, ",")
	for i := range asinList {
		asinList[i] = strings.TrimSpace(asinList[i])
	}

	return &ASINScraper{
		asinList:  asinList,
		domain:    domain,
		results:   make([]ASINResult, 0, len(asinList)),
		outputDir: "output", // 默认输出目录
	}
}

// Run 执行爬虫主流程
func (s *ASINScraper) Run() error {
	log.Infof("开始处理 %d 个 ASIN", len(s.asinList))

	// 1. 获取 Cookie
	if _, err := app.get_cookie(); err != nil {
		log.Warnf("获取 Cookie 失败: %v，将不使用 Cookie", err)
	}

	// 2. 遍历 ASIN 列表
	for i, asin := range s.asinList {
		log.Infof("进度: %d/%d - 处理 ASIN: %s", i+1, len(s.asinList), asin)

		result := s.scrapeASIN(asin)
		s.results = append(s.results, result)

		// 延迟等待（2-3 秒）
		if i < len(s.asinList)-1 {
			delay := 2 + rangdom_range(2)
			log.Infof("等待 %d 秒后继续...", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	// 3. 导出 CSV
	return s.exportCSV()
}

// scrapeASIN 爬取单个 ASIN 的数据
func (s *ASINScraper) scrapeASIN(asin string) ASINResult {
	result := ASINResult{
		ASIN:   asin,
		Status: "success",
	}

	// 构建产品页面 URL
	url := fmt.Sprintf("https://%s/dp/%s", s.domain, asin)

	// 检查 robots.txt
	if err := robot.IsAllow(userAgent, url); err != nil {
		log.Errorf("robots.txt 不允许访问: %v", err)
		result.Status = "error"
		result.ErrorMessage = err.Error()
		return result
	}

	// 发送请求
	client := get_client()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = err.Error()
		return result
	}

	// 设置请求头（复用 product.go 的逻辑）
	s.setRequestHeaders(req)

	// 添加 Cookie
	if app.cookie != "" {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = err.Error()
		return result
	}

	// 检查验证页面
	if doc.Find("h4").First().Text() == "Enter the characters you see below" {
		result.Status = "error"
		result.ErrorMessage = "需要验证"
		// 尝试切换 Cookie
		if err := app.handleCookieInvalid(); err != nil {
			log.Errorf("切换 Cookie 失败: %v", err)
		}
		return result
	}

	// 提取评论数
	result.ReviewCount = s.extractReviewCount(doc)

	// 提取评分
	// 如果评论数为0，则评分强制为0（避免误提取到其他元素的 5.0 文本）
	if result.ReviewCount == 0 {
		result.Rating = 0
	} else {
		result.Rating = s.extractRating(doc)
	}

	log.Infof("ASIN: %s, 评分: %.1f, 评论数: %d", asin, result.Rating, result.ReviewCount)

	return result
}

// extractRating 提取评分
func (s *ASINScraper) extractRating(doc *goquery.Document) float64 {
	// 尝试多种选择器
	selectors := []string{
		"[data-hook=\"average-star-rating\"] .a-icon-alt",
		"[data-hook=\"review-out-of-town\"] .a-icon-alt",
		"i[data-hook=\"average-star-rating\"]",
		"#averageCustomerReviews .a-size-small .a-color-base",
		// 去掉过于通用的 ".a-icon-alt" 和 "span.a-size-small.a-color-base"，因为它们容易误匹配品牌或广告中的文本
	}

	for _, selector := range selectors {
		var foundRating float64
		doc.Find(selector).EachWithBreak(func(i int, elem *goquery.Selection) bool {
			text := strings.TrimSpace(elem.Text())
			if text == "" {
				return true
			}

			// 1. 尝试解析类似 "4.5 out of 5 stars" 的文本
			var rating float64
			if _, err := fmt.Sscanf(text, "%f out of", &rating); err == nil {
				foundRating = rating
				return false
			}

			// 2. 尝试直接解析纯数字 "4.2"
			// 跳过非数字前缀，提取第一个出现的数字/点组合
			cleanText := ""
			started := false
			for _, r := range text {
				if (r >= '0' && r <= '9') || r == '.' {
					cleanText += string(r)
					started = true
				} else if started {
					break
				}
			}

			if cleanText != "" {
				if _, err := fmt.Sscanf(cleanText, "%f", &rating); err == nil {
					// 评分通常在 0-5 之间
					if rating > 0 && rating <= 5 {
						foundRating = rating
						return false
					}
				}
			}

			return true
		})

		if foundRating > 0 {
			return foundRating
		}
	}

	return 0
}

// extractReviewCount 提取评论数
func (s *ASINScraper) extractReviewCount(doc *goquery.Document) int {
	// 尝试多种选择器
	selectors := []string{
		"[data-hook=\"total-review-count\"]",
		"#acrCustomerReviewText",
		"[data-hook=\"review-out-of-town\"]",
		"#averageCustomerReviews .a-size-base.a-color-secondary",
	}

	for _, selector := range selectors {
		var foundCount int
		doc.Find(selector).EachWithBreak(func(i int, elem *goquery.Selection) bool {
			text := strings.TrimSpace(elem.Text())
			if text == "" {
				return true
			}

			// 提取所有连续的数字，忽略中间的逗号或点（作为千分位）
			cleanText := ""
			started := false
			for _, r := range text {
				if r >= '0' && r <= '9' {
					cleanText += string(r)
					started = true
				} else if started && (r == ',' || r == '.') {
					// 可能是千分位符，继续看后面是不是数字
					continue
				} else if started {
					// 遇到其他字符且已经开始提取，则结束
					break
				}
			}

			if cleanText != "" {
				var count int
				if _, err := fmt.Sscanf(cleanText, "%d", &count); err == nil {
					foundCount = count
					return false
				}
			}
			return true
		})

		if foundCount > 0 {
			return foundCount
		}
	}

	return 0
}

// setRequestHeaders 设置请求头
func (s *ASINScraper) setRequestHeaders(req *http.Request) {
	req.Header.Set("Authority", s.domain)
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8`)
	req.Header.Set("Accept-Language", `es-MX,es;q=0.9,en;q=0.8`)
	req.Header.Set("Cache-Control", `max-age=0`)
	req.Header.Set("Upgrade-Insecure-Requests", `1`)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Sec-Fetch-Dest", `empty`)
	req.Header.Set("Sec-Fetch-Mode", `cors`)
	req.Header.Set("Sec-Fetch-Site", `same-origin`)
}

// exportCSV 导出 CSV 文件
func (s *ASINScraper) exportCSV() error {
	// 创建输出目录
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 生成文件名（带时间戳）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/asins_%s.csv", s.outputDir, timestamp)

	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建 CSV 文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{"ASIN", "Rating", "ReviewCount", "Status", "ErrorMessage"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// 写入数据
	for _, r := range s.results {
		var ratingStr string
		if r.Rating > 0 {
			ratingStr = fmt.Sprintf("%.1f", r.Rating)
		}

		var reviewCountStr string
		if r.ReviewCount > 0 {
			reviewCountStr = fmt.Sprintf("%d", r.ReviewCount)
		}

		record := []string{r.ASIN, ratingStr, reviewCountStr, r.Status, r.ErrorMessage}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	log.Infof("CSV 文件已导出: %s", filename)
	return nil
}
