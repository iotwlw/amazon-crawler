# amazon-crawler

一种在亚马逊平台，根据关键词支持多主机搜索商家信息（名称、地址、税号）的工具，适用多种主流操作系统

**问题反馈**： 联系方式位于[个人首页](https://github.com/tengfei-xy)

# 一、爬虫说明

请尊守网站的robots.txt，本项目中的程序的任何http请求均取决于robots.txt

# 二、项目说明

本项目是通过关键词在亚马逊进行搜索的，并将搜索结果保留到数据库中并提取每个商品的卖家的信息，最后从卖家信息中提取TRN。由于每个步骤相互独立，因此适合分布式运行。

# 三、初始化说明

[下载](https://github.com/tengfei-xy/amazon-crawler/releases)

## 数据库

> 仅支持mysql

导入[SQL文件](https://github.com/tengfei-xy/amazon-crawler/blob/main/sql/ddl.sql)并初始化数据库参考命令：

```bash
# 执行后，将丢弃原表的数据
bin/mysql -u root -p < ddl.sql
```

由于工具需要关键词来进行搜索，所以需要初始化一些关键词，参考[关键词数据库文件]((https://github.com/tengfei-xy/amazon-crawler/blob/main/sql/category.sql))

导入命令

```bash
# 此语句应建表后立即使用
bin/mysql -D amazon -u root -p < category.sql
# 若只添加测试使用单条命令，使用下列语句,并设置最低的优先级为0
# insert into `category` (`zh_key`,`en_key`,priority) VALUES ('音响','speakers',0);
```

创建amazone用户

```sql
# mysql 5.7.x
GRANT ALL PRIVILEGES ON amazon.* to 'amazon'@'%'  identified by 'password';
flush privileges;

# mysql 8.x
create user 'amazon'@'%' identified by 'password';
GRANT ALL PRIVILEGES ON amazon.* TO 'amazon'@'%' WITH GRANT OPTION;
flush privileges;
```



## 配置文件

复制config.yaml.save为config.yaml

配置文件具体含义参考[文件中的注释](https://github.com/tengfei-xy/amazon-crawler/blob/main/config.yaml.save)

## cookie

你应该为每个配置文件中的host_id填充cookie，填充位置是数据库的cookie表，分别包含host_id和对应的cookie，

cookie格式如下

```
session-id=xxxx; session-id-time=2082787201l; i18n-prefs=GBP; sp-cdn="L5Z9:JP"; ubid-acbuk=xxx; session-token="xxxx"; csm-hit=xxx
```

> Cookie 解析
>
> sp-cdn：页面中选择的配送地区
>
> lc-main：页面中选择的语言

注：亚马逊的cokie，半小时左右后会失效，但也会传回有效cookie，接着用就行。替换三次左右，cookie就变成天单位的有效时长了。至少有个2天。

注：cookie在程序运行时可随时修改，每次发送http请求时都会从数据库从重新获取一次

注：对于从亚马逊网页中获取cookie时，最好同意页面中提示的cookie，让cookie的存活更久

# 四、启动

## 命令行模式（原有方式）

从数据库 `amc_category` 表读取关键词，自动循环执行：

```bash
./amazon-crawler -c config.yaml
```

## HTTP 服务模式（API 调用）

启动 HTTP 服务，通过 API 接收任务：

```bash
./amazon-crawler -c config.yaml -serve :8080
```

### API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/crawl | 提交爬取任务 |
| GET | /api/status | 查看任务状态 |
| GET | /health | 健康检查 |

### 提交任务

```bash
curl -X POST http://localhost:8080/api/crawl \
  -H "Content-Type: application/json" \
  -d '{"keywords": ["nike", "adidas", "puma"]}'
```

响应：
```json
{"code":0,"message":"任务已提交","data":{"total":3,"inserted":3,"skipped":0}}
```

### 查看状态

```bash
curl http://localhost:8080/api/status
```

响应：
```json
{"code":0,"message":"ok","data":{"pending":10,"completed":50,"failed":2}}
```

### 任务状态说明

| 状态值 | 字段 | 说明 |
|--------|------|------|
| 0 | pending | 待执行 |
| 1 | completed | 已执行 |
| 2 | failed | 失败 |

### 数据库表变更

HTTP 服务模式需要先执行数据库变更：

```sql
ALTER TABLE `amc_category`
ADD COLUMN `task_status` TINYINT(1) NOT NULL DEFAULT 0,
ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;

ALTER TABLE `amc_category` ADD INDEX `idx_task_status` (`task_status`);
```

### 执行流程

```
API 提交关键词 → 写入 amc_category 表 (task_status=0)
                         ↓
              Worker 轮询读取待执行任务
                         ↓
              搜索商品 → 提取卖家ID → 获取卖家详情
                         ↓
              更新 task_status (1=成功, 2=失败)
```



# 五、运行情况

> 4天

1k多个关键词，两次搜索，共发现235334个产品

```sql
mysql> select count(*) as `计数` from product;
+--------+
| 计数   |
+--------+
| 235334 |
+--------+
1 row in set (0.06 sec)
```

其中，对每个产品检查商家信息

```sql
mysql> SELECT * FROM amazon.产品检查表;
+--------------+--------------+
| 状态         | 链接数量     |
+--------------+--------------+
| 未搜索       |        32691 |
| 准备检查     |          391 |
| 检查结束     |       100891 |
| 其他错误     |           13 |
| 没有商家     |       101348 |
+--------------+--------------+
5 rows in set (0.10 sec)
```





