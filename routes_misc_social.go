package main

// 切片 social 的 HTTP 路由。集成由父 agent 在 routes.go 里调用
// registerMiscSocialRoutes(api, appServer)。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// --- 请求体 ---------------------------------------------------------------

type miscSocialPageRequest struct {
	Cursor string `json:"cursor"`
	Count  string `json:"count"`
}

type miscSocialFamiliarRecommendRequest struct {
	SecUserID string `json:"sec_user_id"`
	MaxCursor string `json:"max_cursor"`
	MinCursor string `json:"min_cursor"`
	Count     string `json:"count"`
}

type miscSocialLiveFeedRequest struct {
	Scene string `json:"scene"`
}

type miscSocialHistoryWriteRequest struct {
	AuthorID string `json:"author_id"`
	AwemeID  string `json:"aweme_id" binding:"required"`
}

type miscSocialAwemeStatsRequest struct {
	ItemID    string `json:"item_id" binding:"required"`
	AwemeType string `json:"aweme_type"`
	PlayDelta string `json:"play_delta"`
	Source    string `json:"source"`
}

type miscSocialMarkSeenRequest struct {
	ItemIDList string `json:"item_id_list" binding:"required"`
	Type       string `json:"type"`
}

type miscSocialDanmakuRequest struct {
	ItemID    string `json:"item_id" binding:"required"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Duration  string `json:"duration"`
	AuthToken string `json:"authentication_token"`
}

type miscSocialDanmakuConfRequest struct {
	HardwareConcurrency string `json:"hardware_concurrency"`
}

type miscSocialSeriesWatchRequest struct {
	ItemID   string `json:"item_id" binding:"required"`
	SeriesID string `json:"series_id"`
	Episode  string `json:"episode"`
}

// registerMiscSocialRoutes 注册关注/朋友 tab 的 11 个 HTTP 路由。
func registerMiscSocialRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/misc/social/follow/feed", appServer.miscSocialFollowFeedHandler)
	api.POST("/misc/social/familiar/feed", appServer.miscSocialFamiliarFeedHandler)
	api.POST("/misc/social/familiar/recommend/feed", appServer.miscSocialFamiliarRecommendFeedHandler)
	api.POST("/misc/social/follow/live/top", appServer.miscSocialFollowLiveTopHandler)
	api.POST("/misc/social/follow/live/feed", appServer.miscSocialFollowLiveFeedHandler)
	api.POST("/misc/social/history/write", appServer.miscSocialHistoryWriteHandler)
	api.POST("/misc/social/aweme/stats", appServer.miscSocialAwemeStatsHandler)
	api.POST("/misc/social/following/seen", appServer.miscSocialMarkFollowingSeenHandler)
	api.POST("/misc/social/danmaku", appServer.miscSocialDanmakuHandler)
	api.POST("/misc/social/danmaku/conf", appServer.miscSocialDanmakuConfHandler)
	api.POST("/misc/social/series/watch", appServer.miscSocialSeriesWatchHandler)
}

// --- 处理器 ---------------------------------------------------------------

func (s *AppServer) miscSocialFollowFeedHandler(c *gin.Context) {
	var req miscSocialPageRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.FollowFeed(c.Request.Context(), req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FOLLOW_FEED_FAILED", "获取关注流失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取关注流成功")
}

func (s *AppServer) miscSocialFamiliarFeedHandler(c *gin.Context) {
	var req miscSocialPageRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.FamiliarFeed(c.Request.Context(), req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAMILIAR_FEED_FAILED", "获取朋友流失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取朋友流成功")
}

func (s *AppServer) miscSocialFamiliarRecommendFeedHandler(c *gin.Context) {
	var req miscSocialFamiliarRecommendRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.FamiliarRecommendFeed(c.Request.Context(), req.SecUserID, req.MaxCursor, req.MinCursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAMILIAR_RECOMMEND_FEED_FAILED", "获取朋友推荐失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取朋友推荐成功")
}

func (s *AppServer) miscSocialFollowLiveTopHandler(c *gin.Context) {
	res, err := s.service.FollowLiveTop(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FOLLOW_LIVE_TOP_FAILED", "获取关注页直播卡片失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取关注页直播卡片成功")
}

func (s *AppServer) miscSocialFollowLiveFeedHandler(c *gin.Context) {
	var req miscSocialLiveFeedRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.FollowLiveFeed(c.Request.Context(), req.Scene)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FOLLOW_LIVE_FEED_FAILED", "获取关注页直播流失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取关注页直播流成功")
}

func (s *AppServer) miscSocialHistoryWriteHandler(c *gin.Context) {
	var req miscSocialHistoryWriteRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ReportHistoryWrite(c.Request.Context(), req.AuthorID, req.AwemeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "HISTORY_WRITE_FAILED", "上报观看历史失败", err.Error())
		return
	}
	respondSuccess(c, res, "上报观看历史成功")
}

func (s *AppServer) miscSocialAwemeStatsHandler(c *gin.Context) {
	var req miscSocialAwemeStatsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ReportAwemeStats(c.Request.Context(), req.ItemID, req.AwemeType, req.PlayDelta, req.Source)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "AWEME_STATS_FAILED", "上报作品统计失败", err.Error())
		return
	}
	respondSuccess(c, res, "上报作品统计成功")
}

func (s *AppServer) miscSocialMarkFollowingSeenHandler(c *gin.Context) {
	var req miscSocialMarkSeenRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.MarkFollowingSeen(c.Request.Context(), req.ItemIDList, req.Type)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FOLLOWING_SEEN_FAILED", "标记已看作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "标记已看作品成功")
}

func (s *AppServer) miscSocialDanmakuHandler(c *gin.Context) {
	var req miscSocialDanmakuRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetDanmaku(c.Request.Context(), req.ItemID, req.StartTime, req.EndTime, req.Duration, req.AuthToken)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DANMAKU_FAILED", "获取弹幕失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取弹幕成功")
}

func (s *AppServer) miscSocialDanmakuConfHandler(c *gin.Context) {
	var req miscSocialDanmakuConfRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetDanmakuConf(c.Request.Context(), req.HardwareConcurrency)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DANMAKU_CONF_FAILED", "获取弹幕配置失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取弹幕配置成功")
}

func (s *AppServer) miscSocialSeriesWatchHandler(c *gin.Context) {
	var req miscSocialSeriesWatchRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ReportSeriesWatch(c.Request.Context(), req.ItemID, req.SeriesID, req.Episode)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SERIES_WATCH_FAILED", "上报系列观看失败", err.Error())
		return
	}
	respondSuccess(c, res, "上报系列观看成功")
}
