package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yunsur/douyin-mcp/cookies"
	"github.com/yunsur/douyin-mcp/douyin"
)

// respondError writes the standard error envelope.
func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	if statusCode < http.StatusInternalServerError {
		logrus.Warnf("%s %s %d", c.Request.Method, c.Request.URL.Path, statusCode)
	} else {
		logrus.Errorf("%s %s %d", c.Request.Method, c.Request.URL.Path, statusCode)
	}
	c.JSON(statusCode, ErrorResponse{Error: message, Code: code, Details: details})
}

// respondSuccess writes the standard success envelope.
func respondSuccess(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, SuccessResponse{Success: true, Data: data, Message: message})
}

func bindOrFail(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return false
	}
	return true
}

// --- Request payloads ------------------------------------------------------

type userRequest struct {
	User string `json:"user" binding:"required"`
}

type userPostsRequest struct {
	User      string `json:"user" binding:"required"`
	MaxCursor string `json:"max_cursor"`
	Count     string `json:"count"`
}

type userAllPostsRequest struct {
	User  string `json:"user" binding:"required"`
	Limit int    `json:"limit"`
}

type videoRequest struct {
	Video string `json:"video" binding:"required"`
}

type videoCommentsRequest struct {
	Video  string `json:"video" binding:"required"`
	Cursor string `json:"cursor"`
	Count  string `json:"count"`
}

type allVideoCommentsRequest struct {
	Video          string `json:"video" binding:"required"`
	Limit          int    `json:"limit"`
	IncludeReplies bool   `json:"include_replies"`
}

type subCommentsRequest struct {
	Video     string `json:"video" binding:"required"`
	CommentID string `json:"comment_id" binding:"required"`
	Cursor    string `json:"cursor"`
	Count     string `json:"count"`
}

type relationRequest struct {
	User    string `json:"user" binding:"required"`
	MaxTime string `json:"max_time"`
	Count   string `json:"count"`
}

type allRelationRequest struct {
	User  string `json:"user" binding:"required"`
	Limit int    `json:"limit"`
}

type allNoticesRequest struct {
	NoticeGroup string `json:"notice_group"`
	Limit       int    `json:"limit"`
}

type noticesRequest struct {
	NoticeGroup string `json:"notice_group"`
	Count       string `json:"count"`
}

type favoritesRequest struct {
	SecUserID string `json:"sec_user_id" binding:"required"`
	MaxCursor string `json:"max_cursor"`
	Count     string `json:"count"`
}

type feedRequest struct {
	Count        string `json:"count"`
	RefreshIndex string `json:"refresh_index"`
}

type searchRequest struct {
	Keyword        string `json:"keyword" binding:"required"`
	Offset         string `json:"offset"`
	Count          int    `json:"count"`
	Channel        string `json:"channel"`
	SortType       string `json:"sort_type"`
	PublishTime    string `json:"publish_time"`
	FilterDuration string `json:"filter_duration"`
	SearchRange    string `json:"search_range"`
	ContentType    string `json:"content_type"`
}

type diggRequest struct {
	AwemeID  string `json:"aweme_id" binding:"required"`
	DiggType string `json:"digg_type"`
}

type commentRequest struct {
	AwemeID string `json:"aweme_id" binding:"required"`
	Content string `json:"content" binding:"required"`
	ReplyID string `json:"reply_id"`
}

type collectRequest struct {
	AwemeID string `json:"aweme_id" binding:"required"`
	Action  string `json:"action"`
}

type moveCollectRequest struct {
	AwemeID     string `json:"aweme_id" binding:"required"`
	CollectName string `json:"collect_name"`
	CollectID   string `json:"collect_id"`
}

type liveInfoRequest struct {
	WebRID string `json:"web_rid" binding:"required"`
}

type liveCommentRequest struct {
	RoomID  string `json:"room_id" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type liveLikeRequest struct {
	RoomID string `json:"room_id" binding:"required"`
	Count  string `json:"count"`
}

type liveRankRequest struct {
	RoomID      string `json:"room_id" binding:"required"`
	AnchorID    string `json:"anchor_id"`
	SecAnchorID string `json:"sec_anchor_id"`
}

type livePKRankRequest struct {
	WebRID string `json:"web_rid" binding:"required"`
	Side   string `json:"side"`
}

type liveProductionRequest struct {
	PageURL  string `json:"page_url" binding:"required"`
	RoomID   string `json:"room_id"`
	AuthorID string `json:"author_id"`
	Offset   string `json:"offset"`
}

type productCommentsRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	ShopID    string `json:"shop_id" binding:"required"`
	Cursor    string `json:"cursor"`
}

type conversationRequest struct {
	ToUserID            int64 `json:"to_user_id" binding:"required"`
	ConversationShortID int64 `json:"conversation_short_id"`
}

type sendDMRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

type qrCheckRequest struct {
	Token string `json:"token" binding:"required"`
}

type phoneCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type phoneLoginRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

type publishRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Images     []string `json:"images"`
	Tags       []string `json:"tags"`
	Visibility string   `json:"visibility"`
}

type publishVideoRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Video      string   `json:"video" binding:"required"`
	Tags       []string `json:"tags"`
	Visibility string   `json:"visibility"`
}

type liveProductionDetailRequest struct {
	PageURL     string `json:"page_url" binding:"required"`
	PromotionID string `json:"promotion_id" binding:"required"`
	OriginType  string `json:"origin_type"`
}

type productCounterRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	ShopID    string `json:"shop_id" binding:"required"`
	StatID    string `json:"stat_id"`
}

type linkmicRequest struct {
	RoomID    string `json:"room_id" binding:"required"`
	ChannelID string `json:"channel_id" binding:"required"`
}

type pkContextRequest struct {
	WebRID    string `json:"web_rid" binding:"required"`
	ChannelID string `json:"channel_id"`
}

type thousandTicketRequest struct {
	RoomID string `json:"room_id" binding:"required"`
	WebRID string `json:"web_rid" binding:"required"`
}

type pkContributionRequest struct {
	ChannelID string `json:"channel_id" binding:"required"`
	AnchorID  string `json:"anchor_id" binding:"required"`
}

type conversationRefRequest struct {
	Conversation string `json:"conversation" binding:"required"`
}

type conversationHistoryRequest struct {
	UnreadOnly   bool   `json:"unread_only,omitempty"`
	Conversation string `json:"conversation" binding:"required"`
	Cursor       int64  `json:"cursor"`
	Count        int    `json:"count"`
}

type participantsRequest struct {
	Conversation string `json:"conversation" binding:"required"`
	Offset       int64  `json:"offset"`
	Count        int    `json:"count"`
}

type conversationSettingRequest struct {
	Conversation string `json:"conversation" binding:"required"`
	Pin          *bool  `json:"pin"`
	Mute         *bool  `json:"mute"`
}

type noticeDetailRequest struct {
	NoticeID string `json:"notice_id_str" binding:"required"`
}

type noticeDeleteRequest struct {
	NoticeID   string `json:"notice_id_str" binding:"required"`
	ActionType string `json:"action_type"`
}

type pageCursorRequest struct {
	Cursor string `json:"cursor"`
	Offset string `json:"offset"`
	Count  string `json:"count"`
}

type appointmentRequest struct {
	AppointmentType string `json:"appointment_type"`
	Count           int    `json:"count"`
}

type suggestRequest struct {
	Keyword string `json:"keyword" binding:"required"`
}

type imUserInfoRequest struct {
	SecUIDs []string `json:"sec_uids" binding:"required"`
}

type listConversationsRequest struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type sendDMMediaRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	Kind     string `json:"kind" binding:"required"`
	Path     string `json:"path" binding:"required"`
}

type sendDMStickerRequest struct {
	ToUserID int64          `json:"to_user_id" binding:"required"`
	Sticker  map[string]any `json:"sticker" binding:"required"`
}

type sendDMCardRequest struct {
	ToUserID int64          `json:"to_user_id" binding:"required"`
	Card     map[string]any `json:"card" binding:"required"`
}

type shareDMAwemeRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	AwemeID  string `json:"aweme_id" binding:"required"`
}

type shareDMWebRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	URL      string `json:"url" binding:"required"`
}

type sendDMUserCardRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	SecUID   string `json:"sec_uid" binding:"required"`
}

// --- Handlers --------------------------------------------------------------

func (s *AppServer) checkLoginStatusHandler(c *gin.Context) {
	res, err := s.service.CheckLoginStatus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED", "检查登录状态失败", err.Error())
		return
	}
	respondSuccess(c, res, "检查登录状态成功")
}

func (s *AppServer) getLoginQRCodeHandler(c *gin.Context) {
	res, err := s.service.CreateLoginQRCode(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "QRCODE_FAILED", "获取登录二维码失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取登录二维码成功")
}

func (s *AppServer) checkLoginQRCodeHandler(c *gin.Context) {
	var req qrCheckRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.CheckLoginQRCode(c.Request.Context(), req.Token)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "QRCODE_CHECK_FAILED", "检查二维码状态失败", err.Error())
		return
	}
	respondSuccess(c, res, "检查二维码状态成功")
}

func (s *AppServer) sendPhoneCodeHandler(c *gin.Context) {
	var req phoneCodeRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendPhoneCode(c.Request.Context(), req.Phone)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_CODE_FAILED", "发送验证码失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送验证码成功")
}

func (s *AppServer) loginByPhoneHandler(c *gin.Context) {
	var req phoneLoginRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LoginByPhone(c.Request.Context(), req.Phone, req.Code)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PHONE_LOGIN_FAILED", "手机号登录失败", err.Error())
		return
	}
	respondSuccess(c, res, "手机号登录成功")
}

func (s *AppServer) deleteCookiesHandler(c *gin.Context) {
	res, err := s.service.DeleteCookies(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_COOKIES_FAILED", "删除 cookies 失败", err.Error())
		return
	}
	res["cookie_path"] = cookies.GetCookiesFilePath()
	respondSuccess(c, res, "删除 cookies 成功")
}

func (s *AppServer) getUserInfoHandler(c *gin.Context) {
	var req userRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserInfo(c.Request.Context(), req.User)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_USER_INFO_FAILED", "获取用户信息失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取用户信息成功")
}

func (s *AppServer) getUserPostsHandler(c *gin.Context) {
	var req userPostsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserPosts(c.Request.Context(), req.User, req.MaxCursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_USER_POSTS_FAILED", "获取用户作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取用户作品成功")
}

func (s *AppServer) getUserAllPostsHandler(c *gin.Context) {
	var req userAllPostsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserAllPosts(c.Request.Context(), req.User, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_USER_POSTS_FAILED", "获取用户全部作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取用户全部作品成功")
}

func (s *AppServer) getVideoDetailHandler(c *gin.Context) {
	var req videoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetVideoDetail(c.Request.Context(), req.Video)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_VIDEO_FAILED", "获取作品详情失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取作品详情成功")
}

func (s *AppServer) getVideoCommentsHandler(c *gin.Context) {
	var req videoCommentsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetVideoComments(c.Request.Context(), req.Video, req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COMMENTS_FAILED", "获取评论失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取评论成功")
}

func (s *AppServer) getAllVideoCommentsHandler(c *gin.Context) {
	var req allVideoCommentsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetAllVideoComments(c.Request.Context(), req.Video, req.Limit, req.IncludeReplies)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COMMENTS_FAILED", "获取全部评论失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取全部评论成功")
}

func (s *AppServer) getSubCommentsHandler(c *gin.Context) {
	var req subCommentsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetSubComments(c.Request.Context(), req.Video, req.CommentID, req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_REPLIES_FAILED", "获取评论回复失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取评论回复成功")
}

func (s *AppServer) searchVideosHandler(c *gin.Context) {
	var req searchRequest
	if c.Request.Method == http.MethodPost {
		if !bindOrFail(c, &req) {
			return
		}
	} else {
		req.Keyword = c.Query("keyword")
		req.Offset = c.Query("offset")
		req.Channel = c.Query("channel")
		req.SortType = c.Query("sort_type")
		req.PublishTime = c.Query("publish_time")
		req.FilterDuration = c.Query("filter_duration")
		req.SearchRange = c.Query("search_range")
		req.ContentType = c.Query("content_type")
	}
	if req.Keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", nil)
		return
	}
	res, err := s.service.SearchVideos(c.Request.Context(), SearchVideosRequest{
		Keyword:        req.Keyword,
		Offset:         req.Offset,
		Count:          req.Count,
		Channel:        req.Channel,
		SortType:       req.SortType,
		PublishTime:    req.PublishTime,
		FilterDuration: req.FilterDuration,
		SearchRange:    req.SearchRange,
		ContentType:    req.ContentType,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_VIDEOS_FAILED", "搜索视频失败", err.Error())
		return
	}
	respondSuccess(c, res, "搜索视频成功")
}

func (s *AppServer) searchUsersHandler(c *gin.Context) {
	keyword, offset, count := c.Query("keyword"), c.Query("offset"), c.Query("count")
	if c.Request.Method == http.MethodPost {
		var req searchRequest
		if !bindOrFail(c, &req) {
			return
		}
		keyword, offset, count = req.Keyword, req.Offset, ""
	}
	if keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", nil)
		return
	}
	res, err := s.service.SearchUsers(c.Request.Context(), keyword, offset, count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_USERS_FAILED", "搜索用户失败", err.Error())
		return
	}
	respondSuccess(c, res, "搜索用户成功")
}

func (s *AppServer) searchLivesHandler(c *gin.Context) {
	keyword, offset, count := c.Query("keyword"), c.Query("offset"), c.Query("count")
	if c.Request.Method == http.MethodPost {
		var req searchRequest
		if !bindOrFail(c, &req) {
			return
		}
		keyword, offset, count = req.Keyword, req.Offset, ""
	}
	if keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", nil)
		return
	}
	res, err := s.service.SearchLives(c.Request.Context(), keyword, offset, count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_LIVES_FAILED", "搜索直播失败", err.Error())
		return
	}
	respondSuccess(c, res, "搜索直播成功")
}

func (s *AppServer) getUserFollowersHandler(c *gin.Context) {
	var req relationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserFollowers(c.Request.Context(), req.User, req.MaxTime, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FOLLOWERS_FAILED", "获取粉丝列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取粉丝列表成功")
}

func (s *AppServer) getUserFollowingHandler(c *gin.Context) {
	var req relationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserFollowing(c.Request.Context(), req.User, req.MaxTime, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FOLLOWING_FAILED", "获取关注列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取关注列表成功")
}

func (s *AppServer) getNoticesHandler(c *gin.Context) {
	var req noticesRequest
	if c.Request.Method == http.MethodPost && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetNotices(c.Request.Context(), req.NoticeGroup, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_NOTICES_FAILED", "获取消息通知失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取消息通知成功")
}

func (s *AppServer) getUserFavoritesHandler(c *gin.Context) {
	var req favoritesRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetUserFavorites(c.Request.Context(), req.SecUserID, req.MaxCursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FAVORITES_FAILED", "获取收藏列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取收藏列表成功")
}

func (s *AppServer) getCollectListHandler(c *gin.Context) {
	res, err := s.service.GetCollectList(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COLLECTS_FAILED", "获取收藏夹失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取收藏夹成功")
}

func (s *AppServer) getHomeFeedHandler(c *gin.Context) {
	var req feedRequest
	if c.Request.Method == http.MethodPost && !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetHomeFeed(c.Request.Context(), req.Count, req.RefreshIndex)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FEED_FAILED", "获取推荐流失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取推荐流成功")
}

func (s *AppServer) diggVideoHandler(c *gin.Context) {
	var req diggRequest
	if !bindOrFail(c, &req) {
		return
	}
	if req.DiggType == "" {
		req.DiggType = "1"
	}
	res, err := s.service.DiggVideo(c.Request.Context(), req.AwemeID, req.DiggType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DIGG_FAILED", "点赞操作失败", err.Error())
		return
	}
	respondSuccess(c, res, "点赞操作成功")
}

func (s *AppServer) postVideoCommentHandler(c *gin.Context) {
	var req commentRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.PostVideoComment(c.Request.Context(), req.AwemeID, req.Content, req.ReplyID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "COMMENT_FAILED", "发布评论失败", err.Error())
		return
	}
	respondSuccess(c, res, "发布评论成功")
}

func (s *AppServer) collectVideoHandler(c *gin.Context) {
	var req collectRequest
	if !bindOrFail(c, &req) {
		return
	}
	if req.Action == "" {
		req.Action = "1"
	}
	res, err := s.service.CollectVideo(c.Request.Context(), req.AwemeID, req.Action)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "COLLECT_FAILED", "收藏操作失败", err.Error())
		return
	}
	respondSuccess(c, res, "收藏操作成功")
}

func (s *AppServer) moveCollectVideoHandler(c *gin.Context) {
	var req moveCollectRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.MoveCollectVideo(c.Request.Context(), req.AwemeID, req.CollectName, req.CollectID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "MOVE_COLLECT_FAILED", "移动收藏失败", err.Error())
		return
	}
	respondSuccess(c, res, "移动收藏成功")
}

func (s *AppServer) removeCollectVideoHandler(c *gin.Context) {
	var req moveCollectRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.RemoveCollectVideo(c.Request.Context(), req.AwemeID, req.CollectName, req.CollectID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "REMOVE_COLLECT_FAILED", "取消收藏失败", err.Error())
		return
	}
	respondSuccess(c, res, "取消收藏成功")
}

func (s *AppServer) liveInfoHandler(c *gin.Context) {
	var req liveInfoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveInfo(c.Request.Context(), req.WebRID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_INFO_FAILED", "获取直播间信息失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取直播间信息成功")
}

func (s *AppServer) startLiveListenHandler(c *gin.Context) {
	var req liveInfoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.StartLiveListen(c.Request.Context(), req.WebRID)
	if err != nil {
		if isInvalidInput(err) {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "启动直播间监听失败", err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "LIVE_LISTEN_FAILED", "启动直播间监听失败", err.Error())
		return
	}
	respondSuccess(c, res, "启动直播间监听成功")
}

func (s *AppServer) stopLiveListenHandler(c *gin.Context) {
	respondSuccess(c, s.service.StopLiveListen(), "停止直播间监听成功")
}

func (s *AppServer) liveEventsHandler(c *gin.Context) {
	respondSuccess(c, s.service.LiveEvents(), "获取直播间事件成功")
}

func (s *AppServer) sendLiveCommentHandler(c *gin.Context) {
	var req liveCommentRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendLiveComment(c.Request.Context(), req.RoomID, req.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_COMMENT_FAILED", "发送弹幕失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送弹幕成功")
}

func (s *AppServer) likeLiveRoomHandler(c *gin.Context) {
	var req liveLikeRequest
	if !bindOrFail(c, &req) {
		return
	}
	if req.Count == "" {
		req.Count = "1"
	}
	res, err := s.service.LikeLiveRoom(c.Request.Context(), req.RoomID, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_LIKE_FAILED", "直播间点赞失败", err.Error())
		return
	}
	respondSuccess(c, res, "直播间点赞成功")
}

func (s *AppServer) liveContributionRankHandler(c *gin.Context) {
	var req liveRankRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveContributionRank(c.Request.Context(), req.RoomID, req.AnchorID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_RANK_FAILED", "获取贡献榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取贡献榜成功")
}

func (s *AppServer) livePKRankHandler(c *gin.Context) {
	var req livePKRankRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LivePKRank(c.Request.Context(), req.WebRID, req.Side)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PK_FAILED", "获取 PK 榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 PK 榜成功")
}

func (s *AppServer) liveRankListHandler(c *gin.Context) {
	var req liveRankRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveRankList(c.Request.Context(), req.RoomID, req.AnchorID, req.SecAnchorID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_RANK_FAILED", "获取排行榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取排行榜成功")
}

func (s *AppServer) liveProductionHandler(c *gin.Context) {
	var req liveProductionRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveProduction(c.Request.Context(), req.PageURL, req.RoomID, req.AuthorID, req.Offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PRODUCTION_FAILED", "获取直播商品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取直播商品成功")
}

func (s *AppServer) productCommentsHandler(c *gin.Context) {
	var req productCommentsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ProductComments(c.Request.Context(), req.ProductID, req.ShopID, req.Cursor)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PRODUCT_COMMENTS_FAILED", "获取商品评论失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取商品评论成功")
}

func (s *AppServer) createConversationHandler(c *gin.Context) {
	var req conversationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.CreateConversation(c.Request.Context(), req.ToUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CREATE_CONVERSATION_FAILED", "创建会话失败", err.Error())
		return
	}
	respondSuccess(c, res, "创建会话成功")
}

func (s *AppServer) conversationListHandler(c *gin.Context) {
	var req conversationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ConversationList(c.Request.Context(), req.ToUserID, req.ConversationShortID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CONVERSATION_LIST_FAILED", "查询会话失败", err.Error())
		return
	}
	respondSuccess(c, res, "查询会话成功")
}

func (s *AppServer) sendDMHandler(c *gin.Context) {
	var req sendDMRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendDM(c.Request.Context(), req.ToUserID, req.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_DM_FAILED", "发送私信失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送私信成功")
}

func (s *AppServer) startIMListenHandler(c *gin.Context) {
	res, err := s.service.StartIMListen(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_LISTEN_FAILED", "启动私信监听失败", err.Error())
		return
	}
	respondSuccess(c, res, "启动私信监听成功")
}

func (s *AppServer) stopIMListenHandler(c *gin.Context) {
	respondSuccess(c, s.service.StopIMListen(), "停止私信监听成功")
}

func (s *AppServer) imMessagesHandler(c *gin.Context) {
	convType := 0
	if v := c.Query("conversation_type"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			convType = n
		}
	}
	respondSuccess(c,
		s.service.IMMessages(c.Query("conversation_id"), convType),
		"获取私信成功")
}

func (s *AppServer) publishHandler(c *gin.Context) {
	var req publishRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.PublishContent(c.Request.Context(), douyin.PublishImageRequest{
		Title:      req.Title,
		Content:    req.Content,
		Images:     req.Images,
		Tags:       req.Tags,
		Visibility: req.Visibility,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_FAILED", "发布失败", err.Error())
		return
	}
	respondSuccess(c, res, "发布成功")
}

func (s *AppServer) publishVideoHandler(c *gin.Context) {
	var req publishVideoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.PublishVideo(c.Request.Context(), douyin.PublishVideoRequest{
		Title:      req.Title,
		Content:    req.Content,
		Video:      req.Video,
		Tags:       req.Tags,
		Visibility: req.Visibility,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_VIDEO_FAILED", "视频发布失败", err.Error())
		return
	}
	respondSuccess(c, res, "视频发布成功")
}

func (s *AppServer) allLiveProductionHandler(c *gin.Context) {
	var req liveProductionRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.AllLiveProduction(c.Request.Context(), req.PageURL)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PRODUCTION_FAILED", "获取直播商品失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取直播商品成功")
}

func (s *AppServer) liveProductionDetailHandler(c *gin.Context) {
	var req liveProductionDetailRequest
	if !bindOrFail(c, &req) {
		return
	}
	if req.OriginType == "" {
		req.OriginType = "638303"
	}
	res, err := s.service.LiveProductionDetail(c.Request.Context(), req.PageURL, req.PromotionID, req.OriginType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PRODUCTION_DETAIL_FAILED", "获取商品详情失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取商品详情成功")
}

func (s *AppServer) productCommentCounterHandler(c *gin.Context) {
	var req productCounterRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ProductCommentCounter(c.Request.Context(), req.ProductID, req.ShopID, req.StatID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PRODUCT_COUNTER_FAILED", "获取商品评论统计失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取商品评论统计成功")
}

func (s *AppServer) liveRoomEnterHandler(c *gin.Context) {
	var req liveInfoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveRoomEnter(c.Request.Context(), req.WebRID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_ENTER_FAILED", "直播间进入失败", err.Error())
		return
	}
	respondSuccess(c, res, "直播间进入成功")
}

func (s *AppServer) liveLinkmicListHandler(c *gin.Context) {
	var req linkmicRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveLinkmicList(c.Request.Context(), req.RoomID, req.ChannelID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_LINKMIC_FAILED", "获取连麦列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取连麦列表成功")
}

func (s *AppServer) livePKContextHandler(c *gin.Context) {
	var req pkContextRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LivePKContext(c.Request.Context(), req.WebRID, req.ChannelID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PK_CONTEXT_FAILED", "获取 PK 上下文失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 PK 上下文成功")
}

func (s *AppServer) liveThousandTicketRankHandler(c *gin.Context) {
	var req thousandTicketRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LiveThousandTicketRank(c.Request.Context(), req.RoomID, req.WebRID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_RANK_FAILED", "获取千票榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取千票榜成功")
}

func (s *AppServer) livePKContributionRankHandler(c *gin.Context) {
	var req pkContributionRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LivePKContributionRank(c.Request.Context(), req.ChannelID, req.AnchorID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIVE_PK_RANK_FAILED", "获取 PK 贡献榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取 PK 贡献榜成功")
}

func (s *AppServer) sendDMMediaHandler(c *gin.Context) {
	var req sendDMMediaRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendDMMedia(c.Request.Context(), req.ToUserID, req.Kind, req.Path)
	if err != nil {
		if isInvalidInput(err) {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "发送私信媒体失败", err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "SEND_DM_MEDIA_FAILED", "发送私信媒体失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送私信媒体成功")
}

func (s *AppServer) sendDMStickerHandler(c *gin.Context) {
	var req sendDMStickerRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendDMSticker(c.Request.Context(), req.ToUserID, req.Sticker)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_DM_STICKER_FAILED", "发送表情包失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送表情包成功")
}

func (s *AppServer) sendDMCardHandler(c *gin.Context) {
	var req sendDMCardRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendDMCard(c.Request.Context(), req.ToUserID, req.Card)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_DM_CARD_FAILED", "发送卡片失败", err.Error())
		return
	}
	respondSuccess(c, res, "发送卡片成功")
}

func (s *AppServer) shareDMAwemeHandler(c *gin.Context) {
	var req shareDMAwemeRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ShareDMAweme(c.Request.Context(), req.ToUserID, req.AwemeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHARE_DM_FAILED", "分享作品失败", err.Error())
		return
	}
	respondSuccess(c, res, "分享作品成功")
}

func (s *AppServer) shareDMWebHandler(c *gin.Context) {
	var req shareDMWebRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ShareDMWeb(c.Request.Context(), req.ToUserID, req.URL)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHARE_DM_FAILED", "分享链接失败", err.Error())
		return
	}
	respondSuccess(c, res, "分享链接成功")
}

func (s *AppServer) sendDMUserCardHandler(c *gin.Context) {
	var req sendDMUserCardRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SendDMUserCard(c.Request.Context(), req.ToUserID, req.SecUID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_DM_CARD_FAILED", "分享用户名片失败", err.Error())
		return
	}
	respondSuccess(c, res, "分享用户名片成功")
}
func (s *AppServer) getAllUserFollowersHandler(c *gin.Context) {
	var req allRelationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetAllUserFollowers(c.Request.Context(), req.User, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FOLLOWERS_FAILED", "获取粉丝列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取粉丝列表成功")
}

func (s *AppServer) getAllUserFollowingHandler(c *gin.Context) {
	var req allRelationRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetAllUserFollowing(c.Request.Context(), req.User, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FOLLOWING_FAILED", "获取关注列表失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取关注列表成功")
}

func (s *AppServer) getAllNoticesHandler(c *gin.Context) {
	var req allNoticesRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetAllNotices(c.Request.Context(), req.NoticeGroup, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_NOTICES_FAILED", "获取消息通知失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取消息通知成功")
}
func (s *AppServer) listConversationsHandler(c *gin.Context) {
	var req listConversationsRequest
	if c.Request.Method == http.MethodPost && !bindOrFail(c, &req) {
		return
	} else if c.Request.Method == http.MethodGet {
		req.Name = c.Query("name")
	}
	convs, err := s.service.ListConversations(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_CONVERSATIONS_FAILED", "获取会话列表失败", err.Error())
		return
	}
	out := make([]any, 0, len(convs))
	for _, conv := range convs {
		if req.Name != "" && !stringsContainsFold(conv.Name, req.Name) {
			continue
		}
		if req.Type != 0 && int(conv.ConversationType) != req.Type {
			continue
		}
		out = append(out, conv)
	}
	respondSuccess(c, out, "获取会话列表成功")
}

func (s *AppServer) conversationInfoHandler(c *gin.Context) {
	var req conversationRefRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ConversationInfo(c.Request.Context(), req.Conversation)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CONVERSATION_INFO_FAILED", "获取会话信息失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取会话信息成功")
}

func (s *AppServer) conversationHistoryHandler(c *gin.Context) {
	var req conversationHistoryRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ConversationHistory(c.Request.Context(), req.Conversation, req.Cursor, req.Count, req.UnreadOnly)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CONVERSATION_HISTORY_FAILED", "获取聊天记录失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取聊天记录成功")
}

func (s *AppServer) conversationParticipantsHandler(c *gin.Context) {
	var req participantsRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ConversationParticipants(c.Request.Context(), req.Conversation, req.Offset, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PARTICIPANTS_FAILED", "获取群成员失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取群成员成功")
}

func (s *AppServer) markConversationReadHandler(c *gin.Context) {
	var req conversationRefRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.MarkConversationRead(c.Request.Context(), req.Conversation)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "MARK_READ_FAILED", "标记已读失败", err.Error())
		return
	}
	respondSuccess(c, res, "标记已读成功")
}

func (s *AppServer) strangerConversationsHandler(c *gin.Context) {
	res, err := s.service.StrangerConversations(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STRANGERS_FAILED", "获取陌生人会话失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取陌生人会话成功")
}

func (s *AppServer) imUserInfoHandler(c *gin.Context) {
	var req imUserInfoRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.IMUserInfo(c.Request.Context(), req.SecUIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "IM_USER_INFO_FAILED", "获取用户资料失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取用户资料成功")
}

// stringsContainsFold is a tiny helper so handlers stay dependency-free.
func stringsContainsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
func (s *AppServer) conversationSettingHandler(c *gin.Context) {
	var req conversationSettingRequest
	if !bindOrFail(c, &req) {
		return
	}
	if req.Pin == nil && req.Mute == nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "缺少参数", "pin 与 mute 至少需要一个")
		return
	}
	res, err := s.service.SetConversationSetting(c.Request.Context(), req.Conversation, req.Pin, req.Mute)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CONVERSATION_SETTING_FAILED", "设置会话失败", err.Error())
		return
	}
	respondSuccess(c, res, "设置会话成功")
}

func (s *AppServer) leaveConversationHandler(c *gin.Context) {
	var req conversationRefRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.LeaveConversation(c.Request.Context(), req.Conversation)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LEAVE_CONVERSATION_FAILED", "退出群聊失败", err.Error())
		return
	}
	respondSuccess(c, res, "退出群聊成功")
}

func (s *AppServer) shareConversationHandler(c *gin.Context) {
	var req conversationRefRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.ShareConversation(c.Request.Context(), req.Conversation)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHARE_CONVERSATION_FAILED", "获取群分享信息失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取群分享信息成功")
}
func (s *AppServer) noticeCountHandler(c *gin.Context) {
	res, err := s.service.NoticeCount(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "NOTICE_COUNT_FAILED", "获取未读数失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取未读数成功")
}

func (s *AppServer) noticeDetailHandler(c *gin.Context) {
	var req noticeDetailRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.NoticeDetail(c.Request.Context(), req.NoticeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "NOTICE_DETAIL_FAILED", "获取通知详情失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取通知详情成功")
}

func (s *AppServer) noticeDeleteHandler(c *gin.Context) {
	var req noticeDeleteRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.NoticeDelete(c.Request.Context(), req.ActionType, req.NoticeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "NOTICE_DELETE_FAILED", "删除通知失败", err.Error())
		return
	}
	respondSuccess(c, res, "删除通知成功")
}
func (s *AppServer) watchHistoryHandler(c *gin.Context) {
	var req pageCursorRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.WatchHistory(c.Request.Context(), req.Cursor, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "HISTORY_FAILED", "获取观看历史失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取观看历史成功")
}

func (s *AppServer) clearWatchHistoryHandler(c *gin.Context) {
	res, err := s.service.ClearWatchHistory(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "HISTORY_CLEAR_FAILED", "清空观看历史失败", err.Error())
		return
	}
	respondSuccess(c, res, "清空观看历史成功")
}

func (s *AppServer) watchLaterHandler(c *gin.Context) {
	var req pageCursorRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.WatchLater(c.Request.Context(), req.Offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "WATCHLATER_FAILED", "获取稍后再看失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取稍后再看成功")
}

func (s *AppServer) appointmentsHandler(c *gin.Context) {
	var req appointmentRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.Appointments(c.Request.Context(), req.AppointmentType, req.Count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "APPOINTMENT_FAILED", "获取我的预约失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取我的预约成功")
}
func (s *AppServer) searchSuggestHandler(c *gin.Context) {
	var req suggestRequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.SearchSuggest(c.Request.Context(), req.Keyword)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_SUGGEST_FAILED", "获取搜索建议失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取搜索建议成功")
}

func (s *AppServer) hotSearchBoardHandler(c *gin.Context) {
	res, err := s.service.HotSearchBoard(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "HOT_SEARCH_FAILED", "获取热搜榜失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取热搜榜成功")
}

func (s *AppServer) searchChallengesHandler(c *gin.Context) {
	keyword, cursor, count := c.Query("keyword"), c.Query("cursor"), c.Query("count")
	if c.Request.Method == http.MethodPost {
		var req searchRequest
		if !bindOrFail(c, &req) {
			return
		}
		keyword, cursor, count = req.Keyword, req.Offset, ""
	}
	if keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", nil)
		return
	}
	res, err := s.service.SearchChallenges(c.Request.Context(), keyword, cursor, count)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_CHALLENGE_FAILED", "搜索话题失败", err.Error())
		return
	}
	respondSuccess(c, res, "搜索话题成功")
}
