# 搜索引擎语法基准参考手册

> **版本**: 2026-06  
> **说明**: 本文档汇总了各类搜索引擎（通用搜索引擎、网络空间搜索引擎、威胁情报搜索引擎）的查询语法，所有语法均经过官方文档或权威来源核实。  
> **注意**: 各平台语法可能随版本更新而变化，建议定期查阅官方文档确认最新语法。

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
  - [2.8 BinaryEdge](#28-binaryedge)
  - [2.9 Onyphe](#29-onyphe)
  - [2.10 GreyNoise](#210-greynoise)
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
| `cache` | `cache:URL` | 查看Google缓存的页面（已逐步弱化） | `cache:example.com` |
| `related` | `related:域名` | 搜索与指定网站相关的网站 | `related:example.com` |
| `info` | `info:域名` | 获取指定URL的Google索引信息 | `info:example.com` |
| `link` | `link:域名` | 搜索链接到指定网站的页面（已弱化） | `link:example.com` |
| `define` | `define:术语` | 搜索术语定义 | `define:phishing` |
| `OR` | `词1 OR 词2` | 逻辑或运算 | `site:example.com OR site:test.com` |
| `-`（减号） | `-关键词` | 排除包含指定关键词的结果 | `jaguar -car` |
| `""`（引号） | `"精确短语"` | 精确匹配短语 | `"admin panel"` |
| `*`（星号） | `词*` | 通配符，匹配任意词 | `how to * a website` |
| `..`（范围） | `数字1..数字2` | 数字范围搜索 | `2020..2024` |
| `AROUND(X)` | `词1 AROUND(X) 词2` | 两个词之间相隔不超过X个词 | `apple AROUND(3) iphone` |

**注意事项**：
- Google 已逐步弱化 `cache:`、`link:` 运算符，部分功能可能不再可用。
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

Shodan 是全球最早的网络空间搜索引擎，专注于互联网连接设备的发现与搜索。

**官方文档**: https://www.shodan.io/search/filters

#### 基础搜索过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `city` | `city:"城市名"` | 按城市搜索 | `city:"Beijing"` |
| `country` | `country:"国家代码"` | 按国家搜索（2字母ISO代码） | `country:"CN"` |
| `geo` | `geo:纬度,经度` | 按地理坐标搜索 | `geo:39.9042,116.4074` |
| `hostname` | `hostname:"域名"` | 按主机名搜索 | `hostname:"example.com"` |
| `net` | `net:CIDR` | 按IP网段搜索 | `net:192.168.1.0/24` |
| `org` | `org:"组织名"` | 按组织/ISP搜索 | `org:"China Telecom"` |
| `isp` | `isp:"ISP名"` | 按ISP搜索 | `isp:"China Unicom"` |
| `asn` | `asn:ASN号` | 按自治系统号搜索 | `asn:AS4134` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Windows Server 2016"` |
| `port` | `port:端口号` | 按端口号搜索 | `port:80` |
| `product` | `product:"产品名"` | 按产品名搜索 | `product:"Apache httpd"` |
| `version` | `version:"版本号"` | 按版本号搜索 | `version:"2.4.49"` |
| `vuln` | `vuln:CVE编号` | 按CVE漏洞搜索 | `vuln:CVE-2021-44228` |
| `has_screenshot` | `has_screenshot:true` | 搜索有截图的设备 | `has_screenshot:true` |
| `has_vuln` | `has_vuln:true` | 搜索有已知漏洞的设备 | `has_vuln:true` |
| `http.title` | `http.title:"标题"` | 按HTTP页面标题搜索 | `http.title:"admin"` |
| `http.html` | `http.html:"内容"` | 按HTTP页面内容搜索 | `http.html:"login"` |
| `http.status` | `http.status:状态码` | 按HTTP状态码搜索 | `http.status:200` |
| `http.server` | `http.server:"服务器"` | 按HTTP Server头搜索 | `http.server:"nginx"` |
| `http.location` | `http.location:"URL"` | 按HTTP Location头搜索 | `http.location:"/login"` |
| `http.favicon.hash` | `http.favicon.hash:哈希值` | 按favicon图标哈希搜索 | `http.favicon.hash:-247388890` |
| `ssl.cert.subject.cn` | `ssl.cert.subject.cn:"CN"` | 按SSL证书主题CN搜索 | `ssl.cert.subject.cn:"example.com"` |
| `ssl.cert.issuer.cn` | `ssl.cert.issuer.cn:"CN"` | 按SSL证书颁发者CN搜索 | `ssl.cert.issuer.cn:"Let's Encrypt"` |
| `ssl.cert.serial` | `ssl.cert.serial:序列号` | 按SSL证书序列号搜索 | `ssl.cert.serial:0A0B0C` |
| `ssl.version` | `ssl.version:版本` | 按SSL/TLS版本搜索 | `ssl.version:TLSv1.2` |
| `ssl.ja3.hash` | `ssl.ja3.hash:哈希值` | 按JA3指纹哈希搜索 | `ssl.ja3.hash:"e7d705a3286e19ea42f587b344ee6865"` |
| `ssl.jarm` | `ssl.jarm:指纹` | 按JARM指纹搜索 | `ssl.jarm:"07d14d16d21d21d07c42d41d00041d..."` |
| `ntp.ip` | `ntp.ip:IP` | 按NTP IP搜索 | `ntp.ip:"1.1.1.1"` |
| `ntp.port` | `ntp.port:端口` | 按NTP端口搜索 | `ntp.port:123` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格 | 逻辑与（AND） | `port:80 country:"CN"` |
| `OR` | 逻辑或 | `port:80 OR port:443` |
| `-` | 逻辑非（NOT） | `port:80 -country:"US"` |
| `""` | 精确匹配 | `product:"Apache httpd"` |

---

### 2.2 FOFA

FOFA 是国内使用最广泛的网络空间搜索引擎之一，由白帽汇运营。

**官方文档**: https://fofa.info/library

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `title` | `title="关键词"` | 按网页标题搜索 | `title="后台管理"` |
| `body` | `body="关键词"` | 按网页正文内容搜索 | `body="login"` |
| `header` | `header="关键词"` | 按HTTP响应头搜索 | `header="Server: Apache"` |
| `domain` | `domain="域名"` | 按域名搜索（含子域名） | `domain="example.com"` |
| `host` | `host="域名/IP"` | 按主机名或IP搜索 | `host="192.168.1.1"` |
| `ip` | `ip="IP地址"` | 按IP地址搜索 | `ip="1.1.1.1"` |
| `port` | `port="端口号"` | 按端口号搜索 | `port="80"` |
| `protocol` | `protocol="协议名"` | 按协议搜索 | `protocol="https"` |
| `server` | `server="服务器名"` | 按HTTP Server头搜索 | `server="nginx"` |
| `os` | `os="操作系统"` | 按操作系统搜索 | `os="Windows"` |
| `cert` | `cert="关键词"` | 按SSL证书内容搜索 | `cert="example.com"` |
| `cert.subject` | `cert.subject="CN"` | 按证书主题搜索 | `cert.subject="example.com"` |
| `cert.issuer` | `cert.issuer="CN"` | 按证书颁发者搜索 | `cert.issuer="DigiCert"` |
| `cert.subject.org` | `cert.subject.org="组织"` | 按证书主题组织搜索 | `cert.subject.org="Google"` |
| `cert.issuer.org` | `cert.issuer.org="组织"` | 按证书颁发者组织搜索 | `cert.issuer.org="DigiCert Inc"` |
| `icon_hash` | `icon_hash="哈希值"` | 按网站图标favicon的MMH3哈希搜索 | `icon_hash="-247388890"` |
| `fid` | `fid="指纹ID"` | 按FOFA指纹ID搜索 | `fid="sKU8qJ91lLdP6Dj2IeO7gA=="` |
| `js_name` | `js_name="文件名"` | 按JavaScript文件名搜索 | `js_name="jquery.js"` |
| `js_md5` | `js_md5="MD5值"` | 按JavaScript文件MD5搜索 | `js_md5="5a3e5f..."` |
| `banner` | `banner="关键词"` | 按Banner信息搜索 | `banner="SSH-2.0"` |
| `banner` | `banner="关键词"` | 按服务Banner信息搜索 | `banner="ftp"` |
| `after` | `after="日期"` | 搜索指定日期之后的资产 | `after="2024-01-01"` |
| `before` | `before="日期"` | 搜索指定日期之前的资产 | `before="2024-12-31"` |
| `icp` | `icp="备案号"` | 按ICP备案号搜索 | `icp="京ICP备12345678号"` |
| `country` | `country="国家代码"` | 按国家搜索 | `country="CN"` |
| `region` | `region="省份"` | 按省份搜索 | `region="Beijing"` |
| `city` | `city="城市"` | 按城市搜索 | `city="Beijing"` |
| `org` | `org="组织名"` | 按组织/ISP搜索 | `org="China Telecom"` |
| `asn` | `asn="ASN号"` | 按自治系统号搜索 | `asn="AS4134"` |
| `link` | `link="域名/IP"` | 搜索与指定IP/域名同IP的资产 | `link="1.1.1.1"` |
| `status_code` | `status_code="状态码"` | 按HTTP状态码搜索 | `status_code="200"` |
| `base_protocol` | `base_protocol="协议"` | 按传输层协议搜索 | `base_protocol="udp"` |
| `type` | `type="子协议"` | 按服务子协议搜索 | `type="subdomain"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `&&` 或 `AND` | 逻辑与 | `title="admin" && country="CN"` |
| `\|\|` 或 `OR` | 逻辑或 | `port="80" \|\| port="443"` |
| `!=` | 逻辑非（不等于） | `port!="80"` |
| `*` | 通配符（模糊匹配） | `title="*管理*"` |
| `""` | 精确匹配 | `title="后台管理"` |
| `()` | 分组 | `(port="80" \|\| port="443") && country="CN"` |

**注意事项**：
- FOFA 使用 `=` 而非 `:` 作为键值分隔符。
- `icon_hash` 使用 MMH3 算法计算 favicon 图标的哈希值，可通过 `mmh3-base64` 工具计算。
- `after` 和 `before` 日期格式为 `YYYY-MM-DD`。
- `fid` 是 FOFA 特有的指纹ID，用于精确匹配特定应用指纹。

---

### 2.3 Hunter（奇安信鹰图）

Hunter 是奇安信推出的网络空间测绘平台，支持丰富的查询语法。

**官方文档**: https://hunter.qianxin.com/

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `web.title` | `web.title="关键词"` | 按网页标题搜索 | `web.title="后台管理"` |
| `web.body` | `web.body="关键词"` | 按网页正文内容搜索 | `web.body="login"` |
| `web.header` | `web.header="关键词"` | 按HTTP响应头搜索 | `web.header="Server: nginx"` |
| `web.icon` | `web.icon="哈希值"` | 按网站图标哈希搜索 | `web.icon="c60ea375c39d1ab273c4d1bee717287a"` |
| `web.favicon` | `web.favicon="哈希值"` | 按favicon图标哈希搜索（同web.icon） | `web.favicon="c60ea375..."` |
| `ip.port` | `ip.port="端口号"` | 按端口号搜索 | `ip.port="80"` |
| `ip.city` | `ip.city="城市"` | 按IP所在城市搜索 | `ip.city="北京"` |
| `ip.country` | `ip.country="国家"` | 按IP所在国家搜索 | `ip.country="中国"` |
| `ip.province` | `ip.province="省份"` | 按IP所在省份搜索 | `ip.province="广东"` |
| `ip.isp` | `ip.isp="ISP名"` | 按ISP搜索 | `ip.isp="电信"` |
| `ip.os` | `ip.os="操作系统"` | 按操作系统搜索 | `ip.os="Windows"` |
| `ip.domain` | `ip.domain="域名"` | 按IP关联域名搜索 | `ip.domain="example.com"` |
| `cert.subject` | `cert.subject="关键词"` | 按证书主题搜索 | `cert.subject="example.com"` |
| `cert.issuer` | `cert.issuer="关键词"` | 按证书颁发者搜索 | `cert.issuer="DigiCert"` |
| `cert.subject.cn` | `cert.subject.cn="CN"` | 按证书主题CN搜索 | `cert.subject.cn="example.com"` |
| `cert.issuer.cn` | `cert.issuer.cn="CN"` | 按证书颁发者CN搜索 | `cert.issuer.cn="DigiCert"` |
| `icp.number` | `icp.number="备案号"` | 按ICP备案号搜索 | `icp.number="京ICP备12345678号"` |
| `icp.name` | `icp.name="公司名"` | 按ICP备案公司名搜索 | `icp.name="某某科技"` |
| `is.web` | `is.web="true/false"` | 是否为Web资产 | `is.web="true"` |
| `is_risk` | `is_risk="true/false"` | 是否存在风险 | `is_risk="true"` |
| `protocol` | `protocol="协议名"` | 按协议搜索 | `protocol="http"` |
| `service` | `service="服务名"` | 按服务名搜索 | `service="ftp"` |
| `banner` | `banner="关键词"` | 按Banner信息搜索 | `banner="SSH-2.0"` |
| `app.name` | `app.name="应用名"` | 按应用名搜索 | `app.name="WordPress"` |
| `product` | `product="产品名"` | 按产品名搜索 | `product="Nginx"` |
| `device.name` | `device.name="设备名"` | 按设备名搜索 | `device.name="Router"` |
| `device.type` | `device.type="设备类型"` | 按设备类型搜索 | `device.type="路由器"` |
| `org` | `org="组织名"` | 按组织名搜索 | `org="China Telecom"` |
| `asn` | `asn="ASN号"` | 按ASN号搜索 | `asn="AS4134"` |
| `after` | `after="日期"` | 搜索指定日期之后的资产 | `after="2024-01-01"` |
| `before` | `before="日期"` | 搜索指定日期之前的资产 | `before="2024-12-31"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `&&` 或 `AND` | 逻辑与 | `web.title="admin" && ip.port="80"` |
| `\|\|` 或 `OR` | 逻辑或 | `ip.port="80" \|\| ip.port="443"` |
| `!=` | 逻辑非（不等于） | `ip.port!="80"` |
| `""` | 精确匹配 | `web.title="后台管理"` |
| `()` | 分组 | `(ip.port="80" \|\| ip.port="443") && ip.country="中国"` |

**注意事项**：
- Hunter 使用 `类别.字段` 的命名方式（如 `web.title`、`ip.port`），与 FOFA 的扁平命名方式不同。
- `web.icon` 的哈希值为 MD5 格式，与 FOFA 的 `icon_hash`（MMH3格式）不同。
- Hunter 支持直接输入域名、IP进行搜索，无需指定字段。

---

### 2.4 ZoomEye（知道创宇）

ZoomEye 是由知道创宇推出的网络空间搜索引擎，支持主机设备搜索和Web应用搜索。

**官方文档**: https://www.zoomeye.org/

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `app` | `app:"应用名"` | 按应用/组件名搜索 | `app:"Apache httpd"` |
| `ver` | `ver:"版本号"` | 按应用版本搜索 | `ver:"2.4.49"` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Linux"` |
| `port` | `port:"端口号"` | 按端口号搜索 | `port:"80"` |
| `country` | `country:"国家代码"` | 按国家搜索 | `country:"CN"` |
| `city` | `city:"城市名"` | 按城市搜索 | `city:"Beijing"` |
| `hostname` | `hostname:"主机名"` | 按主机名搜索 | `hostname:"example.com"` |
| `site` | `site:"域名"` | 按站点域名搜索 | `site:"example.com"` |
| `service` | `service:"服务名"` | 按服务名搜索 | `service:"http"` |
| `banner` | `banner:"关键词"` | 按Banner信息搜索 | `banner:"SSH-2.0"` |
| `cidr` | `cidr:"IP/掩码"` | 按CIDR网段搜索 | `cidr:"192.168.1.0/24"` |
| `ssl` | `ssl:"关键词"` | 按SSL证书信息搜索 | `ssl:"example.com"` |
| `ssl.cert.subject.cn` | `ssl.cert.subject.cn:"CN"` | 按SSL证书主题CN搜索 | `ssl.cert.subject.cn:"example.com"` |
| `ssl.cert.issuer.cn` | `ssl.cert.issuer.cn:"CN"` | 按SSL证书颁发者CN搜索 | `ssl.cert.issuer.cn:"Let's Encrypt"` |
| `webapp` | `webapp:"应用名"` | 按Web应用名搜索 | `webapp:"WordPress"` |
| `header` | `header:"关键词"` | 按HTTP响应头搜索 | `header:"Server: nginx"` |
| `title` | `title:"关键词"` | 按网页标题搜索 | `title:"后台管理"` |
| `desc` | `desc:"关键词"` | 按描述信息搜索 | `desc:"router"` |
| `keywords` | `keywords:"关键词"` | 按关键词搜索 | `keywords:"admin"` |
| `ip` | `ip:"IP地址"` | 按IP地址搜索 | `ip:"1.1.1.1"` |
| `asn` | `asn:"ASN号"` | 按ASN号搜索 | `asn:"AS4134"` |
| `org` | `org:"组织名"` | 按组织名搜索 | `org:"China Telecom"` |
| `iconhash` | `iconhash:"哈希值"` | 按favicon图标哈希搜索 | `iconhash:"-247388890"` |
| `device` | `device:"设备类型"` | 按设备类型搜索 | `device:"router"` |
| `subdomain` | `subdomain:"域名"` | 搜索子域名 | `subdomain:"example.com"` |
| `time` | `time:"日期范围"` | 按时间范围搜索 | `time:"2024-01-01+2024-12-31"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格或`+` | 逻辑与（AND） | `port:"80" country:"CN"` |
| `\|` | 逻辑或（OR） | `port:"80" \| port:"443"` |
| `-` | 逻辑非（NOT） | `port:"80" -country:"US"` |
| `""` | 精确匹配 | `app:"Apache httpd"` |

**注意事项**：
- ZoomEye 使用 `:` 作为键值分隔符，值需要用引号包裹。
- ZoomEye 的逻辑或使用单竖线 `|` 而非 `OR`。
- `iconhash` 使用 MMH3 算法，与 FOFA 的 `icon_hash` 计算方式一致。
- ZoomEye 区分"主机搜索"和"Web搜索"两种模式，部分字段仅在特定模式下可用。

---

### 2.5 Quake（360网络空间测绘）

Quake 是360推出的网络空间测绘平台，支持服务、主机、证书等多维度搜索。

**官方文档**: https://quake.360.net/

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `service` | `service:"服务名"` | 按服务名搜索 | `service:"http"` |
| `port` | `port:"端口号"` | 按端口号搜索 | `port:"80"` |
| `ip` | `ip:"IP地址"` | 按IP地址搜索 | `ip:"1.1.1.1"` |
| `domain` | `domain:"域名"` | 按域名搜索 | `domain:"example.com"` |
| `hostname` | `hostname:"主机名"` | 按主机名搜索 | `hostname:"www.example.com"` |
| `title` | `title:"关键词"` | 按网页标题搜索 | `title:"后台管理"` |
| `body` | `body:"关键词"` | 按网页正文内容搜索 | `body:"login"` |
| `header` | `header:"关键词"` | 按HTTP响应头搜索 | `header:"Server: nginx"` |
| `cert` | `cert:"关键词"` | 按SSL证书内容搜索 | `cert:"example.com"` |
| `cert.subject.cn` | `cert.subject.cn:"CN"` | 按证书主题CN搜索 | `cert.subject.cn:"example.com"` |
| `cert.issuer.cn` | `cert.issuer.cn:"CN"` | 按证书颁发者CN搜索 | `cert.issuer.cn:"DigiCert"` |
| `favicon` | `favicon:"哈希值"` | 按favicon图标哈希搜索 | `favicon:"c60ea375..."` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Linux"` |
| `server` | `server:"服务器名"` | 按HTTP Server头搜索 | `server:"nginx"` |
| `app` | `app:"应用名"` | 按应用名搜索 | `app:"WordPress"` |
| `product` | `product:"产品名"` | 按产品名搜索 | `product:"Apache"` |
| `country` | `country:"国家代码"` | 按国家搜索 | `country:"CN"` |
| `city` | `city:"城市名"` | 按城市搜索 | `city:"Beijing"` |
| `region` | `region:"省份"` | 按省份搜索 | `region:"Beijing"` |
| `org` | `org:"组织名"` | 按组织名搜索 | `org:"China Telecom"` |
| `asn` | `asn:"ASN号"` | 按ASN号搜索 | `asn:"AS4134"` |
| `isp` | `isp:"ISP名"` | 按ISP搜索 | `isp:"电信"` |
| `device` | `device:"设备类型"` | 按设备类型搜索 | `device:"router"` |
| `response` | `response:"关键词"` | 按响应内容搜索 | `response:"nginx"` |
| `transport` | `transport:"协议"` | 按传输层协议搜索 | `transport:"udp"` |
| `notice` | `notice:"关键词"` | 按通知信息搜索 | `notice:"漏洞"` |
| `vuln` | `vuln:"CVE编号"` | 按CVE漏洞搜索 | `vuln:"CVE-2021-44228"` |
| `is_vul` | `is_vul:"true/false"` | 是否存在漏洞 | `is_vul:"true"` |
| `is_web` | `is_web:"true/false"` | 是否为Web资产 | `is_web:"true"` |
| `is_domain` | `is_domain:"true/false"` | 是否为域名资产 | `is_domain:"true"` |
| `is_ipv6` | `is_ipv6:"true/false"` | 是否为IPv6资产 | `is_ipv6:"true"` |
| `time` | `time:"日期范围"` | 按时间范围搜索 | `time:"2024-01-01" + "2024-12-31"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格或`AND` | 逻辑与 | `service:"http" AND port:"80"` |
| `OR` | 逻辑或 | `port:"80" OR port:"443"` |
| `-` | 逻辑非（NOT） | `port:"80" -country:"US"` |
| `""` | 精确匹配 | `title:"后台管理"` |
| `*` | 通配符 | `title:"*管理*"` |
| `()` | 分组 | `(port:"80" OR port:"443") AND country:"CN"` |

**注意事项**：
- Quake 使用 `:` 作为键值分隔符，值需要用引号包裹。
- Quake 支持 `AND` 和 `OR` 关键字进行逻辑运算。
- `favicon` 的哈希值为 MD5 格式。
- Quake 提供了 `is_vul`、`is_web`、`is_domain`、`is_ipv6` 等布尔型过滤器。

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
| `vul.cve` | `vul.cve="CVE编号"` | 搜索CVE漏洞 | `vul.cve="CVE-2021-42013"` | - |
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

### 2.8 BinaryEdge

BinaryEdge 是一个网络空间搜索引擎，专注于扫描互联网上的服务和设备。

**官方文档**: https://docs.binaryedge.io/

#### 查询语法

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:地址` | 按IP地址搜索 | `ip:1.1.1.1` |
| `port` | `port:端口` | 按端口号搜索 | `port:80` |
| `product` | `product:"产品名"` | 按产品名搜索 | `product:"Apache"` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Linux"` |
| `device` | `device:"设备类型"` | 按设备类型搜索 | `device:"router"` |
| `country` | `country:"国家代码"` | 按国家搜索 | `country:"CN"` |
| `asn` | `asn:ASN号` | 按ASN号搜索 | `asn:4134` |
| `domain` | `domain:"域名"` | 按域名搜索 | `domain:"example.com"` |
| `title` | `title:"标题"` | 按网页标题搜索 | `title:"admin"` |
| `body` | `body:"内容"` | 按网页内容搜索 | `body:"login"` |
| `header` | `header:"内容"` | 按HTTP头搜索 | `header:"nginx"` |
| `cert` | `cert:"内容"` | 按证书内容搜索 | `cert:"example.com"` |
| `ssh` | `ssh:"内容"` | 按SSH Banner搜索 | `ssh:"OpenSSH"` |
| `vnc` | `vnc:"内容"` | 按VNC信息搜索 | `vnc:"RealVNC"` |
| `rdp` | `rdp:"内容"` | 按RDP信息搜索 | `rdp:"Microsoft"` |
| `smb` | `smb:"内容"` | 按SMB信息搜索 | `smb:"Windows"` |
| `mqtt` | `mqtt:"内容"` | 按MQTT信息搜索 | `mqtt:"Mosquitto"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格 | 逻辑与 | `port:80 country:"CN"` |
| `OR` | 逻辑或 | `port:80 OR port:443` |
| `-` | 逻辑非 | `port:80 -country:"US"` |
| `""` | 精确匹配 | `product:"Apache httpd"` |

---

### 2.9 Onyphe

Onyphe 是一个网络空间搜索引擎，专注于互联网暴露面的发现和威胁情报收集。

**官方文档**: https://search.onyphe.io/docs/onyphe-query-language

#### 查询语法（OQL - Onyphe Query Language）

Onyphe 使用 OQL 语法，以 `category:值 filter:值` 的形式进行搜索。

##### 搜索类别（Category）

| 类别 | 说明 |
|------|------|
| `datascan` | 数据扫描结果 |
| `inetnum` | IP网段信息 |
| `pastries` | Pastebin等粘贴站数据 |
| `resolver` | DNS解析记录 |
| `threatlist` | 威胁列表 |
| `vulnscan` | 漏洞扫描结果 |
| `sniffer` | 嗅探数据 |
| `onionscan` | 暗网洋葱服务扫描 |

##### 常用过滤器

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:地址` | 按IP地址搜索 | `ip:1.1.1.1` |
| `port` | `port:端口` | 按端口号搜索 | `port:80` |
| `domain` | `domain:"域名"` | 按域名搜索 | `domain:"example.com"` |
| `hostname` | `hostname:"主机名"` | 按主机名搜索 | `hostname:"www.example.com"` |
| `country` | `country:"国家代码"` | 按国家搜索 | `country:"CN"` |
| `city` | `city:"城市"` | 按城市搜索 | `city:"Beijing"` |
| `asn` | `asn:ASN号` | 按ASN号搜索 | `asn:4134` |
| `organization` | `organization:"组织名"` | 按组织搜索 | `organization:"China Telecom"` |
| `product` | `product:"产品名"` | 按产品名搜索 | `product:"Apache"` |
| `version` | `version:"版本号"` | 按版本号搜索 | `version:"2.4.49"` |
| `os` | `os:"操作系统"` | 按操作系统搜索 | `os:"Linux"` |
| `device` | `device:"设备类型"` | 按设备类型搜索 | `device:"router"` |
| `protocol` | `protocol:"协议"` | 按协议搜索 | `protocol:"http"` |
| `cve` | `cve:"CVE编号"` | 按CVE漏洞搜索 | `cve:"CVE-2021-44228"` |
| `tag` | `tag:"标签"` | 按标签搜索 | `tag:"compromised"` |
| `subnet` | `subnet:"CIDR"` | 按子网搜索 | `subnet:"192.168.1.0/24"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `+` | 逻辑与 | `category:datascan + port:80` |
| 空格 | 逻辑与 | `category:datascan port:80` |
| `OR` | 逻辑或 | `port:80 OR port:443` |
| `-` | 逻辑非 | `port:80 -country:"US"` |
| `""` | 精确匹配 | `product:"Apache httpd"` |

**注意事项**：
- Onyphe 的查询以 `category` 开头，限定搜索的数据类别。
- OQL 语法使用 `+` 或空格表示逻辑与。
- Onyphe 的 `pastries` 类别可搜索 Pastebin 等粘贴站泄露的数据。

---

### 2.10 GreyNoise

GreyNoise 是一个专门收集和分析互联网"噪音"流量的威胁情报平台，帮助安全分析师区分有针对性的攻击和互联网背景噪音。

**官方文档**: https://docs.greynoise.io/docs/using-the-greynoise-query-language-gnql

#### 查询语法（GNQL - GreyNoise Query Language）

GreyNoise 使用 GNQL 语法，以类似 Lucene 的方式进行搜索。

| 过滤器 | 语法 | 说明 | 示例 |
|--------|------|------|------|
| `ip` | `ip:地址` | 按IP地址搜索 | `ip:1.1.1.1` |
| `classification` | `classification:类型` | 按分类搜索（malicious/benign/unknown） | `classification:malicious` |
| `noise` | `noise:true/false` | 是否为互联网噪音流量 | `noise:true` |
| `riot` | `riot:true/false` | 是否为常见业务服务（RIOT） | `riot:true` |
| `tag` | `tag:"标签名"` | 按活动标签搜索 | `tag:"GPON Router Auth Bypass"` |
| `metadata.organization` | `metadata.organization:"组织"` | 按组织搜索 | `metadata.organization:"Shodan"` |
| `metadata.os` | `metadata.os:"操作系统"` | 按操作系统搜索 | `metadata.os:"Linux"` |
| `metadata.device` | `metadata.device:"设备"` | 按设备类型搜索 | `metadata.device:"Router"` |
| `metadata.category` | `metadata.category:"类别"` | 按类别搜索 | `metadata.category:"iot"` |
| `metadata.intention` | `metadata.intention:"意图"` | 按意图搜索 | `metadata.intention:"malicious"` |
| `last_seen` | `last_seen:"日期"` | 按最后出现时间搜索 | `last_seen:"2024-01-01"` |
| `first_seen` | `first_seen:"日期"` | 按首次出现时间搜索 | `first_seen:"2024-01-01"` |
| `data_port` | `data_port:端口` | 按数据端口搜索 | `data_port:80` |
| `data_protocol` | `data_protocol:"协议"` | 按数据协议搜索 | `data_protocol:"HTTP"` |
| `c2` | `c2:true/false` | 是否为C2通信 | `c2:true` |
| `vpn` | `vpn:true/false` | 是否为VPN服务 | `vpn:true` |
| `vpn_service` | `vpn_service:"服务名"` | 按VPN服务名搜索 | `vpn_service:"NordVPN"` |
| `spoofable` | `spoofable:true/false` | IP是否可被欺骗 | `spoofable:true` |
| `id` | `id:"标识符"` | 按GreyNoise标识符搜索 | `id:"GNXX-XXXX"` |

#### 逻辑运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| 空格 | 逻辑与 | `classification:malicious data_port:80` |
| `OR` | 逻辑或 | `data_port:80 OR data_port:443` |
| `-` | 逻辑非 | `classification:malicious -metadata.category:"iot"` |
| `""` | 精确匹配 | `tag:"GPON Router Auth Bypass"` |
| `*` | 通配符 | `metadata.organization:"Sho*"` |

**注意事项**：
- GreyNoise 的 `classification` 有三个值：`malicious`（恶意）、`benign`（良性）、`unknown`（未知）。
- `riot` 字段标识该IP是否属于常见业务服务（如Google、Cloudflare等），帮助过滤误报。
- `tag` 字段是 GreyNoise 的核心功能，每个标签代表一种特定的扫描活动或攻击行为。
- `c2` 字段标识该IP是否为命令与控制（C2）服务器。

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
| `positives` | `positives:检测数` | 按杀软检测数搜索 | `positives:10+` |
| `name` | `name:"文件名"` | 按文件名搜索 | `name:"malware.exe"` |
| `tag` | `tag:标签` | 按标签搜索 | `tag:trojan` |
| `fsr` | `fsr:分数` | 按First Seen Ratio搜索 | `fsr:0.8+` |
| `itw` | `itw:URL` | 按In-The-Wild URL搜索 | `itw:"example.com"` |
| `have` | `have:属性` | 搜索具有指定属性的文件 | `have:imports` |
| `behaviour` | `behaviour:"行为"` | 按沙箱行为搜索 | `behaviour:"creates_mutant"` |
| `signature` | `signature:"签名"` | 按杀软签名搜索 | `signature:"Trojan.Generic"` |
| `engines` | `engines:引擎名:` | 按特定引擎检测结果搜索 | `engines:microsoft:trojan` |
| `pe` | `pe:PE属性` | 按PE文件属性搜索 | `pe:has_resources` |
| `pdf` | `pdf:PDF属性` | 按PDF文件属性搜索 | `pdf:has_js` |
| `android` | `android:Android属性` | 按Android文件属性搜索 | `android:has_dex` |
| `submission` | `submission:日期` | 按提交日期搜索 | `submission:2024-01-01+` |
| `creation` | `creation:日期` | 按文件创建日期搜索 | `creation:2024-01-01+` |
| `imphash` | `imphash:"哈希值"` | 按导入表哈希搜索 | `imphash:"abc123..."` |
| `richpehash` | `richpehash:"哈希值"` | 按Rich PE哈希搜索 | `richpehash:"def456..."` |
| `dhash` | `dhash:"哈希值"` | 按图标哈希搜索 | `dhash:"ghi789..."` |
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
- `fsr`（First Seen Ratio）表示首次发现比率，值越接近1表示越新出现的文件。
- `have` 修饰符用于搜索具有特定属性的文件，如 `have:imports`、`have:exports`、`have:resources` 等。

---

## 四、DNS搜索引擎

### 4.1 DnsDB

DnsDB 是一个DNS历史记录搜索引擎，由微步在线运营，支持查询域名与IP之间的历史解析记录。

**官方文档**: https://www.dnsdb.io/

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
| **域名搜索** | `hostname:"域名"` | `domain="域名"` | `ip.domain="域名"` | `site:"域名"` | `domain:"域名"` | - | `domain="域名"` |
| **标题搜索** | `http.title:"标题"` | `title="标题"` | `web.title="标题"` | `title:"标题"` | `title:"标题"` | `services.http.response.html_title:标题` | `web.title="标题"` |
| **正文搜索** | `http.html:"内容"` | `body="内容"` | `web.body="内容"` | - | `body:"内容"` | `services.http.response.body:内容` | `web.body="内容"` |
| **响应头搜索** | `http.server:"值"` | `header="值"` | `web.header="值"` | `header:"值"` | `header:"值"` | `services.http.response.headers:值` | `web.header="值"` |
| **国家搜索** | `country:"代码"` | `country="代码"` | `ip.country="国家"` | `country:"代码"` | `country:"代码"` | `location.country_code:代码` | `ip.country="国家"` |
| **城市搜索** | `city:"城市"` | `city="城市"` | `ip.city="城市"` | `city:"城市"` | `city:"城市"` | `location.city:城市` | `ip.city="城市"` |
| **操作系统** | `os:"系统"` | `os="系统"` | `ip.os="系统"` | `os:"系统"` | `os:"系统"` | `operating_system:系统` | `ip.os="系统"` |
| **组织/ISP** | `org:"组织"` | `org="组织"` | `org="组织"` | `org:"组织"` | `org:"组织"` | `autonomous_system.name:名称` | `asn.org="组织"` |
| **ASN** | `asn:ASN` | `asn="ASN"` | `asn="ASN"` | `asn:"ASN"` | `asn:"ASN"` | `autonomous_system.asn:ASN` | `asn.number="ASN"` |
| **证书主题CN** | `ssl.cert.subject.cn:"CN"` | `cert.subject="CN"` | `cert.subject.cn="CN"` | `ssl.cert.subject.cn:"CN"` | `cert.subject.cn:"CN"` | `parsed.subject.common_name:CN` | `cert.subject.cn="CN"` |
| **证书颁发者CN** | `ssl.cert.issuer.cn:"CN"` | `cert.issuer="CN"` | `cert.issuer.cn="CN"` | `ssl.cert.issuer.cn:"CN"` | `cert.issuer.cn:"CN"` | `parsed.issuer.common_name:CN` | `cert.issuer.cn="CN"` |
| **Favicon哈希** | `http.favicon.hash:值` | `icon_hash="值"` | `web.icon="值"` | `iconhash:"值"` | `favicon:"值"` | - | `web.icon="值"` |
| **漏洞搜索** | `vuln:CVE` | - | - | - | `vuln:"CVE"` | - | `vul.cve="CVE"` |
| **Banner搜索** | 直接输入 | `banner="值"` | `banner="值"` | `banner:"值"` | `service:"值"` | `services.banner:值` | `protocol.banner="值"` |
| **逻辑与** | 空格 | `&&` 或 `AND` | `&&` 或 `AND` | 空格或`+` | 空格或`AND` | `AND` | `&&` |
| **逻辑或** | `OR` | `\|\|` 或 `OR` | `\|\|` 或 `OR` | `\|` | `OR` | `OR` | `\|\|` |
| **逻辑非** | `-` | `!=` | `!=` | `-` | `-` | `NOT` | `!=` |
| **键值分隔符** | `:` | `=` | `=` | `:` | `:` | `:` | `=` |

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
| `*` | 匹配任意字符 | FOFA、Quake、Censys、GreyNoise |
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
