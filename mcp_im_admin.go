package main

// 群管理类写操作的 MCP 工具。

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type conversationNameArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id 或名称（群聊）"`
	Name         string `json:"name,omitempty" jsonschema:"新群名；不想改就留空"`
	Desc         string `json:"desc,omitempty" jsonschema:"新群简介，可留空"`
	Notice       string `json:"notice,omitempty" jsonschema:"新群公告，可留空"`
}

type kickParticipantsArgs struct {
	Conversation string  `json:"conversation" jsonschema:"会话 id 或名称（群聊）"`
	UserIDs      []int64 `json:"user_ids" jsonschema:"要移出的成员数字 uid 列表（从 get_conversation_participants 取）"`
}

type recallMessageArgs struct {
	Conversation    string `json:"conversation" jsonschema:"会话 id 或名称"`
	ServerMessageID int64  `json:"server_message_id" jsonschema:"要撤回的消息 server_message_id（从 get_conversation_history 取，服务端限 3 分钟内）"`
}

type deleteMessageArgs struct {
	Conversation string `json:"conversation" jsonschema:"会话 id 或名称"`
	MessageID    int64  `json:"message_id" jsonschema:"要删除的消息 id（从 get_conversation_history 取）"`
}

// registerIMAdminTools 注册群管理/消息管理写操作。
func registerIMAdminTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "update_conversation_name", "修改群名/群简介/群公告（仅群聊，需要群主或管理员权限）", false, appServer,
		func(ctx context.Context, args conversationNameArgs) (*MCPToolResult, error) {
			res, err := svc.SetConversationName(ctx, args.Conversation, args.Name, args.Desc, args.Notice)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "kick_conversation_participants", "把成员移出群聊（仅群聊，需要群主或管理员权限）", false, appServer,
		func(ctx context.Context, args kickParticipantsArgs) (*MCPToolResult, error) {
			res, err := svc.KickConversationParticipants(ctx, args.Conversation, args.UserIDs)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "recall_message", "撤回一条自己发出的私信/群消息（服务端限 3 分钟内）", false, appServer,
		func(ctx context.Context, args recallMessageArgs) (*MCPToolResult, error) {
			res, err := svc.RecallMessage(ctx, args.Conversation, args.ServerMessageID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
	addTool(server, "delete_message", "删除一条消息（只影响自己侧）", false, appServer,
		func(ctx context.Context, args deleteMessageArgs) (*MCPToolResult, error) {
			res, err := svc.DeleteMessage(ctx, args.Conversation, args.MessageID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
