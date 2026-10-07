package main

// 群管理类写操作：改群名/公告、移除群成员、撤回消息、删除消息。
// 这些操作都会真实改动会话状态，调用前请确认目标会话。

import (
	"context"
	"fmt"
	"strings"

	"github.com/yunsur/douyin-mcp/douyin"
)

// douyinCoreInfo 把非空的字段转成指针选项（只有传了的字段才会写）。
func douyinCoreInfo(name, desc, notice string) douyin.IMCoreInfoOptions {
	var opts douyin.IMCoreInfoOptions
	if strings.TrimSpace(name) != "" {
		opts.Name = new(name)
	}
	if strings.TrimSpace(desc) != "" {
		opts.Desc = new(desc)
	}
	if strings.TrimSpace(notice) != "" {
		opts.Notice = new(notice)
	}
	return opts
}

// SetConversationName 改群名（也可以传 desc/notice 一起改）。
func (s *DouyinService) SetConversationName(ctx context.Context, idOrName, name, desc, notice string) (map[string]any, error) {
	if strings.TrimSpace(name) == "" && strings.TrimSpace(desc) == "" && strings.TrimSpace(notice) == "" {
		return nil, fmt.Errorf("name/desc/notice 至少要传一个")
	}
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	if conv.ConversationType != 2 {
		return nil, fmt.Errorf("只有群聊可以改群名/公告（当前会话 type=%d）", conv.ConversationType)
	}
	opts := douyinCoreInfo(name, desc, notice)
	res, err := s.Client().IMSetConversationCoreInfo(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, opts)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["conversation_name"] = conv.Name
	if name != "" {
		res["name"] = name
	}
	if desc != "" {
		res["desc"] = desc
	}
	if notice != "" {
		res["notice"] = notice
	}
	return res, nil
}

// KickConversationParticipants 把成员踢出群（群主/管理员权限）。
func (s *DouyinService) KickConversationParticipants(ctx context.Context, idOrName string, uids []int64) (map[string]any, error) {
	if len(uids) == 0 {
		return nil, fmt.Errorf("uids 不能为空")
	}
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	if conv.ConversationType != 2 {
		return nil, fmt.Errorf("只有群聊可以移除成员（当前会话 type=%d）", conv.ConversationType)
	}
	res, err := s.Client().IMRemoveParticipants(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, uids)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["conversation_name"] = conv.Name
	res["removed"] = uids
	return res, nil
}

// RecallMessage 撤回一条自己发出的消息（服务端 3 分钟内限制）。
func (s *DouyinService) RecallMessage(ctx context.Context, idOrName string, serverMessageID int64) (map[string]any, error) {
	if serverMessageID == 0 {
		return nil, fmt.Errorf("server_message_id 不能为空（取 get_conversation_history 返回的 server_message_id）")
	}
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().IMRecallMessage(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, serverMessageID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["server_message_id"] = serverMessageID
	res["recalled"] = true
	return res, nil
}

// DeleteMessage 删除一条消息（只影响自己侧）。
func (s *DouyinService) DeleteMessage(ctx context.Context, idOrName string, messageID int64) (map[string]any, error) {
	if messageID == 0 {
		return nil, fmt.Errorf("message_id 不能为空（取 get_conversation_history 返回的 server_message_id）")
	}
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().IMDeleteMessage(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, messageID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["message_id"] = messageID
	res["deleted"] = true
	return res, nil
}
