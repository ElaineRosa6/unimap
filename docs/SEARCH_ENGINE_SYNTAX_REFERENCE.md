# 搜索引擎语法基准参考手册

> **版本**: 2026-06  
> **最后更新**: 2026-06-08（基于 5 引擎官方语法全量核查 + 6 项严重错误修正）  
> **说明**: 本文档汇总了各类搜索引擎（通用搜索引擎、网络空间搜索引擎、威胁情报搜索引擎）的查询语法，所有语法均经过官方文档或权威来源核实。  
> **注意**: 各平台语法可能随版本更新而变化，建议定期查阅官方文档确认最新语法。部分平台（如 FOFA、Hunter、Quake、DayDayMap）的官方文档需要 JavaScript 清染，本文档基于 2026 年 6 月可获取的最新官方文档。

---

## 目录

- [一、通用搜索引擎](#一通用搜索引擎)
  - [1.1 Google 高级搜索语法](#11-google-高级搜索语法)
  - [1.2 Bing 高级搜索语法](#12-bing-高级搜索语法)
  - [1.3 百度高级搜索语法](#13-百度高级搜索语法)
- [二、网络空间搜索引擎](#二网络空间搜索引擎)
  - [2.1 Shodan](#21-shodan)
  - [2.2 FOFA](#22-fofa)
  - [2.3 Hunter（奇安信鹰图）](#23-hunter奇安信鹰图)
  - [2.4 ZoomEye（知道创宇）](#24-zoomeye知道创宇)
  - [2.5 Quake（360网络空间测绘）](#25-quake360网络空间测绘)
  - [2.6 Censys](#26-censys)
  - [2.7 DayDayMap（盛邦安全）](#27-daydaymap盛邦安全)
- [三、威胁情报与恶意软件搜索引擎](#三威胁情报与恶意软件搜索引擎)
  - [3.1 VirusTotal Intelligence](#31-virustotal-intelligence)
- [四、DNS搜索引擎](#四dns搜索引擎)
  - [4.1 DnsDB](#41-dnsdb)
- [五、语法对比速查表](#五语法对比速查表)
- [六、逻辑运算符与通配符通用规则](#六逻辑运算符与通配符通用规则)

---

## 一、通用搜索引擎

### 1.1 Google 高级搜索语法

Google 高级搜索运算符（Search Operators）用于精确限定搜索范围，是信息收集和 Google Hacking 的基础。

| 运算符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `site` | `site:域名` | 限定搜索范围为指定域名下的网页 | `site:example.com` |
| `filetype` | `filetype:扩展名` | 搜索指定文件类型 | `filetype:pdf 渗透测试` |
| `intitle` | `intitle:关键词` | 搜索标题中包含指定关键词的网页 | `intitle:"admin login"` |
| `allintitle` | `allintitle:词1 词2` | 搜索标题中同时包含所有指定关键词的网页 | `allintitle:admin login` |
| `inurl` | `inurl:关键词` | 搜索URL中包含指定关键词的网页 | `inurl:admin` |
| `allinurl` | `allinurl:词1 词2` | 搜索URL中同时包含所有指定关键词的网页 | `allinurl:admin login` |
| `intext` | `intext:关键词` | 搜索正文内容中包含指定关键词的网页 | `intext:"password"` |
| `allintext` | `allintext:词1 词2` | 搜索正文中同时包含所有指定关键词的网页 | `allintext:username password` |
| `inanchor` | `inanchor:关键词` | 搜索锚文本中包含指定关键词的网页 | `inanchor:click here` |
| `allinanchor` | `allinanchor:词1 词2` | 搜索锚文本中同时包含所有指定关键词的网页 | `allinanchor:download pdf` |
| `cache` | `cache:URL` | ~~查看Google缓存的页面~~ **已于2024年9月完全移除，不可使用** | ~~`cache:example.com`~~ |
| `related` | `related:域名` | 搜索与指定网站相关的网站 | `related:example.com` |
| `info` | `info:域名` | 获取指定URL的Google索引信息 | `info:example.com` |
| `link` | `link:域名` | ~~搜索链接到指定网站的页面~~ **已完全废弃，不可使用** | ~~`link:example.com`~~ |
| `define` | `define:术语` | 搜索术语定义 | `define:phishing` |
| `OR` | `词1 OR 词2` | 逻辑或运算 | `site:example.com OR site:test.com` |
| `-`（减号） | `-关键词` | 排除包含指定关键词的结果 | `jaguar -car` |
| `""`（引号） | `"精确短语"` | 精确匹配短语 | `"admin panel"` |
| `*`（星号） | `词*` | 通配符，匹配任意词 | `how to * a website` |
| `..`（范围） | `数字1..数字2` | 数字范围搜索 | `2020..2024` |
| `AROUND(X)` | `词1 AROUND(X) 词2` | 两个词之间相隔不超过X个词 | `apple AROUND(3) iphone` |

**注意事项**：
- Google 已于2024年9月完全移除 `cache:` 运算符（Danny Sullivan 已公开确认），搜索框输入 `cache:URL` 不再触发任何功能。`link:` 运算符已完全废弃（Ahrefs、Search Engine Land 等已确认），不再返回任何反向链接结果。
- `filetype:` 支持的常见扩展名包括 pdf、doc、docx、xls、xlsx、ppt、pptx、txt、xml、csv、sql、log、conf、bak 等。
- 运算符不区分大小写，但关键词区分大小写。
- 多个运算符可组合使用，如 `site:example.com filetype:pdf intitle:report`。

---

### 1.2 Bing 高级搜索语法

Bing 高级搜索关键字与 Google 类似，但存在差异。

| 运算符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `site` | `site:域名` | 限定搜索范围为指定域名 | `site:example.com` |
| `filetype` | `filetype:扩展名` | 搜索指定文件类型 | `filetype:pdf 报告` |
| `intitle` | `intitle:关键词` | 搜索标题中包含指定关键词的网页 | `intitle:"admin"` |
| `inbody` | `inbody:关键词` | 搜索正文内容中包含指定关键词的网页（Bing特有，对应Google的intext） | `inbody:password` |
| `inanchor` | `inanchor:关键词` | 搜索锚文本中包含指定关键词的网页 | `inanchor:login` |
| `contains` | `contains:文件类型` | 搜索包含指向指定文件类型链接的网页 | `music contains:wma` |
| `ip` | `ip:IP地址` | 搜索由指定IP地址托管的网站 | `ip:207.46.249.252` |
| `language` | `language:语言代码` | 限定搜索结果的语言 | `language:en` |
| `loc` | `loc:地区代码` | 限定搜索结果的地区 | `loc:US` |
| `location` | `location:地区代码` | 同loc，限定搜索结果的地区 | `location:CN` |
| `prefer` | `prefer:关键词` | 对指定搜索词添加强调权重 | `prefer:security` |
| `feed` | `feed:关键词` | 在RSS/Atom订阅源中搜索 | `feed:security` |
| `hasfeed` | `hasfeed:关键词` | 搜索包含RSS/Atom订阅源的网页 | `hasfeed:news` |
| `url` | `url:URL` | 搜索指定URL的网页 | `url:example.com/admin` |
| `OR` | `词1 OR 词2` | 逻辑或运算 | `site:bbc.co.uk OR site:cnn.com` |
| `-`（减号） | `-关键词` | 排除包含指定关键词的结果 | `jaguar -car` |
| `""`（引号） | `"精确短语"` | 精确匹配短语 | `"admin panel"` |

**注意事项**：
- Bing 使用 `inbody` 而非 Google 的 `intext`。
- `contains:` 是 Bing 特有运算符，搜索包含指向特定文件类型链接的页面（而非文件本身）。
- `ip:` 运算符可直接搜索由指定IP托管的网站。
- `language:` 和 `loc:` 支持标准语言/地区代码（如 en、zh、US、CN）。

---

### 1.3 百度高级搜索语法

百度搜索引擎支持部分高级搜索语法，功能较 Google 和 Bing 更为有限。

| 运算符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `site` | `site:域名` | 限定搜索范围为指定域名 | `site:example.com` |
| `filetype` | `filetype:扩展名` | 搜索指定文件类型 | `filetype:pdf 报告` |
| `intitle` | `intitle:关键词` | 搜索标题中包含指定关键词的网页 | `intitle:后台管理` |
| `inurl` | `inurl:关键词` | 搜索URL中包含指定关键词的网页 | `inurl:admin` |
| `-`（减号） | `-关键词` | 排除包含指定关键词的结果 | `苹果 -手机` |
| `""`（引号） | `"精确短语"` | 精确匹配短语 | `"网络空间测绘"` |
| `OR` | `词1 OR 词2` | 逻辑或运算 | `site:a.com OR site:b.com` |

**注意事项**：
- 百度对高级搜索语法的支持不如 Google 和 Bing 完善，部分运算符可能不稳定。
- `filetype:` 支持的格式包括 pdf、doc、xls、ppt、rtf、txt 等。
- 百度不支持 `intext`、`inanchor`、`cache`、`related` 等运算符。

---

## 二、网络空间搜索引擎

### 2.1 Shodan

Shodan 是全球最早的网络空间搜索引擎，专注于互联网连接设备的发现与搜索。使用 `:` 分隔符，值可用引号包裹。

**官方文档**: https://www.shodan.io/search/filters

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格 | 逻辑与（AND） | `port:80 country:"CN"` |
| `OR` | 逻辑或 | `port:80 OR port:443` |
| `-` | 逻辑非（NOT） | `port:80 -country:"US"` |
| `""` | 精确匹配 | `product:"Apache httpd"` |
| `,` | 多值 OR（字段内） | `hostname:google.com,facebook.com` |

#### 通用过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:IP` | 按IP地址搜索 | `ip:8.8.8.8` |
| `net` | `net:CIDR` | 按IP网段搜索 | `net:192.168.1.0/24` |
| `port` | `port:端口` | 按端口号搜索 | `port:80` |
| `hostname` | `hostname:"域名"` | 按主机名搜索 | `hostname:"google.com"` |
| `domain` | `domain:"域名"` | 按域名搜索 | `domain:"example.com"` |
| `asn` | `asn:ASN号` | 按自治系统号搜索 | `asn:AS4134` |
| `org` | `org:"组织名"` | 按组织搜索 | `org:"Google"` |
| `isp` | `isp:"ISP名"` | 按ISP搜索 | `isp:"China Unicom"` |
| `city` | `city:"城市"` | 按城市搜索 | `city:"Beijing"` |
| `country` | `country:"国家代码"` | 按国家搜索（2字母ISO代码） | `country:"CN"` |
| `region` | `region:"区域代码"` | 按区域代码搜索 | `region:"09"` |
| `state` | `state:"州/省"` | 按州/省搜索 | `state:"California"` |
| `postal` | `postal:"邮编"` | 按邮编搜索 | `postal:"94103"` |
| `geo` | `geo:纬度,经度` | 按地理坐标搜索 | `geo:39.9042,116.4074` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Windows Server 2016"` |
| `product` | `product:"产品名"` | 按产品/软件名搜索 | `product:"Apache httpd"` |
| `version` | `version:"版本号"` | 按版本号搜索 | `version:"2.4.49"` |
| `device` | `device:"设备类型"` | 按设备类型搜索 | `device:"router"` |
| `cpe` | `cpe:"CPE"` | 按CPE搜索 | `cpe:"cpe:/a:apache:http_server"` |
| `hash` | `hash:数值` | 按banner数据数值哈希搜索 | `hash:123456789` |
| `link` | `link:"域名/IP"` | 搜索与指定IP/域名同网络的资产 | `link:"1.1.1.1"` |
| `vuln` | `vuln:CVE编号` | 按CVE漏洞搜索（需付费API计划） | `vuln:CVE-2021-44228` |

#### HTTP 过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `http.title` | `http.title:"标题"` | 按HTTP页面标题搜索 | `http.title:"admin"` |
| `http.html` | `http.html:"内容"` | 按HTTP页面HTML内容搜索 | `http.html:"Apache"` |
| `http.html_hash` | `http.html_hash:哈希` | 按HTML内容哈希搜索 | `http.html_hash:123456789` |
| `http.status` | `http.status:状态码` | 按HTTP状态码搜索 | `http.status:200` |
| `http.server` | `http.server:"服务器"` | 按HTTP Server头搜索 | `http.server:"nginx"` |
| `http.location` | `http.location:"URL"` | 按HTTP Location头搜索 | `http.location:"/login"` |
| `http.headers_hash` | `http.headers_hash:哈希` | 按HTTP响应头哈希搜索 | `http.headers_hash:123456789` |
| `http.favicon.hash` | `http.favicon.hash:哈希值` | 按favicon图标哈希搜索 | `http.favicon.hash:-247388890` |
| `http.robots_hash` | `http.robots_hash:哈希` | 按robots.txt哈希搜索 | `http.robots_hash:123456789` |
| `http.component` | `http.component:"技术"` | 按Web技术组件搜索 | `http.component:"bootstrap"` |
| `http.component_category` | `http.component_category:"分类"` | 按Web技术分类搜索 | `http.component_category:"cms"` |
| `http.waf` | `http.waf:"WAF名"` | 按Web应用防火墙搜索 | `http.waf:"Cloudflare"` |

#### SSL/TLS 过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `has_ssl` | `has_ssl:true` | 筛选是否有SSL证书 | `has_ssl:true` |
| `ssl.cert.subject.cn` | `ssl.cert.subject.cn:"CN"` | 按证书主题CN搜索 | `ssl.cert.subject.cn:"google.com"` |
| `ssl.cert.issuer.cn` | `ssl.cert.issuer.cn:"CN"` | 按证书颁发者CN搜索 | `ssl.cert.issuer.cn:"Let's Encrypt"` |
| `ssl.cert.fingerprint` | `ssl.cert.fingerprint:"SHA1"` | 按证书SHA1指纹搜索 | `ssl.cert.fingerprint:"F3C98F22..."` |
| `ssl.cert.serial` | `ssl.cert.serial:序列号` | 按证书序列号搜索 | `ssl.cert.serial:0A0B0C` |
| `ssl.cert.pubkey.bits` | `ssl.cert.pubkey.bits:位数` | 按公钥位数搜索 | `ssl.cert.pubkey.bits:2048` |
| `ssl.cert.pubkey.type` | `ssl.cert.pubkey.type:"类型"` | 按公钥类型搜索 | `ssl.cert.pubkey.type:"RSA"` |
| `ssl.cert.expired` | `ssl.cert.expired:true/false` | 筛选证书是否过期 | `ssl.cert.expired:true` |
| `ssl.chain_count` | `ssl.chain_count:数量` | 按证书链计数搜索 | `ssl.chain_count:3` |
| `ssl.version` | `ssl.version:"版本"` | 按SSL/TLS版本搜索 | `ssl.version:tlsv1.3` |
| `ssl.cipher.bits` | `ssl.cipher.bits:位数` | 按加密套件位数搜索 | `ssl.cipher.bits:256` |
| `ssl.cipher.version` | `ssl.cipher.version:"版本"` | 按加密套件版本搜索 | `ssl.cipher.version:"TLSv1.3"` |
| `ssl.ja3s` | `ssl.ja3s:哈希` | 按JA3S指纹搜索 | `ssl.ja3s:45094d08...` |
| `ssl.jarm` | `ssl.jarm:"指纹"` | 按JARM指纹搜索 | `ssl.jarm:"07d14d16..."` |
| `ssl.alpn` | `ssl.alpn:"协议"` | 按ALPN协议搜索 | `ssl.alpn:h2` |

#### 截图 / 布尔过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `has_screenshot` | `has_screenshot:true` | 筛选是否有截图 | `has_screenshot:true` |
| `has_vuln` | `has_vuln:true` | 筛选是否有已知漏洞 | `has_vuln:true` |
| `has_ipv6` | `has_ipv6:true` | 筛选是否有IPv6地址 | `has_ipv6:true` |
| `screenshot.hash` | `screenshot.hash:哈希` | 按截图图像哈希搜索 | `screenshot.hash:123456789` |
| `screenshot.label` | `screenshot.label:"标签"` | 按截图分类标签搜索 | `screenshot.label:"ics"` |

#### 云平台

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `cloud.provider` | `cloud.provider:"提供商"` | 按云服务商搜索 | `cloud.provider:"Amazon"` |
| `cloud.region` | `cloud.region:"区域"` | 按云区域搜索 | `cloud.region:"us-east-1"` |

**语法注意事项**：
- Shodan 使用 `:` 作为键值分隔符，值可用引号包裹（含空格的值必须加引号）。
- 逻辑运算符为空格(AND)、`OR`、`-`(NOT)。字段内多值用 `,`（如 `hostname:google.com,facebook.com`）。
- `http.server` 是搜索 HTTP Server 头的正确字段（非 `product`，`product` 搜索的是软件/产品名）。
- `http.html` 搜索 HTML 正文内容，Shodan **无独立 HTTP 响应头内容搜索**（仅有 `http.headers_hash` 哈希匹配）。
- `ssl.cert.subject.cn` 用于按证书主题 CN 搜索（如 `*.google.com`）。
- 截图分类标签（`screenshot.label`）包括：blank、desktop、ics、loggedin、login、osx、pos、screensaver、terminal、webcam、windows。

---

### 2.2 FOFA

FOFA 是国内使用最广泛的网络空间搜索引擎之一，由白帽汇运营。

**官方文档**: https://fofa.info/library（需 JavaScript 渲染）| **API 文档**: https://fofa.info/api

#### 逻辑连接符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `=` | 匹配（`=""` 查不存在或值为空） | `title="管理"` |
| `==` | 完全匹配（`==""` 查存在且值为空） | `title=="后台管理"` |
| `!=` | 不匹配（`!=""` 查值不为空） | `port!="80"` |
| `*=` | 模糊匹配（支持 `*` 和 `?`） | `title=*管理*` |
| `&&` | 逻辑与 | `title="admin" && country="CN"` |
| `\|\|` | 逻辑或 | `port="80" \|\| port="443"` |
| `()` | 分组，括号内容优先级最高 | `(port="80" \|\| port="443") && country="CN"` |

> **注意**: FOFA 使用 `=` 而非 `:` 作为键值分隔符。多条件混合时尽量用 `()` 包含。
>
> 权限标注：`✓` 所有用户、`[注册]` 注册用户及以上、`[个人]` 个人版及以上、`[专业]` 专业版及以上、`[商业]` 商业版及以上。

#### 基础类（General）

| 过滤器 | 语法 | 说明 | 示例 | 支持 |
|--------|------|------|------|------|
| `ip` | `ip="IP地址/CIDR"` | 按IPv4/IPv6/C段查询 | `ip="1.1.1.1"` / `ip="220.181.111.1/24"` | `=` `!=` |
| `port` | `port="端口号"` | 按端口号查询 | `port="6379"` | `=` `!=` `*=` |
| `domain` | `domain="域名"` | 按根域名查询（含子域名） | `domain="qq.com"` | `=` `!=` `*=` |
| `host` | `host="主机名"` | 按主机名查询 | `host=".fofa.info"` | `=` `!=` `*=` |
| `os` | `os="操作系统"` | 按操作系统查询 | `os="centos"` | `=` `!=` `*=` |
| `server` | `server="服务器名"` | 按HTTP Server头查询 | `server="Microsoft-IIS/10"` | `=` `!=` `*=` |
| `asn` | `asn="ASN号"` | 按自治系统号查询 | `asn="19551"` | `=` `!=` `*=` |
| `org` | `org="组织名"` | 按所属组织查询 | `org="LLC Baxet"` | `=` `!=` `*=` |
| `is_domain` | `is_domain=true/false` | 筛选有/无域名的资产 | `is_domain=true` | `=` |
| `is_ipv6` | `is_ipv6=true/false` | 筛选IPv6/IPv4资产 | `is_ipv6=true` | `=` |

#### 标记类（Special Label）

| 过滤器 | 语法 | 说明 | 权限 |
|--------|------|------|------|
| `app` | `app="应用名"` | 按建站软件规则查询 | `=` `!=` `*=` |
| `fid` | `fid="指纹ID"` | 按FOFA聚合站点指纹查询 | `=` `!=` |
| `product` | `product="产品名"` | 按产品名查询 | `=` `!=` |
| `product.version` | `product.version="版本"` | 按产品版本号查询 | [商业] |
| `category` | `category="分类"` | 按分类查询（如 `"服务"`） | [个人] |
| `type` | `type="service"` 或 `type="subdomain"` | 筛选协议资产或网站资产 | `=` |
| `cloud_name` | `cloud_name="云厂商"` | 按云服务商查询 | `=` `!=` `*=` |
| `is_cloud` | `is_cloud=true/false` | 筛选是否云服务 | `=` |
| `is_fraud` | `is_fraud=true/false` | 筛选仿冒垃圾站群 | [专业] |
| `is_honeypot` | `is_honeypot=true/false` | 筛选蜜罐资产 | [专业] |

#### 协议类（type=service）

| 过滤器 | 语法 | 说明 | 权限 |
|--------|------|------|------|
| `protocol` | `protocol="协议名"` | 按协议名称查询 | `=` `!=` `*=` |
| `banner` | `banner="关键词"` | 按协议返回信息查询 | [注册] |
| `banner_hash` | `banner_hash="哈希值"` | 按协议响应体哈希查询 | [个人] |
| `banner_fid` | `banner_fid="指纹值"` | 按协议返回信息结构指纹查询 | [个人] |
| `base_protocol` | `base_protocol="tcp/udp"` | 按传输层协议查询 | `=` `!=` |

#### 网站类（type=subdomain）

| 过滤器 | 语法 | 说明 | 权限 |
|--------|------|------|------|
| `title` | `title="关键词"` | 按网页标题查询 | `=` `!=` `*=` |
| `body` | `body="关键词"` | 按HTML正文查询 | [注册] |
| `header` | `header="关键词"` | 按HTTP响应头查询 | [注册] |
| `header_hash` | `header_hash="哈希值"` | 按响应头哈希查询 | [个人] |
| `body_hash` | `body_hash="哈希值"` | 按HTML正文哈希查询 | [个人] |
| `js_name` | `js_name="文件路径"` | 按HTML中JS文件名查询 | `=` `!=` `*=` |
| `js_md5` | `js_md5="MD5值"` | 按JS源码MD5查询 | `=` `!=` `*=` |
| `cname` | `cname="别名记录"` | 按CNAME记录查询 | `=` `!=` `*=` |
| `cname_domain` | `cname_domain="域名"` | 按CNAME解析的主域名查询 | `=` `!=` `*=` |
| `icon_hash` | `icon_hash="哈希值"` | 按网站favicon哈希查询（MMH3算法） | `=` `!=` |
| `status_code` | `status_code="状态码"` | 按HTTP状态码查询 | `=` `!=` |
| `icp` | `icp="备案号"` | 按ICP备案号查询 | `=` `!=` `*=` |
| `sdk_hash` | `sdk_hash="哈希值"` | 按第三方嵌入代码哈希查询 | [商业] |

#### 地理位置（Location）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `country` | `country="代码/中文"` | 按国家查询 | `country="CN"` / `country="中国"` |
| `region` | `region="省份"` | 按省份查询 | `region="Zhejiang"` / `region="浙江"` |
| `city` | `city="城市"` | 按城市查询 | `city="Hangzhou"` |

> 地理位置字段仅支持 `=` `!=`，不支持 `*=`。中文名仅支持中国地区。

#### 证书类（Certificate）

| 过滤器 | 语法 | 说明 | 权限 |
|--------|------|------|------|
| `cert` | `cert="关键词"` | 按证书内容查询 | [注册] |
| `cert.subject` | `cert.subject="持有者"` | 按证书持有者查询 | `=` `!=` `*=` |
| `cert.issuer` | `cert.issuer="颁发者"` | 按证书颁发者查询 | `=` `!=` `*=` |
| `cert.subject.org` | `cert.subject.org="组织"` | 按持有者组织查询 | `=` `!=` `*=` |
| `cert.subject.cn` | `cert.subject.cn="通用名"` | 按持有者通用名称查询 | `=` `!=` `*=` |
| `cert.issuer.org` | `cert.issuer.org="组织"` | 按颁发者组织查询 | `=` `!=` `*=` |
| `cert.issuer.cn` | `cert.issuer.cn="通用名"` | 按颁发者通用名称查询 | `=` `!=` `*=` |
| `cert.domain` | `cert.domain="域名"` | 按证书持有者根域名查询 | `=` `!=` `*=` |
| `cert.sn` | `cert.sn="序列号"` | 按证书序列号查询 | `=` `!=` |
| `cert.is_equal` | `cert.is_equal=true/false` | 筛选颁发者与持有者是否匹配 | [个人] |
| `cert.is_valid` | `cert.is_valid=true/false` | 筛选证书是否有效 | [个人] |
| `cert.is_match` | `cert.is_match=true/false` | 筛选证书与域名是否匹配 | [个人] |
| `cert.is_expired` | `cert.is_expired=true/false` | 筛选证书是否过期 | [个人] |
| `jarm` | `jarm="指纹值"` | 按JARM指纹查询 | `=` `!=` `*=` |
| `tls.version` | `tls.version="版本"` | 按TLS协议版本查询 | [个人] |
| `tls.ja3s` | `tls.ja3s="指纹值"` | 按JA3S指纹查询 | `=` `!=` `*=` |
| `cert.not_after.after` | `cert.not_after.after="日期"` | 筛选证书到期日之后 | [个人] |
| `cert.not_after.before` | `cert.not_after.before="日期"` | 筛选证书到期日之前 | [个人] |
| `cert.not_before.after` | `cert.not_before.after="日期"` | 筛选证书生效日之后 | [个人] |
| `cert.not_before.before` | `cert.not_before.before="日期"` | 筛选证书生效日之前 | [个人] |

#### 时间类（Last update time）

| 过滤器 | 语法 | 说明 | 权限 |
|--------|------|------|------|
| `after` | `after="YYYY-MM-DD"` | 筛选指定日期之后有更新的资产 | [个人] |
| `before` | `before="YYYY-MM-DD"` | 筛选指定日期之前有更新的资产 | [个人] |

#### 独立IP语法（商业版及以上）

需配合 `ip_filter` / `ip_exclude` 组合使用，用于同源IP多资产特征匹配。

| 过滤器 | 语法 | 说明 |
|--------|------|------|
| `ip_filter()` | `ip_filter(banner="SSH-2.0")` | 同源IP多资产特征匹配 |
| `ip_exclude()` | `ip_exclude(title="EdgeOS")` | 同源IP多资产特征不匹配 |
| `port_size` | `port_size="6"` | 筛选开放端口数量等于N的独立IP |
| `port_size_gt` | `port_size_gt="6"` | 筛选开放端口数量大于N的独立IP |
| `port_size_lt` | `port_size_lt="12"` | 筛选开放端口数量小于N的独立IP |
| `ip_ports` | `ip_ports="80,161"` | 筛选同时开放指定端口的独立IP |
| `ip_country` | `ip_country="CN"` | 按国家查询独立IP |
| `ip_region` | `ip_region="Zhejiang"` | 按省份查询独立IP |
| `ip_city` | `ip_city="Hangzhou"` | 按城市查询独立IP |
| `ip_after` | `ip_after="2021-03-18"` | 筛选指定日期之后有更新的独立IP |
| `ip_before` | `ip_before="2019-09-09"` | 筛选指定日期之前有更新的独立IP |

**语法注意事项**：
- `=` 精确匹配、`*=` 模糊匹配（支持 `*` 和 `?` 通配符）；`=`=`""` 查不存在/空值，`==""` 查存在且为空，`!=""` 查非空。
- `icon_hash` 使用 MMH3 算法计算 favicon 的哈希值，可通过 `mmh3-base64` 工具计算。
- `fid` 是 FOFA 聚合的站点指纹ID，用于精确匹配特定应用。
- 大部分**协议类**和**证书类**字段仅付费版本可用，免费用户主要使用基础类和部分网站类字段。
- FOFA 不区分大小写的国家/省份代码（`CN` 等同 `cn`）。

---

### 2.3 Hunter（奇安信鹰图）

Hunter 是奇安信推出的网络空间测绘平台，使用 `类别.字段` 命名方式，同时支持短格式别名。

**官方文档**: https://hunter.qianxin.com/

> 权限标注：无标注 = 免费可用；`[付费]` = 需有付费记录（特色语法，仅扣除权益积分）。
> Hunter 全面支持短格式别名：`title`=`web.title`、`body`=`web.body`、`port`=`ip.port`、`country`=`ip.country`、`province`=`ip.province`、`city`=`ip.city`、`isp`=`ip.isp`、`os`=`ip.os`、`server`=`header.server`、`status_code`=`header.status_code`、`app`=`app.name`、`asn`=`asn`、`domain`=`domain`、`icon`=`web.icon`、`icp_number`=`icp.number`、`icp_web_name`=`icp.web_name`、`icp_company_name`=`icp.name`、`icp_company_type`=`icp.type`。

#### 逻辑连接符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `=` | 模糊查询（包含关键词） | `title="北京"` |
| `==` | 精确查询（有且仅有关键词） | `server=="Microsoft-IIS/10"` |
| `!=` | 模糊剔除（`!=""` 查非空） | `domain!="app"` |
| `>` / `<` / `>=` / `<=` | 数值比较（如 `port_count`） | `port_count>"2"` |
| `&&` | 逻辑与 | `title="admin" && port="80"` |
| `\|\|` | 逻辑或 | `port="80" \|\| port="443"` |
| `()` | 分组，括号内容优先级最高 | `(port="80" \|\| port="443") && country="CN"` |
| `=""` | 查询字段为空 | `icp.name=""` |
| `!=""` | 查询字段非空 | `title!=""` |

#### IP 资产

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip="IP/CIDR"` | 按IP或C段搜索 | `ip="1.1.1.1"` / `ip="220.181.111.0/24"` |
| `ip.port` | `ip.port="端口"` | 按端口号搜索 | `ip.port="80"` |
| `ip.country` | `ip.country="国家"` | 按国家搜索（支持中文/代码） | `ip.country="CN"` |
| `ip.province` | `ip.province="省份"` | 按省份搜索 | `ip.province="江苏"` |
| `ip.city` | `ip.city="城市"` | 按城市搜索 | `ip.city="北京"` |
| `ip.isp` | `ip.isp="运营商"` | 按运营商搜索 | `ip.isp="电信"` |
| `ip.os` | `ip.os="系统"` | 按操作系统搜索 | `ip.os="Windows"` |
| `ip.port_count` | `ip.port_count>"N"` | 按开放端口数量筛选（支持 `>` `<` `>=` `<=` `=`） | `ip.port_count>"2"` |
| `ip.ports` | `ip.ports="端口"` | 多端口组合查询 | `ip.ports="80" && ip.ports="443"` |
| `ip.tag` | `ip.tag="标签"` | 按IP标签搜索 | [付费] |
| `is_domain` | `is_domain=true` | 搜索有域名标记的资产 | `is_domain=true` |
| `is_web` | `is_web=true` | 搜索web资产 | `is_web=true` |

#### 域名（Domain）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `domain` | `domain="关键词"` | 按域名搜索（含子域名） | `domain="qianxin"` |
| `domain.suffix` | `domain.suffix="主域"` | 按主域搜索 | `domain.suffix="qianxin.com"` |
| `domain.cname` | `domain.cname="CNAME"` | 按CNAME记录搜索 | `domain.cname="example.com"` |
| `domain.registrant_email` | `domain.registrant_email=="邮箱"` | 按域名注册邮箱搜索 | [付费] |
| `domain.status` | `domain.status="状态"` | 按域名状态搜索 | [付费] |
| `domain.whois_server` | `domain.whois_server="服务器"` | 按WHOIS服务器搜索 | [付费] |
| `domain.name_server` | `domain.name_server="NS"` | 按名称服务器搜索 | [付费] |
| `domain.creation_date` | `domain.creation_date="日期"` | 按域名创建时间搜索 | [付费] |
| `domain.expiry_date` | `domain.expiry_date="日期"` | 按域名到期时间搜索 | [付费] |
| `domain.updated_date` | `domain.updated_date="日期"` | 按域名更新时间搜索 | [付费] |

#### 响应头（Header）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `header` | `header="关键词"` | 按HTTP响应头搜索 | `header="elastic"` |
| `header.server` | `header.server=="全名"` | 按Server头精确搜索 | `header.server=="Microsoft-IIS/10"` |
| `header.status_code` | `header.status_code="状态码"` | 按HTTP状态码搜索 | `header.status_code="402"` |
| `header.content_length` | `header.content_length="大小"` | 按响应体大小搜索 | `header.content_length="691"` |

#### 网站信息（Web）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `web.title` | `web.title="关键词"` | 按网页标题搜索 | `web.title="北京"` |
| `web.body` | `web.body="关键词"` | 按网页正文内容搜索 | `web.body="网络空间测绘"` |
| `web.icon` | `web.icon="MD5"` | 按网站icon MD5哈希搜索 | `web.icon="22eeab765346f14faf564a4709f98548"` |
| `web.similar` | `web.similar="host:port"` | 按网络特征搜索相似资产 | [付费] |
| `web.similar_icon` | `web.similar_icon=="哈希"` | 搜索icon相似资产 | [付费] |
| `web.similar_id` | `web.similar_id="ID"` | 搜索相似网页资产 | [付费] |
| `web.tag` | `web.tag="标签"` | 按资产标签搜索（如 "登录页面"） | [付费] |
| `web.is_vul` | `web.is_vul=true` | 搜索存在历史漏洞的资产 | `web.is_vul=true` |
| `after` | `after="YYYY-MM-DD"` | 搜索指定日期之后的资产 | `after="2021-01-01"` |
| `before` | `before="YYYY-MM-DD"` | 搜索指定日期之前的资产 | `before="2021-12-31"` |

#### 协议/服务

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `protocol` | `protocol="协议名"` | 按协议名称搜索 | `protocol="http"` |
| `protocol.transport` | `protocol.transport="传输层"` | 按传输层协议搜索 | `protocol.transport="udp"` |
| `protocol.banner` | `protocol.banner="关键词"` | 按端口响应内容搜索 | `protocol.banner="nginx"` |
| `app.name` | `app.name="应用名"` | 按应用名搜索 | `app.name="小米 Router"` |
| `app.type` | `app.type="分类"` | 按组件分类搜索 | `app.type="开发与运维"` |
| `app.vendor` | `app.vendor="厂商"` | 按组件厂商搜索 | `app.vendor="PHP"` |
| `app.version` | `app.version="版本"` | 按组件版本搜索 | `app.version="1.8.1"` |

#### ICP 备案

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `icp.number` | `icp.number="备案号"` | 按ICP备案号搜索 | `icp.number="京ICP备16020626号-8"` |
| `icp.name` | `icp.name="公司名"` | 按ICP备案单位名搜索 | `icp.name="奇安信"` |
| `icp.web_name` | `icp.web_name="网站名"` | 按ICP网站名搜索 | `icp.web_name="奇安信"` |
| `icp.type` | `icp.type="主体类型"` | 按备案主体类型搜索 | `icp.type="企业"` |
| `icp.industry` | `icp.industry="行业"` | 按备案行业搜索 | `icp.industry="软件和信息技术服务业"` |
| `icp.province` | `icp.province="省份"` | 按备案企业注册省份搜索 | `icp.province="江苏"` |
| `icp.city` | `icp.city="城市"` | 按备案企业注册城市搜索 | `icp.city="上海"` |
| `icp.district` | `icp.district="区县"` | 按备案企业注册区县搜索 | `icp.district="杨浦"` |
| `icp.is_exception` | `icp.is_exception=true` | 搜索ICP备案异常资产 | [付费] |

#### 证书（Certificate）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `cert` | `cert="关键词"` | 按证书内容搜索 | `cert="baidu"` |
| `cert.subject` | `cert.subject="关键词"` | 按证书持有者搜索 | `cert.subject="qianxin.com"` |
| `cert.subject.suffix` | `cert.subject.suffix="域名"` | 按证书持有者精确搜索 | `cert.subject.suffix="qianxin.com"` |
| `cert.subject_org` | `cert.subject_org="组织"` | 按持有者组织搜索 | `cert.subject_org="奇安信科技"` |
| `cert.issuer` | `cert.issuer="关键词"` | 按证书颁发者搜索 | `cert.issuer="Let's Encrypt"` |
| `cert.issuer_org` | `cert.issuer_org="组织"` | 按颁发者组织搜索 | `cert.issuer_org="Let's Encrypt"` |
| `cert.sha-1` | `cert.sha-1="哈希"` | 按SHA-1哈希搜索 | `cert.sha-1="be7605a3..."` |
| `cert.sha-256` | `cert.sha-256="哈希"` | 按SHA-256哈希搜索 | `cert.sha-256="4e529a65..."` |
| `cert.sha-md5` | `cert.sha-md5="哈希"` | 按MD5哈希搜索 | `cert.sha-md5="aeedfb3c..."` |
| `cert.serial_number` | `cert.serial_number="序列号"` | 按证书序列号搜索 | `cert.serial_number="35351242..."` |
| `cert.is_expired` | `cert.is_expired=true` | 搜索证书已过期的资产 | [付费] |
| `cert.is_trust` | `cert.is_trust=true` | 搜索证书可信的资产 | [付费] |

#### ASN / 漏洞 / TLS-JARM

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `asn` | `asn="ASN号"` | 按ASN号搜索 | `asn="136800"` |
| `as.name` | `as.name="ASN名"` | 按ASN名称搜索 | `as.name="CLOUDFLARENET"` |
| `as.org` | `as.org="注册机构"` | 按ASN注册机构搜索 | `as.org="PDR"` |
| `vul.cve` | `vul.cve="CVE编号"` | 按CVE漏洞搜索 | `vul.cve="CVE-2021-2194"` |
| `vul.gev` | `vul.gev="GEV编号"` | 按奇安信专项漏洞搜索 | `vul.gev="GEV-2021-1075"` |
| `vul.state` | `vul.state="状态"` | 按漏洞修复状态搜索 | `vul.state="已修复"` |
| `tls-jarm.hash` | `tls-jarm.hash="哈希"` | 按JARM指纹哈希搜索 | `tls-jarm.hash="21d19d00..."` |

#### 地理位置 / 组织

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `country` | `country="国家"` | 按国家搜索（短格式） | `country="CN"` |
| `province` | `province="省份"` | 按省份搜索（短格式） | `province="江苏"` |
| `city` | `city="城市"` | 按城市搜索（短格式） | `city="北京"` |
| `org` | `org="组织名"` | 按组织名搜索 | `org="China Telecom"` |

**语法注意事项**：
- Hunter 使用 `类别.字段` 命名方式（如 `web.title`、`ip.port`），同时支持短格式别名（如 `title`、`port`）。
- `web.icon` 使用 MD5 哈希，与 FOFA 的 `icon_hash`（MMH3格式）不同。
- **数值比较**：支持 `>`、`<`、`>=`、`<=` 用于数值字段（如 `ip.port_count`）。
- **空值判断**：`=""` 查询字段为空、`!=""` 查询字段非空（仅支持 title、domain、icp.name）。
- **付费语法**：特色语法/功能需有付费记录，包括 `ip.tag`、`web.similar*`、`web.tag`、`icp.is_exception`、`cert.is_expired`、`cert.is_trust`、`domain.*`（whois 相关）。
- `server` 短格式等同 `header.server`（**注意**：不是 `server` 直接字段，而是 `header.server` 的别名）。

---

### 2.4 ZoomEye（知道创宇）

ZoomEye 是由知道创宇推出的网络空间搜索引擎，搜索范围覆盖设备（IPv4/IPv6）及网站（域名），全局模式匹配涵盖 HTTP/SSH/FTP 等多种协议。

**官方文档**: https://www.zoomeye.org/

#### 逻辑连接符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `=` | 模糊搜索（包含关键词，分词匹配，不区分大小写） | `title="知道创宇"` |
| `==` | 精准搜索（完全匹配，区分大小写，可搜空值） | `title=="知道创宇"` |
| `!=` | 非（排除包含关键词的资产） | `subdivisions!="beijing"` |
| `&&` | 逻辑与 | `device="router" && after="2020-01-01"` |
| `\|\|` | 逻辑或 | `service="ssh" \|\| service="http"` |
| `()` | 分组，括号内容优先级最高 | `(country="CN" && port!=80) \|\| (country="US")` |
| `*` | 模糊匹配（通配符） | `title="google*"` |

> **注意**：ZoomEye 使用 `=` 作为键值分隔符（非 `:`），值需用引号包裹。搜索字符串不区分大小写（`==` 除外），会进行分词匹配。含引号的值用 `\` 转义，含括号的值也需转义（如 `portinfo\(\)`）。
> 搜索不区分大小写，使用 `==` 时严格匹配大小写。

#### IP 及域名

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip="IP地址"` | 按IPv4/IPv6地址搜索 | `ip="8.8.8.8"` |
| `cidr` | `cidr="IP/掩码"` | 按CIDR网段搜索 | `cidr="52.2.254.36/24"` |
| `hostname` | `hostname="主机名"` | 按主机名搜索 | `hostname="google.com"` |
| `site` | `site="域名"` | 按站点域名搜索（域名和子域名） | `site="baidu.com"` |
| `domain` | `domain="域名"` | 按域名搜索 | `domain="baidu.com"` |
| `asn` | `asn=ASN号` | 按ASN号搜索 | `asn=42893` |
| `org` | `org="组织名"` | 按组织搜索（同 `organization`） | `org="北京大学"` |
| `isp` | `isp="ISP名"` | 按ISP搜索 | `isp="China Mobile"` |
| `port` | `port=端口号` | 按端口号搜索 | `port=80` |

#### 指纹/应用

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `app` | `app="应用名"` | 按应用/组件名搜索 | `app="Cisco ASA SSL VPN"` |
| `service` | `service="服务名"` | 按服务协议搜索 | `service="ssh"` |
| `device` | `device="设备类型"` | 按设备类型搜索 | `device="router"` |
| `os` | `os="操作系统"` | 按操作系统搜索 | `os="RouterOS"` |
| `title` | `title="关键词"` | 按网页标题搜索 | `title="Cisco"` |
| `product` | `product="产品名"` | 按组件信息搜索 | `product="Cisco"` |
| `protocol` | `protocol="传输协议"` | 按传输协议搜索 | `protocol="TCP"` |
| `industry` | `industry="行业"` | 按行业类型搜索 | `industry="政府"` |
| `is_honeypot` | `is_honeypot="True"` | 筛选蜜罐资产 | `is_honeypot="True"` |

#### HTTP 信息

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `http.header` | `http.header="关键词"` | 按HTTP响应头搜索 | `http.header="http"` |
| `http.header.server` | `http.header.server="服务器"` | 按Server头搜索 | `http.header.server="Nginx"` |
| `http.header.status_code` | `http.header.status_code="状态码"` | 按HTTP状态码搜索 | `http.header.status_code="200"` |
| `http.header.version` | `http.header.version="版本"` | 按HTTP服务版本搜索 | `http.header.version="1.2"` |
| `http.header_hash` | `http.header_hash="哈希"` | 按响应头哈希搜索 | `http.header_hash="27f9973f..."` |
| `http.body` | `http.body="关键词"` | 按HTML正文搜索 | `http.body="document"` |
| `http.body_hash` | `http.body_hash="哈希"` | 按HTML正文哈希搜索 | `http.body_hash="84a18166..."` |

#### 证书（SSL）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ssl` | `ssl="关键词"` | 按SSL证书信息搜索 | `ssl="google"` |
| `ssl.cert.subject.cn` | `ssl.cert.subject.cn="CN"` | 按证书持有者通用域名搜索 | `ssl.cert.subject.cn="example.com"` |
| `ssl.cert.issuer.cn` | `ssl.cert.issuer.cn="CN"` | 按证书签发者通用域名搜索 | `ssl.cert.issuer.cn="pbx.wildix.com"` |
| `ssl.cert.serial` | `ssl.cert.serial="序列号"` | 按证书序列号搜索 | `ssl.cert.serial="18460192..."` |
| `ssl.cert.fingerprint` | `ssl.cert.fingerprint="指纹"` | 按证书指纹搜索 | `ssl.cert.fingerprint="F3C98F22..."` |
| `ssl.cert.alg` | `ssl.cert.alg="算法"` | 按签名算法搜索 | `ssl.cert.alg="SHA256-RSA"` |
| `ssl.cert.pubkey.type` | `ssl.cert.pubkey.type="类型"` | 按公钥类型搜索 | `ssl.cert.pubkey.type="RSA"` |
| `ssl.cert.pubkey.rsa.bits` | `ssl.cert.pubkey.rsa.bits=位数` | 按RSA公钥位数搜索 | `ssl.cert.pubkey.rsa.bits=2048` |
| `ssl.cert.pubkey.ecdsa.bits` | `ssl.cert.pubkey.ecdsa.bits=位数` | 按ECDSA公钥位数搜索 | `ssl.cert.pubkey.ecdsa.bits=256` |
| `ssl.chain_count` | `ssl.chain_count=数量` | 按SSL链计数搜索 | `ssl.chain_count=3` |
| `ssl.version` | `ssl.version="版本"` | 按SSL/TLS版本搜索 | `ssl.version="TLSv1.3"` |
| `ssl.cipher.name` | `ssl.cipher.name="名称"` | 按加密套件名称搜索 | `ssl.cipher.name="TLS_AES_128_GCM_SHA256"` |
| `ssl.cipher.version` | `ssl.cipher.version="版本"` | 按加密套件版本搜索 | `ssl.cipher.version="TLSv1.3"` |
| `ssl.cipher.bits` | `ssl.cipher.bits="位数"` | 按加密套件位数搜索 | `ssl.cipher.bits="128"` |
| `ssl.jarm` | `ssl.jarm="指纹"` | 按JARM指纹搜索 | `ssl.jarm="29d29d15..."` |
| `ssl.ja3s` | `ssl.ja3s="指纹"` | 按JA3S指纹搜索 | `ssl.ja3s="45094d08..."` |

#### 地理位置

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `country` | `country="国家"` | 按国家搜索（代码或中/英文全称） | `country="CN"` / `country="中国"` |
| `subdivisions` | `subdivisions="行政区"` | 按行政区搜索（中/英文） | `subdivisions="beijing"` / `subdivisions="北京"` |
| `city` | `city="城市"` | 按城市搜索（中/英文） | `city="changsha"` / `city="长沙"` |

#### ICP 备案

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `icp.number` | `icp.number="备案号"` | 按ICP备案号搜索 | `icp.number="京ICP备10040895号-40"` |
| `icp.name` | `icp.name="企业名"` | 按ICP备案企业名搜索 | `icp.name="知道创宇"` |

#### 时间 / 其他

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `after` | `after="YYYY-MM-DD"` | 搜索更新时间在指定日期之后的资产 | `after="2020-01-01"` |
| `before` | `before="YYYY-MM-DD"` | 搜索更新时间在指定日期之前的资产 | `before="2020-01-01"` |
| `banner` | `banner="关键词"` | 按协议报文搜索（非HTTP协议） | `banner="FTP"` |
| `iconhash` | `iconhash="哈希"` | 按favicon图标哈希搜索（MD5或MMH3） | `iconhash="f3418a443e7d..."` |
| `dig` | `dig="内容"` | 按dig解析内容搜索 | `dig="baidu.com 220.181.38.148"` |
| `filehash` | `filehash="哈希"` | 按上传文件解析内容搜索 | `filehash="0b5ce08d..."` |

**语法注意事项**：
- ZoomEye 使用 **`=`** 作为键值分隔符（非 `:`），值必须用引号包裹。
- 搜索字符串默认分词匹配且不区分大小写，`==` 精准匹配严格区分大小写。
- 含引号的值用 `\` 转义（如 `"a\"b"`），含括号的值也需转义（如 `portinfo\(\)`）。
- `after`/`before` 需与其他过滤器组合使用，不可单独使用。
- ZoomEye 区分"主机搜索"和"Web搜索"两种模式，部分字段仅在特定模式下可用。

---

### 2.5 Quake（360网络空间测绘）

Quake 是360推出的网络空间测绘平台，使用 `:` 分隔符、英文逻辑词 `AND`/`OR`/`NOT`，值需双引号包裹。

**官方文档**: https://quake.360.net/

#### 逻辑连接符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `AND` | 逻辑与（空格默认分词） | `port:"80" AND app:"Apache"` |
| `OR` | 逻辑或 | `title:"管理" OR body:"login"` |
| `NOT` | 逻辑非 | `title:"login" AND (NOT title:"管理")` |
| `""` | 精确匹配（禁用分词） | `country:"United States"`（不加引号会拆为 United + States） |
| `()` | 分组，括号内容优先级最高 | `(port:"80" OR port:"443") AND country:"CN"` |
| `[N TO M]` | 数值区间 | `port:[50 TO 60]` |

> **注意**：Quake 默认对含空格的值进行分词（`United States` → `United` + `States`），必须用双引号包裹才能精确匹配。

#### 基础信息

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:"IP/CIDR"` | 按IP或CIDR网段搜索 | `ip:"1.1.1.1/22"` |
| `port` | `port:"端口"` 或 `port:[N TO M]` | 按端口号搜索，支持区间 | `port:"80"` / `port:[50 TO 60]` |
| `transport` | `transport:"协议"` | 按传输层协议搜索 | `transport:"udp"` |
| `domain` | `domain:"域名"` | 按域名搜索 | `domain:"google.com"` |
| `asn` | `asn:"ASN号"` | 按ASN号搜索 | `asn:"12345"` |
| `org` | `org:"组织名"` | 按组织名搜索 | `org:"No.31,Jin-rong Street"` |
| `hostname` | `hostname:"主机名"` | 按主机名搜索 | `hostname:"unifiedlayer.com"` |
| `is_ipv6` | `is_ipv6:"true/false"` | 搜索IPv6资产 | `is_ipv6:"true"` |

#### 协议/服务

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `service` | `service:"服务名"` | 按服务名搜索 | `service:"http"` |
| `app` | `app:"应用名"` | 按应用名搜索 | `app:"Apache"` |
| `response` | `response:"关键词"` | 按HTTP响应体内容搜索 | `response:"Qihoo Technology"` |
| `title` | `title:"关键词"` | 按网页标题搜索 | `title:"Management System"` |
| `server` | `server:"服务器名"` | 按HTTP Server头搜索 | `server:"Apache"` |
| `cert` | `cert:"关键词"` | 按SSL证书内容搜索 | `cert:"Qihoo"` |

#### 地理位置

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `country` | `country:"国家英文名"` | 按国家搜索（英文） | `country:"China"` |
| `country_cn` | `country_cn:"中文名"` | 按国家搜索（中文） | `country_cn:"中国"` |
| `province` | `province:"省份英文名"` | 按省份搜索（英文） | `province:"Sichuan"` |
| `province_cn` | `province_cn:"中文名"` | 按省份搜索（中文） | `province_cn:"四川"` |
| `city` | `city:"城市英文名"` | 按城市搜索（英文） | `city:"Chengdu"` |
| `city_cn` | `city_cn:"中文名"` | 按城市搜索（中文） | `city_cn:"成都"` |

#### 归属/备案

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `owner` | `owner:"归属者"` | 按资产归属者搜索 | `owner:"tencent.com"` |
| `isp` | `isp:"ISP名"` | 按ISP搜索 | `isp:"amazon.com"` |
| `icp_nature` | `icp_nature:"性质"` | 按ICP备案主体性质搜索 | `icp_nature:"企业"` |

#### 布尔过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `is_domain` | `is_domain:true/false` | 搜索有/无域名的资产 | `is_domain:true` |
| `is_ipv6` | `is_ipv6:"true/false"` | 搜索IPv6资产 | `is_ipv6:"true"` |

**语法注意事项**：
- Quake 使用 `:` 作为键值分隔符，值必须用双引号包裹。
- 逻辑运算符使用英文 `AND`/`OR`/`NOT`（非 `&&`/`||`/`!=`）。
- **正文内容**字段名为 `response`（非 `body`），这是 Quake 特有的命名。
- 地理位置支持中/英双语字段：`country`/`country_cn`、`province`/`province_cn`、`city`/`city_cn`。
- **空格分词**：含空格的值必须用双引号包裹，否则会被分词查询。
- 范围查询：`port:[50 TO 60]`（含端点）。

---

### 2.6 Censys

Censys 是由密歇根大学研究人员创建的网络空间搜索引擎，专注于主机和证书的搜索与分析。

**官方文档**: https://docs.censys.com/docs/censys-query-language

#### 查询语法

Censys 使用结构化查询语言，支持主机（Hosts）、服务（Services）和证书（Certificates）三大类搜索。

##### 主机搜索（Hosts）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:地址` | 按IP地址搜索 | `ip:1.1.1.1` |
| `location.country_code` | `location.country_code:代码` | 按国家代码搜索 | `location.country_code:CN` |
| `location.city` | `location.city:城市` | 按城市搜索 | `location.city:"Beijing"` |
| `location.province` | `location.province:省份` | 按省份搜索 | `location.province:"Beijing"` |
| `autonomous_system.asn` | `autonomous_system.asn:ASN` | 按ASN号搜索 | `autonomous_system.asn:4134` |
| `autonomous_system.name` | `autonomous_system.name:名称` | 按AS名称搜索 | `autonomous_system.name:"China Telecom"` |
| `operating_system` | `operating_system:名称` | 按操作系统搜索 | `operating_system:"Linux"` |

##### 服务搜索（Services）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `services.port` | `services.port:端口` | 按端口号搜索 | `services.port:80` |
| `services.service_name` | `services.service_name:名称` | 按服务名搜索 | `services.service_name:"HTTP"` |
| `services.transport_protocol` | `services.transport_protocol:协议` | 按传输协议搜索 | `services.transport_protocol:"TCP"` |
| `services.banner` | `services.banner:内容` | 按Banner信息搜索 | `services.banner:"SSH-2.0"` |
| `services.http.response.html_title` | `services.http.response.html_title:标题` | 按HTTP页面标题搜索 | `services.http.response.html_title:"admin"` |
| `services.http.response.body` | `services.http.response.body:内容` | 按HTTP响应体搜索 | `services.http.response.body:"login"` |
| `services.http.response.status_code` | `services.http.response.status_code:码` | 按HTTP状态码搜索 | `services.http.response.status_code:200` |
| `services.http.response.headers` | `services.http.response.headers:内容` | 按HTTP响应头搜索 | `services.http.response.headers.raw:"Server: nginx"` |
| `services.tls.certificates.leaf` | `services.tls.certificates.leaf:内容` | 按TLS证书搜索 | `services.tls.certificates.leaf.subject:"example.com"` |
| `services.tls.ja3s` | `services.tls.ja3s:哈希` | 按JA3S指纹搜索 | `services.tls.ja3s:"e7d705a3..."` |
| `services.tls.jarm` | `services.tls.jarm:指纹` | 按JARM指纹搜索 | `services.tls.jarm:"07d14d16d21d..."` |
| `services.software.product` | `services.software.product:名称` | 按软件产品搜索 | `services.software.product:"Apache"` |
| `services.software.version` | `services.software.version:版本` | 按软件版本搜索 | `services.software.version:"2.4.49"` |

##### 证书搜索（Certificates）

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `parsed.subject.common_name` | `parsed.subject.common_name:CN` | 按证书主题CN搜索 | `parsed.subject.common_name:"example.com"` |
| `parsed.issuer.common_name` | `parsed.issuer.common_name:CN` | 按证书颁发者CN搜索 | `parsed.issuer.common_name:"DigiCert"` |
| `parsed.subject.organization` | `parsed.subject.organization:组织` | 按证书主题组织搜索 | `parsed.subject.organization:"Google"` |
| `parsed.issuer.organization` | `parsed.issuer.organization:组织` | 按证书颁发者组织搜索 | `parsed.issuer.organization:"DigiCert Inc"` |
| `parsed.validity.start` | `parsed.validity.start:日期` | 按证书生效日期搜索 | `parsed.validity.start:"2024-01-01"` |
| `parsed.validity.end` | `parsed.validity.end:日期` | 按证书过期日期搜索 | `parsed.validity.end:"2025-01-01"` |
| `parsed.serial_number` | `parsed.serial_number:序列号` | 按证书序列号搜索 | `parsed.serial_number:"0a0b0c"` |
| `fingerprint.sha256` | `fingerprint.sha256:哈希` | 按SHA256指纹搜索 | `fingerprint.sha256:"a1b2c3..."` |
| `tags` | `tags:标签` | 按证书标签搜索 | `tags:"unexpired"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `AND` | 逻辑与 | `services.port:80 AND location.country_code:CN` |
| `OR` | 逻辑或 | `services.port:80 OR services.port:443` |
| `NOT` | 逻辑非 | `NOT location.country_code:US` |
| `=` | 等于 | `services.port:80` |
| `!=` | 不等于 | `services.port != 80` |
| `[]` | 数组/路径表示 | `services.http.response.html_title:"admin"` |
| `""` | 精确匹配 | `services.service_name:"HTTP"` |
| `*` | 通配符 | `parsed.subject.common_name:"*.example.com"` |
| `/{d}/` | 正则表达式 | `services.banner:/SSH-2\.0.*/` |

**注意事项**：
- Censys 使用层级结构化路径表示字段（如 `services.http.response.html_title`），与其他引擎的扁平命名方式不同。
- Censys 支持正则表达式搜索，使用 `/pattern/` 语法。
- Censys 的证书搜索功能非常强大，支持详细的证书字段搜索。

---

### 2.7 DayDayMap（盛邦安全）

> **官方语法核查**: 2026-06-08，基于用户提供的官方语法文档全量核实，14 大类 50+ 字段全部覆盖。

DayDayMap 是盛邦安全（WebRAY）推出的网络空间资产测绘平台，支持14大类、50+查询语法，并兼容其他平台语法。

**官方文档**: https://www.daydaymap.com/help/document?type=syntax-search

#### 基本规则

1. 请使用双引号 `"..."` 标记搜索字符串（如 `"1.1.1.1"`、`"Ubuntu Linux"` 等）。
2. 搜索字符串不区分大小写。
3. 支持兼容其他平台语法（如 `ip`、`port`、`domain`、`country`、`region` 等）。

#### IP 类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `is_ipv6` | `is_ipv6="true/false"` | 判断IP类型是否为IPv6 | `is_ipv6="true"` | - |
| `ip` | `ip="IP地址"` | 搜索指定IP或IP范围 | `ip="1.1.1.1"` | - |
| `ip` | `ip="CIDR"` | 搜索IP网段 | `ip="1.1.1.1/24"` | - |
| `ip` | `ip="IP范围"` | 搜索IP范围 | `ip="1.1.1.0-1.1.1.255"` | - |
| `ip.port` | `ip.port="端口"` | 搜索开放端口 | `ip.port="80"` | `port` |
| `ip.port` | `ip.port>="端口"` | 搜索大于等于指定端口 | `ip.port>="80"` | - |
| `ip.port` | `ip.port>"80" && ip.port<"1024"` | 搜索端口范围 | `ip.port>"80" && ip.port<"1024"` | - |
| `ip.isp` | `ip.isp="ISP名"` | 搜索IP的ISP | `ip.isp="电信"` | - |
| `ip.os_family` | `ip.os_family="系统类型"` | 搜索操作系统类型 | `ip.os_family="Windows"` | `os_family` |
| `ip.os` | `ip.os="操作系统"` | 搜索操作系统 | `ip.os="Windows Server 2016"` | - |
| `ip.tag` | `ip.tag="标签"` | 搜索IP标签 | `ip.tag="CDN"` | - |
| `ip.industry` | `ip.industry="行业"` | 搜索IP所属行业 | `ip.industry="银行"` | `industry` |

**ip.tag 支持的标签值**: CDN、蜜罐、Starlink、云厂商、终端截图等。

**ip.industry 支持的行业值**: 银行、教育、医疗、工业、金融等。

#### 域名类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `is_domain` | `is_domain="true/false"` | 搜索是否为域名资产 | `is_domain="true"` | - |
| `domain` | `domain="域名"` | 搜索域名及其子域名 | `domain="www.webray.com.cn"` | - |
| `domain.root` | `domain.root="主域名"` | 搜索主域名的所有子域名 | `domain.root="webray.com.cn"` | - |

#### 地理位置类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `ip.country` | `ip.country="国家"` | 搜索IP所属国家 | `ip.country="中国"` | `country` |
| `ip.province` / `ip.region` | `ip.province="省份"` | 搜索IP所属省份 | `ip.province="陕西省"` | `province`/`region` |
| `ip.city` | `ip.city="城市"` | 搜索IP所属城市 | `ip.city="北京市"` | `city` |
| `ip.district` / `ip.county` | `ip.district="区县"` | 搜索IP所属区县 | `ip.district="朝阳区"` | `district`/`county` |

**ip.country 支持**: 中文全称、中文简称、英文名、ISO2（两字母代码）、ISO3（三字母代码）、ISO数字代码。

**ip.province 支持**: 中文全称、简称、缩写、全拼、缩写拼音。

**ip.city 支持**: 直辖市、地级市、特别行政区（不含县级市和区县），支持中文全称、简称、全拼、缩写拼音。

#### ICP 类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `icp.number` | `icp.number="备案号"` | 搜索ICP备案号 | `icp.number="京ICP备17003970号"` | - |
| `icp.name` | `icp.name="公司名"` | 搜索ICP备案公司名 | `icp.name="远江盛邦"` | - |
| `icp.name_prefix` | `icp.name_prefix="前缀"` | 搜索ICP备案公司名（前缀匹配） | `icp.name_prefix="远江"` | - |
| `icp.webname` | `icp.webname="网站名"` | 搜索ICP备案网站名 | `icp.webname="盛邦安全"` | - |

#### 自治系统类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `asn.number` | `asn.number="ASN号"` | 搜索ASN号 | `asn.number="AS15169"` | - |
| `asn.org` | `asn.org="组织名"` | 搜索ASN实体名 | `asn.org="amazon"` | - |

#### Web 类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `is_web` | `is_web="true/false"` | 搜索是否为Web资产 | `is_web="true"` | `is_website` |
| `web.server` | `web.server="服务器"` | 搜索Web服务类型 | `web.server="Apache"` | `server` |
| `web.status_code` | `web.status_code="状态码"` | 搜索响应状态码 | `web.status_code="200"` | `status_code`/`code`/`http_status` |
| `web.header` | `web.header="关键词"` | 搜索响应头 | `web.header="elastic"` | `header`/`web.response`/`response` |
| `web.title` | `web.title="关键词"` | 搜索网站标题 | `web.title="北京"` | `title` |
| `web.lang` | `web.lang="语言"` | 搜索Web开发语言 | `web.lang="PHP"` | `lang` |
| `web.body` | `web.body="关键词"` | 搜索网页内容 | `web.body="网络空间测绘"` | `body` |
| `web.icon` | `web.icon="哈希值"` | 搜索图标（MD5哈希） | `web.icon="c60ea375c39d1ab273c4d1bee717287a"` | `icon` |

#### 协议类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `protocol.transport` | `protocol.transport="协议"` | 搜索传输层协议 | `protocol.transport="udp"` | `transport`/`protocol` |
| `protocol.service` | `protocol.service="服务"` | 搜索服务协议 | `protocol.service="http"` | `service` |
| `protocol.banner` | `protocol.banner="关键词"` | 搜索Banner详情 | `protocol.banner="nginx"` | `banner` |

#### 应用类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `app.name` | `app.name="应用名"` | 搜索应用名称 | `app.name="物联网平台"` | - |

#### 组件类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `product` | `product="组件名"` | 搜索组件名称 | `product="Nginx"` | - |

#### 设备类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `device.name` | `device.name="设备名"` | 搜索设备名称 | `device.name="Aruba Device"` | `device` |
| `device.type` | `device.type="设备类型"` | 搜索设备类型 | `device.type="安全防护设备"` | `device_type` |
| `device.type_sub` | `device.type_sub="子类型"` | 搜索设备子类型 | `device.type_sub="邮件安全系统"` | - |
| `brand` | `brand="品牌名"` | 搜索设备品牌 | `brand="Cisco"` | - |
| `model` | `model="型号"` | 搜索设备型号 | `model="Chromecast"` | - |
| `manufacturer` | `manufacturer="厂商"` | 搜索设备制造商 | `manufacturer="Hikvision"` | - |

#### 证书类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `cert.issuer` | `cert.issuer="关键词"` | 搜索证书颁发者 | `cert.issuer="Amazon"` | - |
| `cert.issuer.cn` | `cert.issuer.cn="CN"` | 搜索证书颁发者CN | `cert.issuer.cn="GeoTrust CN RSA CA G1"` | - |
| `cert.issuer.country` | `cert.issuer.country="代码"` | 搜索证书颁发者国家 | `cert.issuer.country="US"` | - |
| `cert.issuer.org` | `cert.issuer.org="组织"` | 搜索证书颁发者组织 | `cert.issuer.org="DigiCert Inc"` | - |
| `cert.subject` | `cert.subject="关键词"` | 搜索证书主题 | `cert.subject="Technicolor"` | - |
| `cert.subject.cn` | `cert.subject.cn="CN"` | 搜索证书主题CN | `cert.subject.cn="daydaymap.com"` | - |
| `cert.subject.country` | `cert.subject.country="代码"` | 搜索证书主题国家 | `cert.subject.country="CN"` | - |
| `cert.subject.org` | `cert.subject.org="组织"` | 搜索证书主题组织 | `cert.subject.org="DigiCert Inc"` | - |
| `cert.sn` | `cert.sn="序列号"` | 搜索证书序列号 | `cert.sn="0ECDAB152D2161F7C843D25F3F00FCDE"` | - |
| `cert.org` | `cert.org="组织"` | 搜索证书持有者组织 | `cert.org="Plesk"` | - |
| `cert.md5` | `cert.md5="MD5值"` | 搜索证书MD5 | `cert.md5="0aeb8908c10b3bff4b920bdb199eb09a"` | - |
| `cert.is_expired` | `cert.is_expired="true/false"` | 搜索过期/未过期证书 | `cert.is_expired="true"` | - |
| `cert.is_trust` | `cert.is_trust="true/false"` | 搜索可信/不可信证书 | `cert.is_trust="true"` | - |
| `cert.startdate` | `cert.startdate="日期"` | 搜索证书起始时间 | `cert.startdate="2024-01-01 00:00:00"` | `startdate` |
| `cert.enddate` | `cert.enddate="日期"` | 搜索证书到期时间 | `cert.enddate="2024-01-01 00:00:00"` | `enddate` |

**cert.startdate/cert.enddate 支持比较运算符**: `>`、`>=`、`<`、`<=`、`=`。

#### 时间类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `time` | `time="日期"` | 搜索资产更新时间 | `time="2024-01-01 08:00:00"` | `time_stamp` |
| `time` | `time>"日期"` | 搜索指定日期之后更新的资产 | `time>"2024-01-01"` | - |
| `time` | `time>="日期"` | 搜索指定日期及之后更新的资产 | `time>="2024-01-01"` | - |

#### 漏洞指纹类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `vul.cve` | `vul.cve="CVE编号"` | 搜索CVE漏洞（支持模糊匹配） | `vul.cve="CVE-2021-42013"` | `cve` |
| `vul.dvb` | `vul.dvb="DVB编号"` | 搜索DVB漏洞 | `vul.dvb="DVB-2021-2898"` | - |

#### 资产权属类

| 过滤器 | 语法 | 说明 | 示例 | 兼容语法 |
|--------|------|------|------|----------|
| `org.name` | `org.name="组织名"` | 搜索资产权属 | `org.name="远江盛邦"` | - |
| `org.name_prefix` | `org.name_prefix="前缀"` | 搜索资产权属（前缀匹配） | `org.name_prefix="远江"` | `org_prefix` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `&&` | 逻辑与 | `ip.port="80" && ip.country="中国"` |
| `\|\|` | 逻辑或 | `ip.port="80" \|\| ip.port="443"` |
| `!=` | 不等于 | `ip.port!="80"` |
| `>` / `>=` / `<` / `<=` | 比较运算符 | `ip.port>="80"` |
| `""` | 精确匹配 | `web.title="后台管理"` |

**注意事项**：
- DayDayMap 支持14大类、50+查询语法，是语法最丰富的国内网络空间搜索引擎之一。
- DayDayMap 兼容其他平台语法，如 `ip`、`port`、`domain`、`country`、`region`、`city`、`title`、`body`、`header`、`banner`、`server`、`service`、`icon` 等。
- DayDayMap 支持自然语言搜索，AI引擎可将自然语言转换为专业语法。
- `ip.port`、`cert.startdate`、`cert.enddate`、`time` 等字段支持比较运算符（`>`、`>=`、`<`、`<=`）。

---

## 三、威胁情报与恶意软件搜索引擎

### 3.1 VirusTotal Intelligence

VirusTotal Intelligence 是 VirusTotal 的高级搜索功能，支持对文件、URL、域名和IP地址进行高级搜索。

**官方文档**: https://docs.virustotal.com/docs/file-search-modifiers

#### 文件搜索修饰符

| 修饰符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `type` | `type:文件类型` | 按文件类型搜索 | `type:peexe` |
| `size` | `size:字节数` | 按文件大小搜索 | `size:1MB+` |
| `detections` | `detections:检测数` | 按杀软检测数搜索 | `detections:10+` |
| `name` | `name:"文件名"` | 按文件名搜索 | `name:"malware.exe"` |
| `tag` | `tag:标签` | 按标签搜索 | `tag:trojan` |
| `fs` | `fs:日期` | 按首次出现日期搜索 | `fs:2024-01-01+` |
| `itw` | `itw:URL` | 按In-The-Wild URL搜索 | `itw:"example.com"` |
| `have` | `have:属性` | 搜索具有指定属性的文件 | `have:imports` |
| `behaviour` | `behaviour:"行为"` | 按沙箱行为搜索 | `behaviour:"creates_mutant"` |
| `signature` | `signature:"签名"` | 按杀软签名搜索 | `signature:"Trojan.Generic"` |
| `engines` | `engines:引擎名:` | 按特定引擎检测结果搜索 | `engines:microsoft:trojan` |
| `pe` | `pe:PE属性` | 按PE文件属性搜索 | `pe:has_resources` |
| `creation_date` | `creation_date:日期` | 按文件创建日期搜索 | `creation_date:2024-01-01+` |
| `imphash` | `imphash:"哈希值"` | 按导入表哈希搜索 | `imphash:"abc123..."` |
| `main_icon_dhash` | `main_icon_dhash:"哈希值"` | 按图标哈希搜索 | `main_icon_dhash:"ghi789..."` |
| `ssdeep` | `ssdeep:"模糊哈希"` | 按SSDeep模糊哈希搜索 | `ssdeep:"3:AX..."` |
| `tlsh` | `tlsh:"哈希值"` | 按TLSH哈希搜索 | `tlsh:"1F2D3..."` |

#### URL搜索修饰符

| 修饰符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `positives` | `positives:检测数` | 按检测数搜索 | `positives:5+` |
| `status_code` | `status_code:状态码` | 按HTTP状态码搜索 | `status_code:200` |
| `url` | `url:"URL"` | 按URL搜索 | `url:"example.com/path"` |

#### 域名搜索修饰符

| 修饰符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `positives` | `positives:检测数` | 按检测数搜索 | `positives:5+` |
| `whois` | `whois:"关键词"` | 按WHOIS信息搜索 | `whois:"example"` |
| `creation_date` | `creation_date:日期` | 按域名创建日期搜索 | `creation_date:2024-01-01+` |

#### IP搜索修饰符

| 修饰符 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `positives` | `positives:检测数` | 按检测数搜索 | `positives:5+` |
| `country` | `country:"国家代码"` | 按国家搜索 | `country:"CN"` |
| `asn` | `asn:ASN号` | 按ASN号搜索 | `asn:4134` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `AND` | 逻辑与 | `type:peexe AND positives:10+` |
| `OR` | 逻辑或 | `tag:trojan OR tag:ransomware` |
| `NOT` | 逻辑非 | `type:peexe NOT tag:legit` |
| `+` / `-` | 大于/小于 | `positives:10+`（大于等于10） |
| `""` | 精确匹配 | `name:"malware.exe"` |
| `()` | 分组 | `(tag:trojan OR tag:ransomware) AND positives:5+` |

**注意事项**：
- VirusTotal Intelligence 需要付费订阅才能使用高级搜索功能。
- `type` 支持的文件类型包括：peexe、pedll、pdf、android、html、flash、office、script 等。
- `fs` 修饰符按首次出现日期过滤（非 `fsr`），`creation_date` 按创建日期过滤。
- 图标哈希搜索修饰符为 `main_icon_dhash`（非 `dhash`）。
- `have` 修饰符用于搜索具有特定属性的文件，如 `have:imports`、`have:exports`、`have:resources` 等。

---

## 四、DNS搜索引擎

### 4.1 DnsDB ⚠️ 已停用

> **⚠️ DnsDB 已不再使用**，服务已停止。以下语法仅供参考归档，不可用于实际查询。项目中不会实现 DnsDB 适配器。

DnsDB 曾是一个DNS历史记录搜索引擎，由微步在线运营。

**原官方文档**: https://www.dnsdb.io/ （已不可访问）

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `domain` | `domain:"域名"` | 按域名搜索DNS记录 | `domain:"example.com"` |
| `ip` | `ip:"IP地址"` | 按IP地址搜索DNS记录 | `ip:"1.1.1.1"` |
| `type` | `type:"记录类型"` | 按DNS记录类型搜索 | `type:"A"` |
| `value` | `value:"记录值"` | 按DNS记录值搜索 | `value:"1.1.1.1"` |
| `time` | `time:"日期范围"` | 按时间范围搜索 | `time:"2024-01-01,2024-12-31"` |

**支持的DNS记录类型**: A、AAAA、CNAME、MX、NS、TXT、SOA、PTR、SRV、CAA 等。

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格 | 逻辑与 | `domain:"example.com" type:"A"` |
| `""` | 精确匹配 | `domain:"example.com"` |

**注意事项**：
- DnsDB 主要用于查询DNS历史解析记录，可发现域名与IP之间的历史关联。
- 支持子域名搜索，如 `domain:"example.com"` 会返回所有子域名的解析记录。
- `time` 格式为 `开始日期,结束日期`。

---

## 五、语法对比速查表

以下为各网络空间搜索引擎常用查询语法的对比速查表：

| 功能 | Shodan | FOFA | Hunter | ZoomEye | Quake | Censys | DayDayMap |
|------|--------|------|--------|---------|-------|--------|-----------|
| **IP搜索** | `net:CIDR` 或直接输入 | `ip="IP"` | `ip="IP"` | `ip:"IP"` 或 `cidr:"CIDR"` | `ip:"IP"` | `ip:地址` | `ip="IP"` |
| **端口搜索** | `port:端口` | `port="端口"` | `ip.port="端口"` | `port:"端口"` | `port:"端口"` | `services.port:端口` | `ip.port="端口"` |
| **域名搜索** | `hostname:"域名"` | `domain="域名"` | `domain="域名"` / `domain.suffix="主域"` | `site="域名"` | `domain:"域名"` | - | `domain="域名"` |
| **标题搜索** | `http.title:"标题"` | `title="标题"` | `web.title="标题"` | `title:"标题"` | `title:"标题"` | `services.http.response.html_title:标题` | `web.title="标题"` |
| **正文搜索** | `http.html:"内容"` | `body="内容"` [注册] | `web.body="内容"` [注册] | `http.body="内容"` | `response:"内容"` | `services.http.response.body:内容` | `web.body="内容"` |
| **响应头搜索** | `http.server:"值"` | `header="值"` [注册] | `web.header="值"` [注册] | `header:"值"` | `headers:"值"` [注册] | `services.http.response.headers:值` | `web.header="值"` |
| **国家搜索** | `country:"代码"` | `country="代码"` | `ip.country="国家"` | `country:"代码"` | `country:"代码"` | `location.country_code:代码` | `ip.country="国家"` |
| **城市搜索** | `city:"城市"` | `city="城市"` | `ip.city="城市"` | `city:"城市"` | `city:"城市"` / `city_cn:"中文"` | `location.city:城市` | `ip.city="城市"` |
| **操作系统** | `os:"系统"` | `os="系统"` | `ip.os="系统"` | `os:"系统"` | `os:"系统"` | `operating_system:系统` | `ip.os="系统"` |
| **组织/ISP** | `org:"组织"` | `org="组织"` | `org="组织"` | `org:"组织"` | `org:"组织"` | `autonomous_system.name:名称` | `asn.org="组织"` |
| **ASN** | `asn:ASN` | `asn="ASN"` | `asn="ASN"` | `asn:"ASN"` | `asn:"ASN"` | `autonomous_system.asn:ASN` | `asn.number="ASN"` |
| **证书主题CN** | `ssl.cert.subject.cn:"CN"` | `cert.subject.cn="CN"` | `cert.subject.suffix="CN"` | `ssl.cert.subject.cn:"CN"` | `cert.subject.cn:"CN"` | `parsed.subject.common_name:CN` | `cert.subject.cn="CN"` |
| **证书颁发者CN** | `ssl.cert.issuer.cn:"CN"` | `cert.issuer.cn="CN"` | `cert.issuer="CN"` / `cert.issuer_org="CN"` | `ssl.cert.issuer.cn:"CN"` | `cert.issuer.cn:"CN"` | `parsed.issuer.common_name:CN` | `cert.issuer.cn="CN"` |
| **Favicon哈希** | `http.favicon.hash:值` | `icon_hash="值"` | `web.icon="值"` | `iconhash:"值"` | `favicon:"值"` | - | `web.icon="值"` |
| **漏洞搜索** | `vuln:CVE` | - | - | - | `vuln:"CVE"` | - | `vul.cve="CVE"` |
| **Banner搜索** | 直接输入 | `banner="值"` [注册] | `protocol.banner="值"` | `banner="值"` | `service:"值"` | `services.banner:值` | `protocol.banner="值"` |
| **逻辑与** | 空格 | `&&` 或 `AND` | `&&` 或 `AND` | 空格或`+` | 空格或`AND` | `AND` | `&&` |
| **逻辑或** | `OR` | `\|\|` 或 `OR` | `\|\|` 或 `OR` | `\|` | `OR` | `OR` | `\|\|` |
| **逻辑非** | `-` | `!=` | `!=` | `-` | `-` | `NOT` | `!=` |
| **键值分隔符** | `:` | `=` | `=` | `=` | `:` | `:` | `=` |

---

## 六、逻辑运算符与通配符通用规则

### 6.1 逻辑运算符

| 运算符 | 含义 | 通用说明 |
|--------|------|----------|
| `AND` / `&&` / 空格 | 逻辑与 | 所有条件必须同时满足 |
| `OR` / `\|\|` / `\|` | 逻辑或 | 任一条件满足即可 |
| `NOT` / `-` / `!=` | 逻辑非 | 排除指定条件 |
| `()` | 分组 | 改变运算优先级 |

### 6.2 通配符

| 通配符 | 含义 | 适用平台 |
|--------|------|----------|
| `*` | 匹配任意字符 | FOFA、Quake、Censys |
| `?` | 匹配单个字符 | 部分平台支持 |
| `..` | 数字范围 | Google |
| `""` | 精确匹配 | 所有平台 |

### 6.3 比较运算符

| 运算符 | 含义 | 适用平台 |
|--------|------|----------|
| `=` / `:` | 等于 | 所有平台 |
| `!=` | 不等于 | FOFA、Hunter、DayDayMap |
| `>` / `>=` | 大于/大于等于 | DayDayMap、Shodan |
| `<` / `<=` | 小于/小于等于 | DayDayMap、Shodan |
| `+` / `-` | 大于等于/小于等于 | VirusTotal |

### 6.4 正则表达式

| 平台 | 正则语法 | 示例 |
|------|----------|------|
| Censys | `/pattern/` | `services.banner:/SSH-2\.0.*/` |
| Shodan | 不直接支持正则 | - |
| FOFA | 不直接支持正则 | - |

---

> **免责声明**: 本文档仅供安全研究和合法授权测试使用。请遵守相关法律法规，不得将搜索引擎语法用于非法入侵、未授权访问等违法行为。使用本文档中的语法进行搜索时，请确保已获得目标系统的合法授权。
