package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/tengfei-xy/go-log"
	"gopkg.in/yaml.v3"
)

const MYSQL_APPLICATION_STATUS_START int = 0
const MYSQL_APPLICATION_STATUS_OVER int = 1
const MYSQL_APPLICATION_STATUS_SEARCH int = 2
const MYSQL_APPLICATION_STATUS_PRODUCT int = 3
const MYSQL_APPLICATION_STATUS_SELLER int = 4

type appConfig struct {
	Mysql      `yaml:"mysql"`
	Basic      `yaml:"basic"`
	Proxy      `yaml:"proxy"`
	Exec       `yaml:"exec"`
	db         *sql.DB
	cookie     string
	cookieID   int64 // 当前使用的 cookie 记录 ID
	primary_id int64
}
type Exec struct {
	Enable          `yaml:"enable"`
	Loop            `yaml:"loop"`
	Search_priority int `yaml:"search_priority"`
}
type Enable struct {
	Search  bool `yaml:"search"`
	Product bool `yaml:"product"`
	Seller  bool `yaml:"seller"`
}
type Loop struct {
	All          int `yaml:"all"`
	all_time     int
	Search       int `yaml:"search"`
	search_time  int
	Product      int `yaml:"product"`
	product_time int
	Seller       int `yaml:"seller"`
	seller_time  int
}
type Basic struct {
	App_id  int    `yaml:"app_id"`
	Host_id int    `yaml:"host_id"`
	Test    bool   `yaml:"test"`
	Domain  string `yaml:"domain"`
}
type Proxy struct {
	Enable bool `yaml:"enable"`

	Sockc5 []string `yaml:"socks5"`
}
type Mysql struct {
	Ip       string `yaml:"ip"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}
type flagStruct struct {
	config_file string
	serve       string // HTTP 服务模式，值为监听地址如 ":8080"
	asin        string // ASIN 列表，逗号分隔
	domain      string // 指定亚马逊域名（仅 ASIN 模式有效）
}

var app appConfig
var robot Robots

const userAgent = `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36`

func init_config(flag flagStruct) {
	log.Infof("读取配置文件:%s", flag.config_file)

	yamlFile, err := os.ReadFile(flag.config_file)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(yamlFile, &app)
	if err != nil {
		panic(err)
	}
	if !app.Exec.Enable.Search && !app.Exec.Enable.Product && !app.Exec.Enable.Seller {
		panic("没有启动功能，检查配置文件的enable配置的选项")
	}
	if app.Exec.Loop.All == 0 {
		app.Exec.Loop.All = 999999
	}
	if app.Exec.Loop.Search == 0 {
		app.Exec.Loop.Search = 999999
	}
	if app.Exec.Loop.Product == 0 {
		app.Exec.Loop.Product = 999999
	}
	if app.Exec.Loop.Seller == 0 {
		app.Exec.Loop.Seller = 999999
	}
	app.Exec.product_time = 0
	app.Exec.search_time = 0
	app.Exec.seller_time = 0

	log.Infof("程序标识:%d 主机标识:%d", app.Basic.App_id, app.Basic.Host_id)
}
func init_rebots() {
	robotTxt := fmt.Sprintf("https://%s/robots.txt", app.Domain)

	log.Infof("加载文件: %s", robotTxt)
	txt, err := request_get(robotTxt, userAgent)
	if err != nil {
		log.Error("网络错误")
		panic(err)
	}
	robot = GetRobotFromTxt(txt)
}
func init_mysql() {
	DB, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", app.Mysql.Username, app.Mysql.Password, app.Mysql.Ip, app.Mysql.Port, app.Mysql.Database))
	if err != nil {
		panic(err)
	}
	DB.SetConnMaxLifetime(100)
	DB.SetMaxIdleConns(10)
	if err := DB.Ping(); err != nil {
		panic(err)
	}
	log.Info("数据库已连接")
	app.db = DB
}
func init_network() {
	log.Info("网络测试开始")

	var s searchStruct
	s.en_key = "Hardware+electrician"
	_, err := s.request(0)
	if err != nil {
		log.Error("网络错误")
		panic(err)
	}

}
func init_signal() {
	// 创建一个通道来接收操作系统的信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGABRT)

	go func() {
		<-sigCh
		log.Info("")
		log.Infof("程序即将结束")
		app.end()
		app.db.Close()
		log.Infof("程序结束")
		os.Exit(0)
	}()
}
func init_flag() flagStruct {
	var f flagStruct
	flag.StringVar(&f.config_file, "c", "config.yaml", "打开配置文件")
	flag.StringVar(&f.serve, "serve", "", "启动 HTTP 服务模式，指定监听地址如 :8080")
	flag.StringVar(&f.asin, "asin", "", "ASIN 列表，逗号分隔（如：B08N5WRWNW,B07XYZ）")
	flag.StringVar(&f.domain, "domain", "www.amazon.com.mx", "亚马逊域名（仅 ASIN 模式有效）")
	flag.Parse()
	return f
}

func main() {
	f := init_flag()
	init_config(f)
	init_rebots()
	init_mysql()
	init_network()
	init_signal()

	// 判断运行模式
	if f.asin != "" {
		// ASIN 评论爬虫模式
		log.Infof("启动 ASIN 评论爬虫模式")
		scraper := NewASINScraper(f.asin, f.domain)
		if err := scraper.Run(); err != nil {
			log.Errorf("ASIN 爬虫执行失败: %v", err)
			os.Exit(1)
		}
		return
	} else if f.serve != "" {
		// HTTP 服务模式
		log.Infof("启动 HTTP 服务模式")

		// 初始化并启动任务消费者
		InitTaskWorker()
		taskWorker.Start()

		// 启动 HTTP 服务（阻塞）
		StartHTTPServer(f.serve)
	} else {
		// 原有命令行模式
		log.Infof("启动命令行模式")
		app.start()

		for app.Exec.Loop.all_time = 0; app.Exec.Loop.all_time < app.Exec.Loop.All; app.Exec.Loop.all_time++ {
			var search searchStruct
			search.main()

			var product productStruct
			product.main()

			var seller sellerStruct
			seller.main()
		}
	}
}
func (app *appConfig) get_cookie() (string, error) {
	var cookie string
	var cookieID int64
	if app.Basic.Host_id == 0 {
		return "", fmt.Errorf("配置文件中host_id为0，cookie将为空")
	}

	// 查询当前 host_id 绑定的正常状态的 cookie
	err := app.db.QueryRow(
		"SELECT id, cookie FROM amc_cookie WHERE host_id = ? AND status = 1 LIMIT 1",
		app.Basic.Host_id,
	).Scan(&cookieID, &cookie)

	if err == sql.ErrNoRows {
		// 当前 host_id 没有可用的 cookie，尝试从未分配的 cookie 中获取一个
		log.Warnf("host_id=%d 没有可用的 cookie，尝试获取新的", app.Basic.Host_id)
		return app.acquireNewCookie()
	} else if err != nil {
		return "", err
	}

	cookie = strings.TrimSpace(cookie)
	if app.cookie != cookie {
		previewLen := 50
		if len(cookie) < previewLen {
			previewLen = len(cookie)
		}
		log.Infof("使用 cookie (id=%d): %s...", cookieID, cookie[:previewLen])
	}

	app.cookie = cookie
	app.cookieID = cookieID
	return app.cookie, nil
}

// acquireNewCookie 从未分配的正常 cookie 中获取一个并绑定到当前 host_id
func (app *appConfig) acquireNewCookie() (string, error) {
	tx, err := app.db.Begin()
	if err != nil {
		return "", fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	var cookieID int64
	var cookie string

	// 查找一个未分配的正常 cookie（host_id 为 NULL）
	err = tx.QueryRow(
		"SELECT id, cookie FROM amc_cookie WHERE host_id IS NULL AND status = 1 LIMIT 1 FOR UPDATE",
	).Scan(&cookieID, &cookie)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("没有可用的未分配 cookie，请通过 SKILL 获取新的 Session")
	} else if err != nil {
		return "", fmt.Errorf("查询未分配 cookie 失败: %w", err)
	}

	// 将 cookie 绑定到当前 host_id
	_, err = tx.Exec(
		"UPDATE amc_cookie SET host_id = ? WHERE id = ?",
		app.Basic.Host_id, cookieID,
	)
	if err != nil {
		return "", fmt.Errorf("绑定 cookie 失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("提交事务失败: %w", err)
	}

	cookie = strings.TrimSpace(cookie)
	log.Infof("获取新 cookie (id=%d) 并绑定到 host_id=%d", cookieID, app.Basic.Host_id)

	app.cookie = cookie
	app.cookieID = cookieID
	return app.cookie, nil
}

// markCookieInvalid 将当前使用的 cookie 标记为失效
func (app *appConfig) markCookieInvalid() error {
	if app.cookieID == 0 {
		return fmt.Errorf("没有正在使用的 cookie")
	}

	_, err := app.db.Exec(
		"UPDATE amc_cookie SET status = 0 WHERE id = ?",
		app.cookieID,
	)
	if err != nil {
		return fmt.Errorf("标记 cookie 失效失败: %w", err)
	}

	log.Warnf("cookie (id=%d) 已标记为失效", app.cookieID)
	app.cookie = ""
	app.cookieID = 0
	return nil
}

// handleCookieInvalid 处理 cookie 失效的情况：标记失效并尝试获取新的
func (app *appConfig) handleCookieInvalid() error {
	// 标记当前 cookie 为失效
	if err := app.markCookieInvalid(); err != nil {
		log.Errorf("标记 cookie 失效出错: %v", err)
	}

	// 尝试获取新的 cookie
	_, err := app.acquireNewCookie()
	return err
}
func (app *appConfig) start() {
	if app.Basic.Test {
		log.Infof("测试模式启动")
		return
	}
	r, err := app.db.Exec("insert into amc_application (app_id) values(?)", app.Basic.App_id)
	if err != nil {
		panic(err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		panic(err)
	}
	app.primary_id = id
}
func (app *appConfig) update(status int) {
	_, err := app.db.Exec("update amc_application set status=? where id=?", status, app.primary_id)
	if err != nil {
		panic(err)
	}
}
func (app *appConfig) end() {
	if app.Basic.Test {
		return
	}
	if _, err := app.db.Exec("update amc_application set status=? where id=?", MYSQL_APPLICATION_STATUS_OVER, app.primary_id); err != nil {
		log.Error(err)
	}
}
