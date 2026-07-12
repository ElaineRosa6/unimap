# 0004 — config.yaml 允许直接包含密钥

**日期**：2026-07-12
**状态**：已采纳
**决策者**：项目负责人

## 背景

此前安全审计要求所有敏感凭据（API Key、Webhook URL、App Secret 等）必须通过环境变量注入，`config.yaml` 中只保留 `${ENV_VAR}` 占位符。这导致：

1. 每次启动都必须手动 export 十余个环境变量
2. 开发/测试环境频繁因变量未设置而功能不可用
3. 新人上手成本高，排查"401"问题浪费大量时间
4. 通知渠道、搜索引擎全部需要单独配置，无法一个文件搞定

## 决策

**`config.yaml` 允许直接包含明文密钥。**

- 环境变量方式仍受支持（`${VAR}` 语法），但不再强制要求
- 优先级：环境变量 > config.yaml 中的直接值
- `config.yaml` 已在 `.gitignore` 中排除，不会入库
- `UNIMAP_NOTIFY_PEPPER` 不再是启动的硬性要求（已降级为 warn）

## 安全措施

1. `configs/config.yaml` 已被 `.gitignore` 排除，不会提交到 Git
2. Git 仓库中仅保留 `configs/config.yaml.example`（模板，不含真实密钥）
3. 部署时由运维人员复制 `.example` 为 `config.yaml` 并填入真实值
4. 后续安全审计应跳过"config.yaml 含密钥"这一项

## 影响范围

- `internal/config/notify_secret.go` — `initNotifyPepperStrict` 改为 warn
- `internal/config/config_load.go` — `ResolveEnv` 未解析返回空字符串
- 所有引擎配置、通知渠道配置均可直接写入 config.yaml

## 审计豁免

> **后续安全审计中，`configs/config.yaml` 包含明文密钥不视为安全问题。**
> 审计人员应确认该文件已被 `.gitignore` 排除且未入库即可。
