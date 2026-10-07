package main

// MCP tool registration for the feeds slice (精选/推荐 tab).

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool argument structs -------------------------------------------------

type channelModuleFeedArgs struct {
	ModuleID          string `json:"module_id,omitempty" jsonschema:"模块号，默认 3003101（实测 精选 17 个分类 tab 发的都是这一个）"`
	Count             string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
	RefreshIndex      string `json:"refresh_index,omitempty" jsonschema:"刷新索引，默认 1"`
	UseLiteType       string `json:"use_lite_type,omitempty" jsonschema:"默认 1（JSON 视频流）；传 2 与浏览器实录一致（protobuf，已解码）"`
	PreItemIDs        string `json:"pre_item_ids,omitempty" jsonschema:"上一页作品 id 列表（逗号分隔），首次留空"`
	PreLogID          string `json:"pre_log_id,omitempty" jsonschema:"上一页日志 id，首次留空"`
	EncodedPreItemIDs string `json:"encoded_pre_item_ids,omitempty" jsonschema:"浏览器混淆后的预加载作品 id（body），首次留空"`
	EncodedPreRoomIDs string `json:"encoded_pre_room_ids,omitempty" jsonschema:"浏览器混淆后的预加载直播间 id（body），首次留空"`
}

type courseCategoryTagsArgs struct {
	TabID string `json:"tab_id,omitempty" jsonschema:"页面 tab，默认 screen_course_page"`
}

type courseCategoryVideosArgs struct {
	TabID     string `json:"tab_id,omitempty" jsonschema:"页面 tab，默认 screen_course_page"`
	Offset    string `json:"offset,omitempty" jsonschema:"偏移，默认 0"`
	Size      string `json:"size,omitempty" jsonschema:"每页条数，默认 6"`
	TagIDList string `json:"tag_id_list,omitempty" jsonschema:"分类标签 id 数组（JSON 串），默认 [0,0,0]"`
	IDList    string `json:"id_list,omitempty" jsonschema:"已展示过的作品 id（逗号分隔），用于翻页去重"`
}

type solutionResourcesArgs struct {
	SpotKeys string `json:"spot_keys,omitempty" jsonschema:"资源位 key，默认 7359502129541449780_douyin_pc_discover_subtab"`
	AppID    string `json:"app_id,omitempty" jsonschema:"应用 id，默认 6383"`
}

type emojiListArgs struct {
	NeedAll string `json:"need_all,omitempty" jsonschema:"是否返回全部表情，默认 true"`
}

type publishHighlightArgs struct {
	HighlightType string `json:"highlight_type,omitempty" jsonschema:"高亮类型，默认 app_publish_and_pc_not_publish"`
}

type mixListCollectionArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"分页游标，默认 0"`
	Count  string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
}

type studyNotesArgs struct {
	Offset      string `json:"offset,omitempty" jsonschema:"分页偏移，默认 0"`
	Count       string `json:"count,omitempty" jsonschema:"每页数量，默认 1"`
	FilterDraft string `json:"filter_draft,omitempty" jsonschema:"是否过滤草稿，默认 true"`
}

// --- Registration ----------------------------------------------------------

func registerMiscFeedsTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_channel_module_feed", "获取精选/推荐 V2 频道模块流（POST /aweme/v2/web/module/feed/）", true, appServer,
		func(ctx context.Context, args channelModuleFeedArgs) (*MCPToolResult, error) {
			res, err := svc.GetChannelModuleFeed(ctx, args.ModuleID, args.Count, args.RefreshIndex, args.UseLiteType, args.PreItemIDs, args.PreLogID, args.EncodedPreItemIDs, args.EncodedPreRoomIDs)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_course_category_videos", "获取精选「公开课」分类的作品列表（唯一有独立接口的分类）", true, appServer,
		func(ctx context.Context, args courseCategoryVideosArgs) (*MCPToolResult, error) {
			res, err := svc.GetCourseCategoryVideos(ctx, args.TabID, args.Offset, args.Size, args.TagIDList, args.IDList)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_course_category_tags", "获取精选页课程分类标签", true, appServer,
		func(ctx context.Context, args courseCategoryTagsArgs) (*MCPToolResult, error) {
			res, err := svc.GetCourseCategoryTags(ctx, args.TabID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_solution_resources", "获取精选页子 tab 资源位内容", true, appServer,
		func(ctx context.Context, args solutionResourcesArgs) (*MCPToolResult, error) {
			res, err := svc.GetSolutionResources(ctx, args.SpotKeys, args.AppID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_multicast_config", "获取 PC 端 multicast 配置下发", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.GetMulticastConfig(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "page_turn_offline", "上报推荐页翻页（page turn offline）", false, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.PageTurnOffline(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_emoji_list", "获取评论表情列表", true, appServer,
		func(ctx context.Context, args emojiListArgs) (*MCPToolResult, error) {
			res, err := svc.GetEmojiList(ctx, args.NeedAll)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_publish_highlight", "获取创作者发布高亮配置", true, appServer,
		func(ctx context.Context, args publishHighlightArgs) (*MCPToolResult, error) {
			res, err := svc.GetPublishHighlight(ctx, args.HighlightType)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_mix_list_collection", "获取合集收藏列表（x-secsdk-web-signature 签名）", true, appServer,
		func(ctx context.Context, args mixListCollectionArgs) (*MCPToolResult, error) {
			res, err := svc.GetMixListCollection(ctx, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_seo_inner_link", "获取精选页 SEO 内链数据", true, appServer,
		func(ctx context.Context, _ any) (*MCPToolResult, error) {
			res, err := svc.GetSEOInnerLink(ctx)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_study_notes", "获取精选页学习 AI 笔记列表", true, appServer,
		func(ctx context.Context, args studyNotesArgs) (*MCPToolResult, error) {
			res, err := svc.GetStudyNotes(ctx, args.Offset, args.Count, args.FilterDraft)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
