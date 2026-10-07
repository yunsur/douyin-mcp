package main

// 群管理类写操作的 HTTP 路由。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type conversationNameRequest struct {
	Conversation string `json:"conversation"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	Notice       string `json:"notice"`
}

type kickParticipantsRequest struct {
	Conversation string  `json:"conversation"`
	UserIDs      []int64 `json:"user_ids"`
}

type recallMessageRequest struct {
	Conversation    string `json:"conversation"`
	ServerMessageID int64  `json:"server_message_id"`
}

type deleteMessageRequest struct {
	Conversation string `json:"conversation"`
	MessageID    int64  `json:"message_id"`
}

func (s *AppServer) conversationNameHandler(c *gin.Context) {
	var req conversationNameRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SetConversationName(c.Request.Context(), req.Conversation, req.Name, req.Desc, req.Notice)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SET_CONVERSATION_NAME_FAILED", "修改群信息失败", err.Error())
		return
	}
	respondSuccess(c, res, "修改群信息成功")
}

func (s *AppServer) kickParticipantsHandler(c *gin.Context) {
	var req kickParticipantsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.KickConversationParticipants(c.Request.Context(), req.Conversation, req.UserIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "KICK_PARTICIPANTS_FAILED", "移除群成员失败", err.Error())
		return
	}
	respondSuccess(c, res, "移除群成员成功")
}

func (s *AppServer) recallMessageHandler(c *gin.Context) {
	var req recallMessageRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.RecallMessage(c.Request.Context(), req.Conversation, req.ServerMessageID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "RECALL_MESSAGE_FAILED", "撤回消息失败", err.Error())
		return
	}
	respondSuccess(c, res, "撤回消息成功")
}

func (s *AppServer) deleteMessageHandler(c *gin.Context) {
	var req deleteMessageRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.DeleteMessage(c.Request.Context(), req.Conversation, req.MessageID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_MESSAGE_FAILED", "删除消息失败", err.Error())
		return
	}
	respondSuccess(c, res, "删除消息成功")
}

// registerIMAdminRoutes 注册群管理/消息管理路由。
func registerIMAdminRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/im/conversation/name", appServer.conversationNameHandler)
	api.POST("/im/conversation/kick", appServer.kickParticipantsHandler)
	api.POST("/im/message/recall", appServer.recallMessageHandler)
	api.POST("/im/message/delete", appServer.deleteMessageHandler)
}
