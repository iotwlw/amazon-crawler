---
argument-hint: [--count=1|3] [--save-to=db|file] [--host-id=1]
description: 通过 Playwright MCP 自动获取亚马逊 Cookie
---

# Claude Command: Fetch Amazon Cookie

通过 Playwright MCP 浏览器自动化工具，自动访问亚马逊美国站并获取有效的 Cookie。

## 前置要求

必须先安装 Playwright MCP：
```bash
claude mcp add playwright npx @playwright/mcp@latest
```

## 使用方法

```bash
# 获取 1 个 Cookie（默认）
/fetch-amazon-cookie

# 获取 3 个 Cookie
/fetch-amazon-cookie --count=3

# 指定保存方式
/fetch-amazon-cookie --save-to=db --host-id=1
/fetch-amazon-cookie --save-to=file
```

## 命令参数

- `--count=N`: 获取 Cookie 的数量，默认 1，最大 5
- `--save-to=db|file`:
  - `db`: 保存到数据库 amc_cookie 表（需要指定 --host-id）
  - `file`: 保存到 cookies.json 文件（默认）
- `--host-id=N`: 数据库 host_id，仅 --save-to=db 时有效

## 美国邮编池

以下是 20 个美国主要城市的邮编，每次随机选择一个：

```
10001 - 纽约 (New York, NY)
10013 - 纽约曼哈顿 (Manhattan, NY)
90001 - 洛杉矶 (Los Angeles, CA)
90210 - 比佛利山庄 (Beverly Hills, CA)
60601 - 芝加哥 (Chicago, IL)
60611 - 芝加哥市中心 (Chicago Downtown, IL)
77001 - 休斯顿 (Houston, TX)
77002 - 休斯顿市中心 (Houston Downtown, TX)
85001 - 凤凰城 (Phoenix, AZ)
19101 - 费城 (Philadelphia, PA)
78201 - 圣安东尼奥 (San Antonio, TX)
92101 - 圣地亚哥 (San Diego, CA)
75201 - 达拉斯 (Dallas, TX)
95101 - 圣何塞 (San Jose, CA)
78701 - 奥斯汀 (Austin, TX)
32801 - 奥兰多 (Orlando, FL)
33101 - 迈阿密 (Miami, FL)
98101 - 西雅图 (Seattle, WA)
80201 - 丹佛 (Denver, CO)
02101 - 波士顿 (Boston, MA)
```

## 执行步骤

### 步骤 1: 从邮编池随机选择一个邮编

从上面的列表中随机选择一个邮编，记录选中的邮编和城市名称。

### 步骤 2: 打开亚马逊首页

使用 Playwright MCP 的 `browser_navigate` 工具：
```
打开 https://www.amazon.com
```

### 步骤 3: 等待页面加载并获取快照

使用 `browser_snapshot` 获取页面结构，找到 "Deliver to" 或地址选择区域的元素引用。

通常这个元素的特征是：
- 包含 "Deliver to" 文字
- 位于页面左上角导航栏
- 可能的选择器：`#nav-global-location-popover-link` 或 `#glow-ingress-block`

### 步骤 4: 点击地址区域触发弹窗

使用 `browser_click` 点击地址区域，这会弹出一个模态对话框。

### 步骤 5: 等待弹窗出现

使用 `browser_wait_for` 等待弹窗内容加载：
```
等待文字 "Choose your location" 或 "Enter a US zip code" 出现
```

### 步骤 6: 获取弹窗快照并定位输入框

使用 `browser_snapshot` 获取弹窗的结构，找到邮编输入框的元素引用。

输入框特征：
- placeholder 可能包含 "zip code" 或 "Enter ZIP code"
- 可能的 id: `GLUXZipUpdateInput`

### 步骤 7: 输入随机邮编

使用 `browser_type` 在输入框中输入选中的邮编：
```
输入邮编（如 "10001"）
```

### 步骤 8: 点击确认按钮

使用 `browser_click` 点击 "Apply" 或 "Done" 按钮。

按钮特征：
- 文字为 "Apply" 或 "Done"
- 可能的选择器：`#GLUXZipUpdate` 或包含 "Apply" 文字的按钮

### 步骤 9: 等待页面刷新

使用 `browser_wait_for` 等待：
```
等待 2-3 秒让页面刷新完成
```

或者等待地址区域显示更新后的地址信息。

### 步骤 10: 获取 Cookie

使用 `browser_evaluate` 执行 JavaScript 获取 Cookie：
```javascript
() => document.cookie
```

### 步骤 11: 存储 Cookie

根据 `--save-to` 参数：

**保存到文件 (cookies.json):**
```json
{
  "cookies": [
    {
      "zipcode": "10001",
      "city": "New York, NY",
      "cookie": "session-id=xxx; session-token=xxx; ...",
      "created_at": "2024-01-01T12:00:00Z"
    }
  ]
}
```

**保存到数据库:**
```sql
INSERT INTO amc_cookie (host_id, cookie) VALUES (?, ?)
ON DUPLICATE KEY UPDATE cookie = VALUES(cookie);
```

### 步骤 12: 关闭浏览器（可选）

如果需要获取多个 Cookie（--count > 1），重复步骤 1-11。

使用 `browser_close` 关闭浏览器。

## 输出格式

成功获取后，输出类似：

```
Cookie 获取成功！

共获取 3 个 Cookie：

1. 邮编: 10001 (New York, NY)
   Cookie: session-id=xxx-xxx-xxx; session-token=...

2. 邮编: 90210 (Beverly Hills, CA)
   Cookie: session-id=yyy-yyy-yyy; session-token=...

3. 邮编: 60601 (Chicago, IL)
   Cookie: session-id=zzz-zzz-zzz; session-token=...

已保存到: cookies.json (或 数据库 amc_cookie 表)
```

## 错误处理

1. **页面加载超时**: 重试最多 3 次
2. **弹窗未出现**: 尝试刷新页面后重试
3. **Cookie 为空**: 记录错误，尝试下一个邮编
4. **验证码/人机验证**: 提示用户手动处理或更换 IP

## 注意事项

- 每次获取 Cookie 建议间隔 5-10 秒，避免被检测为机器人
- 使用代理可以提高成功率
- Cookie 有效期通常为 14 天左右
- 建议定期刷新 Cookie

## 项目集成

获取的 Cookie 可以用于 amazon-crawler 项目：

1. 文件方式：程序启动时读取 `cookies.json`
2. 数据库方式：程序从 `amc_cookie` 表读取 Cookie
