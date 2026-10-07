package main

// 点赞通知「谁赞了我」的 HTTP 路由。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type noticeDiggListRequest struct {
	NoticeID string `json:"notice_id"`
	Count    string `json:"count"`
	MaxTime  string `json:"max_time"`
	MinTime  string `json:"min_time"`
}

func (s *AppServer) noticeDiggListHandler(c *gin.Context) {
	var req noticeDiggListRequest
	if c.Request.ContentLength > 0 && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.NoticeDiggList(c.Request.Context(), req.NoticeID, req.Count, req.MaxTime, req.MinTime)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_NOTICE_DIGG_LIST_FAILED", "获取点赞用户列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取点赞用户列表成功")
}

// registerMiscNoticeRoutes 注册通知补充接口路由。
func registerMiscNoticeRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/notice/digg/list", appServer.noticeDiggListHandler)
}
