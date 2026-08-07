# 企业微信群机器人 Webhook API 及使用明细

> **文档来源**：基于企业微信官方开发者文档核对整理
> **官方文档地址**：https://developer.work.weixin.qq.com/document/path/91770
> **全局错误码地址**：https://developer.work.weixin.qq.com/document/path/96213
> **文档更新日期**：2025/08/07（官方文档最后更新）
> **本文档整理日期**：2026-08-06

---

## 目录

- [1. 概述](#1-概述)
- [2. Webhook 地址获取](#2-webhook-地址获取)
- [3. 发送消息接口](#3-发送消息接口)
- [4. 消息类型及数据格式](#4-消息类型及数据格式)
  - [4.1 文本消息 (text)](#41-文本消息-text)
  - [4.2 Markdown 消息 (markdown)](#42-markdown-消息-markdown)
  - [4.3 Markdown V2 消息 (markdown_v2)](#43-markdown-v2-消息-markdown_v2)
  - [4.4 图片消息 (image)](#44-图片消息-image)
  - [4.5 图文消息 (news)](#45-图文消息-news)
  - [4.6 文件消息 (file)](#46-文件消息-file)
  - [4.7 语音消息 (voice)](#47-语音消息-voice)
  - [4.8 模板卡片消息 (template_card)](#48-模板卡片消息-template_card)
    - [4.8.1 文本通知模板卡片 (text_notice)](#481-文本通知模板卡片-text_notice)
    - [4.8.2 图文展示模板卡片 (news_notice)](#482-图文展示模板卡片-news_notice)
- [5. 文件上传接口](#5-文件上传接口)
- [6. 消息发送频率限制](#6-消息发送频率限制)
- [7. 返回码说明](#7-返回码说明)
  - [7.1 消息发送返回格式](#71-消息发送返回格式)
  - [7.2 常用错误码速查](#72-常用错误码速查)
- [8. 安全注意事项](#8-安全注意事项)
- [9. 使用示例](#9-使用示例)
  - [9.1 curl 示例](#91-curl-示例)
  - [9.2 Python 示例](#92-python-示例)
  - [9.3 Node.js 示例](#93-nodejs-示例)
- [10. 常见问题](#10-常见问题)

---

## 1. 概述

企业微信群机器人（现官方名称为 **"消息推送"**，原称"群机器人"）提供了一种简单的 Webhook 方式，允许开发者通过 HTTP POST 请求向企业微信群组发送消息。

**核心特点**：

| 项目 | 说明 |
|------|------|
| 请求方式 | HTTP POST |
| 数据格式 | application/json |
| 认证方式 | Webhook URL 中的 key 参数（无需 access_token） |
| 支持的消息类型 | 文本、Markdown、Markdown V2、图片、图文、文件、语音、模板卡片（共 8 种） |
| 频率限制 | 每个机器人 20 条/分钟 |
| 适用场景 | 内部群消息推送（外部群不支持） |

---

## 2. Webhook 地址获取

Webhook 地址可在以下页面获取：
- 创建消息推送页面
- 创建完成页面
- 消息推送详情页面

**Webhook URL 格式**：

```
https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=693a91f6-7xxx-4bc4-97a0-0ec2sifa5aaa
```

其中 `key` 参数为该消息推送的唯一凭证。

> **重要安全提示**：务必保护 Webhook 地址不被泄露！不要将其提交到 GitHub、博客等可被公开查阅的地方，否则可能被用于发送垃圾消息。

---

## 3. 发送消息接口

### 接口说明

| 项目 | 说明 |
|------|------|
| 请求方式 | POST (HTTPS) |
| 请求地址 | `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=KEY` |
| Content-Type | application/json |
| 参数 | key（在 URL 中传递，为 Webhook 的唯一凭证） |

### 基本请求结构

所有消息请求均包含 `msgtype` 字段来指定消息类型，不同消息类型对应不同的消息体字段：

```json
{
    "msgtype": "text",
    "text": {
        "content": "消息内容"
    }
}
```

---

## 4. 消息类型及数据格式

当前支持 8 种消息类型：

| 消息类型 | msgtype 值 | 说明 |
|----------|-----------|------|
| 文本 | `text` | 纯文本消息，支持 @ 群成员 |
| Markdown | `markdown` | Markdown 格式消息，支持 @ 群成员 |
| Markdown V2 | `markdown_v2` | 增强版 Markdown，支持表格、图片等 |
| 图片 | `image` | Base64 编码的图片 |
| 图文 | `news` | 图文链接消息 |
| 文件 | `file` | 文件消息（需先上传获取 media_id） |
| 语音 | `voice` | 语音消息（需先上传获取 media_id） |
| 模板卡片 | `template_card` | 结构化卡片消息 |

### 4.1 文本消息 (text)

**请求示例**：

```json
{
    "msgtype": "text",
    "text": {
        "content": "广州今日天气：29度，大部分多云，降雨概率：60%",
        "mentioned_list": ["wangqing", "@all"],
        "mentioned_mobile_list": ["13800001111", "@all"]
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `text` |
| text.content | 是 | 文本内容，最长不超过 **2048 个字节**，必须是 UTF-8 编码 |
| text.mentioned_list | 否 | userid 列表，提醒群中的指定成员（@某个成员）。`@all` 表示提醒所有人。如果获取不到 userid，可使用 mentioned_mobile_list |
| text.mentioned_mobile_list | 否 | 手机号列表，提醒手机号对应的群成员（@某个成员）。`@all` 表示提醒所有人 |

> **@ 群成员语法**：text 和 markdown 类型消息支持在 content 中使用 `<@userid>` 扩展语法来 @ 群成员（markdown_v2 不支持此语法）。

### 4.2 Markdown 消息 (markdown)

**请求示例**：

```json
{
    "msgtype": "markdown",
    "markdown": {
        "content": "实时新增用户反馈<font color=\"warning\">132例</font>，请相关同事注意。\n>类型:<font color=\"comment\">用户反馈</font>\n>普通用户反馈:<font color=\"comment\">117例</font>\n>VIP用户反馈:<font color=\"comment\">15例</font>"
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `markdown` |
| markdown.content | 是 | Markdown 内容，最长不超过 **4096 个字节**，必须是 UTF-8 编码 |

**支持的 Markdown 语法子集**：

| 语法 | 示例 |
|------|------|
| 标题（1~6 级，# 与文字间要有空格） | `# 标题一` / `## 标题二` / `### 标题三` |
| 加粗 | `**bold**` |
| 链接 | `[这是一个链接](https://work.weixin.qq.com/api/doc)` |
| 行内代码段（暂不支持跨行） | `` `code` `` |
| 引用 | `> 引用文字` |
| 字体颜色（仅支持 3 种内置颜色） | 见下表 |

**内置字体颜色**：

| 颜色 | 语法 | 效果 |
|------|------|------|
| 绿色 | `<font color="info">绿色</font>` | 绿色文字 |
| 灰色 | `<font color="comment">灰色</font>` | 灰色文字 |
| 橙红色 | `<font color="warning">橙红色</font>` | 橙红色文字 |

### 4.3 Markdown V2 消息 (markdown_v2)

**请求示例**：

```json
{
    "msgtype": "markdown_v2",
    "markdown_v2": {
        "content": "# 一、标题\n## 二级标题\n### 三级标题\n# 二、字体\n*斜体*\n\n**加粗**\n# 三、列表 \n- 无序列表 1 \n- 无序列表 2\n - 无序列表 2.1\n - 无序列表 2.2\n1. 有序列表 1\n2. 有序列表 2\n# 四、引用\n> 一级引用\n>>二级引用\n>>>三级引用\n# 五、链接\n[这是一个链接](https:work.weixin.qq.com\/api\/doc)\n![](https://res.mail.qq.com/node/ww/wwopenmng/images/independent/doc/test_pic_msg1.png)\n# 六、分割线\n\n---\n# 七、代码\n`这是行内代码`\n```\n这是独立代码块\n```\n\n# 八、表格\n| 姓名 | 文化衫尺寸 | 收货地址 |\n| :----- | :----: | -------: |\n| 张三 | S | 广州 |\n| 李四 | L | 深圳 |\n"
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `markdown_v2` |
| markdown_v2.content | 是 | Markdown V2 内容，最长不超过 **4096 个字节**，必须是 UTF-8 编码 |

**markdown_v2 与 markdown 的区别**：

| 对比项 | markdown | markdown_v2 |
|--------|----------|-------------|
| 字体颜色 | 支持（info/comment/warning） | **不支持** |
| @ 群成员 | 支持 | **不支持** |
| 斜体 | 不支持 | 支持 `*斜体*` |
| 有序/无序列表 | 不支持 | 支持 |
| 多级引用 | 不支持 | 支持（多级嵌套） |
| 图片 | 不支持 | 支持 `![](url)` |
| 分割线 | 不支持 | 支持 `---` |
| 代码块 | 仅行内代码 | 行内代码 + 独立代码块 |
| 表格 | 不支持 | 支持 |

> **兼容性提示**：markdown_v2 消息在客户端 4.1.36 版本以下（安卓端为 4.1.38 以下）表现为纯文本，建议使用最新客户端版本体验。

**markdown_v2 支持的完整语法列表**：

```
# 标题（1~6 级）
*斜体*
**加粗**
- 无序列表
1. 有序列表
> 引用（支持多级嵌套）
[链接](url)
![图片](url)
---
`行内代码`
```
独立代码块
```
| 表格 | 支持 | 对齐 |
```

### 4.4 图片消息 (image)

**请求示例**：

```json
{
    "msgtype": "image",
    "image": {
        "base64": "DATA",
        "md5": "MD5"
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `image` |
| image.base64 | 是 | 图片内容的 Base64 编码 |
| image.md5 | 是 | 图片内容（Base64 编码前）的 MD5 值 |

**图片限制**：

| 限制项 | 值 |
|--------|-----|
| 最大大小 | **2 MB**（Base64 编码前） |
| 支持格式 | JPG、PNG |

### 4.5 图文消息 (news)

**请求示例**：

```json
{
    "msgtype": "news",
    "news": {
        "articles": [
            {
                "title": "中秋节礼品领取",
                "description": "今年中秋节公司有豪礼相送",
                "url": "www.qq.com",
                "picurl": "https://res.mail.qq.com/node/ww/wwopenmng/images/independent/doc/test_pic_msg1.png"
            }
        ]
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `news` |
| news.articles | 是 | 图文消息数组，支持 **1 到 8 条**图文 |
| articles.title | 是 | 标题，不超过 **128 个字节**，超过自动截断 |
| articles.description | 否 | 描述，不超过 **512 个字节**，超过自动截断 |
| articles.url | 是 | 点击后跳转的链接 |
| articles.picurl | 否 | 图文消息的图片链接，支持 JPG、PNG 格式。推荐大图 1068×455，小图 150×150 |

### 4.6 文件消息 (file)

**请求示例**：

```json
{
    "msgtype": "file",
    "file": {
        "media_id": "3a8asd892asd8asd"
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `file` |
| file.media_id | 是 | 文件 ID，通过 [文件上传接口](#5-文件上传接口) 获取 |

### 4.7 语音消息 (voice)

**请求示例**：

```json
{
    "msgtype": "voice",
    "voice": {
        "media_id": "MEDIA_ID"
    }
}
```

**参数说明**：

| 参数 | 是否必填 | 说明 |
|------|----------|------|
| msgtype | 是 | 消息类型，固定为 `voice` |
| voice.media_id | 是 | 语音文件 ID，通过 [文件上传接口](#5-文件上传接口) 获取 |

### 4.8 模板卡片消息 (template_card)

模板卡片支持两种类型：**文本通知模板卡片**（`text_notice`）和 **图文展示模板卡片**（`news_notice`）。

#### 4.8.1 文本通知模板卡片 (text_notice)

**请求示例**：

```json
{
    "msgtype": "template_card",
    "template_card": {
        "card_type": "text_notice",
        "source": {
            "icon_url": "https://wework.qpic.cn/wwpic/252813_jOfDHtcISzuodLa_1629280209/0",
            "desc": "企业微信",
            "desc_color": 0
        },
        "main_title": {
            "title": "欢迎使用企业微信",
            "desc": "您的好友正在邀请您加入企业微信"
        },
        "emphasis_content": {
            "title": "100",
            "desc": "数据含义"
        },
        "quote_area": {
            "type": 1,
            "url": "https://work.weixin.qq.com/?from=openApi",
            "appid": "APPID",
            "pagepath": "PAGEPATH",
            "title": "引用文本标题",
            "quote_text": "Jack：企业微信真的很好用~\nBalian：超级好的一款软件！"
        },
        "sub_title_text": "下载企业微信还能抢红包！",
        "horizontal_content_list": [
            {
                "keyname": "邀请人",
                "value": "张三"
            },
            {
                "keyname": "企微官网",
                "value": "点击访问",
                "type": 1,
                "url": "https://work.weixin.qq.com/?from=openApi"
            },
            {
                "keyname": "企微下载",
                "value": "企业微信.apk",
                "type": 2,
                "media_id": "MEDIAID"
            }
        ],
        "jump_list": [
            {
                "type": 1,
                "url": "https://work.weixin.qq.com/?from=openApi",
                "title": "企业微信官网"
            },
            {
                "type": 2,
                "appid": "APPID",
                "pagepath": "PAGEPATH",
                "title": "跳转小程序"
            }
        ],
        "card_action": {
            "type": 1,
            "url": "https://work.weixin.qq.com/?from=openApi",
            "appid": "APPID",
            "pagepath": "PAGEPATH"
        }
    }
}
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| msgtype | String | 是 | 固定为 `template_card` |
| template_card | Object | 是 | 模板卡片参数 |
| template_card.card_type | String | 是 | 固定为 `text_notice` |
| template_card.source | Object | 否 | 卡片来源样式信息 |
| source.icon_url | String | 否 | 来源图片 URL |
| source.desc | String | 否 | 来源图片描述，建议不超过 13 个字 |
| source.desc_color | Int | 否 | 来源文字颜色：0=灰色（默认），1=黑色，2=红色，3=绿色 |
| template_card.main_title | Object | 是 | 主要内容（一级标题 + 标题辅助信息） |
| main_title.title | String | 否 | 一级标题，建议不超过 26 个字 |
| main_title.desc | String | 否 | 标题辅助信息，建议不超过 30 个字 |
| template_card.emphasis_content | Object | 否 | 关键数据样式 |
| emphasis_content.title | String | 否 | 关键数据内容，建议不超过 10 个字 |
| emphasis_content.desc | String | 否 | 关键数据描述，建议不超过 15 个字 |
| template_card.quote_area | Object | 否 | 引用文献样式（建议不与关键数据共用） |
| quote_area.type | Int | 否 | 点击事件：0=无，1=跳转 URL，2=跳转小程序 |
| quote_area.url | String | 否 | 跳转 URL（type=1 时必填） |
| quote_area.appid | String | 否 | 跳转小程序 appid（type=2 时必填） |
| quote_area.pagepath | String | 否 | 跳转小程序 pagepath（type=2 时选填） |
| quote_area.title | String | 否 | 引用文献标题 |
| quote_area.quote_text | String | 否 | 引用文献文案 |
| template_card.sub_title_text | String | 否 | 二级普通文本，建议不超过 112 个字 |
| template_card.horizontal_content_list | Object[] | 否 | 二级标题+文本列表，长度不超过 6 |
| horizontal_content_list.type | Int | 否 | 内容类型：1=URL，2=文件附件，3=跳转成员详情 |
| horizontal_content_list.keyname | String | 是 | 二级标题，建议不超过 5 个字 |
| horizontal_content_list.value | String | 否 | 二级文本（type=2 时为文件名），建议不超过 26 个字 |
| horizontal_content_list.url | String | 否 | 链接 URL（type=1 时必填） |
| horizontal_content_list.media_id | String | 否 | 附件 media_id（type=2 时必填） |
| horizontal_content_list.userid | String | 否 | 成员 userid（type=3 时必填） |
| template_card.jump_list | Object[] | 否 | 跳转指引列表，长度不超过 3 |
| jump_list.type | Int | 否 | 跳转类型：0=非链接，1=跳转 URL，2=跳转小程序 |
| jump_list.title | String | 是 | 跳转文案，建议不超过 13 个字 |
| jump_list.url | String | 否 | 跳转 URL（type=1 时必填） |
| jump_list.appid | String | 否 | 跳转小程序 appid（type=2 时必填） |
| jump_list.pagepath | String | 否 | 跳转小程序 pagepath（type=2 时选填） |
| template_card.card_action | Object | 是 | 整体卡片点击跳转事件（text_notice 中必填） |
| card_action.type | Int | 是 | 跳转类型：1=跳转 URL，2=打开小程序 |
| card_action.url | String | 否 | 跳转 URL（type=1 时必填） |
| card_action.appid | String | 否 | 跳转小程序 appid（type=2 时必填） |
| card_action.pagepath | String | 否 | 跳转小程序 pagepath（type=2 时选填） |

> **注意**：`main_title.title` 和 `sub_title_text` 必须有一项填写。

#### 4.8.2 图文展示模板卡片 (news_notice)

**请求示例**：

```json
{
    "msgtype": "template_card",
    "template_card": {
        "card_type": "news_notice",
        "source": {
            "icon_url": "https://wework.qpic.cn/wwpic/252813_jOfDHtcISzuodLa_1629280209/0",
            "desc": "企业微信",
            "desc_color": 0
        },
        "main_title": {
            "title": "欢迎使用企业微信",
            "desc": "您的好友正在邀请您加入企业微信"
        },
        "card_image": {
            "url": "https://wework.qpic.cn/wwpic/354393_4zpkKXd7SrGMvfg_1629280616/0",
            "aspect_ratio": 2.25
        },
        "image_text_area": {
            "type": 1,
            "url": "https://work.weixin.qq.com",
            "title": "欢迎使用企业微信",
            "desc": "您的好友正在邀请您加入企业微信",
            "image_url": "https://wework.qpic.cn/wwpic/354393_4zpkKXd7SrGMvfg_1629280616/0"
        },
        "quote_area": {
            "type": 1,
            "url": "https://work.weixin.qq.com/?from=openApi",
            "title": "引用文本标题",
            "quote_text": "Jack：企业微信真的很好用~"
        },
        "vertical_content_list": [
            {
                "title": "惊喜红包等你来拿",
                "desc": "下载企业微信还能抢红包！"
            }
        ],
        "horizontal_content_list": [
            {
                "keyname": "邀请人",
                "value": "张三"
            },
            {
                "keyname": "企微官网",
                "value": "点击访问",
                "type": 1,
                "url": "https://work.weixin.qq.com/?from=openApi"
            }
        ],
        "jump_list": [
            {
                "type": 1,
                "url": "https://work.weixin.qq.com/?from=openApi",
                "title": "企业微信官网"
            }
        ],
        "card_action": {
            "type": 1,
            "url": "https://work.weixin.qq.com/?from=openApi"
        }
    }
}
```

**news_notice 与 text_notice 的差异参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| template_card.card_image | Object | 是 | 图片样式 |
| card_image.url | String | 是 | 图片 URL |
| card_image.aspect_ratio | Float | 否 | 图片宽高比，范围 1.3~2.25，默认 1.3 |
| template_card.image_text_area | Object | 否 | 左图右文样式 |
| image_text_area.type | Int | 否 | 点击事件：0=无，1=跳转 URL，2=跳转小程序 |
| image_text_area.url | String | 否 | 跳转 URL（type=1 时必填） |
| image_text_area.appid | String | 否 | 跳转小程序 appid（type=2 时必填） |
| image_text_area.pagepath | String | 否 | 跳转小程序 pagepath（type=2 时选填） |
| image_text_area.title | String | 否 | 左图右文样式标题 |
| image_text_area.desc | String | 否 | 左图右文样式描述 |
| image_text_area.image_url | String | 是 | 左图右文样式图片 URL |
| template_card.vertical_content_list | Object[] | 否 | 卡片二级垂直内容，长度不超过 4 |
| vertical_content_list.title | String | 是 | 卡片二级标题，建议不超过 26 个字 |
| vertical_content_list.desc | String | 否 | 二级普通文本，建议不超过 112 个字 |

> news_notice 中 `main_title.title` 为**必填**，`card_image` 也为**必填**。`card_action` 同样必填，type 取值范围为 [1, 2]。

---

## 5. 文件上传接口

用于上传文件或语音素材，获取 `media_id` 后用于发送文件消息和语音消息。

### 接口说明

| 项目 | 说明 |
|------|------|
| 请求方式 | POST (HTTPS) |
| 请求地址 | `https://qyapi.weixin.qq.com/cgi-bin/webhook/upload_media?key=KEY&type=TYPE` |
| Content-Type | multipart/form-data |
| 文件标识名 | `media` |

### 参数说明

| 参数 | 必填 | 说明 |
|------|------|------|
| key | 是 | 调用接口凭证，即 Webhook URL 中的 key 参数 |
| type | 是 | 文件类型：`voice`（语音）或 `file`（普通文件） |

### 请求示例

```http
POST https://qyapi.weixin.qq.com/cgi-bin/webhook/upload_media?key=693a91f6-7xxx-4bc4-97a0-0ec2sifa5aaa&type=file HTTP/1.1
Content-Type: multipart/form-data; boundary=-------------------------acebdf13572468
Content-Length: 220

---------------------------acebdf13572468
Content-Disposition: form-data; name="media";filename="wework.txt"; filelength=6
Content-Type: application/octet-stream

mytext
---------------------------acebdf13572468--
```

### 返回数据

```json
{
    "errcode": 0,
    "errmsg": "ok",
    "type": "file",
    "media_id": "1G6nrLmr5EC3MMb_-zK1dDdzmd0p7cNliYu9V5w7o8K0",
    "created_at": "1380000000"
}
```

### 返回参数说明

| 参数 | 说明 |
|------|------|
| type | 文件类型：`voice` 或 `file` |
| media_id | 媒体文件上传后获取的唯一标识，**3 天内有效** |
| created_at | 媒体文件上传时间戳 |

### 文件大小限制

| 类型 | 大小限制 | 格式限制 | 其他限制 |
|------|----------|----------|----------|
| 普通文件 (file) | ≤ 20 MB | - | 所有类型文件大小需 > 5 字节 |
| 语音 (voice) | ≤ 2 MB | 仅支持 AMR 格式 | 播放长度 ≤ 60 秒 |

> **注意**：media_id 仅 3 天内有效，且只能被对应上传文件的消息推送（即同一个 Webhook 的 key）使用。

---

## 6. 消息发送频率限制

| 限制项 | 值 |
|--------|-----|
| 频率限制 | 每个机器人（Webhook）**不超过 20 条/分钟** |
| 限制维度 | 针对同一个机器人的全局限制，无论消息发送是否成功都计数 |

> 超过频率限制时，接口将返回错误码 `45009`（接口调用超过限制）。

---

## 7. 返回码说明

### 7.1 消息发送返回格式

```json
{
    "errcode": 0,
    "errmsg": "ok"
}
```

| 字段 | 说明 |
|------|------|
| errcode | 返回码，0 表示成功 |
| errmsg | 对返回码的文本描述 |

> **重要提示**：开发者应根据 `errcode` 判断出错情况，不应依赖 `errmsg` 来匹配，因为 `errmsg` 可能会调整。如果请求参数不符合 JSON 规范（如类型不匹配、格式有问题），errmsg 中可能包含 "Warning: wrong json format."。

### 7.2 常用错误码速查

以下是群机器人 Webhook 场景中最常遇到的错误码：

| 错误码 | 说明 | 排查方法 |
|--------|------|----------|
| **0** | 请求成功 | 接口调用成功 |
| **-1** | 系统繁忙 | 服务器暂不可用，建议稍候重试，重试次数不超过 3 次 |
| **40001** | 不合法的 secret 参数 | 检查 key 是否正确 |
| **40003** | 无效的 UserID | 检查 mentioned_list 中的 userid |
| **40004** | 不合法的媒体文件类型 | 检查文件格式 |
| **40006** | 不合法的文件大小 | 检查文件大小是否在限制内 |
| **40007** | 不合法的 media_id 参数 | media_id 过期或不属于当前机器人 |
| **40008** | 不合法的 msgtype 参数 | 检查 msgtype 取值是否正确 |
| **40013** | 不合法的 CorpID | 检查 CorpID |
| **40033** | 不合法的请求字符 | 不能包含 `\uxxxx` 格式的字符 |
| **40035** | 不合法的参数 | 检查请求参数 |
| **40063** | 参数为空 | 检查必填参数是否为空 |
| **40093** | secret 不合法 | 可能用了别的企业的 secret |
| **41006** | 缺少 media_id 参数 | media_id 为必填参数 |
| **42001** | access_token 已过期 | access_token 有时效性，需重新获取 |
| **44001** | 多媒体文件为空 | 检查上传格式 |
| **44004** | 文本消息 content 参数为空 | content 为必填参数 |
| **45001** | 多媒体文件大小超过限制 | 图片≤5M，音频≤5M，文件≤20M |
| **45002** | 消息内容大小超过限制 | 检查消息内容长度 |
| **45007** | 语音播放时间超过限制 | 语音不能超过 60 秒 |
| **45008** | 图文消息文章数量不符合限制 | 不能超过 8 条 |
| **45009** | 接口调用超过限制 | 超过 20 条/分钟频率限制 |
| **45033** | 接口并发调用超过限制 | 降低并发 |
| **45034** | url 必须有协议头 | 在 url 前加 http:// 或 https:// |
| **46004** | 指定的用户不存在 | 检查 userid 是否正确 |
| **48002** | API 接口无权限调用 | 检查权限 |
| **48005** | API 接口已废弃 | 接口不再支持 |
| **50002** | 成员不在权限范围 | 检查应用或管理组权限范围 |
| **50003** | 应用已禁用 | 检查应用状态 |

---

## 8. 安全注意事项

1. **保护 Webhook 地址**：Webhook 地址等同于密钥，泄露后任何人都可以向群组发送消息。切勿将其提交到代码仓库（GitHub 等）、博客等公开位置。

2. **内容必须 UTF-8 编码**：所有消息内容必须是 UTF-8 编码，否则可能导致发送失败或乱码。

3. **JSON 格式必须合法**：请求参数必须符合 JSON 规范（类型匹配、格式正确），否则企业微信解析时可能截断参数。

4. **media_id 时效性**：通过文件上传接口获取的 media_id 仅 3 天内有效，且仅可用于同一个 Webhook key 的消息推送。

5. **频率限制是全局的**：20 条/分钟的限制针对同一个机器人的所有消息（含失败消息），建议做好限流和重试机制。

6. **仅支持内部群**：企业微信群机器人仅支持内部群，外部群（群名后缀有【外部】标签的社群）不能使用。上下游群里的企业微信群机器人也可以支持。

---

## 9. 使用示例

### 9.1 curl 示例

**发送文本消息**：

```bash
curl 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY' \
   -H 'Content-Type: application/json' \
   -d '
   {
        "msgtype": "text",
        "text": {
            "content": "hello world"
        }
   }'
```

**发送 Markdown 消息**：

```bash
curl 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY' \
   -H 'Content-Type: application/json' \
   -d '
   {
        "msgtype": "markdown",
        "markdown": {
            "content": "# 项目周报\n> **本周完成**：API文档整理\n> **下周计划**：接口联调"
        }
   }'
```

**上传文件**：

```bash
curl 'https://qyapi.weixin.qq.com/cgi-bin/webhook/upload_media?key=YOUR_KEY&type=file' \
   -F 'media=@/path/to/wework.txt'
```

### 9.2 Python 示例

```python
import requests
import json
import base64
import hashlib

WEBHOOK_URL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"

def send_text(content, mentioned_list=None, mentioned_mobile_list=None):
    """发送文本消息"""
    data = {
        "msgtype": "text",
        "text": {
            "content": content,
        }
    }
    if mentioned_list:
        data["text"]["mentioned_list"] = mentioned_list
    if mentioned_mobile_list:
        data["text"]["mentioned_mobile_list"] = mentioned_mobile_list
    resp = requests.post(WEBHOOK_URL, json=data)
    return resp.json()

def send_markdown(content):
    """发送 Markdown 消息"""
    data = {
        "msgtype": "markdown",
        "markdown": {
            "content": content
        }
    }
    resp = requests.post(WEBHOOK_URL, json=data)
    return resp.json()

def send_image(image_path):
    """发送图片消息"""
    with open(image_path, "rb") as f:
        image_data = f.read()
    base64_data = base64.b64encode(image_data).decode("utf-8")
    md5 = hashlib.md5(image_data).hexdigest()
    data = {
        "msgtype": "image",
        "image": {
            "base64": base64_data,
            "md5": md5
        }
    }
    resp = requests.post(WEBHOOK_URL, json=data)
    return resp.json()

def upload_file(file_path):
    """上传文件获取 media_id"""
    upload_url = WEBHOOK_URL.replace("/send?", "/upload_media?") + "&type=file"
    with open(file_path, "rb") as f:
        resp = requests.post(upload_url, files={"media": f})
    return resp.json()

def send_file(file_path):
    """发送文件消息（先上传再发送）"""
    result = upload_file(file_path)
    if result.get("errcode") != 0:
        return result
    media_id = result["media_id"]
    data = {
        "msgtype": "file",
        "file": {
            "media_id": media_id
        }
    }
    resp = requests.post(WEBHOOK_URL, json=data)
    return resp.json()

# 使用示例
if __name__ == "__main__":
    # 发送文本消息（@所有人）
    send_text("每日构建完成", mentioned_list=["@all"])

    # 发送 Markdown 消息
    send_markdown(
        "# 构建报告\n"
        "> 状态：<font color=\"info\">成功</font>\n"
        "> 耗时：**3分28秒**\n"
        "> 详情：[查看日志](https://example.com/log)"
    )

    # 发送图片消息
    send_image("/path/to/chart.png")

    # 发送文件消息
    send_file("/path/to/report.pdf")
```

### 9.3 Node.js 示例

```javascript
const axios = require('axios');
const fs = require('fs');
const crypto = require('crypto');

const WEBHOOK_URL = 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY';

async function sendText(content, mentionedList = [], mentionedMobileList = []) {
    const text = { content };
    if (mentionedList.length) text.mentioned_list = mentionedList;
    if (mentionedMobileList.length) text.mentioned_mobile_list = mentionedMobileList;
    const { data } = await axios.post(WEBHOOK_URL, { msgtype: 'text', text });
    return data;
}

async function sendMarkdown(content) {
    const { data } = await axios.post(WEBHOOK_URL, {
        msgtype: 'markdown',
        markdown: { content }
    });
    return data;
}

async function sendImage(imagePath) {
    const imageBuffer = fs.readFileSync(imagePath);
    const base64 = imageBuffer.toString('base64');
    const md5 = crypto.createHash('md5').update(imageBuffer).digest('hex');
    const { data } = await axios.post(WEBHOOK_URL, {
        msgtype: 'image',
        image: { base64, md5 }
    });
    return data;
}

async function uploadFile(filePath) {
    const formData = new FormData();
    const fileBuffer = fs.readFileSync(filePath);
    formData.append('media', new Blob([fileBuffer]), 'upload.txt');
    const uploadUrl = WEBHOOK_URL.replace('/send?', '/upload_media?') + '&type=file';
    const { data } = await axios.post(uploadUrl, formData);
    return data;
}

async function sendFile(filePath) {
    const uploadResult = await uploadFile(filePath);
    if (uploadResult.errcode !== 0) return uploadResult;
    const { data } = await axios.post(WEBHOOK_URL, {
        msgtype: 'file',
        file: { media_id: uploadResult.media_id }
    });
    return data;
}

// 使用示例
(async () => {
    await sendText('部署完成', ['@all']);
    await sendMarkdown('# 部署报告\n> 状态：<font color="info">成功</font>');
    await sendImage('/path/to/screenshot.png');
    await sendFile('/path/to/report.csv');
})();
```

---

## 10. 常见问题

### Q1: Webhook 地址泄露了怎么办？

在群聊设置中删除该机器人并重新创建，旧 Webhook 地址将立即失效，新机器人将获得新的 Webhook 地址。

### Q2: 发送消息返回 errcode: 45009 怎么办？

触发频率限制（20 条/分钟）。建议：
- 在发送端添加限流/队列机制
- 将多条消息合并为一条（如用 Markdown 整合）
- 使用多个机器人分散流量

### Q3: media_id 提示不合法（errcode: 40007）？

可能原因：
- media_id 已过期（仅 3 天有效）
- media_id 来自其他机器人的上传
- 上传时文件不合法

### Q4: markdown_v2 消息在手机上显示为纯文本？

markdown_v2 需要客户端版本 4.1.36 以上（安卓端 4.1.38 以上）才能正常渲染。建议升级到最新版本。

### Q5: 外部群能否使用群机器人？

不能。企业微信群机器人仅支持内部群。外部群（群名后缀有【外部】标签）不支持定时发送消息。上下游群里的企业微信群机器人可以支持。

### Q6: 能否通过 API 创建或删除群机器人？

Webhook 方式的群机器人不支持通过 API 创建/删除。机器人需要在企业微信客户端中手动添加到群聊。如需程序化管理，请使用 [应用推送消息](https://developer.work.weixin.qq.com/document/path/90248) 或 [应用发送消息到群聊会话](https://developer.work.weixin.qq.com/document/path/90245) 接口。

### Q7: 图片 Base64 编码后总大小超过限制怎么办？

2 MB 限制是指** Base64 编码前**的图片大小。Base64 编码后体积约为原文件的 1.33 倍。建议在上传前先压缩图片。

### Q8: 是否支持发送 @ 部分人的消息？

支持。在 text 消息中使用 `mentioned_list`（userid 列表）或 `mentioned_mobile_list`（手机号列表）。`@all` 表示 @ 所有人。markdown 消息也支持在 content 中使用 `<@userid>` 语法。markdown_v2 不支持 @ 功能。

---

## 附录：消息类型速查表

| 消息类型 | msgtype | 内容限制 | 支持 @ | 需上传 | 特点 |
|----------|---------|----------|--------|--------|------|
| 文本 | text | 2048 字节 | 支持 | 否 | 最简单，支持 @ 群成员 |
| Markdown | markdown | 4096 字节 | 支持 | 否 | 支持标题/加粗/链接/引用/字体颜色 |
| Markdown V2 | markdown_v2 | 4096 字节 | 不支持 | 否 | 支持表格/图片/列表/代码块 |
| 图片 | image | 2 MB | 不支持 | 否 | Base64 + MD5 |
| 图文 | news | 1-8 条 | 不支持 | 否 | 标题+描述+链接+缩略图 |
| 文件 | file | 20 MB | 不支持 | 是 | 需先上传获取 media_id |
| 语音 | voice | 2 MB / 60s | 不支持 | 是 | 仅 AMR 格式 |
| 模板卡片 | template_card | - | 不支持 | 可能 | 结构化卡片，2 种子类型 |

---

> **免责声明**：本文档基于企业微信官方开发者文档整理核对。如有差异，以[官方文档](https://developer.work.weixin.qq.com/document/path/91770)为准。
