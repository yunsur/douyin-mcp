# 抖音 MCP HTTP API 文档

## 概述

该项目同时提供 **MCP (Model Context Protocol)** 服务和 **标准 HTTP REST API**。本文档描述 HTTP API 的使用方法。

**Base URL**: `http://localhost:18080`

**注意**: 以下响应示例仅展示主要字段结构，完整的字段信息请通过实际 API 调用查看（抖音原始返回会原样透传）。

## 访问鉴权（可选）

鉴权默认关闭。设置环境变量 `AUTH_TOKEN` 或启动参数 `-token` 后，所有 `/api/v1/*` 和 `/mcp` 接口都需要携带 Bearer Token；`/health` 健康检查和 CORS `OPTIONS` 预检请求保持公开。非空的 `-token` 优先于 `AUTH_TOKEN`，留空则读取 `AUTH_TOKEN`。

```bash
# 启用鉴权
AUTH_TOKEN=your-secret-token ./douyin-mcp -port :18080

# 调用 HTTP API
curl http://localhost:18080/api/v1/login/status \
  -H "Authorization: Bearer your-secret-token"
```

Token 缺失或无效时，接口返回 HTTP `401 Unauthorized`。命令行参数可能被进程列表看到，部署环境优先使用 `AUTH_TOKEN`。

## 通用响应格式

所有 HTTP API 响应都使用统一的 JSON 格式：

### 成功响应
```json
{
  "success": true,
  "data": {},
  "message": "操作成功消息"
}
```

### 错误响应
```json
{
  "error": "错误消息",
  "code": "ERROR_CODE",
  "details": "详细错误信息"
}
```

> 抖音接口的业务错误码（如 `status_code != 0`）会作为 `error` 透传，含义与抖音网页端一致。

### 状态码约定

| 状态码 | 场景 | `code` |
| --- | --- | --- |
| 200 | 成功 | — |
| 400 | 请求参数错误（JSON 解析失败、必填缺失、媒体类型不支持、房间号为空等） | `INVALID_REQUEST` / `MISSING_KEYWORD` 等 |
| 401 | 启用鉴权后 Token 缺失或无效 | `UNAUTHORIZED` |
| 404 | 路径不存在 | `NOT_FOUND` |
| 405 | 路径存在但 HTTP 方法不匹配 | `METHOD_NOT_ALLOWED` |
| 500 | 上游失败或服务内部错误 | 各接口自有 code（如 `LIVE_LISTEN_FAILED`） |

所有 4xx/5xx 都是上面的错误信封，不会返回空响应体。

## 环境变量

| 变量 | 说明 |
| --- | --- |
| `DOUYIN_COOKIES` / `DY_COOKIES` | Cookie 字符串（优先级高于文件） |
| `DOUYIN_COOKIES_FILE` | cookie 文件路径，默认 `./cookies.txt` |
| `DOUYIN_TICKET` / `DY_TICKET` | bd-ticket-guard ticket（写接口 / 创作者发布需要） |
| `DOUYIN_TS_SIGN` / `DY_TS_SIGN` | bd-ticket-guard ts_sign |
| `DOUYIN_CLIENT_CERT` / `DY_CLIENT_CERT` | bd-ticket-guard client cert |
| `DOUYIN_PRIVATE_KEY` / `DY_PRIVATE_KEY` | bd-ticket-guard EC 私钥 |
| `DOUYIN_DTRAIT_BLOB` / `DY_DTRAIT_BLOB` | x-tt-session-dtrait 设备特征 blob（创作者发布必需） |
| `DOUYIN_PROXY` / `DY_PROXY` | 代理，如 `http://127.0.0.1:7890` |
| `DOUYIN_PORT` | 监听端口，默认 `:18080` |
| `AUTH_TOKEN` | 静态 Bearer Token 鉴权，留空关闭 |
| `DY_FP_*` | 指纹档案覆盖（UA / 屏幕尺寸 / CPU 核数等），需与 cookie 所属浏览器一致 |
| `DY_HTTP_PROFILE` | TLS 指纹档位，默认 `chrome_150` |
| `DY_NODE` | `__ac_signature` 求解用的 Node 路径（留空则找 PATH / mise） |

## API 端点一览

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/v1/login/status` | 登录状态 |
| GET | `/api/v1/login/qrcode` | 获取登录二维码 |
| POST | `/api/v1/login/qrcode/check` | 轮询扫码状态 |
| POST | `/api/v1/login/phone/code` | 发送短信验证码 |
| POST | `/api/v1/login/phone` | 手机号 + 验证码登录 |
| DELETE | `/api/v1/login/cookies` | 删除 cookie，重置登录 |
| POST | `/api/v1/user/info` | 用户资料 |
| POST | `/api/v1/user/posts` | 用户作品（单页） |
| POST | `/api/v1/user/posts/all` | 用户全部作品 |
| POST | `/api/v1/video/detail` | 作品详情 |
| POST | `/api/v1/video/comments` | 作品评论（单页） |
| POST | `/api/v1/video/comments/all` | 作品全部评论 |
| POST | `/api/v1/video/comments/replies` | 评论回复 |
| GET/POST | `/api/v1/search/videos` | 搜索视频 |
| GET/POST | `/api/v1/search/users` | 搜索用户 |
| GET/POST | `/api/v1/search/lives` | 搜索直播 |
| POST | `/api/v1/search/suggest` | 搜索联想建议 |
| GET | `/api/v1/search/hot` | 热搜榜 |
| GET/POST | `/api/v1/search/challenges` | 搜索话题/挑战 |
| POST | `/api/v1/user/followers` | 粉丝列表 |
| POST | `/api/v1/user/following` | 关注列表 |
| POST | `/api/v1/user/followers/all` | 粉丝列表（全部） |
| POST | `/api/v1/user/following/all` | 关注列表（全部） |
| POST | `/api/v1/notices` | 消息通知 |
| POST | `/api/v1/notices/all` | 消息通知（全部） |
| GET | `/api/v1/notices/count` | 通知未读数（各分区） |
| POST | `/api/v1/notices/detail` | 单条通知详情 |
| POST | `/api/v1/notices/delete` | 删除通知 |
| POST | `/api/v1/user/favorites` | 喜欢（点赞过的作品） |
| POST | `/api/v1/user/history` | 观看历史 |
| POST | `/api/v1/user/history/clear` | 清空观看历史 |
| POST | `/api/v1/user/watchlater` | 稍后再看 |
| POST | `/api/v1/user/appointments` | 我的预约 |
| GET | `/api/v1/collects` | 收藏夹列表 |
| GET | `/api/v1/feed` | 推荐流 |
| POST | `/api/v1/video/digg` | 点赞 / 取消点赞 |
| POST | `/api/v1/video/comment` | 发表 / 回复评论 |
| POST | `/api/v1/video/collect` | 收藏 / 取消收藏 |
| POST | `/api/v1/video/collect/move` | 移动收藏 |
| POST | `/api/v1/video/collect/remove` | 取消收藏（指定收藏夹） |
| POST | `/api/v1/live/info` | 直播间信息 |
| POST | `/api/v1/live/listen/start` | 启动直播间监听 |
| POST | `/api/v1/live/listen/stop` | 停止直播间监听 |
| GET | `/api/v1/live/events` | 已收集的直播间事件 |
| POST | `/api/v1/live/comment` | 发送直播间弹幕 |
| POST | `/api/v1/live/like` | 直播间点赞 |
| POST | `/api/v1/live/rank/contribution` | 贡献榜 |
| POST | `/api/v1/live/rank/pk` | PK 榜 |
| POST | `/api/v1/live/rank/list` | 榜单列表 |
| POST | `/api/v1/live/rank/thousand_ticket` | 千票榜 |
| POST | `/api/v1/live/rank/pk/contribution` | PK 贡献榜 |
| POST | `/api/v1/live/enter` | 直播间进入（预热会话） |
| POST | `/api/v1/live/linkmic/list` | 连麦列表 |
| POST | `/api/v1/live/pk/context` | PK 上下文 |
| POST | `/api/v1/live/production` | 直播带货商品（单页） |
| POST | `/api/v1/live/production/all` | 直播带货商品（全部） |
| POST | `/api/v1/live/production/detail` | 商品详情 |
| POST | `/api/v1/live/product/comments` | 商品评论 |
| POST | `/api/v1/live/product/comments/counter` | 商品评论统计 |
| POST | `/api/v1/im/conversation/create` | 创建私信会话 |
| POST | `/api/v1/im/conversation/list` | 查询会话（含未读 `unread` / `unread_classes`） |
| GET/POST | `/api/v1/im/conversations` | 会话列表（群聊 + 私聊，含群名；支持 name/type 过滤） |
| POST | `/api/v1/im/conversation/info` | 会话详情（群名、群主等） |
| POST | `/api/v1/im/conversation/history` | 聊天记录（群/私聊，游标翻页；`unread_only=true` 只返回未读） |
| POST | `/api/v1/im/conversation/participants` | 群成员列表（含昵称、sec_uid） |
| POST | `/api/v1/im/conversation/mark_read` | 标记会话已读 |
| POST | `/api/v1/im/conversation/setting` | 置顶聊天 / 消息免打扰 |
| POST | `/api/v1/im/conversation/leave` | 退出群聊 |
| POST | `/api/v1/im/conversation/share` | 群聊分享信息（群名 + 链接 + 文案） |
| GET | `/api/v1/im/strangers` | 陌生人会话 |
| POST | `/api/v1/im/user_info` | 按 sec_uid 批量查昵称/头像 |
| POST | `/api/v1/im/send` | 发送私信文本 |
| POST | `/api/v1/im/send_media` | 发送图片/视频/语音/文件 |
| POST | `/api/v1/im/send_sticker` | 发送表情包 |
| POST | `/api/v1/im/send_card` | 发送卡片 |
| POST | `/api/v1/im/share_aweme` | 分享作品 |
| POST | `/api/v1/im/share_web` | 分享链接 |
| POST | `/api/v1/im/send_user_card` | 分享用户名片 |
| POST | `/api/v1/im/listen/start` | 启动私信接收 |
| POST | `/api/v1/im/listen/stop` | 停止私信接收 |
| GET | `/api/v1/im/messages` | 已接收私信 |
| POST | `/api/v1/publish` | 发布图文 |
| POST | `/api/v1/publish_video` | 发布视频 |

### 五个主导航 tab 补齐的接口（2026-10-06）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/channel/module/feed` | 精选/推荐频道模块流（`/aweme/v2/web/module/feed/`） |
| POST | `/api/v1/course/category/tags` | 精选「公开课」分类标签 |
| POST | `/api/v1/course/category/videos` | 精选「公开课」分类作品列表 |
| POST | `/api/v1/solution/resources` | 精选页资源位 |
| POST | `/api/v1/multicast/config` | 多播配置 |
| POST | `/api/v1/page/turn/offline` | 页面下线开关 |
| POST | `/api/v1/emoji/list` | 表情列表 |
| POST | `/api/v1/creator/publish/highlight` | 投稿入口角标（未发布/未使用） |
| POST | `/api/v1/mix/listcollection` | 合集列表 |
| POST | `/api/v1/seo/inner/link` | 页脚内链 |
| POST | `/api/v1/study/notes` | 学习 AI 笔记列表 |
| POST | `/api/v1/misc/social/follow/feed` | 关注视频流 |
| POST | `/api/v1/misc/social/familiar/feed` | 朋友视频流 |
| POST | `/api/v1/misc/social/familiar/recommend/feed` | 朋友推荐流 |
| POST | `/api/v1/misc/social/follow/live/top` | 关注中的直播（顶bar） |
| POST | `/api/v1/misc/social/follow/live/feed` | 关注/朋友在播流 |
| POST | `/api/v1/misc/social/history/write` | 上报播放历史 |
| POST | `/api/v1/misc/social/aweme/stats` | 上报播放统计 |
| POST | `/api/v1/misc/social/following/seen` | 标记关注列表已读 |
| POST | `/api/v1/misc/social/danmaku` | 视频弹幕 |
| POST | `/api/v1/misc/social/danmaku/conf` | 弹幕配置 |
| POST | `/api/v1/misc/social/series/watch` | 上报短剧观看进度 |
| POST | `/api/v1/user/my-profile` | 自己的资料（`profile/self`） |
| POST | `/api/v1/user/dashboard` | 创作者数据概览 |
| POST | `/api/v1/user/social-count` | 关注/粉丝/获赞计数 |
| POST | `/api/v1/user/settings` | 用户设置（`source=www\|hj`） |
| POST | `/api/v1/user/custom-settings` | 自定义设置 |
| POST | `/api/v1/user/collected-awemes` | 收藏的作品列表（`aweme/listcollection`） |
| POST | `/api/v1/baike/check-worldbook` | 百科词书权限 |
| POST | `/api/v1/baike/binding-subject` | 百科绑定主体 |
| POST | `/api/v1/im/spotlight/relation` | 消息面板「互动/关注动态」 |
| POST | `/api/v1/im/active/status` | 私信在线状态（批量） |
| POST | `/api/v1/im/active/heartbeat` | 在线心跳 |
| POST | `/api/v1/im/active/config` | 在线状态配置 |
| POST | `/api/v1/im/strategy/config` | 互动资源策略配置 |
| POST | `/api/v1/im/resources` | 互动资源（自定义表情等） |
| POST | `/api/v1/im/emoticon/trending` | 热门表情 |
| POST | `/api/v1/im/feedback/entrance` | 反馈入口 |
| POST | `/api/v1/im/messages/pull` | 增量拉取私信/群消息（protobuf cmd 2048） |
| POST | `/api/v1/im/conversation/name` | 改群名/群简介/群公告（cmd 902） |
| POST | `/api/v1/im/conversation/kick` | 把成员移出群（cmd 651） |
| POST | `/api/v1/im/message/recall` | 撤回消息（cmd 702） |
| POST | `/api/v1/im/message/delete` | 删除消息（cmd 701） |
| POST | `/api/v1/hot/search/videos` | 热搜词视频列表（www-hj 域） |
| POST | `/api/v1/music/aweme` | 某个音乐下的视频（webSign） |
| POST | `/api/v1/music/detail` | 音乐详情 |
| POST | `/api/v1/music/listcollection` | 我收藏的音乐 |
| POST | `/api/v1/music/collect` | 收藏/取消收藏音乐（写） |
| POST | `/api/v1/ecom/product/sku/list` | 商品 SKU/规格（www-hj 域） |
| POST | `/api/v1/notice/digg/list` | 某条通知下的点赞用户（参数名固定 `notice_id`） |

## 示例

```bash
# 健康检查
curl -s localhost:18080/health

# 作品详情
curl -s localhost:18080/api/v1/video/detail \
  -H 'content-type: application/json' -d '{"video":"7690200882625006867"}'

# 搜索视频（也支持 GET query）
curl -s 'localhost:18080/api/v1/search/videos?keyword=榴莲&sort_type=2&publish_time=7'
```

## MCP 端点

MCP 走 `http://localhost:18080/mcp`（Streamable HTTP，无状态）。客户端配置见 [README](../README.md#2-mcp-客户端接入)。
