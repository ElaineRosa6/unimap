# UniMap 🌍

> **多引擎网络空间资产查询与网页监控工具**

UniMap 是一个功能强大的统一查询与资产监控平台，专为网络安全研究、资产测绘、红蓝对抗和企业安全监控设计。系统目前已深度集成并支持 **FOFA、Hunter、ZoomEye、Quake、Shodan** 五大主流空间测绘搜索引擎，并提供 **Web端、CLI命令行端、GUI桌面端** 三端独立运行能力。

## 🚀 核心特性

- 🔍 **多引擎统一查询 (UQL)**
  采用独立统一的查询语法设计，底层自动翻译适配各引擎查询语言。支持跨引擎结果归并去重，多规格（CSV / Excel / JSON）快速数据导出。
- 📸 **截图路由容灾高可用**
  采用原生 `CDP` 与 `Chrome Extension` 双模式探针。实时探测引擎健康状态进行自动容灾和降级备份切换，无惧防反爬机制。
- 🛡️ **智能网页篡改监控**
  原生支持 5 种监控模式机制（strict / relaxed / malicious / performance / full），对被监控页面进行基于渲染对比与恶意代码检测的高效防护。
- ⏱️ **高可用定时任务调度 (Scheduler)**
  内置 20+ 种 Runner，提供高、中、低多级优先级队列分发机制和异常记录持久化系统，保障任务调度的高稳定性。
- 🌐 **分布式任务节点集群支持**
  内置轻量级的分布式管理框架：支持独立节点动态注册、心跳保活、任务自动领取及脱机故障转移（Failover）。
- 📢 **全链路状态与告警**
  提供 Webhook 及 Log 多通知渠道。配备完善的监控阀值设定、去重机制、静默时间管理以及触发频率精细控制。

## 🛠️ 技术底座 / Tech Stack

* **运行时**: `Go 1.26` 
* **应用程序端**:
  * `Web`: `net/http` + gorillas + go-resty 
  * `CLI`: Cobra
  * `GUI`: Fyne v2
* **引擎及监控**: pterm (chromedp) / goquery / html sanitizer
* **存储及缓存**: SQLite + LRU 内存缓存 + Redis (go-redis/v9)
* **监控生态**: Prometheus 客户端
* **配置**: 基于 YAML 及 Viper 的热生效体系

## 🧩 快速启动

1. **环境准备及配置**
   复制示例配置并补充相关的 API Key 与 Webhook 配置：
   ```bash
   cp configs/config.yaml.example configs/config.yaml
   ```

2. **选择适合的端启动**
   
   **(A) Web UI 前后置模式 (默认端口 8448)**
   ```bash
   go run ./cmd/unimap-web
   ```
   > 访问浏览器进入 `http://localhost:8448`
   
   **(B) CLI 命令行并发模式**
   ```bash
   # 联合查询 FOFA 与 Hunter 示例
   go run ./cmd/unimap-cli -q 'country="CN" && port="80"' -e fofa,hunter -l 100
   ```
   
   **(C) GUI 图形交互桌面端模式**
   ```bash
   go run -tags gui ./cmd/unimap-gui
   ```

## 📚 详细文档体系

在项目的 `docs/` 目录下存放了完善的开发部署文档记录，可供查阅指引：
- **`QUICKSTART.md`** - 新手快速开始指南与入门指引
- **`ARCHITECTURE.md`** - 项目架构与结构设计拆解
- **`UQL_GUIDE.md`** - 联合查询语法 UQL 详解词典
- **`RUNBOOK.md`** - 故障定位查修与运维保障手册
- **`PRODUCTION_READINESS_PLAN.md`** - 上线前的生产高可用就绪清单
- **`PLUGIN_DEVELOPMENT_GUIDE.md`** - 开发者插件生态拓展指南与 API 文档

## 🛡️ License

请遵循对应开源协议以及各类引擎厂商接口的合规性保护条款。使用时应仅限于授权及合法的技术用途范围，严禁用于任何恶意网络攻击中。
