package main

// IM 消息面板（misc）MCP 工具注册。由 registerTools 统一调用。

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool argument structs -------------------------------------------------

type miscImSpotlightArgs struct {
	Count   string `json:"count,omitempty" jsonschema:"返回数量，默认 50"`
	MaxTime string `json:"max_time,omitempty" jsonschema:"翻页游标（毫秒时间戳），首页传 0（默认）"`
}

type miscImActiveStatusArgs struct {
	ConvIDs []string `json:"conv_ids,omitempty" jsonschema:"会话 id 列表（JSON 数组，可为空）"`
	SecUIDs []string `json:"sec_user_ids,omitempty" jsonschema:"要查询在线状态用户的 sec_uid 列表"`
}

type miscImHeartbeatArgs struct {
	NewUserLogin string `json:"new_user_login,omitempty" jsonschema:"是否新用户登录，默认 0"`
}

type miscImStrategyArgs struct {
	Scenes string `json:"scenes,omitempty" jsonschema:"场景 JSON 数组串，默认 [\"interactive_resources\"]"`
}

type miscImResourcesArgs struct {
	Scenes       string `json:"scenes,omitempty" jsonschema:"资源场景，默认 CUSTOM_STICKER_PAGE"`
	CustomCursor string `json:"custom_cursor,omitempty" jsonschema:"翻页游标，默认 0"`
	CustomLimit  string `json:"custom_limit,omitempty" jsonschema:"每页数量，默认 50"`
}

type miscImEmoticonArgs struct {
	Cursor  string `json:"cursor,omitempty" jsonschema:"翻页游标，默认 0"`
	Count   string `json:"count,omitempty" jsonschema:"返回数量，默认 50"`
	GroupID string `json:"group_id,omitempty" jsonschema:"表情分组 id，默认 1"`
}

type miscImFeedbackArgs struct {
	Entrance string `json:"entrance,omitempty" jsonschema:"反馈入口标识，默认 IM6383-3586"`
}

type miscImPullArgs struct {
	Cursor    int64 `json:"cursor,omitempty" jsonschema:"拉取游标（微秒时间戳），0 表示当前时间"`
	Timestamp int64 `json:"timestamp,omitempty" jsonschema:"请求时间戳（微秒），0 表示当前时间"`
}

// registerMiscImTools 注册 IM 消息面板相关的全部 MCP 工具。
func registerMiscImTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_im_spotlight_relation", "获取消息面板好友/关系列表（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImSpotlightArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetSpotlightRelation(ctx, args.Count, args.MaxTime)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_active_status", "批量查询会话/用户的在线状态（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImActiveStatusArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetActiveStatus(ctx, args.ConvIDs, args.SecUIDs)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "im_active_heartbeat", "上报在线心跳（消息 tab）", false, appServer,
		func(ctx context.Context, args miscImHeartbeatArgs) (*MCPToolResult, error) {
			res, err := svc.IMActiveHeartbeat(ctx, args.NewUserLogin)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_active_config", "获取在线状态配置（消息 tab）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.IMGetActiveConfig(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_strategy_config", "获取 IM 策略配置（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImStrategyArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetStrategyConfig(ctx, args.Scenes)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_resources", "获取消息面板资源聚合列表（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImResourcesArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetResources(ctx, args.Scenes, args.CustomCursor, args.CustomLimit)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_emoticon_trending", "获取热门表情列表（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImEmoticonArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetEmoticonTrending(ctx, args.Cursor, args.Count, args.GroupID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_im_feedback_entrance", "获取在线反馈入口（消息 tab）", true, appServer,
		func(ctx context.Context, args miscImFeedbackArgs) (*MCPToolResult, error) {
			res, err := svc.IMGetFeedbackEntrance(ctx, args.Entrance)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "pull_im_messages", "通过 IM 协议增量拉取消息（protobuf，cmd 2048）", true, appServer,
		func(ctx context.Context, args miscImPullArgs) (*MCPToolResult, error) {
			res, err := svc.IMPullMessages(ctx, args.Cursor, args.Timestamp)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
