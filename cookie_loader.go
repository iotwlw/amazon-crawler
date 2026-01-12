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

// SaveCookieToDatabase 保存 Cookie 到数据库
func SaveCookieToDatabase(hostID int, cookie string) error {
	// 使用 INSERT ... ON DUPLICATE KEY UPDATE 实现 upsert
	_, err := app.db.Exec(
		`INSERT INTO amc_cookie (host_id, cookie) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE cookie = VALUES(cookie)`,
		hostID, cookie,
	)
	if err != nil {
		return fmt.Errorf("保存 Cookie 到数据库失败: %w", err)
	}

	log.Infof("已保存 Cookie 到数据库 (host_id=%d)", hostID)
	return nil
}

// GetRandomZipcode 随机获取一个邮编
func GetRandomZipcode() (string, string) {
	idx := time.Now().UnixNano() % int64(len(USZipCodes))
	return USZipCodes[idx].Zipcode, USZipCodes[idx].City
}
