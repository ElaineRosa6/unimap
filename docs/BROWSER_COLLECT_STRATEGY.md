# Browser Collect Strategy

> 定义浏览器扩展采集结构化结果时的 DOM 选择器、字段映射与回传协议。

## 协议格式

扩展回传 `structured_collected_data` 的 JSON 结构：

```json
{
  "title": "FOFA - 搜索结果",
  "total": 1250,
  "has_more": true,
  "items": [
    {
      "ip": "1.2.3.4",
      "port": 443,
      "protocol": "https",
      "host": "example.com",
      "title": "Example Site",
      "country_code": "CN",
      "server": "nginx/1.21"
    }
  ]
}
```

后端通过 `parseStructuredCollectedData()` 将 `items[]` 映射到 `model.UnifiedAsset`，未知字段存入 `Extra`。

## 防腐层

- 旧扩展只返回 `CollectedData string` → 回退为 `CollectResult.Title`，行为不变
- 新扩展返回 `StructuredCollectedData` → 优先解析为 `Assets[]`
- 两条路径并存，外部调用零感知

## 引擎 DOM 选择器

### FOFA (https://fofa.info/result?qbase64=...)

| 字段 | DOM 选择器 | 备注 |
|------|-----------|------|
| items | `.list_content > tbody > tr` | 每行一条资产 |
| ip | `td:nth-child(1) a` 或 `td:nth-child(1)` | IP 地址 |
| port | `td:nth-child(2)` | 端口号 |
| protocol | `td:nth-child(3)` | 协议 |
| host | `td:nth-child(4) a` | 域名 |
| title | `td:nth-child(5)` | 网页标题 |
| country | `td:nth-child(6)` | 国家代码 |
| banner | `td:nth-child(7)` | 协议头/横幅 |
| total | `.total-count` 或解析页面上显示的总数 | |
| has_more | 存在"下一页"按钮 | `.next-page` 存在即为 true |

### Hunter (https://hunter.qianxin.com/list?searchValue=...)

| 字段 | DOM 选择器 | 备注 |
|------|-----------|------|
| items | `.result-list > .result-item` | 每条资产卡片 |
| ip | `.ip-address` | IP 地址 |
| port | `.port` | 端口号 |
| protocol | `.protocol` | 协议 |
| host | `.domain` | 域名 |
| title | `.web-title` | 网页标题 |
| banner | `.header-info` | HTTP 头信息 |
| company | `.company` | 组织/公司 |
| total | `.total-count` | 总记录数 |
| has_more | 分页组件存在"下一页" | |

### ZoomEye (https://www.zoomeye.org/searchResult?q=...)

| 字段 | DOM 选择器 | 备注 |
|------|-----------|------|
| items | `div[class*="search-result-item"]` 或 `.result-list > .item` | |
| ip | `.ip` 或 `[data-ip]` | |
| port | `.port` 或 `[data-port]` | |
| protocol | `.service` 或 `[data-service]` | |
| host | `.domain` | |
| title | `.title` | |
| country | `.location` | |
| banner | `.banner` | |
| total | `.total` | |
| has_more | 分页导航存在 | |

### Quake (https://quake.360.cn/quake/#/searchResult?searchVal=...)

| 字段 | DOM 选择器 | 备注 |
|------|-----------|------|
| items | `.result-list > .result-row` | |
| ip | `.ip` | |
| port | `.port` | |
| protocol | `.transport` 或 `.protocol` | |
| host | `.hostname` | |
| title | `.title` | |
| server | `.server` | Web 服务器 |
| city | `.city` | 城市 |
| isp | `.isp` | ISP |
| total | `.total-count` | |
| has_more | 存在下一页按钮 | |

## 字段映射（扩展 → UnifiedAsset）

| 扩展字段 | UnifiedAsset 字段 | 类型 |
|----------|------------------|------|
| ip | IP | string |
| port | Port | int |
| protocol | Protocol | string |
| host | Host | string |
| url | URL | string |
| title | Title | string |
| body_snippet | BodySnippet | string |
| server | Server | string |
| status_code | StatusCode | int |
| country_code | CountryCode | string |
| region | Region | string |
| city | City | string |
| asn | ASN | string |
| org | Org | string |
| isp | ISP | string |
| (其他) | Extra[key] | interface{} |

## 扩展端 action 分流

扩展 `background.js` 的 `handleTask()` 需处理三种 action：

| action | 行为 | 回传字段 |
|--------|------|---------|
| `open` | 打开页面，不截图、不采集 | 无 |
| `collect` | 打开页面 → 等待加载 → 提取 DOM → 回传 | `structured_collected_data` |
| `screenshot` / 无 action | 打开页面 → 等待加载 → 截图 | `image_data` + `image_path` |
