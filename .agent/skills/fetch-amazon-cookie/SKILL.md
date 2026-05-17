---
name: fetch-amazon-cookie
description: 通过 Playwright 浏览器自动化自动获取亚马逊 Cookie，支持批量获取和多种保存方式
---

# Fetch Amazon Cookie

通过浏览器自动化工具，自动访问亚马逊美国站并获取有效的 Cookie。

## 参数

- `--count=N`: 获取 Cookie 的数量，默认 1，最大 5
- `--save-to=db|file`:
  - `db`: 保存到数据库 `amc_cookie` 表（需要指定 `--host-id`）
  - `file`: 保存到 `cookies.json` 文件（默认）
- `--host-id=N`: 数据库 `host_id`，仅 `--save-to=db` 时有效

## 美国邮编池

每次随机选择一个邮编用于设置配送地址：

| 邮编 | 城市 |
|------|------|
| 10001 | New York, NY |
| 10013 | Manhattan, NY |
| 90001 | Los Angeles, CA |
| 90210 | Beverly Hills, CA |
| 60601 | Chicago, IL |
| 60611 | Chicago Downtown, IL |
| 77001 | Houston, TX |
| 77002 | Houston Downtown, TX |
| 85001 | Phoenix, AZ |
| 19101 | Philadelphia, PA |
| 78201 | San Antonio, TX |
| 92101 | San Diego, CA |
| 75201 | Dallas, TX |
| 95101 | San Jose, CA |
| 78701 | Austin, TX |
| 32801 | Orlando, FL |
| 33101 | Miami, FL |
| 98101 | Seattle, WA |
| 80201 | Denver, CO |
| 02101 | Boston, MA |

## 执行步骤

### 步骤 1: 随机选择邮编

从上面的邮编池中随机选择一个邮编，记录选中的邮编和城市名称。

### 步骤 2: 打开亚马逊首页

使用 `browser_subagent` 工具导航到 `https://www.amazon.com`。

### 步骤 3: 等待页面加载

页面加载完成后，查找 "Deliver to" 或地址选择区域：
- 包含 "Deliver to" 文字
- 位于页面左上角导航栏
- 可能的选择器：`#nav-global-location-popover-link` 或 `#glow-ingress-block`

### 步骤 4: 点击地址区域

点击地址区域触发弹窗，弹出模态对话框。

### 步骤 5: 等待弹窗出现

等待弹窗加载，寻找 "Choose your location" 或 "Enter a US zip code" 文字。

### 步骤 6: 定位输入框

在弹窗中找到邮编输入框：
- placeholder 可能包含 "zip code" 或 "Enter ZIP code"
- 可能的 id: `GLUXZipUpdateInput`

### 步骤 7: 输入邮编

在输入框中输入步骤 1 选中的邮编。

### 步骤 8: 点击确认

点击 "Apply" 或 "Done" 按钮：
- 可能的选择器：`#GLUXZipUpdate` 或包含 "Apply" 文字的按钮

### 步骤 9: 等待页面刷新

等待 2-3 秒让页面刷新完成，确认地址区域已更新。

### 步骤 10: 获取 Cookie

在浏览器中执行 JavaScript 获取 Cookie：
```javascript
() => document.cookie
```

### 步骤 11: 保存 Cookie

根据 `--save-to` 参数选择保存方式：

**保存到文件** (`cookies.json`)：
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

**保存到数据库**：
```sql
INSERT INTO amc_cookie (host_id, cookie) VALUES (?, ?)
ON DUPLICATE KEY UPDATE cookie = VALUES(cookie);
```

### 步骤 12: 重复或关闭

- 如果 `--count > 1`，重复步骤 1-11，每次间隔 5-10 秒
- 全部完成后关闭浏览器

## 输出格式

```
Cookie 获取成功！

共获取 N 个 Cookie：

1. 邮编: 10001 (New York, NY)
   Cookie: session-id=xxx-xxx-xxx; session-token=...

已保存到: cookies.json (或 数据库 amc_cookie 表)
```

## 错误处理

1. **页面加载超时**: 重试最多 3 次
2. **弹窗未出现**: 刷新页面后重试
3. **Cookie 为空**: 记录错误，尝试下一个邮编
4. **验证码/人机验证**: 提示用户手动处理或更换 IP

## 注意事项

- 每次获取 Cookie 建议间隔 5-10 秒，避免被检测为机器人
- 使用代理可以提高成功率
- Cookie 有效期通常为 14 天左右
- 建议定期刷新 Cookie
- 获取的 Cookie 可通过文件 (`cookies.json`) 或数据库 (`amc_cookie` 表) 集成到项目中
