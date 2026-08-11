# Hunter（奇安信鹰图）— 查询语法及 API 使用明细

> **复核日期**：2026-08-10  
> **官方平台**：https://hunter.qianxin.com/  
> **运营方/产品归属**：奇安信  
> **重要核验说明**：Hunter 的详细帮助/API 页面在当前公开访问环境中主要依赖登录态和前端动态加载，无法像 FOFA、Shodan、Censys、ZoomEye 一样完整抓取在线 API Schema。因此本文把内容分成：
>
> 1. **官方平台可确认的能力与查询字段体系**；
> 2. **公开客户端长期使用的兼容调用格式**，明确标为“需账户内官方帮助中心复核”。
>
> 在生产系统中，应登录 Hunter 后用帮助中心/API 页面再次核对 Endpoint、参数和额度。

---

## 1. 查询语法总体形式

Hunter 常见语法采用字段查询：

```text
field="value"
```

并通过逻辑连接符组合。

常见组合形式：

```text
condition1 && condition2
condition1 || condition2
```

建议字符串始终使用双引号。

---

## 2. 常用字段体系

> 下列字段属于 Hunter 搜索中常见且稳定的资产维度；少见/新字段应以当前搜索页的语法提示为准。

### 2.1 IP / 地理 / 端口

```text
ip="203.0.113.10"
ip.port="443"
ip.country="日本"
ip.province="东京"
ip.city="东京"
```

### 2.2 域名

```text
domain="example.com"
domain.suffix="example.com"
```

`domain.suffix` 常用于主域/后缀关联的子域资产检索。

### 2.3 Web

```text
web.title="Example"
web.body="login"
web.header="nginx"
web.status_code="200"
```

### 2.4 协议/Banner

```text
protocol="https"
protocol.banner="OpenSSH"
```

### 2.5 应用/指纹

Hunter 支持应用、组件和设备类指纹检索。具体字段名与可选指纹应从当前搜索页的语法/联想提示复制，避免用 FOFA/ZoomEye 字段名进行猜测。

### 2.6 ICP / 企业关联

Hunter 支持 ICP、企业/组织等资产关系检索。实际字段和高级企业查询能力与账户权益可能有关。

---

## 3. 组合示例

```text
# 主域相关资产
domain.suffix="example.com"

# 日本 443 Web 资产
ip.country="日本" && ip.port="443"

# 标题中包含关键词
web.title="Example"

# Header 中包含 Nginx
web.header="nginx"

# 主域 + 200
domain.suffix="example.com" && web.status_code="200"
```

---

## 4. API Key

Hunter 开放 API 使用账号 API Key。

应从登录后的个人中心/API 页面取得 Key。

推荐：

```bash
export HUNTER_API_KEY='YOUR_KEY'
```

不要把 Key 写入浏览器前端、公开脚本或 Git 仓库。

---

## 5. 搜索 API（需账户内官方页面复核）

公开客户端和长期兼容调用中常见 Endpoint：

```text
GET https://hunter.qianxin.com/openApi/search
```

常见参数：

| 参数 | 说明 | 核验级别 |
|---|---|---|
| `api-key` | API Key | 需账户内复核 |
| `search` | 查询语句编码值 | 需账户内复核 |
| `page` | 页码 | 需账户内复核 |
| `page_size` | 每页条数 | 需账户内复核 |
| `is_web` | Web/非 Web/全部筛选 | 需账户内复核 |
| `start_time` | 开始时间 | 需账户内复核 |
| `end_time` | 结束时间 | 需账户内复核 |

常见 `is_web` 约定：

```text
1 = Web
2 = 非 Web
3 = 全部
```

**这一节不要当作永久 API 契约。登录后的 Hunter API 帮助页优先级最高。**

---

## 6. search 编码

Hunter 公开集成通常会对原始搜索语句进行 Base64 URL-safe 编码。

Python：

```python
import base64

query = 'domain.suffix="example.com"'
encoded = base64.urlsafe_b64encode(query.encode("utf-8")).decode("ascii")
print(encoded)
```

某些实现会去掉末尾 `=` padding；是否需要去掉应以当前官方 API 示例为准。

---

## 7. 请求示例（兼容格式，需账户内复核）

```python
import os
import base64
import requests

query = 'domain.suffix="example.com"'
search = base64.urlsafe_b64encode(query.encode()).decode()

r = requests.get(
    "https://hunter.qianxin.com/openApi/search",
    params={
        "api-key": os.environ["HUNTER_API_KEY"],
        "search": search,
        "page": 1,
        "page_size": 10,
        "is_web": 3,
    },
    timeout=30,
)
r.raise_for_status()
print(r.json())
```

CURL：

```bash
curl -G 'https://hunter.qianxin.com/openApi/search' \
  --data-urlencode "api-key=$HUNTER_API_KEY" \
  --data-urlencode "search=$HUNTER_SEARCH" \
  --data-urlencode 'page=1' \
  --data-urlencode 'page_size=10' \
  --data-urlencode 'is_web=3'
```

---

## 8. 返回结构（兼容实现观察，需账户内复核）

常见客户端会读取：

```text
code
message
data.total
data.time
data.arr[]
```

`data.arr[]` 中常见资产字段会包含 IP、端口、域名、Web 标题、协议、时间、地理信息、ICP/企业信息、指纹信息等，但具体结构可能随接口版本和 `is_web` 类型不同。

开发时建议先保存一份真实响应样本，再生成强类型模型。

---

## 9. 权限、积分与频率

Hunter 的 API 条数、时间范围、积分消耗、每日/每月额度以及高级字段权限与会员等级有关，而且属于动态商业策略。

因此客户端应：

1. 读取服务端错误码/错误文本；
2. 对 401/403/业务权限错误单独处理；
3. 不把固定“每天多少次”“每页最多多少条”等旧教程数字写死；
4. 在账号续费/升级后重新读取官方说明。

---

## 10. 为什么本文件没有伪造“完整字段表”

Hunter 当前的官方帮助中心需要登录/动态渲染，公开检索无法可靠取得整个最新语法表。如果直接根据博客或其他测绘平台补齐字段，会造成两类风险：

- 把旧版 Hunter 字段写成当前字段；
- 把 FOFA/Quake/ZoomEye 的字段误认为 Hunter 兼容字段。

所以本文件只保留高置信常用字段，并明确要求在当前账户 UI 中核对扩展字段。

---

## 11. 建议的自动化适配策略

如果你要把 Hunter 集成进统一资产搜索 Agent：

```text
平台适配器
├── query_builder
├── encoder
├── request
├── response_normalizer
└── capability/plan errors
```

尤其不要把 Hunter 的语法和 FOFA 语法共用一套字符串模板；应在“统一抽象字段 → 平台字段”层做映射。

---

## 12. 官方入口

- Hunter：https://hunter.qianxin.com/
- 奇安信产品/支持入口：https://www.qianxin.com/

> **状态标记**：本文件是 7 份文档中唯一一份因官方帮助中心登录/动态加载限制而无法对全部 API Schema 做公开逐字段复核的文档。Endpoint、编码和参数在投入生产前必须登录 Hunter 再核对。  
> 本文档用于合法资产管理、安全研究和已授权测试。
