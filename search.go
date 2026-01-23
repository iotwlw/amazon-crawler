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

const MYSQL_SEARCH_STATUS_START int64 = 0
const MYSQL_SEARCH_STATUS_OVER int64 = 1

type searchStruct struct {
	zh_key        string
	en_key        string
	category_id   int64
	url           string
	start         int
	end           int
	html          string
	valid         int
	product_url   string
	product_param string
}

func (s *searchStruct) main() error {
	if !app.Exec.Enable.Search {
		log.Warn("跳过 搜索")
		return nil
	}
	if app.Exec.Loop.Search == app.Exec.Loop.search_time {
		log.Warn("已经达到执行次数 搜索")
		return nil
	}

	log.Infof("------------------------")
	log.Infof("1. 开始搜索关键词")

	if app.Exec.Loop.Search == 0 {
		log.Info("循环次数无限")
	} else {
		log.Infof("循环次数剩余:%d", app.Exec.Loop.Search-app.Exec.Loop.search_time)
	}
	app.Exec.Loop.search_time++

	app.update(MYSQL_APPLICATION_STATUS_SEARCH)

	row, err := s.get_category()
	if err != nil {
		log.Error(err)
		log.Infof("------------------------")
	}
	s.start = 1
	s.end = 2 // 为了适应亚马逊系统，这里只搜索首页，
	for row.Next() {
		s.valid = 0
		row.Scan(&s.category_id, &s.zh_key, &s.en_key)
		s.en_key = s.set_en_key()
		insert_id, err := s.search_start()
		if err != nil {
			log.Errorf("插入失败 关键词:%s %v", s.zh_key, err)
			continue
		}
		for ; s.start < s.end; s.start++ {
			h, err := s.request(s.start)
			switch err {
			case nil:
				break
			case ERROR_NOT_404:
			case ERROR_NOT_503:
				s.start--
				log.Warn("遇到503错误，尝试获取新的Cookie")
				if handleErr := app.handleCookieInvalid(); handleErr != nil {
					log.Errorf("获取新Cookie失败: %v，等待120秒后重试", handleErr)
					sleep(120)
				}
				continue

			default:
				log.Error(err)
				continue
			}
			s.get_product_url(h)
		}
		err = s.search_end(insert_id)
		if err != nil {
			log.Errorf("更新结果失败 关键词:%s %v", s.zh_key, err)
			continue
		}
		s.start = 1
	}
	log.Infof("------------------------")
	return nil
}
func (s *searchStruct) get_category() (*sql.Rows, error) {
	switch app.Exec.Search_priority {
	case 1:
		log.Infof("搜索优先级优先")
		return app.db.Query(`select id,zh_key,en_key from amc_category order by priority DESC`)
	case 2:
		log.Infof("搜索次数少优先")
		return app.db.Query(`SELECT c.id, c.zh_key, c.en_key  FROM amc_category c LEFT JOIN amc_search_statistics s ON s.category_id = c.id GROUP BY c.id ORDER BY COUNT(s.category_id),id`)
	}
	log.Infof("错误的输入，按搜索优先级优先")
	return app.db.Query(`select id,zh_key,en_key from amc_category order by priority DESC `)
}
func (s *searchStruct) search_start() (int64, error) {
	r, err := app.db.Exec("insert into amc_search_statistics(category_id,app) values(?,?)", s.category_id, app.Basic.App_id)
	if err != nil {
		return 0, err
	}

	id, err := r.LastInsertId()
	if err != nil {
		return 0, err
	}
	log.Infof("开始搜索 关键词:%s 关键词ID:%d 状态:%d(开始)", s.zh_key, s.category_id, MYSQL_SEARCH_STATUS_START)
	return id, nil
}
func (s *searchStruct) search_end(insert_id int64) error {
	_, err := app.db.Exec("update amc_search_statistics set status=?,end=CURRENT_TIMESTAMP,valid=? where id=?", MYSQL_SEARCH_STATUS_OVER, s.valid, insert_id)
	if err != nil {
		return err
	}
	log.Infof("搜索完成 关键词:%s 完成ID:%d 有效数:%d", s.zh_key, insert_id, s.valid)
	return nil
}
func (s *searchStruct) set_en_key() string {
	return strings.ReplaceAll(strings.ReplaceAll(s.en_key, " ", "+"), "'", "%27")
}
func (s *searchStruct) request(seq int) (*goquery.Document, error) {
	url := fmt.Sprintf("https://%s/s?k=%s&page=%d&dc&crid=2V9436DZJ6IJF&qid=1699839233&sprefix=clothe%%2Caps%%2C552&ref=sr_pg_2", app.Domain, s.en_key, seq)
	// 链接增加 &dc 表示直接搜索，避免转移到其他关键词
	err := robot.IsAllow(userAgent, url)
	if err != nil {
		return nil, err
	}
	log.Infof("开始搜索 关键词:%s 页面:%d url:%s", s.zh_key, seq, url)

	client := get_client()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Language", `zh-CN,zh;q=0.9`)
	req.Header.Set("cache-control", `max-age=0`)
	req.Header.Set("device-memory", `8`)
	req.Header.Set("device-memory", `8`)
	req.Header.Set("downlink", `1.55'`)
	req.Header.Set("dpr", `2`)
	req.Header.Set("ect", `3g`)
	req.Header.Set("pragma", `400`)
	if _, err := app.get_cookie(); err != nil {
		log.Error(err)
	} else {
		req.Header.Set("Cookie", app.cookie)
	}
	req.Header.Set("upgrade-insecure-requests", `1`)
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
		return nil, err

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
		return nil, fmt.Errorf("状态码:%d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("内部错误:%v", err)
	}

	// 检测验证页面（亚马逊会要求人机验证）
	title := doc.Find("title").Text()
	if strings.Contains(title, "Enter the characters you see below") ||
		strings.Contains(title, "Type the characters you see in this image") ||
		strings.Contains(title, "Robot check") ||
		doc.Find("form[action*=/captcha/").Length() > 0 ||
		doc.Find("[method=post]").Find("input[type=text][name*=field-keywords]").Length() > 0 {
		log.Warn("检测到验证页面，Cookie 可能已失效")
		return nil, ERROR_VERIFICATION
	}

	return doc, nil
}

func (s *searchStruct) get_product_url(doc *goquery.Document) {

	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	res := doc.Find("div[class~=s-search-results]").First()

	if res.Length() == 0 {
		log.Errorf("错误的页面结构 关键词:%s", s.zh_key)
		return
	}
	// len res.Find("div[data-index]")
	data_index := res.Find("div[data-index]")
	if data_index.Length() == 0 {
		log.Errorf("没有找到商品项 关键词:%s", s.zh_key)
		return
	}
	log.Infof("找到商品项数:%d 关键词:%s", data_index.Length(), s.zh_key)

	// ASIN 计数器
	withBoughtCount := 0    // 有销量标签的数量
	noBoughtCount := 0      // 无销量标签的数量
	const maxTotalASIN = 10 // 最少保证 10 个 ASIN

	data_index.Each(func(i int, g *goquery.Selection) {

		link, exist := g.Find("a").First().Attr("href")

		if exist {
			link, _ = url.QueryUnescape(link)

			title := ""
			titleElement := g.Find("h2").First()
			if titleElement.Length() > 0 {
				title = strings.TrimSpace(titleElement.Text())
			}

			boughtCount := ""
			boughtSpan := g.Find("span.a-size-base.a-color-secondary").FilterFunction(func(i int, s *goquery.Selection) bool {
				text := strings.TrimSpace(s.Text())
				return strings.Contains(text, "bought in past month")
			})
			if boughtSpan.Length() > 0 {
				// 有销量标签，全部加入
				boughtText := strings.TrimSpace(boughtSpan.First().Text())
				parts := strings.Split(boughtText, "+")
				if len(parts) > 0 {
					boughtCount = strings.TrimSpace(parts[0])
				}
				withBoughtCount++
			} else {
				// 没有销量标签，只有当有销量标签的数量不足 10 个时才补足
				if withBoughtCount >= maxTotalASIN {
					return // 有销量的已经够了，不需要补足
				}
				if withBoughtCount+noBoughtCount >= maxTotalASIN {
					return // 总数已经达到 10 个，不再补足
				}
				noBoughtCount++
			}

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

			rating := ""
			ratingSpan := g.Find("span.a-size-small.a-color-base[aria-hidden=true]").First()
			if ratingSpan.Length() > 0 {
				rating = strings.TrimSpace(ratingSpan.Text())
			}

			reviewCount := ""
			reviewSpan := g.Find("span.a-size-mini.puis-normal-weight-text.s-underline-text[aria-hidden=true]").First()
			if reviewSpan.Length() > 0 {
				reviewText := strings.TrimSpace(reviewSpan.Text())
				reviewText = strings.Trim(reviewText, "()")
				reviewCount = reviewText
			}

			if err := robot.IsAllow(userAgent, link); err != nil {
				log.Errorf("此链接不允许访问 关键词:%s %v", s.zh_key, err)
				return
			}

			if strings.HasPrefix(link, "/gp/") || strings.Contains(link, `javascript:void(0)`) {
				link = fmt.Sprintf("https://%s%s", app.Domain, link)
				// log.Warnf("非预设的链接跳过此链接 关键词:%s 捕获链接:%s", s.zh_key, link)
			} else if strings.HasPrefix(link, "https://aax-") {
				// log.Warnf("非预设的链接跳过此链接 关键词:%s 捕获链接:%s", s.zh_key, link)
				return
			}
			if strings.Contains(link, `/dp/`) {
				link = "/dp/" + strings.Split(link, "/dp/")[1]
			}
			s.deal_prouct_url(link, title, boughtCount, price, rating, reviewCount)

		} else {
			if i != 0 {
				log.Warnf("此商品项中未找到链接 关键词:%s 序号:%d", s.zh_key, i)
			}
		}

	})
}
func (s *searchStruct) deal_prouct_url(link string, title string, boughtCount string, price string, rating string, reviewCount string) {
	if !strings.Contains(link, "/ref=") || strings.HasPrefix(link, "https://") {
		log.Errorf("非预设的链接跳过此链接:%s", link)
		return
	}
	url := strings.Split(link, "/ref=")

	asin := ""
	if strings.Contains(url[0], "/dp/") {
		asin = strings.Split(url[0], "/dp/")[1]
	}
	_, err := app.db.Exec(`INSERT INTO amc_product(url,param,title,asin,keyword,bought_count,price,rating,review_count) values(?,?,?,?,?,?,?,?,?)`, url[0], "/ref="+url[1], title, asin, s.en_key, boughtCount, price, rating, reviewCount)

	link = fmt.Sprintf("https://%s%s", app.Domain, link)
	if is_duplicate_entry(err) {
		log.Infof("商品已存在 关键词:%s 链接:%s ", s.en_key, link)
		return
	}
	if err != nil {
		log.Errorf("商品插入失败 关键词:%s 链接:%s %v ", s.en_key, link, err)
		return
	}

	log.Infof("商品插入成功 关键词:%s 链接:%s 标题:%s ASIN:%s 购买数量:%s 价格:%s 星级:%s 评分数量:%s", s.en_key, link, title, asin, boughtCount, price, rating, reviewCount)
	s.valid += 1
}

// searchStartForAPI 为 API 模式插入搜索统计记录
// category_id 设为 0 表示来自 API 调用
func (s *searchStruct) searchStartForAPI() (int64, error) {
	r, err := app.db.Exec("INSERT INTO amc_search_statistics(category_id, app) VALUES(0, ?)", app.Basic.App_id)
	if err != nil {
		return 0, err
	}

	id, err := r.LastInsertId()
	if err != nil {
		return 0, err
	}
	log.Infof("开始搜索(API模式) 关键词:%s 状态:%d(开始)", s.zh_key, MYSQL_SEARCH_STATUS_START)
	return id, nil
}
