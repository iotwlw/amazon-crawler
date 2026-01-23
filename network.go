package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// BrowserProfile 完整的浏览器指纹配置（UA 和 sec-ch-ua 必须匹配）
type BrowserProfile struct {
	ID              string
	UserAgent       string
	SecChUa         string
	SecChUaPlatform string
	SecChUaMobile   string
	AcceptLanguage  string
	AcceptEncoding  string
	Accept          string
	DeviceMemory    string
	Downlink        string
	ECT             string
	RTT             string
	DPR             string
}

// 预定义的浏览器配置（确保 UA 和 sec-ch-ua 版本一致）
var browserProfiles = []BrowserProfile{
	// Chrome 120 Windows
	{
		ID:              "chrome-120-win",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		SecChUa:         `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		SecChUaPlatform: `"Windows"`,
		SecChUaMobile:   "?0",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		DeviceMemory:    "8",
		Downlink:        "10",
		ECT:             "4g",
		RTT:             "50",
		DPR:             "1",
	},
	// Chrome 119 macOS
	{
		ID:              "chrome-119-mac",
		UserAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		SecChUa:         `"Google Chrome";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`,
		SecChUaPlatform: `"macOS"`,
		SecChUaMobile:   "?0",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		DeviceMemory:    "8",
		Downlink:        "10",
		ECT:             "4g",
		RTT:             "100",
		DPR:             "2",
	},
	// Firefox 121 Windows
	{
		ID:              "firefox-121-win",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		SecChUa:         "",
		SecChUaPlatform: "",
		SecChUaMobile:   "",
		AcceptLanguage:  "en-US,en;q=0.5",
		AcceptEncoding:  "gzip, deflate, br",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		DeviceMemory:    "8",
		Downlink:        "10",
		ECT:             "4g",
		RTT:             "50",
		DPR:             "1",
	},
	// Edge 120 Windows
	{
		ID:              "edge-120-win",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		SecChUa:         `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`,
		SecChUaPlatform: `"Windows"`,
		SecChUaMobile:   "?0",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		DeviceMemory:    "8",
		Downlink:        "10",
		ECT:             "4g",
		RTT:             "50",
		DPR:             "1",
	},
	// Safari 17 macOS
	{
		ID:              "safari-17-mac",
		UserAgent:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		SecChUa:         "",
		SecChUaPlatform: "",
		SecChUaMobile:   "",
		AcceptLanguage:  "en-US,en;q=0.9",
		AcceptEncoding:  "gzip, deflate, br",
		Accept:          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		DeviceMemory:    "8",
		Downlink:        "10",
		ECT:             "4g",
		RTT:             "50",
		DPR:             "2",
	},
}

// getRandomBrowserProfile 随机获取一个浏览器配置
func getRandomBrowserProfile() *BrowserProfile {
	idx := rand.Intn(len(browserProfiles))
	profile := browserProfiles[idx]
	return &profile
}

// getBrowserProfileByID 根据 ID 获取浏览器配置
func getBrowserProfileByID(id string) *BrowserProfile {
	for i := range browserProfiles {
		if browserProfiles[i].ID == id {
			return &browserProfiles[i]
		}
	}
	// 如果找不到，返回第一个
	return &browserProfiles[0]
}

// HeaderProfile 请求头配置
type HeaderProfile struct {
	Accept         string
	AcceptLanguage string
	AcceptEncoding string
	SecFetchDest   string
	SecFetchMode   string
	SecFetchSite   string
	CacheControl   string
}

// 请求头库：15种不同的浏览器请求头配置
var headerProfiles = []HeaderProfile{
	// Chrome 120+ Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "max-age=0",
	},
	// Chrome 119 macOS
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "same-origin",
		CacheControl:   "no-cache",
	},
	// Firefox 121 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.5",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "max-age=0",
	},
	// Firefox 120 Linux
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.7,es;q=0.3",
		AcceptEncoding: "gzip, deflate",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "cross-site",
		CacheControl:   "no-cache",
	},
	// Edge 120 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "max-age=0",
	},
	// Safari 17 macOS
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "max-age=0",
	},
	// Chrome 118 Android
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "same-origin",
		CacheControl:   "no-cache",
	},
	// Chrome 117 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9,fr;q=0.8",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "max-age=0",
	},
	// Firefox 119 macOS
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.5",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "cross-site",
		CacheControl:   "no-cache",
	},
	// Edge 119 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9,de;q=0.8",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "same-origin",
		CacheControl:   "max-age=0",
	},
	// Chrome 116 Linux
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "no-cache",
	},
	// Safari 16 iOS
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9",
		AcceptEncoding: "gzip, deflate",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "same-origin",
		CacheControl:   "max-age=0",
	},
	// Firefox 118 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.5,ja;q=0.3",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "none",
		CacheControl:   "no-cache",
	},
	// Chrome 115 macOS
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9,pt;q=0.8",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "cross-site",
		CacheControl:   "max-age=0",
	},
	// Edge 118 Windows
	{
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		AcceptLanguage: "en-US,en;q=0.9,it;q=0.8",
		AcceptEncoding: "gzip, deflate, br",
		SecFetchDest:   "document",
		SecFetchMode:   "navigate",
		SecFetchSite:   "same-origin",
		CacheControl:   "no-cache",
	},
}

// get_random_header_profile 随机获取一个请求头配置
func get_random_header_profile() HeaderProfile {
	return headerProfiles[rangdom_range(len(headerProfiles))]
}

func rangdom_range(max int) int {
	rand.NewSource(time.Now().UnixNano())
	return rand.Intn(max)
}
func get_socks5_proxy() (proxy.Dialer, error) {
	// 创建一个SOCKS5代理拨号器
	len := len(app.Proxy.Sockc5)
	if len == 0 {
		return nil, fmt.Errorf("没有可用的代理")
	}
	return proxy.SOCKS5("tcp", app.Proxy.Sockc5[rangdom_range(len)], nil, proxy.Direct)
}
func get_client() http.Client {

	proxy, err := get_socks5_proxy()
	if err != nil {
		return http.Client{Timeout: time.Second * 60}
	}
	if app.Proxy.Enable {
		return http.Client{
			Transport: &http.Transport{
				Dial: proxy.Dial,
			},

			Timeout: time.Second * 60,
		}
	} else {
		return http.Client{Timeout: time.Second * 60}
	}
}

func telnet(ip string) bool {
	conn, err := net.DialTimeout("tcp", ip, 5*time.Second)
	if err != nil {
		return false
	} else {
		if conn != nil {
			_ = conn.Close()
			return true
		} else {
			return false
		}
	}
}
func request_get(url string, ua string) (string, error) {
	client := get_client()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// 随机选择一个请求头配置，模拟不同的浏览器
	profile := get_random_header_profile()

	// 设置请求头
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", profile.Accept)
	req.Header.Set("Accept-Language", profile.AcceptLanguage)
	req.Header.Set("Accept-Encoding", profile.AcceptEncoding)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", profile.SecFetchDest)
	req.Header.Set("Sec-Fetch-Mode", profile.SecFetchMode)
	req.Header.Set("Sec-Fetch-Site", profile.SecFetchSite)
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", profile.CacheControl)

	resp, err := client.Do(req)
	if err != nil {
		return "", err

	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("状态码:%d", resp.StatusCode)
	}

	resp_data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(resp_data), nil
}

// setCommonHeaders 设置统一的请求头（使用绑定的浏览器指纹）
func (app *appConfig) setCommonHeaders(req *http.Request) {
	if app.browserProfile == nil {
		// 如果没有浏览器指纹，随机选择一个（兜底逻辑）
		app.browserProfile = getRandomBrowserProfile()
	}

	profile := app.browserProfile

	// 设置核心浏览器指纹头部
	req.Header.Set("User-Agent", profile.UserAgent)
	req.Header.Set("Accept", profile.Accept)
	req.Header.Set("Accept-Language", profile.AcceptLanguage)
	req.Header.Set("Accept-Encoding", profile.AcceptEncoding)

	// 设置 sec-ch-ua 系列头部（Chrome/Edge 需要，Firefox/Safari 不需要）
	if profile.SecChUa != "" {
		req.Header.Set("sec-ch-ua", profile.SecChUa)
		req.Header.Set("sec-ch-ua-platform", profile.SecChUaPlatform)
		req.Header.Set("sec-ch-ua-mobile", profile.SecChUaMobile)
	}

	// 设置设备特征头部
	if profile.DeviceMemory != "" {
		req.Header.Set("device-memory", profile.DeviceMemory)
	}
	if profile.Downlink != "" {
		req.Header.Set("downlink", profile.Downlink)
	}
	if profile.ECT != "" {
		req.Header.Set("ect", profile.ECT)
	}
	if profile.RTT != "" {
		req.Header.Set("rtt", profile.RTT)
	}
	if profile.DPR != "" {
		req.Header.Set("dpr", profile.DPR)
	}

	// 设置通用头部
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	// 设置 Sec-Fetch 系列头部
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	// 设置 Cookie
	req.Header.Set("Cookie", app.cookie)
}
