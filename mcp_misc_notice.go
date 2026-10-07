package main

// 点赞通知的「谁赞了我」工具（notice/digg/list）。

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type noticeDiggListArgs struct {
	NoticeID string `json:"notice_id" jsonschema:"通知 ID（取 get_notices 结果里的 nid_str，接口参数名固定为 notice_id）"`
	Count    string `json:"count,omitempty" jsonschema:"每页数量，默认 20"`
	MaxTime  string `json:"max_time,omitempty" jsonschema:"分页游标（毫秒时间戳），首次 0 或留空"`
	MinTime  string `json:"min_time,omitempty" jsonschema:"分页下界，首次 0 或留空"`
}

// registerMiscNoticeTools 注册通知相关的补充工具。
func registerMiscNoticeTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_notice_digg_list", "获取某条点赞通知下的点赞用户列表（notice_id 用 get_notices 的 nid_str）", true, appServer,
		func(ctx context.Context, args noticeDiggListArgs) (*MCPToolResult, error) {
			res, err := svc.NoticeDiggList(ctx, args.NoticeID, args.Count, args.MaxTime, args.MinTime)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
