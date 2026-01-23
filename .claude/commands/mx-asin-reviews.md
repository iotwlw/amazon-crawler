---
argument-hint: <ASIN列表> [--domain=www.amazon.com.mx]
description: 查询亚马逊墨西哥站点 ASIN 的评价信息（评分、评论数）
---

# Claude Command: 查询亚马逊墨西哥站 ASIN 评价

查询亚马逊墨西哥站点指定 ASIN 的评价信息，包括评分和评论数。

## 前置要求

1. 确保 Go 程序已编译：
```bash
go build -o amazon-crawler
```

2. 确保数据库中有可用的 Cookie：
```sql
-- 查看当前可用 Cookie
SELECT id, host_id, status, zipcode, city FROM amc_cookie WHERE status = 1;

-- 如果没有 Cookie，使用 /fetch-amazon-cookie 命令获取
```

## 使用方法

```bash
# 查询单个 ASIN（墨西哥站）
/mx-asin-reviews B08N5WRWNW

# 查询多个 ASIN（逗号分隔）
/mx-asin-reviews B08N5WRWNW,B07XYZ,B123ABC

# 指定其他亚马逊站点
/mx-asin-reviews B08N5WRWNW --domain=www.amazon.com
/mx-asin-reviews B08N5WRWNW --domain=www.amazon.com.br
```

## 命令参数

- `<ASIN列表>`: 必填，要查询的 ASIN，多个用逗号分隔
- `--domain=<域名>`: 可选，亚马逊域名，默认 `www.amazon.com.mx`（墨西哥站）

## 支持的亚马逊站点

| 域名 | 站点 |
|------|------|
| www.amazon.com.mx | 墨西哥站（默认） |
| www.amazon.com | 美国站 |
| www.amazon.com.br | 巴西站 |
| www.amazon.es | 西班牙站 |

## 执行步骤

### 步骤 1: 编译程序

如果尚未编译，执行：
```bash
go build -o amazon-crawler
```

### 步骤 2: 运行爬虫

使用以下命令格式运行：
```bash
./amazon-crawler -c config.yaml --asin=<ASIN列表> --domain=<域名>
```

例如：
```bash
# 查询单个 ASIN（墨西哥站）
./amazon-crawler -c config.yaml --asin=B08N5WRWNW --domain=www.amazon.com.mx

# 查询多个 ASIN
./amazon-crawler -c config.yaml --asin=B08N5WRWNW,B07XYZ,B123ABC --domain=www.amazon.com.mx
```

### 步骤 3: 查看结果

结果将保存为 CSV 文件，位于 `output/` 目录：

```csv
ASIN,Rating,ReviewCount,Status,ErrorMessage
B08N5WRWNW,4.5,1287,success,
B07XYZ,4.2,534,success,
B123ABC,0,0,success,
```

同时控制台会输出实时日志：
```
[INFO] 开始处理 3 个 ASIN
[INFO] 进度: 1/3 - 处理 ASIN: B08N5WRWNW
[INFO] ASIN: B08N5WRWNW, 评分: 4.5, 评论数: 1287
[INFO] 等待 3 秒后继续...
[INFO] 进度: 2/3 - 处理 ASIN: B07XYZ
[INFO] ASIN: B07XYZ, 评分: 4.2, 评论数: 534
[INFO] CSV 文件已导出: output/asins_20260116_143022.csv
```

## 输出格式

### CSV 文件字段

| 字段 | 说明 |
|------|------|
| ASIN | 亚马逊商品标识 |
| Rating | 评分（0-5） |
| ReviewCount | 评论数 |
| Status | 状态：success/error |
| ErrorMessage | 错误信息（如有） |

## 错误处理

1. **Cookie 失效**: 程序会自动切换到新的 Cookie
2. **需要验证**: 如果遇到验证页面，会标记 Cookie 失效并尝试切换
3. **网络错误**: 记录错误信息到 ErrorMessage 字段

## 注意事项

- 每个请求之间有 2-3 秒延迟，避免被检测
- Cookie 失效会自动切换，如果无可用的 Cookie 会提示获取
- 确保配置文件 `config.yaml` 中的 `host_id` 配置正确

## 示例输出

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
