package main

// IM 消息面板（misc）服务层封装：仅透传到 douyin.Client 对应方法。

import (
	"context"
)

// IMGetSpotlightRelation 获取消息面板好友/关系列表。
func (s *DouyinService) IMGetSpotlightRelation(ctx context.Context, count, maxTime string) (map[string]any, error) {
	return s.Client().IMGetSpotlightRelation(ctx, count, maxTime)
}

// IMGetActiveStatus 批量查询会话/用户在线状态。
func (s *DouyinService) IMGetActiveStatus(ctx context.Context, convIDs, secUIDs []string) (map[string]any, error) {
	return s.Client().IMGetActiveStatus(ctx, convIDs, secUIDs)
}

// IMActiveHeartbeat 上报在线心跳。
func (s *DouyinService) IMActiveHeartbeat(ctx context.Context, newUserLogin string) (map[string]any, error) {
	return s.Client().IMActiveHeartbeat(ctx, newUserLogin)
}

// IMGetActiveConfig 获取在线状态配置。
func (s *DouyinService) IMGetActiveConfig(ctx context.Context) (map[string]any, error) {
	return s.Client().IMGetActiveConfig(ctx)
}

// IMGetStrategyConfig 获取 IM 策略配置。
func (s *DouyinService) IMGetStrategyConfig(ctx context.Context, scenes string) (map[string]any, error) {
	return s.Client().IMGetStrategyConfig(ctx, scenes)
}

// IMGetResources 获取消息面板资源聚合列表。
func (s *DouyinService) IMGetResources(ctx context.Context, scenes, customCursor, customLimit string) (map[string]any, error) {
	return s.Client().IMGetResources(ctx, scenes, customCursor, customLimit)
}

// IMGetEmoticonTrending 获取热门表情列表。
func (s *DouyinService) IMGetEmoticonTrending(ctx context.Context, cursor, count, groupID string) (map[string]any, error) {
	return s.Client().IMGetEmoticonTrending(ctx, cursor, count, groupID)
}

// IMGetFeedbackEntrance 获取在线反馈入口。
func (s *DouyinService) IMGetFeedbackEntrance(ctx context.Context, entrance string) (map[string]any, error) {
	return s.Client().IMGetFeedbackEntrance(ctx, entrance)
}

// IMPullMessages 通过 IM 协议增量拉取消息（cmd 2048）。
func (s *DouyinService) IMPullMessages(ctx context.Context, cursor, timestamp int64) (map[string]any, error) {
	return s.Client().IMPullMessages(ctx, cursor, timestamp)
}
