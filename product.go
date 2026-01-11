package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	log "github.com/tengfei-xy/go-log"
)

type productStruct struct {
	url string

	id string

	brand_name string

	brand_store_url string

	keyword string

	seller_name string
}

const MYSQL_PRODUCT_STATUS_INSERT int = 0
const MYSQL_PRODUCT_STATUS_CHEKCK int = 1
const MYSQL_PRODUCT_STATUS_OVER int = 2
const MYSQL_PRODUCT_STATUS_ERROR_OVER int = 3
const MYSQL_PRODUCT_STATUS_NO_PRODUCT int = 4
const MYSQL_PRODUCT_STATUS_FROM_SEARCH int = 5 // 从搜索页获取的产品暂时不做查询

func (product *productStruct) main() error {
	if !app.Exec.Enable.Product {
		log.Warn("跳过 产品")
		return nil
	}
	if app.Exec.Loop.Product == app.Exec.Loop.product_time {
		log.Warn("已达到执行次数 产品")
		return nil
	}
	log.Infof("------------------------")
	log.Infof("2. 开始从产品页获取商家ID")
	if app.Exec.Loop.Product == 0 {
		log.Info("循环次数无限")
	} else {
		log.Infof("循环次数剩余:%d", app.Exec.Loop.Product-app.Exec.Loop.product_time)
	}
	app.Exec.Loop.product_time++

	app.update(MYSQL_APPLICATION_STATUS_PRODUCT)

	_, err := app.db.Exec("UPDATE amc_product SET status = ? ,app = ? WHERE (status = ? or status=?) and (app=? or app=?)  LIMIT 1000", MYSQL_PRODUCT_STATUS_CHEKCK, app.Basic.App_id, MYSQL_PRODUCT_STATUS_INSERT, MYSQL_PRODUCT_STATUS_ERROR_OVER, 0, app.Basic.App_id)
	if err != nil {
		log.Errorf("更新product表失败,%v", err)
		return err
	}

	row, err := app.db.Query(`select id,url,param,keyword from amc_product where status=? and app = ?`, MYSQL_PRODUCT_STATUS_CHEKCK, app.Basic.App_id)
	if err != nil {
		log.Errorf("查询product表失败,%v", err)
		return err
	}
	for row.Next() {
		var primary_id int64
		var url, param, keyword string
		if err := row.Scan(&primary_id, &url, &param, &keyword); err != nil {
			log.Errorf("获取product表的值失败,%v", err)
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

		// 立即保存当前商品的 seller_id 和其他信息，避免被下一个循环覆盖
		currentSellerID := product.get_seller_id()
		currentBrandName := product.brand_name
		currentBrandStoreURL := product.brand_store_url
		currentSellerName := product.seller_name
		currentKeyword := keyword

		if currentSellerID == "" && currentBrandName == "" {
			// 如果没有找到 seller_id 和 brand_name，标记为无商家
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
	log.Infof("2. 结束从产品页获取商家ID")
	log.Infof("------------------------")

	return nil
}

func (product *productStruct) request(url string) error {
	client := get_client()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authority", app.Domain)
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("cache-control", `max-age=0`)
	req.Header.Set("device-memory", `8`)
	req.Header.Set("downlink", `1.5'`)
	req.Header.Set("dpr", `2`)
	req.Header.Set("ect", `3g`)
	req.Header.Set("rtt", `350`)
	if _, err := app.get_cookie(); err != nil {
		log.Error(err)
	} else {
		req.Header.Set("Cookie", app.cookie)
	}
	req.Header.Set("upgrade-insecure-requests", `1`)
	req.Header.Set("Referer", fmt.Sprintf("https://%s/?k=Hardware+electricia%%27n&crid=3CR8DCX0B3L5U&sprefix=hardware+electricia%%27n%%2Caps%%2C714&ref=nb_sb_noss", app.Domain))
	req.Header.Set("Sec-Fetch-Dest", `empty`)
	req.Header.Set("Sec-Fetch-Mode", `cors`)
	req.Header.Set("Sec-Fetch-Site", `same-origin`)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("sec-ch-ua", `"Not.A/Brand";v="8", "Chromium";v="114", "Google Chrome";v="114"`)
	req.Header.Set("sec-ch-ua-mobile", `?0`)
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("内部错误:%v", err)
		return err

	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Errorf("状态码:%d", resp.StatusCode)
		return err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("内部错误:%v", err)
	}

	if doc.Find("h4").First().Text() == "Enter the characters you see below" {
		return ERROR_VERIFICATION
	}

	res := doc.Find("a[id=sellerProfileTriggerId]").First()

	url, exist := res.Attr("href")
	if !exist {
		return ERROR_NOT_SELLER_URL
	}

	product.url = url

	sellerName := strings.TrimSpace(res.Text())
	if sellerName != "" {
		product.seller_name = sellerName
		log.Infof("提取到卖家名称:%s", product.seller_name)
	}

	bylineInfo := doc.Find("a[id=bylineInfo]").First()
	if bylineInfo.Length() > 0 {
		brandText := strings.TrimSpace(bylineInfo.Text())
		if strings.Contains(brandText, "Brand:") {
			product.brand_name = strings.TrimSpace(strings.ReplaceAll(brandText, "Brand:", ""))
		} else if strings.Contains(brandText, "Visit the") && strings.Contains(brandText, "Store") {
			storeUrl, exists := bylineInfo.Attr("href")
			if exists {
				product.brand_store_url = storeUrl
				log.Infof("提取到旗舰店链接:%s", product.brand_store_url)
			}
			parts := strings.Split(brandText, "Visit the")
			if len(parts) > 1 {
				brandPart := strings.Split(parts[1], "Store")
				if len(brandPart) > 0 {
					product.brand_name = strings.TrimSpace(brandPart[0])
				}
			}
		} else {
			product.brand_name = brandText
		}
		product.brand_name = strings.ToLower(product.brand_name)
		log.Infof("提取到品牌名称:%s", product.brand_name)
	}

	return nil
}

// get_seller_id 从 URL 中提取 seller ID
func (product *productStruct) get_seller_id() string {
	for _, j := range strings.Split(product.url, "&") {
		if strings.HasPrefix(j, "seller=") {
			return strings.Split(j, "seller=")[1]
		}
	}
	return ""
}

// insert_selll_id 插入卖家信息到数据库
func (product *productStruct) insert_selll_id(sellerID, sellerName, keyword string) error {
	_, err := app.db.Exec("insert into amc_seller (seller_id,seller_name,keyword,app_id) values(?,?,?,?)", sellerID, sellerName, keyword, 0)
	return err
}

func (product *productStruct) update_status(id int64, s int, seller_id string, brand_name string, brand_store_url string) error {
	if seller_id != "" || brand_name != "" || brand_store_url != "" {
		_, err := app.db.Exec("UPDATE amc_product SET status = ?, app = ?, seller_id = ?, brand_name = ?, brand_store_url = ? WHERE id = ?", s, app.Basic.App_id, seller_id, brand_name, brand_store_url, id)
		if err != nil {
			log.Infof("更新product表状态失败 ID:%d app:%d 状态:%d seller_id:%s brand_name:%s brand_store_url:%s", id, app.Basic.App_id, s, seller_id, brand_name, brand_store_url)
			return err
		}
		log.Infof("更新product表状态成功 ID:%d 状态:%d app:%d seller_id:%s brand_name:%s brand_store_url:%s", id, s, app.Basic.App_id, seller_id, brand_name, brand_store_url)
	} else {
		_, err := app.db.Exec("UPDATE amc_product SET status = ?, app = ? WHERE id = ?", s, app.Basic.App_id, id)
		if err != nil {
			log.Infof("更新product表状态失败 ID:%d app:%d 状态:%d", id, app.Basic.App_id, s)
			return err
		}
		log.Infof("更新product表状态成功 ID:%d 状态:%d app:%d", id, s, app.Basic.App_id)
	}
	return nil
}
