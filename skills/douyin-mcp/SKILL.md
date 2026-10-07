---
name: douyin-mcp
description: >
  抖音数据与操作技能。通过 douyin-mcp 提供的 MCP 工具读取抖音数据（用户资料、作品、
  评论、搜索、热搜、直播、通知、私信）并执行互动/发布操作（点赞、评论、收藏、
  直播间弹幕、私信收发、创作者图文/视频发布）。用户说"查抖音""搜抖音""看抖音评论"
  "发一条抖音""监听直播间"时使用。
---

# 抖音 MCP 技能

本技能通过 `douyin-mcp` 服务暴露的 MCP 工具访问抖音。服务默认地址：`http://localhost:18080/mcp`。

## 前置检查

1. 确认服务已启动并可用：调用 `check_login_status`。
   - 未登录时调用 `get_login_qrcode` 拿到二维码图片 + `token`，让用户扫码，再用 `check_login_qrcode` 轮询；
     或调用 `send_phone_code` / `login_by_phone` 走短信登录。
2. 写操作（点赞 / 评论 / 私信 / 发布）需要 bd-ticket-guard 材料，服务端未配置时会返回明确错误，直接转述即可。

## 工具分组

- **登录**：`check_login_status`、`get_login_qrcode`、`check_login_qrcode`、`send_phone_code`、`login_by_phone`、`delete_cookies`
- **用户 / 作品**：`get_user_info`、`get_user_posts`、`get_user_all_posts`、`get_video_detail`
- **评论**：`get_video_comments`、`get_all_video_comments`、`get_sub_comments`、`post_video_comment`
- **搜索**：`search_videos`、`search_users`、`search_lives`、`search_suggest`、`get_hot_search_board`、`search_challenges`
- **社交 / 关系**：`get_user_followers`、`get_user_following`、`get_user_favorites`
- **通知**：`get_notice_count`、`get_notices`、`get_notice_detail`、`delete_notice`
- **互动**：`digg_video`、`collect_video`、`move_collect_video`、`remove_collect_video`
- **直播**：`get_live_info`、`start_live_listen`、`stop_live_listen`、`get_live_events`、`send_live_comment`、`like_live_room` 及各榜单
- **私信**：`list_conversations`、`get_conversation_history`、`send_dm`（文本/媒体/表情/卡片/分享）、`start_im_listen`、`get_im_messages`
- **发布**：`publish_content`（图文）、`publish_with_video`（视频）

完整 133 个工具以服务端 `ListTools` 返回为准。

## 工作流程示例

### 查某个用户的作品并总结评论

```
1. get_user_info(user=<sec_uid 或主页链接>)
2. get_user_posts(user=<sec_uid>, count=<n>)
3. 对感兴趣的 aweme_id：get_video_detail(video=<id>) → get_video_comments(video=<id>)
```

### 关键词搜索

```
search_videos(keyword="榴莲", count=20, sort_type="2")   # 2 = 最新发布
```

### 监听直播间

```
1. get_live_info(web_rid=<房间号>)         # 拿到 room_id 等
2. start_live_listen(web_rid=<房间号>)
3. 周期调用 get_live_events()               # 返回 chat / gift / member / like / room_stats / pk
4. stop_live_listen()
```

### 创作者发布

```
1. 确认已配置 DOUYIN_TICKET / TS_SIGN / CLIENT_CERT / PRIVATE_KEY，发布另需 DOUYIN_DTRAIT_BLOB
2. publish_content(title=..., content=..., images=[<本地绝对路径或 URL>, ...], tags=[...])
   或 publish_with_video(title=..., content=..., video=<本地绝对路径>)
```

## 注意事项

- 只读工具（`get_*` / `search_*`）优先，写工具（`digg_*` / `post_*` / `publish*` / `send_*` / 直播间 `*_comment`）执行前应向用户确认。
- 结果为抖音原始结构，字段可能随时变化；如需精简，请在展示前自行归纳，不要编造字段。
- 不要向用户回显 cookie / ticket / 私钥等敏感材料。
