package douyin

// Hermetic coverage for the content/user/notice/profile-tab web APIs:
// api_work.go, api_collect.go, api_comment.go, api_interact.go, api_user.go,
// api_notice.go and api_profile_tabs.go.
//
// Every test runs against the shared stubTransport and never touches the
// network. The client's msToken is pinned with SetMsToken so MsToken() never
// kicks off the background mssdk exchange, keeping the recorded request list
// deterministic.

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

// --- shared local helpers --------------------------------------------------

// contentTestClient builds a stub-backed client with a pinned msToken.
func contentTestClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	c, st := newStubClient(t, handle)
	c.SetMsToken("stub-ms-token")
	return c, st
}

// contentOKClient answers every request with {"status_code":0}.
func contentOKClient(t *testing.T) (*Client, *stubTransport) {
	t.Helper()
	return contentTestClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0}), nil
	})
}

// contentRequest returns the first recorded request whose URL contains path.
func contentRequest(t *testing.T, st *stubTransport, path string) stubRequest {
	t.Helper()
	reqs := contentAllRequests(st, path)
	if len(reqs) == 0 {
		t.Fatalf("没有命中 %q 的请求: %v", path, contentURLs(st))
	}
	return reqs[0]
}

// contentAllRequests returns every recorded request whose URL contains path.
func contentAllRequests(st *stubTransport, path string) []stubRequest {
	var out []stubRequest
	for _, r := range st.requests() {
		if strings.Contains(r.URL, path) {
			out = append(out, r)
		}
	}
	return out
}

func contentURLs(st *stubTransport) []string {
	var out []string
	for _, r := range st.requests() {
		out = append(out, r.Method+" "+r.URL)
	}
	return out
}

// contentParam parses key out of a recorded URL's query (encoded or raw).
func contentParam(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// contentForm parses a recorded form body.
func contentForm(t *testing.T, body []byte) url.Values {
	t.Helper()
	v, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("解析表单 body 失败 %q: %v", body, err)
	}
	return v
}

// contentPublishClient builds a client that satisfies publishCredentialsOK
// without any network crypto material, using a freshly generated P-256 key.
func contentPublishClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	priv, _, err := GenerateECKeypair()
	if err != nil {
		t.Fatalf("GenerateECKeypair: %v", err)
	}
	st := &stubTransport{handle: handle}
	c, err := NewClient("sessionid=abc; UIFID=uif", Options{
		Transport:     st,
		Ticket:        "tk-1",
		TsSign:        "ts.1abc",
		PrivateKey:    priv,
		SessionDtrait: "dtrait-blob",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetMsToken("stub-ms-token")
	return c, st
}

// --- api_work.go: pure helpers ---------------------------------------------

func TestWorkPureHelpers(t *testing.T) {
	t.Run("duUserID", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"https://www.douyin.com/user/ABC", "ABC"},
			{"https://www.douyin.com/user/ABC?showTab=like", "ABC"},
			{"MS4wLjABAAAA", "MS4wLjABAAAA"},
			{"", ""},
		}
		for _, tc := range cases {
			if got := duUserID(tc.in); got != tc.want {
				t.Errorf("duUserID(%q) = %q want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("duUserURL", func(t *testing.T) {
		if got := duUserURL("SEC"); got != douyinBase+"/user/SEC" {
			t.Errorf("duUserURL(bare) = %q", got)
		}
		full := "https://www.douyin.com/user/SEC?x=1"
		if got := duUserURL(full); got != full {
			t.Errorf("duUserURL(url) = %q", got)
		}
	})

	t.Run("duStr", func(t *testing.T) {
		cases := []struct {
			in   any
			want string
		}{
			{"x", "x"},
			{nil, ""},
			{float64(42), "42"},
			{float64(1.5), "1.5"},
			{json.Number("7687099550015753000"), "7687099550015753000"},
			{int(7), "7"},
			{int64(9), "9"},
			{true, "true"},
			{false, "false"},
			{[]any{}, ""},
		}
		for _, tc := range cases {
			if got := duStr(tc.in); got != tc.want {
				t.Errorf("duStr(%v) = %q want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("duSlice", func(t *testing.T) {
		if got := duSlice([]any{1, 2}); len(got) != 2 {
			t.Errorf("duSlice([]any) = %v", got)
		}
		if duSlice("x") != nil || duSlice(nil) != nil {
			t.Error("duSlice(non-slice) 应为 nil")
		}
	})

	t.Run("duMap", func(t *testing.T) {
		if got := duMap(map[string]any{"a": 1}); got["a"] == nil {
			t.Errorf("duMap(map) = %v", got)
		}
		if duMap([]any{}) != nil || duMap(nil) != nil {
			t.Error("duMap(non-map) 应为 nil")
		}
	})

	t.Run("duHasMore", func(t *testing.T) {
		cases := []struct {
			in   map[string]any
			want bool
		}{
			{map[string]any{"has_more": json.Number("1")}, true},
			{map[string]any{"has_more": float64(1)}, true},
			{map[string]any{"has_more": json.Number("0")}, false},
			{map[string]any{}, false},
			{nil, false},
		}
		for _, tc := range cases {
			if got := duHasMore(tc.in); got != tc.want {
				t.Errorf("duHasMore(%v) = %v want %v", tc.in, got, tc.want)
			}
		}
	})

	t.Run("duQuote", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"abcXYZ0-_.~/", "abcXYZ0-_.~/"},
			{"a b", "a%20b"},
			{"a=b&c", "a%3Db%26c"},
			{"中", "%E4%B8%AD"},
		}
		for _, tc := range cases {
			if got := duQuote(tc.in); got != tc.want {
				t.Errorf("duQuote(%q) = %q want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("duUUID4", func(t *testing.T) {
		re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
		a, b := duUUID4(), duUUID4()
		if !re.MatchString(a) {
			t.Fatalf("duUUID4() = %q 不是合法 v4 UUID", a)
		}
		if a == b {
			t.Error("duUUID4 两次调用不应相同")
		}
	})

	t.Run("duCompactJSON", func(t *testing.T) {
		if got := duCompactJSON("raw"); got != "raw" {
			t.Errorf("duCompactJSON(string) = %q", got)
		}
		if got := duCompactJSON(map[string]any{"text": "中"}); got != `{"text":"中"}` {
			t.Errorf("duCompactJSON(map) = %q", got)
		}
		if got := duCompactJSON(map[string]any{"a": "<b>"}); got != `{"a":"<b>"}` {
			t.Errorf("duCompactJSON 不应转义 HTML: %q", got)
		}
		if got := duCompactJSON([]any{}); got != "[]" {
			t.Errorf("duCompactJSON([]) = %q", got)
		}
	})
}

// --- api_work.go: GetWorkInfo ----------------------------------------------

func TestWorkGetWorkInfo(t *testing.T) {
	c, st := contentTestClient(t, func(stubRequest) (*Response, error) {
		r := jsonResponse(map[string]any{
			"status_code":  0,
			"aweme_detail": map[string]any{"aweme_id": "123", "desc": "hi"},
		})
		r.Cookies = []*fhttp.Cookie{ck("absorbed", "yes")}
		return r, nil
	})

	res, err := c.GetWorkInfo(t.Context(), "123")
	if err != nil {
		t.Fatalf("GetWorkInfo: %v", err)
	}
	if toInt64(res["status_code"]) != 0 {
		t.Fatalf("status_code = %v", res["status_code"])
	}
	if got := duStr(duMap(res["aweme_detail"])["aweme_id"]); got != "123" {
		t.Fatalf("aweme_id = %q", got)
	}
	if c.Cookie.Get("absorbed") != "yes" {
		t.Error("响应 Set-Cookie 应被 absorbCookies 吸收")
	}

	req := contentRequest(t, st, "/aweme/v1/web/aweme/detail/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	if !strings.HasPrefix(req.URL, douyinBase+"/aweme/v1/web/aweme/detail/") {
		t.Errorf("host/path = %s", req.URL)
	}
	for _, want := range []string{
		"aweme_id=123", "request_source=600", "origin_type=video_page",
		"version_code=190500", "version_name=19.5.0",
		"webid=", "a_bogus=", "msToken=stub-ms-token",
	} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
	if req.Header("referer") != douyinBase+"/video/123" {
		t.Errorf("referer = %q", req.Header("referer"))
	}
}

func TestWorkGetWorkInfoBadURLIssuesNoRequest(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetWorkInfo(t.Context(), "not-a-url"); err == nil {
		t.Fatal("无法解析的链接应报错")
	}
	if got := len(st.requests()); got != 0 {
		t.Fatalf("解析失败不应发请求, 实际 %d 个", got)
	}
}

func TestWorkGetWorkInfoNonJSONBody(t *testing.T) {
	c, st := contentTestClient(t, func(stubRequest) (*Response, error) {
		return statusResponse(500, "<html>oops</html>"), nil
	})
	if _, err := c.GetWorkInfo(t.Context(), "123"); err == nil {
		t.Fatal("HTTP 5xx / 非 JSON 应报错")
	}
	if len(st.requests()) == 0 {
		t.Fatal("应至少发出一次请求")
	}
}

// --- api_collect.go --------------------------------------------------------

func TestCollectGetUserFavorite(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetUserFavorite(t.Context(), "SEC1", "10", "5"); err != nil {
		t.Fatalf("GetUserFavorite: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/aweme/favorite/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"sec_user_id=SEC1", "max_cursor=10", "min_cursor=0", "count=5", "cut_version=1", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
	if req.Header("referer") != douyinBase+"/user/SEC1?showTab=like" {
		t.Errorf("referer = %q", req.Header("referer"))
	}
}

func TestCollectGetCollectList(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetCollectList(t.Context()); err != nil {
		t.Fatalf("GetCollectList: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/collects/list/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"cursor=0", "count=20", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
}

func TestCollectCollectAweme(t *testing.T) {
	c, st := contentOKClient(t)
	res, err := c.CollectAweme(t.Context(), "42", "1")
	if err != nil {
		t.Fatalf("CollectAweme: %v", err)
	}
	if toInt64(res["status_code"]) != 0 {
		t.Fatalf("status_code = %v", res["status_code"])
	}
	req := contentRequest(t, st, "/aweme/v1/web/aweme/collect/")
	if req.Method != fhttp.MethodPost {
		t.Errorf("method = %s", req.Method)
	}
	if got := string(req.Body); got != "action=1&aweme_id=42&aweme_type=0" {
		t.Errorf("body = %q", got)
	}
	// uid 是 md5(登录数字 ID)，此处 uid 解析失败回落到 0。
	if !strings.Contains(req.URL, "uid="+MD5Hex("0")) {
		t.Errorf("URL 缺少 uid: %s", req.URL)
	}
	if req.Header("origin") != douyinBase {
		t.Errorf("origin = %q", req.Header("origin"))
	}
}

func TestCollectMoveAndRemoveAweme(t *testing.T) {
	t.Run("move", func(t *testing.T) {
		c, st := contentOKClient(t)
		if _, err := c.MoveCollectAweme(t.Context(), "42", "默认收藏夹", "cid-1"); err != nil {
			t.Fatalf("MoveCollectAweme: %v", err)
		}
		req := contentRequest(t, st, "/aweme/v1/web/collects/video/move/")
		if req.Method != fhttp.MethodPost {
			t.Errorf("method = %s", req.Method)
		}
		if len(req.Body) != 0 {
			t.Errorf("move 的 body 应为空, 实际 %q", req.Body)
		}
		for _, want := range []string{
			"item_ids=42", "move_collects_list=cid-1", "to_collects_id=cid-1",
			"update_collects_sort=true", "item_type=2", "a_bogus=",
		} {
			if !strings.Contains(req.URL, want) {
				t.Errorf("URL 缺少 %q: %s", want, req.URL)
			}
		}
		// URL 用 encodeURIComponent 风格编码，"默认收藏夹" 应被百分号编码。
		if !strings.Contains(req.URL, "collects_name=%E9%BB%98%E8%AE%A4%E6%94%B6%E8%97%8F%E5%A4%B9") {
			t.Errorf("collects_name 未被编码: %s", req.URL)
		}
	})

	t.Run("remove", func(t *testing.T) {
		c, st := contentOKClient(t)
		if _, err := c.RemoveCollectAweme(t.Context(), "42", "夹子", "cid-1"); err != nil {
			t.Fatalf("RemoveCollectAweme: %v", err)
		}
		req := contentRequest(t, st, "/aweme/v1/web/collects/video/move/")
		if req.Method != fhttp.MethodPost {
			t.Errorf("method = %s", req.Method)
		}
		if len(req.Body) != 0 {
			t.Errorf("remove 的 body 应为空, 实际 %q", req.Body)
		}
		for _, want := range []string{"item_ids=42", "from_collects_id=cid-1", "collects_name="} {
			if !strings.Contains(req.URL, want) {
				t.Errorf("URL 缺少 %q: %s", want, req.URL)
			}
		}
		if strings.Contains(req.URL, "to_collects_id=") {
			t.Errorf("remove 不应带 to_collects_id: %s", req.URL)
		}
	})
}

// --- api_comment.go --------------------------------------------------------

func TestCommentOutComment(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetWorkOutComment(t.Context(), "https://www.douyin.com/video/123", "7"); err != nil {
		t.Fatalf("GetWorkOutComment: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/comment/list/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{
		"aweme_id=123", "cursor=7", "count=5", "item_type=0",
		"whale_cut_token=", "rcFT=", "uifid=uif", "a_bogus=",
	} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
}

func TestCommentAllOutCommentPaginatesAndTruncates(t *testing.T) {
	handle := func(r stubRequest) (*Response, error) {
		if strings.Contains(r.URL, "/aweme/v1/web/comment/list/") {
			if contentParam(r.URL, "cursor") == "0" {
				return jsonResponse(map[string]any{
					"status_code": 0, "has_more": 1, "cursor": 9,
					"comments": []any{map[string]any{"cid": "c1"}, map[string]any{"cid": "c2"}},
				}), nil
			}
			return jsonResponse(map[string]any{
				"status_code": 0, "has_more": 0, "cursor": 9,
				"comments": []any{map[string]any{"cid": "c3"}},
			}), nil
		}
		return jsonResponse(map[string]any{"status_code": 0}), nil
	}

	c, st := contentTestClient(t, handle)
	all, err := c.GetWorkAllOutComment(t.Context(), "123", 0)
	if err != nil {
		t.Fatalf("GetWorkAllOutComment: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("分页应聚合 3 条, 实际 %d", len(all))
	}
	if got := len(contentAllRequests(st, "/aweme/v1/web/comment/list/")); got != 2 {
		t.Fatalf("应翻 2 页, 实际 %d 页", got)
	}

	c2, _ := contentTestClient(t, handle)
	truncated, err := c2.GetWorkAllOutComment(t.Context(), "123", 1)
	if err != nil {
		t.Fatalf("GetWorkAllOutComment(limit=1): %v", err)
	}
	if len(truncated) != 1 {
		t.Fatalf("limit=1 应截断为 1 条, 实际 %d", len(truncated))
	}
}

func TestCommentInnerComment(t *testing.T) {
	c, st := contentOKClient(t)
	comment := map[string]any{"aweme_id": "123", "cid": "c1"}
	if _, err := c.GetWorkInnerComment(t.Context(), comment, "3", "5"); err != nil {
		t.Fatalf("GetWorkInnerComment: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/comment/list/reply/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"item_id=123", "comment_id=c1", "cursor=3", "count=5", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
	// 这个端点严格校验 a_bogus，且不带 uifid。
	if strings.Contains(req.URL, "uifid=") {
		t.Errorf("reply 端点不应带 uifid: %s", req.URL)
	}
}

func TestCommentAllInnerCommentPaginates(t *testing.T) {
	handle := func(r stubRequest) (*Response, error) {
		if strings.Contains(r.URL, "/aweme/v1/web/comment/list/reply/") {
			if contentParam(r.URL, "cursor") == "0" {
				return jsonResponse(map[string]any{
					"status_code": 0, "has_more": 1, "cursor": 4,
					"comments": []any{map[string]any{"cid": "r1"}},
				}), nil
			}
			return jsonResponse(map[string]any{
				"status_code": 0, "has_more": 0, "cursor": 4,
				"comments": []any{map[string]any{"cid": "r2"}},
			}), nil
		}
		return jsonResponse(map[string]any{"status_code": 0}), nil
	}
	c, st := contentTestClient(t, handle)
	comment := map[string]any{"aweme_id": "123", "cid": "c1"}
	all, err := c.GetWorkAllInnerComment(t.Context(), comment, 0)
	if err != nil {
		t.Fatalf("GetWorkAllInnerComment: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("应聚合 2 条回复, 实际 %d", len(all))
	}
	if got := len(contentAllRequests(st, "/aweme/v1/web/comment/list/reply/")); got != 2 {
		t.Fatalf("应翻 2 页, 实际 %d", got)
	}
}

func TestCommentAllCommentReplies(t *testing.T) {
	handle := func(r stubRequest) (*Response, error) {
		switch {
		case strings.Contains(r.URL, "/aweme/v1/web/comment/list/reply/"):
			return jsonResponse(map[string]any{
				"status_code": 0, "has_more": 0,
				"comments": []any{map[string]any{"cid": "r1"}},
			}), nil
		case strings.Contains(r.URL, "/aweme/v1/web/comment/list/"):
			return jsonResponse(map[string]any{
				"status_code": 0, "has_more": 0,
				"comments": []any{map[string]any{"cid": "c1", "aweme_id": "123", "reply_comment_total": 1}},
			}), nil
		}
		return jsonResponse(map[string]any{"status_code": 0}), nil
	}

	// includeReplies=true 会为每个有回复的评论补 reply_comment。
	c, st := contentTestClient(t, handle)
	out, err := c.GetWorkAllComment(t.Context(), "123", 0, true)
	if err != nil {
		t.Fatalf("GetWorkAllComment: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("应返回 1 条评论, 实际 %d", len(out))
	}
	replies := duSlice(duMap(out[0])["reply_comment"])
	if len(replies) != 1 || duStr(duMap(replies[0])["cid"]) != "r1" {
		t.Fatalf("reply_comment = %v", replies)
	}
	if len(contentAllRequests(st, "/aweme/v1/web/comment/list/reply/")) != 1 {
		t.Error("应拉取一次回复列表")
	}

	// includeReplies=false 只补空数组，且不发回复请求。
	c2, st2 := contentTestClient(t, handle)
	out2, err := c2.GetWorkAllComment(t.Context(), "123", 0, false)
	if err != nil {
		t.Fatalf("GetWorkAllComment(no replies): %v", err)
	}
	if got := duSlice(duMap(out2[0])["reply_comment"]); got == nil || len(got) != 0 {
		t.Fatalf("reply_comment 应为空数组, 实际 %v", got)
	}
	if len(contentAllRequests(st2, "/aweme/v1/web/comment/list/reply/")) != 0 {
		t.Error("includeReplies=false 不应请求回复")
	}
}

func TestCommentFindComment(t *testing.T) {
	t.Run("by cid", func(t *testing.T) {
		handle := func(r stubRequest) (*Response, error) {
			if strings.Contains(r.URL, "/aweme/v1/web/comment/list/") {
				return jsonResponse(map[string]any{
					"status_code": 0, "has_more": 0,
					"comments": []any{map[string]any{"cid": "c1"}, map[string]any{"cid": "c2"}},
				}), nil
			}
			return jsonResponse(map[string]any{"status_code": 0}), nil
		}
		c, _ := contentTestClient(t, handle)
		got, err := c.FindComment(t.Context(), "https://www.douyin.com/video/123", "c2")
		if err != nil {
			t.Fatalf("FindComment: %v", err)
		}
		if duStr(got["cid"]) != "c2" {
			t.Fatalf("cid = %v", got["cid"])
		}
	})

	t.Run("by nickname", func(t *testing.T) {
		handle := func(r stubRequest) (*Response, error) {
			if strings.Contains(r.URL, "/aweme/v1/web/comment/list/") {
				return jsonResponse(map[string]any{
					"status_code": 0, "has_more": 0,
					"comments": []any{map[string]any{"cid": "c1", "user": map[string]any{"nickname": "小明"}}},
				}), nil
			}
			return jsonResponse(map[string]any{"status_code": 0}), nil
		}
		c, _ := contentTestClient(t, handle)
		got, err := c.FindComment(t.Context(), "https://www.douyin.com/video/123", "小明")
		if err != nil {
			t.Fatalf("FindComment(nickname): %v", err)
		}
		if duStr(got["cid"]) != "c1" {
			t.Fatalf("cid = %v", got["cid"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		c, _ := contentTestClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0, "has_more": 0, "comments": []any{}}), nil
		})
		if _, err := c.FindComment(t.Context(), "https://www.douyin.com/video/123", "missing"); err == nil {
			t.Fatal("找不到评论应报错")
		}
	})
}

// --- api_interact.go -------------------------------------------------------

func TestInteractDigg(t *testing.T) {
	c, st := contentOKClient(t)
	ok, err := c.Digg(t.Context(), "123", "1")
	if err != nil {
		t.Fatalf("Digg: %v", err)
	}
	if !ok {
		t.Fatal("status_code=0 时 Digg 应返回 true")
	}
	req := contentRequest(t, st, "/aweme/v1/web/commit/item/digg/")
	if req.Method != fhttp.MethodPost {
		t.Errorf("method = %s", req.Method)
	}
	if got := string(req.Body); got != "aweme_id=123&item_type=0&type=1" {
		t.Errorf("body = %q", got)
	}
	if !strings.Contains(req.URL, "uid="+MD5Hex("0")) {
		t.Errorf("URL 缺少 uid: %s", req.URL)
	}
	if req.Header("referer") != douyinBase+"/discover?modal_id=123" {
		t.Errorf("referer = %q", req.Header("referer"))
	}
}

func TestInteractDiggAcceptsURL(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.Digg(t.Context(), "https://www.douyin.com/video/555", "0"); err != nil {
		t.Fatalf("Digg(url): %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/commit/item/digg/")
	if got := contentForm(t, req.Body).Get("aweme_id"); got != "555" {
		t.Fatalf("应把链接解析成 555, 实际 %q", got)
	}
	if got := contentForm(t, req.Body).Get("type"); got != "0" {
		t.Fatalf("type = %q", got)
	}
}

func TestInteractDiggBusinessStatusReturnsFalse(t *testing.T) {
	c, _ := contentTestClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 5, "status_msg": "参数不合法"}), nil
	})
	ok, err := c.Digg(t.Context(), "123", "1")
	if err != nil {
		t.Fatalf("Digg 业务失败不应返回 transport 错误: %v", err)
	}
	if ok {
		t.Fatal("非 0 status_code 应返回 false")
	}
}

func TestInteractPublishCommentGuards(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		c, st := contentOKClient(t)
		if _, err := c.PublishComment(t.Context(), "123", "hi", ""); err == nil {
			t.Fatal("缺少 ticket/ts_sign 应报错")
		}
		if got := len(st.requests()); got != 0 {
			t.Fatalf("凭据校验失败不应发请求, 实际 %d", got)
		}
	})

	t.Run("no dtrait", func(t *testing.T) {
		st := &stubTransport{}
		c, err := NewClient("a=b", Options{Transport: st, Ticket: "tk", TsSign: "ts.1abc"})
		if err != nil {
			t.Fatal(err)
		}
		c.SetMsToken("stub-ms-token")
		if _, err := c.PublishComment(t.Context(), "123", "hi", ""); err == nil {
			t.Fatal("缺少 dtrait 素材应报错")
		}
		if got := len(st.requests()); got != 0 {
			t.Fatalf("dtrait 校验失败不应发请求, 实际 %d", got)
		}
	})
}

func TestInteractPublishCommentRequest(t *testing.T) {
	sent := map[string]any{"status_code": 0, "comment": map[string]any{"cid": "c9"}}
	c, st := contentPublishClient(t, func(stubRequest) (*Response, error) { return jsonResponse(sent), nil })

	res, err := c.PublishComment(t.Context(), "123", "你好", "456")
	if err != nil {
		t.Fatalf("PublishComment: %v", err)
	}
	if duStr(duMap(res["comment"])["cid"]) != "c9" {
		t.Fatalf("comment.cid = %v", res["comment"])
	}

	req := contentRequest(t, st, "/aweme/v1/web/comment/publish")
	if req.Method != fhttp.MethodPost {
		t.Errorf("method = %s", req.Method)
	}
	form := contentForm(t, req.Body)
	if form.Get("aweme_id") != "123" || form.Get("text") != "你好" {
		t.Errorf("body 关键字段错误: %v", form)
	}
	if form.Get("reply_id") != "456" {
		t.Errorf("reply_id = %q", form.Get("reply_id"))
	}
	if form.Get("one_level_comment_rank") != "-1" {
		t.Errorf("默认 OneLevelCommentRank 应为 -1, 实际 %q", form.Get("one_level_comment_rank"))
	}
	if form.Get("paste_edit_method") != "non_paste" {
		t.Errorf("paste_edit_method = %q", form.Get("paste_edit_method"))
	}
	if form.Get("text_extra") != "[]" {
		t.Errorf("text_extra = %q", form.Get("text_extra"))
	}
	if !strings.Contains(req.URL, "a_bogus=") {
		t.Errorf("URL 缺少 a_bogus: %s", req.URL)
	}
	if req.Header("bd-ticket-guard-client-data") == "" {
		t.Error("发布接口应带 bd-ticket-guard-client-data 头")
	}
	if req.Header("referer") != douyinBase+"/video/123" {
		t.Errorf("referer = %q", req.Header("referer"))
	}
}

func TestInteractPublishCommentWithOpts(t *testing.T) {
	c, st := contentPublishClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 0}), nil
	})
	opts := CommentOpts{
		ReplyToReplyID:       "rt-1",
		CommentSendCelltime:  5,
		CommentVideoCelltime: 6,
		OneLevelCommentRank:  2,
		PasteEditMethod:      "paste",
		TextExtra:            []any{map[string]any{"type": 1}},
	}
	if _, err := c.PublishCommentWithOpts(t.Context(), "123", "c", "", opts); err != nil {
		t.Fatalf("PublishCommentWithOpts: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/comment/publish")
	form := contentForm(t, req.Body)
	if _, ok := form["reply_id"]; ok {
		t.Errorf("replyID 为空时不应带 reply_id: %v", form)
	}
	want := map[string]string{
		"reply_to_reply_id":      "rt-1",
		"comment_send_celltime":  "5",
		"comment_video_celltime": "6",
		"one_level_comment_rank": "2",
		"paste_edit_method":      "paste",
		"text_extra":             `[{"type":1}]`,
	}
	for k, v := range want {
		if got := form.Get(k); got != v {
			t.Errorf("%s = %q want %q", k, got, v)
		}
	}
}

// --- api_user.go -----------------------------------------------------------

func TestUserGetUserInfo(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetUserInfo(t.Context(), "SEC9"); err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/user/profile/other/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"sec_user_id=SEC9", "personal_center_strategy=1", "profile_other_record_enable=1", "land_to=1", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
	if req.Header("referer") != douyinBase+"/user/SEC9" {
		t.Errorf("referer = %q", req.Header("referer"))
	}
}

func TestUserGetUserWorkInfo(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetUserWorkInfo(t.Context(), "SEC9", "0"); err != nil {
		t.Fatalf("GetUserWorkInfo: %v", err)
	}
	first := contentRequest(t, st, "/aweme/v1/web/aweme/post/")
	if first.Method != fhttp.MethodGet {
		t.Errorf("method = %s", first.Method)
	}
	for _, want := range []string{
		"sec_user_id=SEC9", "max_cursor=0", "count=18",
		"need_time_list=1", "version_code=290100", "version_name=29.1.0",
		"from_user_page=1", "a_bogus=",
	} {
		if !strings.Contains(first.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, first.URL)
		}
	}

	// 非首页 max_cursor != 0 时 need_time_list 回到 0。
	if _, err := c.GetUserWorkInfo(t.Context(), "SEC9", "50"); err != nil {
		t.Fatalf("GetUserWorkInfo(page2): %v", err)
	}
	var second stubRequest
	found := false
	for _, r := range contentAllRequests(st, "/aweme/v1/web/aweme/post/") {
		if strings.Contains(r.URL, "max_cursor=50") {
			second, found = r, true
		}
	}
	if !found {
		t.Fatal("未找到 max_cursor=50 的请求")
	}
	if !strings.Contains(second.URL, "need_time_list=0") {
		t.Errorf("翻页时 need_time_list 应为 0: %s", second.URL)
	}
}

func TestUserAllWorkInfoPaginates(t *testing.T) {
	handle := func(r stubRequest) (*Response, error) {
		if strings.Contains(r.URL, "/aweme/v1/web/aweme/post/") {
			if contentParam(r.URL, "max_cursor") == "0" {
				return jsonResponse(map[string]any{
					"status_code": 0, "has_more": 1, "max_cursor": 50,
					"aweme_list": []any{map[string]any{"aweme_id": "a1"}},
				}), nil
			}
			return jsonResponse(map[string]any{
				"status_code": 0, "has_more": 0, "max_cursor": 50,
				"aweme_list": []any{map[string]any{"aweme_id": "a2"}},
			}), nil
		}
		return jsonResponse(map[string]any{"status_code": 0}), nil
	}
	c, st := contentTestClient(t, handle)
	works, err := c.GetUserAllWorkInfo(t.Context(), "SEC9", 0)
	if err != nil {
		t.Fatalf("GetUserAllWorkInfo: %v", err)
	}
	if len(works) != 2 {
		t.Fatalf("应聚合 2 个作品, 实际 %d", len(works))
	}
	if got := len(contentAllRequests(st, "/aweme/v1/web/aweme/post/")); got != 2 {
		t.Fatalf("应翻 2 页, 实际 %d", got)
	}
}

// --- api_notice.go ---------------------------------------------------------

func TestNoticeCount(t *testing.T) {
	c, st := contentOKClient(t)
	res, err := c.NoticeCount(t.Context())
	if err != nil {
		t.Fatalf("NoticeCount: %v", err)
	}
	if toInt64(res["status_code"]) != 0 {
		t.Fatalf("status_code = %v", res["status_code"])
	}
	req := contentRequest(t, st, "/aweme/v1/web/notice/count/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"is_new_notice=1", "need_social_count=1", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}
}

func TestNoticeCountBusinessError(t *testing.T) {
	c, _ := contentTestClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"status_code": 5, "status_msg": "参数不合法"}), nil
	})
	_, err := c.NoticeCount(t.Context())
	if err == nil {
		t.Fatal("非 0 status_code 应报错")
	}
	if !strings.Contains(err.Error(), "5") || !strings.Contains(err.Error(), "参数不合法") {
		t.Fatalf("错误应携带 code 与 status_msg, 实际 %v", err)
	}
}

func TestNoticeCountNonJSON(t *testing.T) {
	c, _ := contentTestClient(t, func(stubRequest) (*Response, error) {
		return statusResponse(502, "bad gateway"), nil
	})
	if _, err := c.NoticeCount(t.Context()); err == nil {
		t.Fatal("HTTP 5xx / 非 JSON 应报错且不 panic")
	}
}

func TestNoticeDetail(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.NoticeDetail(t.Context(), "nid-1"); err != nil {
		t.Fatalf("NoticeDetail: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/notice/detail/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	if !strings.Contains(req.URL, "notice_id_str=nid-1") {
		t.Errorf("URL 缺少 notice_id_str: %s", req.URL)
	}
}

func TestNoticeDetailEmptyIDIssuesNoRequest(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.NoticeDetail(t.Context(), "   "); err == nil {
		t.Fatal("空白 notice_id_str 应报错")
	}
	if got := len(st.requests()); got != 0 {
		t.Fatalf("ID 校验失败不应发请求, 实际 %d", got)
	}
}

func TestNoticeDelete(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.NoticeDelete(t.Context(), "3", "nid-1"); err != nil {
		t.Fatalf("NoticeDelete: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/notice/del/")
	if req.Method != fhttp.MethodPost {
		t.Errorf("method = %s", req.Method)
	}
	if got := string(req.Body); got != "action_type=3&notice_id_str=nid-1" {
		t.Errorf("body = %q", got)
	}
	if req.Header("origin") != douyinBase {
		t.Errorf("origin = %q", req.Header("origin"))
	}
}

func TestNoticeDeleteEmptyIDIssuesNoRequest(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.NoticeDelete(t.Context(), "0", ""); err == nil {
		t.Fatal("空 notice_id_str 应报错")
	}
	if got := len(st.requests()); got != 0 {
		t.Fatalf("ID 校验失败不应发请求, 实际 %d", got)
	}
}

// --- api_profile_tabs.go ---------------------------------------------------

func TestProfileTabWatchHistory(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetWatchHistory(t.Context(), "", ""); err != nil {
		t.Fatalf("GetWatchHistory: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/history/read/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"cursor=0", "count=20", "version_code=170400", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}

	c2, st2 := contentOKClient(t)
	if _, err := c2.GetWatchHistory(t.Context(), "10", "5"); err != nil {
		t.Fatalf("GetWatchHistory(page): %v", err)
	}
	req2 := contentRequest(t, st2, "/aweme/v1/web/history/read/")
	if !strings.Contains(req2.URL, "cursor=10") || !strings.Contains(req2.URL, "count=5") {
		t.Errorf("分页参数未透传: %s", req2.URL)
	}
}

func TestProfileTabClearWatchHistory(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.ClearWatchHistory(t.Context()); err != nil {
		t.Fatalf("ClearWatchHistory: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/history/clear/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	if !strings.Contains(req.URL, "channel_pc_web") {
		t.Errorf("URL 缺少公共参数: %s", req.URL)
	}
}

func TestProfileTabWatchLater(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetWatchLater(t.Context(), ""); err != nil {
		t.Fatalf("GetWatchLater: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/watchlater/list/")
	if !strings.HasPrefix(req.URL, douyinHJBase) {
		t.Errorf("稍后再看应走 www-hj 域名, 实际 %s", req.URL)
	}
	for _, want := range []string{"offset=0", "list_type=0", "operate_type=0"} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}

	c2, st2 := contentOKClient(t)
	if _, err := c2.GetWatchLater(t.Context(), "30"); err != nil {
		t.Fatalf("GetWatchLater(offset): %v", err)
	}
	req2 := contentRequest(t, st2, "/aweme/v1/web/watchlater/list/")
	if !strings.Contains(req2.URL, "offset=30") {
		t.Errorf("offset 未透传: %s", req2.URL)
	}
}

func TestProfileTabAppointments(t *testing.T) {
	c, st := contentOKClient(t)
	if _, err := c.GetAppointments(t.Context(), "", 0); err != nil {
		t.Fatalf("GetAppointments: %v", err)
	}
	req := contentRequest(t, st, "/aweme/v1/web/user/appointment/list/")
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s", req.Method)
	}
	for _, want := range []string{"appointment_type=100", "count=-1", "version_code=320600", "a_bogus="} {
		if !strings.Contains(req.URL, want) {
			t.Errorf("URL 缺少 %q: %s", want, req.URL)
		}
	}

	c2, st2 := contentOKClient(t)
	if _, err := c2.GetAppointments(t.Context(), "200", 5); err != nil {
		t.Fatalf("GetAppointments(values): %v", err)
	}
	req2 := contentRequest(t, st2, "/aweme/v1/web/user/appointment/list/")
	if !strings.Contains(req2.URL, "appointment_type=200") || !strings.Contains(req2.URL, "count=5") {
		t.Errorf("参数未透传: %s", req2.URL)
	}
}
