package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	"github.com/yunsur/douyin-mcp/douyin"
)

// --- Tool argument structs -------------------------------------------------

type userArgs struct {
	User string `json:"user" jsonschema:"用户 sec_user_id 或主页链接"`
}

type userPostsArgs struct {
	User      string `json:"user" jsonschema:"用户 sec_user_id 或主页链接"`
	MaxCursor string `json:"max_cursor,omitempty" jsonschema:"分页游标，首次传 0 或留空"`
	Count     string `json:"count,omitempty" jsonschema:"数量参数（保留字段）"`
}

type userAllPostsArgs struct {
	User  string `json:"user" jsonschema:"用户 sec_user_id 或主页链接"`
	Limit int    `json:"limit,omitempty" jsonschema:"最多返回的作品数量，0 表示全部"`
}

type videoArgs struct {
	Video string `json:"video" jsonschema:"作品 aweme_id 或作品链接"`
}

type videoCommentsArgs struct {
	Video  string `json:"video" jsonschema:"作品 aweme_id 或作品链接"`
	Cursor string `json:"cursor,omitempty" jsonschema:"评论游标，首次传 0 或留空"`
	Count  string `json:"count,omitempty" jsonschema:"数量参数（保留字段）"`
}

type allVideoCommentsArgs struct {
	Video          string `json:"video" jsonschema:"作品 aweme_id 或作品链接"`
	Limit          int    `json:"limit,omitempty" jsonschema:"最多返回的一级评论数，0 表示全部"`
	IncludeReplies bool   `json:"include_replies,omitempty" jsonschema:"是否同时抓取二级回复"`
}

type subCommentsArgs struct {
	Video     string `json:"video" jsonschema:"作品 aweme_id 或作品链接"`
	CommentID string `json:"comment_id" jsonschema:"一级评论 ID（cid）"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"回复游标，首次传 0 或留空"`
	Count     string `json:"count,omitempty" jsonschema:"每页数量，默认 5"`
}

type searchVideosArgs struct {
	Keyword        string `json:"keyword" jsonschema:"搜索关键词"`
	Offset         string `json:"offset,omitempty" jsonschema:"分页偏移，首次 0"`
	Count          int    `json:"count,omitempty" jsonschema:"自动翻页抓取的作品数量，>0 时忽略 offset"`
	Channel        string `json:"channel,omitempty" jsonschema:"general(综合,默认) | video(视频频道)"`
	SortType       string `json:"sort_type,omitempty" jsonschema:"0 综合排序 | 1 最多点赞 | 2 最新发布"`
	PublishTime    string `json:"publish_time,omitempty" jsonschema:"0 不限 | 1 一天内 | 7 一周内 | 180 半年内"`
	FilterDuration string `json:"filter_duration,omitempty" jsonschema:"视频时长：空 不限 | 0-1 | 1-5 | 5-10000"`
	SearchRange    string `json:"search_range,omitempty" jsonschema:"0 不限 | 1 最近看过 | 2 还未看过 | 3 关注的人"`
	ContentType    string `json:"content_type,omitempty" jsonschema:"0 不限 | 1 视频 | 2 图文"`
}

type searchUsersArgs struct {
	Keyword string `json:"keyword" jsonschema:"搜索关键词"`
	Offset  string `json:"offset,omitempty" jsonschema:"分页偏移，留空则自动翻页返回 25 个"`
	Count   string `json:"count,omitempty" jsonschema:"每页数量，默认 25"`
}

type searchLivesArgs struct {
	Keyword string `json:"keyword" jsonschema:"搜索关键词"`
	Offset  string `json:"offset,omitempty" jsonschema:"分页偏移，留空则自动翻页返回 15 个"`
	Count   string `json:"count,omitempty" jsonschema:"每页数量，默认 15"`
}

type suggestArgs struct {
	Keyword string `json:"keyword" jsonschema:"搜索关键词"`
}

type relationArgs struct {
	User    string `json:"user" jsonschema:"用户 sec_user_id 或主页链接"`
	MaxTime string `json:"max_time,omitempty" jsonschema:"分页游标，首次 0 或留空"`
	Count   string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
}

type noticesArgs struct {
	NoticeGroup string `json:"notice_group,omitempty" jsonschema:"通知分组：700 全部(默认) | 401 粉丝 | 601 @我的 | 2 评论 | 3 点赞 | 520 弹幕"`
	Count       string `json:"count,omitempty" jsonschema:"每页数量，默认 10"`
}

type favoritesArgs struct {
	SecUserID string `json:"sec_user_id" jsonschema:"用户 sec_user_id"`
	MaxCursor string `json:"max_cursor,omitempty" jsonschema:"分页游标，首次 0 或留空"`
	Count     string `json:"count,omitempty" jsonschema:"每页数量，默认 18"`
}

type homeFeedArgs struct {
	Count        string `json:"count,omitempty" jsonschema:"数量，默认 20"`
	RefreshIndex string `json:"refresh_index,omitempty" jsonschema:"刷新索引，默认 2"`
}

type diggArgs struct {
	AwemeID  string `json:"aweme_id" jsonschema:"作品 aweme_id 或作品链接"`
	DiggType string `json:"digg_type,omitempty" jsonschema:"1 点赞（默认）| 0 取消点赞"`
}

type commentArgs struct {
	AwemeID string `json:"aweme_id" jsonschema:"作品 aweme_id 或作品链接"`
	Content string `json:"content" jsonschema:"评论内容"`
	ReplyID string `json:"reply_id,omitempty" jsonschema:"要回复的评论 ID，留空为发表一级评论"`
}

type collectArgs struct {
	AwemeID string `json:"aweme_id" jsonschema:"作品 aweme_id 或作品链接"`
	Action  string `json:"action,omitempty" jsonschema:"1 收藏（默认）| 0 取消收藏"`
}

type moveCollectArgs struct {
	AwemeID     string `json:"aweme_id" jsonschema:"作品 aweme_id 或作品链接"`
	CollectName string `json:"collect_name" jsonschema:"收藏夹名称"`
	CollectID   string `json:"collect_id" jsonschema:"收藏夹 ID"`
}

type liveInfoArgs struct {
	WebRID string `json:"web_rid" jsonschema:"直播间号或 https://live.douyin.com/<直播间号>"`
}

type liveCommentArgs struct {
	RoomID  string `json:"room_id" jsonschema:"直播间 room_id（由 get_live_info 获取）"`
	Content string `json:"content" jsonschema:"弹幕内容"`
}

type liveLikeArgs struct {
	RoomID string `json:"room_id" jsonschema:"直播间 room_id"`
	Count  string `json:"count,omitempty" jsonschema:"点赞次数，默认 1"`
}

type liveRankArgs struct {
	RoomID      string `json:"room_id" jsonschema:"直播间 room_id"`
	AnchorID    string `json:"anchor_id,omitempty" jsonschema:"主播 ID"`
	SecAnchorID string `json:"sec_anchor_id,omitempty" jsonschema:"主播 sec_uid"`
}

type livePKRankArgs struct {
	WebRID string `json:"web_rid" jsonschema:"直播间号或直播链接"`
	Side   string `json:"side,omitempty" jsonschema:"current(默认) | both"`
}

type liveProductionArgs struct {
	PageURL  string `json:"page_url" jsonschema:"直播间页面 URL，如 https://live.douyin.com/<房间号>"`
	RoomID   string `json:"room_id,omitempty" jsonschema:"直播间 room_id"`
	AuthorID string `json:"author_id,omitempty" jsonschema:"主播 ID"`
	Offset   string `json:"offset,omitempty" jsonschema:"分页偏移"`
}

type productCommentsArgs struct {
	ProductID string `json:"product_id" jsonschema:"商品 ID"`
	ShopID    string `json:"shop_id" jsonschema:"店铺 ID"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"分页游标"`
}

type imMessagesArgs struct {
	ConversationID   string `json:"conversation_id,omitempty" jsonschema:"只返回该会话的消息（私聊形如 0:1:a:b，群聊为纯数字 id）"`
	ConversationType int    `json:"conversation_type,omitempty" jsonschema:"1 私聊 | 2 群聊，0 不限"`
}

type listConversationsArgs struct {
	Name string `json:"name,omitempty" jsonschema:"按名称过滤（模糊匹配），留空返回全部"`
	Type int    `json:"type,omitempty" jsonschema:"1 私聊 | 2 群聊，0 不限"`
}

type conversationRefArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id（群为纯数字；私聊形如 0:1:a:b）或会话名称，如 九号mz5闲聊群"`
}

type conversationHistoryArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id 或名称"`
	Cursor       int64  `json:"cursor,omitempty" jsonschema:"翻页游标，0 表示最新一页；返回 next_cursor 可继续向前翻"`
	Count        int    `json:"count,omitempty" jsonschema:"每页条数，默认 50（上限 50）"`
	UnreadOnly   bool   `json:"unread_only,omitempty" jsonschema:"只返回未读消息（按服务端未读计数从最新往回取，自动翻页）"`
}

type conversationParticipantsArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id 或名称"`
	Offset       int64  `json:"offset,omitempty" jsonschema:"分页偏移，默认 0"`
	Count        int    `json:"count,omitempty" jsonschema:"每页条数，默认 50"`
}

type conversationSettingArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id 或名称"`
	Pin          *bool  `json:"pin,omitempty" jsonschema:"是否置顶聊天（true 置顶 / false 取消置顶），不传则不改动"`
	Mute         *bool  `json:"mute,omitempty" jsonschema:"是否消息免打扰（true 免打扰 / false 取消），不传则不改动"`
}

type noticeDetailArgs struct {
	NoticeID string `json:"notice_id_str" jsonschema:"通知 ID（从 get_notices / get_notice_digg_list 结果里取）"`
}

type noticeDeleteArgs struct {
	NoticeID   string `json:"notice_id_str" jsonschema:"要删除的通知 ID"`
	ActionType string `json:"action_type,omitempty" jsonschema:"删除动作类型，默认 0"`
}

type pageCursorArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"翻页游标，0 表示第一页"`
	Offset string `json:"offset,omitempty" jsonschema:"偏移量（稍后再看），默认 0"`
	Count  string `json:"count,omitempty" jsonschema:"每页条数，默认 20"`
}

type appointmentArgs struct {
	AppointmentType string `json:"appointment_type,omitempty" jsonschema:"预约类型，默认 100"`
	Count           int    `json:"count,omitempty" jsonschema:"条数，默认 -1 表示全部"`
}

type imUserInfoArgs struct {
	SecUIDs []string `json:"sec_uids" jsonschema:"要查询的 sec_uid 列表（最多几十个）"`
}

type conversationArgs struct {
	ToUserID            int64 `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	ConversationShortID int64 `json:"conversation_short_id,omitempty" jsonschema:"会话短 ID（查询会话时必填，取 list_conversations 里该会话的 conversation_short_id；为 0 时服务端返回 request.MGet empty）"`
}

type sendDMArgs struct {
	ToUserID int64  `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	Content  string `json:"content" jsonschema:"私信文本内容"`
}

type publishArgs struct {
	Title      string   `json:"title,omitempty" jsonschema:"标题"`
	Content    string   `json:"content" jsonschema:"正文内容"`
	Images     []string `json:"images,omitempty" jsonschema:"图片路径或 URL 列表（图集必填）"`
	Tags       []string `json:"tags,omitempty" jsonschema:"话题标签列表"`
	Visibility string   `json:"visibility,omitempty" jsonschema:"可见范围"`
}

type publishVideoArgs struct {
	Title      string   `json:"title,omitempty" jsonschema:"标题"`
	Content    string   `json:"content" jsonschema:"正文内容"`
	Video      string   `json:"video" jsonschema:"本地视频绝对路径"`
	Tags       []string `json:"tags,omitempty" jsonschema:"话题标签列表"`
	Visibility string   `json:"visibility,omitempty" jsonschema:"可见范围"`
}

type qrCheckArgs struct {
	Token string `json:"token" jsonschema:"二维码 token（由 get_login_qrcode 返回）"`
}

type phoneCodeArgs struct {
	Phone string `json:"phone" jsonschema:"手机号"`
}

type phoneLoginArgs struct {
	Phone string `json:"phone" jsonschema:"手机号"`
	Code  string `json:"code" jsonschema:"短信验证码"`
}

type liveProductionDetailArgs struct {
	PageURL     string `json:"page_url" jsonschema:"直播间页面 URL"`
	PromotionID string `json:"promotion_id" jsonschema:"商品 promotion_id"`
	OriginType  string `json:"origin_type,omitempty" jsonschema:"来源类型，默认 638303"`
}

type productCounterArgs struct {
	ProductID string `json:"product_id" jsonschema:"商品 ID"`
	ShopID    string `json:"shop_id" jsonschema:"店铺 ID"`
	StatID    string `json:"stat_id,omitempty" jsonschema:"统计 ID"`
}

type linkmicArgs struct {
	RoomID    string `json:"room_id" jsonschema:"直播间 room_id"`
	ChannelID string `json:"channel_id" jsonschema:"连麦 channel_id"`
}

type pkContextArgs struct {
	WebRID    string `json:"web_rid" jsonschema:"直播间号或直播链接"`
	ChannelID string `json:"channel_id,omitempty" jsonschema:"channel_id"`
}

type thousandTicketArgs struct {
	RoomID string `json:"room_id" jsonschema:"直播间 room_id"`
	WebRID string `json:"web_rid" jsonschema:"直播间号"`
}

type pkContributionArgs struct {
	ChannelID string `json:"channel_id" jsonschema:"PK channel_id"`
	AnchorID  string `json:"anchor_id" jsonschema:"主播 ID"`
}

type sendDMMediaArgs struct {
	ToUserID int64  `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	Kind     string `json:"kind" jsonschema:"媒体类型：image|video|audio|file"`
	Path     string `json:"path" jsonschema:"本地文件绝对路径"`
}

type sendDMStickerArgs struct {
	ToUserID int64          `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	Sticker  map[string]any `json:"sticker" jsonschema:"表情包内容（与抖音 Web 端表情包消息的 content 结构一致）"`
}

type sendDMCardArgs struct {
	ToUserID int64          `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	Card     map[string]any `json:"card" jsonschema:"卡片内容"`
}

type shareDMAwemeArgs struct {
	ToUserID int64  `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	AwemeID  string `json:"aweme_id" jsonschema:"要分享的作品 aweme_id"`
}

type shareDMWebArgs struct {
	ToUserID int64  `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	URL      string `json:"url" jsonschema:"要分享的链接"`
}

type sendDMUserCardArgs struct {
	ToUserID int64  `json:"to_user_id" jsonschema:"对方用户数字 uid"`
	SecUID   string `json:"sec_uid" jsonschema:"要分享的用户 sec_uid"`
}

type allRelationArgs struct {
	User  string `json:"user" jsonschema:"用户 sec_user_id 或主页链接"`
	Limit int    `json:"limit,omitempty" jsonschema:"最多返回数量，0 表示全部"`
}

type allNoticesArgs struct {
	NoticeGroup string `json:"notice_group,omitempty" jsonschema:"通知分组：700 全部(默认) | 401 粉丝 | 601 @我的 | 2 评论 | 3 点赞 | 520 弹幕"`
	Limit       int    `json:"limit,omitempty" jsonschema:"最多返回数量，默认 20"`
}

// --- Server ----------------------------------------------------------------

// InitMCPServer creates the MCP server and registers every tool.
func InitMCPServer(appServer *AppServer) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "douyin-mcp", Version: "1.0.0"}, nil)
	registerTools(server, appServer)
	logrus.Info("MCP Server initialized with official SDK")
	return server
}

func withPanicRecovery[T any](
	toolName string,
	handler func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error),
) func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args T) (result *mcp.CallToolResult, resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{"tool": toolName, "panic": r}).Error("Tool handler panicked")
				logrus.Errorf("Stack trace:\n%s", debug.Stack())
				result = &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("工具 %s 执行时发生内部错误: %v\n\n请查看服务端日志获取详细信息。", toolName, r),
					}},
					IsError: true,
				}
				resp, err = nil, nil
			}
		}()
		return handler(ctx, req, args)
	}
}

// addTool registers a tool whose handler returns an internal result.
func addTool[T any](server *mcp.Server, name, description string, readOnly bool, appServer *AppServer,
	fn func(ctx context.Context, args T) (*MCPToolResult, error)) {
	annotations := &mcp.ToolAnnotations{Title: name}
	if readOnly {
		annotations.ReadOnlyHint = true
	} else {
		annotations.DestructiveHint = new(true)
	}
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description, Annotations: annotations},
		withPanicRecovery(name, func(ctx context.Context, req *mcp.CallToolRequest, args T) (*mcp.CallToolResult, any, error) {
			result, err := fn(ctx, args)
			if err != nil {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				}, nil, nil
			}
			return convertToMCPResult(result), nil, nil
		}))
}

func registerTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "check_login_status", "检查抖音登录状态", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.CheckLoginStatus(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_login_qrcode", "获取登录二维码（返回 Base64 图片和 token）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.CreateLoginQRCode(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "check_login_qrcode", "检查扫码登录状态；成功后 Cookie 自动写入本地", true, appServer,
		func(ctx context.Context, args qrCheckArgs) (*MCPToolResult, error) {
			res, err := svc.CheckLoginQRCode(ctx, args.Token)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_phone_code", "发送手机号登录验证码", false, appServer,
		func(ctx context.Context, args phoneCodeArgs) (*MCPToolResult, error) {
			res, err := svc.SendPhoneCode(ctx, args.Phone)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "login_by_phone", "使用手机号 + 验证码登录", false, appServer,
		func(ctx context.Context, args phoneLoginArgs) (*MCPToolResult, error) {
			res, err := svc.LoginByPhone(ctx, args.Phone, args.Code)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "delete_cookies", "删除本地 cookies，重置登录状态", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.DeleteCookies(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_user_info", "获取用户资料（昵称、粉丝数等）", true, appServer,
		func(ctx context.Context, args userArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserInfo(ctx, args.User)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_user_posts", "获取用户作品列表（单页，含播放直链）", true, appServer,
		func(ctx context.Context, args userPostsArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserPosts(ctx, args.User, args.MaxCursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_user_all_posts", "自动翻页获取用户全部作品", true, appServer,
		func(ctx context.Context, args userAllPostsArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserAllPosts(ctx, args.User, args.Limit)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_video_detail", "获取作品详情（含无水印播放直链、封面、图文图片）", true, appServer,
		func(ctx context.Context, args videoArgs) (*MCPToolResult, error) {
			res, err := svc.GetVideoDetail(ctx, args.Video)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_video_comments", "获取作品评论（单页）", true, appServer,
		func(ctx context.Context, args videoCommentsArgs) (*MCPToolResult, error) {
			res, err := svc.GetVideoComments(ctx, args.Video, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_all_video_comments", "自动翻页获取作品全部一级评论，可选抓取二级回复", true, appServer,
		func(ctx context.Context, args allVideoCommentsArgs) (*MCPToolResult, error) {
			res, err := svc.GetAllVideoComments(ctx, args.Video, args.Limit, args.IncludeReplies)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_sub_comments", "获取某条评论的回复列表", true, appServer,
		func(ctx context.Context, args subCommentsArgs) (*MCPToolResult, error) {
			res, err := svc.GetSubComments(ctx, args.Video, args.CommentID, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "search_suggest", "搜索联想建议（搜索框下拉）", true, appServer,
		func(ctx context.Context, args suggestArgs) (*MCPToolResult, error) {
			res, err := svc.SearchSuggest(ctx, args.Keyword)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_hot_search_board", "获取抖音热搜榜", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.HotSearchBoard(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "search_challenges", "搜索话题/挑战", true, appServer,
		func(ctx context.Context, args searchVideosArgs) (*MCPToolResult, error) {
			res, err := svc.SearchChallenges(ctx, args.Keyword, args.Offset, "")
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "search_videos", "搜索视频（综合/视频频道，支持排序、发布时间等筛选）", true, appServer,
		func(ctx context.Context, args searchVideosArgs) (*MCPToolResult, error) {
			res, err := svc.SearchVideos(ctx, SearchVideosRequest{
				Keyword: args.Keyword, Offset: args.Offset, Count: args.Count, Channel: args.Channel,
				SortType: args.SortType, PublishTime: args.PublishTime, FilterDuration: args.FilterDuration,
				SearchRange: args.SearchRange, ContentType: args.ContentType,
			})
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "search_users", "搜索用户", true, appServer,
		func(ctx context.Context, args searchUsersArgs) (*MCPToolResult, error) {
			res, err := svc.SearchUsers(ctx, args.Keyword, args.Offset, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "search_lives", "搜索直播间", true, appServer,
		func(ctx context.Context, args searchLivesArgs) (*MCPToolResult, error) {
			res, err := svc.SearchLives(ctx, args.Keyword, args.Offset, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_user_followers", "获取用户粉丝列表", true, appServer,
		func(ctx context.Context, args relationArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserFollowers(ctx, args.User, args.MaxTime, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_user_following", "获取用户关注列表", true, appServer,
		func(ctx context.Context, args relationArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserFollowing(ctx, args.User, args.MaxTime, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_notice_count", "获取通知未读数（评论/@、赞、粉丝等分区）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.NoticeCount(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_notice_detail", "获取单条通知详情", true, appServer,
		func(ctx context.Context, args noticeDetailArgs) (*MCPToolResult, error) {
			res, err := svc.NoticeDetail(ctx, args.NoticeID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "delete_notice", "删除一条通知", false, appServer,
		func(ctx context.Context, args noticeDeleteArgs) (*MCPToolResult, error) {
			res, err := svc.NoticeDelete(ctx, args.ActionType, args.NoticeID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_notices", "获取消息通知列表", true, appServer,
		func(ctx context.Context, args noticesArgs) (*MCPToolResult, error) {
			res, err := svc.GetNotices(ctx, args.NoticeGroup, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_watch_history", "获取「观看历史」（个人主页-观看历史 tab）", true, appServer,
		func(ctx context.Context, args pageCursorArgs) (*MCPToolResult, error) {
			res, err := svc.WatchHistory(ctx, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "clear_watch_history", "清空观看历史", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.ClearWatchHistory(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_watch_later", "获取「稍后再看」列表（个人主页-稍后再看 tab）", true, appServer,
		func(ctx context.Context, args pageCursorArgs) (*MCPToolResult, error) {
			res, err := svc.WatchLater(ctx, args.Offset)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_my_appointments", "获取「我的预约」（个人主页-我的预约 tab）", true, appServer,
		func(ctx context.Context, args appointmentArgs) (*MCPToolResult, error) {
			res, err := svc.Appointments(ctx, args.AppointmentType, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_user_favorites", "获取用户收藏的作品列表", true, appServer,
		func(ctx context.Context, args favoritesArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserFavorites(ctx, args.SecUserID, args.MaxCursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_collect_list", "获取自己的收藏夹列表", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.GetCollectList(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_homefeed", "获取推荐流", true, appServer,
		func(ctx context.Context, args homeFeedArgs) (*MCPToolResult, error) {
			res, err := svc.GetHomeFeed(ctx, args.Count, args.RefreshIndex)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "digg_video", "点赞/取消点赞作品", false, appServer,
		func(ctx context.Context, args diggArgs) (*MCPToolResult, error) {
			diggType := args.DiggType
			if diggType == "" {
				diggType = "1"
			}
			res, err := svc.DiggVideo(ctx, args.AwemeID, diggType)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "post_video_comment", "发表评论或回复评论", false, appServer,
		func(ctx context.Context, args commentArgs) (*MCPToolResult, error) {
			res, err := svc.PostVideoComment(ctx, args.AwemeID, args.Content, args.ReplyID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "collect_video", "收藏/取消收藏作品", false, appServer,
		func(ctx context.Context, args collectArgs) (*MCPToolResult, error) {
			action := args.Action
			if action == "" {
				action = "1"
			}
			res, err := svc.CollectVideo(ctx, args.AwemeID, action)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "move_collect_video", "移动收藏作品到指定收藏夹", false, appServer,
		func(ctx context.Context, args moveCollectArgs) (*MCPToolResult, error) {
			res, err := svc.MoveCollectVideo(ctx, args.AwemeID, args.CollectName, args.CollectID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "remove_collect_video", "从指定收藏夹移除作品", false, appServer,
		func(ctx context.Context, args moveCollectArgs) (*MCPToolResult, error) {
			res, err := svc.RemoveCollectVideo(ctx, args.AwemeID, args.CollectName, args.CollectID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_live_info", "获取直播间信息（房间号、主播、拉流地址）", true, appServer,
		func(ctx context.Context, args liveInfoArgs) (*MCPToolResult, error) {
			res, err := svc.LiveInfo(ctx, args.WebRID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "start_live_listen", "启动直播间实时监听（弹幕/礼物/进场/点赞/关注/房间热度/PK）", false, appServer,
		func(ctx context.Context, args liveInfoArgs) (*MCPToolResult, error) {
			res, err := svc.StartLiveListen(ctx, args.WebRID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "stop_live_listen", "停止直播间实时监听", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			return textResult(svc.StopLiveListen()), nil
		})
	addTool(server, "get_live_events", "获取已收集的直播间事件", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			return textResult(svc.LiveEvents()), nil
		})
	addTool(server, "send_live_comment", "在直播间发送弹幕", false, appServer,
		func(ctx context.Context, args liveCommentArgs) (*MCPToolResult, error) {
			res, err := svc.SendLiveComment(ctx, args.RoomID, args.Content)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "like_live_room", "给直播间点赞", false, appServer,
		func(ctx context.Context, args liveLikeArgs) (*MCPToolResult, error) {
			count := args.Count
			if count == "" {
				count = "1"
			}
			res, err := svc.LikeLiveRoom(ctx, args.RoomID, count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_contribution_rank", "获取直播间贡献榜", true, appServer,
		func(ctx context.Context, args liveRankArgs) (*MCPToolResult, error) {
			res, err := svc.LiveContributionRank(ctx, args.RoomID, args.AnchorID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_pk_rank", "获取直播间 PK 排行榜", true, appServer,
		func(ctx context.Context, args livePKRankArgs) (*MCPToolResult, error) {
			side := args.Side
			if side == "" {
				side = "current"
			}
			res, err := svc.LivePKRank(ctx, args.WebRID, side)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_rank_list", "获取直播间榜单列表", true, appServer,
		func(ctx context.Context, args liveRankArgs) (*MCPToolResult, error) {
			res, err := svc.LiveRankList(ctx, args.RoomID, args.AnchorID, args.SecAnchorID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_production", "获取直播间带货商品列表", true, appServer,
		func(ctx context.Context, args liveProductionArgs) (*MCPToolResult, error) {
			res, err := svc.LiveProduction(ctx, args.PageURL, args.RoomID, args.AuthorID, args.Offset)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_product_comments", "获取商品评论", true, appServer,
		func(ctx context.Context, args productCommentsArgs) (*MCPToolResult, error) {
			res, err := svc.ProductComments(ctx, args.ProductID, args.ShopID, args.Cursor)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "list_conversations", "获取全部会话列表（群聊 + 私聊，含群名称、会话 id、最近消息、未读数 unread 与分类计数 unread_classes）", true, appServer,
		func(ctx context.Context, args listConversationsArgs) (*MCPToolResult, error) {
			convs, err := svc.ListConversations(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]any, 0, len(convs))
			for _, conv := range convs {
				if args.Name != "" && !strings.Contains(strings.ToLower(conv.Name), strings.ToLower(args.Name)) {
					continue
				}
				if args.Type != 0 && int(conv.ConversationType) != args.Type {
					continue
				}
				out = append(out, conv)
			}
			return textResult(out), nil
		})
	addTool(server, "get_conversation_history", "读取群聊/私聊的聊天记录（按会话 id 或群名，如 九号mz5闲聊群）；unread_only=true 时只返回该会话的未读消息", true, appServer,
		func(ctx context.Context, args conversationHistoryArgs) (*MCPToolResult, error) {
			res, err := svc.ConversationHistory(ctx, args.Conversation, args.Cursor, args.Count, args.UnreadOnly)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_conversation_info", "获取会话详情（名称、类型、会话 id、群主、未读数 unread 等）", true, appServer,
		func(ctx context.Context, args conversationRefArgs) (*MCPToolResult, error) {
			res, err := svc.ConversationInfo(ctx, args.Conversation)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_conversation_participants", "获取群成员列表（含昵称、sec_uid）", true, appServer,
		func(ctx context.Context, args conversationParticipantsArgs) (*MCPToolResult, error) {
			res, err := svc.ConversationParticipants(ctx, args.Conversation, args.Offset, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_stranger_conversations", "获取陌生人会话列表（未关注的人发来的私信）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.StrangerConversations(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "mark_conversation_read", "把会话标记为已读", false, appServer,
		func(ctx context.Context, args conversationRefArgs) (*MCPToolResult, error) {
			res, err := svc.MarkConversationRead(ctx, args.Conversation)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_im_user_info", "按 sec_uid 批量查询用户昵称/头像（用于把消息发送者 uid 换成昵称）", true, appServer,
		func(ctx context.Context, args imUserInfoArgs) (*MCPToolResult, error) {
			res, err := svc.IMUserInfo(ctx, args.SecUIDs)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "set_conversation_setting", "置顶聊天 / 消息免打扰（可只传其中一个）", false, appServer,
		func(ctx context.Context, args conversationSettingArgs) (*MCPToolResult, error) {
			if args.Pin == nil && args.Mute == nil {
				return nil, fmt.Errorf("pin 与 mute 至少需要一个")
			}
			res, err := svc.SetConversationSetting(ctx, args.Conversation, args.Pin, args.Mute)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "leave_conversation", "退出群聊（仅群聊可用）", false, appServer,
		func(ctx context.Context, args conversationRefArgs) (*MCPToolResult, error) {
			res, err := svc.LeaveConversation(ctx, args.Conversation)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "share_conversation", "获取群聊分享信息（群名、会话 id、分享链接与文案）", true, appServer,
		func(ctx context.Context, args conversationRefArgs) (*MCPToolResult, error) {
			res, err := svc.ShareConversation(ctx, args.Conversation)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "create_conversation", "创建私信会话", false, appServer,
		func(ctx context.Context, args conversationArgs) (*MCPToolResult, error) {
			res, err := svc.CreateConversation(ctx, args.ToUserID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_conversation_list", "查询私信会话信息", true, appServer,
		func(ctx context.Context, args conversationArgs) (*MCPToolResult, error) {
			res, err := svc.ConversationList(ctx, args.ToUserID, args.ConversationShortID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_dm", "给指定用户发送私信（自动创建会话）", false, appServer,
		func(ctx context.Context, args sendDMArgs) (*MCPToolResult, error) {
			res, err := svc.SendDM(ctx, args.ToUserID, args.Content)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "start_im_listen", "启动私信实时接收（WebSocket）", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.StartIMListen(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "stop_im_listen", "停止私信实时接收", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			return textResult(svc.StopIMListen()), nil
		})
	addTool(server, "get_im_messages", "获取已接收的私信/群聊消息，可按 conversation_id 或 conversation_type(1 私聊/2 群聊) 过滤", true, appServer,
		func(ctx context.Context, args imMessagesArgs) (*MCPToolResult, error) {
			return textResult(svc.IMMessages(args.ConversationID, args.ConversationType)), nil
		})

	addTool(server, "publish_content", "发布抖音图文作品到创作者中心", false, appServer,
		func(ctx context.Context, args publishArgs) (*MCPToolResult, error) {
			res, err := svc.PublishContent(ctx, toPublishImageRequest(args))
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "publish_with_video", "发布抖音视频到创作者中心", false, appServer,
		func(ctx context.Context, args publishVideoArgs) (*MCPToolResult, error) {
			res, err := svc.PublishVideo(ctx, toPublishVideoRequest(args))
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_all_user_followers", "自动翻页获取用户全部粉丝", true, appServer,
		func(ctx context.Context, args allRelationArgs) (*MCPToolResult, error) {
			res, err := svc.GetAllUserFollowers(ctx, args.User, args.Limit)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_all_user_following", "自动翻页获取用户全部关注", true, appServer,
		func(ctx context.Context, args allRelationArgs) (*MCPToolResult, error) {
			res, err := svc.GetAllUserFollowing(ctx, args.User, args.Limit)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_all_notices", "自动翻页获取全部消息通知", true, appServer,
		func(ctx context.Context, args allNoticesArgs) (*MCPToolResult, error) {
			res, err := svc.GetAllNotices(ctx, args.NoticeGroup, args.Limit)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_all_live_production", "自动翻页获取直播间全部带货商品", true, appServer,
		func(ctx context.Context, args liveProductionArgs) (*MCPToolResult, error) {
			res, err := svc.AllLiveProduction(ctx, args.PageURL)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_production_detail", "获取直播间商品详情", true, appServer,
		func(ctx context.Context, args liveProductionDetailArgs) (*MCPToolResult, error) {
			origin := args.OriginType
			if origin == "" {
				origin = "638303"
			}
			res, err := svc.LiveProductionDetail(ctx, args.PageURL, args.PromotionID, origin)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_product_comment_counter", "获取商品评论统计", true, appServer,
		func(ctx context.Context, args productCounterArgs) (*MCPToolResult, error) {
			res, err := svc.ProductCommentCounter(ctx, args.ProductID, args.ShopID, args.StatID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "live_room_enter", "发送直播间进入请求（预热会话）", false, appServer,
		func(ctx context.Context, args liveInfoArgs) (*MCPToolResult, error) {
			res, err := svc.LiveRoomEnter(ctx, args.WebRID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_linkmic_list", "获取直播间连麦列表", true, appServer,
		func(ctx context.Context, args linkmicArgs) (*MCPToolResult, error) {
			res, err := svc.LiveLinkmicList(ctx, args.RoomID, args.ChannelID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_pk_context", "获取直播间 PK 上下文", true, appServer,
		func(ctx context.Context, args pkContextArgs) (*MCPToolResult, error) {
			res, err := svc.LivePKContext(ctx, args.WebRID, args.ChannelID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_thousand_ticket_rank", "获取直播间千票榜", true, appServer,
		func(ctx context.Context, args thousandTicketArgs) (*MCPToolResult, error) {
			res, err := svc.LiveThousandTicketRank(ctx, args.RoomID, args.WebRID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_live_pk_contribution_rank", "获取 PK 贡献榜", true, appServer,
		func(ctx context.Context, args pkContributionArgs) (*MCPToolResult, error) {
			res, err := svc.LivePKContributionRank(ctx, args.ChannelID, args.AnchorID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_dm_media", "发送私信图片/视频/语音/文件（自动创建会话）", false, appServer,
		func(ctx context.Context, args sendDMMediaArgs) (*MCPToolResult, error) {
			res, err := svc.SendDMMedia(ctx, args.ToUserID, args.Kind, args.Path)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_dm_sticker", "发送私信表情包", false, appServer,
		func(ctx context.Context, args sendDMStickerArgs) (*MCPToolResult, error) {
			res, err := svc.SendDMSticker(ctx, args.ToUserID, args.Sticker)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_dm_card", "发送私信卡片消息", false, appServer,
		func(ctx context.Context, args sendDMCardArgs) (*MCPToolResult, error) {
			res, err := svc.SendDMCard(ctx, args.ToUserID, args.Card)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "share_dm_aweme", "分享作品到私信会话", false, appServer,
		func(ctx context.Context, args shareDMAwemeArgs) (*MCPToolResult, error) {
			res, err := svc.ShareDMAweme(ctx, args.ToUserID, args.AwemeID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "share_dm_web", "分享链接到私信会话", false, appServer,
		func(ctx context.Context, args shareDMWebArgs) (*MCPToolResult, error) {
			res, err := svc.ShareDMWeb(ctx, args.ToUserID, args.URL)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "send_dm_user_card", "分享用户名片到私信会话", false, appServer,
		func(ctx context.Context, args sendDMUserCardArgs) (*MCPToolResult, error) {
			res, err := svc.SendDMUserCard(ctx, args.ToUserID, args.SecUID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	// Tab-coverage slices (精选/推荐/关注/朋友/我的 的补齐接口).
	registerMiscFeedsTools(server, appServer)
	registerMiscSocialTools(server, appServer)
	registerMiscProfileTools(server, appServer)
	registerMiscImTools(server, appServer)
	registerMiscHotMusicTools(server, appServer)
	registerMiscEcomTools(server, appServer)
	registerMiscNoticeTools(server, appServer)
	registerIMAdminTools(server, appServer)

	logrus.Infof("Registered %d MCP tools", 83+miscSliceToolCount)
}

// convertToMCPResult converts an internal result into the SDK shape.
func convertToMCPResult(result *MCPToolResult) *mcp.CallToolResult {
	var contents []mcp.Content
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			contents = append(contents, &mcp.TextContent{Text: c.Text})
		case "image":
			contents = append(contents, &mcp.ImageContent{Data: []byte(c.Data), MIMEType: c.MimeType})
		}
	}
	return &mcp.CallToolResult{Content: contents, IsError: result.IsError}
}

// toPublishImageRequest converts MCP args into the client request struct.
func toPublishImageRequest(args publishArgs) douyin.PublishImageRequest {
	return douyin.PublishImageRequest{
		Title:      args.Title,
		Content:    args.Content,
		Images:     args.Images,
		Tags:       args.Tags,
		Visibility: args.Visibility,
	}
}

// toPublishVideoRequest converts MCP args into the client request struct.
func toPublishVideoRequest(args publishVideoArgs) douyin.PublishVideoRequest {
	return douyin.PublishVideoRequest{
		Title:      args.Title,
		Content:    args.Content,
		Video:      args.Video,
		Tags:       args.Tags,
		Visibility: args.Visibility,
	}
}
