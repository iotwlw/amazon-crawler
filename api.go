package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/tengfei-xy/go-log"
)

// 任务状态常量
const (
	TASK_STATUS_PENDING   = 0 // 待执行
	TASK_STATUS_COMPLETED = 1 // 已执行
	TASK_STATUS_FAILED    = 2 // 失败
)

// APIResponse 统一响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CrawlRequest 爬取请求结构
type CrawlRequest struct {
	Keywords []string `json:"keywords"`
}

// CrawlResponseData 爬取响应数据
type CrawlResponseData struct {
	Total    int `json:"total"`    // 提交的总数
	Inserted int `json:"inserted"` // 新插入的数量
	Skipped  int `json:"skipped"`  // 跳过的数量（已存在）
}

type ASINInspectionOptions struct {
	IncludeOffer  bool `json:"include_offer"`
	IncludeSeller bool `json:"include_seller"`
}

type ASINInspectionRequestItem struct {
	ASIN     string `json:"asin"`
	URL      string `json:"url"`
	Original string `json:"original"`
}

type ASINInspectionRequest struct {
	JobID   string                      `json:"job_id"`
	Domain  string                      `json:"domain"`
	Items   []ASINInspectionRequestItem `json:"items"`
	Options ASINInspectionOptions       `json:"options"`
}

type ASINInspectionResponseData struct {
	JobID string                       `json:"job_id,omitempty"`
	Items []ASINInspectionResponseItem `json:"items"`
}

type ASINInspectionResponseItem struct {
	Input           string `json:"input"`
	URL             string `json:"url"`
	Domain          string `json:"domain"`
	OriginalASIN    string `json:"original_asin"`
	ASIN            string `json:"asin"`
	Status          string `json:"status"`
	ProductTitle    string `json:"product_title"`
	Price           string `json:"price"`
	Coupon          string `json:"coupon"`
	IsDeal          string `json:"is_deal"`
	PrimeExclusive  string `json:"prime_exclusive"`
	DisplayDiscount string `json:"display_discount"`
	Rating          string `json:"rating"`
	ReviewCount     int    `json:"review_count"`
	PromoCheck      string `json:"promo_check"`
	Promotion       string `json:"promotion"`
	PromoCode       string `json:"promo_code"`
	Keep            string `json:"keep"`
	ChoiceBadge     string `json:"choice_badge"`
	FrequentReturn  string `json:"frequent_return"`
	NewerModel      string `json:"newer_model"`
	ErrorMessage    string `json:"error_message"`
	CapturedAt      string `json:"captured_at"`
}

// StartHTTPServer 启动 HTTP 服务
func StartHTTPServer(addr string) {
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/api/crawl", handleCrawl)
	mux.HandleFunc("/api/asin-inspection", handleASINInspection)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/health", handleHealth)

	log.Infof("HTTP 服务启动在 %s", addr)
	log.Infof("可用接口:")
	log.Infof("  POST /api/crawl  - 提交爬取任务")
	log.Infof("  POST /api/asin-inspection - ASIN/链接实时巡检")
	log.Infof("  GET  /api/status - 查看任务状态")
	log.Infof("  GET  /health     - 健康检查")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Errorf("HTTP 服务启动失败: %v", err)
		panic(err)
	}
}

// handleCrawl 处理爬取请求 - 将关键词写入 amc_category 表
func handleCrawl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    -1,
			Message: "只支持 POST 方法",
		})
		return
	}

	// 解析请求体
	var req CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: fmt.Sprintf("请求解析失败: %v", err),
		})
		return
	}

	// 验证参数
	if len(req.Keywords) == 0 {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "keywords 不能为空",
		})
		return
	}

	// 将关键词写入数据库
	inserted := 0
	skipped := 0
	for _, kw := range req.Keywords {
		err := insertKeywordTask(kw)
		if err != nil {
			if is_duplicate_entry(err) {
				skipped++
				log.Infof("关键词已存在，跳过: %s", kw)
			} else {
				log.Errorf("插入关键词失败: %s, 错误: %v", kw, err)
			}
		} else {
			inserted++
			log.Infof("关键词已入库: %s", kw)
		}
	}

	log.Infof("收到爬取请求，共 %d 个关键词，新增 %d 个，跳过 %d 个", len(req.Keywords), inserted, skipped)

	// 通知 Worker 有新任务（非阻塞）
	select {
	case taskNotify <- struct{}{}:
	default:
	}

	// 返回成功响应
	writeJSON(w, http.StatusOK, APIResponse{
		Code:    0,
		Message: "任务已提交",
		Data: CrawlResponseData{
			Total:    len(req.Keywords),
			Inserted: inserted,
			Skipped:  skipped,
		},
	})
}

func handleASINInspection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !checkCrawlerToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    -1,
			Message: "只支持 POST 方法",
		})
		return
	}

	var req ASINInspectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: fmt.Sprintf("请求解析失败: %v", err),
		})
		return
	}

	items, err := buildASINInspectionItems(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	if _, err := app.get_cookie(); err != nil {
		log.Warnf("获取 Cookie 失败: %v，将不使用 Cookie", err)
	}

	domain := normalizeDomain(req.Domain)
	if domain == "" {
		domain = normalizeDomain(app.Domain)
	}
	inspector := NewLinkInspector("", domain, "")
	responseItems := make([]ASINInspectionResponseItem, 0, len(items))
	for i, item := range items {
		log.Infof("ASIN巡检: %d/%d %s", i+1, len(items), item.Original)
		result := inspector.inspectItem(item)
		responseItems = append(responseItems, linkInspectionResultToAPIItem(result, time.Now().UTC().Format(time.RFC3339)))
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: ASINInspectionResponseData{
			JobID: req.JobID,
			Items: responseItems,
		},
	})
}

func checkCrawlerToken(w http.ResponseWriter, r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("CRAWLER_API_TOKEN"))
	if token == "" {
		return true
	}
	if r.Header.Get("X-Crawler-Token") == token {
		return true
	}
	writeJSON(w, http.StatusUnauthorized, APIResponse{
		Code:    -1,
		Message: "无效的 X-Crawler-Token",
	})
	return false
}

func buildASINInspectionItems(req ASINInspectionRequest) ([]LinkInspectionItem, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("items 不能为空")
	}
	if len(req.Items) > 50 {
		return nil, fmt.Errorf("items 不能超过 50 条")
	}

	defaultDomain := normalizeDomain(req.Domain)
	if defaultDomain == "" {
		defaultDomain = normalizeDomain(app.Domain)
	}
	if defaultDomain == "" {
		defaultDomain = "www.amazon.com"
	}

	items := make([]LinkInspectionItem, 0, len(req.Items))
	seen := make(map[string]struct{})
	for i, rawItem := range req.Items {
		raw := strings.TrimSpace(rawItem.URL)
		if raw == "" {
			raw = strings.TrimSpace(rawItem.ASIN)
		}
		if raw == "" {
			raw = strings.TrimSpace(rawItem.Original)
		}
		if raw == "" {
			return nil, fmt.Errorf("items[%d] 缺少 asin 或 url", i)
		}

		item, ok := parseLinkInspectionItem(raw, defaultDomain)
		if !ok {
			return nil, fmt.Errorf("items[%d] 无法识别 ASIN 或商品链接: %s", i, raw)
		}
		if _, exists := seen[item.URL]; exists {
			continue
		}
		seen[item.URL] = struct{}{}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("items 没有可巡检的 ASIN 或商品链接")
	}
	return items, nil
}

func linkInspectionResultToAPIItem(result LinkInspectionResult, capturedAt string) ASINInspectionResponseItem {
	status := "success"
	if result.ErrorMessage != "" {
		status = "failed"
	}
	return ASINInspectionResponseItem{
		Input:           result.Item.Original,
		URL:             result.Item.URL,
		Domain:          result.Item.Domain,
		OriginalASIN:    result.Item.ASIN,
		ASIN:            result.ASIN,
		Status:          status,
		ProductTitle:    result.Product,
		Price:           result.Price,
		Coupon:          result.Coupon,
		IsDeal:          result.IsDeal,
		PrimeExclusive:  result.PrimeExclusive,
		DisplayDiscount: result.DisplayDiscount,
		Rating:          result.Rating,
		ReviewCount:     result.ReviewCount,
		PromoCheck:      result.PromoCheck,
		Promotion:       result.Promotion,
		PromoCode:       result.PromoCode,
		Keep:            result.Keep,
		ChoiceBadge:     result.Choice,
		FrequentReturn:  result.FrequentReturn,
		NewerModel:      result.NewerModel,
		ErrorMessage:    result.ErrorMessage,
		CapturedAt:      capturedAt,
	}
}

// insertKeywordTask 将关键词插入到 amc_category 表
func insertKeywordTask(keyword string) error {
	// zh_key 和 en_key 都使用同一个关键词
	_, err := app.db.Exec(
		"INSERT INTO amc_category (zh_key, en_key, task_status) VALUES (?, ?, ?)",
		keyword, keyword, TASK_STATUS_PENDING,
	)
	return err
}

// handleStatus 查看任务状态
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{
			Code:    -1,
			Message: "只支持 GET 方法",
		})
		return
	}

	// 查询各状态的任务数量
	var pending, completed, failed int
	app.db.QueryRow("SELECT COUNT(*) FROM amc_category WHERE task_status = ?", TASK_STATUS_PENDING).Scan(&pending)
	app.db.QueryRow("SELECT COUNT(*) FROM amc_category WHERE task_status = ?", TASK_STATUS_COMPLETED).Scan(&completed)
	app.db.QueryRow("SELECT COUNT(*) FROM amc_category WHERE task_status = ?", TASK_STATUS_FAILED).Scan(&failed)

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data: map[string]interface{}{
			"pending":   pending,
			"completed": completed,
			"failed":    failed,
		},
	})
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
	})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
