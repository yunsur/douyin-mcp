package douyin

// 无网络单元测试：PC IM 的请求/响应用层。
//
// 覆盖信封构造（per-cmd body 包装字段、字段号、int64 精确）、UUID/hex 契约、
// 各端点请求形状（会话列表/历史/成员/已读/设置/退出/分享 + 管理类 cmd）、
// 错误映射与入参校验，以及 wire 解码助手。
//
// 全部通过 stub transport 驱动，断言记录下来的原始 URL/头/body 字节。

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"testing"
)

// --- helpers ---------------------------------------------------------------

// imUnitResponse 构造一个 Response 信封：字段 3=code、4=msg，字段 6 为可选的
// cmd 子 body。字段 6 始终存在（哪怕为空），这样 code=0 时 imResponseBody 也认。
func imUnitResponse(code int64, msg string, respField int, inner *pbw) *Response {
	resp := &pbw{}
	resp.IntAlways(3, code)
	resp.Str(4, msg)
	if inner != nil {
		body := &pbw{}
		body.Msg(respField, inner)
		if len(body.b) > 0 {
			resp.Msg(6, body)
			return &Response{StatusCode: 200, Body: resp.b}
		}
	}
	resp.tag(6, 2)
	resp.varint(0)
	return &Response{StatusCode: 200, Body: resp.b}
}

// imUnitEnvelope 解析记录下来的原始信封字节。
func imUnitEnvelope(t *testing.T, raw []byte) map[int]any {
	t.Helper()
	env, err := pbParse(raw)
	if err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	return env
}

// imUnitAssertPost 断言一次 imCall 发出的请求形状，返回被包装的 cmd body。
func imUnitAssertPost(t *testing.T, req stubRequest, path string, cmd int64, bodyField int) map[int]any {
	t.Helper()
	if req.Method != "POST" {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if want := imBase + path; req.URL != want {
		t.Errorf("url = %q, want %q", req.URL, want)
	}
	if ct := req.Header("content-type"); ct != "application/x-protobuf" {
		t.Errorf("content-type = %q, want application/x-protobuf", ct)
	}
	if ref := req.Header("referer"); ref != "https://www.douyin.com/" {
		t.Errorf("referer = %q", ref)
	}
	env := imUnitEnvelope(t, req.Body)
	if got := pbInt(env, 1); got != cmd {
		t.Errorf("cmd = %d, want %d", got, cmd)
	}
	body, ok := pbMsg(env, 8)
	if !ok {
		t.Fatalf("envelope has no body(8): %v", env)
	}
	inner, ok := pbMsg(body, bodyField)
	if !ok {
		t.Fatalf("cmd body not wrapped in field %d (body fields: %v)", bodyField, body)
	}
	return inner
}

// imUnitAssertSendBody 断言 /v1/message/send 请求的 cmd/包装/头（URL 带 query，
// 不做整串比较），返回 send_message_body。
func imUnitAssertSendBody(t *testing.T, req stubRequest) map[int]any {
	t.Helper()
	if req.Method != "POST" {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if ct := req.Header("content-type"); ct != "application/x-protobuf" {
		t.Errorf("content-type = %q, want application/x-protobuf", ct)
	}
	env := imUnitEnvelope(t, req.Body)
	if got := pbInt(env, 1); got != 100 {
		t.Errorf("cmd = %d, want 100", got)
	}
	body, ok := pbMsg(env, 8)
	if !ok {
		t.Fatalf("envelope has no body(8): %v", env)
	}
	inner, ok := pbMsg(body, 100)
	if !ok {
		t.Fatalf("send_message_body (field 100) missing: %v", body)
	}
	return inner
}

func imUnitFindRequest(t *testing.T, reqs []stubRequest, pred func(stubRequest) bool) stubRequest {
	t.Helper()
	for _, r := range reqs {
		if pred(r) {
			return r
		}
	}
	t.Fatalf("no matching request among %d recorded", len(reqs))
	return stubRequest{}
}

// --- envelope + id exactness ----------------------------------------------

// 管理类端点的请求形状：method/host/path + cmd + body 包装字段，且 int64
// 参数在原始 wire 字节里保持精确（>2^53 不丢精度）。
func TestIMUnitAdminEndpoints(t *testing.T) {
	ctx := t.Context()
	name := "群名"
	notice := "公告"
	opts := IMCoreInfoOptions{Name: &name, Notice: &notice}
	const bigID = int64(9007199254740993) // 2^53+1：float64 无法精确表示

	cases := []struct {
		name      string
		path      string
		cmd       int64
		bodyField int
		call      func(context.Context, *Client) error
		check     func(*testing.T, map[int]any)
	}{
		{
			name: "rename", path: "/v1/conversation/set_core_info", cmd: 902, bodyField: 902,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMSetConversationCoreInfo(ctx, "cid", 42, 2, opts)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if got := pbString(inner, 1); got != "cid" {
					t.Errorf("field1 = %q", got)
				}
				if got := pbString(inner, 4); got != name {
					t.Errorf("field4 = %q, want %q", got, name)
				}
				if got := pbString(inner, 7); got != notice {
					t.Errorf("field7 = %q, want %q", got, notice)
				}
				if pbInt(inner, 8) != 1 || pbInt(inner, 11) != 1 {
					t.Errorf("is_*_set flags = %d/%d, want 1/1", pbInt(inner, 8), pbInt(inner, 11))
				}
			},
		},
		{
			name: "kick", path: "/v1/conversation/remove_participants", cmd: 651, bodyField: 651,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMRemoveParticipants(ctx, "cid", 42, 2, []int64{bigID, 7})
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				list := pbList(inner, 4)
				if len(list) != 2 {
					t.Fatalf("participants = %v, want 2", list)
				}
				if got := toInt64(list[0]); got != bigID {
					t.Errorf("participants[0] = %d, want %d (precision lost?)", got, bigID)
				}
				if got := toInt64(list[1]); got != 7 {
					t.Errorf("participants[1] = %d, want 7", got)
				}
			},
		},
		{
			name: "recall", path: "/v1/message/recall", cmd: 702, bodyField: 702,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMRecallMessage(ctx, "cid", 42, 2, bigID)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if got := pbInt(inner, 4); got != bigID {
					t.Errorf("server_message_id = %d, want %d", got, bigID)
				}
			},
		},
		{
			name: "delete", path: "/v1/message/delete", cmd: 701, bodyField: 701,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMDeleteMessage(ctx, "cid", 42, 2, 1234)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if got := pbInt(inner, 4); got != 1234 {
					t.Errorf("message_id = %d, want 1234", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := newStubClient(t, func(stubRequest) (*Response, error) {
				return imUnitResponse(0, "", int(tc.bodyField), &pbw{}), nil
			})
			if err := tc.call(ctx, c); err != nil {
				t.Fatalf("call: %v", err)
			}
			reqs := st.requests()
			if len(reqs) != 1 {
				t.Fatalf("requests = %d, want 1", len(reqs))
			}
			inner := imUnitAssertPost(t, reqs[0], tc.path, tc.cmd, int(tc.bodyField))
			tc.check(t, inner)
		})
	}
}

// --- read / mutation endpoints --------------------------------------------

func TestIMUnitReadEndpoints(t *testing.T) {
	ctx := t.Context()
	pin := true

	convCore := func() *pbw {
		w := &pbw{}
		w.Str(1, "cid")
		w.IntAlways(2, 42)
		w.IntAlways(3, 2)
		return w
	}
	// 响应把 core 包在 body 的字段 1 里（imCall 返回的 cmd body）。
	respCore := func() *pbw {
		w := &pbw{}
		w.Msg(1, convCore())
		return w
	}

	cases := []struct {
		name      string
		path      string
		cmd       int64
		bodyField int
		respField int
		inner     *pbw
		call      func(context.Context, *Client) error
		check     func(*testing.T, map[int]any)
	}{
		{
			name: "init_list", path: "/v1/message/get_message_by_init", cmd: 2043, bodyField: 2043, respField: 2043,
			call: func(ctx context.Context, c *Client) error { _, err := c.IMListConversations(ctx); return err },
			check: func(t *testing.T, inner map[int]any) {
				if got := pbInt(inner, 2); got != 0 {
					t.Errorf("field2 = %d, want 0", got)
				}
			},
		},
		{
			name: "conversation_info", path: "/v2/conversation/get_info_list", cmd: 610, bodyField: 610, respField: 610,
			inner: respCore(),
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMConversationInfo(ctx, "cid", 42, 2)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				data, ok := pbMsg(inner, 1)
				if !ok {
					t.Fatalf("body field1 (data) missing: %v", inner)
				}
				if pbString(data, 1) != "cid" || pbInt(data, 2) != 42 || pbInt(data, 3) != 2 {
					t.Errorf("data = %v", data)
				}
			},
		},
		{
			name: "history", path: "/v1/message/get_by_conversation", cmd: 301, bodyField: 301, respField: 301,
			call: func(ctx context.Context, c *Client) error {
				_, _, err := c.IMConversationHistory(ctx, "cid", 42, 2, 0, 5)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if pbString(inner, 1) != "cid" || pbInt(inner, 2) != 2 || pbInt(inner, 3) != 42 {
					t.Errorf("conversation ref = %v", inner)
				}
				if pbInt(inner, 4) != 1 || pbInt(inner, 5) != 0 || pbInt(inner, 6) != 5 {
					t.Errorf("paging = %d/%d/%d, want 1/0/5", pbInt(inner, 4), pbInt(inner, 5), pbInt(inner, 6))
				}
			},
		},
		{
			name: "participants", path: "/v1/conversation/participants_list", cmd: 605, bodyField: 605, respField: 605,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMParticipants(ctx, "cid", 42, 2, 0, 10)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if pbString(inner, 1) != "cid" || pbInt(inner, 2) != 42 || pbInt(inner, 3) != 2 {
					t.Errorf("conversation ref = %v", inner)
				}
				if pbInt(inner, 4) != 0 || pbInt(inner, 5) != 10 {
					t.Errorf("paging = %d/%d, want 0/10", pbInt(inner, 4), pbInt(inner, 5))
				}
			},
		},
		{
			name: "set_setting", path: "/v1/conversation/set_setting_info", cmd: 921, bodyField: 921, respField: 921,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMSetConversationSetting(ctx, "cid", 42, 2, &pin, nil)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if pbString(inner, 1) != "cid" || pbInt(inner, 2) != 42 || pbInt(inner, 3) != 2 {
					t.Errorf("conversation ref = %v", inner)
				}
				if got := pbInt(inner, 4); got != 1 {
					t.Errorf("pin field4 = %d, want 1", got)
				}
				if _, ok := inner[5]; ok {
					t.Errorf("mute nil should not write field5: %v", inner)
				}
			},
		},
		{
			name: "leave", path: "/v1/conversation/leave", cmd: 652, bodyField: 652, respField: 652,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMLeaveConversation(ctx, "cid", 42, 2)
				return err
			},
			check: func(t *testing.T, inner map[int]any) {
				if pbString(inner, 1) != "cid" || pbInt(inner, 2) != 42 || pbInt(inner, 3) != 2 {
					t.Errorf("conversation ref = %v", inner)
				}
			},
		},
		{
			name: "stranger", path: "/v1/stranger/get_conversation_list", cmd: 1001, bodyField: 1000, respField: 1000,
			call: func(ctx context.Context, c *Client) error { _, err := c.IMStrangerConversations(ctx); return err },
			check: func(t *testing.T, inner map[int]any) {
				if pbInt(inner, 1) != 0 || pbInt(inner, 2) != 1 || pbInt(inner, 3) != 1 {
					t.Errorf("stranger body = %v, want 1/2/3 = 0/1/1", inner)
				}
			},
		},
		{
			name: "mark_read", path: "/v3/conversation/mark_read", cmd: 2002, bodyField: 604, respField: 604,
			call: func(ctx context.Context, c *Client) error { _, err := c.IMMarkRead(ctx, "cid", 42, 2, 7); return err },
			check: func(t *testing.T, inner map[int]any) {
				if pbString(inner, 1) != "cid" || pbInt(inner, 2) != 42 || pbInt(inner, 3) != 2 {
					t.Errorf("conversation ref = %v", inner)
				}
				if pbInt(inner, 4) != 7 || pbInt(inner, 6) != 140 {
					t.Errorf("index/limit = %d/%d, want 7/140", pbInt(inner, 4), pbInt(inner, 6))
				}
				mark, ok := pbMsg(inner, 11)
				if !ok {
					t.Fatalf("mark_read sub-message (field11) missing: %v", inner)
				}
				if pbInt(mark, 1) != 50 || pbInt(mark, 2) != 0 {
					t.Errorf("mark = %v, want {1:50,2:0}", mark)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := newStubClient(t, func(stubRequest) (*Response, error) {
				return imUnitResponse(0, "", tc.respField, tc.inner), nil
			})
			if err := tc.call(ctx, c); err != nil {
				t.Fatalf("call: %v", err)
			}
			reqs := st.requests()
			if len(reqs) != 1 {
				t.Fatalf("requests = %d, want 1", len(reqs))
			}
			inner := imUnitAssertPost(t, reqs[0], tc.path, tc.cmd, tc.bodyField)
			tc.check(t, inner)
		})
	}
}

// 分页 count 边界：0 或超上限都夹到默认值。
func TestIMUnitPaginationClamp(t *testing.T) {
	ctx := t.Context()
	cases := []struct {
		name      string
		call      func(context.Context, *Client) error
		path      string
		cmd       int64
		bodyField int
		field     int
		want      int64
	}{
		{
			name: "history_zero",
			call: func(ctx context.Context, c *Client) error {
				_, _, err := c.IMConversationHistory(ctx, "cid", 1, 2, 0, 0)
				return err
			},
			path: "/v1/message/get_by_conversation", cmd: 301, bodyField: 301, field: 6, want: 50,
		},
		{
			name: "history_over",
			call: func(ctx context.Context, c *Client) error {
				_, _, err := c.IMConversationHistory(ctx, "cid", 1, 2, 0, 500)
				return err
			},
			path: "/v1/message/get_by_conversation", cmd: 301, bodyField: 301, field: 6, want: 50,
		},
		{
			name: "participants_zero",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMParticipants(ctx, "cid", 1, 2, 0, 0)
				return err
			},
			path: "/v1/conversation/participants_list", cmd: 605, bodyField: 605, field: 5, want: 50,
		},
		{
			name: "participants_over",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.IMParticipants(ctx, "cid", 1, 2, 0, 999)
				return err
			},
			path: "/v1/conversation/participants_list", cmd: 605, bodyField: 605, field: 5, want: 50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := newStubClient(t, func(stubRequest) (*Response, error) {
				return imUnitResponse(0, "", int(tc.bodyField), &pbw{}), nil
			})
			if err := tc.call(ctx, c); err != nil {
				t.Fatalf("call: %v", err)
			}
			inner := imUnitAssertPost(t, st.last(t), tc.path, tc.cmd, tc.bodyField)
			if got := pbInt(inner, tc.field); got != tc.want {
				t.Errorf("field%d = %d, want %d", tc.field, got, tc.want)
			}
		})
	}
}

// --- send / share path ------------------------------------------------------

// SendMsg 风格的分享卡走 /v1/message/send：断言 host/path、cmd=100、body 包装在
// 字段 100、message_type、会话 id（含 64 位精确）与卡片内容。
func TestIMUnitSendShareRequest(t *testing.T) {
	ctx := t.Context()
	const bigShortID = int64(9007199254740993)

	cases := []struct {
		name      string
		wantType  int64
		shortID   int64
		call      func(context.Context, *Client) (map[string]any, error)
		checkCard func(*testing.T, map[string]any)
	}{
		{
			name: "share_aweme", wantType: imShareAweme, shortID: bigShortID,
			call: func(ctx context.Context, c *Client) (map[string]any, error) {
				return c.ShareAweme(ctx, "0:1:5:6", bigShortID, "ticket-x", "1234567890123456789")
			},
			checkCard: func(t *testing.T, card map[string]any) {
				if got := imStr(card["itemId"]); got != "1234567890123456789" {
					t.Errorf("card itemId = %q", got)
				}
				if got := toInt64(card["aweType"]); got != 800 {
					t.Errorf("card aweType = %d, want 800", got)
				}
			},
		},
		{
			name: "share_photos", wantType: imSharePhotos, shortID: 42,
			call: func(ctx context.Context, c *Client) (map[string]any, error) {
				return c.SharePhotos(ctx, "0:1:5:6", 42, "ticket-x", "555")
			},
			checkCard: func(t *testing.T, card map[string]any) {
				if got := imStr(card["itemId"]); got != "555" {
					t.Errorf("card itemId = %q", got)
				}
				if got := toInt64(card["awemeType"]); got != 68 {
					t.Errorf("card awemeType = %d, want 68", got)
				}
			},
		},
		{
			name: "share_web", wantType: imShareWeb, shortID: 42,
			call: func(ctx context.Context, c *Client) (map[string]any, error) {
				return c.ShareWeb(ctx, "0:1:5:6", 42, "ticket-x", "https://example.com/a")
			},
			checkCard: func(t *testing.T, card map[string]any) {
				if got := imStr(card["link_url"]); !strings.Contains(got, "example.com") {
					t.Errorf("card link_url = %q", got)
				}
			},
		},
		{
			name: "user_card", wantType: imShareUser, shortID: 42,
			call: func(ctx context.Context, c *Client) (map[string]any, error) {
				return c.SendUserCard(ctx, "0:1:5:6", 42, "ticket-x", "MS4wLjABAAAA")
			},
			checkCard: func(t *testing.T, card map[string]any) {
				if got := imStr(card["secUID"]); got != "MS4wLjABAAAA" {
					t.Errorf("card secUID = %q", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := newStubClient(t, func(r stubRequest) (*Response, error) {
				if strings.Contains(r.URL, imIdentityTokenPath) {
					return jsonResponse(map[string]any{
						"data": map[string]any{
							"identity_security_token": "tok-secret",
							"device_id":               "dev-1",
						},
					}), nil
				}
				// 发送响应：空的 protobuf Response 信封。
				return &Response{StatusCode: 200, Body: nil}, nil
			})
			c.SetUID(555)
			c.SetMsToken("pinned-ms") // 阻止后台 mssToken 换取
			c.Ticket = "user-ticket"
			c.TsSign = "ts.2.abc"
			c.PrivateKey = testIMPrivateKeyHex

			if _, err := tc.call(ctx, c); err != nil {
				t.Fatalf("call: %v", err)
			}
			reqs := st.requests()

			// identity token GET：host/path + 关键 query。
			tok := imUnitFindRequest(t, reqs, func(r stubRequest) bool {
				return strings.Contains(r.URL, imIdentityTokenPath)
			})
			if tok.Method != "GET" {
				t.Errorf("token method = %q, want GET", tok.Method)
			}
			for _, frag := range []string{"scene=web_im", "aid=6383", "biz_trace_id="} {
				if !strings.Contains(tok.URL, frag) {
					t.Errorf("token url %q missing %q", tok.URL, frag)
				}
			}

			// 发送 POST：找到 cmd=100 的那条。
			send := imUnitFindRequest(t, reqs, func(r stubRequest) bool {
				if r.Method != "POST" || !strings.HasPrefix(r.URL, imapiBase+imMessageSendPath) {
					return false
				}
				env, err := pbParse(r.Body)
				return err == nil && pbInt(env, 1) == 100
			})
			if !strings.HasPrefix(send.URL, imapiBase+imMessageSendPath+"?") {
				t.Errorf("send url = %q", send.URL)
			}
			for _, frag := range []string{"msToken=pinned-ms", "a_bogus=", "verifyFp=", "fp="} {
				if !strings.Contains(send.URL, frag) {
					t.Errorf("send url %q missing %q", send.URL, frag)
				}
			}

			inner := imUnitAssertSendBody(t, send)
			if got := pbString(inner, 1); got != "0:1:5:6" {
				t.Errorf("conversation_id = %q", got)
			}
			if got := pbInt(inner, 2); got != 1 {
				t.Errorf("conversation_type = %d, want 1", got)
			}
			if got := pbInt(inner, 3); got != tc.shortID {
				t.Errorf("conversation_short_id = %d, want %d (precision lost?)", got, tc.shortID)
			}
			if got := pbInt(inner, 6); got != tc.wantType {
				t.Errorf("message_type = %d, want %d", got, tc.wantType)
			}
			if got := pbString(inner, 7); got != "ticket-x" {
				t.Errorf("ticket = %q", got)
			}
			clientMsgID := pbString(inner, 8)
			if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(clientMsgID) {
				t.Errorf("client_message_id = %q, want uuid4", clientMsgID)
			}

			ext := pbList(inner, 5)
			if len(ext) != 3 {
				t.Fatalf("ext = %v, want 3 entries", ext)
			}
			keys := make([]string, 0, len(ext))
			for _, e := range ext {
				raw, ok := e.([]byte)
				if !ok {
					t.Fatalf("ext entry not bytes: %v", e)
				}
				em, err := pbParse(raw)
				if err != nil {
					t.Fatalf("parse ext: %v", err)
				}
				keys = append(keys, pbString(em, 1))
			}
			wantKeys := []string{"s:mentioned_users", "s:client_message_id", "s:stime"}
			if strings.Join(keys, ",") != strings.Join(wantKeys, ",") {
				t.Errorf("ext keys = %v, want %v", keys, wantKeys)
			}

			var card map[string]any
			if err := json.Unmarshal([]byte(pbString(inner, 4)), &card); err != nil {
				t.Fatalf("content is not JSON: %v (%q)", err, pbString(inner, 4))
			}
			tc.checkCard(t, card)
		})
	}
}

// --- uuid / hex / content encoding ----------------------------------------

func TestIMUnitUUID4Contract(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 256 {
		u := imUUID4()
		if !re.MatchString(u) {
			t.Fatalf("imUUID4() = %q, not a v4 uuid", u)
		}
		if seen[u] {
			t.Fatalf("imUUID4() repeated: %q", u)
		}
		seen[u] = true
	}

	reHex := regexp.MustCompile(`^[0-9a-f]{8}$`)
	seenHex := map[string]bool{}
	for range 256 {
		h := imTraceID8()
		if !reHex.MatchString(h) {
			t.Fatalf("imTraceID8() = %q, want 8 lowercase hex", h)
		}
		if seenHex[h] {
			t.Fatalf("imTraceID8() repeated: %q", h)
		}
		seenHex[h] = true
	}
}

func TestIMUnitEncodeContent(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string_verbatim", `{"text":"hi"}`, `{"text":"hi"}`},
		{"empty_string", "", ""},
		{"int64_exact", map[string]any{"id": int64(math.MaxInt64)}, `{"id":9223372036854775807}`},
		{"map_keys_sorted", map[string]any{"b": 1, "a": 2}, `{"a":2,"b":1}`},
		{"list", []any{1, 2, 3}, `[1,2,3]`},
		{"nested", map[string]any{"nested": map[string]any{"x": true}, "arr": []any{"a"}}, `{"arr":["a"],"nested":{"x":true}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := imEncodeContent(tc.in); got != tc.want {
				t.Errorf("imEncodeContent(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIMUnitImStr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"s", "s"},
		{float64(42), "42"},
		{json.Number("9007199254740993"), "9007199254740993"},
		{true, "true"},
		{false, "false"},
		{int(7), "7"},
	}
	for _, tc := range cases {
		if got := imStr(tc.in); got != tc.want {
			t.Errorf("imStr(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// message_type 推断优先级：显式 message_type > aweType 映射 > awemeType 68 > 报错。
func TestIMUnitCardMessageType(t *testing.T) {
	cases := []struct {
		name    string
		card    map[string]any
		want    int
		wantErr bool
	}{
		{"explicit", map[string]any{"message_type": 42}, 42, false},
		{"explicit_zero_falls_through", map[string]any{"message_type": 0, "aweType": 800}, imShareAweme, false},
		{"awe800", map[string]any{"aweType": 800}, imShareAweme, false},
		{"awe510", map[string]any{"aweType": 510}, imBigEmoji, false},
		{"awe15001", map[string]any{"aweType": 15001}, imFile, false},
		{"awe2702", map[string]any{"aweType": 2702}, imStoryPic, false},
		{"awe700", map[string]any{"aweType": 700}, imText, false},
		{"awemeType68", map[string]any{"awemeType": 68}, imSharePhotos, false},
		{"unknown", map[string]any{"aweType": 1}, 0, true},
		{"empty", map[string]any{}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := imCardMessageType(tc.card)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %d, want %d", got, tc.want)
			}
		})
	}
}

// --- error mapping ----------------------------------------------------------

func TestIMUnitErrorMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("server_code", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return imUnitResponse(10086, "request.MGet empty", 0, nil), nil
		})
		_, err := c.IMLeaveConversation(ctx, "cid", 42, 2)
		if err == nil {
			t.Fatal("want error for non-zero server code")
		}
		if !strings.Contains(err.Error(), "10086") || !strings.Contains(err.Error(), "request.MGet empty") {
			t.Errorf("error = %v, want code and message", err)
		}
		if n := len(st.requests()); n != 1 {
			t.Errorf("requests = %d, want 1", n)
		}
	})

	t.Run("malformed_body", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(500, "\xff\xff"), nil
		})
		_, err := c.IMListConversations(ctx)
		if err == nil || !strings.Contains(err.Error(), "解析响应失败") {
			t.Errorf("error = %v, want parse failure", err)
		}
		if n := len(st.requests()); n != 1 {
			t.Errorf("requests = %d, want 1", n)
		}
	})

	t.Run("token_not_json", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "[1,2]"), nil
		})
		c.SetMsToken("m")
		c.Ticket = "t"
		c.TsSign = "ts.2.x"
		c.PrivateKey = testIMPrivateKeyHex
		_, _, err := c.GetIdentitySecurityToken(ctx, true)
		if err == nil || !strings.Contains(err.Error(), "不可解析响应") {
			t.Errorf("error = %v, want undecodable-token error", err)
		}
	})
}

// --- input validation -------------------------------------------------------

// 校验类失败必须在发出任何请求之前返回。
func TestIMUnitValidationZeroRequests(t *testing.T) {
	ctx := t.Context()

	cases := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{
			name: "share_aweme_empty_id",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ShareAweme(ctx, "cid", 1, "t", "")
				return err
			},
		},
		{
			name: "share_photos_empty_id",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.SharePhotos(ctx, "cid", 1, "t", "")
				return err
			},
		},
		{
			name: "audio_file_path",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.SendIMAudio(ctx, "cid", 1, "t", "/tmp/voice.mp3")
				return err
			},
		},
		{
			name: "card_no_type",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.SendIMCard(ctx, "cid", 1, "t", map[string]any{})
				return err
			},
		},
		{
			name: "audio_bad_json",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.SendIMAudio(ctx, "cid", 1, "t", `{not json`)
				return err
			},
		},
		{
			name: "video_empty_path",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.SendIMVideoWithThumb(ctx, "cid", 1, "t", "", "")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := newStubClient(t, func(stubRequest) (*Response, error) {
				return imUnitResponse(0, "", 100, &pbw{}), nil
			})
			c.SetUID(7)
			c.SetMsToken("m")
			if err := tc.call(ctx, c); err == nil {
				t.Fatal("want validation error")
			}
			if n := len(st.requests()); n != 0 {
				t.Errorf("issued %d requests, want 0 before validation", n)
			}
		})
	}

	t.Run("im_user_info_empty", func(t *testing.T) {
		c, st := newStubClient(t, nil)
		out, err := c.IMUserInfo(ctx, nil)
		if err != nil || out != nil {
			t.Fatalf("IMUserInfo(nil) = %v, %v; want nil, nil", out, err)
		}
		if n := len(st.requests()); n != 0 {
			t.Errorf("issued %d requests, want 0", n)
		}
	})
}

// --- im_core.go decode helpers ---------------------------------------------

func TestIMUnitWireDecodeHelpers(t *testing.T) {
	t.Run("known_and_unknown_fields", func(t *testing.T) {
		w := &pbw{}
		w.IntAlways(1, 123)
		w.Str(2, "abc")
		w.IntAlways(99, 7) // 未知字段照样解码
		fields, err := imDecodeWire(w.b)
		if err != nil {
			t.Fatalf("imDecodeWire: %v", err)
		}
		f1, ok := imFirstWire(fields, 1)
		if !ok || f1.varint != 123 {
			t.Errorf("field1 = %+v", f1)
		}
		f2, ok := imFirstWire(fields, 2)
		if !ok || string(f2.bytes) != "abc" {
			t.Errorf("field2 = %+v", f2)
		}
		if _, ok := imFirstWire(fields, 99); !ok {
			t.Errorf("unknown field 99 missing: %+v", fields)
		}
		if _, ok := imFirstWire(fields, 77); ok {
			t.Errorf("absent field 77 reported present")
		}
	})

	t.Run("truncated_is_error", func(t *testing.T) {
		for _, raw := range [][]byte{
			{0x08},             // varint tag, no value
			{0x0a, 0x05, 0x01}, // bytes len 5, only 1 present
			{0x0d, 0x01},       // fixed32, short
			{0x0a},             // bytes tag, no length
			{0xff, 0xff},       // bad tag
		} {
			if _, err := imDecodeWire(raw); err == nil {
				t.Errorf("imDecodeWire(% x) = nil error, want truncated error", raw)
			}
		}
	})

	t.Run("server_message", func(t *testing.T) {
		if got := imServerMessage(nil); got != "" {
			t.Errorf("imServerMessage(nil) = %q, want empty", got)
		}
		w := &pbw{}
		w.Str(4, strings.Repeat("x", 200))
		fields, err := imDecodeWire(w.b)
		if err != nil {
			t.Fatalf("imDecodeWire: %v", err)
		}
		if got, want := imServerMessage(fields), "（服务端: "+strings.Repeat("x", 120)+"）"; got != want {
			t.Errorf("imServerMessage = %q, want %q", got, want)
		}
	})

	t.Run("pb_map_to_any", func(t *testing.T) {
		w := &pbw{}
		w.IntAlways(1, 42)
		w.Str(2, "hello")
		nested := &pbw{}
		nested.Str(1, "x")
		w.Msg(3, nested)
		w.Bytes(4, []byte(`{"json":true}`))
		m, err := pbParse(w.b)
		if err != nil {
			t.Fatalf("pbParse: %v", err)
		}
		out := pbMapToAny(m)
		if got := toInt64(out["1"]); got != 42 {
			t.Errorf("out[1] = %v", out["1"])
		}
		if got, _ := out["2"].(string); got != "hello" {
			t.Errorf("out[2] = %v", out["2"])
		}
		nestedOut, ok := out["3"].(map[string]any)
		if !ok || nestedOut["1"] != "x" {
			t.Errorf("out[3] = %#v", out["3"])
		}
		jsonOut, ok := out["4"].(map[string]any)
		if !ok || jsonOut["json"] != true {
			t.Errorf("out[4] = %#v", out["4"])
		}

		arr, ok := pbValueToAny([]any{int64(5), []byte(`[1,2]`)}).([]any)
		if !ok || len(arr) != 2 {
			t.Fatalf("pbValueToAny list = %#v", arr)
		}
		if toInt64(arr[0]) != 5 {
			t.Errorf("arr[0] = %v", arr[0])
		}
		inner, ok := arr[1].([]any)
		if !ok || len(inner) != 2 || toInt64(inner[1]) != 2 {
			t.Errorf("arr[1] = %#v", arr[1])
		}
	})

	t.Run("parse_message", func(t *testing.T) {
		w := &pbw{}
		w.Str(1, "0:1:1:2")
		w.IntAlways(2, 1)
		w.IntAlways(3, 9007199254740993) // >2^53
		w.IntAlways(4, 7)
		w.IntAlways(5, 42)
		w.IntAlways(6, int64(imText))
		w.IntAlways(7, 8888888888)
		w.Str(8, `{"text":"hello","kind":"x"}`)
		w.IntAlways(10, 1700000000000)
		w.Str(14, "MS4wLjA")
		m, err := pbParse(w.b)
		if err != nil {
			t.Fatalf("pbParse: %v", err)
		}
		msg := imParseMessage(m)
		if msg.ConversationID != "0:1:1:2" || msg.ConversationType != 1 {
			t.Errorf("conversation = %q/%d", msg.ConversationID, msg.ConversationType)
		}
		if msg.ServerMessageID != 9007199254740993 {
			t.Errorf("ServerMessageID = %d (precision lost?)", msg.ServerMessageID)
		}
		if msg.Index != 7 || msg.ConversationShortID != 42 || msg.MessageType != imText {
			t.Errorf("index/short/type = %d/%d/%d", msg.Index, msg.ConversationShortID, msg.MessageType)
		}
		if msg.SenderUID != 8888888888 || msg.SenderSecUID != "MS4wLjA" || msg.TimestampMs != 1700000000000 {
			t.Errorf("sender/sec/ts = %d/%q/%d", msg.SenderUID, msg.SenderSecUID, msg.TimestampMs)
		}
		if msg.Text() != "hello" || imStr(msg.Content["kind"]) != "x" {
			t.Errorf("text/content = %q/%v", msg.Text(), msg.Content)
		}
		if !strings.Contains(msg.ContentJSON, `"text":"hello"`) {
			t.Errorf("ContentJSON = %q", msg.ContentJSON)
		}
	})

	t.Run("parse_messages_skips_bad", func(t *testing.T) {
		w := &pbw{}
		w.Str(1, "0:1:1:2")
		w.IntAlways(3, 99)
		msgs := imParseMessages([]any{[]byte{0xff}, w.b, "not-bytes"})
		if len(msgs) != 1 {
			t.Fatalf("imParseMessages = %d messages, want 1", len(msgs))
		}
		if msgs[0].ServerMessageID != 99 || msgs[0].ConversationID != "0:1:1:2" {
			t.Errorf("msg = %+v", msgs[0])
		}
	})
}

// The 私信媒体发送 path validates the attachment kind before creating a
// conversation, so this list is a contract callers rely on.
func TestIMUnitMediaKindValidation(t *testing.T) {
	for _, kind := range []string{"image", "video", "audio", "file"} {
		if !ValidIMMediaKind(kind) {
			t.Errorf("ValidIMMediaKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"", "sticker", "Image", "video ", "img"} {
		if ValidIMMediaKind(kind) {
			t.Errorf("ValidIMMediaKind(%q) = true, want false", kind)
		}
	}
	if got, want := IMMediaKindList(), "image|video|audio|file"; got != want {
		t.Errorf("IMMediaKindList() = %q, want %q", got, want)
	}
}
