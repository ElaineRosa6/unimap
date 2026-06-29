# unimap 项目 2026 年 6 月提交梳理

- 仓库：unimap
- 分支：## develop...origin/develop
- 统计范围：2026-06-02 至 2026-06-29（按 git log --since=2026-06-01 --until=2026-06-29）
- 提交数量：133
- 作者：ElaineRosa6
- 变更文件数：723
- 行数统计：+60552 / -35442（包含文档归档、运行时产物清理等机械性变更，仅作规模参考）

## 一、总体概览

6 月提交主要围绕安全加固、调度器完善、搜索引擎适配、浏览器采集链路、强类型重构、Web 测试覆盖率提升和工程卫生清理展开。整体上，项目从功能补齐和问题修复逐步转向接口边界强类型化、模块拆分、测试补齐与运行时产物治理。

## 二、阶段梳理

| 时间 | 阶段主题 | 主要内容 |
| --- | --- | --- |
| 6/2-6/3 | 多用户认证、调度器与通知基础闭环 | 新增用户注册/登录/CRUD/密码管理，修复认证安全问题；优化定时任务前端、Runner 验证、飞书通知和 bridge token 认证。 |
| 6/7-6/9 | 采集链路、搜索引擎适配与大文件拆分 | 修复扩展采集 0 assets、重配对和选择器问题；新增/修正 Censys、DayDayMap、GreyNoise 等适配；完成 scheduler、tamper、screenshot、config、adapter 等大文件拆分。 |
| 6/10-6/12 | 网络采集、Web 安全稳定性与强类型边界 | 接入 L1 Network 采集；修复数据竞态、XSS、panic recovery、CSP、rate-limit 等问题；引入 typed boundary structs，完成查询历史服务端持久化。 |
| 6/15-6/18 | 浏览器采集字段修复、Hunter/ZoomEye/Shodan 闭环与 UI 调整 | 集中修复 FOFA/Hunter/Quake 选择器、Hunter 字段清洗、ZoomEye title、Shodan timestamp；修复 P0 admin-token 权限提升；现代化 Web UI。 |
| 6/20-6/23 | 引擎收敛、API 模式启用、安全审计与 map→struct 迁移 | 移除 BinaryEdge/Onyphe/GreyNoise，启用并修复 Censys/DayDayMap；完成安全审计修复；多引擎 adapter 由 map[string]interface{} 迁移到 typed structs。 |
| 6/25-6/29 | ICP 比对、篡改检测、安全加固、测试覆盖与归档清理 | 新增 ICP compare 前端；增强篡改检测和安全加固；关闭 CI 问题；集中补 Web handler/middleware 测试；清理运行时产物并归档历史文档。 |

## 三、主题归类

| 主题 | 梳理 |
| --- | --- |
| 认证与权限 | 多用户体系、admin/operator/readonly 角色、bootstrap 注册、session/CSRF 强化、admin-token 权限提升修复。 |
| 调度器与通知 | 任务分组、模板、Runner 测试负载、任务 payload 强类型化、飞书应用渠道、通知持久化与验证补齐。 |
| 搜索引擎适配 | 修正 FOFA/Hunter/Quake/ZoomEye/Shodan 语法和字段映射；新增并后续收敛 Censys/DayDayMap/GreyNoise 等引擎；完成 map→struct 迁移。 |
| 浏览器采集与截图 | 修复 Extension bridge、登录墙识别、选择器、collect_and_capture、截图等待时间和图片推送。 |
| 安全与稳定性 | 覆盖数据竞态、XSS、panic recovery、CSP、rate limiting、SSRF 说明、配置热更新、运行时产物入库等问题。 |
| 前端体验 | 修复查询结果渲染、错误展开、API 错误显示、登录/配额页面样式问题；新增 ICP 比对 UI 和账号/设置页能力。 |
| 测试覆盖 | 新增 parser、history API、scheduler、notification/config/screenshot/ICP/metrics/cookie/user/middleware/CDP/tamper 等测试，Web 覆盖率从 54.8% 提升至 59.1%。 |
| 文档与工程卫生 | 维护实施计划、审计记录、memory；归档历史规划文档，清理 hash_store/coverage 等运行时文件。 |

## 四、提交类型统计

| 类型 | 数量 |
| --- | ---: |
| fix | 49 |
| docs | 28 |
| refactor | 22 |
| feat | 21 |
| test | 10 |
| chore | 2 |
| style | 1 |

## 五、主要影响范围

| 范围 | 提交数 |
| --- | ---: |
| adapter | 40 |
| general | 14 |
| web | 14 |
| docs | 13 |
| scheduler | 9 |
| test | 7 |
| screenshot/extension | 6 |
| security/auth | 6 |
| config | 5 |
| screenshot | 4 |
| security | 3 |
| icp | 2 |

## 六、按日期提交数量

| 日期 | 提交数 |
| --- | ---: |
| 2026-06-02 | 13 |
| 2026-06-03 | 1 |
| 2026-06-07 | 6 |
| 2026-06-08 | 11 |
| 2026-06-09 | 18 |
| 2026-06-10 | 1 |
| 2026-06-11 | 5 |
| 2026-06-12 | 15 |
| 2026-06-15 | 6 |
| 2026-06-16 | 10 |
| 2026-06-17 | 2 |
| 2026-06-18 | 4 |
| 2026-06-20 | 1 |
| 2026-06-21 | 5 |
| 2026-06-22 | 7 |
| 2026-06-23 | 12 |
| 2026-06-25 | 2 |
| 2026-06-26 | 1 |
| 2026-06-27 | 6 |
| 2026-06-28 | 6 |
| 2026-06-29 | 1 |

## 七、后续关注点

- 6 月存在多轮 adapter 新增、修正、移除与强类型迁移，建议后续继续以真实 API 回归测试确认各引擎字段映射稳定。
- Web 测试覆盖率已经提升，但仍需关注跨页面端到端流程：登录、查询、截图、通知、定时任务执行和 ICP 比对。
- 运行时产物清理已完成，后续应保持 .gitignore 与测试输出目录同步，避免再次提交 hash_store、coverage 等生成文件。

## 八、提交明细

| 日期 | Hash | 类型 | 范围 | 提交说明 |
| --- | --- | --- | --- | --- |
| 2026-06-02 | $(@{FullHash=1b68df4d7e07a57e8b377cdd9e9a9e839dac9942; Hash=1b68df4; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=test; Subject=docs: 全部测试查询统一为 ip+port 组合格式}.Hash) | docs | test | docs: 全部测试查询统一为 ip+port 组合格式 |
| 2026-06-02 | $(@{FullHash=545c0633d3550f1e76f56e2e819cd917ed6d7b0d; Hash=545c063; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=test; Subject=docs: 测试查询改为单IP精确查询，结果控制在1条}.Hash) | docs | test | docs: 测试查询改为单IP精确查询，结果控制在1条 |
| 2026-06-02 | $(@{FullHash=58959698d868714d82b97665da1ee3f58f4d2eb1; Hash=5895969; Date=2026-06-02; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Extension模式P0 — 补齐Shodan搜索引擎URL构建}.Hash) | fix | adapter | fix: Extension模式P0 — 补齐Shodan搜索引擎URL构建 |
| 2026-06-02 | $(@{FullHash=629cea39593d561cd7000758b63680de0b35549d; Hash=629cea3; Date=2026-06-02; Author=ElaineRosa6; Type=feat; Scope=scheduler; Subject=feat: 定时任务系统前端优化+Runner验证+飞书通知增强 (P3-P8)}.Hash) | feat | scheduler | feat: 定时任务系统前端优化+Runner验证+飞书通知增强 (P3-P8) |
| 2026-06-02 | $(@{FullHash=6626b398d049f8aa2aca48a3b5f15e05a71b4252; Hash=6626b39; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=test; Subject=docs: 各引擎使用不同端口测试，避免重复结果}.Hash) | docs | test | docs: 各引擎使用不同端口测试，避免重复结果 |
| 2026-06-02 | $(@{FullHash=74a6e72a8c9106cc7195462b8e5249a48c53df55; Hash=74a6e72; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=test; Subject=docs: 测试查询从example.com/unimap改为稀有真实资产}.Hash) | docs | test | docs: 测试查询从example.com/unimap改为稀有真实资产 |
| 2026-06-02 | $(@{FullHash=9ad7c413b6487ff3aaf5695cebd8fb9b646dbe6e; Hash=9ad7c41; Date=2026-06-02; Author=ElaineRosa6; Type=fix; Scope=screenshot/extension; Subject=fix: Extension模式P1 — 修复登录墙检测被success=false吞掉的bug}.Hash) | fix | screenshot/extension | fix: Extension模式P1 — 修复登录墙检测被success=false吞掉的bug |
| 2026-06-02 | $(@{FullHash=b312879cdd9113534538a0aae62c18169f6e16cd; Hash=b312879; Date=2026-06-02; Author=ElaineRosa6; Type=fix; Scope=security/auth; Subject=fix: 认证系统安全加固 — 修复CRITICAL/HIGH审查问题}.Hash) | fix | security/auth | fix: 认证系统安全加固 — 修复CRITICAL/HIGH审查问题 |
| 2026-06-02 | $(@{FullHash=d07b421c9b3f37f3f09a4edff7bc328f3eae0eac; Hash=d07b421; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=scheduler; Subject=docs: 定时任务系统优化与测试计划}.Hash) | docs | scheduler | docs: 定时任务系统优化与测试计划 |
| 2026-06-02 | $(@{FullHash=d8468cd169c6f7aafe632b5d34b8ce4a31361396; Hash=d8468cd; Date=2026-06-02; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: 综合修复周期 — 31项修复 + 代码审查整改 + ICP/Hunter增强}.Hash) | fix | adapter | fix: 综合修复周期 — 31项修复 + 代码审查整改 + ICP/Hunter增强 |
| 2026-06-02 | $(@{FullHash=dd9b19ca0bc08cd4c158d41f8e00b43ce1be1400; Hash=dd9b19c; Date=2026-06-02; Author=ElaineRosa6; Type=fix; Scope=config; Subject=fix: 修复剩余4项问题 — config数据竞态/账户页多用户/Runner命名/Update变异}.Hash) | fix | config | fix: 修复剩余4项问题 — config数据竞态/账户页多用户/Runner命名/Update变异 |
| 2026-06-02 | $(@{FullHash=ea28bc3bfd3699e1282976500301fea76aca7fed; Hash=ea28bc3; Date=2026-06-02; Author=ElaineRosa6; Type=feat; Scope=general; Subject=feat: 用户管理系统 — 注册/登录/CRUD/密码修改/角色管理}.Hash) | feat | general | feat: 用户管理系统 — 注册/登录/CRUD/密码修改/角色管理 |
| 2026-06-02 | $(@{FullHash=fe5f8b4162a70d2cdb6076ec62b3594e457e15c9; Hash=fe5f8b4; Date=2026-06-02; Author=ElaineRosa6; Type=docs; Scope=scheduler; Subject=docs: 更新定时任务计划 — 飞书通知已实现+浏览器采集测试+Chrome MCP}.Hash) | docs | scheduler | docs: 更新定时任务计划 — 飞书通知已实现+浏览器采集测试+Chrome MCP |
| 2026-06-03 | $(@{FullHash=fc373583364739a9c49e0ef57c3ad21c2da5abd8; Hash=fc37358; Date=2026-06-03; Author=ElaineRosa6; Type=feat; Scope=scheduler; Subject=feat: bridge token 认证改造 + 通知增强 + 定时任务 P3-P8}.Hash) | feat | scheduler | feat: bridge token 认证改造 + 通知增强 + 定时任务 P3-P8 |
| 2026-06-07 | $(@{FullHash=3ce543dc3732a051ca4a62eb70c1da9de2aa707e; Hash=3ce543d; Date=2026-06-07; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: 文档归档整理 + 三层采集/反爬架构 + Quake 认知更正}.Hash) | docs | adapter | docs: 文档归档整理 + 三层采集/反爬架构 + Quake 认知更正 |
| 2026-06-07 | $(@{FullHash=5ffe1abca0b9cb19024c218a68517ef3e5ceb90a; Hash=5ffe1ab; Date=2026-06-07; Author=ElaineRosa6; Type=feat; Scope=notify; Subject=feat(notify): 飞书应用渠道 (feishu_app) + 截图路径泄露修复}.Hash) | feat | notify | feat(notify): 飞书应用渠道 (feishu_app) + 截图路径泄露修复 |
| 2026-06-07 | $(@{FullHash=72500ec56c5c53759b9f49a60430cf77cb1cad3a; Hash=72500ec; Date=2026-06-07; Author=ElaineRosa6; Type=fix; Scope=extension; Subject=fix(extension): queryOne 移入注入函数 — 修复采集 0 assets 根因}.Hash) | fix | extension | fix(extension): queryOne 移入注入函数 — 修复采集 0 assets 根因 |
| 2026-06-07 | $(@{FullHash=bacfb034d64a49fb2e562ed08ce8bc9cb3691e77; Hash=bacfb03; Date=2026-06-07; Author=ElaineRosa6; Type=feat; Scope=web; Subject=feat(web): admin token 端点 + 账号页/设置页前端}.Hash) | feat | web | feat(web): admin token 端点 + 账号页/设置页前端 |
| 2026-06-07 | $(@{FullHash=c4b14d532c67b30fdffcdd08c4a764bb6f3d7aaf; Hash=c4b14d5; Date=2026-06-07; Author=ElaineRosa6; Type=fix; Scope=extension; Subject=fix(extension): 重配对 bug 修复 + 选择器修复 + options 页}.Hash) | fix | extension | fix(extension): 重配对 bug 修复 + 选择器修复 + options 页 |
| 2026-06-07 | $(@{FullHash=f2d97da2837e8f807db82fcfe0a916d49ec84652; Hash=f2d97da; Date=2026-06-07; Author=ElaineRosa6; Type=fix; Scope=screenshot; Subject=fix(screenshot): 修复采集数据映射 + 添加 bridge 诊断日志}.Hash) | fix | screenshot | fix(screenshot): 修复采集数据映射 + 添加 bridge 诊断日志 |
| 2026-06-08 | $(@{FullHash=0e3fcc353bb2ee0f09b9f63bc8babfff5dc93525; Hash=0e3fcc3; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): 五引擎语法翻译全面修正 + 截图 URL 更新}.Hash) | fix | adapter | fix(adapter): 五引擎语法翻译全面修正 + 截图 URL 更新 |
| 2026-06-08 | $(@{FullHash=0e8b25742b5d4e2eb0cb39b1e83b1609d3977cb9; Hash=0e8b257; Date=2026-06-08; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: 实施计划更新 — 阶段二 P1/P2 已完成，状态头+Onyphe/BinaryEdge/DnsDB 标注}.Hash) | docs | adapter | docs: 实施计划更新 — 阶段二 P1/P2 已完成，状态头+Onyphe/BinaryEdge/DnsDB 标注 |
| 2026-06-08 | $(@{FullHash=3fd93deda70596af344e1bf876d91a188c036f76; Hash=3fd93de; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=security; Subject=fix(security): SEC-1 轮换泄露 admin token + 清理 git 跟踪}.Hash) | fix | security | fix(security): SEC-1 轮换泄露 admin token + 清理 git 跟踪 |
| 2026-06-08 | $(@{FullHash=61c3549428a3f0e217d6f196a5fcc906ba24c75d; Hash=61c3549; Date=2026-06-08; Author=ElaineRosa6; Type=feat; Scope=adapter; Subject=feat(adapter): 新增 Censys 搜索引擎适配器 (P1)}.Hash) | feat | adapter | feat(adapter): 新增 Censys 搜索引擎适配器 (P1) |
| 2026-06-08 | $(@{FullHash=7b013a886755f9057d3a1cb623e635f974d9d68f; Hash=7b013a8; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=docs; Subject=fix(docs): DnsDB 标注已停用，从实施计划移除}.Hash) | fix | docs | fix(docs): DnsDB 标注已停用，从实施计划移除 |
| 2026-06-08 | $(@{FullHash=7ea19d35d8efd414de3081620314d43044a026e0; Hash=7ea19d3; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): FOFA B-1a/B-4a/B-7 修复 + ZoomEye分隔符确认}.Hash) | fix | adapter | fix(adapter): FOFA B-1a/B-4a/B-7 修复 + ZoomEye分隔符确认 |
| 2026-06-08 | $(@{FullHash=a3ed95bb0d2bdf063ad8cbb74ff6a759b7d7a4ca; Hash=a3ed95b; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=docs; Subject=fix(docs): 语法参考手册全量审计修正 + BinaryEdge/Onyphe adapter 修正}.Hash) | fix | docs | fix(docs): 语法参考手册全量审计修正 + BinaryEdge/Onyphe adapter 修正 |
| 2026-06-08 | $(@{FullHash=b9c5fabcb4173e485dd192cca3171aea5d354d14; Hash=b9c5fab; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): 5引擎官方语法核查 + 4处映射bug修复}.Hash) | fix | adapter | fix(adapter): 5引擎官方语法核查 + 4处映射bug修复 |
| 2026-06-08 | $(@{FullHash=d80c7adcbd80780300bb4b18ad5b994312ca2368; Hash=d80c7ad; Date=2026-06-08; Author=ElaineRosa6; Type=fix; Scope=docs; Subject=fix(docs): DayDayMap 语法核查确认 + vul.cve 模糊匹配标注}.Hash) | fix | docs | fix(docs): DayDayMap 语法核查确认 + vul.cve 模糊匹配标注 |
| 2026-06-08 | $(@{FullHash=ee6bc31db3c9fb17542ab26899873da4275e93d4; Hash=ee6bc31; Date=2026-06-08; Author=ElaineRosa6; Type=feat; Scope=adapter; Subject=feat(adapter): 新增 DayDayMap 搜索引擎适配器 (P1)}.Hash) | feat | adapter | feat(adapter): 新增 DayDayMap 搜索引擎适配器 (P1) |
| 2026-06-08 | $(@{FullHash=fd583b3ca8e1837e832d2b0193d3ad9a5dae72a8; Hash=fd583b3; Date=2026-06-08; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: 全量实施计划 + 搜索引擎语法参考手册}.Hash) | docs | docs | docs: 全量实施计划 + 搜索引擎语法参考手册 |
| 2026-06-09 | $(@{FullHash=0e80116dd152446fe555fd54af25a0d2adc4066e; Hash=0e80116; Date=2026-06-09; Author=ElaineRosa6; Type=docs; Scope=screenshot/extension; Subject=docs: 更新文档反映 API shim 移除 + 代码拆分进度 + 三层采集 spike 结论}.Hash) | docs | screenshot/extension | docs: 更新文档反映 API shim 移除 + 代码拆分进度 + 三层采集 spike 结论 |
| 2026-06-09 | $(@{FullHash=17056000b8571776ecd8645fef5ff55cc211b573; Hash=1705600; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=screenshot; Subject=refactor(screenshot): manager.go 1189行拆分为4个文件}.Hash) | refactor | screenshot | refactor(screenshot): manager.go 1189行拆分为4个文件 |
| 2026-06-09 | $(@{FullHash=179e269a10103171da1fc12f8650d5b3b5208bcd; Hash=179e269; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): orchestrator.go 910行拆分为3个文件}.Hash) | refactor | adapter | refactor(adapter): orchestrator.go 910行拆分为3个文件 |
| 2026-06-09 | $(@{FullHash=24a37f7420899bed4152d1c7637be73dadbc2b8e; Hash=24a37f7; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=web; Subject=refactor(web): 移除 API 旧路径 shim + 修复 monitor 测试}.Hash) | refactor | web | refactor(web): 移除 API 旧路径 shim + 修复 monitor 测试 |
| 2026-06-09 | $(@{FullHash=398ee4ce44a4dc30b0fd9423f0464e5978cb8e9d; Hash=398ee4c; Date=2026-06-09; Author=ElaineRosa6; Type=feat; Scope=adapter; Subject=feat(adapter): 新增 GreyNoise 威胁情报搜索引擎适配器 (P3)}.Hash) | feat | adapter | feat(adapter): 新增 GreyNoise 威胁情报搜索引擎适配器 (P3) |
| 2026-06-09 | $(@{FullHash=45ec4ccb5c2eeb9778d936ecf93b0f2ff3be0751; Hash=45ec4cc; Date=2026-06-09; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: CLAUDE.md 标记文件超800行全部闭环}.Hash) | docs | docs | docs: CLAUDE.md 标记文件超800行全部闭环 |
| 2026-06-09 | $(@{FullHash=498d19ad38be630e6ff6618bfb5ab96d19c13e83; Hash=498d19a; Date=2026-06-09; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): DayDayMap cert 映射修正 cert.subject.cn → cert.subject + 测试}.Hash) | fix | adapter | fix(adapter): DayDayMap cert 映射修正 cert.subject.cn → cert.subject + 测试 |
| 2026-06-09 | $(@{FullHash=4e621b69b82491d87ce19431cd1cedb3c854f58f; Hash=4e621b6; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=screenshot; Subject=refactor(screenshot): router.go 853行拆分为2个文件}.Hash) | refactor | screenshot | refactor(screenshot): router.go 853行拆分为2个文件 |
| 2026-06-09 | $(@{FullHash=566fec7dea223552dab07b18b7213e1fd0e05457; Hash=566fec7; Date=2026-06-09; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: ENGINE_ADAPTER_PLAN 标记 TD-1 文件拆分已完成}.Hash) | docs | adapter | docs: ENGINE_ADAPTER_PLAN 标记 TD-1 文件拆分已完成 |
| 2026-06-09 | $(@{FullHash=567c251a86887ebe6f90b8ea502c976e64b31fc1; Hash=567c251; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=scheduler; Subject=refactor(scheduler): scheduler.go 1545行拆分为4个文件}.Hash) | refactor | scheduler | refactor(scheduler): scheduler.go 1545行拆分为4个文件 |
| 2026-06-09 | $(@{FullHash=5f4361213812aabbf5052381328efba75fa57513; Hash=5f43612; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=tamper; Subject=refactor(tamper): detector.go 1797行拆分为3个文件}.Hash) | refactor | tamper | refactor(tamper): detector.go 1797行拆分为3个文件 |
| 2026-06-09 | $(@{FullHash=710dcb0714baf8ac16e087296678c04393c71f2a; Hash=710dcb0; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=general; Subject=refactor: TD-2函数拆分+CR-3/CR-5确认+ARC-6/7/8架构改进}.Hash) | refactor | general | refactor: TD-2函数拆分+CR-3/CR-5确认+ARC-6/7/8架构改进 |
| 2026-06-09 | $(@{FullHash=838d7ce4db80dfee5a0d97816c48105847776d45; Hash=838d7ce; Date=2026-06-09; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: 更新文档反映9个大文件拆分全量闭环}.Hash) | docs | docs | docs: 更新文档反映9个大文件拆分全量闭环 |
| 2026-06-09 | $(@{FullHash=8fc7af7006138eea68bc8b38e09c121e5f6e0ccd; Hash=8fc7af7; Date=2026-06-09; Author=ElaineRosa6; Type=feat; Scope=general; Subject=feat: L1 Network层实现+函数拆分续+ARC-4全量验证}.Hash) | feat | general | feat: L1 Network层实现+函数拆分续+ARC-4全量验证 |
| 2026-06-09 | $(@{FullHash=c6fcd387198ea61864430e0a2bed2b5ce87a2487; Hash=c6fcd38; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=config; Subject=refactor(config): applyDefaults 446行拆分为7个子函数}.Hash) | refactor | config | refactor(config): applyDefaults 446行拆分为7个子函数 |
| 2026-06-09 | $(@{FullHash=cc8545f617f57d72081cab739385531d97373d29; Hash=cc8545f; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=scheduler; Subject=refactor(scheduler): executor.go 1720行拆分为3个文件}.Hash) | refactor | scheduler | refactor(scheduler): executor.go 1720行拆分为3个文件 |
| 2026-06-09 | $(@{FullHash=d08e9929207ad572c1c89e49e67f12ff6297d0d1; Hash=d08e992; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=service; Subject=refactor(service): screenshot_app_service.go 817行拆分为3个文件}.Hash) | refactor | service | refactor(service): screenshot_app_service.go 817行拆分为3个文件 |
| 2026-06-09 | $(@{FullHash=d1fecb8cd6582ed94d4120250e34f3d8c094aa4d; Hash=d1fecb8; Date=2026-06-09; Author=ElaineRosa6; Type=refactor; Scope=config; Subject=refactor(config): config.go 1074行拆分为4个文件}.Hash) | refactor | config | refactor(config): config.go 1074行拆分为4个文件 |
| 2026-06-10 | $(@{FullHash=a28044925a503cdf9c6c80a07c59b5b9daf158f2; Hash=a280449; Date=2026-06-10; Author=ElaineRosa6; Type=feat; Scope=screenshot/extension; Subject=feat: L1 Network采集集成到combined collect+capture路径 + service层修复}.Hash) | feat | screenshot/extension | feat: L1 Network采集集成到combined collect+capture路径 + service层修复 |
| 2026-06-11 | $(@{FullHash=18eb28877e4c40f5d8c3c209fe532bf8d1429e7b; Hash=18eb288; Date=2026-06-11; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: quota.html unclosed style tag caused 500 on quota page}.Hash) | fix | general | fix: quota.html unclosed style tag caused 500 on quota page |
| 2026-06-11 | $(@{FullHash=79ce4522b5d1f5f8289cc1dd8a433170777bb3bf; Hash=79ce452; Date=2026-06-11; Author=ElaineRosa6; Type=fix; Scope=security/auth; Subject=fix: security & stability fixes — CRITICAL data race, P0 XSS, P1 panic recovery, resource monitor idempotent stop, fetch unified resp.ok check, screenshot app service RWMutex, batch metrics correction, cleanup goroutine, logger sync, dockerignore}.Hash) | fix | security/auth | fix: security & stability fixes — CRITICAL data race, P0 XSS, P1 panic recovery, resource monitor idempotent stop, fetch unified resp.ok check, screenshot app service RWMutex, batch metrics correction, cleanup goroutine, logger sync, dockerignore |
| 2026-06-11 | $(@{FullHash=db055679c192c21fb566dae1ba74a6334dc10118; Hash=db05567; Date=2026-06-11; Author=ElaineRosa6; Type=refactor; Scope=security/auth; Subject=refactor: CSP unsafe-inline removal, admin endpoint rate limiting, handleScreenshot via Router, auth test coverage 94.7%, server.go split}.Hash) | refactor | security/auth | refactor: CSP unsafe-inline removal, admin endpoint rate limiting, handleScreenshot via Router, auth test coverage 94.7%, server.go split |
| 2026-06-11 | $(@{FullHash=e6576d41054c0930d85ce1c4b399d414e4f42dd8; Hash=e6576d4; Date=2026-06-11; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: login.html missing utils.css link — hidden-init class undefined, decoy fields/spinner/error visible}.Hash) | fix | general | fix: login.html missing utils.css link — hidden-init class undefined, decoy fields/spinner/error visible |
| 2026-06-11 | $(@{FullHash=f91584b40a1419c7836ab5329271751ee1dc1df1; Hash=f91584b; Date=2026-06-11; Author=ElaineRosa6; Type=feat; Scope=screenshot/extension; Subject=feat: batch screenshot progress provider interface + main.js API refactor + full project audit docs}.Hash) | feat | screenshot/extension | feat: batch screenshot progress provider interface + main.js API refactor + full project audit docs |
| 2026-06-12 | $(@{FullHash=125b010eead9c6df744a509fd18ab859310e2879; Hash=125b010; Date=2026-06-12; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: query page loading stuck, timeout decoupling, engine filtering, domain corrections, login status display}.Hash) | fix | adapter | fix: query page loading stuck, timeout decoupling, engine filtering, domain corrections, login status display |
| 2026-06-12 | $(@{FullHash=138c6a627388c7527c115794d784d332d2ae5069; Hash=138c6a6; Date=2026-06-12; Author=ElaineRosa6; Type=feat; Scope=general; Subject=feat: migrate query history modal from localStorage to server API}.Hash) | feat | general | feat: migrate query history modal from localStorage to server API |
| 2026-06-12 | $(@{FullHash=1ac93c1cdb47177148f3556970d15b6eafa85df9; Hash=1ac93c1; Date=2026-06-12; Author=ElaineRosa6; Type=test; Scope=test; Subject=test: add history API integration tests}.Hash) | test | test | test: add history API integration tests |
| 2026-06-12 | $(@{FullHash=1e61088d8a2bc3c2ab302e25cc42adfb6482a299; Hash=1e61088; Date=2026-06-12; Author=ElaineRosa6; Type=feat; Scope=scheduler; Subject=feat: add typed boundary structs for plugin, scheduler, bridge, and API responses}.Hash) | feat | scheduler | feat: add typed boundary structs for plugin, scheduler, bridge, and API responses |
| 2026-06-12 | $(@{FullHash=1fdedde6a10f3185a698ea2709432a75bae1b337; Hash=1fdedde; Date=2026-06-12; Author=ElaineRosa6; Type=feat; Scope=general; Subject=feat: auto-save query results to server history on successful query}.Hash) | feat | general | feat: auto-save query results to server history on successful query |
| 2026-06-12 | $(@{FullHash=45dc40f119c26bfd50e0dbcfe02cc54b394ffef9; Hash=45dc40f; Date=2026-06-12; Author=ElaineRosa6; Type=feat; Scope=test; Subject=feat: add history database, models, repository with tests}.Hash) | feat | test | feat: add history database, models, repository with tests |
| 2026-06-12 | $(@{FullHash=54f8d0dac5261c89ff8c84d037923de2ac96f8b5; Hash=54f8d0d; Date=2026-06-12; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: update CLAUDE.md with TD-4/L-05 completion and history persistence}.Hash) | docs | docs | docs: update CLAUDE.md with TD-4/L-05 completion and history persistence |
| 2026-06-12 | $(@{FullHash=579d00fa4917bbe73737388a9d1a2b055d04845a; Hash=579d00f; Date=2026-06-12; Author=ElaineRosa6; Type=test; Scope=screenshot/extension; Subject=test: update test files for typed TaskPayload and BridgeCollectedData}.Hash) | test | screenshot/extension | test: update test files for typed TaskPayload and BridgeCollectedData |
| 2026-06-12 | $(@{FullHash=687079d62289908e5117112ba9af2b8097461233; Hash=687079d; Date=2026-06-12; Author=ElaineRosa6; Type=refactor; Scope=web; Subject=refactor: migrate web API handlers to typed APIResponse structs}.Hash) | refactor | web | refactor: migrate web API handlers to typed APIResponse structs |
| 2026-06-12 | $(@{FullHash=69f00e7d57239c14965f569e704953668d826917; Hash=69f00e7; Date=2026-06-12; Author=ElaineRosa6; Type=refactor; Scope=screenshot/extension; Subject=refactor: migrate BridgeResult.StructuredCollectedData to typed BridgeCollectedData}.Hash) | refactor | screenshot/extension | refactor: migrate BridgeResult.StructuredCollectedData to typed BridgeCollectedData |
| 2026-06-12 | $(@{FullHash=76abf7c3bd1034f48b09853edff6982511e01c7a; Hash=76abf7c; Date=2026-06-12; Author=ElaineRosa6; Type=refactor; Scope=general; Subject=refactor: migrate distributed task queue to typed TaskPayload and TaskOutput}.Hash) | refactor | general | refactor: migrate distributed task queue to typed TaskPayload and TaskOutput |
| 2026-06-12 | $(@{FullHash=7c9b1745944a792d1f7c6204fe25f8658bbf558d; Hash=7c9b174; Date=2026-06-12; Author=ElaineRosa6; Type=refactor; Scope=config; Subject=refactor: migrate plugin interfaces to typed structs (PluginConfig, HookData, HealthDetails, NotificationMetadata)}.Hash) | refactor | config | refactor: migrate plugin interfaces to typed structs (PluginConfig, HookData, HealthDetails, NotificationMetadata) |
| 2026-06-12 | $(@{FullHash=884aeffa54cf6bd900575de703f8cb67ddef5a41; Hash=884aeff; Date=2026-06-12; Author=ElaineRosa6; Type=feat; Scope=config; Subject=feat: add history config, database wiring, and API handlers}.Hash) | feat | config | feat: add history config, database wiring, and API handlers |
| 2026-06-12 | $(@{FullHash=96afcab0374f8bbd9c8b6f7942a47d8b5873d677; Hash=96afcab; Date=2026-06-12; Author=ElaineRosa6; Type=refactor; Scope=scheduler; Subject=refactor: migrate scheduler to typed TaskPayload and TaskHandler interface}.Hash) | refactor | scheduler | refactor: migrate scheduler to typed TaskPayload and TaskHandler interface |
| 2026-06-12 | $(@{FullHash=a44a637c11aed55d6df7c67020c07da55e1b96b1; Hash=a44a637; Date=2026-06-12; Author=ElaineRosa6; Type=fix; Scope=icp; Subject=fix: resolve PageSizeICP JSON tag collision that silently dropped ICP page_size=40}.Hash) | fix | icp | fix: resolve PageSizeICP JSON tag collision that silently dropped ICP page_size=40 |
| 2026-06-15 | $(@{FullHash=0bb15353f86adcf986bd19ccf9b10dd9e03c249d; Hash=0bb1535; Date=2026-06-15; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: collect_and_capture syntax error — else if after else}.Hash) | fix | general | fix: collect_and_capture syntax error — else if after else |
| 2026-06-15 | $(@{FullHash=987c310bf9085c46e42df12750c46024061f334a; Hash=987c310; Date=2026-06-15; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: update stale Go CDP selectors for FOFA, Hunter, Quake}.Hash) | fix | adapter | fix: update stale Go CDP selectors for FOFA, Hunter, Quake |
| 2026-06-15 | $(@{FullHash=9d5de504ab95cc11a5d3a79b74be6f11a7b50431; Hash=9d5de50; Date=2026-06-15; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: collect_and_capture variable bug, HasMore field, parser consistency}.Hash) | fix | general | fix: collect_and_capture variable bug, HasMore field, parser consistency |
| 2026-06-15 | $(@{FullHash=b7a9a3479fe7da35a94e4affa3a504afc84bb42c; Hash=b7a9a34; Date=2026-06-15; Author=ElaineRosa6; Type=test; Scope=test; Subject=test: add comprehensive tests for collection/parser package}.Hash) | test | test | test: add comprehensive tests for collection/parser package |
| 2026-06-15 | $(@{FullHash=d1cb19b8b4357e9e6d954cb66742cd98f2ea2243; Hash=d1cb19b; Date=2026-06-15; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: audit fixes — login wall, field regressions, code quality}.Hash) | fix | general | fix: audit fixes — login wall, field regressions, code quality |
| 2026-06-15 | $(@{FullHash=faed71ddb732073f08cf057b6b6573c3c8d8b9db; Hash=faed71d; Date=2026-06-15; Author=ElaineRosa6; Type=feat; Scope=adapter; Subject=feat: ZoomEye selectors update, SPA timing fix, collect_and_capture, port extraction, web tests}.Hash) | feat | adapter | feat: ZoomEye selectors update, SPA timing fix, collect_and_capture, port extraction, web tests |
| 2026-06-16 | $(@{FullHash=14452472fc2ae6d03e7eea3ee3c1060f9850ecff; Hash=1445247; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Hunter DOM extraction — remove tooltip cross-row pollution}.Hash) | fix | adapter | fix: Hunter DOM extraction — remove tooltip cross-row pollution |
| 2026-06-16 | $(@{FullHash=36582f625a8b587c3a0a828e918337464810e08d; Hash=36582f6; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: remove Hunter-specific post-processing from capture.js}.Hash) | fix | adapter | fix: remove Hunter-specific post-processing from capture.js |
| 2026-06-16 | $(@{FullHash=5ac6a4db69f07f7a00d85d67028f533c0b6474cf; Hash=5ac6a4d; Date=2026-06-16; Author=ElaineRosa6; Type=feat; Scope=adapter; Subject=feat: Go-side Hunter field cleanup (country/title/host)}.Hash) | feat | adapter | feat: Go-side Hunter field cleanup (country/title/host) |
| 2026-06-16 | $(@{FullHash=905ff3ad3ceb4ee4b6e93e16197769f287868144; Hash=905ff3a; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Hunter extraction — protocol dedup title cleanup}.Hash) | fix | adapter | fix: Hunter extraction — protocol dedup title cleanup |
| 2026-06-16 | $(@{FullHash=93ac99e204db6635700a7599278ea1c0add08678; Hash=93ac99e; Date=2026-06-16; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: update verified issue list (13 items, all confirmed)}.Hash) | docs | docs | docs: update verified issue list (13 items, all confirmed) |
| 2026-06-16 | $(@{FullHash=aac6127c02245df24ca256c799966e48cc38dddd; Hash=aac6127; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Hunter dedup + country/title/host cleanup}.Hash) | fix | adapter | fix: Hunter dedup + country/title/host cleanup |
| 2026-06-16 | $(@{FullHash=b30f49fdc26c4aed4f8ad2f9327ef0ca668205a1; Hash=b30f49f; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: simplify Hunter title cleanup regex to avoid crash}.Hash) | fix | adapter | fix: simplify Hunter title cleanup regex to avoid crash |
| 2026-06-16 | $(@{FullHash=dce12201a26967b7739464fb2f88767ee5938d4a; Hash=dce1220; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Hunter extraction — raw cell text + protocol/title cleanup}.Hash) | fix | adapter | fix: Hunter extraction — raw cell text + protocol/title cleanup |
| 2026-06-16 | $(@{FullHash=e2b6097e43734c2a57e01b4f06ec11c27142abd1; Hash=e2b6097; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: revert extractCellTextFromCells to span-based extraction}.Hash) | fix | general | fix: revert extractCellTextFromCells to span-based extraction |
| 2026-06-16 | $(@{FullHash=f729655b0642d6c4d5623b5cbd0c1ecce2f6195b; Hash=f729655; Date=2026-06-16; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: Hunter title/host cleanup — preserve domain, strip category labels}.Hash) | fix | adapter | fix: Hunter title/host cleanup — preserve domain, strip category labels |
| 2026-06-17 | $(@{FullHash=37bc1d996e37f91f034da6fddb5fc5ab65ed0eb2; Hash=37bc1d9; Date=2026-06-17; Author=ElaineRosa6; Type=fix; Scope=security; Subject=fix(security): P0 admin-token privilege escalation + multi-user enablement}.Hash) | fix | security | fix(security): P0 admin-token privilege escalation + multi-user enablement |
| 2026-06-17 | $(@{FullHash=50dc187efabc5efdae8e1396e4d76e4325b2b72c; Hash=50dc187; Date=2026-06-17; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix: ZoomEye title precise extraction + Shodan timestamp field flow}.Hash) | fix | adapter | fix: ZoomEye title precise extraction + Shodan timestamp field flow |
| 2026-06-18 | $(@{FullHash=307b328c16e06c88d1841d8674d2edb9cddf5f00; Hash=307b328; Date=2026-06-18; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: update CLAUDE.md + memory with 2026-06-18 fixes}.Hash) | docs | docs | docs: update CLAUDE.md + memory with 2026-06-18 fixes |
| 2026-06-18 | $(@{FullHash=a6948bdcd9d1bc987c92b380b2aba58fe6736cd7; Hash=a6948bd; Date=2026-06-18; Author=ElaineRosa6; Type=fix; Scope=screenshot; Subject=fix(screenshot): Unify wait timing to 15s + fix feishu_app image push}.Hash) | fix | screenshot | fix(screenshot): Unify wait timing to 15s + fix feishu_app image push |
| 2026-06-18 | $(@{FullHash=aba13aa24a594f8a6c657d60a6733190f90a2325; Hash=aba13aa; Date=2026-06-18; Author=ElaineRosa6; Type=fix; Scope=web; Subject=fix(web): rate-limit + frontend interaction bugs (CSP, hidden-init, typos)}.Hash) | fix | web | fix(web): rate-limit + frontend interaction bugs (CSP, hidden-init, typos) |
| 2026-06-18 | $(@{FullHash=d013932b935b3fa487994fe0df4bd7a3fa4c9d53; Hash=d013932; Date=2026-06-18; Author=ElaineRosa6; Type=style; Scope=web; Subject=style(web): modernize UI — indigo accent, layered shadows, gradients}.Hash) | style | web | style(web): modernize UI — indigo accent, layered shadows, gradients |
| 2026-06-20 | $(@{FullHash=fb6dcdbdbb02e65a4e444bc944ba522099679046; Hash=fb6dcdb; Date=2026-06-20; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): remove BinaryEdge/Onyphe/GreyNoise engines}.Hash) | refactor | adapter | refactor(adapter): remove BinaryEdge/Onyphe/GreyNoise engines |
| 2026-06-21 | $(@{FullHash=0a6adeb44f01348a292083185f59604f010b0f96; Hash=0a6adeb; Date=2026-06-21; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): Quake 响应解析兼容 data 为对象格式}.Hash) | fix | adapter | fix(adapter): Quake 响应解析兼容 data 为对象格式 |
| 2026-06-21 | $(@{FullHash=5bf4ab1aed162fcf5f28cfb441cada148e4e3b27; Hash=5bf4ab1; Date=2026-06-21; Author=ElaineRosa6; Type=fix; Scope=web; Subject=fix(web): 修复查询结果表格不渲染和错误展开失效}.Hash) | fix | web | fix(web): 修复查询结果表格不渲染和错误展开失效 |
| 2026-06-21 | $(@{FullHash=a283689f84c364e12ad39497943ad868d96819ba; Hash=a283689; Date=2026-06-21; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: sync CLAUDE.md remaining-work to 7-engine state (fb6dcdb)}.Hash) | docs | adapter | docs: sync CLAUDE.md remaining-work to 7-engine state (fb6dcdb) |
| 2026-06-21 | $(@{FullHash=e588988d27518f111d6cdd31f54611e2a6c7f137; Hash=e588988; Date=2026-06-21; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: 记录 2026-06-21 前端/Quake 修复（14 项）}.Hash) | docs | adapter | docs: 记录 2026-06-21 前端/Quake 修复（14 项） |
| 2026-06-21 | $(@{FullHash=fe30cbd63194eb01f667c0e1a90eff48e73aa516; Hash=fe30cbd; Date=2026-06-21; Author=ElaineRosa6; Type=fix; Scope=web; Subject=fix(web): 修复前端 API 错误显示与交互问题 (10 项)}.Hash) | fix | web | fix(web): 修复前端 API 错误显示与交互问题 (10 项) |
| 2026-06-22 | $(@{FullHash=07581a813b4162ade4f59a56b5dba224b8fae730; Hash=07581a8; Date=2026-06-22; Author=ElaineRosa6; Type=fix; Scope=censys; Subject=fix(censys): rewrite adapter for v3 API (free tier, Bearer token)}.Hash) | fix | censys | fix(censys): rewrite adapter for v3 API (free tier, Bearer token) |
| 2026-06-22 | $(@{FullHash=4fa161c4e7d6888cf226fc3292370efd2c688f7d; Hash=4fa161c; Date=2026-06-22; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: 记录 ZoomEye title 清理修复}.Hash) | docs | adapter | docs: 记录 ZoomEye title 清理修复 |
| 2026-06-22 | $(@{FullHash=76d6535446d8da852e11147bd38b9b0c38f27f41; Hash=76d6535; Date=2026-06-22; Author=ElaineRosa6; Type=docs; Scope=security/auth; Subject=docs: mark FINDING-003 SSRF as already fixed by urlguard.SafeHTTPClient}.Hash) | docs | security/auth | docs: mark FINDING-003 SSRF as already fixed by urlguard.SafeHTTPClient |
| 2026-06-22 | $(@{FullHash=7e619f83b4ec5d30d16b61c14c291e8e687fcb9b; Hash=7e619f8; Date=2026-06-22; Author=ElaineRosa6; Type=fix; Scope=zoomeye; Subject=fix(zoomeye): 清理 title 中的元数据前缀}.Hash) | fix | zoomeye | fix(zoomeye): 清理 title 中的元数据前缀 |
| 2026-06-22 | $(@{FullHash=8e23f9f60b591b93ad971e4ebc30da942a3d9756; Hash=8e23f9f; Date=2026-06-22; Author=ElaineRosa6; Type=fix; Scope=security; Subject=fix(security): audit FINDING-002/004/005/006 + doc corrections}.Hash) | fix | security | fix(security): audit FINDING-002/004/005/006 + doc corrections |
| 2026-06-22 | $(@{FullHash=f89d4b129d6eb711e206d64f5e15f1153bb3d6bb; Hash=f89d4b1; Date=2026-06-22; Author=ElaineRosa6; Type=fix; Scope=daydaymap; Subject=fix(daydaymap): rewrite adapter to match actual API format}.Hash) | fix | daydaymap | fix(daydaymap): rewrite adapter to match actual API format |
| 2026-06-22 | $(@{FullHash=fa314edc0bbf093a8a5f75a67856c935a35333fe; Hash=fa314ed; Date=2026-06-22; Author=ElaineRosa6; Type=feat; Scope=engines; Subject=feat(engines): enable Censys + DayDayMap with API mode}.Hash) | feat | engines | feat(engines): enable Censys + DayDayMap with API mode |
| 2026-06-23 | $(@{FullHash=1476490529984a3c36c32f146dd2678368afce14; Hash=1476490; Date=2026-06-23; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): migrate ZoomEye (17→5) and Quake (12→6) to typed structs}.Hash) | refactor | adapter | refactor(adapter): migrate ZoomEye (17→5) and Quake (12→6) to typed structs |
| 2026-06-23 | $(@{FullHash=1bcc688f84a2bf05a28027cb243e339ef3faad8b; Hash=1bcc688; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: mark Shodan timestamp + ZoomEye title as fixed (both resolved in 50dc187/9debb8f)}.Hash) | docs | adapter | docs: mark Shodan timestamp + ZoomEye title as fixed (both resolved in 50dc187/9debb8f) |
| 2026-06-23 | $(@{FullHash=4b8de2b8fdce47d64ca5d0008e46b80ae27cbb7a; Hash=4b8de2b; Date=2026-06-23; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): migrate Censys from map[string]interface{} to typed structs (25→0)}.Hash) | refactor | adapter | refactor(adapter): migrate Censys from map[string]interface{} to typed structs (25→0) |
| 2026-06-23 | $(@{FullHash=58ce18097f4616d7b533de0d60b2a1347f349c50; Hash=58ce180; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: update memory files with map→struct Phase 7 patterns and knowledge}.Hash) | docs | docs | docs: update memory files with map→struct Phase 7 patterns and knowledge |
| 2026-06-23 | $(@{FullHash=72c66747b1d71266983c0392f4ab2393a8db545c; Hash=72c6674; Date=2026-06-23; Author=ElaineRosa6; Type=fix; Scope=scheduler; Subject=fix(scheduler): close ScheduleType validation gaps + harden persistence/notifications}.Hash) | fix | scheduler | fix(scheduler): close ScheduleType validation gaps + harden persistence/notifications |
| 2026-06-23 | $(@{FullHash=7fb4998c9ee65050060a1fc5dc0c134f36b91af1; Hash=7fb4998; Date=2026-06-23; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): migrate Shodan/Hunter/Fofa from map[string]interface{} to typed structs}.Hash) | refactor | adapter | refactor(adapter): migrate Shodan/Hunter/Fofa from map[string]interface{} to typed structs |
| 2026-06-23 | $(@{FullHash=84c23d78ac17cc32363f4911aa44ffc6091359d0; Hash=84c23d7; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: update memory files — Phase 7完结, ZoomEye/Quake patterns, json.Number/json.RawMessage tech constraints}.Hash) | docs | adapter | docs: update memory files — Phase 7完结, ZoomEye/Quake patterns, json.Number/json.RawMessage tech constraints |
| 2026-06-23 | $(@{FullHash=989df6b9ee04e77950ffd38541a95105848532a6; Hash=989df6b; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: update remaining work — Censys/DayDayMap verified, ZoomEye fixed, exe gitignored}.Hash) | docs | adapter | docs: update remaining work — Censys/DayDayMap verified, ZoomEye fixed, exe gitignored |
| 2026-06-23 | $(@{FullHash=9debb8f463a1e141ef10dd2a73d85764da59c7b5; Hash=9debb8f; Date=2026-06-23; Author=ElaineRosa6; Type=fix; Scope=shodan,zoomeye; Subject=fix(shodan,zoomeye): map last_seen/timestamp to UnifiedAsset.LastSeen}.Hash) | fix | shodan,zoomeye | fix(shodan,zoomeye): map last_seen/timestamp to UnifiedAsset.LastSeen |
| 2026-06-23 | $(@{FullHash=a2a6beb784b765cb1ca830aff104c9092640752d; Hash=a2a6beb; Date=2026-06-23; Author=ElaineRosa6; Type=refactor; Scope=adapter; Subject=refactor(adapter): migrate DayDayMap from map[string]interface{} to typed structs + fix tests}.Hash) | refactor | adapter | refactor(adapter): migrate DayDayMap from map[string]interface{} to typed structs + fix tests |
| 2026-06-23 | $(@{FullHash=b6b8714c52acb6e0ed85c34ac53aa7c7da0143b8; Hash=b6b8714; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: update map→struct migration status (Phase 7 complete, 799→~170)}.Hash) | docs | docs | docs: update map→struct migration status (Phase 7 complete, 799→~170) |
| 2026-06-23 | $(@{FullHash=e8dc3c2139ab22c78a47800c34fc2f09e2b3238d; Hash=e8dc3c2; Date=2026-06-23; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: finalize map→struct migration (ZoomEye+Quake, Phase 7 complete)}.Hash) | docs | adapter | docs: finalize map→struct migration (ZoomEye+Quake, Phase 7 complete) |
| 2026-06-25 | $(@{FullHash=6f98928e2c3372178738e888aae544564abe58e3; Hash=6f98928; Date=2026-06-25; Author=ElaineRosa6; Type=feat; Scope=icp; Subject=feat(icp): add frontend compare UI for /api/v1/icp/compare}.Hash) | feat | icp | feat(icp): add frontend compare UI for /api/v1/icp/compare |
| 2026-06-25 | $(@{FullHash=c52e20fd8e4a21c8860a1d12c4b82bd4699058ab; Hash=c52e20f; Date=2026-06-25; Author=ElaineRosa6; Type=feat; Scope=security/auth; Subject=feat: tamper detection enhancements + security hardening}.Hash) | feat | security/auth | feat: tamper detection enhancements + security hardening |
| 2026-06-26 | $(@{FullHash=a20304968a51910f05e85bcfc95a2011fa0187bc; Hash=a203049; Date=2026-06-26; Author=ElaineRosa6; Type=fix; Scope=general; Subject=fix: close out CI workflow issues}.Hash) | fix | general | fix: close out CI workflow issues |
| 2026-06-27 | $(@{FullHash=3398534f10f206c32ca68cdfc250be81f302a34d; Hash=3398534; Date=2026-06-27; Author=ElaineRosa6; Type=docs; Scope=adapter; Subject=docs: update Censys/DayDayMap verification status (2026-06-27 real machine API test passed)}.Hash) | docs | adapter | docs: update Censys/DayDayMap verification status (2026-06-27 real machine API test passed) |
| 2026-06-27 | $(@{FullHash=4ada8e3f118f76ec0ed64bd65f33fe9b2e60115c; Hash=4ada8e3; Date=2026-06-27; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve coverage from 54.8% to 59.1% — add tests for notification handlers, config handlers, screenshot handlers, ICP handlers, metrics, cookie handlers}.Hash) | test | web | test(web): improve coverage from 54.8% to 59.1% — add tests for notification handlers, config handlers, screenshot handlers, ICP handlers, metrics, cookie handlers |
| 2026-06-27 | $(@{FullHash=637f1e596f335dc9c9987b667035baab1907f75f; Hash=637f1e5; Date=2026-06-27; Author=ElaineRosa6; Type=fix; Scope=adapter; Subject=fix(adapter): ZoomEye protocol field mapping — use transport protocol field, remove invalid url→site mapping}.Hash) | fix | adapter | fix(adapter): ZoomEye protocol field mapping — use transport protocol field, remove invalid url→site mapping |
| 2026-06-27 | $(@{FullHash=88c6ca1be2c9d5c6f90ae5419354cecd51048bc3; Hash=88c6ca1; Date=2026-06-27; Author=ElaineRosa6; Type=fix; Scope=security/auth; Subject=fix: security audit 28/28 + map→struct refactoring (Phase 1-18)}.Hash) | fix | security/auth | fix: security audit 28/28 + map→struct refactoring (Phase 1-18) |
| 2026-06-27 | $(@{FullHash=ba4f8b70a3ff52b95db730142f3858d7053974b1; Hash=ba4f8b7; Date=2026-06-27; Author=ElaineRosa6; Type=docs; Scope=docs; Subject=docs: mark map→struct refactoring as complete, prohibit future redo}.Hash) | docs | docs | docs: mark map→struct refactoring as complete, prohibit future redo |
| 2026-06-27 | $(@{FullHash=d0e148f12dee1417972c8723040ac8b9dea07314; Hash=d0e148f; Date=2026-06-27; Author=ElaineRosa6; Type=chore; Scope=general; Subject=chore: remove runtime artifacts from tracking + update .gitignore}.Hash) | chore | general | chore: remove runtime artifacts from tracking + update .gitignore |
| 2026-06-28 | $(@{FullHash=332b02f074552dedc3a6ebd34f03f58465d2da7d; Hash=332b02f; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve middleware_security.go coverage — cspNonceFromContext 40→100%}.Hash) | test | web | test(web): improve middleware_security.go coverage — cspNonceFromContext 40→100% |
| 2026-06-28 | $(@{FullHash=59ffc718ad969b6e9174eb98be4e1f43ac86fa98; Hash=59ffc71; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve user_handlers.go coverage — handleGetUser 19→88%, handleDeleteUser 16→74%, handleListUsers 47→82%}.Hash) | test | web | test(web): improve user_handlers.go coverage — handleGetUser 19→88%, handleDeleteUser 16→74%, handleListUsers 47→82% |
| 2026-06-28 | $(@{FullHash=5acfefe650b129ba2c0021afc882708e4cac0f73; Hash=5acfefe; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve middleware_auth.go coverage — adminToken 31→100%}.Hash) | test | web | test(web): improve middleware_auth.go coverage — adminToken 31→100% |
| 2026-06-28 | $(@{FullHash=7ac2ae9e7f7b390aba09c875dc90f1efabf8218b; Hash=7ac2ae9; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): add validateTaskPayload and task handler tests for scheduler}.Hash) | test | web | test(web): add validateTaskPayload and task handler tests for scheduler |
| 2026-06-28 | $(@{FullHash=c6a0244569ec72662ae4ce58b816f69be36ff8cc; Hash=c6a0244; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve cdp_util.go coverage — resolveCDPURL 37→100%}.Hash) | test | web | test(web): improve cdp_util.go coverage — resolveCDPURL 37→100% |
| 2026-06-28 | $(@{FullHash=e257b3516a7edad97cf76de75c16b437565558d3; Hash=e257b35; Date=2026-06-28; Author=ElaineRosa6; Type=test; Scope=web; Subject=test(web): improve tamper_handlers.go coverage — tamperAllocatorFactory 33→100%}.Hash) | test | web | test(web): improve tamper_handlers.go coverage — tamperAllocatorFactory 33→100% |
| 2026-06-29 | $(@{FullHash=12c8169906f2ec4ecd8410b9d2cae4bb44446e05; Hash=12c8169; Date=2026-06-29; Author=ElaineRosa6; Type=chore; Scope=docs; Subject=chore: archive historical docs + clean runtime artifacts}.Hash) | chore | docs | chore: archive historical docs + clean runtime artifacts |
