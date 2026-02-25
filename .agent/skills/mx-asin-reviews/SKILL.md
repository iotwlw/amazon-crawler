---
name: mx-asin-reviews
description: 查询亚马逊墨西哥站点 ASIN 的评价信息（评分、评论数），支持多站点
---

# 查询亚马逊 ASIN 评价

使用 amazon-crawler Go 程序查询亚马逊指定站点 ASIN 的评价信息，包括评分和评论数。默认查询墨西哥站。

## 参数

- `<ASIN列表>`: **必填**，要查询的 ASIN，多个用逗号分隔
- `--domain=<域名>`: 可选，亚马逊域名，默认 `www.amazon.com.mx`（墨西哥站）

## 支持的亚马逊站点

| 域名 | 站点 |
|------|------|
| www.amazon.com.mx | 墨西哥站（默认） |
| www.amazon.com | 美国站 |
| www.amazon.com.br | 巴西站 |
| www.amazon.es | 西班牙站 |

## 前置要求

1. Go 程序已编译（如未编译需先执行编译）
2. 数据库中有可用的 Cookie（`amc_cookie` 表中 `status = 1` 的记录）

## 执行步骤

### 步骤 1: 检查并编译程序

确认 Go 程序已编译，如果没有则运行：
```bash
go build -o amazon-crawler
```

### 步骤 2: 检查 Cookie 可用性

检查数据库中是否有可用 Cookie：
```sql
SELECT id, host_id, status, zipcode, city FROM amc_cookie WHERE status = 1;
```

如果没有可用 Cookie，提示用户先使用 `fetch-amazon-cookie` 技能获取。

### 步骤 3: 运行爬虫

使用以下命令格式运行：
```bash
./amazon-crawler -c config.yaml --asin=<ASIN列表> --domain=<域名>
```

示例：
```bash
# 单个 ASIN（墨西哥站）
./amazon-crawler -c config.yaml --asin=B08N5WRWNW --domain=www.amazon.com.mx

# 多个 ASIN
./amazon-crawler -c config.yaml --asin=B08N5WRWNW,B07XYZ,B123ABC --domain=www.amazon.com.mx
```

### 步骤 4: 查看结果

结果以 CSV 文件保存在 `output/` 目录。

## 输出格式

### CSV 字段

| 字段 | 说明 |
|------|------|
| ASIN | 亚马逊商品标识 |
| Rating | 评分（0-5） |
| ReviewCount | 评论数 |
| Status | 状态：success / error |
| ErrorMessage | 错误信息（如有） |

### 控制台输出示例

```
[INFO] 启动 ASIN 评论爬虫模式
[INFO] 程序标识:1 主机标识:1
[INFO] 使用 cookie (id=5): session-id=xxx-xxx-xxx; session-token=...
[INFO] 开始处理 2 个 ASIN
[INFO] 进度: 1/2 - 处理 ASIN: B08N5WRWNW
[INFO] ASIN: B08N5WRWNW, 评分: 4.5, 评论数: 1287
[INFO] 等待 3 秒后继续...
[INFO] 进度: 2/2 - 处理 ASIN: B07XYZ12345
[INFO] ASIN: B07XYZ12345, 评分: 4.2, 评论数: 534
[INFO] CSV 文件已导出: output/asins_20260116_143022.csv
```

## 错误处理

1. **Cookie 失效**: 程序会自动切换到新的 Cookie
2. **需要验证**: 遇到验证页面时，会标记 Cookie 失效并尝试切换
3. **网络错误**: 记录错误信息到 `ErrorMessage` 字段

## 注意事项

- 每个请求之间有 2-3 秒延迟，避免被检测
- Cookie 失效会自动切换，无可用 Cookie 时会提示获取
- 确保 `config.yaml` 中的 `host_id` 配置正确
