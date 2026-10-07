package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 「我的」tab 的 MCP 工具参数。

type profileUserDashboardArgs struct {
	UserID string `json:"user_id,omitempty" jsonschema:"目标用户数字 UID，留空则查询自己"`
}

type profileUserSettingsArgs struct {
	Source                  string `json:"source,omitempty" jsonschema:"www(默认，主站 /aweme/v1/web/get/user/settings/) | hj(www-hj /aweme/v1/web/user/settings/)"`
	IsFetchFrequencyControl string `json:"is_fetch_frequency_control,omitempty" jsonschema:"仅 hj 版本可选：是否拉取频率控制配置"`
	HasLocalCache           string `json:"has_local_cache,omitempty" jsonschema:"仅 hj 版本可选：是否有本地缓存"`
	RequestSource           string `json:"request_source,omitempty" jsonschema:"仅 hj 版本可选：请求来源标记"`
}

type profileCustomSettingsArgs struct {
	SettingTypes string `json:"setting_types,omitempty" jsonschema:"设置类型，默认 1"`
}

type profileCollectedAwemesArgs struct {
	Count  string `json:"count,omitempty" jsonschema:"每页数量，默认 10"`
	Cursor string `json:"cursor,omitempty" jsonschema:"分页游标，首次传 0 或留空"`
}

type profileBindingSubjectArgs struct {
	AccountUID string `json:"account_uid,omitempty" jsonschema:"账号 UID，留空则查询自己"`
}

// registerMiscProfileTools 注册「我的」tab（资料/设置/收藏/百科）相关 MCP 工具。
func registerMiscProfileTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_my_profile", "获取登录用户自己的资料（我的 tab）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.GetMyProfile(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_user_dashboard", "获取用户数据面板（我的 tab），留空 user_id 查询自己", true, appServer,
		func(ctx context.Context, args profileUserDashboardArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserDashboard(ctx, args.UserID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_social_count", "获取登录用户的社交计数（关注/粉丝/获赞等，我的 tab）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.GetSocialCount(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_user_settings", "获取用户设置（我的-设置 tab），source 可选 www(默认)/hj", true, appServer,
		func(ctx context.Context, args profileUserSettingsArgs) (*MCPToolResult, error) {
			res, err := svc.GetUserSettings(ctx, args.Source, args.IsFetchFrequencyControl, args.HasLocalCache, args.RequestSource)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_custom_settings", "获取自定义设置项（我的-设置 tab），默认 setting_types=1", true, appServer,
		func(ctx context.Context, args profileCustomSettingsArgs) (*MCPToolResult, error) {
			res, err := svc.GetCustomSettings(ctx, args.SettingTypes)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "get_collected_awemes", "获取「收藏」tab 的收藏作品列表（我的 tab）", true, appServer,
		func(ctx context.Context, args profileCollectedAwemesArgs) (*MCPToolResult, error) {
			res, err := svc.GetCollectedAwemes(ctx, args.Count, args.Cursor)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "baike_check_worldbook", "查询当前账号是否有 IP 百科词条访问权限（我的-百科）", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.BaikeCheckWorldbook(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})

	addTool(server, "baike_binding_subject", "查询账号绑定的百科主体（我的-百科），留空 account_uid 查询自己", true, appServer,
		func(ctx context.Context, args profileBindingSubjectArgs) (*MCPToolResult, error) {
			res, err := svc.BaikeBindingSubject(ctx, args.AccountUID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
