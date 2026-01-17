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

// ProductInfo 商品信息（从搜索结果获取）
type ProductInfo struct {
	URL         string // 商品URL
	Param       string // URL参数
	Title       string // 商品标题
	ASIN        string // 亚马逊商品标识（去重键）
	Keyword     string // 关键词/品牌名
	BoughtCount string // 购买次数
	Price       string // 价格
	Rating      string // 评分
	ReviewCount string // 评论数
}

// SellerInfo 卖家信息（从商品页提取）
type SellerInfo struct {
	SellerID    string   // 卖家ID（去重键）
	SellerName  string   // 卖家名称
	Keyword     string   // 来源关键词
	ProductURLs []string // 关联的商品URL列表（可选，用于统计）
}

// SellerDetail 卖家详情（从卖家页获取）
type SellerDetail struct {
	SellerID   string // 卖家ID
	SellerName string // 卖家名称
	Keyword    string // 来源关键词
	Name       string // 公司名称
	Address    string // 公司地址
	TRN        string // 税号
	TRNStatus  int    // 税号状态
	AllStatus  int    // 信息状态
	FB1Month   int    // 1个月反馈数
	FB3Month   int    // 3个月反馈数
	FB12Month  int    // 12个月反馈数
	FBLifetime int    // 总反馈数
}

// ExecuteCrawl 执行单个关键词的完整爬取流程
// 流程: 搜索商品 -> 提取卖家ID -> 获取卖家详情
func ExecuteCrawl(task CrawlTask) {
	ExecuteCrawlWithStatus(task)
}

// ExecuteCrawlWithStatus 执行单个关键词的完整爬取流程，返回是否成功
// 使用内存传递模式，最后批量写入数据库
func ExecuteCrawlWithStatus(task CrawlTask) bool {
	keyword := task.Keyword
	log.Infof("========================================")
	log.Infof("开始爬取关键词: %s (内存优化模式)", keyword)
	log.Infof("========================================")

	// 阶段1: 搜索商品（返回内存列表，不写数据库）
	products, err := crawlSearchInMemory(keyword, task.ID)
	if err != nil {
		log.Errorf("搜索阶段失败: %s, 错误: %v", keyword, err)
		// 更新任务状态为失败
		app.db.Exec("UPDATE amc_category SET task_status = ? WHERE id = ?", TASK_STATUS_FAILED, task.ID)
		return false
	}

	if len(products) == 0 {
		log.Warnf("没有找到商品: %s", keyword)
		// 更新任务状态为完成（虽然没找到商品）
		app.db.Exec("UPDATE amc_category SET task_status = ? WHERE id = ?", TASK_STATUS_COMPLETED, task.ID)
		return true
	}

	// 阶段2: 从商品列表中提取卖家信息（内存去重）
	sellerMap, err := crawlProductsFromMemory(products, keyword)
	if err != nil {
		log.Errorf("提取卖家信息失败: %s, 错误: %v", keyword, err)
		app.db.Exec("UPDATE amc_category SET task_status = ? WHERE id = ?", TASK_STATUS_FAILED, task.ID)
		return false
	}

	if len(sellerMap) == 0 {
		log.Warnf("没有找到卖家: %s", keyword)
		// 仍然保存商品数据
		_ = batchSaveAll(keyword, task.ID, products, []*SellerDetail{})
		return true
	}

	// 阶段3: 获取卖家详情
	sellerDetails, err := fetchSellerDetails(sellerMap)
	if err != nil {
		log.Errorf("获取卖家详情失败: %s, 错误: %v", keyword, err)
		app.db.Exec("UPDATE amc_category SET task_status = ? WHERE id = ?", TASK_STATUS_FAILED, task.ID)
		return false
	}

	// 阶段4: 批量保存所有数据到数据库（事务）
	if err := batchSaveAll(keyword, task.ID, products, sellerDetails); err != nil {
		log.Errorf("批量保存数据失败: %s, 错误: %v", keyword, err)
		app.db.Exec("UPDATE amc_category SET task_status = ? WHERE id = ?", TASK_STATUS_FAILED, task.ID)
		return false
	}

	log.Infof("========================================")
	log.Infof("关键词爬取完成: %s (商品=%d, 卖家=%d)", keyword, len(products), len(sellerDetails))
	log.Infof("========================================")
	return true
}

// ============================================================
// 以下为旧的数据库传递模式（命令行模式使用，保持兼容）
// ============================================================

// ExecuteCrawlLegacy 旧的执行方式（数据库传递模式），保留用于命令行模式
func ExecuteCrawlLegacy(task CrawlTask) bool {
	keyword := task.Keyword
	log.Infof("========================================")
	log.Infof("开始爬取关键词: %s (传统模式)", keyword)
	log.Infof("========================================")

	// 阶段1: 搜索商品（传入任务 ID 用于搜索统计）
	if err := crawlSearch(keyword, task.ID); err != nil {
		log.Errorf("搜索阶段失败: %s, 错误: %v", keyword, err)
		return false
	}

	// 阶段2: 提取卖家ID
	if err := crawlProduct(keyword); err != nil {
		log.Errorf("产品处理阶段失败: %s, 错误: %v", keyword, err)
		return false
	}

	// 阶段3: 获取卖家详情
	if err := crawlSeller(keyword); err != nil {
		log.Errorf("卖家信息获取阶段失败: %s, 错误: %v", keyword, err)
		return false
	}

	log.Infof("========================================")
	log.Infof("关键词爬取完成: %s", keyword)
	log.Infof("========================================")
	return true
}

// crawlSearch 针对单个关键词执行搜索
func crawlSearch(keyword string, categoryID int64) error {
	log.Infof("------------------------")
	log.Infof("1. 开始搜索关键词: %s", keyword)

	var s searchStruct
	s.en_key = formatKeyword(keyword)
	s.zh_key = keyword
	s.category_id = categoryID

	// 插入搜索统计记录（使用真实的 category_id）
	insert_id, err := s.search_start()
	if err != nil {
		log.Errorf("插入搜索统计失败: %v", err)
		return err
	}

	s.start = 1
	s.end = 2 // 只搜索首页
	s.valid = 0

	for ; s.start < s.end; s.start++ {
		h, err := s.request(s.start)
		switch err {
		case nil:
			break
		case ERROR_NOT_404, ERROR_NOT_503:
			s.start--
			sleep(120)
			continue
		default:
			log.Error(err)
			continue
		}
		s.get_product_url(h)
	}

	if err := s.search_end(insert_id); err != nil {
		log.Errorf("更新搜索统计失败: %v", err)
		return err
	}

	log.Infof("------------------------")
	return nil
}

// crawlProduct 处理与关键词相关的产品
func crawlProduct(keyword string) error {
	log.Infof("------------------------")
	log.Infof("2. 开始处理关键词相关产品: %s", keyword)

	formattedKeyword := formatKeyword(keyword)

	// 更新该关键词相关的产品状态
	_, err := app.db.Exec("UPDATE amc_product SET status = ?, app = ? WHERE (status = ? OR status = ?) AND keyword = ? LIMIT 1000",
		MYSQL_PRODUCT_STATUS_CHEKCK, app.Basic.App_id, MYSQL_PRODUCT_STATUS_INSERT, MYSQL_PRODUCT_STATUS_ERROR_OVER, formattedKeyword)
	if err != nil {
		log.Errorf("更新product表失败: %v", err)
		return err
	}

	row, err := app.db.Query(`SELECT id, url, param, keyword FROM amc_product WHERE status = ? AND app = ? AND keyword = ?`,
		MYSQL_PRODUCT_STATUS_CHEKCK, app.Basic.App_id, formattedKeyword)
	if err != nil {
		log.Errorf("查询product表失败: %v", err)
		return err
	}
	defer row.Close()

	var product productStruct
	for row.Next() {
		var primary_id int64
		var url, param, kw string
		if err := row.Scan(&primary_id, &url, &param, &kw); err != nil {
			log.Errorf("获取product表的值失败: %v", err)
			continue
		}
		if strings.HasPrefix(url, "http") {
			continue
		}
		url = "https://" + app.Domain + url + param
		if err := robot.IsAllow(userAgent, url); err != nil {
			log.Errorf("%v", err)
			continue
		}

		log.Infof("查找商品链接 ID:%d url:%s", primary_id, url)
		err := product.request(url)
		if err != nil {
			if err == ERROR_NOT_SELLER_URL {
				product.update_status(primary_id, MYSQL_PRODUCT_STATUS_NO_PRODUCT, "", "", "")
				continue
			} else if err == ERROR_NOT_404 || err == ERROR_NOT_503 {
				product.update_status(primary_id, MYSQL_PRODUCT_STATUS_ERROR_OVER, "", "", "")
				log.Error(err)
				sleep(300)
				continue
			} else if err == ERROR_VERIFICATION {
				// Cookie 失效，标记失效并尝试获取新的
				product.update_status(primary_id, MYSQL_PRODUCT_STATUS_ERROR_OVER, "", "", "")
				log.Error(err)
				if err := app.handleCookieInvalid(); err != nil {
					log.Errorf("处理 cookie 失效失败: %v", err)
				}
				sleep(300)
				continue
			} else {
				product.update_status(primary_id, MYSQL_PRODUCT_STATUS_ERROR_OVER, "", "", "")
				log.Error(err)
				sleep(300)
				continue
			}
		}

		currentSellerID := product.get_seller_id()
		currentBrandName := product.brand_name
		currentBrandStoreURL := product.brand_store_url
		currentSellerName := product.seller_name
		currentKeyword := kw

		if currentSellerID == "" && currentBrandName == "" {
			product.update_status(primary_id, MYSQL_PRODUCT_STATUS_NO_PRODUCT, "", "", "")
			continue
		}

		if strings.ToLower(currentKeyword) == strings.ToLower(currentBrandName) && currentSellerID != "" {
			err = product.insert_selll_id(currentSellerID, currentSellerName, currentKeyword)
			if is_duplicate_entry(err) {
				log.Infof("店铺已存在 商家ID:%s", currentSellerID)
				err = nil
			}
			if err != nil {
				log.Error(err)
				continue
			}
		}
		if err := product.update_status(primary_id, MYSQL_PRODUCT_STATUS_OVER, currentSellerID, currentBrandName, currentBrandStoreURL); err != nil {
			log.Error(err)
			continue
		}
	}

	log.Infof("2. 结束处理关键词相关产品: %s", keyword)
	log.Infof("------------------------")
	return nil
}

// crawlSeller 处理与关键词相关的卖家
func crawlSeller(keyword string) error {
	log.Infof("------------------------")
	log.Infof("3. 开始获取关键词相关卖家信息: %s", keyword)

	formattedKeyword := formatKeyword(keyword)

	// 更新该关键词相关的卖家状态
	_, err := app.db.Exec("UPDATE amc_seller SET app_id = ? WHERE all_status = ? AND keyword = ? LIMIT 100",
		app.Basic.App_id, MYSQL_SELLER_STATUS_INFO_INSERT, formattedKeyword)
	if err != nil {
		log.Errorf("更新seller表失败: %v", err)
		return err
	}

	row, err := app.db.Query("SELECT id, seller_id, seller_name, keyword FROM amc_seller WHERE all_status = ? AND app_id = ? AND keyword = ?",
		MYSQL_SELLER_STATUS_INFO_INSERT, app.Basic.App_id, formattedKeyword)
	if err != nil {
		log.Errorf("查询seller表失败: %v", err)
		return err
	}
	defer row.Close()

	var seller sellerStruct
	for row.Next() {
		seller.seller_name = ""
		seller.keyword = ""
		seller.businessName = ""
		seller.address = ""
		seller.trn = ""
		seller.fb_1month = 0
		seller.fb_3month = 0
		seller.fb_12month = 0
		seller.fb_lifetime = 0

		if err := row.Scan(&seller.primary_id, &seller.seller_id, &seller.seller_name, &seller.keyword); err != nil {
			log.Error(err)
			continue
		}
		seller.url = "https://" + app.Domain + "/sp?ie=UTF8&seller=" + seller.seller_id

		if err := robot.IsAllow(userAgent, seller.url); err != nil {
			log.Errorf("%v", err)
			continue
		}

		for err := seller.request(); err != nil; {
			log.Error(err)
			sleep(120)
		}

		seller.trnCheck()
		seller.addressCheck()
		seller.nameCheck()
		if err := seller.update(); err != nil {
			log.Error(err)
			continue
		}
		if err := seller.syncToAmazonShop(); err != nil {
			log.Error(err)
		}
	}

	log.Infof("3. 结束获取关键词相关卖家信息: %s", keyword)
	log.Infof("------------------------")
	return nil
}

// formatKeyword 格式化关键词（用于 URL）
func formatKeyword(keyword string) string {
	return strings.ReplaceAll(strings.ReplaceAll(keyword, " ", "+"), "'", "%27")
}

// ============================================================
// 以下为 HTTP 模式内存传递优化相关函数
// ============================================================

// crawlSearchInMemory 搜索商品并返回内存列表（不写数据库）
func crawlSearchInMemory(keyword string, categoryID int64) ([]*ProductInfo, error) {
	log.Infof("------------------------")
	log.Infof("1. 开始搜索关键词: %s (内存模式)", keyword)

	formattedKeyword := formatKeyword(keyword)

	// 构建搜索URL
	searchURL := fmt.Sprintf("https://%s/s?k=%s&page=1&dc", app.Domain, formattedKeyword)

	if err := robot.IsAllow(userAgent, searchURL); err != nil {
		log.Errorf("robots.txt 不允许: %v", err)
		return nil, err
	}

	// 发送请求
	client := get_client()
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("User-Agent", userAgent)
	if _, err := app.get_cookie(); err == nil {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 处理响应状态码
	switch resp.StatusCode {
	case 200:
		// OK
	case 404:
		return nil, ERROR_NOT_404
	case 503:
		return nil, ERROR_NOT_503
	default:
		return nil, fmt.Errorf("状态码:%d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检测验证页面
	title := doc.Find("title").Text()
	if strings.Contains(title, "Enter the characters") ||
		strings.Contains(title, "Type the characters") ||
		strings.Contains(title, "Robot check") ||
		doc.Find("form[action*=/captcha/").Length() > 0 {
		return nil, ERROR_VERIFICATION
	}

	// 解析商品列表
	products, err := parseSearchResults(doc, keyword)
	if err != nil {
		return nil, err
	}

	// 插入搜索统计记录（保持统计功能）
	var s searchStruct
	s.en_key = formattedKeyword
	s.zh_key = keyword
	s.category_id = categoryID
	s.valid = len(products)
	if insertID, err := s.searchStartForAPI(); err == nil {
		s.search_end(insertID)
	}

	log.Infof("搜索完成，找到 %d 个商品", len(products))
	log.Infof("------------------------")
	return products, nil
}

// parseSearchResults 解析搜索结果页面，返回商品列表（内存去重）
func parseSearchResults(doc *goquery.Document, keyword string) ([]*ProductInfo, error) {
	res := doc.Find("div[class~=s-search-results]").First()
	if res.Length() == 0 {
		return nil, fmt.Errorf("错误的页面结构")
	}

	dataIndex := res.Find("div[data-index]")
	if dataIndex.Length() == 0 {
		return nil, fmt.Errorf("没有找到商品项")
	}

	// 使用 map 进行 ASIN 去重
	productMap := make(map[string]*ProductInfo)

	// ASIN 计数器
	withBoughtCount := 0
	noBoughtCount := 0
	const maxTotalASIN = 10

	dataIndex.Each(func(i int, g *goquery.Selection) {
		link, exist := g.Find("a").First().Attr("href")
		if !exist {
			return
		}

		link, _ = url.QueryUnescape(link)

		// 跳过无效链接
		if strings.HasPrefix(link, "/gp/") ||
			strings.Contains(link, "javascript:void(0)") ||
			strings.HasPrefix(link, "https://aax-") {
			return
		}

		// 提取 ASIN
		var asin string
		if strings.Contains(link, "/dp/") {
			asin = strings.Split(link, "/dp/")[1]
			// 提取纯 ASIN（去掉后面的参数）
			if idx := strings.Index(asin, "/"); idx > 0 {
				asin = asin[:idx]
			}
		}

		// ASIN 去重
		if asin == "" || productMap[asin] != nil {
			return
		}

		// 提取标题
		title := ""
		titleElement := g.Find("h2").First()
		if titleElement.Length() > 0 {
			title = strings.TrimSpace(titleElement.Text())
		}

		// 提取购买数量
		boughtCount := ""
		boughtSpan := g.Find("span.a-size-base.a-color-secondary").FilterFunction(func(i int, s *goquery.Selection) bool {
			text := strings.TrimSpace(s.Text())
			return strings.Contains(text, "bought in past month")
		})
		if boughtSpan.Length() > 0 {
			boughtText := strings.TrimSpace(boughtSpan.First().Text())
			parts := strings.Split(boughtText, "+")
			if len(parts) > 0 {
				boughtCount = strings.TrimSpace(parts[0])
			}
			withBoughtCount++
		} else {
			// 没有销量标签，检查是否需要补足
			if withBoughtCount >= maxTotalASIN {
				return
			}
			if withBoughtCount+noBoughtCount >= maxTotalASIN {
				return
			}
			noBoughtCount++
		}

		// 提取价格
		price := ""
		priceSpan := g.Find("span.a-price[data-a-size=xl]").First()
		if priceSpan.Length() > 0 {
			priceWhole := priceSpan.Find("span.a-price-whole").First().Text()
			priceFraction := priceSpan.Find("span.a-price-fraction").First().Text()
			if priceWhole != "" {
				price = priceWhole
				if priceFraction != "" {
					price = price + "." + priceFraction
				}
			}
		}

		// 提取评分
		rating := ""
		ratingSpan := g.Find("span.a-size-small.a-color-base[aria-hidden=true]").First()
		if ratingSpan.Length() > 0 {
			rating = strings.TrimSpace(ratingSpan.Text())
		}

		// 提取评论数
		reviewCount := ""
		reviewSpan := g.Find("span.a-size-mini.puis-normal-weight-text.s-underline-text[aria-hidden=true]").First()
		if reviewSpan.Length() > 0 {
			reviewText := strings.TrimSpace(reviewSpan.Text())
			reviewText = strings.Trim(reviewText, "()")
			reviewCount = reviewText
		}

		// 分离 URL 和参数
		var url, param string
		if !strings.Contains(link, "/ref=") {
			return
		}
		urlParts := strings.Split(link, "/ref=")
		url = urlParts[0]
		param = "/ref=" + urlParts[1]

		// 添加到结果集
		productMap[asin] = &ProductInfo{
			URL:         url,
			Param:       param,
			Title:       title,
			ASIN:        asin,
			Keyword:     keyword,
			BoughtCount: boughtCount,
			Price:       price,
			Rating:      rating,
			ReviewCount: reviewCount,
		}
	})

	// 转换为切片
	result := make([]*ProductInfo, 0, len(productMap))
	for _, p := range productMap {
		result = append(result, p)
	}

	return result, nil
}

// crawlProductsFromMemory 从商品列表中提取卖家信息（内存去重）
func crawlProductsFromMemory(products []*ProductInfo, keyword string) (map[string]*SellerInfo, error) {
	log.Infof("------------------------")
	log.Infof("2. 开始处理 %d 个商品，提取卖家信息 (内存模式)", len(products))

	// 使用 map 进行 seller_id 去重
	sellerMap := make(map[string]*SellerInfo)

	for _, p := range products {
		// 构建完整URL
		fullURL := "https://" + app.Domain + p.URL + p.Param

		if err := robot.IsAllow(userAgent, fullURL); err != nil {
			log.Errorf("robots.txt 不允许: %v", err)
			continue
		}

		log.Infof("处理商品 ASIN:%s URL:%s", p.ASIN, fullURL)

		// 请求商品页获取卖家信息
		sellerID, sellerName, brandName, err := fetchSellerInfoFromProduct(fullURL)
		if err != nil {
			if err == ERROR_VERIFICATION {
				log.Errorf("Cookie 验证失败，尝试获取新 Cookie: %v", err)
				if handleErr := app.handleCookieInvalid(); handleErr != nil {
					log.Errorf("获取新 Cookie 失败: %v", handleErr)
				}
				// 重试一次
				sellerID, sellerName, brandName, err = fetchSellerInfoFromProduct(fullURL)
				if err != nil {
					log.Errorf("重试后仍然失败: %v", err)
					continue
				}
			} else if err == ERROR_NOT_SELLER_URL {
				log.Infof("商品没有卖家链接: %s", p.ASIN)
				continue
			} else {
				log.Errorf("获取卖家信息失败: %v", err)
				continue
			}
		}

		// 如果没有 seller_id 且没有品牌名，跳过
		if sellerID == "" && brandName == "" {
			log.Infof("商品没有卖家ID和品牌名: %s", p.ASIN)
			continue
		}

		// 如果品牌名与关键词相同且存在卖家ID，记录卖家
		if strings.ToLower(keyword) == strings.ToLower(brandName) && sellerID != "" {
			if existing, found := sellerMap[sellerID]; found {
				// 卖家已存在，更新信息
				if existing.SellerName == "" && sellerName != "" {
					existing.SellerName = sellerName
				}
			} else {
				// 新卖家
				sellerMap[sellerID] = &SellerInfo{
					SellerID:   sellerID,
					SellerName: sellerName,
					Keyword:    keyword,
				}
				log.Infof("发现新卖家: ID=%s, Name=%s", sellerID, sellerName)
			}
		}
	}

	log.Infof("处理完成，共发现 %d 个独立卖家", len(sellerMap))
	log.Infof("------------------------")
	return sellerMap, nil
}

// fetchSellerInfoFromProduct 从商品页面提取卖家信息
func fetchSellerInfoFromProduct(productURL string) (sellerID, sellerName, brandName string, err error) {
	client := get_client()
	req, err := http.NewRequest("GET", productURL, nil)
	if err != nil {
		return "", "", "", err
	}

	// 设置请求头
	req.Header.Set("Authority", app.Domain)
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("User-Agent", userAgent)
	if _, err := app.get_cookie(); err == nil {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", "", fmt.Errorf("状态码:%d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	// 检测验证页面
	if doc.Find("h4").First().Text() == "Enter the characters you see below" {
		return "", "", "", ERROR_VERIFICATION
	}

	// 提取卖家链接
	sellerLink := doc.Find("a[id=sellerProfileTriggerId]").First()
	href, exist := sellerLink.Attr("href")
	if !exist {
		return "", "", "", ERROR_NOT_SELLER_URL
	}

	// 提取卖家名称
	sellerName = strings.TrimSpace(sellerLink.Text())

	// 从 href 中提取 seller_id
	for _, part := range strings.Split(href, "&") {
		if strings.HasPrefix(part, "seller=") {
			sellerID = strings.Split(part, "seller=")[1]
			break
		}
	}

	// 提取品牌名
	bylineInfo := doc.Find("a[id=bylineInfo]").First()
	if bylineInfo.Length() > 0 {
		brandText := strings.TrimSpace(bylineInfo.Text())
		if strings.Contains(brandText, "Brand:") {
			brandName = strings.TrimSpace(strings.ReplaceAll(brandText, "Brand:", ""))
		} else if strings.Contains(brandText, "Visit the") && strings.Contains(brandText, "Store") {
			parts := strings.Split(brandText, "Visit the")
			if len(parts) > 1 {
				brandPart := strings.Split(parts[1], "Store")
				if len(brandPart) > 0 {
					brandName = strings.TrimSpace(brandPart[0])
				}
			}
		} else {
			brandName = brandText
		}
		brandName = strings.ToLower(brandName)
	}

	return sellerID, sellerName, brandName, nil
}

// fetchSellerDetails 获取卖家详情信息
func fetchSellerDetails(sellerMap map[string]*SellerInfo) ([]*SellerDetail, error) {
	log.Infof("------------------------")
	log.Infof("3. 开始获取 %d 个卖家的详情信息 (内存模式)", len(sellerMap))

	details := make([]*SellerDetail, 0, len(sellerMap))

	for sellerID, info := range sellerMap {
		sellerURL := fmt.Sprintf("https://%s/sp?ie=UTF8&seller=%s", app.Domain, sellerID)

		if err := robot.IsAllow(userAgent, sellerURL); err != nil {
			log.Errorf("robots.txt 不允许: %v", err)
			continue
		}

		log.Infof("获取卖家详情 ID:%s URL:%s", sellerID, sellerURL)

		detail, err := fetchSellerDetailFromPage(sellerURL, info)
		if err != nil {
			if err == ERROR_VERIFICATION {
				log.Errorf("Cookie 验证失败，尝试获取新 Cookie: %v", err)
				if handleErr := app.handleCookieInvalid(); handleErr != nil {
					log.Errorf("获取新 Cookie 失败: %v", handleErr)
				}
				// 重试一次
				detail, err = fetchSellerDetailFromPage(sellerURL, info)
				if err != nil {
					log.Errorf("重试后仍然失败: %v", err)
					continue
				}
			} else if err == ERROR_NOT_404 || err == ERROR_NOT_503 {
				log.Errorf("请求失败: %v", err)
				sleep(120)
				// 重试
				detail, err = fetchSellerDetailFromPage(sellerURL, info)
				if err != nil {
					log.Errorf("重试后仍然失败: %v", err)
					continue
				}
			} else {
				log.Errorf("获取卖家详情失败: %v", err)
				continue
			}
		}

		details = append(details, detail)
		log.Infof("卖家详情获取成功: ID=%s, Name=%s, Address=%s, TRN=%s",
			detail.SellerID, detail.Name, detail.Address, detail.TRN)
	}

	log.Infof("获取完成，共 %d 个卖家详情", len(details))
	log.Infof("------------------------")
	return details, nil
}

// fetchSellerDetailFromPage 从卖家页面提取详情
func fetchSellerDetailFromPage(sellerURL string, info *SellerInfo) (*SellerDetail, error) {
	client := get_client()
	req, err := http.NewRequest("GET", sellerURL, nil)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Authority", app.Domain)
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("User-Agent", userAgent)
	if _, err := app.get_cookie(); err == nil {
		req.Header.Set("Cookie", app.cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		// OK
	case 404:
		return nil, ERROR_NOT_404
	case 503:
		return nil, ERROR_NOT_503
	default:
		return nil, fmt.Errorf("状态码:%d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	detail := &SellerDetail{
		SellerID:   info.SellerID,
		SellerName: info.SellerName,
		Keyword:    info.Keyword,
		TRNStatus:  0,
		AllStatus:  0,
	}

	// 提取商家信息文本
	sellerTxt := doc.Find("div#page-section-detail-seller-info").Find("span").Text()

	// 解析商家信息
	var infoList []string
	for _, line := range strings.Split(sellerTxt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "Business Name:") || strings.Contains(line, "Business Type:") ||
			strings.Contains(line, "Address:") || strings.Contains(line, "VAT Number:") {
			infoList = append(infoList, line)
		} else {
			// 继续上一个字段
			if len(infoList) > 0 {
				infoList[len(infoList)-1] += " " + line
			}
		}
	}

	// 解析各个字段
	for _, item := range infoList {
		if strings.Contains(item, "Business Name:") {
			detail.Name = strings.TrimSpace(strings.ReplaceAll(item, "Business Name:", ""))
		} else if strings.Contains(item, "Address:") {
			detail.Address = strings.TrimSpace(strings.ReplaceAll(item, "Address:", ""))
		} else if strings.Contains(item, "VAT Number:") {
			detail.TRN = strings.TrimSpace(strings.ReplaceAll(item, "VAT Number:", ""))
		}
	}

	// 提取反馈数
	fbSummary := doc.Find("div#seller-feedback-summary-rating").First()
	if fbSummary.Length() > 0 {
		// 1个月
		thirtyElement := fbSummary.Find("div#rating-thirty").First()
		if thirtyElement.Length() > 0 {
			countText := strings.ReplaceAll(thirtyElement.Find("span.ratings-reviews-count").First().Text(), ",", "")
			fmt.Sscanf(countText, "%d", &detail.FB1Month)
		}

		// 3个月
		ninetyElement := fbSummary.Find("div#rating-ninety").First()
		if ninetyElement.Length() > 0 {
			countText := strings.ReplaceAll(ninetyElement.Find("span.ratings-reviews-count").First().Text(), ",", "")
			fmt.Sscanf(countText, "%d", &detail.FB3Month)
		}

		// 12个月
		yearElement := fbSummary.Find("div#rating-year").First()
		if yearElement.Length() > 0 {
			countText := strings.ReplaceAll(yearElement.Find("span.ratings-reviews-count").First().Text(), ",", "")
			fmt.Sscanf(countText, "%d", &detail.FB12Month)
		}

		// 总数
		lifetimeElement := fbSummary.Find("div#rating-lifetime").First()
		if lifetimeElement.Length() > 0 {
			countText := strings.ReplaceAll(lifetimeElement.Find("span.ratings-reviews-count").First().Text(), ",", "")
			fmt.Sscanf(countText, "%d", &detail.FBLifetime)
		}
	}

	// 检查 TRN 状态
	detail.TRNStatus = checkTRNStatus(detail.TRN)

	// 检查信息完整性
	detail.AllStatus = checkSellerInfoComplete(detail.Name, detail.Address, detail.TRN)

	return detail, nil
}

// checkTRNStatus 检查 TRN 状态
func checkTRNStatus(trn string) int {
	if trn == "" {
		return 2 // 空 TRN
	}
	// 检查是否是中国 TRN (18位，9开头)
	if len(trn) == 18 && strings.HasPrefix(trn, "9") {
		return 1 // 中国 TRN
	}
	// 其他 TRN
	return 3 // 其他 TRN
}

// checkSellerInfoComplete 检查卖家信息完整性
func checkSellerInfoComplete(name, address, trn string) int {
	if name == "" {
		return 2 // 没有名称
	}
	if address == "" {
		return 3 // 没有地址
	}
	if trn == "" {
		return 4 // 没有 TRN
	}
	return 1 // 信息完整
}

// batchSaveAll 批量保存所有数据到数据库（事务）
func batchSaveAll(keyword string, categoryID int64, products []*ProductInfo, sellerDetails []*SellerDetail) error {
	log.Infof("------------------------")
	log.Infof("4. 开始批量保存数据到数据库 (事务模式)")

	tx, err := app.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 1. 批量插入商品
	productCount, err := batchInsertProducts(tx, products)
	if err != nil {
		return fmt.Errorf("批量插入商品失败: %w", err)
	}
	log.Infof("商品插入完成: %d 条", productCount)

	// 2. 批量插入/更新卖家
	sellerCount, err := batchUpsertSellers(tx, sellerDetails)
	if err != nil {
		return fmt.Errorf("批量插入卖家失败: %w", err)
	}
	log.Infof("卖家更新完成: %d 条", sellerCount)

	// 3. 批量同步到 tb_amazon_shop
	shopCount, err := batchSyncToAmazonShop(tx, sellerDetails)
	if err != nil {
		return fmt.Errorf("批量同步到 tb_amazon_shop 失败: %w", err)
	}
	log.Infof("tb_amazon_shop 同步完成: %d 条", shopCount)

	// 4. 更新任务状态
	_, err = tx.Exec("UPDATE amc_category SET task_status = ?, updated_at = NOW() WHERE id = ?",
		TASK_STATUS_COMPLETED, categoryID)
	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Infof("批量保存完成: 商品=%d, 卖家=%d, 店铺=%d", productCount, sellerCount, shopCount)
	log.Infof("------------------------")
	return nil
}

// batchInsertProducts 批量插入商品
func batchInsertProducts(tx *sql.Tx, products []*ProductInfo) (int, error) {
	if len(products) == 0 {
		return 0, nil
	}

	// 构建批量插入 SQL
	sql := `INSERT IGNORE INTO amc_product (url, param, title, asin, keyword, bought_count, price, rating, review_count, status, app) VALUES `
	values := make([]string, 0, len(products))
	args := make([]interface{}, 0, len(products)*11)

	for _, p := range products {
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, p.URL, p.Param, p.Title, p.ASIN, p.Keyword,
			p.BoughtCount, p.Price, p.Rating, p.ReviewCount,
			MYSQL_PRODUCT_STATUS_OVER, app.Basic.App_id)
	}

	sql += strings.Join(values, ", ")

	result, err := tx.Exec(sql, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// batchUpsertSellers 批量插入或更新卖家
func batchUpsertSellers(tx *sql.Tx, details []*SellerDetail) (int, error) {
	if len(details) == 0 {
		return 0, nil
	}

	count := 0
	for _, d := range details {
		// 先尝试插入
		_, err := tx.Exec(
			`INSERT INTO amc_seller (seller_id, seller_name, keyword, name, address, trn, trn_status, all_status, app_id, fb_1month, fb_3month, fb_12month, fb_lifetime)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.SellerID, d.SellerName, d.Keyword, d.Name, d.Address, d.TRN, d.TRNStatus, d.AllStatus, app.Basic.App_id,
			d.FB1Month, d.FB3Month, d.FB12Month, d.FBLifetime,
		)

		if err != nil && !is_duplicate_entry(err) {
			// 如果是重复错误，尝试更新
			_, err = tx.Exec(
				`UPDATE amc_seller SET
					seller_name = ?, name = ?, address = ?, trn = ?, trn_status = ?, all_status = ?,
					app_id = ?, fb_1month = ?, fb_3month = ?, fb_12month = ?, fb_lifetime = ?
				WHERE seller_id = ?`,
				d.SellerName, d.Name, d.Address, d.TRN, d.TRNStatus, d.AllStatus, app.Basic.App_id,
				d.FB1Month, d.FB3Month, d.FB12Month, d.FBLifetime,
				d.SellerID,
			)
			if err != nil {
				return 0, err
			}
		}
		count++
	}

	return count, nil
}

// batchSyncToAmazonShop 批量同步到 tb_amazon_shop
func batchSyncToAmazonShop(tx *sql.Tx, details []*SellerDetail) (int, error) {
	if len(details) == 0 {
		return 0, nil
	}

	count := 0
	for _, d := range details {
		shopURL := fmt.Sprintf("https://%s/sp?ie=UTF8&seller=%s", app.Domain, d.SellerID)
		domain := strings.ToLower(d.Keyword)

		// 检查是否存在
		var existingID int
		err := tx.QueryRow("SELECT id FROM tb_amazon_shop WHERE domain = ? AND shop_id = ?", domain, d.SellerID).Scan(&existingID)

		if err == sql.ErrNoRows {
			// 插入新记录
			_, err = tx.Exec(`
				INSERT INTO tb_amazon_shop
					(user_id, domain, shop_id, shop_name, shop_url, marketplace,
					 company_name, company_address, fb_1month, fb_3month, fb_12month, fb_lifetime,
					 main_products, avg_price, estimated_monthly_sales, crawl_time, create_time, update_time)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', 0, 0, NOW(), NOW(), NOW())
			`, 1, domain, d.SellerID, d.SellerName, shopURL, "US",
				d.Name, d.Address, d.FB1Month, d.FB3Month, d.FB12Month, d.FBLifetime)
			if err != nil {
				return 0, err
			}
		} else if err != nil {
			return 0, err
		} else {
			// 更新现有记录
			_, err = tx.Exec(`
				UPDATE tb_amazon_shop SET
					shop_name = ?, shop_url = ?, company_name = ?, company_address = ?,
					fb_1month = ?, fb_3month = ?, fb_12month = ?, fb_lifetime = ?,
					crawl_time = NOW(), update_time = NOW()
				WHERE id = ?
			`, d.SellerName, shopURL, d.Name, d.Address,
				d.FB1Month, d.FB3Month, d.FB12Month, d.FBLifetime, existingID)
			if err != nil {
				return 0, err
			}
		}
		count++
	}

	return count, nil
}
