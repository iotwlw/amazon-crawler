package main

import (
	"strings"

	log "github.com/tengfei-xy/go-log"
)

// ExecuteCrawl 执行单个关键词的完整爬取流程
// 流程: 搜索商品 -> 提取卖家ID -> 获取卖家详情
func ExecuteCrawl(task CrawlTask) {
	ExecuteCrawlWithStatus(task)
}

// ExecuteCrawlWithStatus 执行单个关键词的完整爬取流程，返回是否成功
func ExecuteCrawlWithStatus(task CrawlTask) bool {
	keyword := task.Keyword
	log.Infof("========================================")
	log.Infof("开始爬取关键词: %s", keyword)
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
			} else if err == ERROR_NOT_404 || err == ERROR_NOT_503 || err == ERROR_VERIFICATION {
				product.update_status(primary_id, MYSQL_PRODUCT_STATUS_ERROR_OVER, "", "", "")
				log.Error(err)
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
