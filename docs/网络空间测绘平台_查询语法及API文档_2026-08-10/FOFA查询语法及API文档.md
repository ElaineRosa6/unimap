# FOFA 网络空间测绘平台 — 查询语法及 API 使用明细

> **复核日期**：2026-08-10  
> **当前官方页面版本**：FOFA Search Engine 5.5.0  
> **官方 API 文档**：https://fofa.info/api  
> **英文 API 文档**：https://en.fofa.info/api  
> **说明**：FOFA 会持续增加字段、会员能力与数据权限。本文件重点整理稳定、常用、可用于程序开发的查询/API 规则；完整字段集合应同时以登录后的 FOFA Syntax/API 页面为准。

---

## 1. 查询语法基础

FOFA 的基本形式为：

```text
字段="值"
```

典型示例：

```text
domain="example.com"
ip="1.1.1.1"
port="443"
title="nginx"
server="nginx"
product="nginx"
country="CN"
```

### 1.1 常用运算符

| 运算符 | 含义 | 示例 |
|---|---|---|
| `=` | 匹配字段值 | `title="nginx"` |
| `==` | 精确匹配（字段支持时） | `title=="Example Domain"` |
| `!=` | 排除 | `country!="CN"` |
| `&&` | AND | `port="443" && country="JP"` |
| `||` | OR | `port="80" || port="443"` |
| `()` | 分组/优先级 | `(port="80" || port="443") && country="JP"` |

> 不同字段对模糊/精确匹配的支持能力可能不同。批量程序不要假设所有字段的 `=` / `==` 行为完全一致，应以 FOFA 当前 Syntax 面板为准。

---

## 2. 常用查询字段

### 2.1 主机、网络与端口

| 字段 | 用途 | 示例 |
|---|---|---|
| `host` | 主机（IP、域名或 URL 相关主机标识） | `host="example.com"` |
| `ip` | IP | `ip="1.1.1.1"` |
| `port` | 端口 | `port="443"` |
| `protocol` | 协议 | `protocol="https"` |
| `asn` | ASN | `asn="13335"` |
| `org` | 组织 | `org="Cloudflare"` |
| `isp` | ISP | `isp="China Telecom"` |
| `country` | 国家/地区代码 | `country="JP"` |
| `country_name` | 国家/地区名称 | `country_name="Japan"` |
| `region` | 省/州/地区 | `region="Tokyo"` |
| `city` | 城市 | `city="Tokyo"` |

### 2.2 域名与 Web

| 字段 | 用途 | 示例 |
|---|---|---|
| `domain` | 域名及相关资产 | `domain="example.com"` |
| `title` | HTML 标题 | `title="Example"` |
| `header` | HTTP 响应头 | `header="nginx"` |
| `body` | HTTP 响应正文 | `body="login"` |
| `server` | HTTP Server | `server="nginx"` |
| `status_code` | HTTP 状态码 | `status_code="200"` |
| `icon_hash` | 网站图标哈希 | `icon_hash="HASH_VALUE"` |

### 2.3 指纹、组件与 Banner

| 字段 | 用途 | 示例 |
|---|---|---|
| `product` | 产品/组件指纹 | `product="nginx"` |
| `banner` | 服务 Banner | `banner="OpenSSH"` |
| `os` | 操作系统 | `os="Linux"` |

### 2.4 证书与时间

常见证书字段可用于证书主体、颁发者、序列号/哈希等检索；FOFA 字段集合可能随版本演进。工程中建议从当前 Syntax 面板复制字段名，不要根据其他平台语法自行猜测。

常用时间过滤形式：

```text
after="2026-01-01"
before="2026-08-01"
```

组合：

```text
product="nginx" && after="2026-01-01"
```

---

## 3. 查询组合示例

```text
# 日本的 443 端口资产
country="JP" && port="443"

# Nginx Web 服务
server="nginx" && (port="80" || port="443")

# 指定域名相关资产
domain="example.com"

# 指定时间以后更新的 Nginx 资产
product="nginx" && after="2026-01-01"

# 排除某国家/地区
product="nginx" && country!="CN"
```

---

## 4. API 认证

FOFA 当前 API 使用 API Key。

账号信息接口可用于检查 Key/账号状态：

```text
GET https://fofa.info/api/v1/info/my?key=YOUR_KEY
```

不要把 API Key 写死到公开仓库。推荐使用环境变量：

```bash
export FOFA_KEY='YOUR_KEY'
```

---

## 5. 搜索 API

### 5.1 接口

```text
GET https://fofa.info/api/v1/search/all
```

### 5.2 主要参数

| 参数 | 必填 | 说明 |
|---|---:|---|
| `key` | 是 | API Key |
| `qbase64` | 是 | FOFA 查询语句的 Base64 |
| `fields` | 否 | 返回字段，逗号分隔 |
| `page` | 否 | 页码 |
| `size` | 否 | 每页数量 |
| `full` | 否 | 是否使用更完整/更长时间范围数据；是否可用取决于账号权限 |

官方当前文档说明：默认返回字段为 `host,ip,port`，默认 `size=100`；单页最大值与可访问总量受当前账号/套餐约束，程序应读取实际返回并处理权限错误，不应把套餐限制硬编码。

### 5.3 qbase64 构造

原始语句：

```text
domain="example.com" && port="443"
```

Python：

```python
import base64

query = 'domain="example.com" && port="443"'
qbase64 = base64.b64encode(query.encode("utf-8")).decode("ascii")
print(qbase64)
```

### 5.4 CURL

```bash
QUERY='domain="example.com" && port="443"'
QBASE64="$(printf '%s' "$QUERY" | base64 | tr -d '\n')"

curl -G 'https://fofa.info/api/v1/search/all' \
  --data-urlencode "key=$FOFA_KEY" \
  --data-urlencode "qbase64=$QBASE64" \
  --data-urlencode 'fields=host,ip,port,protocol,title,server' \
  --data-urlencode 'page=1' \
  --data-urlencode 'size=100'
```

### 5.5 Python

```python
import os
import base64
import requests

key = os.environ["FOFA_KEY"]
query = 'domain="example.com" && port="443"'
qbase64 = base64.b64encode(query.encode()).decode()

resp = requests.get(
    "https://fofa.info/api/v1/search/all",
    params={
        "key": key,
        "qbase64": qbase64,
        "fields": "host,ip,port,protocol,title,server",
        "page": 1,
        "size": 100,
    },
    timeout=30,
)
resp.raise_for_status()
data = resp.json()
print(data)
```

---

## 6. 返回结构

搜索接口常见结构：

```json
{
  "error": false,
  "query": "...",
  "page": 1,
  "size": 100,
  "results": [
    ["https://example.com", "203.0.113.10", "443"]
  ]
}
```

当使用 `fields` 时，`results` 中每一列的顺序与 `fields` 一致。程序必须按你自己提交的字段列表解析，不要依赖示例列数。

常见错误响应会包含：

```json
{
  "error": true,
  "errmsg": "..."
}
```

---

## 7. 工程注意事项

1. **查询语句先 UTF-8，再 Base64。**
2. 对 `qbase64` 使用标准 URL 参数编码，避免 `+`、`/`、`=` 被错误处理。
3. `fields` 决定二维数组列顺序，建议在代码中保存字段列表并 `zip(fields, row)`。
4. 分页、最大结果数、历史数据深度、统计/下载权限与套餐有关，应动态处理。
5. 查询语法新增较快。罕见字段应从 FOFA 当前 Syntax 页面复制，避免把其他平台字段直接套用。
6. 不要在日志、异常栈、Git 提交中泄露 API Key。

---

## 8. 官方核验入口

- API：https://fofa.info/api
- 英文 API：https://en.fofa.info/api
- FOFA 官方 Python 客户端/组织：https://github.com/fofapro

> 本文档用于合法的资产管理、安全研究和已授权测试。平台套餐、积分、频率和数据范围属于动态策略，以账号页面及官方最新文档为准。
