package main

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// StartHTTPServer 启动 HTTP 服务
func StartHTTPServer(addr string) {
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/api/crawl", handleCrawl)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/health", handleHealth)

	log.Infof("HTTP 服务启动在 %s", addr)
	log.Infof("可用接口:")
	log.Infof("  POST /api/crawl  - 提交爬取任务")
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
