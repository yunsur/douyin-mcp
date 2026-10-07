# 抖音接口覆盖与校准

本文档记录对当前抖音 PC 网页端的接口覆盖情况与逐接口抓包校准结论。

## 搜索功能的覆盖（对照当前 PC 版）

| 搜索面 | 接口 | 状态 |
| --- | --- | --- |
| 综合 | `/aweme/v1/web/general/search/single/` | ✅ `search_videos`（channel=general，支持排序/时间/时长/范围/内容形式） |
| 视频 | `/aweme/v1/web/search/item/` | ✅ `search_videos`（channel=video） |
| 用户 | `/aweme/v1/web/discover/search/` | ✅ `search_users` |
| 直播 | `/aweme/v1/web/live/search/` | ✅ `search_lives` |
| 联想建议 | `/aweme/v1/web/search/sug/` | ✅ `search_suggest`（实测 10 条） |
| 热搜榜 | `/aweme/v1/web/hot/search/list/` | ✅ `get_hot_search_board`（实测 51 词 + 5 上升热点） |
| 话题/挑战 | `/aweme/v1/web/challenge/search/` | ✅ `search_challenges`（实测 9 条） |
| 热搜词视频 | `/aweme/v1/web/hot/search/video/list/` | ✅ `get_hot_search_videos`（此前"被 bdturing 拦截"是打在了 www；改走 **www-hj** + 原样参数后实测 `status_code=0`、`aweme_list=2`） |
| 音乐 | ~~`/aweme/v1/web/music/list/`~~ | ⚠️ 该 path 在当前 PC 客户端**不存在**（douyin_web 全部 chunk grep 无命中）。真实音乐接口已实现：`get_music_aweme`（音乐下的视频，webSign）、`get_music_detail`（音乐详情）、`get_music_collection`（我收藏的音乐，`mc_list`）、`collect_music`（收藏/取消，写） |
| 商品 | 无独立搜索频道 | ⚠️ 已核实：PC 搜索结果页只有 综合/视频/用户/直播 四个 tab（bundle 里 channel 只有 `aweme_video/aweme_image/aweme_live/aweme_related_card/aweme_wenda`，无 commerce）。商品走商品详情页/直播间小黄车：已覆盖 `product/comments`、`product/comment/counter`，新增 `get_product_sku_list` |

## 五个主导航 tab 的接口覆盖（对照当前 PC 版，2026-10-06）

用已登录浏览器注入 XHR/fetch 钩子，对 `精选 / 推荐 / 关注 / 朋友 / 我的`（含个人页 7 个子 tab）
逐 tab 抓包，得到 61 个业务接口；原先未覆盖的 38 个已全部补齐：

| tab | 接口 | MCP 工具 | 实测 |
| --- | --- | --- | --- |
| 精选 | `/aweme/v2/web/module/feed/` | `get_channel_module_feed` | status_code=0（`use_lite_type=1` 返回 JSON 视频流；`=2` 为浏览器默认，返回 protobuf，已内置解码） |
| 精选 | `/aweme/v1/web/douyin/select/tab/course/catagory/tag/` | `get_course_category_tags` | tag_list 6 |
| 精选 | `/aweme/v1/web/douyin/select/tab/course/catagory/video/` | `get_course_category_videos` | video_items 5（offset=6 翻页再取 6 条） |
| 精选 | `/aweme/v1/web/solution/resource/list/` | `get_solution_resources` | status_code=0 |
| 精选 | `/aweme/v1/web/multicast/query/` | `get_multicast_config` | status_code=0 |
| 精选 | `/aweme/v1/web/page/turn/offline` | `page_turn_offline` | status_code=0 |
| 精选 | `/aweme/v1/web/emoji/list` | `get_emoji_list` | emoji_list 371 |
| 推荐 | `/aweme/v1/creator/external/highlight/` | `get_publish_highlight` | status_code=0 |
| 合集 | `/aweme/v1/web/mix/listcollection/` | `get_mix_list_collection` | mix_infos 15 |
| 页脚 | `/aweme/v1/web/seo/inner/link/` | `get_seo_inner_link` | link_data 13 |
| 学习 | `/aweme/v1/web/douyin/select/study/ai_assistant/note/list` | `get_study_notes` | status_code=0 |
| 关注 | `/aweme/v1/web/follow/feed/` | `get_follow_feed` | 真实 aweme 列表 |
| 关注 | `/webcast/feed/follow_top/` | `get_follow_live_top` | 直播卡片 |
| 关注 | `/aweme/v1/web/following/list/item/seen/` | `mark_following_seen` | status_code=0 |
| 关注 | `/aweme/v1/web/danmaku/get_v2/` | `get_danmaku` | danmaku_list |
| 关注 | `/aweme/v1/web/danmaku/conf/get/` | `get_danmaku_conf` | conf_list |
| 关注 | `/aweme/v1/web/series/watch/record/` | `report_series_watch` | status_code=0（`episode/series_id/item_id`，从 PC bundle 反查 + 播放态抓包确认） |
| 关注 | `/aweme/v1/web/history/write/` | `report_history_write` | 服务端返回 `status_code=7 无权限操作`（账号侧权限，参数/签名已对；组件层面如实透传） |
| 朋友 | `/aweme/v1/web/familiar/feed/` | `get_familiar_feed` | 真实 aweme 列表 |
| 朋友 | `/aweme/v1/web/familiar/recommend/feed/` | `get_familiar_recommend_feed` | aweme_list |
| 朋友 | `/webcast/web/feed/follow/` | `get_follow_live_feed` | 拉流地址 |
| 我的 | `/aweme/v1/web/user/profile/self/` | `get_my_profile` | nickname/粉丝数 |
| 我的 | `/aweme/v1/web/user/dashboard` | `get_user_dashboard` | status_code=0 |
| 我的 | `/aweme/v1/web/social/count` | `get_social_count` | status_code=0 |
| 我的 | `/aweme/v1/web/get/user/settings/`、`/aweme/v1/web/user/settings/` | `get_user_settings`（`source=www\|hj`） | 两个 host 都通 |
| 我的 | `/aweme/v1/web/custom/settings/get/` | `get_custom_settings` | status_code=0 |
| 我的 | `/aweme/v1/web/aweme/listcollection/` | `get_collected_awemes` | aweme_list 7（见下方说明） |
| 我的 | `baike.douyin.com/webcast/ip/wiki/*` | `baike_check_worldbook` / `baike_binding_subject` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/spotlight/relation/` | `get_im_spotlight_relation` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/user/active/status/` | `get_im_active_status` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/user/active/update/` | `im_active_heartbeat` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/user/active/config/get` | `get_im_active_config` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/strategy/config` | `get_im_strategy_config` | decision_trees |
| 消息面板 | `/aweme/v1/web/im/resource/list/aggregation/` | `get_im_resources` | status_code=0 |
| 消息面板 | `/aweme/v1/web/im/resources/emoticon/trending` | `get_im_emoticon_trending` | emoticon_data |
| 消息面板 | `/aweme/v1/web/im/get/online_feedback/entrance/` | `get_im_feedback_entrance` | status_code=0 |
| 消息面板 | `imapi.douyin.com/v1/message/get_user_message` | `pull_im_messages` | protobuf cmd/body=2048，实测拉到增量消息 |
| 热搜 | `www-hj.douyin.com/aweme/v1/web/hot/search/video/list/` | `get_hot_search_videos` | `status_code=0`、`aweme_list=2`（`hotword/sentence_id/offest/count/entry_name`） |
| 音乐 | `/aweme/v1/web/music/aweme/` | `get_music_aweme` | `status_code=0`、`aweme_list=1`（webSign 端点） |
| 音乐 | `/aweme/v1/web/music/detail/` | `get_music_detail` | `music_info.id_str` 与请求一致 |
| 音乐 | `/aweme/v1/web/music/listcollection/` | `get_music_collection` | `mc_list=3` |
| 音乐 | `/aweme/v1/web/music/collect/` | `collect_music`（写） | 未真机调用（避免改账号收藏），实现照 bundle |
| 商品 | `www-hj.douyin.com/aweme/v1/web/ecom/product/sku/list/` | `get_product_sku_list` | `status_code=0`、`specs=1`、`skus=5`（首个 SKU「混合随机口味约500g*1箱」）；**必须走 hj 域**，www 同签名请求 403 |
| 通知 | `/aweme/v1/web/notice/digg/list/` | `get_notice_digg_list` | `status_code=0`（参数名固定 `notice_id`；当前账号返回 `digg_list:null`/`user_list:[]`/`total:100`，服务端不回点赞人） |

> **精选页分类 tab 的结论**（全部/游戏/二次元/音乐/影视/美食/知识/小剧场/生活vlog/体育/旅行/亲子/动物/三农/汽车/美妆穿搭）：
> 逐一点击实测，17 个 tab **没有各自的接口** —— 只有 `POST /aweme/v2/web/module/feed/`
> （`module_id=3003101`，仅 `refresh_index`/`pull_type` 随刷页变化，各分类完全一致），且点分类时
> **不产生任何请求**：分类内容来自首屏文档（3.9MB 的 RSC/SSR HTML，实测首屏源码里同时含
> 「游戏」和「音乐」的条目），客户端本地切换；滚动也不会为分类再发请求。
> 唯一例外是 **公开课**：它有独立的 `catagory/tag` + `catagory/video` 接口，已分别由
> `get_course_category_tags` / `get_course_category_videos` 覆盖（`video_items`，offset 翻页可用）。

未覆盖的外壳类接口（会话/AB/埋点/水印，不做工具）：`passport/token/beat/web`、`passport/user_info/get_sec_ts`、
`service/2/abtest_config`、`cloudpush/update_sender`、`aweme/v1/web/app/installed`、`aweme/v1/web/mobile/ab/params`、
`aweme/v1/web/query/account/type`、`webcast/setting`、`webcast/web/live/watermark`、`pcim_saas_vmok_entry/*`。

> `aweme/listcollection`（收藏的作品）此前记录为「本机链路 404 不可达」，实际上是**域名差异**：
> 浏览器走 `www-hj.douyin.com`，并且 query 需要 `timestamp` + `x-secsdk-web-signature` 的 webSign 签名。
> 现在按 `SignWebURL(..., UID)` 签名 + hj 域名 + 表单 body 发送，实测 `aweme_list=7`。

## 个人主页各 tab 的支持情况（对照当前 PC 版）

| tab | 接口 | 状态 |
| --- | --- | --- |
| 作品 | `/aweme/v1/web/aweme/post/` | ✅ `get_user_posts` / `get_user_all_posts`（参数与客户端逐项一致） |
| 喜欢 | `/aweme/v1/web/aweme/favorite/` | ✅ `get_user_favorites`（实测 20 条） |
| 收藏（收藏夹） | `/aweme/v1/web/collects/list/` | ✅ `get_collect_list`（实测返回收藏夹） |
| 收藏（收藏的视频） | `/aweme/v1/web/aweme/listcollection/` | ✅ `get_collected_awemes`（hj 域名 + webSign 签名，实测 `aweme_list=7`；见上一节说明） |
| 观看历史 | `/aweme/v1/web/history/read/` | ✅ `get_watch_history` / `clear_watch_history` |
| 稍后再看 | `/aweme/v1/web/watchlater/list/` | ✅ `get_watch_later`（实测 `list_num=3`、2 条） |
| 我的预约 | `/aweme/v1/web/user/appointment/list/` | ✅ `get_my_appointments`（`version_code=320600`，与客户端一致） |
| 推荐 | `/aweme/v1/web/tab/feed/` | ✅ `get_homefeed` |

> `aweme/listcollection` 的**旧**验证结论：用浏览器抓到该请求后把 URL 原样重放（curl，同一 cookie）返回 404，
> 缺签名返回 403 `Signature Not Found`，于是判断为「边缘路由差异」——**该结论已于 2026-10-07 被推翻**：
> 那条被重放的 URL 里的签名是**旧值**（抓包钩子在 `open()` 时记录、secsdk 在 `send()` 时才重签名，实测 CDP 的 `:path`
> 与钩子记录在 `a_bogus`、`x-secsdk-web-signature` 上均不同）。用浏览器**实际发出**的 URL 本机重放 → 200 + `aweme_list`；
> 本机自算签名（hj 域 + `SignWebURL`）同样 200。所以它不是「本机到不了」。

## 通知功能

`get_notice_count`（未读数，返回 `interactive_group` 分区与各分区 `count`/`dot_count`）、
`get_notices` / `get_all_notices`（列表，`notice_group` 选分区）、`get_notice_detail`、
`get_notice_digg_list`（某条通知下的点赞用户，`notice_id` 用 `nid_str`）、`delete_notice`。
分区取值（与当前 PC 版一致）：`700` 全部、`401` 粉丝、`601` @我的、
`2` 评论、`3` 点赞、`520` 弹幕。

> `/aweme/v1/web/notice/digg/list/`（某条通知下的点赞用户）已补齐为 `get_notice_digg_list`。
> **参数坑**：同模块其它通知接口用 `nid_str` / `notice_id_str`，但这个接口**只认 `notice_id`**
> （传 nid_str/notice_id_str/cid/item_id/aweme_id 等一律 `status_code=5 参数不合法`，
> 2026-10-07 用真实通知逐项实测确认）；`notice_id` 取 `get_notices` 结果里的 `nid_str`。
> 请求层打通后实测 `status_code=0`，但本账号当前的 31/33/41 三类通知（评论/粉丝/赞）
> 返回体均为 `digg_list: null`、`user_list: []`、`total: 100` —— 服务端现阶段不回点赞人列表
> （点赞人本身已经直接出现在通知项里），工具如实透传该结构。

## 消息功能（群聊 / 私聊）

除发送（文本/图片/视频/语音/文件/表情包/卡片/分享）与实时接收外，还支持完整读取：
`list_conversations`（含群名、可过滤，并带服务端未读数 `unread` 与分类计数 `unread_classes`）、`get_conversation_history`（按群名或会话 id 翻页读历史；`unread_only=true` 只返回未读消息）、
`get_conversation_info`、`get_conversation_participants`（群成员昵称）、`get_stranger_conversations`、
`mark_conversation_read`、`set_conversation_setting`（置顶 / 免打扰）、`leave_conversation`（退群）、
`update_conversation_name`（改群名/群简介/群公告）、`kick_conversation_participants`（踢人）、
`recall_message`（撤回消息）、`delete_message`（删除消息）、
`share_conversation`、`get_im_user_info`（uid→昵称）。

协议来自对当前 PC 版网页端的抓包（2026-10-05），cmd 与 body 包装字段：
`2043` 会话列表、`301` 聊天记录、`605` 群成员、`610` 会话详情、`921` 会话设置（4=置顶，5=免打扰）、
`652` 退出群聊、`2002/604` 标记已读、`1001/1000` 陌生人会话，以及表单接口
`www-hj.douyin.com/aweme/v1/web/im/user/info/`。

群管理/消息管理（cmd = body 包装字段号，取自主站 IM 客户端 protobuf 定义）：
`902` 改群名/公告（字段 4=name、8=is_name_set）、`651` 移除群成员（字段 4=participants 重复 int64）、
`702` 撤回消息（字段 4=server_message_id）、`701` 删除消息（字段 4=message_id）。
用现代编号（7218/5210/5618/5610）会被服务端判成「body is nil / is empty」——实测确认，故按上面的编号实现。
这四个都是**写操作**：请求层用不存在的会话 id 实测过路由（服务端返回业务错误而非解析错误），
**没有**在真实群/真实会话上执行过（避免改动别人的群）。

## 私信接收

`start_im_listen` 连接 `frontier-im` WebSocket，`get_im_messages` 返回：
`message_index`、`conversation_id`、`conversation_type`、`notify_type`、`sender`、
`message_type`、`content`。`conversation_type=1` 为私聊，`2` 为群聊；
`conversation_id` 在私聊下形如 `0:1:<a>:<b>`，群聊为纯数字。

## 与当前 PC 版网页端的校准（2026-10-05）

用 chrome-devtools 驱动真实登录的 PC 网页端抓包，逐接口比对请求，发现并修复：

| 问题 | 影响 | 修复 |
| --- | --- | --- |
| 手动设置 `content-length` 与传输层重复 | **所有带 body 的 POST 被 CDN 判 400**（点赞/评论/收藏/发布/私信发送/全部 IM 写接口） | `douyin/httpclient.go` 不再手写该头，交由 HTTP/2 传输层生成；新增回归测试断言只出现一次 |
| IM 请求的 cmd body 未包在 body 类型字段内（如应为 `{2043:{…}}`、`{604:{…}}`） | 服务端解析失败 | `imCall` 增加 bodyField 参数，逐接口按抓包包装 |
| `build_number` 多了一个 `B` | 与浏览器不一致 | 按抓包改为 24 字符 `0d50935:feat/pc-im-group`（旧笔记里的 `B` 其实是下一个字段的 tag 0x42） |
| `mark_read` / 陌生人会话 body 被二次包装 | 报错 `Invalid conversation type` | 修正为单层包装 |
| 通知分区取值写错（960/721…） | 通知列表分区取不到数据 | 按抓包改为 700/401/601/2/3/520，并补 `notice/count`、`notice/detail`、`notice/del` |
| 个人主页 tab 缺 观看历史 / 稍后再看 / 我的预约 | 三个 tab 无接口 | 按抓包补齐（含清空历史）；`喜欢` 补上客户端会发的 msToken |
| 搜索缺 联想/热搜/话题 | 三个搜索面不可用 | 按抓包补齐（实测 10/51/9 条） |
| 热搜词视频 / 音乐 标记为「风控不可达」 | 两批接口被放弃 | 复核为**误判**：热搜词视频要打 **www-hj** 域；`music/list` 这个 path 根本不存在。现按真实端点补齐（`get_hot_search_videos` + `get_music_aweme/detail/listcollection/collect`） |

## 行为修正记录

| 问题 | 处理 |
| --- | --- |
| 关注列表在 `max_time` 为空/`0` 时返回空 | **已修**：替换为当前秒级时间戳并固定 `source_type=1`——否则服务端走推荐分支，返回 `status_code=0` 但 `followings=[]`。`2096`（隐私受限，`followings=null` 且无 `has_more`）返回空列表不崩溃。 |
| 直播间礼物消息重复 | **已修**：消息按 `(method, msgId)` 去重（512 条 LRU）。 |
| 直播断线重连 | 退避指数增长封顶 30s；建链/握手类失败连续 3 次即退出；已建立的连接掉线后继续重连（不设总次数上限，避免无人值守长直播中途永久断开）。 |
| webcast proto 覆盖面 | 可后续替换 `douyin/proto/` 里的 schema。 |
| 缺 `s_v_web_id` | 客户端自动生成，无需外部提供。 |
| 视频下载 | 本仓只返回直链，不落盘。 |
