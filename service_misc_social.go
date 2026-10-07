package main

// 切片 social（关注 / 朋友 tab）的服务层薄封装：仅透传必要默认值。

import (
	"context"
)

// FollowFeed 拉取「关注」tab 视频流。
func (s *DouyinService) FollowFeed(ctx context.Context, cursor, count string) (map[string]any, error) {
	return s.Client().GetFollowFeed(ctx, cursor, count)
}

// FamiliarFeed 拉取「朋友」tab 视频流。
func (s *DouyinService) FamiliarFeed(ctx context.Context, cursor, count string) (map[string]any, error) {
	return s.Client().GetFamiliarFeed(ctx, cursor, count)
}

// FamiliarRecommendFeed 拉取「朋友」推荐卡片，secUserID 留空取当前用户。
func (s *DouyinService) FamiliarRecommendFeed(ctx context.Context, secUserID, maxCursor, minCursor, count string) (map[string]any, error) {
	return s.Client().GetFamiliarRecommendFeed(ctx, secUserID, maxCursor, minCursor, count)
}

// FollowLiveTop 拉取「关注」页顶部直播卡片。
func (s *DouyinService) FollowLiveTop(ctx context.Context) (map[string]any, error) {
	return s.Client().GetFollowLiveTop(ctx)
}

// FollowLiveFeed 拉取「关注」页直播流。
func (s *DouyinService) FollowLiveFeed(ctx context.Context, scene string) (map[string]any, error) {
	return s.Client().GetFollowLiveFeed(ctx, scene)
}

// ReportHistoryWrite 上报一次观看历史写入。
func (s *DouyinService) ReportHistoryWrite(ctx context.Context, authorID, awemeID string) (map[string]any, error) {
	return s.Client().ReportHistoryWrite(ctx, authorID, awemeID)
}

// ReportAwemeStats 上报作品播放统计。
func (s *DouyinService) ReportAwemeStats(ctx context.Context, itemID, awemeType, playDelta, source string) (map[string]any, error) {
	return s.Client().ReportAwemeStats(ctx, itemID, awemeType, playDelta, source)
}

// MarkFollowingSeen 标记关注流已看作品。
func (s *DouyinService) MarkFollowingSeen(ctx context.Context, itemIDList, typ string) (map[string]any, error) {
	return s.Client().MarkFollowingSeen(ctx, itemIDList, typ)
}

// GetDanmaku 拉取作品弹幕。
func (s *DouyinService) GetDanmaku(ctx context.Context, itemID, startTime, endTime, duration, authToken string) (map[string]any, error) {
	return s.Client().GetDanmaku(ctx, itemID, startTime, endTime, duration, authToken)
}

// GetDanmakuConf 获取弹幕鉴权配置。
func (s *DouyinService) GetDanmakuConf(ctx context.Context, hardwareConcurrency string) (map[string]any, error) {
	return s.Client().GetDanmakuConf(ctx, hardwareConcurrency)
}

// ReportSeriesWatch 上报系列/合集观看进度。
func (s *DouyinService) ReportSeriesWatch(ctx context.Context, itemID, seriesID, episode string) (map[string]any, error) {
	return s.Client().ReportSeriesWatch(ctx, itemID, seriesID, episode)
}
