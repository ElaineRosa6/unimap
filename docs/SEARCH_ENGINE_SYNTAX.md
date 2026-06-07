# 搜索引擎语法基准文档

> 用途：UQL → 各引擎查询语法翻译的**事实基准**。适配器 `internal/adapter/*.go` 的 `buildCondition` / `translateNode` 必须以本文档为准。
> 维护：语法随引擎版本变化，修改适配器前请回看官方文档核对。
> 最近核对：2026-06-07（Hunter/Shodan 官方页被网络策略拦截，经多来源交叉确认；ZoomEye/Quake 以用户提供的官方语法为准）。

---

## 1. 通用约定对比

| 引擎 | 字段分隔符 | 值引号 | AND | OR | NOT | 精确匹配 | 分组 `()` | 区间 |
|------|-----------|--------|-----|----|----|---------|----------|------|
| **FOFA** | `=` | `"..."` 必须 | `&&` | `\|\|` | `!=` | `==` | ✅ | — |
| **Hunter** | `=` | `"..."` 必须 | `&&` | `\|\|` | `!=` | `==` | ✅ | `>` `<`（如 `ip.port_count>"2"`） |
| **Quake** | `:` | `"..."` 必须 | `AND` | `OR` | `NOT` | — | ✅ | `[N TO M]` |
| **Shodan** | `:` | 仅含空格时加 `"..."` | 空格 | `,`（同字段逗号） | `-字段:` | 隐式 | ❌ 不支持 | — |
| **ZoomEye(v2)** | `=` | `"..."` 或 `'...'` | `&&` | `\|\|` | `!=` | `==` | ✅ | `after=`/`before=` |

**最易翻译错的四点：**
1. 分隔符两两分裂：FOFA / Hunter / ZoomEye 用 `=`；Quake / Shodan 用 `:`。
2. 连接符两类：FOFA / Hunter / ZoomEye 用符号 `&&` `||`；Quake 用英文词 `AND` `OR`；Shodan 既不用词也不用符号（空格 + 逗号 + `-`）。
3. Shodan 是异类：值默认不加引号，无括号，**不支持跨字段 OR / `AND` 关键字**。
4. ZoomEye v2 已弃用 v1 的 `field:"value"` + `+`/`-` 前缀语法，改为 `field="value"` + `&&`/`||`。

---

## 2. FOFA（参考实现，当前代码正确）

- 语法：`field="value"`，`&&` `||` `!=` `==` `()`。
- 适配器 `fofa.go` 翻译正确，作为其它引擎的对照基准。

| UQL 字段 | FOFA 字段 |
|----------|-----------|
| body | `body` |
| title | `title` |
| header | `header` |
| port | `port` |
| protocol | `protocol` |
| ip | `ip` |
| country | `country` |
| region | `region` |
| city | `city` |
| asn | `asn` |
| org | `org` |
| isp | `isp` |
| domain | `domain` |
| host | `host` |
| server | `server` |
| status_code | `status_code` |
| os | `os` |
| app | `app` |
| cert | `cert` |
| url | `host` |

示例：`(port="80" || port="443") && country="CN"`

---

## 3. Hunter（奇安信鹰图）

- 语法：`field="value"`（模糊）/ `field=="value"`（精确），连接符 **`&&` `||` `!=`**，`()` 分组。
- ⚠️ Hunter 用符号连接符，**不是** `AND`/`OR` 英文词。
- 字段采用点命名空间：`web.*`、`ip.*`、`header.*`、`app.*`。

| UQL 字段 | Hunter 字段 | 备注 |
|----------|-------------|------|
| body | `web.body` | |
| title | `web.title` | |
| header | `header` | 或 `header.server` 视语义 |
| port | `port` | |
| protocol | `protocol` | |
| ip | `ip` | |
| country | `ip.country` | |
| region | `ip.province` | |
| city | `ip.city` | |
| asn | `ip.asn` | |
| org | `ip.org` | |
| isp | `ip.isp` | |
| domain | `domain` | 子域用 `domain.suffix` |
| host | `domain` | |
| server | `header.server` | ⚠️ 点号，非 `header_server` |
| status_code | `web.status_code` | |
| os | `ip.os` | ⚠️ 现代码误为 `os` |
| app | `app.name` | |
| cert | `cert` | |
| url | `web.url` | 待官方核对 |

- API 侧：查询串 base64 后传 `search` 参数（`hunter.go` 已实现）。
- 示例：`web.title="后台登录" && header.server=="Microsoft-IIS/10"`
- 来源：[hunter.qianxin.com 帮助中心](https://hunter.qianxin.com/home/helpCenter?r=8-1)（需登录核对 `web.url` 等存疑项）。

---

## 4. Quake（360 Quake）

- 语法：`field:"value"`，连接符 **`AND` `OR` `NOT`（大写英文词）**，`()` 分组，区间 `port:[50 TO 60]`。
- 当前适配器基本正确。

| UQL 字段 | Quake 字段 | 备注 |
|----------|-----------|------|
| body | `response` | |
| title | `title` | |
| header | `headers` | |
| port | `port` | |
| protocol | `service` | |
| ip | `ip` | 支持 CIDR `ip:"1.1.1.1/22"` |
| country | `country` | 中文用 `country_cn` |
| region | `province` | 中文用 `province_cn` |
| city | `city` | 中文用 `city_cn` |
| asn | `asn` | |
| org | `org` | |
| isp | `isp` | |
| domain | `domain` | |
| host | `domain` | |
| server | `server` | ⚠️ 现代码误为 `app` |
| status_code | `status_code` | |
| os | `os` | |
| app | `app` | |

- 其它专有字段：`is_ipv6`、`transport`(tcp/udp)、`hostname`、`owner`、`cert`、`icp_nature`。
- 示例：`port:"80" AND country:"CN" AND NOT service:"https"`

---

## 5. Shodan

- 语法：`filter:value`（filter 与 value 间**无空格**）。
- 组合：**AND = 空格**；**OR = 同一字段逗号** `port:80,443`；**NOT = `-filter:value`**。
- ⚠️ **不支持 `()` 分组、不支持 `AND`/`OR` 关键字、不支持跨字段 OR。**
- 值含空格须加引号：`org:"Beijing University"`；否则空格被当作 AND 拆词。

| UQL 字段 | Shodan 字段 | 备注 |
|----------|-------------|------|
| body | `http.html` | ⚠️ 现代码误为 `html` |
| title | `http.title` | ⚠️ 现代码误为 `title` |
| header | `http.html` | Shodan 无独立 header filter（暂并入 html，待定） |
| port | `port` | 多端口用逗号 `port:80,443` |
| protocol | `transport` | tcp/udp；存疑（官方 filter list 未明列） |
| ip | `ip` | 即 `net`，支持 CIDR |
| country | `country` | 2 字母国家码 |
| region | `region` | |
| city | `city` | |
| asn | `asn` | 形如 `asn:AS12345` |
| org | `org` | |
| isp | `isp` | |
| domain | `domain` | |
| host | `hostname` | ⚠️ 现代码误为 `hostnames`（复数） |
| server | `product` | Shodan 无 server filter，并入 product（待定） |
| status_code | `http.status` | ⚠️ 现代码误为 `status` |
| os | `os` | |
| app | `product` | |
| cert | `ssl` | |
| url | `hostname` | |

- 示例：`port:80,443 product:"Apache" country:US http.title:"Admin"`
- 来源：[Shodan Filter Reference](https://www.shodan.io/search/filters)、[Search Query Fundamentals](https://help.shodan.io/the-basics/search-query-fundamentals)。

### Shodan 与 UQL 布尔模型的根本差异
UQL/AST 支持任意 `()` 嵌套和跨字段 `OR`，Shodan 不支持。翻译策略二选一（见修复计划 FIX-S4）：
- **A（保守）**：仅支持 AND（空格）与单字段逗号 OR；遇到跨字段 OR 报错或降级提示。
- **B（尽力）**：保留现状的 `(a OR b)` 文本，接受 Shodan 静默忽略 → 结果不可靠（不推荐）。

---

## 6. ZoomEye（v2）

- 语法：`field="value"`（模糊）/ `field=="value"`（精确），连接符 **`&&` `||` `!=`**，`()` 分组，`*` 通配。
- ⚠️ 当前适配器产出的是 **v1（ES）语法** `+field:"value"`，与 `/v2/search` 端点不匹配，必须整体改写。

| UQL 字段 | ZoomEye v2 字段 | 备注 |
|----------|-----------------|------|
| body | `http.body` | ⚠️ 现代码误为 `site` |
| title | `title` | |
| header | `http.header` | ⚠️ 现代码误为 `headers` |
| port | `port` | |
| protocol | `service` | |
| ip | `ip` | CIDR 用 `cidr` |
| country | `country` | |
| region | `subdivisions` | |
| city | `city` | |
| asn | `asn` | |
| org | `org` | 或 `organization` |
| isp | `isp` | |
| domain | `domain` | ⚠️ 现代码误为 `hostname` |
| host | `hostname` | |
| server | `http.header.server` | ⚠️ 现代码误为 `app` |
| status_code | `http.header.status_code` | ⚠️ 现代码误为 `site` |
| os | `os` | |
| app | `app` | |
| cert | `ssl` | |
| device | `device` | |
| banner | `banner` | |

- 专有：`iconhash`、`filehash`、`after=`/`before=`（须配合其它条件）、`is_honeypot`。
- 示例：`service="ssh" && country="CN" && port=80`
- API 侧：查询串 base64url 后传 `qbase64`（`zoomeye.go` 已实现）。

---

## 7. UQL 操作符 → 引擎映射速查

| UQL 操作符 | FOFA | Hunter | Quake | Shodan | ZoomEye |
|-----------|------|--------|-------|--------|---------|
| `=` | `=` | `=` | `:` | `:` | `=` |
| `==` | `==` | `==` | `:`（无精确） | `:` | `==` |
| `!=` `<>` | `!=` | `!=` | `NOT ...:` | `-...:` | `!=` |
| `>` `<` `>=` `<=` | 暂不支持¹ | `>` `<`² | `[N TO M]`³ | 暂不支持¹ | `after`/`before`³ |
| AND | `&&` | `&&` | `AND` | 空格 | `&&` |
| OR | `\|\|` | `\|\|` | `OR` | `,`（同字段） | `\|\|` |
| IN [a,b] | `(...||...)` | `(...||...)` | `(...OR...)` | 同字段 `f:a,b` | `(... ...)` |

> ¹ 当前所有适配器对 `>`/`<` 等比较操作符降级为等值匹配（静默错误），见修复计划 FIX-P1。
> ² Hunter 仅 `ip.port_count` 等数值字段支持。
> ³ 仅特定字段（端口/时间）支持，非通用。
