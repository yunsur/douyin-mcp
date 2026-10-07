package douyin

// Hermetic (stub-transport) tests for the misc endpoint families:
// social tab / hot-music / my-profile / ecom SKU / notice digg / relation.
// Every test drives Client methods through a scripted Transport and asserts on
// the recorded request (method, host, path, query, body, headers) plus the
// decoded response, so they run offline and deterministically.

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- shared helpers --------------------------------------------------------

// miscUnitURL parses a recorded request URL.
func miscUnitURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("解析记录到的 URL 失败 %q: %v", raw, err)
	}
	return u
}

// miscUnitQuery decodes the query of a recorded request.
func miscUnitQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	return miscUnitURL(t, raw).Query()
}

// miscUnitReqsAt returns recorded requests whose URL path equals path.
func miscUnitReqsAt(st *stubTransport, path string) []stubRequest {
	var out []stubRequest
	for _, r := range st.requests() {
		if u, err := url.Parse(r.URL); err == nil && u.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// miscUnitLastAt returns the most recent recorded request to path.
func miscUnitLastAt(t *testing.T, st *stubTransport, path string) stubRequest {
	t.Helper()
	reqs := miscUnitReqsAt(st, path)
	if len(reqs) == 0 {
		t.Fatalf("没有记录到对 %s 的请求", path)
	}
	return reqs[len(reqs)-1]
}

// miscUnitFormBody parses a recorded form-urlencoded request body.
func miscUnitFormBody(t *testing.T, r stubRequest) url.Values {
	t.Helper()
	v, err := url.ParseQuery(string(r.Body))
	if err != nil {
		t.Fatalf("解析表单 body 失败 %q: %v", string(r.Body), err)
	}
	return v
}

// miscUnitEpochSeconds asserts v is a plausible current epoch-second value.
func miscUnitEpochSeconds(t *testing.T, v string) {
	t.Helper()
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		t.Fatalf("不是时间戳: %q (%v)", v, err)
	}
	now := time.Now().Unix()
	if n < now-60 || n > now+60 {
		t.Fatalf("时间戳 %d 不是当前秒级时间（now=%d）", n, now)
	}
}

// --- social tab ------------------------------------------------------------

func TestSocialFollowFeedRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/follow/feed/"
	c, st := newStubJSON(t, map[string]any{
		"status_code": 0,
		"aweme_list":  []any{map[string]any{"aweme_id": "111"}},
		"has_more":    1,
	})
	res, err := c.GetFollowFeed(t.Context(), "", "")
	if err != nil {
		t.Fatalf("GetFollowFeed: %v", err)
	}
	if toInt64(res["status_code"]) != 0 || len(duSlice(res["aweme_list"])) != 1 {
		t.Fatalf("响应解码异常: status=%v list=%v", res["status_code"], res["aweme_list"])
	}

	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{"cursor": "0", "count": "20", "level": "1", "pull_type": "0"} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	if ref := req.Header("referer"); ref != douyinBase+"/?recommend=1" {
		t.Errorf("referer=%q", ref)
	}
}

func TestSocialFamiliarFeedRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/familiar/feed/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "aweme_list": []any{}, "has_more": 0})
	if _, err := c.GetFamiliarFeed(t.Context(), "", ""); err != nil {
		t.Fatalf("GetFamiliarFeed: %v", err)
	}

	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	if len(req.Body) != 0 {
		t.Errorf("朋友流参数应在 query（body=%q）", req.Body)
	}
	if ct := req.Header("content-type"); !strings.Contains(ct, "x-www-form-urlencoded") {
		t.Errorf("content-type=%q", ct)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{
		"cursor": "0", "count": "20", "level": "1",
		"address_book_access": "2", "gps_access": "2", "pull_type": "0",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
}

func TestSocialFollowLiveShape(t *testing.T) {
	t.Run("top", func(t *testing.T) {
		const path = "/webcast/feed/follow_top/"
		c, st := newStubJSON(t, map[string]any{"status_code": 0})
		if _, err := c.GetFollowLiveTop(t.Context()); err != nil {
			t.Fatalf("GetFollowLiveTop: %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if req.Method != "POST" {
			t.Errorf("method=%s want POST", req.Method)
		}
		if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
			t.Errorf("host=%s", host)
		}
		body := miscUnitFormBody(t, req)
		for k, want := range map[string]string{
			"enter_source": "homepage_pc_followtop",
			"need_map":     "1",
			"source_key":   "web_homepage_follow_top",
		} {
			if got := body.Get(k); got != want {
				t.Errorf("body %s=%q want %q", k, got, want)
			}
		}
	})

	t.Run("feed", func(t *testing.T) {
		const path = "/webcast/web/feed/follow/"
		c, st := newStubJSON(t, map[string]any{"status_code": 0})
		if _, err := c.GetFollowLiveFeed(t.Context(), ""); err != nil {
			t.Fatalf("GetFollowLiveFeed: %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if req.Method != "GET" {
			t.Errorf("method=%s want GET", req.Method)
		}
		if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
			t.Errorf("host=%s", host)
		}
		if got := miscUnitQuery(t, req.URL).Get("scene"); got != "aweme_pc_follow_top" {
			t.Errorf("scene=%q want 默认值", got)
		}
		if _, err := c.GetFollowLiveFeed(t.Context(), "custom_scene"); err != nil {
			t.Fatalf("GetFollowLiveFeed(custom): %v", err)
		}
		if got := miscUnitQuery(t, miscUnitLastAt(t, st, path).URL).Get("scene"); got != "custom_scene" {
			t.Errorf("scene=%q want custom_scene", got)
		}
	})
}

func TestSocialDanmakuRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/danmaku/get_v2/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "danmaku_list": []any{}})
	if _, err := c.GetDanmaku(t.Context(), "700", "", "9000", "9000", "tok"); err != nil {
		t.Fatalf("GetDanmaku: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{
		"app_name": "aweme", "format": "json", "group_id": "700", "item_id": "700",
		"start_time": "0", "end_time": "9000", "authentication_token": "tok", "duration": "9000",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	if ref := req.Header("referer"); ref != douyinBase+"/video/700" {
		t.Errorf("referer=%q", ref)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.GetDanmaku(t.Context(), "", "0", "1", "1", ""); err == nil {
		t.Fatal("空 item_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st2, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestSocialDanmakuConfRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/danmaku/conf/get/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "danmaku_authentication_token": "t"})
	if _, err := c.GetDanmakuConf(t.Context(), ""); err != nil {
		t.Fatalf("GetDanmakuConf: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	if q.Get("conf_end_type") != "4" {
		t.Errorf("conf_end_type=%q", q.Get("conf_end_type"))
	}
	if q.Get("hardware_concurrency") != "10" {
		t.Errorf("hardware_concurrency=%q want 默认 10", q.Get("hardware_concurrency"))
	}
}

func TestSocialReportHistoryWriteShape(t *testing.T) {
	const path = "/aweme/v1/web/history/write/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.ReportHistoryWrite(t.Context(), "author-1", "aweme-1"); err != nil {
		t.Fatalf("ReportHistoryWrite: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	body := miscUnitFormBody(t, req)
	if body.Get("author_id") != "author-1" || body.Get("aweme_id") != "aweme-1" {
		t.Errorf("body=%v", body)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.ReportHistoryWrite(t.Context(), "author-1", ""); err == nil {
		t.Fatal("空 aweme_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st2, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestSocialReportAwemeStatsShape(t *testing.T) {
	const path = "/aweme/v2/web/aweme/stats/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.ReportAwemeStats(t.Context(), "item-1", "", "", ""); err != nil {
		t.Fatalf("ReportAwemeStats: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	body := miscUnitFormBody(t, req)
	for k, want := range map[string]string{"item_id": "item-1", "aweme_type": "0", "play_delta": "1", "source": "0"} {
		if got := body.Get(k); got != want {
			t.Errorf("body %s=%q want %q", k, got, want)
		}
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.ReportAwemeStats(t.Context(), "item-2", "1", "5", "3"); err != nil {
		t.Fatalf("ReportAwemeStats(custom): %v", err)
	}
	body2 := miscUnitFormBody(t, miscUnitLastAt(t, st2, path))
	for k, want := range map[string]string{"item_id": "item-2", "aweme_type": "1", "play_delta": "5", "source": "3"} {
		if got := body2.Get(k); got != want {
			t.Errorf("body %s=%q want %q", k, got, want)
		}
	}

	c3, st3 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c3.ReportAwemeStats(t.Context(), "", "0", "1", "0"); err == nil {
		t.Fatal("空 item_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st3, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestSocialMarkFollowingSeenShape(t *testing.T) {
	const path = "/aweme/v1/following/list/item/seen/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.MarkFollowingSeen(t.Context(), "111,222", ""); err != nil {
		t.Fatalf("MarkFollowingSeen: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	if q.Get("item_id_list") != "111,222" || q.Get("type") != "1" {
		t.Errorf("query item_id_list=%q type=%q", q.Get("item_id_list"), q.Get("type"))
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.MarkFollowingSeen(t.Context(), "333", "2"); err != nil {
		t.Fatalf("MarkFollowingSeen(custom): %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("type"); got != "2" {
		t.Errorf("type=%q want 2", got)
	}

	c3, st3 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c3.MarkFollowingSeen(t.Context(), "", "1"); err == nil {
		t.Fatal("空 item_id_list 必须报错")
	}
	if n := len(miscUnitReqsAt(st3, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestSocialReportSeriesWatchShape(t *testing.T) {
	const path = "/aweme/v1/web/series/watch/record/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.ReportSeriesWatch(t.Context(), "item-9", "series-9", ""); err != nil {
		t.Fatalf("ReportSeriesWatch: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{"item_id": "item-9", "series_id": "series-9", "episode": "1"} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.ReportSeriesWatch(t.Context(), "item-9", "series-9", "3"); err != nil {
		t.Fatalf("ReportSeriesWatch(custom): %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("episode"); got != "3" {
		t.Errorf("episode=%q want 3", got)
	}

	c3, st3 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c3.ReportSeriesWatch(t.Context(), "", "series-9", "1"); err == nil {
		t.Fatal("空 item_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st3, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

// --- hot music -------------------------------------------------------------

func TestMusicStatusErrorMapping(t *testing.T) {
	if err := hotMusicStatusError("op", map[string]any{"status_code": int64(0)}); err != nil {
		t.Fatalf("status 0 不应报错: %v", err)
	}
	err := hotMusicStatusError("获取音乐详情", map[string]any{"status_code": int64(5), "status_msg": "音乐不存在"})
	if err == nil {
		t.Fatal("非 0 status_code 必须报错")
	}
	if !strings.Contains(err.Error(), "5") || !strings.Contains(err.Error(), "音乐不存在") {
		t.Fatalf("错误应带 code + msg, got %v", err)
	}

	// 端点路径也要把业务失败转成 error（不能当成空成功）。
	c, _ := newStubJSON(t, map[string]any{"status_code": 5, "status_msg": "音乐不存在"})
	if _, err := c.MusicDetail(t.Context(), "555", ""); err == nil {
		t.Fatal("MusicDetail 业务失败必须返回 error")
	}
}

func TestMusicAwemeRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/music/aweme/"
	c, st := newStubJSON(t, map[string]any{
		"status_code": 0,
		"aweme_list":  []any{map[string]any{"aweme_id": "1"}},
	})
	res, err := c.MusicAweme(t.Context(), "999", "", "")
	if err != nil {
		t.Fatalf("MusicAweme: %v", err)
	}
	if len(duSlice(res["aweme_list"])) != 1 {
		t.Fatalf("aweme_list=%v", res["aweme_list"])
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{"music_id": "999", "cursor": "0", "count": "12"} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	if q.Get("x-secsdk-web-signature") == "" {
		t.Error("音乐接口必须在 query 上带 x-secsdk-web-signature")
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.MusicAweme(t.Context(), "", "0", "12"); err == nil {
		t.Fatal("空 music_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st2, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestMusicDetailRequestShape(t *testing.T) {
	const path = "/aweme/v1/web/music/detail/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "music_info": map[string]any{"id_str": "555"}})
	res, err := c.MusicDetail(t.Context(), "555", "")
	if err != nil {
		t.Fatalf("MusicDetail: %v", err)
	}
	if duMap(res["music_info"])["id_str"] != "555" {
		t.Fatalf("music_info=%v", res["music_info"])
	}
	req := miscUnitLastAt(t, st, path)
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	if q.Get("music_id") != "555" || q.Get("scene") != "1" {
		t.Errorf("query music_id=%q scene=%q", q.Get("music_id"), q.Get("scene"))
	}
	if q.Get("x-secsdk-web-signature") == "" {
		t.Error("detail 接口必须带 x-secsdk-web-signature")
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.MusicDetail(t.Context(), "555", "2"); err != nil {
		t.Fatalf("MusicDetail(scene=2): %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("scene"); got != "2" {
		t.Errorf("scene=%q want 2", got)
	}
}

func TestMusicListCollectionShape(t *testing.T) {
	const path = "/aweme/v1/web/music/listcollection/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "mc_list": []any{}})
	if _, err := c.MusicListCollection(t.Context(), "", ""); err != nil {
		t.Fatalf("MusicListCollection: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if eq := miscUnitURL(t, req.URL).Host; eq != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", eq)
	}
	q := miscUnitQuery(t, req.URL)
	if q.Get("cursor") != "0" || q.Get("count") != "20" {
		t.Errorf("query cursor=%q count=%q", q.Get("cursor"), q.Get("count"))
	}
	if ref := req.Header("referer"); ref != profileSelfRefer+"?showTab=favorite_music" {
		t.Errorf("referer=%q", ref)
	}
}

func TestMusicCollectWriteShape(t *testing.T) {
	const path = "/aweme/v1/web/music/collect/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	c.SetUID(123456789)
	if _, err := c.CollectMusic(t.Context(), "555", "", ""); err != nil {
		t.Fatalf("CollectMusic: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	body := miscUnitFormBody(t, req)
	for k, want := range map[string]string{"music_id": "555", "type": "1", "action": "1"} {
		if got := body.Get(k); got != want {
			t.Errorf("body %s=%q want %q", k, got, want)
		}
	}
	if got := miscUnitQuery(t, req.URL).Get("uid"); got != MD5Hex("123456789") {
		t.Errorf("uid=%q want md5(uid)=%q", got, MD5Hex("123456789"))
	}
	if origin := req.Header("origin"); origin != douyinBase {
		t.Errorf("origin=%q", origin)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	c2.SetUID(123456789)
	if _, err := c2.CollectMusic(t.Context(), "556", "0", "0"); err != nil {
		t.Fatalf("CollectMusic(cancel): %v", err)
	}
	body2 := miscUnitFormBody(t, miscUnitLastAt(t, st2, path))
	if body2.Get("type") != "0" || body2.Get("action") != "0" {
		t.Errorf("取消收藏 body=%v", body2)
	}

	c3, st3 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c3.CollectMusic(t.Context(), "", "1", "1"); err == nil {
		t.Fatal("空 music_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st3, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestMusicHotSearchVideosShape(t *testing.T) {
	const path = "/aweme/v1/web/hot/search/video/list/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "aweme_list": []any{}})
	if _, err := c.HotSearchVideos(t.Context(), "热词", "42", "", "", ""); err != nil {
		t.Fatalf("HotSearchVideos: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	want := map[string]string{
		"hotword": "热词", "sentence_id": "42",
		"offest": "0", // 线上参数名就是拼错的 offest
		"count":  "20", "entry_name": "pc_web",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("query %s=%q want %q", k, got, v)
		}
	}
	if _, ok := q["offset"]; ok {
		t.Error("不应发送正确的 offset 键，实录是 offest")
	}
	if _, ok := q["uifid"]; ok {
		t.Error("热搜词视频实录不带 uifid")
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.HotSearchVideos(t.Context(), "", "42", "0", "20", "pc_web"); err == nil {
		t.Fatal("空 hotword 必须报错")
	}
	if n := len(miscUnitReqsAt(st2, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
	c3, st3 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c3.HotSearchVideos(t.Context(), "热词", "", "0", "20", "pc_web"); err == nil {
		t.Fatal("空 sentence_id 必须报错")
	}
	if n := len(miscUnitReqsAt(st3, path)); n != 0 {
		t.Fatalf("校验失败后仍发出了 %d 个请求", n)
	}
}

func TestMusicCollectIDsExtraction(t *testing.T) {
	var ids []string
	collectMusicIDs(map[string]any{
		"aweme_list": []any{
			map[string]any{"music": map[string]any{"id_str": "7001"}},
			map[string]any{"music": map[string]any{"id": float64(7002)}},
		},
	}, &ids)
	if len(ids) != 2 || ids[0] != "7001" || ids[1] != "7002" {
		t.Fatalf("collectMusicIDs=%v want [7001 7002]", ids)
	}
}

// --- my-profile tab --------------------------------------------------------

func TestMiscProfileSelfShape(t *testing.T) {
	const path = "/aweme/v1/web/user/profile/self/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "user": map[string]any{"uid": "1"}})
	res, err := c.GetMyProfile(t.Context())
	if err != nil {
		t.Fatalf("GetMyProfile: %v", err)
	}
	if duMap(res["user"])["uid"] != "1" {
		t.Fatalf("user=%v", res["user"])
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{"device_platform": "webapp", "aid": "6383", "channel": "channel_pc_web"} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	if ref := req.Header("referer"); ref != profileSelfRefer {
		t.Errorf("referer=%q want %q", ref, profileSelfRefer)
	}
}

func TestMiscProfileDashboardShape(t *testing.T) {
	const path = "/aweme/v1/web/user/dashboard"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	c.SetUID(666)
	if _, err := c.GetUserDashboard(t.Context(), ""); err != nil {
		t.Fatalf("GetUserDashboard: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	q := miscUnitQuery(t, req.URL)
	if got := q.Get("UserId"); got != "666" {
		t.Errorf("UserId=%q want 666（UserID 空时默认自己）", got)
	}
	if q.Get("webcast_sdk_version") != "170400" || q.Get("webcast_version_code") != "170400" {
		t.Errorf("webcast 版本参数缺失: %v", q)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s", host)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	c2.SetUID(666)
	if _, err := c2.GetUserDashboard(t.Context(), "999"); err != nil {
		t.Fatalf("GetUserDashboard(explicit): %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("UserId"); got != "999" {
		t.Errorf("UserId=%q want 999", got)
	}
}

func TestMiscSocialCountShape(t *testing.T) {
	const path = "/aweme/v1/web/social/count"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "follower_count": 3})
	res, err := c.GetSocialCount(t.Context())
	if err != nil {
		t.Fatalf("GetSocialCount: %v", err)
	}
	if toInt64(res["follower_count"]) != 3 {
		t.Fatalf("res=%v", res)
	}
	req := miscUnitLastAt(t, st, path)
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s", host)
	}
	if ref := req.Header("referer"); ref != profileSelfRefer {
		t.Errorf("referer=%q", ref)
	}
}

func TestMiscUserSettingsSourceSwitch(t *testing.T) {
	t.Run("www-default", func(t *testing.T) {
		const path = "/aweme/v1/web/get/user/settings/"
		c, st := newStubJSON(t, map[string]any{"status_code": 0})
		if _, err := c.GetUserSettings(t.Context(), "", "", "", ""); err != nil {
			t.Fatalf("GetUserSettings(www): %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
			t.Errorf("host=%s want www.douyin.com", host)
		}
		q := miscUnitQuery(t, req.URL)
		for _, k := range []string{"is_fetch_frequency_control", "has_local_cache", "request_source"} {
			if _, ok := q[k]; ok {
				t.Errorf("可选参数 %s 默认不应发送", k)
			}
		}
	})

	t.Run("hj", func(t *testing.T) {
		const path = "/aweme/v1/web/user/settings/"
		c, st := newStubJSON(t, map[string]any{"status_code": 0})
		if _, err := c.GetUserSettings(t.Context(), "hj", "1", "true", "2"); err != nil {
			t.Fatalf("GetUserSettings(hj): %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
			t.Errorf("host=%s want www-hj.douyin.com", host)
		}
		q := miscUnitQuery(t, req.URL)
		for k, want := range map[string]string{
			"is_fetch_frequency_control": "1", "has_local_cache": "true", "request_source": "2",
		} {
			if got := q.Get(k); got != want {
				t.Errorf("query %s=%q want %q", k, got, want)
			}
		}
	})
}

func TestMiscCustomSettingsDefault(t *testing.T) {
	const path = "/aweme/v1/web/custom/settings/get/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.GetCustomSettings(t.Context(), ""); err != nil {
		t.Fatalf("GetCustomSettings: %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st, path).URL).Get("setting_types"); got != "1" {
		t.Errorf("setting_types=%q want 默认 1", got)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.GetCustomSettings(t.Context(), "2"); err != nil {
		t.Fatalf("GetCustomSettings(2): %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("setting_types"); got != "2" {
		t.Errorf("setting_types=%q want 2", got)
	}
}

func TestMiscCollectedAwemesShape(t *testing.T) {
	const path = "/aweme/v1/web/aweme/listcollection/"
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "aweme_list": []any{map[string]any{"aweme_id": "1"}}})
	res, err := c.GetCollectedAwemes(t.Context(), "", "")
	if err != nil {
		t.Fatalf("GetCollectedAwemes: %v", err)
	}
	if len(duSlice(res["aweme_list"])) != 1 {
		t.Fatalf("aweme_list=%v", res["aweme_list"])
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	if q := miscUnitQuery(t, req.URL); q.Get("x-secsdk-web-signature") == "" {
		t.Error("收藏作品接口的 query 必须带 x-secsdk-web-signature")
	}
	if ct := req.Header("content-type"); !strings.Contains(ct, "x-www-form-urlencoded") {
		t.Errorf("content-type=%q", ct)
	}
	body := miscUnitFormBody(t, req)
	if body.Get("count") != "10" || body.Get("cursor") != "0" {
		t.Errorf("body=%v want count=10 cursor=0", body)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.GetCollectedAwemes(t.Context(), "25", "10"); err != nil {
		t.Fatalf("GetCollectedAwemes(custom): %v", err)
	}
	body2 := miscUnitFormBody(t, miscUnitLastAt(t, st2, path))
	if body2.Get("count") != "25" || body2.Get("cursor") != "10" {
		t.Errorf("body=%v want count=25 cursor=10", body2)
	}
}

func TestMiscBaikeEndpointsShape(t *testing.T) {
	t.Run("worldbook", func(t *testing.T) {
		const path = "/webcast/ip/wiki/check_worldbook_access"
		c, st := newStubJSON(t, map[string]any{"status_code": 0, "has_access": true, "access": true})
		if _, err := c.BaikeCheckWorldbook(t.Context()); err != nil {
			t.Fatalf("BaikeCheckWorldbook: %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if host := miscUnitURL(t, req.URL).Host; host != "baike.douyin.com" {
			t.Errorf("host=%s want baike.douyin.com", host)
		}
		if _, ok := miscUnitQuery(t, req.URL)["a_bogus"]; !ok {
			t.Error("百科接口必须带 a_bogus")
		}
		if ref := req.Header("referer"); ref != baikeBase+"/" {
			t.Errorf("referer=%q", ref)
		}
	})

	t.Run("binding", func(t *testing.T) {
		const path = "/webcast/ip/wiki/get_account_binding_subject"
		c, st := newStubJSON(t, map[string]any{"status_code": 0})
		c.SetUID(321)
		if _, err := c.BaikeBindingSubject(t.Context(), ""); err != nil {
			t.Fatalf("BaikeBindingSubject: %v", err)
		}
		req := miscUnitLastAt(t, st, path)
		if host := miscUnitURL(t, req.URL).Host; host != "baike.douyin.com" {
			t.Errorf("host=%s", host)
		}
		if got := miscUnitQuery(t, req.URL).Get("account_uid"); got != "321" {
			t.Errorf("account_uid=%q want 321（默认自己）", got)
		}

		c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
		if _, err := c2.BaikeBindingSubject(t.Context(), "42"); err != nil {
			t.Fatalf("BaikeBindingSubject(explicit): %v", err)
		}
		if got := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL).Get("account_uid"); got != "42" {
			t.Errorf("account_uid=%q want 42", got)
		}
	})
}

// --- ecom ------------------------------------------------------------------

func TestEcomSKUListUsesHJHost(t *testing.T) {
	const path = "/aweme/v1/web/ecom/product/sku/list/"
	c, st := newStubClient(t, func(r stubRequest) (*Response, error) {
		if u, err := url.Parse(r.URL); err == nil && u.Path == path {
			return jsonResponse(map[string]any{
				"status_code": 0,
				"data":        map[string]any{"specs": []any{map[string]any{"name": "规格"}}},
			}), nil
		}
		return jsonResponse(map[string]any{}), nil
	})

	res, err := c.GetProductSKUList(t.Context(), "3753", "179")
	if err != nil {
		t.Fatalf("GetProductSKUList: %v", err)
	}
	if data := duMap(res["data"]); len(duSlice(data["specs"])) != 1 {
		t.Fatalf("data=%v", res["data"])
	}

	req := miscUnitLastAt(t, st, path)
	if req.Method != "POST" {
		t.Errorf("method=%s want POST", req.Method)
	}
	// SKU 接口只由 www-hj 集群提供，www 主站返回 403。
	if host := miscUnitURL(t, req.URL).Host; host != "www-hj.douyin.com" {
		t.Errorf("host=%s want www-hj.douyin.com", host)
	}
	if ct := req.Header("content-type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type=%q want application/json", ct)
	}
	q := miscUnitQuery(t, req.URL)
	if q.Get("device_platform") != "webapp" {
		t.Errorf("device_platform=%q", q.Get("device_platform"))
	}
	if _, ok := q["a_bogus"]; !ok {
		t.Error("SKU 接口必须带 a_bogus")
	}
	var body map[string]string
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("body 不是 JSON: %q (%v)", req.Body, err)
	}
	if body["product_id"] != "3753" || body["shop_id"] != "179" {
		t.Errorf("body=%v", body)
	}
}

func TestEcomSKUListRiskError(t *testing.T) {
	const path = "/aweme/v1/web/ecom/product/sku/list/"
	c, _ := newStubClient(t, func(r stubRequest) (*Response, error) {
		if u, err := url.Parse(r.URL); err == nil && u.Path == path {
			return statusResponse(403, ""), nil
		}
		return jsonResponse(map[string]any{}), nil
	})
	_, err := c.GetProductSKUList(t.Context(), "1", "2")
	if err == nil {
		t.Fatal("空 403 响应必须映射为风控错误")
	}
	if !strings.Contains(err.Error(), "空响应") {
		t.Fatalf("error=%v", err)
	}
}

// --- notice digg list ------------------------------------------------------

func TestNoticeDiggListQueryKey(t *testing.T) {
	const path = "/aweme/v1/web/notice/digg/list/"
	c, st := newStubJSON(t, map[string]any{
		"status_code": 0,
		"digg_list":   []any{map[string]any{"uid": "1"}},
	})
	res, err := c.GetNoticeDiggList(t.Context(), "nid-1", "", "", "")
	if err != nil {
		t.Fatalf("GetNoticeDiggList: %v", err)
	}
	if len(duSlice(res["digg_list"])) != 1 {
		t.Fatalf("digg_list=%v", res["digg_list"])
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	// 该接口只认 notice_id；nid_str / notice_id_str / cid / item_id 一律 5（参数不合法）。
	if got := q.Get("notice_id"); got != "nid-1" {
		t.Errorf("notice_id=%q want nid-1", got)
	}
	for _, banned := range []string{"nid_str", "notice_id_str", "cid", "item_id"} {
		if _, ok := q[banned]; ok {
			t.Errorf("不应发送 %s（服务端只认 notice_id）", banned)
		}
	}
	if q.Get("is_new_notice") != "1" || q.Get("is_mark_read") != "1" {
		t.Errorf("公共块缺失: %v", q)
	}
	if q.Get("count") != "20" {
		t.Errorf("count=%q want 默认 20", q.Get("count"))
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.GetNoticeDiggList(t.Context(), "nid-2", "5", "100", "50"); err != nil {
		t.Fatalf("GetNoticeDiggList(custom): %v", err)
	}
	q2 := miscUnitQuery(t, miscUnitLastAt(t, st2, path).URL)
	for k, want := range map[string]string{"notice_id": "nid-2", "count": "5", "max_time": "100", "min_time": "50"} {
		if got := q2.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
}

func TestNoticeDiggListNonJSONError(t *testing.T) {
	c, _ := newStubClient(t, func(r stubRequest) (*Response, error) {
		return statusResponse(200, "<html>blocked</html>"), nil
	})
	_, err := c.GetNoticeDiggList(t.Context(), "nid-1", "", "", "")
	if err == nil {
		t.Fatal("非 JSON 响应必须报错")
	}
	if !strings.Contains(err.Error(), "非 JSON") {
		t.Fatalf("error=%v", err)
	}
}

// --- relation --------------------------------------------------------------

func TestRelationFollowerListQueryShape(t *testing.T) {
	const path = "/aweme/v1/web/user/follower/list/"
	c, st := newStubJSON(t, map[string]any{
		"status_code": 0,
		"followers":   []any{map[string]any{"uid": "9"}},
		"has_more":    0,
	})
	res, err := c.GetUserFollowerList(t.Context(), "111", "SEC-xyz", "", "20")
	if err != nil {
		t.Fatalf("GetUserFollowerList: %v", err)
	}
	if len(duSlice(res["followers"])) != 1 {
		t.Fatalf("followers=%v", res["followers"])
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{
		"user_id": "111", "sec_user_id": "SEC-xyz", "offset": "0", "min_time": "0",
		"count": "20", "source_type": "1", "gps_access": "0", "address_book_access": "0",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	// 空 max_time 必须被替换成当前秒，而不是发 0（否则服务端走空推荐分支）。
	miscUnitEpochSeconds(t, q.Get("max_time"))
	if ref := req.Header("referer"); ref != douyinBase+"/user/SEC-xyz" {
		t.Errorf("referer=%q", ref)
	}
	if _, ok := q["a_bogus"]; !ok {
		t.Error("follower 列表必须带 a_bogus")
	}
}

func TestRelationFollowingListQueryShape(t *testing.T) {
	const path = "/aweme/v1/web/user/following/list/"
	c, st := newStubJSON(t, map[string]any{
		"status_code": 0,
		"followings":  []any{map[string]any{"uid": "9"}},
		"has_more":    0,
	})
	if _, err := c.GetUserFollowingList(t.Context(), "111", "SEC", "", "20"); err != nil {
		t.Fatalf("GetUserFollowingList: %v", err)
	}
	req := miscUnitLastAt(t, st, path)
	if req.Method != "GET" {
		t.Errorf("method=%s want GET", req.Method)
	}
	if host := miscUnitURL(t, req.URL).Host; host != "www.douyin.com" {
		t.Errorf("host=%s want www.douyin.com", host)
	}
	q := miscUnitQuery(t, req.URL)
	for k, want := range map[string]string{
		"user_id": "111", "sec_user_id": "SEC", "offset": "0", "min_time": "0",
		"count": "20", "source_type": "1", "is_top": "1",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s=%q want %q", k, got, want)
		}
	}
	miscUnitEpochSeconds(t, q.Get("max_time"))
	if q.Get("x-secsdk-web-signature") == "" {
		t.Error("following 列表必须带 x-secsdk-web-signature")
	}
}

func TestRelationMaxTimeExplicitPassThrough(t *testing.T) {
	const followerPath = "/aweme/v1/web/user/follower/list/"
	const followingPath = "/aweme/v1/web/user/following/list/"

	c, st := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c.GetUserFollowerList(t.Context(), "111", "SEC", "1700000000", "20"); err != nil {
		t.Fatalf("GetUserFollowerList: %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st, followerPath).URL).Get("max_time"); got != "1700000000" {
		t.Errorf("follower max_time=%q want 1700000000（显式值必须透传）", got)
	}

	c2, st2 := newStubJSON(t, map[string]any{"status_code": 0})
	if _, err := c2.GetUserFollowingList(t.Context(), "111", "SEC", "1700000001", "20"); err != nil {
		t.Fatalf("GetUserFollowingList: %v", err)
	}
	if got := miscUnitQuery(t, miscUnitLastAt(t, st2, followingPath).URL).Get("max_time"); got != "1700000001" {
		t.Errorf("following max_time=%q want 1700000001", got)
	}
}

func TestRelationFollowerPagination(t *testing.T) {
	const path = "/aweme/v1/web/user/follower/list/"
	var mu sync.Mutex
	pages := 0
	c, st := newStubClient(t, func(r stubRequest) (*Response, error) {
		u, err := url.Parse(r.URL)
		if err != nil || u.Path != path {
			return jsonResponse(map[string]any{}), nil
		}
		mu.Lock()
		pages++
		page := pages
		mu.Unlock()
		if page == 1 {
			return jsonResponse(map[string]any{
				"status_code": 0,
				"followers":   []any{map[string]any{"uid": "1"}},
				"has_more":    1,
				"min_time":    1600000000,
			}), nil
		}
		return jsonResponse(map[string]any{
			"status_code": 0,
			"followers":   []any{map[string]any{"uid": "2"}},
			"has_more":    0,
			"min_time":    1599999999,
		}), nil
	})

	out, err := c.GetSomeUserFollowerList(t.Context(), "111", "SEC", 2)
	if err != nil {
		t.Fatalf("GetSomeUserFollowerList: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("翻页结果=%d want 2", len(out))
	}
	reqs := miscUnitReqsAt(st, path)
	if len(reqs) != 2 {
		t.Fatalf("follower 分页请求=%d want 2", len(reqs))
	}
	miscUnitEpochSeconds(t, miscUnitQuery(t, reqs[0].URL).Get("max_time"))
	q2 := miscUnitQuery(t, reqs[1].URL)
	if got := q2.Get("max_time"); got != "1600000000" {
		t.Errorf("第二页 max_time=%q want 上一页 min_time=1600000000", got)
	}
	if got := q2.Get("source_type"); got != "1" {
		t.Errorf("第二页 source_type=%q want 1", got)
	}
}

func TestRelationNoticeListBackfill(t *testing.T) {
	const path = "/aweme/v1/web/notice/"

	t.Run("backfill", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{
			"status_code":    0,
			"notice_list":    []any{},
			"notice_list_v2": []any{map[string]any{"nid_str": "42"}},
		})
		res, err := c.GetNoticeList(t.Context(), "0", "0", "10", "3")
		if err != nil {
			t.Fatalf("GetNoticeList: %v", err)
		}
		if list := duSlice(res["notice_list"]); len(list) != 1 || duMap(list[0])["nid_str"] != "42" {
			t.Fatalf("notice_list 未被 v2 回填: %v", res["notice_list"])
		}
		q := miscUnitQuery(t, miscUnitLastAt(t, st, path).URL)
		for k, want := range map[string]string{
			"notice_group": "3", "is_new_notice": "1", "is_mark_read": "1", "count": "10",
		} {
			if got := q.Get(k); got != want {
				t.Errorf("query %s=%q want %q", k, got, want)
			}
		}
	})

	t.Run("keep-existing", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{
			"status_code":    0,
			"notice_list":    []any{map[string]any{"nid_str": "1"}},
			"notice_list_v2": []any{map[string]any{"nid_str": "2"}},
		})
		res, err := c.GetNoticeList(t.Context(), "0", "0", "10", "")
		if err != nil {
			t.Fatalf("GetNoticeList: %v", err)
		}
		if list := duSlice(res["notice_list"]); len(list) != 1 || duMap(list[0])["nid_str"] != "1" {
			t.Fatalf("非空 notice_list 不应被覆盖: %v", res["notice_list"])
		}
	})
}
