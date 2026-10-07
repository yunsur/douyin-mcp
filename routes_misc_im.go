package main

// IM 消息面板（misc）HTTP 路由注册。由 setupRoutes 在 /api/v1 分组内调用。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// --- Request payloads ------------------------------------------------------

type miscImSpotlightRequest struct {
	Count   string `json:"count"`
	MaxTime string `json:"max_time"`
}

type miscImActiveStatusRequest struct {
	ConvIDs []string `json:"conv_ids"`
	SecUIDs []string `json:"sec_user_ids"`
}

type miscImHeartbeatRequest struct {
	NewUserLogin string `json:"new_user_login"`
}

type miscImStrategyRequest struct {
	Scenes string `json:"scenes"`
}

type miscImResourcesRequest struct {
	Scenes       string `json:"scenes"`
	CustomCursor string `json:"custom_cursor"`
	CustomLimit  string `json:"custom_limit"`
}

type miscImEmoticonRequest struct {
	Cursor  string `json:"cursor"`
	Count   string `json:"count"`
	GroupID string `json:"group_id"`
}

type miscImFeedbackRequest struct {
	Entrance string `json:"entrance"`
}

type miscImPullRequest struct {
	Cursor    int64 `json:"cursor"`
	Timestamp int64 `json:"timestamp"`
}

// --- Handlers --------------------------------------------------------------

func (s *AppServer) miscImSpotlightHandler(c *gin.Context) {
	var req miscImSpotlightRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetSpotlightRelation(c.Request.Context(), req.Count, req.MaxTime)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_SPOTLIGHT_FAILED", "获取好友关系列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取好友关系列表成功")
}

func (s *AppServer) miscImActiveStatusHandler(c *gin.Context) {
	var req miscImActiveStatusRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetActiveStatus(c.Request.Context(), req.ConvIDs, req.SecUIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_ACTIVE_STATUS_FAILED", "获取在线状态失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取在线状态成功")
}

func (s *AppServer) miscImHeartbeatHandler(c *gin.Context) {
	var req miscImHeartbeatRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMActiveHeartbeat(c.Request.Context(), req.NewUserLogin)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_HEARTBEAT_FAILED", "在线心跳上报失败", err.Error())
		return
	}
	respondSuccess(c, res, "在线心跳上报成功")
}

func (s *AppServer) miscImActiveConfigHandler(c *gin.Context) {
	res, err := s.service.IMGetActiveConfig(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_ACTIVE_CONFIG_FAILED", "获取在线状态配置失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取在线状态配置成功")
}

func (s *AppServer) miscImStrategyHandler(c *gin.Context) {
	var req miscImStrategyRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetStrategyConfig(c.Request.Context(), req.Scenes)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_STRATEGY_FAILED", "获取 IM 策略配置失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 IM 策略配置成功")
}

func (s *AppServer) miscImResourcesHandler(c *gin.Context) {
	var req miscImResourcesRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetResources(c.Request.Context(), req.Scenes, req.CustomCursor, req.CustomLimit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_RESOURCES_FAILED", "获取资源列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取资源列表成功")
}

func (s *AppServer) miscImEmoticonHandler(c *gin.Context) {
	var req miscImEmoticonRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetEmoticonTrending(c.Request.Context(), req.Cursor, req.Count, req.GroupID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_EMOTICON_FAILED", "获取热门表情失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取热门表情成功")
}

func (s *AppServer) miscImFeedbackHandler(c *gin.Context) {
	var req miscImFeedbackRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMGetFeedbackEntrance(c.Request.Context(), req.Entrance)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_FEEDBACK_FAILED", "获取在线反馈入口失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取在线反馈入口成功")
}

func (s *AppServer) miscImPullHandler(c *gin.Context) {
	var req miscImPullRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMPullMessages(c.Request.Context(), req.Cursor, req.Timestamp)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_PULL_FAILED", "拉取 IM 消息失败", err.Error())
		return
	}
	respondSuccess(c, res, "拉取 IM 消息成功")
}

// registerMiscImRoutes 注册 IM 消息面板相关路由。
func registerMiscImRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/im/spotlight/relation", appServer.miscImSpotlightHandler)
	api.POST("/im/active/status", appServer.miscImActiveStatusHandler)
	api.POST("/im/active/heartbeat", appServer.miscImHeartbeatHandler)
	api.POST("/im/active/config", appServer.miscImActiveConfigHandler)
	api.POST("/im/strategy/config", appServer.miscImStrategyHandler)
	api.POST("/im/resources", appServer.miscImResourcesHandler)
	api.POST("/im/emoticon/trending", appServer.miscImEmoticonHandler)
	api.POST("/im/feedback/entrance", appServer.miscImFeedbackHandler)
	api.POST("/im/messages/pull", appServer.miscImPullHandler)
}
