package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setupRoutes wires the HTTP API and the MCP streamable endpoint.
func setupRoutes(appServer *AppServer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	// gin 默认把「路径存在但方法不匹配」当 404、未命中路由回空体，两者都会让
	// 客户端拿不到统一的 JSON 信封。这里改成 405 / 404 都给标准错误结构。
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		respondError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不被允许", nil)
	})
	router.NoRoute(func(c *gin.Context) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "请求的路径不存在", nil)
	})
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(errorHandlingMiddleware())
	router.Use(corsMiddleware())

	router.GET("/health", healthHandler)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return appServer.mcpServer },
		&mcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true},
	)

	protected := router.Group("")
	protected.Use(authMiddleware(appServer.authToken))
	protected.Any("/mcp", gin.WrapH(mcpHandler))
	protected.Any("/mcp/*path", gin.WrapH(mcpHandler))

	api := protected.Group("/api/v1")
	{
		// Session / login
		api.GET("/login/status", appServer.checkLoginStatusHandler)
		api.GET("/login/qrcode", appServer.getLoginQRCodeHandler)
		api.POST("/login/qrcode/check", appServer.checkLoginQRCodeHandler)
		api.POST("/login/phone/code", appServer.sendPhoneCodeHandler)
		api.POST("/login/phone", appServer.loginByPhoneHandler)
		api.DELETE("/login/cookies", appServer.deleteCookiesHandler)

		// Users & works
		api.POST("/user/info", appServer.getUserInfoHandler)
		api.POST("/user/posts", appServer.getUserPostsHandler)
		api.POST("/user/posts/all", appServer.getUserAllPostsHandler)
		api.POST("/video/detail", appServer.getVideoDetailHandler)
		api.POST("/video/comments", appServer.getVideoCommentsHandler)
		api.POST("/video/comments/all", appServer.getAllVideoCommentsHandler)
		api.POST("/video/comments/replies", appServer.getSubCommentsHandler)

		// Search
		api.GET("/search/videos", appServer.searchVideosHandler)
		api.POST("/search/videos", appServer.searchVideosHandler)
		api.GET("/search/users", appServer.searchUsersHandler)
		api.POST("/search/users", appServer.searchUsersHandler)
		api.POST("/search/suggest", appServer.searchSuggestHandler)
		api.GET("/search/hot", appServer.hotSearchBoardHandler)
		api.GET("/search/challenges", appServer.searchChallengesHandler)
		api.POST("/search/challenges", appServer.searchChallengesHandler)
		api.GET("/search/lives", appServer.searchLivesHandler)
		api.POST("/search/lives", appServer.searchLivesHandler)

		// Relations / notifications / collections
		api.POST("/user/followers", appServer.getUserFollowersHandler)
		api.POST("/user/following", appServer.getUserFollowingHandler)
		api.POST("/user/followers/all", appServer.getAllUserFollowersHandler)
		api.POST("/user/following/all", appServer.getAllUserFollowingHandler)
		api.POST("/notices", appServer.getNoticesHandler)
		api.POST("/notices/all", appServer.getAllNoticesHandler)
		api.GET("/notices/count", appServer.noticeCountHandler)
		api.POST("/notices/detail", appServer.noticeDetailHandler)
		api.POST("/notices/delete", appServer.noticeDeleteHandler)
		api.POST("/user/favorites", appServer.getUserFavoritesHandler)
		api.POST("/user/history", appServer.watchHistoryHandler)
		api.POST("/user/history/clear", appServer.clearWatchHistoryHandler)
		api.POST("/user/watchlater", appServer.watchLaterHandler)
		api.POST("/user/appointments", appServer.appointmentsHandler)
		api.GET("/collects", appServer.getCollectListHandler)
		api.GET("/feed", appServer.getHomeFeedHandler)

		// Interactions
		api.POST("/video/digg", appServer.diggVideoHandler)
		api.POST("/video/comment", appServer.postVideoCommentHandler)
		api.POST("/video/collect", appServer.collectVideoHandler)
		api.POST("/video/collect/move", appServer.moveCollectVideoHandler)
		api.POST("/video/collect/remove", appServer.removeCollectVideoHandler)

		// Live
		api.POST("/live/info", appServer.liveInfoHandler)
		api.POST("/live/listen/start", appServer.startLiveListenHandler)
		api.POST("/live/listen/stop", appServer.stopLiveListenHandler)
		api.GET("/live/events", appServer.liveEventsHandler)
		api.POST("/live/comment", appServer.sendLiveCommentHandler)
		api.POST("/live/like", appServer.likeLiveRoomHandler)
		api.POST("/live/rank/contribution", appServer.liveContributionRankHandler)
		api.POST("/live/rank/pk", appServer.livePKRankHandler)
		api.POST("/live/rank/list", appServer.liveRankListHandler)
		api.POST("/live/production", appServer.liveProductionHandler)
		api.POST("/live/production/all", appServer.allLiveProductionHandler)
		api.POST("/live/production/detail", appServer.liveProductionDetailHandler)
		api.POST("/live/product/comments", appServer.productCommentsHandler)
		api.POST("/live/product/comments/counter", appServer.productCommentCounterHandler)
		api.POST("/live/enter", appServer.liveRoomEnterHandler)
		api.POST("/live/linkmic/list", appServer.liveLinkmicListHandler)
		api.POST("/live/pk/context", appServer.livePKContextHandler)
		api.POST("/live/rank/thousand_ticket", appServer.liveThousandTicketRankHandler)
		api.POST("/live/rank/pk/contribution", appServer.livePKContributionRankHandler)

		// Direct messages
		api.POST("/im/conversation/create", appServer.createConversationHandler)
		api.POST("/im/conversation/list", appServer.conversationListHandler)
		api.GET("/im/conversations", appServer.listConversationsHandler)
		api.POST("/im/conversations", appServer.listConversationsHandler)
		api.POST("/im/conversation/info", appServer.conversationInfoHandler)
		api.POST("/im/conversation/history", appServer.conversationHistoryHandler)
		api.POST("/im/conversation/participants", appServer.conversationParticipantsHandler)
		api.POST("/im/conversation/mark_read", appServer.markConversationReadHandler)
		api.POST("/im/conversation/setting", appServer.conversationSettingHandler)
		api.POST("/im/conversation/leave", appServer.leaveConversationHandler)
		api.POST("/im/conversation/share", appServer.shareConversationHandler)
		api.GET("/im/strangers", appServer.strangerConversationsHandler)
		api.POST("/im/user_info", appServer.imUserInfoHandler)
		api.POST("/im/send", appServer.sendDMHandler)
		api.POST("/im/send_media", appServer.sendDMMediaHandler)
		api.POST("/im/send_sticker", appServer.sendDMStickerHandler)
		api.POST("/im/send_card", appServer.sendDMCardHandler)
		api.POST("/im/share_aweme", appServer.shareDMAwemeHandler)
		api.POST("/im/share_web", appServer.shareDMWebHandler)
		api.POST("/im/send_user_card", appServer.sendDMUserCardHandler)
		api.POST("/im/listen/start", appServer.startIMListenHandler)
		api.POST("/im/listen/stop", appServer.stopIMListenHandler)
		api.GET("/im/messages", appServer.imMessagesHandler)

		// Creator publishing
		api.POST("/publish", appServer.publishHandler)
		api.POST("/publish_video", appServer.publishVideoHandler)

		// Tab-coverage slices (精选/推荐/关注/朋友/我的 的补齐接口).
		registerMiscFeedsRoutes(api, appServer)
		registerMiscSocialRoutes(api, appServer)
		registerMiscProfile_Routes(api, appServer)
		registerMiscImRoutes(api, appServer)
		registerMiscHotMusicRoutes(api, appServer)
		registerMiscEcomRoutes(api, appServer)
		registerMiscNoticeRoutes(api, appServer)
		registerIMAdminRoutes(api, appServer)
	}

	return router
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
