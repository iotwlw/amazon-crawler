package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// BrowserFingerprint 浏览器指纹配置
type BrowserFingerprint struct {
	UserAgent     string
	SecChUa       string
	SecChUaMobile string
	SecChPlatform string
	AcceptLang    string
	DeviceMemory  string
	Downlink      string
	ECT           string
	RTT           string
	DPR           string
}

// 浏览器指纹池 - 包含多个真实的浏览器配置
var fingerprintPool = []BrowserFingerprint{
	// Chrome 131 on Windows 11
	{
		UserAgent:     `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36`,
		SecChUa:       `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
		SecChUaMobile: `?0`,
		SecChPlatform: `"Windows"`,
		AcceptLang:    `en-US,en;q=0.9`,
		DeviceMemory:  `8`,
		Downlink:      `10`,
		ECT:           `4g`,
		RTT:           `50`,
		DPR:           `1`,
	},
	// Chrome 131 on macOS
	{
		UserAgent:     `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36`,
		SecChUa:       `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
		SecChUaMobile: `?0`,
		SecChPlatform: `"macOS"`,
		AcceptLang:    `en-US,en;q=0.9`,
		DeviceMemory:  `8`,
		Downlink:      `5.6`,
		ECT:           `4g`,
		RTT:           `100`,
		DPR:           `2`,
	},
	// Chrome 130 on Windows 10
	{
		UserAgent:     `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36`,
		SecChUa:       `"Google Chrome";v="130", "Chromium";v="130", "Not_A Brand";v="99"`,
		SecChUaMobile: `?0`,
		SecChPlatform: `"Windows"`,
		AcceptLang:    `en-US,en;q=0.9,zh-CN;q=0.8`,
		DeviceMemory:  `16`,
		Downlink:      `10`,
		ECT:           `4g`,
		RTT:           `50`,
		DPR:           `1.25`,
	},
	// Edge 131 on Windows 11
	{
		UserAgent:     `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0`,
		SecChUa:       `"Microsoft Edge";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
		SecChUaMobile: `?0`,
		SecChPlatform: `"Windows"`,
		AcceptLang:    `en-US,en;q=0.9`,
		DeviceMemory:  `8`,
		Downlink:      `10`,
		ECT:           `4g`,
		RTT:           `50`,
		DPR:           `1`,
	},
	// Chrome 129 on macOS (稍旧版本)
	{
		UserAgent:     `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36`,
		SecChUa:       `"Google Chrome";v="129", "Chromium";v="129", "Not_A Brand";v="24"`,
		SecChUaMobile: `?0`,
		SecChPlatform: `"macOS"`,
		AcceptLang:    `en-US,en;q=0.9`,
		DeviceMemory:  `16`,
		Downlink:      `2.5`,
		ECT:           `4g`,
		RTT:           `150`,
		DPR:           `2`,
	},
}

// 当前会话使用的指纹（每次启动随机选择一个，保持会话一致性）
var currentFingerprint BrowserFingerprint

func init() {
	// 程序启动时随机选择一个指纹
	rand.Seed(time.Now().UnixNano())
	currentFingerprint = fingerprintPool[rand.Intn(len(fingerprintPool))]
}

// GetCurrentFingerprint 获取当前会话的指纹
func GetCurrentFingerprint() BrowserFingerprint {
	return currentFingerprint
}

// RotateFingerprint 轮换指纹（在 Cookie 切换时调用）
func RotateFingerprint() {
	rand.Seed(time.Now().UnixNano())
	currentFingerprint = fingerprintPool[rand.Intn(len(fingerprintPool))]
}

// ApplyFingerprint 将指纹应用到 HTTP 请求
func ApplyFingerprint(req *http.Request, referer string) {
	fp := currentFingerprint

	// 基础请求头
	req.Header.Set("Accept", `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	req.Header.Set("Accept-Language", fp.AcceptLang)
	req.Header.Set("Cache-Control", `max-age=0`)
	req.Header.Set("Connection", `keep-alive`)

	// User-Agent 相关
	req.Header.Set("User-Agent", fp.UserAgent)
	req.Header.Set("sec-ch-ua", fp.SecChUa)
	req.Header.Set("sec-ch-ua-mobile", fp.SecChUaMobile)
	req.Header.Set("sec-ch-ua-platform", fp.SecChPlatform)

	// 客户端提示 (Client Hints)
	req.Header.Set("device-memory", fp.DeviceMemory)
	req.Header.Set("downlink", fp.Downlink)
	req.Header.Set("ect", fp.ECT)
	req.Header.Set("rtt", fp.RTT)
	req.Header.Set("dpr", fp.DPR)

	// Fetch 元数据
	req.Header.Set("Sec-Fetch-Dest", `document`)
	req.Header.Set("Sec-Fetch-Mode", `navigate`)
	req.Header.Set("Sec-Fetch-Site", `same-origin`)
	req.Header.Set("Sec-Fetch-User", `?1`)

	// 升级请求
	req.Header.Set("Upgrade-Insecure-Requests", `1`)

	// Referer（如果提供）
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

// RandomDelay 随机延迟，模拟人类行为
// minSeconds: 最小延迟秒数
// maxSeconds: 最大延迟秒数
func RandomDelay(minSeconds, maxSeconds int) {
	if minSeconds >= maxSeconds {
		sleep(minSeconds)
		return
	}
	delay := minSeconds + rand.Intn(maxSeconds-minSeconds+1)
	sleep(delay)
}

// SmartDelay 智能延迟，根据场景选择合适的延迟
// scenario: "normal" 正常请求, "error" 错误后重试, "captcha" 验证码后
func SmartDelay(scenario string) {
	switch scenario {
	case "normal":
		// 正常请求间隔：3-8秒
		RandomDelay(3, 8)
	case "page":
		// 翻页间隔：5-12秒
		RandomDelay(5, 12)
	case "error":
		// 错误后重试：60-180秒
		RandomDelay(60, 180)
	case "captcha":
		// 验证码/Cookie失效后：180-300秒
		RandomDelay(180, 300)
	case "503":
		// 503错误后：90-150秒
		RandomDelay(90, 150)
	default:
		RandomDelay(3, 8)
	}
}

// GetRandomReferer 生成随机的 Referer
func GetRandomReferer(domain string) string {
	referers := []string{
		fmt.Sprintf("https://%s/", domain),
		fmt.Sprintf("https://%s/gp/browse.html", domain),
		fmt.Sprintf("https://%s/s", domain),
		fmt.Sprintf("https://www.google.com/"),
		"", // 有时候不带 Referer
	}
	return referers[rand.Intn(len(referers))]
}
