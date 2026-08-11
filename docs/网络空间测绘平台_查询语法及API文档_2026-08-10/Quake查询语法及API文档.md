# Quake 360 网络空间测绘 — 查询语法及 API 使用明细

> **复核日期**：2026-08-10  
> **官方平台**：https://quake.360.net  
> **主要核验源**：360quake 官方 `quake_rs` 命令行客户端源码与 README  
> **说明**：Quake Web 帮助中心较多内容由前端动态加载；本文对 API Endpoint、Body 结构和查询示例优先依据 360 官方开源客户端实际实现核对。

---

## 1. 查询语法基础

Quake 常见形式：

```text
field:value
```

字符串可使用双引号：

```text
domain:"*.360.cn"
app:"Apache"
response:"keyword"
```

组合查询支持布尔逻辑：

```text
AND
OR
NOT
```

官方 CLI 示例中也可见小写 `and` / `not`。

---

## 2. 官方客户端已验证的查询字段/示例

| 字段 | 示例 | 说明 |
|---|---|---|
| `ip` | `ip:"203.0.113.10"` | IP |
| `domain` | `domain:"*.example.com"` | 域名/子域 |
| `port` | `port:443` | 端口 |
| `app` | `app:"Apache"` | 应用/指纹 |
| `response` | `response:"keyword"` | 响应内容 |
| `country_cn` | `country_cn:"中国"` | 中文国家 |
| `province_cn` | `province_cn:"河南省"` | 中文省份 |
| `os` | `os:"Linux"` | 操作系统 |

官方 CLI README 中的组合例：

```text
country_cn:"中国" and not province_cn:"台湾省" and app:"Apache"
```

域名查询：

```text
domain:"*.360.cn"
```

OR：

```text
ip:"203.0.113.10" OR domain:"example.com"
```

---

## 3. 结果中的常见字段

从官方客户端当前解析逻辑可以确认常见结构包含：

```text
ip
port
time
location.country_cn
location.province_cn
location.city_cn
location.owner
service.name
service.response
service.cert
service.http.title
service.http.host
service.http.body
service.http.response_headers
service.tls...
components[].product_name_cn
components[].version
```

实际字段随协议和数据类型不同。

---

## 4. API Key

官方 CLI 的初始化方式：

```bash
quake init YOUR_API_KEY
```

API Key 在 Quake 个人中心获取。

自行调用 REST 时建议使用环境变量：

```bash
export QUAKE_API_KEY='YOUR_KEY'
```

---

## 5. REST API：Service Search

### 5.1 Endpoint

360 官方客户端当前源码：

```text
POST https://quake.360.net/api/v3/search/quake_service
```

### 5.2 请求 Body

官方客户端 `Service` / 构造逻辑中包含：

```json
{
  "query": "port:443",
  "start": 0,
  "size": 10,
  "ignore_cache": false,
  "latest": false,
  "start_time": "YYYY-MM",
  "end_time": "YYYY-MM",
  "ip_list": [],
  "shortcuts": []
}
```

其中：

| 字段 | 说明 |
|---|---|
| `query` | Quake 查询语句 |
| `start` | 起始偏移 |
| `size` | 返回数量 |
| `ignore_cache` | 忽略缓存 |
| `latest` | 最新数据模式 |
| `start_time` | 数据开始时间 |
| `end_time` | 数据结束时间 |
| `ip_list` | IP 列表模式 |
| `shortcuts` | Quake 内部过滤快捷项 |

> `shortcuts` 的 ID 属于平台内部实现，不建议业务代码直接硬编码；应优先使用官方 UI/客户端暴露的功能。

### 5.3 CLI 对 size 的约束

官方 CLI README 显示普通查询的：

```text
size 最大 100
默认 10
start 默认 0
```

服务端不同套餐或接口可能另有约束，以实际响应为准。

---

## 6. REST API：Host Search

官方客户端还使用：

```text
POST https://quake.360.net/api/v3/search/quake_host
```

Body：

```json
{
  "query": "ip:203.0.113.10",
  "start": 0,
  "size": 10,
  "ignore_cache": false
}
```

用于以 Host 聚合视角获取 IP 下的端口和服务。

---

## 7. Scroll API

官方客户端当前源码使用：

```text
POST https://quake.360.net/api/v3/scroll/quake_service
POST https://quake.360.net/api/v3/scroll/quake_host
```

Scroll 请求中会使用：

```text
pagination_id
```

用于持续分页。

示意：

```json
{
  "query": "port:443",
  "size": 100,
  "ignore_cache": false,
  "pagination_id": ""
}
```

首次响应取得 `meta.pagination_id` 后继续提交。

---

## 8. API Header

360 官方 `quake_rs` 当前源码的 `header()` 实现明确使用：

```http
X-QuakeToken: YOUR_API_KEY
Content-Type: application/json
```

如果未来服务端调整认证方式，应以官方当前帮助页和最新官方客户端源码为准。

---

## 9. CURL 示例

```bash
curl 'https://quake.360.net/api/v3/search/quake_service' \
  -H "X-QuakeToken: $QUAKE_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "port:443",
    "start": 0,
    "size": 10,
    "ignore_cache": false,
    "latest": false,
    "start_time": "",
    "end_time": "",
    "ip_list": [],
    "shortcuts": []
  }'
```

---

## 10. Python 示例

```python
import os
import requests

payload = {
    "query": 'country_cn:"日本" AND port:443',
    "start": 0,
    "size": 10,
    "ignore_cache": False,
    "latest": False,
    "start_time": "",
    "end_time": "",
    "ip_list": [],
    "shortcuts": [],
}

r = requests.post(
    "https://quake.360.net/api/v3/search/quake_service",
    headers={
        "X-QuakeToken": os.environ["QUAKE_API_KEY"],
        "Content-Type": "application/json",
    },
    json=payload,
    timeout=30,
)
r.raise_for_status()
print(r.json())
```

---

## 11. 返回模型

官方客户端按如下结构读取 Service Search：

```text
code
message
data[]
meta.pagination.count
meta.pagination.total
```

Scroll：

```text
data[]
meta.pagination_id
```

成功码在当前客户端中按：

```text
code == 0
```

处理。

每条 `data` 的字段由资产类型决定。

---

## 12. 时间与过滤能力

官方 CLI 支持：

```text
--time_start
--time_end
```

并支持若干 UI/快捷过滤能力，例如：

- 排除 CDN；
- 蜜罐相关过滤；
- 最新数据；
- 无效请求过滤；
- 数据去重。

这些快捷能力在官方客户端中部分映射为内部 `shortcuts` ID。**不要复制内部 ID 作为长期 API 契约**。

---

## 13. 工程注意事项

1. `quake_service` 与 `quake_host` 是不同返回模型。
2. 普通查询使用 `start/size`；大量翻页优先研究官方 Scroll 接口。
3. `pagination_id` 必须按响应动态传递。
4. 不要把 `shortcuts` 内部 Object ID 写死。
5. 结果中 `response/body/header/cert` 可能很大，批量任务应边查边写磁盘，不要把所有页累积在内存。
6. API Key 只放环境变量/Secret。
7. Web 帮助页、会员权益和时间范围会变化；遇到权限差异以当前账号页面为准。

---

## 14. 官方核验源

- Quake：https://quake.360.net
- 360quake 官方 CLI：https://github.com/360quake/quake_rs
- 官方客户端当前源码中可核验：
  - `/api/v3/search/quake_service`
  - `/api/v3/search/quake_host`
  - `/api/v3/scroll/quake_service`
  - `/api/v3/scroll/quake_host`

> 本文档用于合法资产管理、安全研究和已授权测试。
