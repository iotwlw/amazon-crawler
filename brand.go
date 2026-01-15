package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

// 品牌巡查状态常量
const (
	BRAND_PATROL_PENDING    = 0 // 待巡查
	BRAND_PATROL_PROCESSING = 1 // 巡查中
	BRAND_PATROL_COMPLETED  = 2 // 已完成
	BRAND_PATROL_FAILED     = 3 // 失败
	BRAND_PATROL_NO_RESULT  = 4 // 无结果
)

// BrandConfig 品牌巡查配置
type BrandConfig struct {
	Enable   bool `yaml:"enable"`
	MaxASINs int  `yaml:"max_asins"` // 每个品牌最多搜索的ASIN数，默认5
	Batch    int  `yaml:"batch"`     // 每批处理数量，默认100
	Loop     int  `yaml:"loop"`      // 循环次数，0=无限
}

// brandStruct 品牌巡查结构体
type brandStruct struct {
	id           int64
	brandName    string
	domain       string
	matchedAsins string // 逗号分隔的ASIN列表
	sourceRank   int

	// 巡查结果
	asins      []string
	sellerID   string
	sellerName string
	// 店铺信息（写入 tb_amazon_shop）
	shopName       string
	companyName    string
	companyAddress string
	fb1month       int
	fb3month       int
	fb12month      int
	fbLifetime     int
}

// brandMain 品牌巡查主入口
func brandMain() {
	log.Info("========================")
	log.Info("启动品牌巡查模式")
	log.Info("========================")

	maxASINs := app.Brand.MaxASINs
	if maxASINs == 0 {
		maxASINs = 5
	}
	batch := app.Brand.Batch
	if batch == 0 {
		batch = 100
	}
	loop := app.Brand.Loop
	if loop == 0 {
		loop = 999999
	}

	log.Infof("配置: 每品牌最多 %d 个ASIN, 每批 %d 条, 循环 %d 次", maxASINs, batch, loop)

	for i := 0; i < loop; i++ {
		log.Infof("------------------------")
		log.Infof("第 %d 轮巡查开始", i+1)

		processed := processBrandBatch(batch, maxASINs)
		if processed == 0 {
			log.Info("没有待处理的品牌，巡查结束")
			break
		}

		log.Infof("第 %d 轮巡查完成，处理了 %d 个品牌", i+1, processed)
	}

	log.Info("========================")
	log.Info("品牌巡查模式结束")
	log.Info("========================")
}

// processBrandBatch 处理一批品牌
func processBrandBatch(batch int, maxASINs int) int {
	// 1. 批量标记为处理中
	result, err := app.db.Exec(`
		UPDATE available_brand_domains
		SET patrol_status = ?, patrol_app_id = ?
		WHERE patrol_status = ?
		ORDER BY
			CASE WHEN matched_asins IS NOT NULL AND matched_asins != '' THEN 0 ELSE 1 END,
			source_rank ASC
		LIMIT ?
	`, BRAND_PATROL_PROCESSING, app.Basic.App_id, BRAND_PATROL_PENDING, batch)
	if err != nil {
		log.Errorf("批量标记品牌失败: %v", err)
		return 0
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return 0
	}
	log.Infof("标记 %d 个品牌为处理中", affected)

	// 2. 查询待处理品牌
	rows, err := app.db.Query(`
		SELECT id, brand_name, domain, COALESCE(matched_asins, ''), COALESCE(source_rank, 0)
		FROM available_brand_domains
		WHERE patrol_status = ? AND patrol_app_id = ?
	`, BRAND_PATROL_PROCESSING, app.Basic.App_id)
	if err != nil {
		log.Errorf("查询品牌失败: %v", err)
		return 0
	}
	defer rows.Close()

	// 3. 逐个处理
	processed := 0
	for rows.Next() {
		var b brandStruct
		if err := rows.Scan(&b.id, &b.brandName, &b.domain, &b.matchedAsins, &b.sourceRank); err != nil {
			log.Errorf("扫描品牌数据失败: %v", err)
			continue
		}

		// 重置结果
		b.asins = nil
		b.sellerID = ""
		b.sellerName = ""
		b.shopName = ""
		b.companyName = ""
		b.companyAddress = ""
		b.fb1month = 0
		b.fb3month = 0
		b.fb12month = 0
		b.fbLifetime = 0

		log.Infof("处理品牌: %s (rank=%d)", b.brandName, b.sourceRank)

		// 处理品牌
		err := b.process(maxASINs)
		if err != nil {
			log.Errorf("处理品牌 %s 失败: %v", b.brandName, err)
			b.updateStatus(BRAND_PATROL_FAILED, err.Error())
		} else if b.sellerID == "" {
			log.Warnf("品牌 %s 未找到卖家", b.brandName)
			b.updateStatus(BRAND_PATROL_NO_RESULT, "未找到卖家")
		} else {
			log.Infof("品牌 %s 巡查完成, seller_id=%s", b.brandName, b.sellerID)
			b.updateStatus(BRAND_PATROL_COMPLETED, "")
		}

		processed++
	}

	return processed
}

// process 处理单个品牌
func (b *brandStruct) process(maxASINs int) error {
	if b.matchedAsins != "" {
		// 路径1: 有ASIN，直接访问商品页
		log.Infof("使用已有ASIN: %s", b.matchedAsins)
		return b.processWithASIN()
	}
	// 路径2: 无ASIN，搜索品牌名
	log.Infof("搜索品牌名: %s", b.brandName)
	return b.processWithSearch(maxASINs)
}

// processWithASIN 有ASIN的处理路径
func (b *brandStruct) processWithASIN() error {
	asins := strings.Split(b.matchedAsins, ",")
	for _, asin := range asins {
		asin = strings.TrimSpace(asin)
		if asin == "" {
			continue
		}

		log.Infof("访问ASIN: %s", asin)
		if err := b.fetchProductPage(asin); err != nil {
			log.Warnf("访问ASIN %s 失败: %v", asin, err)
			continue
		}

		if b.sellerID != "" {
			log.Infof("找到卖家: %s", b.sellerID)
			return b.fetchSellerInfo()
		}
	}
	return nil
}

// processWithSearch 无ASIN的处理路径（搜索）
func (b *brandStruct) processWithSearch(maxASINs int) error {
	// 1. 搜索品牌名
	if err := b.search(maxASINs); err != nil {
		return err
	}

	if len(b.asins) == 0 {
		log.Warnf("搜索品牌 %s 未找到ASIN", b.brandName)
		return nil
	}

	log.Infof("搜索到 %d 个ASIN: %v", len(b.asins), b.asins)

	// 2. 处理搜索到的ASIN
	for _, asin := range b.asins {
		log.Infof("访问ASIN: %s", asin)
		if err := b.fetchProductPage(asin); err != nil {
			log.Warnf("访问ASIN %s 失败: %v", asin, err)
			continue
		}

		if b.sellerID != "" {
			log.Infof("找到卖家: %s", b.sellerID)
			return b.fetchSellerInfo()
		}
	}
	return nil
}

// search 搜索品牌名获取ASIN
func (b *brandStruct) search(maxASINs int) error {
	searchURL := fmt.Sprintf("https://%s/s?k=%s", app.Domain, url.QueryEscape(b.brandName))

	if err := robot.IsAllow(userAgent, searchURL); err != nil {
		return fmt.Errorf("robots.txt 不允许: %v", err)
	}

	doc, err := b.request(searchURL)
	if err != nil {
		return err
	}

	b.extractASINs(doc, maxASINs)
	return nil
}

// extractASINs 从搜索结果提取ASIN
func (b *brandStruct) extractASINs(doc *goquery.Document, maxASINs int) {
	doc.Find("div[data-asin]").Each(func(i int, s *goquery.Selection) {
		if len(b.asins) >= maxASINs {
			return
		}
		asin, exists := s.Attr("data-asin")
		if exists && asin != "" && asin != "0" {
			b.asins = append(b.asins, asin)
		}
	})
}

// fetchProductPage 访问商品页提取卖家信息
func (b *brandStruct) fetchProductPage(asin string) error {
	productURL := fmt.Sprintf("https://%s/dp/%s", app.Domain, asin)

	if err := robot.IsAllow(userAgent, productURL); err != nil {
		return fmt.Errorf("robots.txt 不允许: %v", err)
	}

	doc, err := b.request(productURL)
	if err != nil {
		return err
	}

	// 提取卖家链接
	sellerLink := doc.Find("a#sellerProfileTriggerId").First()
	if sellerLink.Length() == 0 {
		return ERROR_NOT_SELLER_URL
	}

	href, _ := sellerLink.Attr("href")
	b.sellerID = extractSellerID(href)
	b.sellerName = strings.TrimSpace(sellerLink.Text())
	b.shopName = b.sellerName

	return nil
}

// extractSellerID 从URL中提取seller ID
func extractSellerID(href string) string {
	// URL 格式: /sp?seller=XXXXX 或 /sp?ie=UTF8&seller=XXXXX
	if strings.Contains(href, "seller=") {
		parts := strings.Split(href, "seller=")
		if len(parts) > 1 {
			sellerPart := parts[1]
			// 去掉后面的参数
			if idx := strings.Index(sellerPart, "&"); idx > 0 {
				sellerPart = sellerPart[:idx]
			}
			return sellerPart
		}
	}
	return ""
}

// fetchSellerInfo 获取卖家详细信息并写入 tb_amazon_shop
func (b *brandStruct) fetchSellerInfo() error {
	sellerURL := fmt.Sprintf("https://%s/sp?ie=UTF8&seller=%s", app.Domain, b.sellerID)

	if err := robot.IsAllow(userAgent, sellerURL); err != nil {
		return fmt.Errorf("robots.txt 不允许: %v", err)
	}

	doc, err := b.request(sellerURL)
	if err != nil {
		return err
	}

	// 提取卖家信息（复用 seller.go 的解析逻辑）
	b.extractSellerDetails(doc)

	// 写入 tb_amazon_shop 表
	return b.saveToAmazonShop()
}

// extractSellerDetails 从卖家页面提取详细信息
func (b *brandStruct) extractSellerDetails(doc *goquery.Document) {
	// 提取商家信息
	sellerTxt := doc.Find("div#page-section-detail-seller-info").Find("span").Text()

	var info []string
	for _, line := range strings.Split(sellerTxt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "Business Name:" {
			info = append(info, line)
		} else if strings.Contains(line, "Business Name:") {
			line = strings.ReplaceAll(line, "Business Name:", "")
			info = append(info, line)
			info = append(info, "Business Type:")
		} else if strings.Contains(line, "Business Address:") {
			line = strings.ReplaceAll(line, "Business Address:", "")
			info = append(info, line)
			info = append(info, "Business Address:")
		} else {
			info = append(info, line)
		}
	}

	for i, line := range info {
		if strings.Contains(line, "Business Name") && i+1 < len(info) {
			b.companyName = info[i+1]
		} else if strings.Contains(line, "Business Address") && i+1 < len(info) {
			b.companyAddress = strings.Join(info[i+1:], " ")
		}
	}

	// 提取 FB 数据
	fbSummary := doc.Find("div#seller-feedback-summary-rating").First()
	if fbSummary.Length() > 0 {
		thirtyElement := fbSummary.Find("div#rating-thirty").First()
		if thirtyElement.Length() > 0 {
			countText := thirtyElement.Find("span.ratings-reviews-count").First().Text()
			countText = strings.ReplaceAll(countText, ",", "")
			fmt.Sscanf(countText, "%d", &b.fb1month)
		}

		ninetyElement := fbSummary.Find("div#rating-ninety").First()
		if ninetyElement.Length() > 0 {
			countText := ninetyElement.Find("span.ratings-reviews-count").First().Text()
			countText = strings.ReplaceAll(countText, ",", "")
			fmt.Sscanf(countText, "%d", &b.fb3month)
		}

		yearElement := fbSummary.Find("div#rating-year").First()
		if yearElement.Length() > 0 {
			countText := yearElement.Find("span.ratings-reviews-count").First().Text()
			countText = strings.ReplaceAll(countText, ",", "")
			fmt.Sscanf(countText, "%d", &b.fb12month)
		}

		lifetimeElement := fbSummary.Find("div#rating-lifetime").First()
		if lifetimeElement.Length() > 0 {
			countText := lifetimeElement.Find("span.ratings-reviews-count").First().Text()
			countText = strings.ReplaceAll(countText, ",", "")
			fmt.Sscanf(countText, "%d", &b.fbLifetime)
		}
	}

	log.Infof("提取卖家信息: company=%s, address=%s, fb=%d/%d/%d/%d",
		b.companyName, b.companyAddress, b.fb1month, b.fb3month, b.fb12month, b.fbLifetime)
}

// saveToAmazonShop 保存店铺信息到 tb_amazon_shop
func (b *brandStruct) saveToAmazonShop() error {
	// domain 字段使用品牌名（小写）
	domain := strings.ToLower(b.brandName)
	shopUrl := fmt.Sprintf("https://%s/sp?ie=UTF8&seller=%s", app.Domain, b.sellerID)

	// 先查询是否存在
	var existingId int
	err := app.db.QueryRow(
		"SELECT id FROM tb_amazon_shop WHERE domain = ? AND shop_id = ?",
		domain, b.sellerID,
	).Scan(&existingId)

	if err == sql.ErrNoRows {
		// 插入新记录
		_, err = app.db.Exec(`
			INSERT INTO tb_amazon_shop (
				user_id, domain, shop_id, shop_name, shop_url, marketplace,
				company_name, company_address,
				fb_1month, fb_3month, fb_12month, fb_lifetime,
				main_products, avg_price, estimated_monthly_sales,
				crawl_time, create_time, update_time
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 0, 0, NOW(), NOW(), NOW())
		`, 1, domain, b.sellerID, b.shopName, shopUrl, "US",
			b.companyName, b.companyAddress,
			b.fb1month, b.fb3month, b.fb12month, b.fbLifetime)
		if err != nil {
			return fmt.Errorf("插入 tb_amazon_shop 失败: %w", err)
		}
		log.Infof("成功插入 tb_amazon_shop: domain=%s, shop_id=%s", domain, b.sellerID)
	} else if err != nil {
		return fmt.Errorf("查询 tb_amazon_shop 失败: %w", err)
	} else {
		// 更新现有记录
		_, err = app.db.Exec(`
			UPDATE tb_amazon_shop SET
				shop_name = ?, shop_url = ?,
				company_name = ?, company_address = ?,
				fb_1month = ?, fb_3month = ?, fb_12month = ?, fb_lifetime = ?,
				crawl_time = NOW(), update_time = NOW()
			WHERE id = ?
		`, b.shopName, shopUrl, b.companyName, b.companyAddress,
			b.fb1month, b.fb3month, b.fb12month, b.fbLifetime, existingId)
		if err != nil {
			return fmt.Errorf("更新 tb_amazon_shop 失败: %w", err)
		}
		log.Infof("成功更新 tb_amazon_shop: id=%d", existingId)
	}

	return nil
}

// request 发送HTTP请求
func (b *brandStruct) request(reqURL string) (*goquery.Document, error) {
	log.Infof("请求: %s", reqURL)

	client := get_client()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authority", app.Domain)
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("cache-control", `max-age=0`)
	req.Header.Set("User-Agent", userAgent)

	if _, err := app.get_cookie(); err != nil {
		log.Warnf("获取 cookie 失败: %v", err)
	} else {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		break
	case 404:
		return nil, ERROR_NOT_404
	case 503:
		return nil, ERROR_NOT_503
	default:
		return nil, fmt.Errorf("状态码: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	// 验证页面检测
	if doc.Find("h4").First().Text() == "Enter the characters you see below" {
		// Cookie 失效，尝试切换
		if err := app.handleCookieInvalid(); err != nil {
			log.Errorf("处理 cookie 失效失败: %v", err)
		}
		return nil, ERROR_VERIFICATION
	}

	return doc, nil
}

// updateStatus 更新巡查状态
func (b *brandStruct) updateStatus(status int, errMsg string) error {
	// 如果搜索到了新的 ASIN，同时更新 matched_asins 字段
	if len(b.asins) > 0 {
		_, err := app.db.Exec(`
			UPDATE available_brand_domains SET
				patrol_status = ?,
				patrol_app_id = ?,
				matched_asins = ?,
				patrol_error = ?
			WHERE id = ?
		`, status, app.Basic.App_id, strings.Join(b.asins, ","), errMsg, b.id)
		return err
	}

	_, err := app.db.Exec(`
		UPDATE available_brand_domains SET
			patrol_status = ?,
			patrol_app_id = ?,
			patrol_error = ?
		WHERE id = ?
	`, status, app.Basic.App_id, errMsg, b.id)
	return err
}
