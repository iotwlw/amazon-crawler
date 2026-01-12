package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	log "github.com/tengfei-xy/go-log"
)

// CookieEntry 单个 Cookie 条目
type CookieEntry struct {
	Zipcode   string    `json:"zipcode"`
	City      string    `json:"city"`
	Cookie    string    `json:"cookie"`
	CreatedAt time.Time `json:"created_at"`
}

// CookieFile Cookie 文件结构
type CookieFile struct {
	Cookies []CookieEntry `json:"cookies"`
}

// 美国邮编池
var USZipCodes = []struct {
	Zipcode string
	City    string
}{
	{"10001", "New York, NY"},
	{"10013", "Manhattan, NY"},
	{"90001", "Los Angeles, CA"},
	{"90210", "Beverly Hills, CA"},
	{"60601", "Chicago, IL"},
	{"60611", "Chicago Downtown, IL"},
	{"77001", "Houston, TX"},
	{"77002", "Houston Downtown, TX"},
	{"85001", "Phoenix, AZ"},
	{"19101", "Philadelphia, PA"},
	{"78201", "San Antonio, TX"},
	{"92101", "San Diego, CA"},
	{"75201", "Dallas, TX"},
	{"95101", "San Jose, CA"},
	{"78701", "Austin, TX"},
	{"32801", "Orlando, FL"},
	{"33101", "Miami, FL"},
	{"98101", "Seattle, WA"},
	{"80201", "Denver, CO"},
	{"02101", "Boston, MA"},
}

// LoadCookiesFromFile 从 cookies.json 文件加载 Cookie
func LoadCookiesFromFile(filePath string) ([]CookieEntry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 Cookie 文件失败: %w", err)
	}

	var cookieFile CookieFile
	if err := json.Unmarshal(data, &cookieFile); err != nil {
		return nil, fmt.Errorf("解析 Cookie 文件失败: %w", err)
	}

	if len(cookieFile.Cookies) == 0 {
		return nil, fmt.Errorf("Cookie 文件为空")
	}

	log.Infof("从文件加载了 %d 个 Cookie", len(cookieFile.Cookies))
	return cookieFile.Cookies, nil
}

// SaveCookiesToFile 保存 Cookie 到文件
func SaveCookiesToFile(filePath string, cookies []CookieEntry) error {
	cookieFile := CookieFile{Cookies: cookies}
	data, err := json.MarshalIndent(cookieFile, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Cookie 失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入 Cookie 文件失败: %w", err)
	}

	log.Infof("已保存 %d 个 Cookie 到 %s", len(cookies), filePath)
	return nil
}

// SaveCookieToDatabase 保存新的 Cookie 到数据库（host_id 为空，等待分配）
func SaveCookieToDatabase(cookie string, zipcode string, city string) error {
	_, err := app.db.Exec(
		`INSERT INTO amc_cookie (cookie, zipcode, city, status) VALUES (?, ?, ?, 1)`,
		cookie, zipcode, city,
	)
	if err != nil {
		return fmt.Errorf("保存 Cookie 到数据库失败: %w", err)
	}

	log.Infof("已保存新 Cookie 到数据库 (zipcode=%s, city=%s)", zipcode, city)
	return nil
}

// SaveCookieToDatabaseWithHostID 保存 Cookie 到数据库并指定 host_id（用于迁移或强制绑定）
func SaveCookieToDatabaseWithHostID(hostID int, cookie string, zipcode string, city string) error {
	_, err := app.db.Exec(
		`INSERT INTO amc_cookie (host_id, cookie, zipcode, city, status) VALUES (?, ?, ?, ?, 1)`,
		hostID, cookie, zipcode, city,
	)
	if err != nil {
		return fmt.Errorf("保存 Cookie 到数据库失败: %w", err)
	}

	log.Infof("已保存 Cookie 到数据库 (host_id=%d, zipcode=%s)", hostID, zipcode)
	return nil
}

// GetRandomZipcode 随机获取一个邮编
func GetRandomZipcode() (string, string) {
	idx := time.Now().UnixNano() % int64(len(USZipCodes))
	return USZipCodes[idx].Zipcode, USZipCodes[idx].City
}

// CookieStats Cookie 统计信息
type CookieStats struct {
	Total       int `json:"total"`        // 总数
	Active      int `json:"active"`       // 正常状态
	Invalid     int `json:"invalid"`      // 已失效
	Unassigned  int `json:"unassigned"`   // 未分配（正常状态且 host_id 为空）
	Assigned    int `json:"assigned"`     // 已分配（正常状态且 host_id 不为空）
}

// GetCookieStats 获取 Cookie 统计信息
func GetCookieStats() (*CookieStats, error) {
	stats := &CookieStats{}

	err := app.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) as invalid,
			SUM(CASE WHEN status = 1 AND host_id IS NULL THEN 1 ELSE 0 END) as unassigned,
			SUM(CASE WHEN status = 1 AND host_id IS NOT NULL THEN 1 ELSE 0 END) as assigned
		FROM amc_cookie
	`).Scan(&stats.Total, &stats.Active, &stats.Invalid, &stats.Unassigned, &stats.Assigned)

	if err != nil {
		return nil, fmt.Errorf("获取 Cookie 统计失败: %w", err)
	}

	return stats, nil
}
