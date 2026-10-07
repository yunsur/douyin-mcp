package main

// MCP tool registration for the hot-search-video + music-page endpoints
// (热搜词视频 / 音乐页).

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool argument structs -------------------------------------------------

type hotSearchVideosArgs struct {
	Hotword    string `json:"hotword,omitempty" jsonschema:"热搜词（必填），取热搜榜条目里的 word"`
	SentenceID string `json:"sentence_id,omitempty" jsonschema:"热搜条目 id（必填），取热搜榜条目里的 sentence_id"`
	Offset     string `json:"offset,omitempty" jsonschema:"偏移，默认 0（服务端参数名是拼错的 offest，此处按惯例写 offset）"`
	Count      string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
	EntryName  string `json:"entry_name,omitempty" jsonschema:"入口名，默认 pc_web"`
}

type musicAwemeArgs struct {
	MusicID string `json:"music_id,omitempty" jsonschema:"音乐 id（必填），如 7693484944814803763"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"分页游标，默认 0"`
	Count   string `json:"count,omitempty" jsonschema:"每页数量，默认 12"`
}

type musicDetailArgs struct {
	MusicID string `json:"music_id,omitempty" jsonschema:"音乐 id（必填）"`
	Scene   string `json:"scene,omitempty" jsonschema:"场景，默认 1（与 PC 端 bundle 一致）"`
}

type musicCollectionArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"分页游标，默认 0"`
	Count  string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
}

type collectMusicArgs struct {
	MusicID string `json:"music_id,omitempty" jsonschema:"音乐 id（必填）"`
	Type    string `json:"type,omitempty" jsonschema:"类型，默认与 action 同值（1=收藏，0=取消）"`
	Action  string `json:"action,omitempty" jsonschema:"动作，默认 1：1=收藏，0=取消收藏"`
}

// --- Registration ----------------------------------------------------------

func registerMiscHotMusicTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_hot_search_videos", "获取热搜词下的视频列表（GET www-hj.douyin.com/aweme/v1/web/hot/search/video/list/）", true, appServer,
		func(ctx context.Context, args hotSearchVideosArgs) (*MCPToolResult, error) {
			res, err := svc.GetHotSearchVideos(ctx, args.Hotword, args.SentenceID, args.Offset, args.Count, args.EntryName)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_music_aweme", "获取某音乐下的视频列表（GET /aweme/v1/web/music/aweme/，webSign 签名）", true, appServer,
		func(ctx context.Context, args musicAwemeArgs) (*MCPToolResult, error) {
			res, err := svc.GetMusicAweme(ctx, args.MusicID, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_music_detail", "获取音乐详情（GET /aweme/v1/web/music/detail/）", true, appServer,
		func(ctx context.Context, args musicDetailArgs) (*MCPToolResult, error) {
			res, err := svc.GetMusicDetail(ctx, args.MusicID, args.Scene)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "get_music_collection", "获取我收藏的音乐列表（GET /aweme/v1/web/music/listcollection/）", true, appServer,
		func(ctx context.Context, args musicCollectionArgs) (*MCPToolResult, error) {
			res, err := svc.GetMusicCollection(ctx, args.Cursor, args.Count)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "collect_music", "收藏/取消收藏音乐（POST /aweme/v1/web/music/collect/，写操作）", false, appServer,
		func(ctx context.Context, args collectMusicArgs) (*MCPToolResult, error) {
			res, err := svc.CollectMusic(ctx, args.MusicID, args.Type, args.Action)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
