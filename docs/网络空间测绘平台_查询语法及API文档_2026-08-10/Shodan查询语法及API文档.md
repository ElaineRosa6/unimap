# Shodan — 查询语法及 API 使用明细

> **复核日期**：2026-08-10  
> **官方 REST API**：https://developer.shodan.io/api  
> **说明**：Shodan 的搜索 Filter 会持续变化。本文列出稳定的语法规则与常用过滤器；**完整、实时过滤器列表应调用官方 `/shodan/host/search/filters` 接口获取**。

---

## 1. 查询模型

Shodan 查询由两部分组成：

```text
自由文本 filter:value filter:value
```

例如：

```text
nginx country:JP port:443
```

未加过滤器的普通字符串主要搜索服务 Banner 的 `data` 字段。

### 1.1 Filter 规则

```text
filter:value
```

冒号两侧不要加入空格。

值含空格时使用引号：

```text
org:"Amazon.com, Inc."
```

多个过滤器并列时通常表示同时满足：

```text
country:JP port:443 product:nginx
```

---

## 2. 常用搜索过滤器

> Shodan 官方提供动态过滤器列表，本表是常用子集，不应视为永久全集。

| Filter | 用途 | 示例 |
|---|---|---|
| `ip` | IP | `ip:8.8.8.8` |
| `net` | CIDR/网段 | `net:203.0.113.0/24` |
| `port` | 端口 | `port:443` |
| `transport` | TCP/UDP | `transport:udp` |
| `country` | 国家代码 | `country:JP` |
| `city` | 城市 | `city:Tokyo` |
| `org` | 组织 | `org:"Example Corp"` |
| `asn` | ASN | `asn:AS13335` |
| `hostname` | 主机名/域名 | `hostname:example.com` |
| `os` | 操作系统 | `os:Linux` |
| `product` | 产品 | `product:nginx` |
| `version` | 版本 | `version:1.24.0` |
| `cpe` | CPE | `cpe:cpe:/a:nginx:nginx` |
| `http.title` | HTTP 标题 | `http.title:"Example"` |
| `http.html` | HTML 内容 | `http.html:"login"` |
| `http.favicon.hash` | favicon mmh3 hash | `http.favicon.hash:HASH` |
| `ssl.cert.subject.cn` | 证书 Subject CN | `ssl.cert.subject.cn:example.com` |
| `ssl.cert.issuer.cn` | 证书 Issuer CN | `ssl.cert.issuer.cn:"Let's Encrypt"` |
| `ssl.cert.fingerprint` | 证书指纹 | `ssl.cert.fingerprint:...` |
| `vuln` | 漏洞标识 | `vuln:CVE-YYYY-NNNN` |
| `tag` | Shodan 标签 | `tag:cloud` |
| `before` | 指定日期前 | `before:2026-01-01` |
| `after` | 指定日期后 | `after:2026-01-01` |

> 某些 Filter 仅在特定数据、账户或计划中有意义。权威列表见 API 的 `search/filters`。

---

## 3. 查询示例

```text
# 日本、443 端口、nginx
product:nginx country:JP port:443

# 指定组织
org:"Example Corp"

# 指定域名
hostname:example.com

# HTTP 标题
http.title:"Example"

# 证书主体域名
ssl.cert.subject.cn:example.com

# 时间过滤
product:nginx after:2026-01-01
```

---

## 4. API Key

Shodan API 使用 API Key。官方说明创建账号即可获得 API Key，但**具体 API 权限和可用额度取决于计划**。

建议：

```bash
export SHODAN_API_KEY='YOUR_KEY'
```

---

## 5. 搜索 API

### 5.1 搜索

```text
GET https://api.shodan.io/shodan/host/search
```

参数：

| 参数 | 必填 | 说明 |
|---|---:|---|
| `key` | 是 | API Key |
| `query` | 是 | Shodan 查询语句 |
| `facets` | 否 | 聚合字段，逗号分隔 |
| `page` | 否 | 页码；每页 100 条 |
| `minify` | 否 | 是否裁剪大字段，默认 true |
| `fields` | 否 | 只返回指定字段，逗号分隔 |

CURL：

```bash
curl -G 'https://api.shodan.io/shodan/host/search' \
  --data-urlencode "key=$SHODAN_API_KEY" \
  --data-urlencode 'query=product:nginx country:JP' \
  --data-urlencode 'facets=country' \
  --data-urlencode 'page=1'
```

Python（官方 SDK）：

```python
import os
from shodan import Shodan

api = Shodan(os.environ["SHODAN_API_KEY"])
result = api.search("product:nginx country:JP")
print(result["total"])
for match in result["matches"]:
    print(match["ip_str"], match["port"])
```

### 5.2 计数接口

```text
GET https://api.shodan.io/shodan/host/count
```

使用与搜索接口相同的查询语法，但主要用于获取总数和 Facets，不返回完整主机结果。官方文档说明该接口不消耗查询积分。

### 5.3 获取当前全部 Filter

```text
GET https://api.shodan.io/shodan/host/search/filters
```

示例：

```bash
curl -G 'https://api.shodan.io/shodan/host/search/filters' \
  --data-urlencode "key=$SHODAN_API_KEY"
```

### 5.4 获取当前 Facet

```text
GET https://api.shodan.io/shodan/host/search/facets
```

### 5.5 解析查询语句

```text
GET https://api.shodan.io/shodan/host/search/tokens
```

参数：

```text
key
query
```

这对于程序自动检查用户输入的 Shodan 查询非常有用。

### 5.6 单个主机查询

Shodan REST API 还提供 Host Lookup，用于获取指定 IP 的当前/历史服务信息。具体路径和可选参数应从当前 REST API 页面读取，因为历史数据能力与计划有关。

---

## 6. 搜索 API 返回结构

典型结构：

```json
{
  "total": 123,
  "matches": [
    {
      "ip_str": "203.0.113.10",
      "port": 443,
      "transport": "tcp",
      "data": "...",
      "location": {},
      "_shodan": {}
    }
  ],
  "facets": {}
}
```

具体 Banner 字段随协议变化；HTTP、TLS、DNS、SSH 等会出现不同的协议对象。

---

## 7. Query Credits

官方当前搜索接口说明：

- 查询中包含 Filter 时，可能消耗 Query Credit。
- 访问第 1 页之后的结果会消耗 Query Credit。
- 每继续获取 100 条结果会按官方规则扣减额度。
- `/shodan/host/count` 用于计数，不消耗 Query Credit。

额度属于动态商业策略，不建议在程序中硬编码套餐数量。

---

## 8. Streaming API

Shodan 另有 Streaming API：

```text
https://stream.shodan.io
```

它用于接收 Shodan 正在采集的实时数据流，不等同于可检索的 REST Search API。部分实时流属于高等级/Enterprise 能力。

---

## 9. 工程注意事项

1. 完整 Filter 列表应通过 `/shodan/host/search/filters` 动态读取。
2. 对用户输入先调用 `/search/tokens` 可提前发现解析问题。
3. 结果字段依协议而异，不要假设每条 Banner 都包含 HTTP/TLS 字段。
4. 分页固定按 100 条一页处理。
5. API Key 不要写入仓库或前端代码。
6. Query Credit/Streaming 权限应运行时处理，不要根据旧教程固定判断。

---

## 10. 官方入口

- REST API：https://developer.shodan.io/api
- API Requirements：https://developer.shodan.io/api/requirements
- Streaming API：https://developer.shodan.io/api/stream

> 本文档用于合法资产管理、安全研究和已授权测试。
