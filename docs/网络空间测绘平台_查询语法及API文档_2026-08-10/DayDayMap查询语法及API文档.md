# DayDayMap 网络空间资产测绘平台 — 查询语法及 API 使用明细（复核修正版）

> **主要依据**：DayDayMap 官方开发者文档（在线复核）
> **官方文档地址**：https://www.daydaymap.com/help/document
> **官方页面标注更新时间**：2024-04-08
> **本修订版复核日期**：2026-08-10
> **交叉核验**：ProjectDiscovery `uncover` 的 DayDayMap 适配器（v1.2.1，2026-05-19 发布）
> **说明**：本文尽量只保留官方文档可确认的语法/API 行为；第三方平台归属、未经官方文档明确说明的功能描述不作为规范事实。

---

## 目录

- [1. 平台概述](#1-平台概述)
- [2. 查询语法总览](#2-查询语法总览)
- [3. 语法连接符](#3-语法连接符)
- [4. 语法分类详解](#4-语法分类详解)
  - [4.1 IP 类](#41-ip-类)
  - [4.2 域名类](#42-域名类)
  - [4.3 地理位置](#43-地理位置)
  - [4.4 ICP 备案](#44-icp-备案)
  - [4.5 自治系统 (AS)](#45-自治系统-as)
  - [4.6 Web 资产](#46-web-资产)
  - [4.7 协议](#47-协议)
  - [4.8 应用](#48-应用)
  - [4.9 组件](#49-组件)
  - [4.10 设备](#410-设备)
  - [4.11 证书](#411-证书)
  - [4.12 时间](#412-时间)
  - [4.13 漏洞指纹](#413-漏洞指纹)
  - [4.14 资产归属](#414-资产归属)
- [5. API 接口](#5-api-接口)
  - [5.1 API 认证](#51-api-认证)
  - [5.2 数据查询接口](#52-数据查询接口)
  - [5.3 请求示例](#53-请求示例)
  - [5.4 响应格式](#54-响应格式)
  - [5.5 API 错误码](#55-api-错误码)
- [6. 数据过滤](#6-数据过滤)
- [7. 兼容/别名语法速查](#7-兼容别名语法速查)
- [8. 常见问题](#8-常见问题)

---

## 1. 平台概述

DayDayMap 官方帮助页将其描述为一款“产学研一体、聚焦空间测绘科研领域的全球网络空间资产测绘平台”。本文聚焦 **查询语法与 API**，不再把首页模块名称、实时资产规模等易变化信息写成固定规范。

> 原稿中的“IPv6 资产接近 39 亿”和一组当前首页模块名称，不属于本次从官方开发者文档中稳定确认的接口/语法规范，因此从修订版移除。

---

## 2. 查询语法总览

DayDayMap 平台提供 **14 大类**、**50+ 种**查询语法（持续更新中）。部分字段提供兼容/别名写法，具体以官方语法页为准。

### 特别强调

1. **搜索字符串必须使用双引号** `""` 包裹，如 `"1.1.1.1"`、`"Ubuntu Linux"`
2. **搜索字符串不区分大小写**

### 精确查询 vs 模糊查询

| 模式 | 说明 |
|------|------|
| **模糊匹配** `=` | 检索包含关键词的资产。`=""` 表示检索字段不存在的记录 |
| **精确匹配** `==` | 检索完全匹配关键词的资产。`==""` 表示检索字段存在且值为空的记录 |
| **排除** `!=` | 检索不包含关键词的资产。`!=""` 表示检索字段值不为空的记录 |

> **规则**：如果该语法支持模糊匹配，则 `=` 为模糊、`==` 为精确；如果不支持模糊匹配，则 `=` 等同 `==`，均为精确匹配。

---

## 3. 语法连接符

| 连接符 | 含义 |
|--------|------|
| `=` | 对支持模糊匹配的字段表示模糊匹配；对不支持模糊匹配的字段等同 `==` |
| `==` | 精确匹配 |
| `!=` | 排除/不匹配 |
| `&&` | 逻辑与（AND），组合多个条件 |
| `\|\|` | 逻辑或（OR），任一条件满足即可 |
| `()` | 用于组合条件、控制逻辑优先级 |
| `>` / `>=` / `<` / `<=` | 仅用于官方明确支持范围比较的字段（如端口、时间、证书时间） |

> **修订说明**：原稿把 `:` 写成“等同 `=` 的官方连接符”，本次未在 DayDayMap 官方语法页找到这一条，因此删除。不要把 `:` 当作已确认的官方查询操作符。

> **空字符串规则**：官方 FAQ 对 `=` / `==` / `!=` 的空字符串语义有专门说明；实际使用时建议先在网页端验证目标字段，再用于批量 API 查询。

**组合查询示例**：

```text
# 查找中国境内开放了 80 端口的 Nginx 服务器
ip.country="中国" && ip.port="80" && web.server="Nginx"

# 查找使用 Apache 或 Nginx 的非 CDN 资产
(web.server="Apache" || web.server="Nginx") && ip.tag!="CDN"

# 查找证书已过期的资产
cert.is_expired="true"

# 查找 2024 年 1 月之后更新的、端口在 80-1024 之间的资产
time>="2024-01-01" && ip.port>="80" && ip.port<="1024"
```

---

## 4. 语法分类详解

### 4.1 IP 类

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `is_ipv6` | 判断 IP 类型 | 否 | `is_ipv6="true"` | 检索 IPv6 资产 | / |
| `is_ipv6` | 同上 | 否 | `is_ipv6="false"` | 检索 IPv4 资产 | / |
| `ip` | 检索指定 IP 或 IP 段 | 否 | `ip="1.1.1.1"` | 检索单个 IPv4 地址 | / |
| `ip` | 同上 | 否 | `ip="1.1.1.1/24"` | 检索 IPv4 C 段 | / |
| `ip` | 同上 | 否 | `ip="2408:80f1:90:2001::2b"` | 检索单个 IPv6 地址 | / |
| `ip` | 同上 | 否 | `ip="1.1.1.0-1.1.1.255"` | 检索 IPv4 地址范围 | / |
| `ip.port` | 检索 IP 开放端口 | 否 | `ip.port="80"` | 检索开放 80 端口的资产 | `port` |
| `ip.port` | 同上 | 否 | `ip.port>="80"` | 检索端口号 ≥80 的资产 | / |
| `ip.port` | 同上 | 否 | `ip.port>"80" && ip.port<"1024"` | 检索端口在 80~1024 之间（不含边界）的资产 | / |
| `ip.port` | 同上 | 否 | `ip.port<="1024"` | 检索端口号 ≤1024 的资产 | / |
| `ip.isp` | 检索 IP 所属 ISP | 是 | `ip.isp="电信"` | 检索 ISP 为"电信"的资产 | `isp` |
| `ip.os_family` | 检索 IP 操作系统类型 | 否 | `ip.os_family="Windows"` | 检索操作系统类型为 Windows 的资产 | `os_family` |
| `ip.os` | 检索 IP 操作系统 | 是 | `ip.os="Windows Server 2016"` | 检索操作系统为 Windows Server 2016 的资产 | `os` |
| `ip.tag` | 检索 IP 标签 | 否 | `ip.tag="CDN"` | 检索带指定标签的资产。官方枚举包含较多标签；`CDN` / `蜜罐` / `Starlink` / `云厂商` / `终端截图` 仅为示例，以平台标签枚举为准 | `tag` |
| `ip.industry` | 检索 IP 行业类型 | 是 | `ip.industry="银行"` | 检索行业为"银行"的资产。可选值：银行 / 教育 / 医疗 / 工业 / 金融 等 | `industry` |

### 4.2 域名类

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `is_domain` | 是否为域名资产 | 否 | `is_domain="true"` | 检索域名资产 | / |
| `is_domain` | 同上 | 否 | `is_domain="false"` | 检索非域名资产 | / |
| `domain` | 检索域名 | 是 | `domain="www.webray.com.cn"` | 检索域名及其子域名资产 | / |
| `domain.root` | 检索主域名的子域名 | 是 | `domain.root="webray.com.cn"` | 检索该主域名的所有子域名资产 | / |

### 4.3 地理位置

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `ip.country` | 检索 IP 所属国家 | 是 | `ip.country="中国"` | 支持 ISO 国家代码：中文全称/简称/英文名/ISO2/ISO3/ISO 数字码 | `country` |
| `ip.country` | 同上 | 是 | `ip.country="CN"` | ISO2 两位字母代码 | `country` |
| `ip.country` | 同上 | 是 | `ip.country="CHN"` | ISO3 三位字母代码 | `country` |
| `ip.country` | 同上 | 是 | `ip.country="156"` | ISO 数字代码 | `country` |
| `ip.province` / `ip.region` | 检索 IP 所属省份 | 是 | `ip.province="陕西省"` | 支持中文全称/简称/短名/全拼/简拼 | `province` / `region` |
| `ip.province` | 同上 | 是 | `ip.province="shaanxi"` | 全拼 | `province` / `region` |
| `ip.province` | 同上 | 是 | `ip.province="sx"` | 简拼 | `province` / `region` |
| `ip.city` | 检索 IP 所属城市 | 是 | `ip.city="北京市"` | 仅指直辖市、地级市、特别行政区，不含县级市和区县。支持中文全称/简称/全拼/简拼 | `city` |
| `ip.city` | 同上 | 是 | `ip.city="xian"` | 全拼 | `city` |
| `ip.district` / `ip.county` | 检索 IP 所属区县 | 是 | `ip.district="朝阳区"` | 含市辖区、县、县级市。支持中文全称/简称/全拼 | `district` / `county` |

### 4.4 ICP 备案

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `icp.number` | 检索 ICP 备案号 | 是 | `icp.number="京ICP备17003970号"` | 检索域名的 ICP 备案号 | `icp` |
| `icp.name` | 检索 ICP 备案企业名 | 是 | `icp.name="远江盛邦"` | 模糊匹配企业名称 | / |
| `icp.name_prefix` | 检索 ICP 备案企业名（前缀匹配） | 否 | `icp.name_prefix="远江"` | 前缀匹配查询企业名称 | / |
| `icp.webname` | 检索 ICP 备案网站名 | 是 | `icp.webname="盛邦安全"` | 模糊匹配网站名称 | / |

### 4.5 自治系统 (AS)

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `asn.number` | 检索 ASN 号 | 否 | `asn.number="AS15169"` | 精确匹配 ASN 号 | `asn` |
| `asn.org` | 检索 ASN 实体名 | 是 | `asn.org="amazon"` | 模糊匹配 ASN 组织名 | / |

### 4.6 Web 资产

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `is_web` | 是否为 Web 资产 | 否 | `is_web="true"` | 检索 Web 类型资产 | `is_website` |
| `is_web` | 同上 | 否 | `is_web="false"` | 检索非 Web 类型资产 | / |
| `web.server` | 检索 Web 服务类型 | 是 | `web.server="Apache"` | 模糊匹配 Web 服务器类型 | `server` |
| `web.status_code` | 检索响应状态码 | 否 | `web.status_code="200"` | 精确匹配 HTTP 状态码 | `status_code` / `code` / `http_status` |
| `web.header` | 检索响应头 | 是 | `web.header="elastic"` | 模糊匹配响应头内容 | `header` / `web.response` / `response` |
| `web.title` | 检索网站标题 | 是 | `web.title="北京"` | 模糊匹配网站标题 | `title` |
| `web.lang` | 检索 Web 开发语言 | 否 | `web.lang="PHP"` | 精确匹配开发语言 | `lang` |
| `web.body` | 检索网页内容 | 是 | `web.body="网络空间测绘"` | 模糊匹配网页正文内容 | `body` |
| `web.icon` | 检索网站图标 | 否 | `web.icon="c60ea375c39d1ab273c4d1bee717287a"` | 精确匹配网站图标的 MD5 哈希值 | `icon` |

### 4.7 协议

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `protocol.transport` | 检索传输层协议 | 否 | `protocol.transport="udp"` | 检索传输层为 UDP 的资产。可选：`tcp` / `udp` | `transport` / `protocol` |
| `protocol.service` | 检索服务协议 | 否 | `protocol.service="http"` | 精确匹配服务协议（http、https、ssh、ftp 等） | `service` |
| `protocol.banner` | 检索 Banner 信息 | 是 | `protocol.banner="nginx"` | 模糊匹配 Banner 详情 | `banner` |

### 4.8 应用

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `app.name` | 检索应用名称 | 否 | `app.name="物联网平台"` | 精确匹配应用名称 | `app` |

> 更多应用指纹查询语法可前往 [指纹中心](https://www.daydaymap.com/fingerprint) 查看。

### 4.9 组件

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `product` | 检索组件名称 | 否 | `product="Nginx"` | 精确匹配组件名称 | / |

### 4.10 设备

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `device.name` | 检索设备名称 | 是 | `device.name="Aruba Device"` | 模糊匹配设备名称 | `device` |
| `device.type` | 检索设备类型 | 是 | `device.type="安全防护设备"` | 模糊匹配设备类型 | `device_type` |
| `device.type_sub` | 检索设备子类型 | 是 | `device.type_sub="邮件安全系统"` | 模糊匹配设备子类型 | / |
| `brand` | 检索设备品牌 | 否 | `brand="Cisco"` | 精确匹配品牌名 | / |
| `model` | 检索设备型号 | 否 | `model="Chromecast"` | 精确匹配型号 | / |
| `manufacturer` | 检索设备制造商 | 是 | `manufacturer="Hikvision"` | 模糊匹配制造商名 | / |

### 4.11 证书

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `cert.issuer` | 检索证书颁发者 | 是 | `cert.issuer="Amazon"` | 模糊匹配 | / |
| `cert.issuer.cn` | 检索证书颁发者 CN | 是 | `cert.issuer.cn="GeoTrust CN RSA CA G1"` | 模糊匹配 Common Name | / |
| `cert.issuer.country` | 检索证书颁发者国家 | 是 | `cert.issuer.country="US"` | 通常用两位字母代码 | / |
| `cert.issuer.org` | 检索证书颁发者组织 | 是 | `cert.issuer.org="DigiCert Inc"` | 模糊匹配组织名 | / |
| `cert.subject` | 检索证书主体 | 是 | `cert.subject="Technicolor"` | 模糊匹配 | / |
| `cert.subject.cn` | 检索证书主体 CN | 是 | `cert.subject.cn="daydaymap.com"` | 模糊匹配 Common Name | / |
| `cert.subject.country` | 检索证书主体国家 | 是 | `cert.subject.country="CN"` | 通常用两位字母代码 | / |
| `cert.subject.org` | 检索证书主体组织 | 是 | `cert.subject.org="DigiCert Inc"` | 模糊匹配组织名 | / |
| `cert.sn` | 检索证书序列号 | 否 | `cert.sn="0ECDAB152D2161F7C843D25F3F00FCDE"` | 十六进制格式 | / |
| `cert.org` | 检索证书持有者组织 | 是 | `cert.org="Plesk"` | 模糊匹配 | / |
| `cert.md5` | 检索证书 MD5 | 否 | `cert.md5="0aeb8908c10b3bff4b920bdb199eb09a"` | 精确匹配 | / |
| `cert.is_expired` | 检索证书是否过期 | 否 | `cert.is_expired="true"` | 检索已过期证书资产 | / |
| `cert.is_expired` | 同上 | 否 | `cert.is_expired="false"` | 检索未过期证书资产 | / |
| `cert.is_trust` | 检索证书可信度 | 否 | `cert.is_trust="true"` | 检索可信证书资产 | / |
| `cert.is_trust` | 同上 | 否 | `cert.is_trust="false"` | 检索不可信证书资产 | / |
| `cert.startdate` | 检索证书起始时间 | 否 | `cert.startdate="2024-01-01 00:00:00"` | 精确匹配。支持 `>` `>=` `<` `<=` 范围查询 | `startdate` |
| `cert.startdate` | 同上 | 否 | `cert.startdate>"2024-01-01"` | 检索 2024-01-01 之后（不含当天）的资产 | / |
| `cert.startdate` | 同上 | 否 | `cert.startdate>="2024-01-01"` | 检索 2024-01-01 及之后的资产 | / |
| `cert.startdate` | 同上 | 否 | `cert.startdate>"2023-01-01" && cert.startdate<"2024-01-01"` | 检索 2023~2024 年之间的资产 | / |
| `cert.enddate` | 检索证书到期时间 | 否 | `cert.enddate="2024-01-01 00:00:00"` | 精确匹配。支持 `>` `>=` `<` `<=` 范围查询 | `enddate` |
| `cert.enddate` | 同上 | 否 | `cert.enddate>"2024-01-01"` | 检索 2024-01-01 之后到期的资产 | / |
| `cert.enddate` | 同上 | 否 | `cert.enddate>="2024-01-01"` | 检索 2024-01-01 及之后到期的资产 | / |
| `cert.enddate` | 同上 | 否 | `cert.enddate>"2023-01-01" && cert.enddate<"2024-01-01"` | 检索到期时间在 2023~2024 之间的资产 | / |

### 4.12 时间

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `time` | 检索资产更新时间 | 否 | `time="2024-01-01 08:00:00"` | 精确匹配。支持 `>` `>=` `<` `<=` 范围查询 | `time_stamp` |
| `time` | 同上 | 否 | `time>"2024-01-01"` | 检索 2024-01-01 之后更新（不含当天）的资产 | / |
| `time` | 同上 | 否 | `time>="2024-01-01"` | 检索 2024-01-01 及之后更新的资产 | / |
| `time` | 同上 | 否 | `time>"2023-01-01" && time<"2024-01-01"` | 检索 2023~2024 年之间更新的资产 | / |

### 4.13 漏洞指纹

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `vul.cve` | 检索 CVE 漏洞 | 是 | `vul.cve="CVE-2021-42013"` | 检索可能受该 CVE 影响的资产 | `cve` |
| `vul.dvb` | 检索 DVB 漏洞 | 否 | `vul.dvb="DVB-2021-2898"` | 检索可能受该 DVB 漏洞影响的资产 | `dvb` |

### 4.14 资产归属

| 查询语法 | 用途 | 模糊匹配 | 查询示例 | 说明 | 兼容语法 |
|----------|------|----------|----------|------|----------|
| `org.name` | 检索资产归属 | 是 | `org.name="远江盛邦"` | 检索 ASN 组织、证书、ICP 备案中含"远江盛邦"的资产 | `org` |
| `org.name_prefix` | 检索资产归属（前缀匹配） | 否 | `org.name_prefix="远江"` | 通过前缀匹配查询资产归属 | `org_prefix` |

---

## 5. API 接口

### 5.1 API 认证

**获取 API KEY**：登录 DayDayMap 平台 → 点击右上角昵称 → 【个人中心】→【个人信息】→ 查看 API KEY 信息 → 点击复制按钮。

| 参数名 | 参数值示例 | 是否必填 | 类型 | 说明 |
|--------|------------|----------|------|------|
| `api-key` | `xxxxxx07fe2248ac9a6944a622xxxxxx` | 是 | String | 用户登录后在个人中心获取 |

### 5.2 数据查询接口

**请求地址**：`POST https://www.daydaymap.com/api/v1/raymap/search/all`

**请求头 (Header)**

| 参数名 | 参数值示例 | 是否必填 | 类型 | 说明 |
|--------|------------|----------|------|------|
| `api-key` | `xxxxxx07fe2248ac9a6944a622xxxxxx` | 是 | String | API 认证凭证 |
| `Content-Type` | `application/json` | 是 | String | 请求体格式 |

**请求体 (Body)**

| 参数名 | 参数值示例 | 是否必填 | 类型 | 说明 |
|--------|------------|----------|------|------|
| `page` | `1` | 是 | Number | 查询页码；官方错误说明中给出合法范围 `1~10000`，但这不代表可获取 1 亿条数据 |
| `page_size` | `10` | 是 | Number | 每页资产条数；当前实现/生态适配器使用的最大值为 `10000` |
| `keyword` | `cG9ydD0iODAi` | 是 | String | 搜索语法的 **Base64 编码**（例如 `port="80"` → `cG9ydD0iODAi`） |
| `fields` | `ip,port` | 否 | String | 自定义响应字段。如 `fields=ip,port` 则响应只包含 ip、port。优先级高于 `exclude_fields` |
| `exclude_fields` | `ip,port` | 否 | String | 反向排除字段。如 `exclude_fields=ip,port` 则响应包含除 ip、port 之外的字段；与 `fields` 同时存在时以 `fields` 为优先 |

> **有效总量上限**：官方错误码 `2005` 表示“最多只能查看前 1 万条数据”。因此 `page` 的参数范围与“最多可获取的数据总量”是两件事；实务中应按 **最多前 10,000 条** 设计分页逻辑。

### 5.3 请求示例

**CURL 示例**

```bash
curl -X POST 'https://www.daydaymap.com/api/v1/raymap/search/all' \
  -H 'api-key: xxxxxx07fe2248ac9a6944a622xxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "page": 1,
    "page_size": 10,
    "keyword": "cG9ydD0iODAi"
  }'
```

> 不建议默认使用 `curl -k`，因为它会关闭 TLS 证书校验。只有在明确知道原因并接受风险时才应这样做。

**Python 示例**

```python
import requests

headers = {
    'api-key': 'xxxxxx07fe2248ac9a6944a622xxxxxx'
}

data = {
    "page": 1,
    "page_size": 10,
    "keyword": "cG9ydD0iODAi"  # port="80" 的 Base64 编码
}

response = requests.post(
    'https://www.daydaymap.com/api/v1/raymap/search/all',
    headers=headers,
    json=data,
    timeout=30
)
response.raise_for_status()
result = response.json()
print(result)
```

**keyword 编码规则**

```python
import base64

# 原始搜索语句
query = 'ip.country="中国" && ip.port="80" && web.server="Nginx"'

# Base64 编码
keyword = base64.b64encode(query.encode('utf-8')).decode('utf-8')
# → aXAuY291bnRyeT0i5Lit5Zu9IiAmJiBpcC5wb3J0PSI4MCIgJiYgd2ViLnNlcnZlcj0iTmdpbngi
```

### 5.4 响应格式

**成功响应**

```json
{
  "code": 200,
  "data": {
    "list": [
      {
        "asn": null,
        "asn_org": null,
        "banner": "",
        "cert": "4cd40a9acc527c53e966ba1ffcceb0e2 2024-01-25T00:00:00.000000+8:00 2025-02-01T23:59:59.000000+8:00 C=GB,ST=Greater Manchester,L=Salford,O=Sectigo Limited,CN=Sectigo RSA Domain Validation Secure Server CA, CN=onlinetelecomipprovedor.com.br, 8a46436f9ad05ebbf88d58c8c92ce952",
        "cert_selfsigned": null,
        "city": null,
        "country": "巴西",
        "device": null,
        "domain": null,
        "header": "HTTP/1.1 301 Moved Permanently Server: nginx......",
        "icp_reg_name": null,
        "industry": [],
        "ip": "2804:7cf4:0:0:0:0:0:4",
        "is_ipv6": true,
        "is_website": true,
        "isp": null,
        "lang": "PHP",
        "os": null,
        "port": 80,
        "product": ["Varnish", "PHP", "Lodash", "HSTS", "Nginx", "Wix", "Sectigo"],
        "protocol": "tcp",
        "province": null,
        "server": "nginx;Pepyaka",
        "service": "https",
        "tags": ["CDN"],
        "time_stamp": "2024-03-24T05:17:03.934963+08:00",
        "title": "Página inicial"
      }
    ],
    "page": 1,
    "page_size": 1,
    "total": 1,
    "use_time": "92"
  },
  "msg": "检索成功"
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | Number | 状态码，200 为成功 |
| `msg` | String | 状态描述 |
| `data.list` | Array | 资产列表 |
| `data.page` | Number | 当前页码 |
| `data.page_size` | Number | 每页条数 |
| `data.total` | Number | 总记录数 |
| `data.use_time` | String | 查询耗时（毫秒） |

**示例响应中的常见资产字段（非穷举）**

| 字段 | 说明 |
|------|------|
| `ip` | IP 地址 |
| `port` | 端口 |
| `protocol` | 传输层协议（tcp/udp） |
| `service` | 服务协议（http/https/ssh 等） |
| `is_ipv6` | 是否 IPv6 |
| `is_website` | 是否 Web 资产 |
| `domain` | 域名 |
| `title` | 网站标题 |
| `header` | HTTP 响应头 |
| `server` | Web 服务器 |
| `lang` | 开发语言 |
| `banner` | Banner 信息 |
| `product` | 组件列表 |
| `os` | 操作系统 |
| `country` | 国家 |
| `province` | 省份 |
| `city` | 城市 |
| `isp` | ISP 运营商 |
| `asn` / `asn_org` | ASN 号 / ASN 组织 |
| `cert` | 证书详情 |
| `cert_selfsigned` | 是否自签证书 |
| `icp_reg_name` | ICP 备案企业名 |
| `industry` | 行业标签 |
| `tags` | 资产标签数组；`CDN` / `蜜罐` / `Starlink` / `云厂商` / `终端截图` 等仅为部分示例 |
| `device` | 设备信息 |
| `time_stamp` | 资产更新时间 |

**失败响应**

```json
{
  "code": 2001,
  "data": {},
  "msg": "请检查api-key是否正确"
}
```

### 5.5 API 错误码

| Code | 错误描述 |
|------|----------|
| `2001` | api-key 错误，请检查后重试 |
| `2002` | 参数错误（查询语句解析错误 / 参数类型错误 / 缺少参数 / 不支持的字段 / page 超出范围） |
| `2003` | 会员权限不足，请提升会员权限 |
| `2004` | 积分不足，请充值后使用 |
| `2005` | 最多只能查看前 1 万条数据 |
| `2006` | 数据获取失败，请重试 |

> **Code 2002 详细场景**：
> - 场景一：查询语句解析错误
> - 场景二：参数应为 String 类型
> - 场景三：缺少必要请求参数
> - 场景四：暂不支持该字段查询，当前可支持 ip、port 等
> - 场景五：page 必须在 1 至 10000 范围内的整数

---

## 6. 数据过滤

进入检索结果页后，点击【数据过滤】，可直接选择以下选项对数据进行过滤：

- **排除 CDN**：过滤掉 CDN 节点资产
- **排除蜜罐**：过滤掉蜜罐资产
- **过滤无效请求**：过滤掉无法正常访问的资产

> 数据过滤是 UI 层功能，API 查询需在搜索语句中通过 `ip.tag!="CDN"` 等语法实现等效过滤。

---

## 7. 兼容/别名语法速查

DayDayMap 官方语法表为部分字段给出了兼容/别名写法。原稿进一步把这些别名硬性归属到 FOFA、Shodan、Hunter 等具体平台，但这一“对应平台”归属不是本次从 DayDayMap 官方开发者文档中稳定确认的内容，因此删除平台归属，仅保留 DayDayMap 可接受的别名。

| DayDayMap 主语法 | 兼容/别名语法 |
|------------------|---------------|
| `ip.port` | `port` |
| `ip.isp` | `isp` |
| `ip.os_family` | `os_family` |
| `ip.os` | `os` |
| `ip.tag` | `tag` |
| `ip.industry` | `industry` |
| `ip.country` | `country` |
| `ip.province` / `ip.region` | `province` / `region` |
| `ip.city` | `city` |
| `ip.district` / `ip.county` | `district` / `county` |
| `icp.number` | `icp` |
| `asn.number` | `asn` |
| `is_web` | `is_website` |
| `web.server` | `server` |
| `web.status_code` | `status_code` / `code` / `http_status` |
| `web.header` | `header` / `web.response` / `response` |
| `web.title` | `title` |
| `web.lang` | `lang` |
| `web.body` | `body` |
| `web.icon` | `icon` |
| `protocol.transport` | `transport` / `protocol` |
| `protocol.service` | `service` |
| `protocol.banner` | `banner` |
| `app.name` | `app` |
| `device.name` | `device` |
| `device.type` | `device_type` |
| `cert.startdate` | `startdate` |
| `cert.enddate` | `enddate` |
| `vul.cve` | `cve` |
| `vul.dvb` | `dvb` |
| `org.name` | `org` |
| `org.name_prefix` | `org_prefix` |
| `time` | `time_stamp` |

> 兼容/别名写法应以 DayDayMap 当前语法页为准；不能据此反推“与某第三方平台完全语法兼容”。

---

## 8. 常见问题

### Q1: API 查询最多能返回多少条数据？

**有效可查看上限是前 10,000 条。** `page_size` 可设置较大值（生态适配器通常以 `10000` 作为最大抓取量），而官方错误码 `2005` 明确表示“最多只能查看前 1 万条数据”。`page=1~10000` 是参数合法范围，不应理解为可以通过翻页获取超过前 1 万条的数据。

### Q2: keyword 参数怎么构造？

keyword 是搜索语句的 Base64 编码。例如要查询 `port="80"`，需先 Base64 编码得到 `cG9ydD0iODAi`，再作为 keyword 传入。

### Q3: 模糊匹配和精确匹配有什么区别？

- `=` 模糊匹配：检索包含关键词的资产（如 `web.title="北京"` 匹配标题中含"北京"的资产）
- `==` 精确匹配：检索完全匹配的资产（如 `web.title=="北京"` 只匹配标题恰好为"北京"的资产）
- **不支持模糊匹配**的字段，`=` 等同 `==`，两者都表示精确匹配

### Q4: 如何排除 CDN 资产？

搜索语句中使用 `ip.tag!="CDN"`，或在结果页点击【数据过滤】→ 排除 CDN。

### Q5: 支持哪些地理编码格式？

- 国家：支持中文全称/简称/英文名/ISO2/ISO3/ISO 数字码
- 省份：支持中文全称/简称/短名/全拼/简拼
- 城市：支持中文全称/简称/全拼/简拼
- 区县：支持中文全称/简称/全拼

### Q6: 证书时间查询支持哪些运算符？

支持 `=`（精确匹配）、`>`（之后）、`>=`（及之后）、`<`（之前）、`<=`（及之前），可使用 `&&` 组合区间查询。

### Q7: 如何查询特定 CVE 漏洞影响的资产？

使用 `vul.cve="CVE-2021-42013"` 或 `vul.dvb="DVB-2021-2898"` 进行查询。

### Q8: `fields` 和 `exclude_fields` 可以同时传吗？

可以出现在同一个请求结构中，但按接口说明，`fields` 优先级更高；指定 `fields` 时，应把 `exclude_fields` 视为不生效。

> 原稿关于“一个账号支持多人同时登录、API 调用频率可能受会员等级限制”的表述，本次未在当前官方开发者文档检索结果中获得足够支撑，因此删除，不再作为规范结论。

---

## 附录：查询语法速查表（按类别）

> 原稿“语法数”把字段、别名和示例行混在一起统计，容易误导。官方只强调 **14 大类、50+ 查询语法**，本表不再给出未经统一口径定义的分类计数。

| 类别 | 核心语法 |
|------|----------|
| IP | `ip`, `ip.port`, `ip.isp`, `ip.os_family`, `ip.os`, `ip.tag`, `ip.industry`, `is_ipv6` |
| 域名 | `domain`, `domain.root`, `is_domain` |
| 地理位置 | `ip.country`, `ip.province` / `ip.region`, `ip.city`, `ip.district` / `ip.county` |
| ICP 备案 | `icp.number`, `icp.name`, `icp.name_prefix`, `icp.webname` |
| 自治系统 | `asn.number`, `asn.org` |
| Web | `is_web`, `web.server`, `web.status_code`, `web.header`, `web.title`, `web.lang`, `web.body`, `web.icon` |
| 协议 | `protocol.transport`, `protocol.service`, `protocol.banner` |
| 应用 | `app.name` |
| 组件 | `product` |
| 设备 | `device.name`, `device.type`, `device.type_sub`, `brand`, `model`, `manufacturer` |
| 证书 | `cert.issuer*`, `cert.subject*`, `cert.sn`, `cert.org`, `cert.md5`, `cert.is_expired`, `cert.is_trust`, `cert.startdate`, `cert.enddate` |
| 时间 | `time` |
| 漏洞 | `vul.cve`, `vul.dvb` |
| 资产归属 | `org.name`, `org.name_prefix` |

---

> **复核说明**：本文档基于 DayDayMap 官方开发者文档在线复核，并用公开生态实现交叉检查 API 结构。官方帮助页当前仍显示部分文档内容更新时间为 2024-04-08，但网站与平台能力会继续变化。若实际接口行为与本文不一致，以 [DayDayMap 官方文档](https://www.daydaymap.com/help/document) 和实时接口返回为准。
