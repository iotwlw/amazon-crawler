---
name: amazon-link-inspection
description: >-
  Run amazon-crawler link inspection for Amazon ASIN/product URL text files and export XLSX reports. Use when the user says "链接巡检", "批量抓取", "亚马逊批量抓取", "ASIN批量抓取", "批量查询ASIN", "批量查询链接", "Ask Rufus", or asks to crawl/scrape/query Amazon product links from a text file with product, ASIN, price, coupon, deal, member price, calculated List Price discount, rating, review count, PromoCheck, Promotion, PromoCode, Keep, Choice, and Ask Rufus question columns.
---

# Amazon Link Inspection

Use this skill to run the repo's built-in link inspection mode. It reads a text file containing Amazon product URLs or bare ASINs and writes an XLSX report with the EasySpider link inspection columns plus the Ask Rufus question column.

## Trigger Phrases

- 链接巡检
- 批量查询链接
- 批量查询ASIN
- ASIN批量抓取
- 亚马逊批量抓取
- Ask Rufus

## Inputs

- Link file: one Amazon product URL or ASIN per line.
- Domain: optional. If omitted, the program detects the first URL domain; use `www.amazon.com` for US links.
- Output path: optional. Defaults to `output/link_inspection_YYYYMMDD_HHMMSS.xlsx`.

## Output Columns

`产品, 原ASIN, ASIN, 价格, 优惠券, 是否秒杀, 会员专享, 显示折扣, 评级, 评价数量, PromoCheck, Promotion, PromoCode, Keep, Choice, Ask Rufus问题`

## Workflow

1. Work from the crawler repo:

```powershell
Set-Location D:\AmazonCode\amazon-crawler
```

2. Confirm the binary exists; build it if needed:

```powershell
if (!(Test-Path .\amazon-crawler.exe)) { go build -o amazon-crawler.exe }
```

3. Confirm `config.yaml` points at a working MySQL database and `amc_cookie` has an active cookie for the configured `host_id`.

4. Run batch scraping and keep a log:

```powershell
$stamp = Get-Date -Format 'yyyy_MM_dd_HH_mm_ss'
$out = "C:\Users\bobo\Desktop\链接巡检_$stamp.xlsx"
$log = "D:\AmazonCode\amazon-crawler\output\link_inspection_$stamp.log"
.\amazon-crawler.exe -c config.yaml -link-file "C:\Users\bobo\Desktop\我的文本.txt" -domain www.amazon.com -link-output $out 2>&1 | Tee-Object -FilePath $log
```

5. To choose the output file explicitly:

```powershell
.\amazon-crawler.exe -c config.yaml -link-file "C:\Users\bobo\Desktop\我的文本.txt" -domain www.amazon.com -link-output "C:\Users\bobo\Desktop\链接巡检.xlsx"
```

6. Report the generated XLSX path and summarize success/failure counts from the command log.

## Notes

- The mode preserves input order and duplicates, matching EasySpider's `removeDuplicate: 0` behavior.
- The `原ASIN` column stores the original input line. The `ASIN` column stores the product page ASIN.
- The `Ask Rufus问题` column stores visible Ask Rufus suggested questions from the product page, separated by newlines.
- `显示折扣` is calculated from List Price and current price, so `List Price: $269.99` with `价格: $242.99` returns `-10%`.
- `优惠券` is extracted from coupon text nodes and script/style content is stripped before matching, avoiding encoded-token false positives.
- The crawler reuses the repo's cookie rotation, browser fingerprint headers, robots.txt checks, proxy settings, and random request delay.
- Promotion fields are parsed from product-page DOM. If Amazon hides a promotion behind dynamic browser-only UI, those fields may be blank while price/rating/review fields still populate.
