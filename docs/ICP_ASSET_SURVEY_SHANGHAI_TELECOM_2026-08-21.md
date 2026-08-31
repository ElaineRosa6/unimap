# 中国电信上海分公司 ICP 备案与资产测绘结果（2026-08-21）

> 执行环境：云端 UniMap 本机 API。本文仅记录查询事实与结果，不包含管理令牌、引擎密钥、Cookie 或通知凭据。

## 1. 执行摘要

| 项目 | 结论 |
|---|---|
| 查询主体 | 中国电信股份有限公司上海分公司 |
| ICP 主备案号 | 沪ICP备05000157号 |
| ICP 记录 | 22 条：网站 5、小程序 17、APP 0、快应用 0 |
| 资产查询引擎 | FOFA、Hunter、DayDayMap |
| 资产查询状态 | `success`，无引擎错误，结果已持久化 |
| 合并资产 | 31 条服务记录 |
| 引擎原始命中 | FOFA 23、DayDayMap 21、Hunter 8 |
| 主要归属 | 上海电信段集中在 `101.95.49/50/52`；`shdict.com` 相关集中在 `114.80.47.27-31` |

## 2. 查询条件

### 2.1 ICP 查询

```text
search=中国电信股份有限公司上海分公司
type=web,app,mapp,kapp
page=1
page_size=100
```

### 2.2 资产测绘查询

先取 ICP 网站备案域名，再构造跨域名组合：

```text
(domain="shtel.com.cn" || domain="shanghaitelecom.com.cn" || domain="infospace.sh.cn" || domain="shdxbwg.com" || domain="shdict.com")
```

| 参数 | 值 |
|---|---|
| 引擎 | `fofa,hunter,daydaymap` |
| `page_size` | 100 |
| 查询方式 | 云端服务本机 `POST /api/v1/query` |
| 认证 | 管理令牌仅从容器运行环境读取，未写入命令历史或本文 |

云端健康检查结果：

```text
/health/ready = ok
engines = ok
icp_db = ok
history_db = ok
screenshot = degraded（云端截图按当前策略保持关闭，不影响查询）
```

## 3. ICP 备案明细

所有记录主体均为“中国电信股份有限公司上海分公司”，主体性质为企业，主备案号均为 `沪ICP备05000157号`。

### 3.1 网站备案

| 域名 | 服务备案号 | 更新时间 |
|---|---|---|
| `shanghaitelecom.com.cn` | 沪ICP备05000157号-1 | 2019-07-30 10:05:14 |
| `infospace.sh.cn` | 沪ICP备05000157号-2 | 2019-07-30 10:05:14 |
| `shtel.com.cn` | 沪ICP备05000157号-3 | 2019-07-30 10:05:14 |
| `shdxbwg.com` | 沪ICP备05000157号-7 | 2019-07-30 10:05:14 |
| `shdict.com` | 沪ICP备05000157号-9 | 2022-04-19 14:21:39 |

### 3.2 小程序备案

| 名称 | 服务备案号 | 更新时间 |
|---|---|---|
| 上海电信 | 沪ICP备05000157号-10X | 2024-08-29 14:55:51 |
| 产数智能运营 | 沪ICP备05000157号-11X | 2024-01-12 10:54:50 |
| 上海电信线上营业厅 | 沪ICP备05000157号-13X | 2024-01-22 15:23:15 |
| 数集电子名片 | 沪ICP备05000157号-14X | 2024-01-24 16:21:12 |
| 翼企无忧 | 沪ICP备05000157号-15X | 2024-01-29 18:15:52 |
| 上海电信云宽带 | 沪ICP备05000157号-16X | 2026-02-25 13:32:32 |
| 投标服务预约 | 沪ICP备05000157号-17X | 2024-02-02 16:32:53 |
| 电信新销售上海店 | 沪ICP备05000157号-18X | 2026-05-09 10:43:39 |
| 上海电信10009政企服务 | 沪ICP备05000157号-20X | 2026-06-11 09:15:26 |
| 上海电信备案核验平台 | 沪ICP备05000157号-22X | 2026-06-05 16:36:36 |
| 智家硬盘 | 沪ICP备05000157号-23X | 2024-04-15 13:55:53 |
| 中国电信上海客服 | 沪ICP备05000157号-24X | 2024-09-26 14:08:55 |
| 上海电信数字门店 | 沪ICP备05000157号-25X | 2024-10-30 10:07:14 |
| 电信派卡真实身份信息登记 | 沪ICP备05000157号-26X | 2024-11-19 09:04:30 |
| 上海电信场景类实名制 | 沪ICP备05000157号-27X | 2024-11-27 15:42:59 |
| 上海电信智慧家庭 | 沪ICP备05000157号-28X | 2026-08-21 10:18:37 |
| 绿色安全上网 | 沪ICP备05000157号-29X | 2025-12-12 16:26:55 |

### 3.3 APP 与快应用

| 类型 | 记录数 |
|---|---:|
| APP 备案 | 0 |
| 快应用备案 | 0 |

## 4. 资产测绘明细

下表为 UniMap 合并去重后的 31 条服务记录。`来源` 是命中该资产的引擎，不代表记录数量简单相加。

| IP | 端口 | 主机 | 标题 / 服务特征 | 状态码 | 地区 | ASN | 来源 |
|---|---:|---|---|---:|---|---|---|
| 101.95.49.104 | 80 | `www.shtel.com.cn` | 中国电信上海公司官方网站 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.49.180 | 80 | `shyjr.shtel.com.cn` | 408 | 0 | 上海市 | AS4812 | DayDayMap |
| 101.95.50.72 | 80 | `aeop.shtel.com.cn` | 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.50.72 | 443 | `aeop.shtel.com.cn` | 403 / 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.50.73 | 80 | `shtel.com.cn` | 防护响应 | 412 | 上海市 | AS4812 | FOFA |
| 101.95.50.73 | 443 | `shtel.com.cn` | 防护响应 | 412 | 上海市 | AS4812 | FOFA |
| 101.95.50.74 | 80 | `i.shtel.com.cn` | 防护响应 | 0 | 上海市 | AS4812 | DayDayMap |
| 101.95.50.74 | 443 | `i.shtel.com.cn` | 中国电信＊智慧服务；Nginx | 0 | 上海市 | AS4812 | DayDayMap |
| 101.95.52.140 | 80 | `www.infospace.sh.cn` | 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.52.140 | 443 | `www.infospace.sh.cn` | 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.52.144 | 80 | `www.shdxbwg.com` | 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.95.52.144 | 443 | `www.shdxbwg.com` | 防护响应 | 412 | 上海市 | AS4812 | FOFA, DayDayMap |
| 101.91.157.67 | 88 | `shdict.com` | 防护响应 | 412 | 上海市 | AS4811 | FOFA |
| 114.80.47.27 | 8000 | `shdict.com` | OpenResty | 200 | 上海市 | AS4812 | FOFA, Hunter |
| 114.80.47.27 | 8001 | `live1-ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | FOFA, Hunter |
| 114.80.47.27 | 8002 | `shdict.com` | FLV | 404 | 上海市 | AS4812 | FOFA |
| 114.80.47.27 | 8180 | `shdict.com` | HLS | 404 | 上海市 | AS4812 | FOFA |
| 114.80.47.28 | 8000 | `live2-ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | FOFA, Hunter |
| 114.80.47.28 | 8001 | `live2-ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | FOFA, Hunter |
| 114.80.47.28 | 8002 | `shdict.com` | FLV | 404 | 上海市 | AS4812 | FOFA |
| 114.80.47.28 | 8180 | `shdict.com` | HLS | 404 | 上海市 | AS4812 | FOFA |
| 114.80.47.29 | 9192 | `ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | Hunter |
| 114.80.47.29 | 9193 | `ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | Hunter |
| 114.80.47.29 | 20002 | `ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | Hunter |
| 114.80.47.29 | 20003 | `ivfp.shdict.com` | OpenResty | 200 | 上海市 | AS4812 | Hunter |
| 114.80.47.31 | 8880 | `epark.shdict.com` | openEuler Nginx 默认页 | 200 | 上海市 | AS4812 | FOFA, Hunter |
| 42.81.118.41 | 80 | `aeop.shtel.com.cn.e.ngaacdn.cn` | CDN InvalidHost / 403 | 403 | 天津市 | AS58542 | DayDayMap |
| 42.81.118.41 | 443 | `aeop.shtel.com.cn.e.ngaacdn.cn` | CDN InvalidHost / 403 | 403 | 天津市 | AS58542 | DayDayMap |
| 42.81.118.41 | 8088 | `aeop.shtel.com.cn.e.ngaacdn.cn` | CDN InvalidHost / 403 | 403 | 天津市 | AS58542 | DayDayMap |
| 113.113.67.41 | 80 | `aeop.shtel.com.cn.e.ngaacdn.cn` | CDN InvalidHost / 403 | 403 | 广东省东莞市 | AS4134 | DayDayMap |
| 113.113.67.41 | 443 | `aeop.shtel.com.cn.e.ngaacdn.cn` | CDN InvalidHost / 403 | 403 | 广东省东莞市 | AS4134 | DayDayMap |

## 5. IP 与域名归组

| 分组 | 资产 | 说明 |
|---|---|---|
| 主站与官网段 | `101.95.49.104/180` | `shtel.com.cn` 主站及子域样本 |
| 应用与防护段 | `101.95.50.72-74` | `shtel.com.cn`、`aeop`、`i` 子域 |
| 信息服务与博物馆站段 | `101.95.52.140/144` | `infospace.sh.cn`、`shdxbwg.com` |
| `shdict.com` 流媒体段 | `114.80.47.27-31` | 多个非标准端口，OpenResty、HLS、FLV |
| 其他上海样本 | `101.91.157.67:88` | `shdict.com`，AS4811 |
| CDN / 外地边缘样本 | `42.81.118.41`、`113.113.67.41` | `aeop.shtel.com.cn.e.ngaacdn.cn` |

## 6. 观察与风险提示

1. **CDN 归属不能等同企业自有服务器。**
   `aeop.shtel.com.cn.e.ngaacdn.cn` 是 CDN CNAME 形态；天津、东莞节点属于解析或边缘样本，不应直接认定为目标自有服务器。

2. **证书主体与 ICP 主体不一致。**
   `i.shtel.com.cn` 的证书主体显示为“上海芘霆电子商务有限公司”，而域名 ICP 主体为“中国电信股份有限公司上海分公司”。建议核实业务归属、证书签发配置和当前是否仍在使用。

3. **`shdict.com` 暴露面较分散。**
   `114.80.47.27-31` 开放 8000、8001、8002、8180、9192、9193、20002、20003、8880 等端口，且存在 OpenResty 默认页和 openEuler Nginx 默认页。建议确认这些服务是否应对公网开放，并复核访问控制、认证与默认页清理。

4. **部分响应被防护层遮蔽。**
   `101.95.50.72/73`、`101.95.52.140/144`、`www.shtel.com.cn` 等返回 412 或 `******`，属于测绘侧防护响应；不能据此判定后端异常。

5. **数据时效不同。**
   DayDayMap 部分样本的 `last_seen` 为 2022—2025 年；FOFA/Hunter 样本多更新至 2026-05 至 2026-08。结论应以当前可达性复测为准。

## 7. 后续建议

| 优先级 | 动作 |
|---|---|
| P0 | 对 31 条记录做当前可达性复测，剔除失效和 CDN 边缘样本。 |
| P0 | 核实 `i.shtel.com.cn` 证书主体与业务归属。 |
| P1 | 复核 `shdict.com` 相关非标准端口、默认页、认证与访问控制。 |
| P1 | 对 `aeop.shtel.com.cn.e.ngaacdn.cn` 做真实 CNAME 和源站归属核实。 |
| P2 | 将 5 个备案域名加入周期测绘与 ICP 变更监控，避免只依赖单次快照。 |

## 8. 结果文件与追溯

- 云端临时结果：`/tmp/shtel_icp.json`、`/tmp/unimap_shtel_query.json`
- UniMap 查询持久化：`status=persisted`
- 查询时间：2026-08-21，云端 UTC 时间 08:08 后
- 本文档不包含任何真实管理令牌、引擎 Key、Cookie、Bridge token 或通知凭据。
