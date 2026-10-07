package main

// HTTP routes for the hot-search-video + music-page endpoints
// (热搜词视频 / 音乐页). Registered by the main agent from routes.go via
// registerMiscHotMusicRoutes.

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type hotSearchVideosRequest struct {
	Hotword    string `json:"hotword"`
	SentenceID string `json:"sentence_id"`
	Offset     string `json:"offset"`
	Count      string `json:"count"`
	EntryName  string `json:"entry_name"`
}

type musicAwemeRequest struct {
	MusicID string `json:"music_id"`
	Cursor  string `json:"cursor"`
	Count   string `json:"count"`
}

type musicDetailRequest struct {
	MusicID string `json:"music_id"`
	Scene   string `json:"scene"`
}

type musicCollectionRequest struct {
	Cursor string `json:"cursor"`
	Count  string `json:"count"`
}

type collectMusicRequest struct {
	MusicID string `json:"music_id"`
	Type    string `json:"type"`
	Action  string `json:"action"`
}

func (s *AppServer) hotSearchVideosHandler(c *gin.Context) {
	var req hotSearchVideosRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetHotSearchVideos(c.Request.Context(), req.Hotword, req.SentenceID, req.Offset, req.Count, req.EntryName)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_HOT_SEARCH_VIDEOS_FAILED", "获取热搜词视频失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取热搜词视频成功")
}

func (s *AppServer) musicAwemeHandler(c *gin.Context) {
	var req musicAwemeRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetMusicAweme(c.Request.Context(), req.MusicID, req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MUSIC_AWEME_FAILED", "获取音乐作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取音乐作品成功")
}

func (s *AppServer) musicDetailHandler(c *gin.Context) {
	var req musicDetailRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetMusicDetail(c.Request.Context(), req.MusicID, req.Scene)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MUSIC_DETAIL_FAILED", "获取音乐详情失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取音乐详情成功")
}

func (s *AppServer) musicCollectionHandler(c *gin.Context) {
	var req musicCollectionRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetMusicCollection(c.Request.Context(), req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MUSIC_COLLECTION_FAILED", "获取收藏音乐失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取收藏音乐成功")
}

func (s *AppServer) collectMusicHandler(c *gin.Context) {
	var req collectMusicRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.CollectMusic(c.Request.Context(), req.MusicID, req.Type, req.Action)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "COLLECT_MUSIC_FAILED", "收藏音乐失败", err.Error())
		return
	}
	respondSuccess(c, res, "收藏音乐成功")
}

func registerMiscHotMusicRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/hot/search/videos", appServer.hotSearchVideosHandler)
	api.POST("/music/aweme", appServer.musicAwemeHandler)
	api.POST("/music/detail", appServer.musicDetailHandler)
	api.POST("/music/listcollection", appServer.musicCollectionHandler)
	api.POST("/music/collect", appServer.collectMusicHandler)
}
