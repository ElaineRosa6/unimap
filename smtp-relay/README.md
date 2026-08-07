# smtp-relay — UniMap Webhook → SMTP 邮件转发

独立容器：接收 UniMap 的 webhook 通知（`TaskNotification` JSON），转成邮件经
SMTP 发送到目标邮箱。**所有参数通过环境变量配置，不写死在代码里。**

```
UniMap 定时任务 ──webhook POST──▶ smtp-relay:8099/webhook ──SMTP──▶ 目标邮箱
```

## 配置

`docker-compose.yml` 里 `smtp-relay` 服务通过 `env_file: ./smtp-relay/.env` 注入配置。
把 `.env.example` 复制为 `.env` 并填写（**`.env` 已 gitignore，含授权码，勿提交**）：

| 变量 | 说明 | 默认 |
|---|---|---|
| `SMTP_HOST` | SMTP 服务器 | `smtp.qq.com` |
| `SMTP_PORT` | 端口（465=隐式 SSL） | `465` |
| `SMTP_USER` | 发件邮箱（必填） | — |
| `SMTP_PASSWORD` | SMTP 凭据（必填；QQ 用**授权码**，非登录密码） | — |
| `MAIL_TO` | 收件邮箱，逗号分隔（必填） | — |
| `MAIL_FROM_NAME` | 发件人显示名 | `UniMap` |
| `MAIL_SUBJECT_PREFIX` | 邮件标题前缀 | `[UniMap]` |
| `RELAY_LISTEN` | webhook 监听地址 | `0.0.0.0:8099` |
| `RELAY_TOKEN` | 可选：webhook 认证 token（设了则 POST 须带 `X-Relay-Token`） | 空 |

端口只绑宿主机 `127.0.0.1:8099`（loopback），不暴露公网；UniMap 容器经 compose
网络用 `http://smtp-relay:8099/webhook` 访问。

## 启动

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build smtp-relay
curl -s http://127.0.0.1:8099/health     # {"status":"ok"}
# 模拟一条通知验证发信（也可用 UniMap 的渠道测试 API）
curl -s -X POST http://127.0.0.1:8099/webhook \
  -H "Content-Type: application/json" \
  -d '{"task_id":"t1","task_name":"测试","task_type":"query","status":"success","result":"**查询完成**\n\n| 资产 | 标题 | 状态 |\n| --- | --- | --- |\n| 1.2.3.4:80 | 测试站点 | http / HTTP 200 |","duration_ms":5000,"timestamp":"2026-08-07T10:00:00+08:00"}'
```

## UniMap 侧绑定

1. 建 webhook 渠道（指向 relay，需 `allow_private_ip=true` 放行容器网络私有地址）：
   ```bash
   curl -s -X POST http://127.0.0.1:8448/api/v1/notifications/channels \
     -H "X-Admin-Token: <token>" -H "Origin: http://127.0.0.1:8448" \
     -H "Content-Type: application/json" \
     -d '{"id":"email_agent","type":"webhook","enabled":true,
          "webhook_url":"http://smtp-relay:8099/webhook","allow_private_ip":true}'
   ```
2. 测试渠道：`POST /api/v1/notifications/channels/test`，body `{"id":"email_agent","type":"webhook"}`。
3. 绑定任务：把任务 `notifications.channel_ids` 加上 `email_agent`（`POST /api/v1/scheduler/tasks/update` 全字段回填）。
