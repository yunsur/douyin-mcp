package douyin

// Hermetic stub-transport tests for the search/feed surfaces:
// api_search.go, api_search_ext.go, api_feed.go, api_misc_feeds.go.
//
// None of these four files override the host to www-hj.douyin.com (the www-hj
// cluster is used by api_profile_tabs.go / api_misc_*.go), so every endpoint
// here must target https://www.douyin.com. The stub records the exact outgoing
// URL/headers/body, so the tests assert on those.

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	"google.golang.org/protobuf/encoding/protowire"
)

// --- helpers ---------------------------------------------------------------

// sfxStubClient builds a stub-backed client with deterministic session bits:
// a pinned msToken/webid means the request parameters are stable and no
// background device-id / mssdk fetch is launched.
func sfxStubClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	c, st := newStubClient(t, handle)
	c.SetMsToken("sfx-token")
	c.SetWebID("sfx-webid")
	return c, st
}

// sfxJSON builds a 200 JSON response with optional response headers.
func sfxJSON(t *testing.T, v any, hdr map[string]string) *Response {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal stub body: %v", err)
	}
	h := fhttp.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &Response{StatusCode: 200, Header: h, Body: body}
}

// sfxProtoResp builds a 200 response advertising a protobuf body.
func sfxProtoResp(body []byte) *Response {
	h := fhttp.Header{}
	h.Set("content-type", "application/x-protobuf")
	return &Response{StatusCode: 200, Header: h, Body: body}
}

// sfxStatus builds a response with an explicit status/body and headers.
func sfxStatus(status int, body string, hdr map[string]string) *Response {
	h := fhttp.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &Response{StatusCode: status, Header: h, Body: []byte(body)}
}

// sfxRequestsFor returns the recorded requests whose URL path contains api, in
// order.
func sfxRequestsFor(st *stubTransport, api string) []stubRequest {
	var out []stubRequest
	for _, r := range st.requests() {
		if strings.Contains(r.URL, api) {
			out = append(out, r)
		}
	}
	return out
}

// sfxRequestFor returns the last recorded request for api.
func sfxRequestFor(t *testing.T, st *stubTransport, api string) stubRequest {
	t.Helper()
	reqs := sfxRequestsFor(st, api)
	if len(reqs) == 0 {
		var urls []string
		for _, r := range st.requests() {
			urls = append(urls, r.URL)
		}
		t.Fatalf("no request for %q; recorded %v", api, urls)
	}
	return reqs[len(reqs)-1]
}

// sfxQuery parses a recorded request URL into its query values.
func sfxQuery(t *testing.T, req stubRequest) url.Values {
	t.Helper()
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("parse %q: %v", req.URL, err)
	}
	return u.Query()
}

// sfxHostPath returns the recorded request's host and path.
func sfxHostPath(t *testing.T, req stubRequest) (string, string) {
	t.Helper()
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("parse %q: %v", req.URL, err)
	}
	return u.Host, u.Path
}

// sfxWantHostPath asserts host www.douyin.com and the exact path.
func sfxWantHostPath(t *testing.T, req stubRequest, path string) {
	t.Helper()
	host, gotPath := sfxHostPath(t, req)
	if host != "www.douyin.com" {
		t.Errorf("host = %q, want www.douyin.com", host)
	}
	if gotPath != path {
		t.Errorf("path = %q, want %q", gotPath, path)
	}
}

// sfxWant asserts one query parameter value.
func sfxWant(t *testing.T, q url.Values, key, want string) {
	t.Helper()
	if got := q.Get(key); got != want {
		t.Errorf("query %s = %q, want %q", key, got, want)
	}
}

// sfxWantAbsent asserts a query parameter is not sent.
func sfxWantAbsent(t *testing.T, q url.Values, key string) {
	t.Helper()
	if _, ok := q[key]; ok {
		t.Errorf("query %s should be absent, got %q", key, q.Get(key))
	}
}

// sfxWantNonEmpty asserts a query parameter is present and non-empty.
func sfxWantNonEmpty(t *testing.T, q url.Values, key string) {
	t.Helper()
	if q.Get(key) == "" {
		t.Errorf("query %s should be present and non-empty", key)
	}
}

// sfxOrder asserts first appears before second in the raw query string, which
// pins the documented a_bogus-before-verifyFp ordering.
func sfxOrder(t *testing.T, req stubRequest, first, second string) {
	t.Helper()
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("parse %q: %v", req.URL, err)
	}
	i, j := strings.Index(u.RawQuery, first), strings.Index(u.RawQuery, second)
	if i < 0 || j < 0 {
		t.Fatalf("query %q lacks %q or %q", u.RawQuery, first, second)
	}
	if i > j {
		t.Errorf("%q should precede %q in %q", first, second, u.RawQuery)
	}
}

func sfxItem(id string) map[string]any { return map[string]any{"id": id} }

// sfxWork is one general-search item: id plus the aweme_info marker the helper
// filters on.
func sfxWork(id string) map[string]any {
	return map[string]any{"id": id, "aweme_info": map[string]any{"aweme_id": id}}
}

func sfxIDs(items []any) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, duStr(duMap(it)["id"]))
	}
	return out
}

func sfxProtoVarint(num protowire.Number, v uint64) []byte {
	b := protowire.AppendTag(nil, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func sfxProtoBytes(num protowire.Number, v []byte) []byte {
	b := protowire.AppendTag(nil, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

// --- search video ----------------------------------------------------------

func TestSearchVideoWorkQueryAndHost(t *testing.T) {
	t.Run("filters and search_id cursor", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return sfxJSON(t, map[string]any{"status_code": 0, "data": []any{}},
				map[string]string{"X-Tt-Logid": "log-42"}), nil
		})
		res, err := c.SearchVideoWorkWithSearchID(t.Context(), "风景", "20", "25", "2", "7", "1-1", "0", "prev-log")
		if err != nil {
			t.Fatalf("SearchVideoWorkWithSearchID: %v", err)
		}
		if toInt64(res["status_code"]) != 0 {
			t.Errorf("status_code = %v, want 0", res["status_code"])
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/search/item/")
		if req.Method != "GET" {
			t.Errorf("method = %q, want GET", req.Method)
		}
		sfxWantHostPath(t, req, "/aweme/v1/web/search/item/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "keyword", "风景")
		sfxWant(t, q, "offset", "20")
		sfxWant(t, q, "count", "25")
		sfxWant(t, q, "sort_type", "2")
		sfxWant(t, q, "publish_time", "7")
		sfxWant(t, q, "filter_duration", "1-1")
		sfxWant(t, q, "search_range", "0")
		sfxWant(t, q, "search_id", "prev-log")
		sfxWant(t, q, "search_channel", "aweme_video_web")
		sfxWant(t, q, "channel", "channel_pc_web")
		sfxWant(t, q, "need_filter_settings", "0")
		sfxWant(t, q, "msToken", "sfx-token")
		sfxWant(t, q, "webid", "sfx-webid")
		sfxWantNonEmpty(t, q, "a_bogus")
		if q.Get("verifyFp") == "" || q.Get("verifyFp") != q.Get("fp") {
			t.Errorf("verifyFp/fp = %q/%q, want equal non-empty", q.Get("verifyFp"), q.Get("fp"))
		}
		if !strings.Contains(req.Header("referer"), "type=video") {
			t.Errorf("referer = %q, want type=video", req.Header("referer"))
		}
	})

	t.Run("omits search_id and flags first page", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.SearchVideoWork(t.Context(), "q", "0", "25", "0", "0", "", ""); err != nil {
			t.Fatalf("SearchVideoWork: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/search/item/")
		q := sfxQuery(t, req)
		sfxWantAbsent(t, q, "search_id")
		sfxWant(t, q, "offset", "0")
		sfxWant(t, q, "need_filter_settings", "1")
		sfxOrder(t, req, "a_bogus", "verifyFp")
	})
}

// --- search user -----------------------------------------------------------

func TestSearchUserQueryAndHost(t *testing.T) {
	t.Run("with fan/type filter", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0, "user_list": []any{}}), nil
		})
		res, err := c.SearchUser(t.Context(), "李白", "0", "20", "100-1000", "1")
		if err != nil {
			t.Fatalf("SearchUser: %v", err)
		}
		if toInt64(res["status_code"]) != 0 {
			t.Errorf("status_code = %v, want 0", res["status_code"])
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/discover/search/")
		sfxWantHostPath(t, req, "/aweme/v1/web/discover/search/")
		if got := req.Header("uifid"); got != "uif" {
			t.Errorf("uifid header = %q, want uif", got)
		}
		q := sfxQuery(t, req)
		sfxWant(t, q, "keyword", "李白")
		sfxWant(t, q, "search_channel", "aweme_user_web")
		sfxWant(t, q, "offset", "0")
		sfxWant(t, q, "count", "20")
		sfxWant(t, q, "is_filter_search", "1")
		sfxWant(t, q, "need_filter_settings", "1")
		sfxWant(t, q, "search_filter_value", `{"douyin_user_fans":["100-1000"],"douyin_user_type":["1"]}`)
		sfxWant(t, q, "uifid", "uif")
		sfxWant(t, q, "round_trip_time", "50")
		if !strings.Contains(req.Header("referer"), "type=user") {
			t.Errorf("referer = %q, want type=user", req.Header("referer"))
		}
	})

	t.Run("no filter drops search_filter_value", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.SearchUser(t.Context(), "q", "25", "20", "", ""); err != nil {
			t.Fatalf("SearchUser: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/discover/search/")
		q := sfxQuery(t, req)
		sfxWantAbsent(t, q, "search_filter_value")
		sfxWant(t, q, "is_filter_search", "0")
		sfxWant(t, q, "need_filter_settings", "0")
		sfxWant(t, q, "offset", "25")
	})
}

// --- search live -----------------------------------------------------------

func TestSearchLiveQueryAndHost(t *testing.T) {
	c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0, "data": []any{}}), nil
	})
	if _, err := c.SearchLive(t.Context(), "直播", "0", "15"); err != nil {
		t.Fatalf("SearchLive: %v", err)
	}
	req := sfxRequestFor(t, st, "/aweme/v1/web/live/search/")
	sfxWantHostPath(t, req, "/aweme/v1/web/live/search/")
	q := sfxQuery(t, req)
	sfxWant(t, q, "keyword", "直播")
	sfxWant(t, q, "search_channel", "aweme_live")
	sfxWant(t, q, "offset", "0")
	sfxWant(t, q, "count", "15")
	sfxWant(t, q, "search_source", "normal_search")
	sfxWant(t, q, "need_filter_settings", "1")
	sfxWantNonEmpty(t, q, "a_bogus")
	if !strings.Contains(req.Header("referer"), "type=live") {
		t.Errorf("referer = %q, want type=live", req.Header("referer"))
	}
	sfxOrder(t, req, "a_bogus", "verifyFp")
}

// --- search general --------------------------------------------------------

func TestSearchGeneralFilterToggles(t *testing.T) {
	t.Run("no filter and non-first page", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.SearchGeneralWork(t.Context(), "q", "0", "0", "15", "", "", ""); err != nil {
			t.Fatalf("SearchGeneralWork: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/general/search/single/")
		sfxWantHostPath(t, req, "/aweme/v1/web/general/search/single/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "search_channel", "aweme_general")
		sfxWant(t, q, "offset", "15")
		sfxWant(t, q, "count", "15")
		sfxWant(t, q, "is_filter_search", "0")
		sfxWant(t, q, "need_filter_settings", "0")
		sfxWant(t, q, "version_code", "190600")
		if !strings.Contains(req.Header("referer"), "type=general") {
			t.Errorf("referer = %q, want type=general", req.Header("referer"))
		}
	})

	t.Run("active filter on first page", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.SearchGeneralWork(t.Context(), "q", "1", "5", "0", "", "", ""); err != nil {
			t.Fatalf("SearchGeneralWork: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/general/search/single/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "is_filter_search", "1")
		sfxWant(t, q, "need_filter_settings", "1")
		sfxWant(t, q, "offset", "0")
	})
}

// --- search suggest / hot board / challenges -------------------------------

func TestSearchSuggestRequest(t *testing.T) {
	c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0, "sug_list": []any{}}), nil
	})
	if _, err := c.SearchSuggest(t.Context(), "榴莲"); err != nil {
		t.Fatalf("SearchSuggest: %v", err)
	}
	req := sfxRequestFor(t, st, "/aweme/v1/web/search/sug/")
	sfxWantHostPath(t, req, "/aweme/v1/web/search/sug/")
	q := sfxQuery(t, req)
	sfxWant(t, q, "keyword", "榴莲")
	sfxWant(t, q, "count", "10")
	sfxWant(t, q, "source", "normal_search")
	sfxWant(t, q, "version_code", "170400")
	sfxWant(t, q, "version_name", "17.4.0")
	if got, want := req.Header("referer"), "https://www.douyin.com/search/%E6%A6%B4%E8%8E%B2"; got != want {
		t.Errorf("referer = %q, want %q", got, want)
	}
	// api_search_ext.go adds verifyFp/fp before a_bogus (order differs from the
	// api_search.go endpoints); only the presence is asserted here.
	sfxWantNonEmpty(t, q, "verifyFp")
	sfxWantNonEmpty(t, q, "a_bogus")
}

func TestHotSearchBoardRequest(t *testing.T) {
	c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0, "data": map[string]any{"word_list": []any{}}}), nil
	})
	if _, err := c.HotSearchBoard(t.Context()); err != nil {
		t.Fatalf("HotSearchBoard: %v", err)
	}
	req := sfxRequestFor(t, st, "/aweme/v1/web/hot/search/list/")
	sfxWantHostPath(t, req, "/aweme/v1/web/hot/search/list/")
	q := sfxQuery(t, req)
	sfxWant(t, q, "detail_list", "1")
	sfxWant(t, q, "source", "6")
	sfxWant(t, q, "main_billboard_count", "5")
	if got, want := req.Header("referer"), "https://www.douyin.com/search/"; got != want {
		t.Errorf("referer = %q, want %q", got, want)
	}
}

func TestSearchChallengesRequest(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0, "challenge_list": []any{}}), nil
		})
		if _, err := c.SearchChallenges(t.Context(), "榴莲", "", ""); err != nil {
			t.Fatalf("SearchChallenges: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v1/web/challenge/search/")
		sfxWantHostPath(t, req, "/aweme/v1/web/challenge/search/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "keyword", "榴莲")
		sfxWant(t, q, "cursor", "0")
		sfxWant(t, q, "count", "10")
		sfxWant(t, q, "search_source", "normal_search")
		if got, want := req.Header("referer"), "https://www.douyin.com/search/%E6%A6%B4%E8%8E%B2?type=challenge"; got != want {
			t.Errorf("referer = %q, want %q", got, want)
		}
	})

	t.Run("explicit cursor and count", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.SearchChallenges(t.Context(), "topic", "7", "3"); err != nil {
			t.Fatalf("SearchChallenges: %v", err)
		}
		q := sfxQuery(t, sfxRequestFor(t, st, "/aweme/v1/web/challenge/search/"))
		sfxWant(t, q, "cursor", "7")
		sfxWant(t, q, "count", "3")
		sfxWant(t, q, "keyword", "topic")
	})
}

// --- risk / decode errors --------------------------------------------------

func TestSearchRiskAndNonJSONErrors(t *testing.T) {
	bdturing := base64.StdEncoding.EncodeToString([]byte(`{"subtype":"verify"}`))
	cases := []struct {
		name    string
		resp    *Response
		call    func(c *Client) error
		wantSub string
	}{
		{
			name:    "non-JSON html body",
			resp:    sfxStatus(200, "<html>blocked</html>", nil),
			call:    func(c *Client) error { _, err := c.SearchLive(t.Context(), "q", "0", "15"); return err },
			wantSub: "非 JSON",
		},
		{
			name: "empty body",
			resp: sfxStatus(200, "", nil),
			call: func(c *Client) error {
				_, err := c.SearchVideoWork(t.Context(), "q", "0", "25", "0", "0", "", "")
				return err
			},
			wantSub: "空响应",
		},
		{
			name: "bdturing header",
			resp: sfxStatus(200, "<html>", map[string]string{"X-Vc-Bdturing-Parameters": bdturing}),
			call: func(c *Client) error {
				_, err := c.SearchGeneralWork(t.Context(), "q", "0", "0", "0", "", "", "")
				return err
			},
			wantSub: "bdturing",
		},
		{
			name: "passport verify header",
			resp: sfxStatus(403, "<html>", map[string]string{
				"X-Tt-Verify-Passport-Decision": `{"event_params":{"verify_scene":"verify"}}`,
			}),
			call:    func(c *Client) error { _, err := c.SearchSuggest(t.Context(), "q"); return err },
			wantSub: "二次身份验证",
		},
		{
			name:    "non-JSON on challenges",
			resp:    sfxStatus(200, "not json at all", nil),
			call:    func(c *Client) error { _, err := c.SearchChallenges(t.Context(), "q", "", ""); return err },
			wantSub: "非 JSON",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := sfxStubClient(t, func(stubRequest) (*Response, error) { return tc.resp, nil })
			err := tc.call(c)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestSearchDecodeKeepsBigIDs(t *testing.T) {
	c, _ := sfxStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{
			"status_code": json.Number("0"),
			"max_cursor":  int64(7123456789012345678),
		}), nil
	})
	res, err := c.SearchLive(t.Context(), "q", "0", "15")
	if err != nil {
		t.Fatalf("SearchLive: %v", err)
	}
	got, ok := res["max_cursor"].(json.Number)
	if !ok {
		t.Fatalf("max_cursor type = %T, want json.Number", res["max_cursor"])
	}
	if got.String() != "7123456789012345678" {
		t.Errorf("max_cursor = %s, want 7123456789012345678", got)
	}
}

// --- auto-pagination -------------------------------------------------------

func TestSearchSomeLivePagination(t *testing.T) {
	t.Run("accumulates in order and stops on has_more 0", func(t *testing.T) {
		c, st := sfxStubClient(t, func(req stubRequest) (*Response, error) {
			switch sfxQuery(t, req).Get("offset") {
			case "0":
				return jsonResponse(map[string]any{"has_more": 1, "data": []any{sfxItem("a"), sfxItem("b")}}), nil
			case "15":
				return jsonResponse(map[string]any{"has_more": 0, "data": []any{sfxItem("c")}}), nil
			}
			return sfxStatus(500, "unexpected offset", nil), nil
		})
		out, err := c.SearchSomeLive(t.Context(), "q", 10)
		if err != nil {
			t.Fatalf("SearchSomeLive: %v", err)
		}
		if want := []string{"a", "b", "c"}; !reflect.DeepEqual(sfxIDs(out), want) {
			t.Errorf("ids = %v, want %v", sfxIDs(out), want)
		}
		reqs := sfxRequestsFor(st, "/aweme/v1/web/live/search/")
		if len(reqs) != 2 {
			t.Fatalf("issued %d requests, want 2", len(reqs))
		}
		if got, want := []string{sfxQuery(t, reqs[0]).Get("offset"), sfxQuery(t, reqs[1]).Get("offset")}, []string{"0", "15"}; !reflect.DeepEqual(got, want) {
			t.Errorf("offsets = %v, want %v", got, want)
		}
		for i, r := range reqs {
			sfxWant(t, sfxQuery(t, r), "count", "15")
			if i == 1 {
				sfxWant(t, sfxQuery(t, r), "offset", "15")
			}
		}
	})

	t.Run("respects the requested limit", func(t *testing.T) {
		c, st := sfxStubClient(t, func(req stubRequest) (*Response, error) {
			switch sfxQuery(t, req).Get("offset") {
			case "0":
				return jsonResponse(map[string]any{"has_more": 1, "data": []any{sfxItem("a"), sfxItem("b")}}), nil
			case "15":
				return jsonResponse(map[string]any{"has_more": 1, "data": []any{sfxItem("c"), sfxItem("d"), sfxItem("e")}}), nil
			}
			return sfxStatus(500, "unexpected offset", nil), nil
		})
		out, err := c.SearchSomeLive(t.Context(), "q", 3)
		if err != nil {
			t.Fatalf("SearchSomeLive: %v", err)
		}
		if want := []string{"a", "b", "c"}; !reflect.DeepEqual(sfxIDs(out), want) {
			t.Errorf("ids = %v, want %v (truncated to limit)", sfxIDs(out), want)
		}
		if got := len(sfxRequestsFor(st, "/aweme/v1/web/live/search/")); got != 2 {
			t.Errorf("issued %d requests, want 2", got)
		}
	})

	t.Run("absent has_more stops after one page", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"data": []any{sfxItem("a"), sfxItem("b")}}), nil
		})
		out, err := c.SearchSomeLive(t.Context(), "q", 10)
		if err != nil {
			t.Fatalf("SearchSomeLive: %v", err)
		}
		if want := []string{"a", "b"}; !reflect.DeepEqual(sfxIDs(out), want) {
			t.Errorf("ids = %v, want %v", sfxIDs(out), want)
		}
		if got := len(sfxRequestsFor(st, "/aweme/v1/web/live/search/")); got != 1 {
			t.Errorf("issued %d requests, want 1", got)
		}
	})
}

func TestSearchSomeUserPagination(t *testing.T) {
	c, st := sfxStubClient(t, func(req stubRequest) (*Response, error) {
		switch sfxQuery(t, req).Get("offset") {
		case "0":
			return jsonResponse(map[string]any{"has_more": 1, "user_list": []any{sfxItem("u1"), sfxItem("u2")}}), nil
		case "25":
			return jsonResponse(map[string]any{"has_more": 0, "user_list": []any{sfxItem("u3")}}), nil
		}
		return sfxStatus(500, "unexpected offset", nil), nil
	})
	out, err := c.SearchSomeUser(t.Context(), "q", 10)
	if err != nil {
		t.Fatalf("SearchSomeUser: %v", err)
	}
	if want := []string{"u1", "u2", "u3"}; !reflect.DeepEqual(sfxIDs(out), want) {
		t.Errorf("ids = %v, want %v", sfxIDs(out), want)
	}
	reqs := sfxRequestsFor(st, "/aweme/v1/web/discover/search/")
	if len(reqs) != 2 {
		t.Fatalf("issued %d requests, want 2", len(reqs))
	}
	if got, want := []string{sfxQuery(t, reqs[0]).Get("offset"), sfxQuery(t, reqs[1]).Get("offset")}, []string{"0", "25"}; !reflect.DeepEqual(got, want) {
		t.Errorf("offsets = %v, want %v", got, want)
	}
	for _, r := range reqs {
		sfxWant(t, sfxQuery(t, r), "count", "25")
	}
}

func TestSearchSomeVideoPagination(t *testing.T) {
	c, st := sfxStubClient(t, func(req stubRequest) (*Response, error) {
		q := sfxQuery(t, req)
		switch {
		case q.Get("offset") == "0" && q.Get("search_id") == "":
			return sfxJSON(t, map[string]any{"has_more": 1, "data": []any{sfxItem("v1")}},
				map[string]string{"X-Tt-Logid": "log-1"}), nil
		case q.Get("offset") == "25" && q.Get("search_id") == "log-1":
			return sfxJSON(t, map[string]any{"has_more": 0, "data": []any{sfxItem("v2")}},
				map[string]string{"X-Tt-Logid": "log-2"}), nil
		}
		return sfxStatus(500, "unexpected page", nil), nil
	})
	out, err := c.SearchSomeVideoWork(t.Context(), "q", 10, "0", "0", "", "")
	if err != nil {
		t.Fatalf("SearchSomeVideoWork: %v", err)
	}
	if want := []string{"v1", "v2"}; !reflect.DeepEqual(sfxIDs(out), want) {
		t.Errorf("ids = %v, want %v", sfxIDs(out), want)
	}
	reqs := sfxRequestsFor(st, "/aweme/v1/web/search/item/")
	if len(reqs) != 2 {
		t.Fatalf("issued %d requests, want 2", len(reqs))
	}
	q0, q1 := sfxQuery(t, reqs[0]), sfxQuery(t, reqs[1])
	if q0.Get("offset") != "0" || q1.Get("offset") != "25" {
		t.Errorf("offsets = %q/%q, want 0/25", q0.Get("offset"), q1.Get("offset"))
	}
	sfxWantAbsent(t, q0, "search_id")
	if got := q1.Get("search_id"); got != "log-1" {
		t.Errorf("second page search_id = %q, want log-1", got)
	}
	for _, r := range reqs {
		sfxWant(t, sfxQuery(t, r), "count", "25")
	}
}

func TestSearchSomeGeneralPagination(t *testing.T) {
	t.Run("filters items and advances by raw page length", func(t *testing.T) {
		c, st := sfxStubClient(t, func(req stubRequest) (*Response, error) {
			switch sfxQuery(t, req).Get("offset") {
			case "0":
				return jsonResponse(map[string]any{"has_more": 1, "data": []any{
					sfxWork("w1"), map[string]any{"id": "skip"}, sfxWork("w2"),
				}}), nil
			case "3":
				return jsonResponse(map[string]any{"has_more": 0, "data": []any{sfxWork("w3")}}), nil
			}
			return sfxStatus(500, "unexpected offset", nil), nil
		})
		out, err := c.SearchSomeGeneralWork(t.Context(), "q", 10, "0", "0", "", "", "")
		if err != nil {
			t.Fatalf("SearchSomeGeneralWork: %v", err)
		}
		if want := []string{"w1", "w2", "w3"}; !reflect.DeepEqual(sfxIDs(out), want) {
			t.Errorf("ids = %v, want %v (items without aweme_info dropped)", sfxIDs(out), want)
		}
		reqs := sfxRequestsFor(st, "/aweme/v1/web/general/search/single/")
		if len(reqs) != 2 {
			t.Fatalf("issued %d requests, want 2", len(reqs))
		}
		if got, want := []string{sfxQuery(t, reqs[0]).Get("offset"), sfxQuery(t, reqs[1]).Get("offset")}, []string{"0", "3"}; !reflect.DeepEqual(got, want) {
			t.Errorf("offsets = %v, want %v", got, want)
		}
		for _, r := range reqs {
			sfxWant(t, sfxQuery(t, r), "count", "15")
		}
	})

	t.Run("stops at the limit after one page", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"has_more": 1, "data": []any{sfxWork("w1"), sfxWork("w2")}}), nil
		})
		out, err := c.SearchSomeGeneralWork(t.Context(), "q", 2, "0", "0", "", "", "")
		if err != nil {
			t.Fatalf("SearchSomeGeneralWork: %v", err)
		}
		if want := []string{"w1", "w2"}; !reflect.DeepEqual(sfxIDs(out), want) {
			t.Errorf("ids = %v, want %v", sfxIDs(out), want)
		}
		if got := len(sfxRequestsFor(st, "/aweme/v1/web/general/search/single/")); got != 1 {
			t.Errorf("issued %d requests, want 1", got)
		}
	})
}

// --- feeds ----------------------------------------------------------------

func TestFeedGetFeedRequest(t *testing.T) {
	c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0, "aweme_list": []any{}}), nil
	})
	if _, err := c.GetFeed(t.Context(), "20", "3"); err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	req := sfxRequestFor(t, st, "/aweme/v1/web/module/feed/")
	if req.Method != "GET" {
		t.Errorf("method = %q, want GET", req.Method)
	}
	sfxWantHostPath(t, req, "/aweme/v1/web/module/feed/")
	q := sfxQuery(t, req)
	sfxWant(t, q, "module_id", "3003101")
	sfxWant(t, q, "count", "20")
	sfxWant(t, q, "refresh_index", "3")
	sfxWant(t, q, "refer_type", "10")
	sfxWant(t, q, "device_platform", "webapp")
	sfxWant(t, q, "aid", "6383")
	sfxWant(t, q, "channel", "channel_pc_web")
	sfxWant(t, q, "version_code", "170400")
	sfxWantNonEmpty(t, q, "screen_width")
	sfxWantNonEmpty(t, q, "browser_name")
	if got, want := req.Header("referer"), "https://www.douyin.com/"; got != want {
		t.Errorf("referer = %q, want %q", got, want)
	}
}

func TestFeedChannelModuleFeedRequest(t *testing.T) {
	t.Run("defaults with protobuf body", func(t *testing.T) {
		protoBody := append(sfxProtoVarint(1, 0), sfxProtoBytes(18, []byte("vid"))...)
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return sfxProtoResp(protoBody), nil
		})
		res, err := c.GetChannelModuleFeed(t.Context(), "", "", "", "2", "", "", "", "")
		if err != nil {
			t.Fatalf("GetChannelModuleFeed: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v2/web/module/feed/")
		if req.Method != "POST" {
			t.Errorf("method = %q, want POST", req.Method)
		}
		sfxWantHostPath(t, req, "/aweme/v2/web/module/feed/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "module_id", "3003101")
		sfxWant(t, q, "count", "20")
		sfxWant(t, q, "refresh_index", "1")
		sfxWant(t, q, "use_lite_type", "2")
		if got, want := string(req.Body), "encoded_pre_item_ids=&encoded_pre_room_ids="; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
		if res["content_type"] != "application/x-protobuf" {
			t.Errorf("content_type = %v, want application/x-protobuf", res["content_type"])
		}
	})

	t.Run("explicit pre-item params", func(t *testing.T) {
		c, st := sfxStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.GetChannelModuleFeed(t.Context(), "77", "5", "2", "2", "1,2", "log", "ENC", "ROOM"); err != nil {
			t.Fatalf("GetChannelModuleFeed: %v", err)
		}
		req := sfxRequestFor(t, st, "/aweme/v2/web/module/feed/")
		q := sfxQuery(t, req)
		sfxWant(t, q, "module_id", "77")
		sfxWant(t, q, "count", "5")
		sfxWant(t, q, "refresh_index", "2")
		sfxWant(t, q, "use_lite_type", "2")
		sfxWant(t, q, "pre_item_ids", "1,2")
		sfxWant(t, q, "pre_log_id", "log")
		if got, want := string(req.Body), "encoded_pre_item_ids=ENC&encoded_pre_room_ids=ROOM"; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
	})
}

// --- pure helpers ----------------------------------------------------------

func TestFeedProtoDecodeHelpers(t *testing.T) {
	t.Run("duProtoMessageToMap", func(t *testing.T) {
		nested := sfxProtoVarint(1, 7)
		msg := sfxProtoVarint(1, 0)
		msg = append(msg, sfxProtoBytes(2, []byte("hello"))...)
		msg = append(msg, sfxProtoBytes(3, nested)...)
		msg = append(msg, sfxProtoBytes(2, []byte("world"))...)
		msg = append(msg, sfxProtoBytes(4, []byte{0xff, 0xfe, 0x00})...)

		m := duProtoMessageToMap(msg, 4)
		if m == nil {
			t.Fatal("duProtoMessageToMap returned nil")
		}
		if v, ok := m["field_1"].(uint64); !ok || v != 0 {
			t.Errorf("field_1 = %#v, want uint64(0)", m["field_1"])
		}
		rep, ok := m["field_2"].([]any)
		if !ok || len(rep) != 2 || rep[0] != "hello" || rep[1] != "world" {
			t.Errorf("field_2 = %#v, want [hello world]", m["field_2"])
		}
		nm, ok := m["field_3"].(map[string]any)
		if !ok {
			t.Fatalf("field_3 = %#v, want nested map", m["field_3"])
		}
		if v, ok := nm["field_1"].(uint64); !ok || v != 7 {
			t.Errorf("field_3.field_1 = %#v, want uint64(7)", nm["field_1"])
		}
		if want := base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x00}); m["field_4"] != want {
			t.Errorf("field_4 = %v, want base64 %q", m["field_4"], want)
		}
	})

	t.Run("depth gates recursion", func(t *testing.T) {
		nested := sfxProtoVarint(1, 7)
		raw := sfxProtoBytes(3, nested)
		m := duProtoMessageToMap(raw, 0)
		if m == nil {
			t.Fatal("duProtoMessageToMap returned nil")
		}
		if _, isMap := m["field_3"].(map[string]any); isMap {
			t.Errorf("field_3 decoded as nested map at depth 0")
		}
		if got, ok := m["field_3"].(string); !ok || got != string(nested) {
			t.Errorf("field_3 = %#v, want %q", m["field_3"], string(nested))
		}
	})

	t.Run("nil on empty or invalid wire", func(t *testing.T) {
		if got := duProtoMessageToMap(nil, 4); got != nil {
			t.Errorf("empty input = %#v, want nil", got)
		}
		if got := duProtoMessageToMap([]byte{}, 4); got != nil {
			t.Errorf("zero input = %#v, want nil", got)
		}
		if got := duProtoMessageToMap([]byte{0xff}, 4); got != nil {
			t.Errorf("invalid wire = %#v, want nil", got)
		}
	})

	t.Run("duDecodeProtoOrJSON", func(t *testing.T) {
		// field_2 carries bytes that are neither a valid nested message nor
		// valid UTF-8, so it decodes to base64.
		raw := []byte{0xff}
		protoBody := sfxProtoVarint(1, 0)
		protoBody = append(protoBody, sfxProtoBytes(2, raw)...)
		out, err := duDecodeProtoOrJSON(sfxProtoResp(protoBody))
		if err != nil {
			t.Fatalf("protobuf body: %v", err)
		}
		if out["content_type"] != "application/x-protobuf" {
			t.Errorf("content_type = %v, want application/x-protobuf", out["content_type"])
		}
		if v, ok := out["status_code"].(uint64); !ok || v != 0 {
			t.Errorf("status_code = %#v, want uint64(0) from field_1", out["status_code"])
		}
		if want := base64.StdEncoding.EncodeToString(raw); out["field_2"] != want {
			t.Errorf("field_2 = %#v, want %q", out["field_2"], want)
		}

		js, err := duDecodeProtoOrJSON(sfxJSON(t, map[string]any{"status_code": 0},
			map[string]string{"content-type": "application/json"}))
		if err != nil {
			t.Fatalf("json body: %v", err)
		}
		if _, ok := js["status_code"].(json.Number); !ok {
			t.Errorf("json status_code type = %T, want json.Number", js["status_code"])
		}

		if _, err := duDecodeProtoOrJSON(sfxProtoResp([]byte{0xff})); err == nil {
			t.Error("invalid protobuf with protobuf content-type should error")
		}
		if _, err := duDecodeProtoOrJSON(sfxProtoResp(nil)); err == nil {
			t.Error("empty protobuf body should error")
		}
	})
}
