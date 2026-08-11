# Censys Platform — CenQL 查询语法及 API v3 使用明细

> **复核日期**：2026-08-10  
> **主接口**：Censys Platform API v3  
> **查询语言**：Censys Query Language（CenQL）  
> **重要**：2026 年应优先使用 Platform API v3。旧 Search API v2 属于 Legacy，本文只在附录中保留迁移说明。  
> **官方文档**：https://docs.censys.com/

---

## 1. 2026 年版本选择

新的 API Base URL：

```text
https://api.platform.censys.io/v3/
```

主要命名空间：

```text
https://api.platform.censys.io/v3/global/
https://api.platform.censys.io/v3/collections/
https://api.platform.censys.io/v3/threat-hunting/
```

Legacy Search 常见接口：

```text
GET /v2/hosts/search
```

对应的新全局搜索接口：

```text
POST /v3/global/search/query
```

不要再为新项目围绕 `/v2/hosts/search` 设计数据层。

---

## 2. CenQL 基础

### 2.1 全文搜索

```text
"example.com"
```

### 2.2 字段查询

基本形式：

```text
field operator value
```

示例：

```text
host.services.protocol = "SSH"
host.services.port = 443
```

### 2.3 常用操作符

| 操作符 | 含义 |
|---|---|
| `:` | 匹配/包含；字符串通常为不区分大小写的 tokenized 查询 |
| `=` | 精确相等；字符串精确比较具有更严格语义 |
| `=~` | 正则表达式匹配 |
| `>` | 大于 |
| `>=` | 大于等于 |
| `<` | 小于 |
| `<=` | 小于等于 |
| `:*` | 字段存在/非空语义 |

布尔：

```text
and
or
not
```

布尔关键词大小写不敏感。

### 2.4 分组

```text
(host.services.port = 80 or host.services.port = 443)
and host.location.country = "Japan"
```

### 2.5 Nested 查询

这是 Censys Platform 数据模型中非常重要的一点。对于同一服务对象上的多个约束，可使用：

```text
host.services: (
  port = 22
  and protocol = "SSH"
)
```

它比把不同服务层级字段简单并列更能表达“同一个 service 同时满足”的语义。

### 2.6 时间

绝对时间：

```text
host.services.scan_time > "2026-01-01"
```

相对时间：

```text
host.services.scan_time > "now-1d"
```

### 2.7 CIDR

```text
host.ip: "203.0.113.0/24"
```

---

## 3. 常见字段与查询思路

Censys v3 的数据模型比 Legacy 更结构化。常见根对象包括 Host、Web Property、Certificate 等。

典型 Host 字段思路：

```text
host.ip
host.services.port
host.services.transport_protocol
host.services.protocol
host.services.software.product
host.services.software.vendor
host.services.software.version
host.services.scan_time
```

例：

```text
# SSH
host.services.protocol = "SSH"

# 443
host.services.port = 443

# 同一个服务为 SSH 且端口不是 22
host.services: (protocol = "SSH" and not port = 22)

# Nginx
host.services.software.product: "nginx"

# 指定网段
host.ip: "203.0.113.0/24"
```

---

## 4. CenQL Alias

Censys 提供一些别名以简化常见查询。官方当前文档列出的别名包括（集合可能继续变化）：

```text
banner
cpe
sha256
labels
product
vendor
vulns
vuln_score
threats
screenshots
sha1
org
```

例如：

```text
product: "nginx"
```

> Alias 不能在所有 nested 上下文里直接替代真实字段。做自动化系统时，建议核心逻辑使用明确的完整字段路径。

---

## 5. API 认证

Platform API v3 使用 Personal Access Token：

```http
Authorization: Bearer YOUR_PERSONAL_ACCESS_TOKEN
```

某些组织场景还可以指定 Organization ID，优先使用查询参数：

```text
organization_id=<UUID>
```

也支持：

```http
X-Organization-ID: <UUID>
```

但官方更推荐查询参数形式。

部分 API 还要求账号具备 `API Access` 角色。具体权限与计划有关。

---

## 6. 全局搜索 API

### 6.1 Endpoint

```text
POST https://api.platform.censys.io/v3/global/search/query
```

### 6.2 JSON Body

| 字段 | 必填 | 说明 |
|---|---:|---|
| `query` | 是 | CenQL |
| `fields` | 否 | 仅返回指定字段，字符串数组 |
| `page_size` | 否 | 每页条数；官方当前默认和最大均为 100 |
| `page_token` | 否 | 下一页 Token |

请求示例：

```json
{
  "query": "host.services: (protocol = \"SSH\" and not port = 22)",
  "fields": [
    "host.ip",
    "host.services.port",
    "host.services.protocol"
  ],
  "page_size": 100
}
```

### 6.3 CURL

```bash
curl 'https://api.platform.censys.io/v3/global/search/query' \
  -H "Authorization: Bearer $CENSYS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "host.services: (protocol = \"SSH\" and not port = 22)",
    "fields": ["host.ip","host.services.port","host.services.protocol"],
    "page_size": 100
  }'
```

### 6.4 Python

```python
import os
import requests

token = os.environ["CENSYS_TOKEN"]

payload = {
    "query": 'host.services: (protocol = "SSH" and not port = 22)',
    "fields": [
        "host.ip",
        "host.services.port",
        "host.services.protocol",
    ],
    "page_size": 100,
}

r = requests.post(
    "https://api.platform.censys.io/v3/global/search/query",
    headers={
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    },
    json=payload,
    timeout=30,
)
r.raise_for_status()
print(r.json())
```

### 6.5 matched_services 注意事项

官方说明：如果指定了 `fields`，但省略以下服务标识字段：

```text
host.services.port
host.services.transport_protocol
host.services.protocol
```

则响应中可能不会返回 `matched_services`。如果你的程序依赖匹配服务定位，建议保留这些字段。

---

## 7. 分页

v3 使用 Token 分页：

```text
page_token
```

而不是把 `page=2` 作为主要模型。

伪代码：

```python
token = None

while True:
    body = {"query": QUERY, "page_size": 100}
    if token:
        body["page_token"] = token

    data = post(body)
    consume(data)

    token = get_next_page_token(data)
    if not token:
        break
```

实际 Token 所在字段以当前响应 Schema 为准。

---

## 8. 聚合 API

```text
POST https://api.platform.censys.io/v3/global/search/aggregate
```

主要参数：

| 字段 | 说明 |
|---|---|
| `query` | CenQL |
| `field` | 聚合字段 |
| `number_of_buckets` | Bucket 数，官方当前范围 1–2000 |
| `filter_by_query` | 是否仅统计满足查询约束的字段值 |
| `count_by_level` | Nested 数据的计数层级控制 |

示例：

```json
{
  "query": "host.services.protocol = \"SSH\"",
  "field": "host.services.port",
  "number_of_buckets": 100,
  "filter_by_query": true
}
```

---

## 9. 单资产查询

Platform v3 的 Host Lookup 迁移目标为：

```text
GET /v3/global/asset/host/{ip}
```

适用于已知 IP 的资产详情读取。

---

## 10. HTTP 错误

搜索 API 当前明确列出：

| HTTP | 含义 |
|---|---|
| `200` | OK |
| `400` | Bad Request |
| `401` | Authorization Token 无效/缺失 |
| `403` | 无访问权限 |
| `422` | 输入无效 |

生产代码应同时处理 HTTP 状态码和返回 JSON 中的错误详情。

---

## 11. Credits 与权限

Censys Platform 已转向基于 Censys Credits 的 API 使用模式。免费/Starter/Enterprise 等计划的具体功能、Credits 和 API Access 权限会变化，因此：

- 不要把额度硬编码进客户端。
- 收到 403 时先检查 Role/Organization/Plan。
- 收到额度错误时读取当前账户与平台提示。
- UI 权限与 API 权限并不等价。

---

## 12. Legacy Search v2（仅迁移参考）

旧接口：

```text
GET https://search.censys.io/api/v2/hosts/search
```

旧语法属于 Censys Search Language；v3 使用 CenQL，数据模型和字段名也不同。

迁移关系：

| Legacy | Platform v3 |
|---|---|
| `GET /v2/hosts/search` | `POST /v3/global/search/query` |
| Legacy aggregate | `POST /v3/global/search/aggregate` |
| Host lookup | `GET /v3/global/asset/host/{ip}` |

**新代码不要把 Legacy 字段映射当作 v3 字段直接复用。**

---

## 13. 工程注意事项

1. 2026 年新项目使用 Platform API v3 + CenQL。
2. CenQL nested 查询是实现正确服务级关联的关键。
3. `page_size` 最大 100，批量读取应使用 `page_token`。
4. 明确指定 `fields` 可以显著减少响应体，但若需要 `matched_services` 要保留关键 service 字段。
5. Alias 适合交互搜索；数据工程更建议完整字段路径。
6. PAT 必须保存在 Secret/环境变量中。
7. 平台正在持续迭代，字段模型迁移应设计成可配置而非硬编码。

---

## 14. 官方入口

- API Reference：https://docs.censys.com/reference/v3-globaldata-search-query
- Platform Transition Guide：https://docs.censys.com/docs/platform-transition-guide-enterprise
- Documentation：https://docs.censys.com/

> 本文档用于合法资产管理、安全研究和已授权测试。
