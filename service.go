package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/yunsur/douyin-mcp/configs"
	"github.com/yunsur/douyin-mcp/cookies"
	"github.com/yunsur/douyin-mcp/douyin"
)

// DouyinService owns the live Douyin session and the stateful listeners.
type DouyinService struct {
	cfg        *configs.Config
	cookiePath string

	mu     sync.Mutex
	client *douyin.Client

	liveMu     sync.Mutex
	live       *douyin.LiveListener
	liveEvents []map[string]any

	imMu       sync.Mutex
	im         *douyin.IMReceiver
	imStop     chan struct{}
	imMessages []map[string]any

	// The IM server only returns conversations that changed since the client's
	// last sync, so the union of everything seen so far is accumulated locally
	// (the web SDK does the same) to approach the full conversation list.
	convMu        sync.Mutex
	convCache     map[string]douyin.IMConversation
	convCachePath string
}

// NewDouyinService builds a session from config/cookie string.
func NewDouyinService(cfg *configs.Config, cookiePath, cookieStr string) (*DouyinService, error) {
	s := &DouyinService{cfg: cfg, cookiePath: cookiePath}
	client, err := newClient(cfg, cookieStr)
	if err != nil {
		return nil, err
	}
	s.client = client
	s.convCache = map[string]douyin.IMConversation{}
	if cookiePath != "" {
		s.convCachePath = filepath.Join(filepath.Dir(cookiePath), "conversations_cache.json")
		s.loadConversationCache()
	}
	// Warm the mssdk msToken so the very first API call already carries a real
	// token instead of the random fallback. Best-effort: never block startup.
	if _, err := client.PrimeMsToken(context.Background()); err != nil {
		logrus.WithError(err).Debug("启动时换取 msToken 失败，首个请求将使用随机回退值")
	}
	return s, nil
}

func newClient(cfg *configs.Config, cookieStr string) (*douyin.Client, error) {
	return douyin.NewClient(cookieStr, douyin.Options{
		Ticket:        cfg.Ticket,
		TsSign:        cfg.TsSign,
		ClientCert:    cfg.ClientCert,
		PrivateKey:    cfg.PrivateKey,
		DtraitBlob:    cfg.DtraitBlob,
		SessionDtrait: cfg.SessionDtrait,
		Proxy:         cfg.Proxy,
	})
}

// imConvID / imConvType coerce the untyped values held in the IM event buffer
// (decoded from JSON, so integral numbers may be json.Number).
func imConvID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	}
	return ""
}

func imConvType(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	}
	return 0
}

// nonNilList guarantees list results serialize as [] instead of null.
func nonNilList(v []any) []any {
	if v == nil {
		return []any{}
	}
	return v
}

// Client returns the current session.
func (s *DouyinService) Client() *douyin.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// Reload rebuilds the session from the on-disk cookie (used after login or a
// cookie reset). A non-empty cookie string overrides the file.
func (s *DouyinService) Reload(cookieStr string) error {
	if cookieStr == "" {
		cookieStr = cookies.LoadCookie(s.cookiePath)
	}
	client, err := newClient(s.cfg, cookieStr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
	return nil
}

// persistCookies writes the current session cookies back to disk.
func (s *DouyinService) persistCookies() error {
	client := s.Client()
	if client == nil || client.CookieStr() == "" {
		return nil
	}
	return cookies.SaveCookie(s.cookiePath, client.CookieStr())
}

// --- Session / login -------------------------------------------------------

// CheckLoginStatus probes the session with a real authenticated request.
func (s *DouyinService) CheckLoginStatus(ctx context.Context) (map[string]any, error) {
	client := s.Client()
	if client == nil || len(client.Cookie.Names()) == 0 {
		return map[string]any{"logged_in": false, "reason": "未配置 Cookies"}, nil
	}
	uid, err := client.UID(ctx)
	if err != nil {
		return map[string]any{"logged_in": false, "reason": err.Error()}, nil
	}
	res := map[string]any{"logged_in": true, "uid": uid}
	if sec, err := client.SecUID(ctx); err == nil {
		res["sec_uid"] = sec
	}
	return res, nil
}

// CreateLoginQRCode starts a QR login session.
func (s *DouyinService) CreateLoginQRCode(ctx context.Context) (map[string]any, error) {
	return s.Client().CreateLoginQRCode(ctx)
}

// CheckLoginQRCode polls the QR login status.
func (s *DouyinService) CheckLoginQRCode(ctx context.Context, token string) (map[string]any, error) {
	res, err := s.Client().CheckQRCodeStatus(ctx, token)
	if err != nil {
		return nil, err
	}
	_ = s.persistCookies()
	return res, nil
}

// SendPhoneCode sends an SMS verification code.
func (s *DouyinService) SendPhoneCode(ctx context.Context, phone string) (map[string]any, error) {
	return s.Client().SendPhoneCode(ctx, phone)
}

// LoginByPhone completes the SMS login flow.
func (s *DouyinService) LoginByPhone(ctx context.Context, phone, code string) (map[string]any, error) {
	newClient, err := s.Client().LoginByPhone(ctx, phone, code)
	if err != nil {
		return nil, err
	}
	if newClient != nil {
		s.mu.Lock()
		s.client = newClient
		s.mu.Unlock()
	}
	_ = s.persistCookies()
	return map[string]any{"success": true, "message": "手机号登录成功"}, nil
}

// DeleteCookies removes the on-disk cookie and resets the session.
func (s *DouyinService) DeleteCookies(_ context.Context) (map[string]any, error) {
	if err := cookies.DeleteCookie(s.cookiePath); err != nil {
		return nil, err
	}
	reset, err := newClient(s.cfg, "")
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.client = reset
	s.mu.Unlock()
	return map[string]any{
		"cookie_path": s.cookiePath,
		"message":     "Cookies 已删除，登录状态已重置，下次操作需重新登录。",
	}, nil
}

// --- Users & works ---------------------------------------------------------

// GetUserInfo returns a user's profile.
func (s *DouyinService) GetUserInfo(ctx context.Context, user string) (map[string]any, error) {
	return s.Client().GetUserInfo(ctx, user)
}

// GetUserPosts returns one page of a user's works.
func (s *DouyinService) GetUserPosts(ctx context.Context, user, maxCursor, count string) (map[string]any, error) {
	_ = count
	return s.Client().GetUserWorkInfo(ctx, user, maxCursor)
}

// GetUserAllPosts returns a user's works up to a limit.
func (s *DouyinService) GetUserAllPosts(ctx context.Context, user string, limit int) ([]any, error) {
	res, err := s.Client().GetUserAllWorkInfo(ctx, user, limit)
	return nonNilList(res), err
}

// GetVideoDetail returns a work's detail.
func (s *DouyinService) GetVideoDetail(ctx context.Context, video string) (map[string]any, error) {
	return s.Client().GetWorkInfo(ctx, video)
}

// GetVideoComments returns one page of top-level comments.
func (s *DouyinService) GetVideoComments(ctx context.Context, video, cursor, count string) (map[string]any, error) {
	_ = count
	return s.Client().GetWorkOutComment(ctx, video, cursor)
}

// GetAllVideoComments returns top-level comments (optionally with replies).
func (s *DouyinService) GetAllVideoComments(ctx context.Context, video string, limit int, includeReplies bool) ([]any, error) {
	res, err := s.Client().GetWorkAllComment(ctx, video, limit, includeReplies)
	return nonNilList(res), err
}

// GetSubComments returns replies for a comment, identified by comment_id or author.
func (s *DouyinService) GetSubComments(ctx context.Context, video, commentID, cursor, count string) (map[string]any, error) {
	client := s.Client()
	comment, err := client.FindComment(ctx, video, commentID)
	if err != nil {
		return nil, err
	}
	return client.GetWorkInnerComment(ctx, comment, cursor, count)
}

// --- Search ----------------------------------------------------------------

// SearchVideosRequest carries the search filters supported by the Douyin web client.
type SearchVideosRequest struct {
	Keyword        string `json:"keyword"`
	Offset         string `json:"offset"`
	Count          int    `json:"count"`
	Channel        string `json:"channel"` // general | video
	SortType       string `json:"sort_type"`
	PublishTime    string `json:"publish_time"`
	FilterDuration string `json:"filter_duration"`
	SearchRange    string `json:"search_range"`
	ContentType    string `json:"content_type"`
}

// SearchVideos searches the general or video channel.
func (s *DouyinService) SearchVideos(ctx context.Context, req SearchVideosRequest) (any, error) {
	client := s.Client()
	if req.Channel == "video" {
		if req.Count > 0 {
			return client.SearchSomeVideoWork(ctx, req.Keyword, req.Count, req.SortType, req.PublishTime, req.FilterDuration, req.SearchRange)
		}
		offset := req.Offset
		if offset == "" {
			offset = "0"
		}
		return client.SearchVideoWork(ctx, req.Keyword, offset, "16", req.SortType, req.PublishTime, req.FilterDuration, req.SearchRange)
	}
	if req.Count > 0 {
		return client.SearchSomeGeneralWork(ctx, req.Keyword, req.Count, req.SortType, req.PublishTime, req.FilterDuration, req.SearchRange, req.ContentType)
	}
	offset := req.Offset
	if offset == "" {
		offset = "0"
	}
	return client.SearchGeneralWork(ctx, req.Keyword, req.SortType, req.PublishTime, offset, req.FilterDuration, req.SearchRange, req.ContentType)
}

// SearchUsers searches users.
func (s *DouyinService) SearchUsers(ctx context.Context, keyword, offset, count string) (any, error) {
	client := s.Client()
	if offset == "" && count == "" {
		return client.SearchSomeUser(ctx, keyword, 25)
	}
	if count == "" {
		count = "25"
	}
	if offset == "" {
		offset = "0"
	}
	return client.SearchUser(ctx, keyword, offset, count, "", "")
}

// SearchLives searches live rooms.
func (s *DouyinService) SearchLives(ctx context.Context, keyword, offset, count string) (any, error) {
	client := s.Client()
	if offset == "" && count == "" {
		return client.SearchSomeLive(ctx, keyword, 15)
	}
	if count == "" {
		count = "15"
	}
	if offset == "" {
		offset = "0"
	}
	return client.SearchLive(ctx, keyword, offset, count)
}

// SearchSuggest returns search-box suggestions.
func (s *DouyinService) SearchSuggest(ctx context.Context, keyword string) (map[string]any, error) {
	return s.Client().SearchSuggest(ctx, keyword)
}

// HotSearchBoard returns the hot-search board.
func (s *DouyinService) HotSearchBoard(ctx context.Context) (map[string]any, error) {
	return s.Client().HotSearchBoard(ctx)
}

// SearchChallenges searches hashtags/话题.
func (s *DouyinService) SearchChallenges(ctx context.Context, keyword, cursor, count string) (map[string]any, error) {
	return s.Client().SearchChallenges(ctx, keyword, cursor, count)
}

// --- Relations / notifications / collections -------------------------------

func (s *DouyinService) resolveUserIDs(ctx context.Context, user string) (string, string, error) {
	client := s.Client()
	info, err := client.GetUserInfo(ctx, user)
	if err != nil {
		return "", "", err
	}
	u, _ := info["user"].(map[string]any)
	uid := fmt.Sprintf("%v", u["uid"])
	sec := fmt.Sprintf("%v", u["sec_uid"])
	if uid == "<nil>" || sec == "<nil>" {
		return "", "", fmt.Errorf("未取到用户 uid/sec_uid")
	}
	return uid, sec, nil
}

// GetUserFollowers returns a page of a user's followers.
func (s *DouyinService) GetUserFollowers(ctx context.Context, user, maxTime, count string) (map[string]any, error) {
	uid, sec, err := s.resolveUserIDs(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.Client().GetUserFollowerList(ctx, uid, sec, maxTime, count)
}

// GetUserFollowing returns a page of the accounts a user follows.
func (s *DouyinService) GetUserFollowing(ctx context.Context, user, maxTime, count string) (map[string]any, error) {
	uid, sec, err := s.resolveUserIDs(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.Client().GetUserFollowingList(ctx, uid, sec, maxTime, count)
}

// GetAllUserFollowers auto-pages a user's followers up to limit.
func (s *DouyinService) GetAllUserFollowers(ctx context.Context, user string, limit int) ([]any, error) {
	uid, sec, err := s.resolveUserIDs(ctx, user)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().GetSomeUserFollowerList(ctx, uid, sec, limit)
	return nonNilList(res), err
}

// GetAllUserFollowing auto-pages the accounts a user follows up to limit.
func (s *DouyinService) GetAllUserFollowing(ctx context.Context, user string, limit int) ([]any, error) {
	uid, sec, err := s.resolveUserIDs(ctx, user)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().GetSomeUserFollowingList(ctx, uid, sec, limit)
	return nonNilList(res), err
}

// GetAllNotices auto-pages the notification list up to limit.
func (s *DouyinService) GetAllNotices(ctx context.Context, noticeGroup string, limit int) ([]any, error) {
	if strings.TrimSpace(noticeGroup) == "" {
		noticeGroup = douyin.NoticeGroupAll
	}
	res, err := s.Client().GetSomeNoticeList(ctx, limit, noticeGroup)
	return nonNilList(res), err
}

// NoticeCount returns unread notification counters.
func (s *DouyinService) NoticeCount(ctx context.Context) (map[string]any, error) {
	return s.Client().NoticeCount(ctx)
}

// NoticeDetail returns one notification's detail.
func (s *DouyinService) NoticeDetail(ctx context.Context, noticeID string) (map[string]any, error) {
	if strings.TrimSpace(noticeID) == "" {
		return nil, fmt.Errorf("缺少 notice_id_str（可从 get_notices / get_notice_digg_list 结果里取）")
	}
	return s.Client().NoticeDetail(ctx, noticeID)
}

// NoticeDelete removes a notification.
func (s *DouyinService) NoticeDelete(ctx context.Context, actionType, noticeID string) (map[string]any, error) {
	if strings.TrimSpace(actionType) == "" {
		actionType = "0"
	}
	return s.Client().NoticeDelete(ctx, actionType, noticeID)
}

// GetNotices returns the message-notification list.
func (s *DouyinService) GetNotices(ctx context.Context, noticeGroup, count string) (map[string]any, error) {
	if strings.TrimSpace(noticeGroup) == "" {
		noticeGroup = douyin.NoticeGroupAll
	}
	if strings.TrimSpace(count) == "" {
		count = "20"
	}
	return s.Client().GetNoticeList(ctx, "0", "0", count, noticeGroup)
}

// GetUserFavorites returns a user's favorited works.
func (s *DouyinService) GetUserFavorites(ctx context.Context, secUID, maxCursor, count string) (map[string]any, error) {
	return s.Client().GetUserFavorite(ctx, secUID, maxCursor, count)
}

// WatchHistory returns the "观看历史" tab.
func (s *DouyinService) WatchHistory(ctx context.Context, cursor, count string) (map[string]any, error) {
	return s.Client().GetWatchHistory(ctx, cursor, count)
}

// ClearWatchHistory empties the watch history.
func (s *DouyinService) ClearWatchHistory(ctx context.Context) (map[string]any, error) {
	return s.Client().ClearWatchHistory(ctx)
}

// WatchLater returns the "稍后再看" tab.
func (s *DouyinService) WatchLater(ctx context.Context, offset string) (map[string]any, error) {
	return s.Client().GetWatchLater(ctx, offset)
}

// Appointments returns the "我的预约" tab.
func (s *DouyinService) Appointments(ctx context.Context, appointmentType string, count int) (map[string]any, error) {
	return s.Client().GetAppointments(ctx, appointmentType, count)
}

// GetCollectList returns the collections list.
func (s *DouyinService) GetCollectList(ctx context.Context) (map[string]any, error) {
	return s.Client().GetCollectList(ctx)
}

// GetHomeFeed returns the recommendation feed.
func (s *DouyinService) GetHomeFeed(ctx context.Context, count, refreshIndex string) (map[string]any, error) {
	return s.Client().GetFeed(ctx, count, refreshIndex)
}

// --- Interactions ----------------------------------------------------------

// DiggVideo likes/unlikes a work.
func (s *DouyinService) DiggVideo(ctx context.Context, awemeID, diggType string) (map[string]any, error) {
	ok, err := s.Client().Digg(ctx, awemeID, diggType)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": ok}, nil
}

// PostVideoComment publishes or replies to a comment.
func (s *DouyinService) PostVideoComment(ctx context.Context, awemeID, content, replyID string) (map[string]any, error) {
	return s.Client().PublishComment(ctx, awemeID, content, replyID)
}

// CollectVideo collects/uncollects a work.
func (s *DouyinService) CollectVideo(ctx context.Context, awemeID, action string) (map[string]any, error) {
	return s.Client().CollectAweme(ctx, awemeID, action)
}

// MoveCollectVideo moves a work into another collection.
func (s *DouyinService) MoveCollectVideo(ctx context.Context, awemeID, name, id string) (map[string]any, error) {
	return s.Client().MoveCollectAweme(ctx, awemeID, name, id)
}

// RemoveCollectVideo removes a work from a collection.
func (s *DouyinService) RemoveCollectVideo(ctx context.Context, awemeID, name, id string) (map[string]any, error) {
	return s.Client().RemoveCollectAweme(ctx, awemeID, name, id)
}

// --- Live ------------------------------------------------------------------

// LiveInfo returns live room info including stream URLs.
func (s *DouyinService) LiveInfo(ctx context.Context, webRID string) (map[string]any, error) {
	return s.Client().GetLiveInfo(ctx, webRID)
}

// StartLiveListen begins collecting live-room events. Start() blocks until the
// listener stops, so it runs in a goroutine while the HTTP/MCP call returns.
func (s *DouyinService) StartLiveListen(ctx context.Context, webRID string) (map[string]any, error) {
	// 空房间号直接拒绝：否则会连着空 id 去建连，用户只看到一个没有事件、
	// 也没有错误的"运行中"监听。
	if strings.TrimSpace(webRID) == "" {
		return nil, invalidInput("web_rid 不能为空")
	}

	s.liveMu.Lock()
	if s.live != nil {
		s.liveMu.Unlock()
		return nil, fmt.Errorf("直播间监听已在运行")
	}
	listener := douyin.NewLiveListener(s.Client(), webRID)
	s.live = listener
	s.liveEvents = nil
	s.liveMu.Unlock()

	events := listener.Events()
	done := listener.Done()
	go func() {
		for {
			select {
			case ev := <-events:
				s.liveMu.Lock()
				s.liveEvents = append(s.liveEvents, ev.Data)
				s.liveMu.Unlock()
			case <-done:
				return
			}
		}
	}()
	go func() {
		if err := listener.Start(context.Background()); err != nil {
			logrus.WithError(err).Warn("直播间监听已退出")
		}
	}()
	return map[string]any{"success": true, "message": "直播间监听已启动"}, nil
}

// StopLiveListen stops the live listener.
func (s *DouyinService) StopLiveListen() map[string]any {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	if s.live != nil {
		s.live.Stop()
		s.live = nil
	}
	return map[string]any{"success": true, "message": "直播间监听已停止"}
}

// LiveEvents returns the events collected so far.
func (s *DouyinService) LiveEvents() []map[string]any {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	out := make([]map[string]any, len(s.liveEvents))
	copy(out, s.liveEvents)
	return out
}

// SendLiveComment sends a danmaku message in a live room.
func (s *DouyinService) SendLiveComment(ctx context.Context, roomID, content string) (map[string]any, error) {
	return s.Client().SendMsgInRoom(ctx, roomID, content)
}

// LikeLiveRoom sends room likes.
func (s *DouyinService) LikeLiveRoom(ctx context.Context, roomID, count string) (map[string]any, error) {
	return s.Client().DiggLiveRoom(ctx, roomID, count)
}

// LiveContributionRank returns the room contribution ranking.
func (s *DouyinService) LiveContributionRank(ctx context.Context, roomID, anchorID string) (map[string]any, error) {
	return s.Client().GetLiveContributionRank(ctx, roomID, anchorID)
}

// LivePKRank returns the PK ranking for a side.
func (s *DouyinService) LivePKRank(ctx context.Context, webRID, side string) (map[string]any, error) {
	return s.Client().GetLivePKRank(ctx, webRID, side)
}

// LiveRankList returns the combined rank list.
func (s *DouyinService) LiveRankList(ctx context.Context, roomID, anchorID, secAnchorID string) (map[string]any, error) {
	return s.Client().GetRankList(ctx, roomID, anchorID, secAnchorID)
}

// LiveProduction returns a room's e-commerce product list.
func (s *DouyinService) LiveProduction(ctx context.Context, pageURL, roomID, authorID, offset string) (map[string]any, error) {
	return s.Client().GetLiveProduction(ctx, pageURL, roomID, authorID, offset)
}

// AllLiveProduction returns the full product list for a room page.
func (s *DouyinService) AllLiveProduction(ctx context.Context, pageURL string) ([]any, error) {
	res, err := s.Client().GetAllLiveProduction(ctx, pageURL)
	return nonNilList(res), err
}

// LiveProductionDetail returns one promotion's detail.
func (s *DouyinService) LiveProductionDetail(ctx context.Context, pageURL, promotionID, originType string) (map[string]any, error) {
	return s.Client().GetLiveProductionDetail(ctx, pageURL, promotionID, originType)
}

// ProductCommentCounter returns a product's comment statistics.
func (s *DouyinService) ProductCommentCounter(ctx context.Context, productID, shopID, statID string) (map[string]any, error) {
	return s.Client().GetProductCommentCounter(ctx, productID, shopID, statID)
}

// LiveRoomEnter warms up a live room session (room enter request).
func (s *DouyinService) LiveRoomEnter(ctx context.Context, webRID string) (map[string]any, error) {
	return s.Client().GetLiveRoomEnter(ctx, webRID)
}

// LiveLinkmicList returns the room's link-mic list.
func (s *DouyinService) LiveLinkmicList(ctx context.Context, roomID, channelID string) (map[string]any, error) {
	return s.Client().GetLiveLinkmicList(ctx, roomID, channelID)
}

// LivePKContext returns the current PK context for a room.
func (s *DouyinService) LivePKContext(ctx context.Context, webRID, channelID string) (map[string]any, error) {
	return s.Client().GetLivePKContext(ctx, webRID, channelID)
}

// LiveThousandTicketRank returns the thousand-ticket contribution rank.
func (s *DouyinService) LiveThousandTicketRank(ctx context.Context, roomID, webRID string) (map[string]any, error) {
	return s.Client().GetLiveThousandTicketRank(ctx, roomID, webRID)
}

// LivePKContributionRank returns the PK contribution rank for a channel.
func (s *DouyinService) LivePKContributionRank(ctx context.Context, channelID, anchorID string) (map[string]any, error) {
	return s.Client().GetLivePKContributionRank(ctx, channelID, anchorID)
}

// ProductComments returns comments for a product.
func (s *DouyinService) ProductComments(ctx context.Context, productID, shopID, cursor string) (map[string]any, error) {
	return s.Client().GetProductComments(ctx, productID, shopID, cursor)
}

// --- Direct messages -------------------------------------------------------

// CreateConversation opens a conversation with a user.
func (s *DouyinService) CreateConversation(ctx context.Context, toUserID int64) (map[string]any, error) {
	convID, shortID, ticket, err := s.Client().CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"conversation_id": convID, "conversation_short_id": shortID, "ticket": ticket}, nil
}

// ConversationList queries a conversation's info.
func (s *DouyinService) ConversationList(ctx context.Context, toUserID int64, shortID int64) (map[string]any, error) {
	return s.Client().GetConversationList(ctx, toUserID, shortID)
}

// SendDM creates (if needed) and sends a text direct message.
func (s *DouyinService) SendDM(ctx context.Context, toUserID int64, content string) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	res, err := client.SendMsg(ctx, convID, shortID, ticket, content)
	if err != nil {
		return nil, err
	}
	res["conversation_id"] = convID
	res["conversation_short_id"] = shortID
	return res, nil
}

// SendDMMedia creates (if needed) and sends an image/video/audio/file message.
// kind is one of image|video|audio|file.
func (s *DouyinService) SendDMMedia(ctx context.Context, toUserID int64, kind, path string) (map[string]any, error) {
	// 先校验类型：不支持的 kind 必须在创建会话（发请求）之前就被拒绝，
	// 否则一次非法调用会先在一个陌生人那里建出一个空会话。
	if !douyin.ValidIMMediaKind(kind) {
		return nil, invalidInput("不支持的私信媒体类型: %s（可选 %s）", kind, douyin.IMMediaKindList())
	}

	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	var res map[string]any
	switch kind {
	case "image":
		res, err = client.SendIMImage(ctx, convID, shortID, ticket, path)
	case "video":
		res, err = client.SendIMVideo(ctx, convID, shortID, ticket, path)
	case "audio":
		res, err = client.SendIMAudio(ctx, convID, shortID, ticket, path)
	case "file":
		res, err = client.SendIMFile(ctx, convID, shortID, ticket, path)
	default:
		// 理论不可达（ValidIMMediaKind 已过滤），留作新增类型时的兜底。
		return nil, invalidInput("不支持的私信媒体类型: %s（可选 %s）", kind, douyin.IMMediaKindList())
	}
	if err != nil {
		return nil, err
	}
	res["conversation_id"] = convID
	res["conversation_short_id"] = shortID
	return res, nil
}

// SendDMSticker sends a big-emoji/sticker message.
func (s *DouyinService) SendDMSticker(ctx context.Context, toUserID int64, sticker map[string]any) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return client.SendIMSticker(ctx, convID, shortID, ticket, sticker)
}

// SendDMCard sends a custom card message.
func (s *DouyinService) SendDMCard(ctx context.Context, toUserID int64, card map[string]any) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return client.SendIMCard(ctx, convID, shortID, ticket, card)
}

// ShareDMAweme shares a video work into a conversation.
func (s *DouyinService) ShareDMAweme(ctx context.Context, toUserID int64, awemeID string) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return client.ShareAweme(ctx, convID, shortID, ticket, awemeID)
}

// ShareDMWeb shares a web link into a conversation.
func (s *DouyinService) ShareDMWeb(ctx context.Context, toUserID int64, url string) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return client.ShareWeb(ctx, convID, shortID, ticket, url)
}

// SendDMUserCard shares a user card into a conversation.
func (s *DouyinService) SendDMUserCard(ctx context.Context, toUserID int64, secUID string) (map[string]any, error) {
	client := s.Client()
	convID, shortID, ticket, err := client.CreateConversation(ctx, toUserID)
	if err != nil {
		return nil, err
	}
	return client.SendUserCard(ctx, convID, shortID, ticket, secUID)
}

// StartIMListen begins receiving direct messages. Start() blocks, so it runs in
// a goroutine while the HTTP/MCP call returns.
func (s *DouyinService) StartIMListen(ctx context.Context) (map[string]any, error) {
	s.imMu.Lock()
	if s.im != nil {
		s.imMu.Unlock()
		return nil, fmt.Errorf("私信监听已在运行")
	}
	receiver, err := douyin.NewIMReceiver(s.Client())
	if err != nil {
		s.imMu.Unlock()
		return nil, err
	}
	s.im = receiver
	s.imMessages = nil
	s.imStop = make(chan struct{})
	stop := s.imStop
	s.imMu.Unlock()

	events := receiver.Events()
	go func() {
		for {
			select {
			case ev := <-events:
				s.imMu.Lock()
				s.imMessages = append(s.imMessages, map[string]any{
					"message_index":     ev.MessageIndex,
					"conversation_id":   ev.ConversationID,
					"conversation_type": ev.ConversationType,
					"notify_type":       ev.NotifyType,
					"sender":            ev.Sender,
					"message_type":      ev.MessageType,
					"content":           ev.Content,
				})
				s.imMu.Unlock()
			case <-stop:
				return
			}
		}
	}()
	go func() {
		if err := receiver.Start(context.Background()); err != nil {
			logrus.WithError(err).Warn("私信监听已退出")
		}
	}()
	return map[string]any{"success": true, "message": "私信监听已启动"}, nil
}

// StopIMListen stops the IM receiver.
func (s *DouyinService) StopIMListen() map[string]any {
	s.imMu.Lock()
	defer s.imMu.Unlock()
	if s.im != nil {
		s.im.Stop()
		s.im = nil
	}
	if s.imStop != nil {
		close(s.imStop)
		s.imStop = nil
	}
	return map[string]any{"success": true, "message": "私信监听已停止"}
}

// IMMessages returns the messages received so far. conversationID and
// conversationType (1=private, 2=group; 0=any) filter the result so a single
// group/chat can be read without pulling every conversation.
func (s *DouyinService) IMMessages(conversationID string, conversationType int) []map[string]any {
	s.imMu.Lock()
	defer s.imMu.Unlock()
	out := make([]map[string]any, 0, len(s.imMessages))
	for _, m := range s.imMessages {
		if conversationID != "" && imConvID(m["conversation_id"]) != conversationID {
			continue
		}
		if conversationType != 0 && imConvType(m["conversation_type"]) != int64(conversationType) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// --- IM: conversations, history, participants ------------------------------

// resolveConversation finds a conversation by id, short id or display name.
func (s *DouyinService) resolveConversation(ctx context.Context, idOrName string) (douyin.IMConversation, error) {
	convs, err := s.Client().IMListConversations(ctx)
	if err != nil {
		return douyin.IMConversation{}, err
	}
	needle := strings.TrimSpace(idOrName)
	if needle == "" {
		return douyin.IMConversation{}, fmt.Errorf("缺少会话 id 或群名")
	}
	lower := strings.ToLower(needle)
	// exact id / short id first
	if i := slices.IndexFunc(convs, func(c douyin.IMConversation) bool {
		return c.ConversationID == needle || strconv.FormatInt(c.ConversationShortID, 10) == needle
	}); i >= 0 {
		return convs[i], nil
	}
	// then name (exact, then substring)
	if i := slices.IndexFunc(convs, func(c douyin.IMConversation) bool {
		return strings.EqualFold(c.Name, needle)
	}); i >= 0 {
		return convs[i], nil
	}
	if i := slices.IndexFunc(convs, func(c douyin.IMConversation) bool {
		return strings.Contains(strings.ToLower(c.Name), lower)
	}); i >= 0 {
		return convs[i], nil
	}
	return douyin.IMConversation{}, fmt.Errorf("未找到会话: %s（可先用 list_conversations 查看）", idOrName)
}

// ListConversations returns every conversation (group + direct) with names.
func (s *DouyinService) ListConversations(ctx context.Context) ([]douyin.IMConversation, error) {
	fresh, err := s.Client().IMListConversations(ctx)
	if err != nil {
		return nil, err
	}
	return s.mergeConversations(fresh), nil
}

// mergeConversations folds the server delta into the local cache and returns
// the union: the delta first, then conversations only known from earlier syncs
// (flagged cached=true).
func (s *DouyinService) mergeConversations(fresh []douyin.IMConversation) []douyin.IMConversation {
	s.convMu.Lock()
	defer s.convMu.Unlock()
	seen := make(map[string]bool, len(fresh))
	for _, conv := range fresh {
		if conv.ConversationID == "" {
			continue
		}
		seen[conv.ConversationID] = true
		// The cached copy is used for id/name lookup and unread state; message
		// bodies stay out of the state file (they are the bulk of its size).
		stored := conv
		stored.LastMessages = nil
		s.convCache[conv.ConversationID] = stored
	}
	out := make([]douyin.IMConversation, 0, len(fresh)+len(s.convCache))
	out = append(out, fresh...)
	for id, conv := range s.convCache {
		if seen[id] {
			continue
		}
		conv.Cached = true
		out = append(out, conv)
	}
	s.saveConversationCacheLocked()
	return out
}

// loadConversationCache restores the accumulated conversation list.
func (s *DouyinService) loadConversationCache() {
	raw, err := os.ReadFile(s.convCachePath)
	if err != nil {
		return
	}
	var stored struct {
		Conversations []douyin.IMConversation `json:"conversations"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		return
	}
	for _, conv := range stored.Conversations {
		if conv.ConversationID != "" {
			s.convCache[conv.ConversationID] = conv
		}
	}
}

// saveConversationCacheLocked persists the cache (best effort, atomic rename).
func (s *DouyinService) saveConversationCacheLocked() {
	if s.convCachePath == "" {
		return
	}
	payload, err := json.MarshalIndent(struct {
		Conversations []douyin.IMConversation `json:"conversations"`
	}{Conversations: mapValues(s.convCache)}, "", "  ")
	if err != nil {
		return
	}
	tmp := s.convCachePath + ".tmp"
	if os.WriteFile(tmp, payload, 0o600) == nil {
		_ = os.Rename(tmp, s.convCachePath)
	}
}

func mapValues(m map[string]douyin.IMConversation) []douyin.IMConversation {
	out := make([]douyin.IMConversation, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// ConversationInfo returns one conversation's info by id or name.
func (s *DouyinService) ConversationInfo(ctx context.Context, idOrName string) (douyin.IMConversation, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return douyin.IMConversation{}, err
	}
	info, err := s.Client().IMConversationInfo(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType)
	if err != nil {
		return conv, err
	}
	if info.Name == "" {
		info.Name = conv.Name
	}
	return info, nil
}

// ConversationHistory returns a page of messages (group or direct message).
func (s *DouyinService) ConversationHistory(ctx context.Context, idOrName string, cursor int64, count int, unreadOnly bool) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	if unreadOnly {
		return s.unreadHistory(ctx, conv)
	}
	msgs, next, err := s.Client().IMConversationHistory(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, cursor, count)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"conversation_id":   conv.ConversationID,
		"conversation_name": conv.Name,
		"conversation_type": conv.ConversationType,
		"messages":          msgs,
		"next_cursor":       next,
	}, nil
}

// unreadHistory returns the messages the conversation has not marked read yet
// (as many as the server's unread counter reports), fetched newest-first from
// the history API and returned in chronological order.
func (s *DouyinService) unreadHistory(ctx context.Context, conv douyin.IMConversation) (map[string]any, error) {
	const (
		pageSize  = 50
		maxPages  = 10
		maxUnread = 500
	)
	want := int(conv.Unread)
	if want > maxUnread {
		want = maxUnread
	}
	out := map[string]any{
		"conversation_id":   conv.ConversationID,
		"conversation_name": conv.Name,
		"conversation_type": conv.ConversationType,
		"unread":            conv.Unread,
		"unread_classes":    conv.UnreadClasses,
		"messages":          []any{},
	}
	if conv.Unread <= 0 {
		out["note"] = "该会话没有未读消息"
		return out, nil
	}

	var all []douyin.IMChatMessage // oldest → newest
	cursor := int64(0)
	for range maxPages {
		msgs, next, err := s.Client().IMConversationHistory(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, cursor, pageSize)
		if err != nil {
			if len(all) == 0 {
				return nil, err
			}
			break
		}
		all = append(msgs, all...)
		if len(all) >= want || next == 0 || next == cursor {
			break
		}
		cursor = next
	}
	out["fetched"] = len(all)
	if len(all) > want {
		all = all[len(all)-want:]
	}
	out["messages"] = all
	out["truncated"] = int(conv.Unread) > maxUnread
	return out, nil
}

// ConversationParticipants lists the members of a group conversation.
func (s *DouyinService) ConversationParticipants(ctx context.Context, idOrName string, offset int64, count int) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	members, err := s.Client().IMParticipants(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, offset, count)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"conversation_id":   conv.ConversationID,
		"conversation_name": conv.Name,
		"count":             len(members),
		"participants":      members,
	}, nil
}

// SetConversationSetting pins/unpins or mutes/unmutes a conversation.
func (s *DouyinService) SetConversationSetting(ctx context.Context, idOrName string, pin, mute *bool) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().IMSetConversationSetting(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, pin, mute)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["conversation_name"] = conv.Name
	if pin != nil {
		res["pinned"] = *pin
	}
	if mute != nil {
		res["muted"] = *mute
	}
	return res, nil
}

// LeaveConversation quits a group conversation.
func (s *DouyinService) LeaveConversation(ctx context.Context, idOrName string) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	if conv.ConversationType != 2 {
		return nil, fmt.Errorf("只有群聊可以退出（当前会话 type=%d）", conv.ConversationType)
	}
	res, err := s.Client().IMLeaveConversation(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["conversation_name"] = conv.Name
	res["left"] = true
	return res, nil
}

// ShareConversation returns a conversation's share info: the ids needed to
// reference the group plus a web link the app itself uses for the chat entry.
func (s *DouyinService) ShareConversation(ctx context.Context, idOrName string) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	link := fmt.Sprintf("https://www.douyin.com/friend?conversation_id=%s&conversation_type=%d", conv.ConversationID, conv.ConversationType)
	return map[string]any{
		"conversation_id":       conv.ConversationID,
		"conversation_short_id": conv.ConversationShortID,
		"conversation_type":     conv.ConversationType,
		"name":                  conv.Name,
		"link":                  link,
		"share_text":            fmt.Sprintf("邀请你加入群聊「%s」：%s", conv.Name, link),
	}, nil
}

// StrangerConversations lists conversations from non-followed accounts.
func (s *DouyinService) StrangerConversations(ctx context.Context) (map[string]any, error) {
	return s.Client().IMStrangerConversations(ctx)
}

// MarkConversationRead clears the unread marker up to the newest message.
func (s *DouyinService) MarkConversationRead(ctx context.Context, idOrName string) (map[string]any, error) {
	conv, err := s.resolveConversation(ctx, idOrName)
	if err != nil {
		return nil, err
	}
	index := int64(0)
	if msgs, _, err := s.Client().IMConversationHistory(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, 0, 1); err == nil && len(msgs) > 0 {
		index = msgs[0].Index
	}
	res, err := s.Client().IMMarkRead(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, index)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = map[string]any{}
	}
	res["conversation_id"] = conv.ConversationID
	res["conversation_name"] = conv.Name
	res["index"] = index
	return res, nil
}

// IMUserInfo resolves nicknames and avatars for sec_uids.
func (s *DouyinService) IMUserInfo(ctx context.Context, secUIDs []string) ([]any, error) {
	res, err := s.Client().IMUserInfo(ctx, secUIDs)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(res))
	for _, v := range res {
		out = append(out, v)
	}
	return out, nil
}

// --- Creator ---------------------------------------------------------------

// PublishContent publishes an image collection to the creator center.
func (s *DouyinService) PublishContent(ctx context.Context, req douyin.PublishImageRequest) (map[string]any, error) {
	return s.Client().PublishImageContent(ctx, req)
}

// PublishVideo publishes a video to the creator center.
func (s *DouyinService) PublishVideo(ctx context.Context, req douyin.PublishVideoRequest) (map[string]any, error) {
	return s.Client().PublishVideoContent(ctx, req)
}
