package main

// HTTP routes for the feeds slice (精选/推荐 tab). Registered by the main
// agent from routes.go via registerMiscFeedsRoutes / registerMiscFeeds_Routes.

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type channelModuleFeedRequest struct {
	ModuleID          string `json:"module_id"`
	Count             string `json:"count"`
	RefreshIndex      string `json:"refresh_index"`
	UseLiteType       string `json:"use_lite_type"`
	PreItemIDs        string `json:"pre_item_ids"`
	PreLogID          string `json:"pre_log_id"`
	EncodedPreItemIDs string `json:"encoded_pre_item_ids"`
	EncodedPreRoomIDs string `json:"encoded_pre_room_ids"`
}

type courseCategoryTagsRequest struct {
	TabID string `json:"tab_id"`
}

type courseCategoryVideosRequest struct {
	TabID     string `json:"tab_id"`
	Offset    string `json:"offset"`
	Size      string `json:"size"`
	TagIDList string `json:"tag_id_list"`
	IDList    string `json:"id_list"`
}

type solutionResourcesRequest struct {
	SpotKeys string `json:"spot_keys"`
	AppID    string `json:"app_id"`
}

type emojiListRequest struct {
	NeedAll string `json:"need_all"`
}

type publishHighlightRequest struct {
	HighlightType string `json:"highlight_type"`
}

type mixListCollectionRequest struct {
	Cursor string `json:"cursor"`
	Count  string `json:"count"`
}

type studyNotesRequest struct {
	Offset      string `json:"offset"`
	Count       string `json:"count"`
	FilterDraft string `json:"filter_draft"`
}

func (s *AppServer) channelModuleFeedHandler(c *gin.Context) {
	var req channelModuleFeedRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetChannelModuleFeed(c.Request.Context(), req.ModuleID, req.Count, req.RefreshIndex, req.UseLiteType, req.PreItemIDs, req.PreLogID, req.EncodedPreItemIDs, req.EncodedPreRoomIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_CHANNEL_MODULE_FEED_FAILED", "获取频道模块流失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取频道模块流成功")
}

func (s *AppServer) courseCategoryTagsHandler(c *gin.Context) {
	var req courseCategoryTagsRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetCourseCategoryTags(c.Request.Context(), req.TabID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COURSE_CATEGORY_TAGS_FAILED", "获取课程分类标签失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取课程分类标签成功")
}

func (s *AppServer) courseCategoryVideosHandler(c *gin.Context) {
	var req courseCategoryVideosRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetCourseCategoryVideos(c.Request.Context(), req.TabID, req.Offset, req.Size, req.TagIDList, req.IDList)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COURSE_CATEGORY_VIDEOS_FAILED", "获取课程分类作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取课程分类作品成功")
}

func (s *AppServer) solutionResourcesHandler(c *gin.Context) {
	var req solutionResourcesRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetSolutionResources(c.Request.Context(), req.SpotKeys, req.AppID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_SOLUTION_RESOURCES_FAILED", "获取资源位失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取资源位成功")
}

func (s *AppServer) multicastConfigHandler(c *gin.Context) {
	res, err := s.service.GetMulticastConfig(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MULTICAST_CONFIG_FAILED", "获取 multicast 配置失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 multicast 配置成功")
}

func (s *AppServer) pageTurnOfflineHandler(c *gin.Context) {
	res, err := s.service.PageTurnOffline(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PAGE_TURN_OFFLINE_FAILED", "翻页上报失败", err.Error())
		return
	}
	respondSuccess(c, res, "翻页上报成功")
}

func (s *AppServer) emojiListHandler(c *gin.Context) {
	var req emojiListRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetEmojiList(c.Request.Context(), req.NeedAll)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_EMOJI_LIST_FAILED", "获取表情列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取表情列表成功")
}

func (s *AppServer) publishHighlightHandler(c *gin.Context) {
	var req publishHighlightRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetPublishHighlight(c.Request.Context(), req.HighlightType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_PUBLISH_HIGHLIGHT_FAILED", "获取发布高亮配置失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取发布高亮配置成功")
}

func (s *AppServer) mixListCollectionHandler(c *gin.Context) {
	var req mixListCollectionRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetMixListCollection(c.Request.Context(), req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MIX_LIST_COLLECTION_FAILED", "获取合集收藏失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取合集收藏成功")
}

func (s *AppServer) seoInnerLinkHandler(c *gin.Context) {
	res, err := s.service.GetSEOInnerLink(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_SEO_INNER_LINK_FAILED", "获取 SEO 内链失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 SEO 内链成功")
}

func (s *AppServer) studyNotesHandler(c *gin.Context) {
	var req studyNotesRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetStudyNotes(c.Request.Context(), req.Offset, req.Count, req.FilterDraft)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_STUDY_NOTES_FAILED", "获取学习笔记失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取学习笔记成功")
}

func registerMiscFeedsRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/channel/module/feed", appServer.channelModuleFeedHandler)
	api.POST("/course/category/tags", appServer.courseCategoryTagsHandler)
	api.POST("/course/category/videos", appServer.courseCategoryVideosHandler)
	api.POST("/solution/resources", appServer.solutionResourcesHandler)
	api.POST("/multicast/config", appServer.multicastConfigHandler)
	api.POST("/page/turn/offline", appServer.pageTurnOfflineHandler)
	api.POST("/emoji/list", appServer.emojiListHandler)
	api.POST("/creator/publish/highlight", appServer.publishHighlightHandler)
	api.POST("/mix/listcollection", appServer.mixListCollectionHandler)
	api.POST("/seo/inner/link", appServer.seoInnerLinkHandler)
	api.POST("/study/notes", appServer.studyNotesHandler)
}
