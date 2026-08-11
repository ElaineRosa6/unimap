# ZoomEye — 查询语法及 API v2 使用明细

> **复核日期**：2026-08-10  
> **官方 API Reference**：https://www.zoomeye.ai/doc  
> **官方 SDK/参考**：Knownsec / ZoomEye-python  
> **API 版本**：v2  
> **说明**：ZoomEye 官方域名在不同资料中存在 `.ai` / `.org` 展示差异。当前官方 API 页面示例使用 `https://api.zoomeye.ai/v2/search`；官方 SDK 历史文档也出现 `api.zoomeye.org`。新项目优先使用当前官方 API Reference 给出的 `.ai` 地址。

---

## 1. 查询语法规则

ZoomEye 搜索支持：

- 全局字符串检索；
- 字段检索；
- 模糊与精确匹配；
- AND / OR / NOT；
- 括号优先级；
- 通配符。

字符串建议使用引号。

### 1.1 运算符

| 运算符 | 含义 | 示例 |
|---|---|---|
| `=` | 包含/匹配关键词 | `title="knownsec"` |
| `==` | 完全精确匹配，严格区分大小写 | `title=="KnownSec"` |
| `!=` | 排除 | `country="CN" && subdivisions!="beijing"` |
| `&&` | AND | `device="router" && port=443` |
| `||` | OR | `service="ssh" || service="http"` |
| `()` | 优先级 | `(port=80 || port=443) && country="JP"` |
| `*` | 通配/模糊 | `title="google*"` |

---

## 2. 地理位置

| 语法 | 说明 |
|---|---|
| `country="CN"` | 国家/地区 |
| `subdivisions="beijing"` | 省/州/行政区 |
| `city="changsha"` | 城市 |

支持的文字/代码形式取决于字段，工程上建议优先使用标准国家代码。

---

## 3. IP、域名与组织

| 语法 | 说明 |
|---|---|
| `ip="8.8.8.8"` | IPv4 |
| `ip="2600:3c00::..."` | IPv6 |
| `cidr="52.2.254.36/24"` | CIDR |
| `hostname="example.com"` | 主机名 |
| `domain="example.com"` | 域名/子域 |
| `org="Example University"` | 组织 |
| `organization="Example University"` | 组织别名 |
| `isp="Example ISP"` | ISP |
| `asn=13335` | ASN |
| `port=443` | 端口 |

---

## 4. Web、Banner 与 HTTP

| 语法 | 说明 |
|---|---|
| `banner="OpenSSH"` | 非 HTTP Banner |
| `http.header="nginx"` | HTTP Header |
| `http.header_hash="..."` | Header Hash |
| `http.header.server="Nginx"` | Server |
| `http.header.version="1.2"` | Server/组件版本 |
| `http.header.status_code="200"` | HTTP 状态码 |
| `http.body="document"` | HTML Body |
| `http.body_hash="..."` | Body Hash |
| `title="Cisco"` | HTML Title |

---

## 5. 指纹、系统与设备

| 语法 | 说明 |
|---|---|
| `app="Cisco ASA SSL VPN"` | 应用指纹 |
| `service="ssh"` | 服务协议 |
| `device="router"` | 设备类型 |
| `os="RouterOS"` | 操作系统 |
| `industry="金融"` | 行业 |
| `product="Cisco"` | 产品/组件 |
| `protocol="TCP"` | 传输层协议 |
| `is_honeypot="True"` | 蜜罐标识 |

---

## 6. ICP

```text
icp.number="京ICP备XXXXXXXX号"
icp.name="示例企业"
```

---

## 7. TLS/SSL

官方当前语法包含较丰富证书字段，例如：

```text
ssl="google"
ssl.cert.fingerprint="FINGERPRINT"
ssl.chain_count=3
ssl.cert.alg="SHA256-RSA"
ssl.cert.issuer.cn="Example CA"
ssl.cert.pubkey.rsa.bits=2048
ssl.cert.pubkey.ecdsa.bits=256
ssl.cert.pubkey.type="RSA"
ssl.cert.serial="SERIAL"
ssl.cipher.bits="128"
ssl.cipher.name="TLS_AES_128_GCM_SHA256"
ssl.cipher.version="TLSv1.3"
ssl.version="TLSv1.3"
ssl.cert.subject.cn="example.com"
ssl.jarm="JARM_VALUE"
ssl.ja3s="JA3S_VALUE"
```

---

## 8. 时间

时间过滤器需与其他条件组合：

```text
after="2026-01-01" && port=443
before="2026-08-01" && service="https"
```

---

## 9. Icon / File Hash

```text
iconhash="MD5_OR_MMH3_VALUE"
filehash="FILE_HASH"
```

ZoomEye 的 `iconhash` 可用于图标关联搜索；不同值可能采用 MD5 或 mmh3 形式，以平台生成/展示的值为准。

---

## 10. API 认证

ZoomEye API v2 使用请求头：

```http
API-KEY: YOUR_API_KEY
```

API Key 可从个人资料获取。

建议：

```bash
export ZOOMEYE_API_KEY='YOUR_KEY'
```

---

## 11. 用户信息接口

```text
POST /v2/userinfo
```

当前主机：

```text
https://api.zoomeye.ai/v2/userinfo
```

示例：

```bash
curl -X POST 'https://api.zoomeye.ai/v2/userinfo' \
  -H "API-KEY: $ZOOMEYE_API_KEY"
```

返回包含账号、订阅与可用积分等信息。具体字段/权益随账户计划变化。

---

## 12. 资产搜索 API

### 12.1 Endpoint

```text
POST https://api.zoomeye.ai/v2/search
```

### 12.2 参数

| 参数 | 必填 | 说明 |
|---|---:|---|
| `qbase64` | 是 | 查询语句 Base64 |
| `fields` | 否 | 返回字段，逗号分隔；官方默认 `ip,port,domain,update_time` |
| `sub_type` | 否 | 数据类型：`v4`、`v6`、`web`；官方文档默认值需以当前页面为准 |
| `page` | 否 | 页码，默认 1 |
| `pagesize` | 否 | 每页条数；官方当前最大 10,000 条/页 |
| `facets` | 否 | 聚合字段 |
| `ignore_cache` | 否 | 是否忽略缓存；权限受计划限制 |

当前官方支持的 Facets：

```text
country
subdivisions
city
product
service
device
os
port
```

### 12.3 Base64

```python
import base64

query = 'country="JP" && port=443'
qbase64 = base64.b64encode(query.encode("utf-8")).decode("ascii")
```

### 12.4 CURL

```bash
QUERY='country="JP" && port=443'
QBASE64="$(printf '%s' "$QUERY" | base64 | tr -d '\n')"

curl 'https://api.zoomeye.ai/v2/search' \
  -H "API-KEY: $ZOOMEYE_API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{
    \"qbase64\": \"$QBASE64\",
    \"page\": 1,
    \"pagesize\": 100,
    \"fields\": \"ip,port,domain,service,title,update_time\"
  }"
```

### 12.5 Python

```python
import os
import base64
import requests

query = 'country="JP" && port=443'
payload = {
    "qbase64": base64.b64encode(query.encode()).decode(),
    "page": 1,
    "pagesize": 100,
    "fields": "ip,port,domain,service,title,update_time",
}

r = requests.post(
    "https://api.zoomeye.ai/v2/search",
    headers={"API-KEY": os.environ["ZOOMEYE_API_KEY"]},
    json=payload,
    timeout=30,
)
r.raise_for_status()
print(r.json())
```

---

## 13. 返回结构

官方示例：

```json
{
  "code": 60000,
  "message": "success",
  "query": "title=\"cisco vpn\"",
  "total": 123,
  "data": [
    {
      "ip": "203.0.113.10",
      "port": 443,
      "domain": "example.com",
      "service": "https",
      "update_time": "2026-08-01T00:00:00"
    }
  ],
  "facets": {}
}
```

常见字段包括：

```text
url
ip
domain
hostname
os
port
service
title
version
device
rdns
product
header
header_hash
body
body_hash
banner
update_time
ssl.jarm
ssl.ja3s
continent.name
country.name
province.name
city.name
isp.name
organization.name
asn
protocol
honeypot
ssl
primary_industry
sub_industry
rank
```

部分字段需要更高等级订阅。

---

## 14. 官方 Python SDK

Knownsec 官方 `ZoomEye-python` SDK 提供：

```python
search(
    dork,
    qbase64='',
    page=1,
    pagesize=20,
    sub_type='all',
    fields='',
    facets=''
)
```

CLI/SDK 可以作为 API 参数和返回模型的辅助参考，但若 SDK README 与当前在线 API Reference 冲突，应以在线官方 API Reference 为准。

---

## 15. 工程注意事项

1. 使用 UTF-8 后再 Base64。
2. 新项目优先 `api.zoomeye.ai` 当前文档地址。
3. `pagesize` 最大值很大，实际程序不要盲目请求 10,000 条并在内存中无限累积。
4. 推荐流式/逐页落盘，尤其是字段包含 `body`、`header`、`ssl` 时。
5. `fields` 可以显著减少返回体和内存占用。
6. 权益字段、历史数据、忽略缓存、行业字段等受套餐影响。
7. API Key 只放 Secret/环境变量。

---

## 16. 官方入口

- API Reference：https://www.zoomeye.ai/doc
- 官方 SDK：https://github.com/knownsec/ZoomEye-python

> 本文档用于合法资产管理、安全研究和已授权测试。
