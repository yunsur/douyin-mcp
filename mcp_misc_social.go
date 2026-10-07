package main

// 切片 social 的 MCP 工具注册。集成由父 agent 在 mcp_server.go 调用
// registerMiscSocialTools(server, appServer)。

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- 参数结构体 -----------------------------------------------------------

type followFeedArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"分页游标，首次 0 或留空"`
	Count  string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
}

type familiarFeedArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"分页游标，首次 0 或留空"`
	Count  string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
}

type familiarRecommendFeedArgs struct {
	SecUserID string `json:"sec_user_id,omitempty" jsonschema:"用户 sec_user_id，留空取当前登录用户"`
	MaxCursor string `json:"max_cursor,omitempty" jsonschema:"分页游标，首次 0 或留空"`
	MinCursor string `json:"min_cursor,omitempty" jsonschema:"最小游标，默认 0"`
	Count     string `json:"count,omitempty" jsonschema:"数量，默认 18"`
}

type followLiveFeedArgs struct {
	Scene string `json:"scene,omitempty" jsonschema:"场景，默认 aweme_pc_follow_top"`
}

type reportHistoryWriteArgs struct {
	AuthorID string `json:"author_id" jsonschema:"作者 uid"`
	AwemeID  string `json:"aweme_id" jsonschema:"作品 aweme_id"`
}

type reportAwemeStatsArgs struct {
	ItemID    string `json:"item_id" jsonschema:"作品 aweme_id"`
	AwemeType string `json:"aweme_type,omitempty" jsonschema:"作品类型，默认 0"`
	PlayDelta string `json:"play_delta,omitempty" jsonschema:"播放增量，默认 1"`
	Source    string `json:"source,omitempty" jsonschema:"来源，默认 0"`
}

type markFollowingSeenArgs struct {
	ItemIDList string `json:"item_id_list" jsonschema:"已看过作品 id 列表，逗号分隔"`
	Type       string `json:"type,omitempty" jsonschema:"类型，默认 1"`
}

type danmakuArgs struct {
	ItemID    string `json:"item_id" jsonschema:"作品 aweme_id"`
	StartTime string `json:"start_time,omitempty" jsonschema:"起始毫秒，默认 0"`
	EndTime   string `json:"end_time,omitempty" jsonschema:"结束毫秒（通常为视频时长）"`
	Duration  string `json:"duration,omitempty" jsonschema:"视频时长毫秒"`
	AuthToken string `json:"authentication_token,omitempty" jsonschema:"弹幕鉴权 token（来自 get_danmaku_conf 或视频详情）"`
}

type danmakuConfArgs struct {
	HardwareConcurrency string `json:"hardware_concurrency,omitempty" jsonschema:"CPU 核数，默认 10"`
}

type reportSeriesWatchArgs struct {
	ItemID   string `json:"item_id" jsonschema:"作品 aweme_id（系列中的一集）"`
	SeriesID string `json:"series_id,omitempty" jsonschema:"系列/合集 id"`
	Episode  string `json:"episode,omitempty" jsonschema:"集数，从 1 开始，默认 1"`
}

// --- 工具注册 -------------------------------------------------------------

// registerMiscSocialTools 注册关注/朋友 tab 的 11 个 MCP 工具。
func registerMiscSocialTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_follow_feed", "获取「关注」tab 视频流（GET /aweme/v1/web/follow/feed/）", true, appServer,
		func(ctx context.Context, args followFeedArgs) (*MCPToolResult, error) {
			res, err := svc.FollowFeed(ctx, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_familiar_feed", "获取「朋友」tab 视频流（POST /aweme/v1/web/familiar/feed/）", true, appServer,
		func(ctx context.Context, args familiarFeedArgs) (*MCPToolResult, error) {
			res, err := svc.FamiliarFeed(ctx, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_familiar_recommend_feed", "获取「朋友」推荐卡片（GET /aweme/v1/web/familiar/recommend/feed/）", true, appServer,
		func(ctx context.Context, args familiarRecommendFeedArgs) (*MCPToolResult, error) {
			res, err := svc.FamiliarRecommendFeed(ctx, args.SecUserID, args.MaxCursor, args.MinCursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_follow_live_top", "获取「关注」页顶部直播卡片（POST /webcast/feed/follow_top/）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.FollowLiveTop(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_follow_live_feed", "获取「关注」页直播流（GET /webcast/web/feed/follow/）", true, appServer,
		func(ctx context.Context, args followLiveFeedArgs) (*MCPToolResult, error) {
			res, err := svc.FollowLiveFeed(ctx, args.Scene)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "report_history_write", "上报观看历史写入（POST /aweme/v1/web/history/write/）", false, appServer,
		func(ctx context.Context, args reportHistoryWriteArgs) (*MCPToolResult, error) {
			res, err := svc.ReportHistoryWrite(ctx, args.AuthorID, args.AwemeID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "report_aweme_stats", "上报作品播放统计（POST /aweme/v2/web/aweme/stats/）", false, appServer,
		func(ctx context.Context, args reportAwemeStatsArgs) (*MCPToolResult, error) {
			res, err := svc.ReportAwemeStats(ctx, args.ItemID, args.AwemeType, args.PlayDelta, args.Source)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "mark_following_seen", "标记关注流已看作品（GET /aweme/v1/following/list/item/seen/）", false, appServer,
		func(ctx context.Context, args markFollowingSeenArgs) (*MCPToolResult, error) {
			res, err := svc.MarkFollowingSeen(ctx, args.ItemIDList, args.Type)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_danmaku", "获取作品弹幕（GET /aweme/v1/web/danmaku/get_v2/）", true, appServer,
		func(ctx context.Context, args danmakuArgs) (*MCPToolResult, error) {
			res, err := svc.GetDanmaku(ctx, args.ItemID, args.StartTime, args.EndTime, args.Duration, args.AuthToken)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_danmaku_conf", "获取弹幕鉴权配置（GET /aweme/v1/web/danmaku/conf/get/）", true, appServer,
		func(ctx context.Context, args danmakuConfArgs) (*MCPToolResult, error) {
			res, err := svc.GetDanmakuConf(ctx, args.HardwareConcurrency)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "report_series_watch", "上报系列/合集观看进度（GET /aweme/v1/web/series/watch/record/）", false, appServer,
		func(ctx context.Context, args reportSeriesWatchArgs) (*MCPToolResult, error) {
			res, err := svc.ReportSeriesWatch(ctx, args.ItemID, args.SeriesID, args.Episode)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
