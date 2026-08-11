# 网络空间测绘平台查询语法与 API 文档合集

> **整理/复核日期**：2026-08-10  
> **包含平台**：DayDayMap、FOFA、Hunter（奇安信鹰图）、Quake、Shodan、Censys、ZoomEye

本合集面向合法的互联网资产管理、安全研究、暴露面分析和已授权安全测试。每个平台独立成文，尽量避免把某个平台的字段/逻辑误套到另一平台。

## 文件清单

| 文件 | 当前主 API / 语法 | 核验状态 |
|---|---|---|
| `DayDayMap查询语法及API文档.md` | DayDayMap Search API | 已按官方开发者文档复核修订 |
| `FOFA查询语法及API文档.md` | `/api/v1/search/all` + FOFA Query | 官方在线 API 5.5.0 |
| `Hunter查询语法及API文档.md` | Hunter Query + Open API | **部分受登录/动态帮助中心限制，需账户内复核** |
| `Quake查询语法及API文档.md` | Quake API v3 + `field:value` | 360 官方 CLI 源码交叉核验 |
| `Shodan查询语法及API文档.md` | Shodan REST API + `filter:value` | 官方 Developer API |
| `Censys查询语法及API文档.md` | Platform API v3 + CenQL | 官方 v3 文档；Legacy v2 单列迁移 |
| `ZoomEye查询语法及API文档.md` | API v2 + ZoomEye Query | 官方 API v2 + Knownsec 官方 SDK |

## 重要版本提示

### Censys
2026 年新项目应优先使用：

```text
POST https://api.platform.censys.io/v3/global/search/query
```

和 CenQL。旧 `/v2/hosts/search` 只作为 Legacy 迁移参考。

### ZoomEye
当前官方 API Reference 展示 v2：

```text
POST https://api.zoomeye.ai/v2/search
```

官方历史 SDK 文档中也曾使用 `api.zoomeye.org`，新项目以当前在线 Reference 为准。

### Quake
360 官方 `quake_rs` 当前源码可确认：

```text
POST /api/v3/search/quake_service
POST /api/v3/search/quake_host
POST /api/v3/scroll/quake_service
POST /api/v3/scroll/quake_host
GET  /api/v3/user/info
```

认证 Header：

```text
X-QuakeToken
```

### Shodan
Filter 集合是动态的。不要只依赖静态速查表，程序可以调用：

```text
GET /shodan/host/search/filters
GET /shodan/host/search/facets
GET /shodan/host/search/tokens
```

### Hunter
Hunter 的详细官方帮助/API 页面需要登录并由前端动态加载。本合集没有用第三方博客“伪造”一个看似完整的官方字段表；对于无法公开逐字段核验的 API 参数，文档均标记了“需账户内复核”。

## 统一集成建议

如果要把这些平台接入一个 Agent / 资产检索系统，建议不要直接复用查询字符串，而是做一层统一抽象：

```text
Unified Query
    │
    ├── FOFA Adapter
    ├── Hunter Adapter
    ├── Quake Adapter
    ├── Shodan Adapter
    ├── Censys Adapter
    ├── ZoomEye Adapter
    └── DayDayMap Adapter
```

统一层只表达：

```text
IP
CIDR
Domain
Port
Country
Organization
Protocol
Product
Title
Body
Header
Certificate
Time
```

然后由每个平台 Adapter 负责：

```text
字段映射
逻辑运算符
编码（Base64/Base64URL/原文）
认证
分页
结果字段归一化
权限/积分错误
```

这样能最大限度避免不同平台“字段看起来相似，但语义不同”导致错误结果。

## 批量 API 的内存安全

对于可能返回大量资产的平台：

- 使用分页/Scroll/Token 分页逐页处理；
- 尽量指定 `fields`，减少响应体；
- 不要把全部页面先 append 到一个内存数组；
- 边查询边写 JSONL/SQLite/数据库；
- 对 `body`、`header`、`banner`、`ssl/cert` 等大字段尤其谨慎；
- 设置连接超时、读取超时和最大响应大小；
- API Key 使用 Secret/环境变量。

## 官方来源入口

- DayDayMap：https://www.daydaymap.com/help/document
- FOFA：https://fofa.info/api
- Hunter：https://hunter.qianxin.com/
- Quake：https://quake.360.net/
- Quake 官方 CLI：https://github.com/360quake/quake_rs
- Shodan：https://developer.shodan.io/api
- Censys：https://docs.censys.com/
- ZoomEye：https://www.zoomeye.ai/doc
- ZoomEye 官方 SDK：https://github.com/knownsec/ZoomEye-python

---

**免责声明**：会员权益、积分、频率限制、最大可查看条数、历史数据范围和部分字段权限属于动态策略。本合集不把 2026-08-10 的商业套餐数字当作永久接口契约；遇到权限/额度差异时，应以当前账号页面和官方最新文档为准。
