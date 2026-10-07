package douyin

// Hermetic unit tests for the live-room HTTP surface (api_live.go,
// live_production.go) and the PK message/cache logic (pk.go). They drive a
// stub Transport, so no network, browser or wall clock is involved.

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

// liveUnitClient builds a stub client with msToken and webid pinned so the
// live methods never spawn the background mssdk fetch or the device-id lookup.
func liveUnitClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	c, st := newStubClient(t, handle)
	c.SetMsToken("fix-ms-token")
	c.SetWebID("fix-web-id")
	return c, st
}

func liveUnitParseURL(t *testing.T, raw string) (*url.URL, url.Values) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u, u.Query()
}

func liveUnitOK(stubRequest) (*Response, error) {
	return jsonResponse(map[string]any{"status_code": 0}), nil
}

// TestLiveHTTPRankEndpoints locks the wire contract (method, host, path,
// required params and referer) of the live.douyin.com GET endpoints.
func TestLiveHTTPRankEndpoints(t *testing.T) {
	cases := []struct {
		name    string
		call    func(context.Context, *Client) error
		path    string
		referer string
		enter   string
		params  map[string]string
	}{
		{
			name: "room_enter",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetLiveRoomEnter(ctx, "403309276429")
				return err
			},
			path:    "/webcast/room/web/enter/",
			referer: liveBase + "/403309276429",
			enter:   "link_share",
			params:  map[string]string{"web_rid": "403309276429"},
		},
		{
			name: "linkmic_list",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetLiveLinkmicList(ctx, "881", "99")
				return err
			},
			path:    "/webcast/linkmic/list/",
			referer: liveBase,
			enter:   "link_share",
			params: map[string]string{
				"room_id": "881", "channel_id": "99", "offset": "0", "count": "50",
				"link_status": "4", "scene": "1", "request_source": "audience_enter_room", "anchor_id": "",
			},
		},
		{
			name: "pk_contribution_rank",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetLivePKContributionRank(ctx, "99", "77")
				return err
			},
			path:    "/webcast/linkmic/battle/ranklist_armies/",
			referer: liveBase,
			enter:   "link_share",
			params:  map[string]string{"channel_id": "99", "anchor_id": "77"},
		},
		{
			name: "contribution_rank",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetLiveContributionRank(ctx, "881", "77")
				return err
			},
			path:    "/webcast/ranklist/audience/",
			referer: liveBase,
			enter:   "web_live",
			params: map[string]string{
				"webcast_sdk_version": "2450", "room_id": "881", "anchor_id": "77",
				"sec_anchor_id": "", "ignoreToast": "true", "rank_type": "30",
			},
		},
		{
			name: "rank_list_alias",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetRankList(ctx, "881", "77", "88")
				return err
			},
			path:    "/webcast/ranklist/audience/",
			referer: liveBase,
			enter:   "web_live",
			params:  map[string]string{"room_id": "881", "anchor_id": "77", "sec_anchor_id": "88"},
		},
		{
			name: "thousand_ticket_rank",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetLiveThousandTicketRank(ctx, "881", "403309276429")
				return err
			},
			path:    "/webcast/ranklist/paygrade_seats/",
			referer: liveBase + "/403309276429",
			enter:   "web_live",
			params:  map[string]string{"webcast_sdk_version": "2450", "room_id": "881", "seats_type": "2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, st := liveUnitClient(t, liveUnitOK)
			if err := tc.call(t.Context(), c); err != nil {
				t.Fatalf("call: %v", err)
			}
			req := st.last(t)
			if req.Method != fhttp.MethodGet {
				t.Fatalf("method = %s, want GET", req.Method)
			}
			u, q := liveUnitParseURL(t, req.URL)
			if u.Host != liveSubHost {
				t.Errorf("host = %q, want %q", u.Host, liveSubHost)
			}
			if u.Path != tc.path {
				t.Errorf("path = %q, want %q", u.Path, tc.path)
			}
			if got := q.Get("aid"); got != "6383" {
				t.Errorf("aid = %q, want 6383", got)
			}
			if got := q.Get("enter_from"); got != tc.enter {
				t.Errorf("enter_from = %q, want %q", got, tc.enter)
			}
			if got := q.Get("msToken"); got != "fix-ms-token" {
				t.Errorf("msToken = %q, want pinned token", got)
			}
			if q.Get("a_bogus") == "" {
				t.Error("a_bogus missing from signed query")
			}
			for key, want := range tc.params {
				got, ok := q[key]
				if !ok {
					t.Errorf("param %q missing", key)
					continue
				}
				if len(got) != 1 || got[0] != want {
					t.Errorf("param %q = %v, want %q", key, got, want)
				}
			}
			if got := req.Header("referer"); got != tc.referer {
				t.Errorf("referer = %q, want %q", got, tc.referer)
			}
		})
	}
}

// TestLiveRoomEnterDecode verifies the canned success body is decoded with
// 64-bit ids intact, and that a non-JSON body maps to a risk error.
func TestLiveRoomEnterDecode(t *testing.T) {
	body := `{"status_code":0,"data":{"data":[{"id_str":"7687099550015753000","owner":{"nickname":"主播"},"linker_map":{"1":7687114086841259023}}]}}`
	c, _ := liveUnitClient(t, func(stubRequest) (*Response, error) {
		return statusResponse(200, body), nil
	})
	res, err := c.GetLiveRoomEnter(t.Context(), "403309276429")
	if err != nil {
		t.Fatalf("GetLiveRoomEnter: %v", err)
	}
	if got := toInt64(res["status_code"]); got != 0 {
		t.Fatalf("status_code = %d, want 0", got)
	}
	room := liveObj(liveArr(liveObj(res["data"])["data"])[0])
	if liveIDText(room, "id") != "7687099550015753000" {
		t.Errorf("room id = %v", room["id_str"])
	}
	if got := liveIDText(liveObj(room["linker_map"]), "1"); got != "7687114086841259023" {
		t.Errorf("linker_map id = %q, want the 64-bit value", got)
	}

	t.Run("empty_body", func(t *testing.T) {
		c2, _ := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, ""), nil
		})
		_, err := c2.GetLiveRoomEnter(t.Context(), "403309276429")
		if err == nil || !strings.Contains(err.Error(), "空响应") {
			t.Fatalf("err = %v, want empty-response mapping", err)
		}
	})
}

// TestLiveInfoHTTP checks the landing-page request and ttwid absorption, plus
// the parse failure path.
func TestLiveInfoHTTP(t *testing.T) {
	page := `<html><script nonce="x">{\"roomId\":\"7654321098765432100\",\"user_unique_id\":\"1234567890\",\"roomInfo\":{\"room\":{\"id_str\":\"7654321098765432100\",\"status\":2,\"status_str\":\"2\",\"title\":\"标题\"}},\"anchor\":{\"id_str\":\"111222333\"},\"sec_uid\":\"SEC\"}</script></html>`
	c, st := liveUnitClient(t, func(stubRequest) (*Response, error) {
		resp := statusResponse(200, page)
		resp.Cookies = []*fhttp.Cookie{ck("ttwid", "TTWID-1")}
		return resp, nil
	})
	info, err := c.GetLiveInfo(t.Context(), "7654321098765432100")
	if err != nil {
		t.Fatalf("GetLiveInfo: %v", err)
	}
	if info["room_id"] != "7654321098765432100" || info["anchor_id"] != "111222333" ||
		info["sec_uid"] != "SEC" || info["ttwid"] != "TTWID-1" {
		t.Fatalf("info = %#v", info)
	}
	if got := c.Cookie.Get("ttwid"); got != "TTWID-1" {
		t.Errorf("absorbed ttwid = %q, want TTWID-1", got)
	}
	req := st.last(t)
	if req.Method != fhttp.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	u, _ := liveUnitParseURL(t, req.URL)
	if u.Host != liveSubHost || u.Path != "/7654321098765432100" {
		t.Errorf("url = %s", req.URL)
	}
	if got := req.Header("referer"); got != liveBase+"/?from_nav=1" {
		t.Errorf("referer = %q", got)
	}

	t.Run("unparsable", func(t *testing.T) {
		c2, _ := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "<html>nothing here</html>"), nil
		})
		if _, err := c2.GetLiveInfo(t.Context(), "42"); err == nil || !strings.Contains(err.Error(), "42") {
			t.Fatalf("err = %v, want parse failure naming the id", err)
		}
	})
}

// TestLivePKContextHTTPFlow exercises the two-request orchestration and the
// resulting context.
func TestLivePKContextHTTPFlow(t *testing.T) {
	var paths []string
	c, _ := liveUnitClient(t, func(r stubRequest) (*Response, error) {
		u, _ := url.Parse(r.URL)
		paths = append(paths, u.Path)
		switch u.Path {
		case "/webcast/room/web/enter/":
			return jsonResponse(map[string]any{
				"status_code": 0,
				"data": map[string]any{"data": []any{map[string]any{
					"id_str":     "7687099550015753000",
					"owner":      map[string]any{"id_str": "1565319530819928", "nickname": "主播"},
					"linker_map": map[string]any{"1": "7687114086841259023"},
				}}},
			}), nil
		case "/webcast/linkmic/list/":
			return jsonResponse(map[string]any{
				"status_code": 0,
				"data": map[string]any{"battle_stats": map[string]any{
					"battle_settings": map[string]any{
						"battle_id_str":  "7687114318937084966",
						"channel_id_str": "7687114086841259023",
						"finished":       0,
					},
					"user_infos": map[string]any{
						"7585857326805287985": map[string]any{
							"user":    map[string]any{"user_id_str": "7585857326805287985", "nick_name": "对手"},
							"room_id": "7687000000000000001",
						},
					},
				}},
			}), nil
		}
		return liveUnitOK(r)
	})

	res, err := c.GetLivePKContext(t.Context(), "403309276429", "")
	if err != nil {
		t.Fatalf("GetLivePKContext: %v", err)
	}
	if got := strings.Join(paths, ","); got != "/webcast/room/web/enter/,/webcast/linkmic/list/" {
		t.Fatalf("request order = %q", got)
	}
	if res["battle_id"] != "7687114318937084966" || res["channel_id"] != "7687114086841259023" {
		t.Errorf("context = %#v", res)
	}
	if anchors := liveArr(res["anchors"]); len(anchors) != 2 {
		t.Fatalf("anchors = %#v, want owner + opponent", res["anchors"])
	}
}

// TestLivePKContextAPIErrorMapping checks that a non-zero business status is
// surfaced as *LivePKAPIError with the endpoint and raw response preserved.
func TestLivePKContextAPIErrorMapping(t *testing.T) {
	t.Run("room_enter", func(t *testing.T) {
		c, _ := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 12012, "data": map[string]any{}}), nil
		})
		_, err := c.GetLivePKContext(t.Context(), "403309276429", "")
		apiErr, ok := errors.AsType[*LivePKAPIError](err)
		if !ok {
			t.Fatalf("err = %v, want *LivePKAPIError", err)
		}
		if apiErr.Endpoint != "room/web/enter" || apiErr.StatusCode != 12012 {
			t.Errorf("apiErr = %+v", apiErr)
		}
		if toInt64(apiErr.Response["status_code"]) != 12012 {
			t.Errorf("raw response not preserved: %#v", apiErr.Response)
		}
	})

	t.Run("linkmic_list", func(t *testing.T) {
		c, _ := liveUnitClient(t, func(r stubRequest) (*Response, error) {
			u, _ := url.Parse(r.URL)
			if u.Path == "/webcast/room/web/enter/" {
				return jsonResponse(map[string]any{
					"status_code": 0,
					"data": map[string]any{"data": []any{map[string]any{
						"id_str":     "7687099550015753000",
						"owner":      map[string]any{"id_str": "1565319530819928"},
						"linker_map": map[string]any{"1": "7687114086841259023"},
					}}},
				}), nil
			}
			return jsonResponse(map[string]any{"status_code": 20003}), nil
		})
		_, err := c.GetLivePKContext(t.Context(), "403309276429", "")
		apiErr, ok := errors.AsType[*LivePKAPIError](err)
		if !ok || apiErr.Endpoint != "linkmic/list" || apiErr.StatusCode != 20003 {
			t.Fatalf("err = %v, want linkmic/list *LivePKAPIError", err)
		}
	})
}

// TestLivePKRankStates covers side validation, the no-battle case and the
// non-zero rank status mapping.
func TestLivePKRankStates(t *testing.T) {
	t.Run("bad_side", func(t *testing.T) {
		c, st := liveUnitClient(t, liveUnitOK)
		if _, err := c.GetLivePKRank(t.Context(), "403309276429", "left"); err == nil {
			t.Fatal("expected error for unsupported side")
		}
		if got := len(st.requests()); got != 0 {
			t.Fatalf("issued %d requests before validating side", got)
		}
	})

	t.Run("not_in_pk", func(t *testing.T) {
		c, st := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{
				"status_code": 0,
				"data": map[string]any{"data": []any{map[string]any{
					"id_str":     "7687099550015753000",
					"owner":      map[string]any{"id_str": "1565319530819928"},
					"linker_map": map[string]any{},
				}}},
			}), nil
		})
		res, err := c.GetLivePKRank(t.Context(), "403309276429", "current")
		if err != nil {
			t.Fatalf("GetLivePKRank: %v", err)
		}
		if res["state"] != "not_in_pk" {
			t.Errorf("state = %v, want not_in_pk", res["state"])
		}
		if liveArr(res["ranks"]) != nil {
			t.Errorf("ranks = %#v, want empty", res["ranks"])
		}
		if got := len(st.requests()); got != 1 {
			t.Errorf("issued %d requests, want 1 (no linkmic query without a channel)", got)
		}
	})

	run := func(t *testing.T, rankStatus int64) (map[string]any, []stubRequest) {
		t.Helper()
		c, st := liveUnitClient(t, func(r stubRequest) (*Response, error) {
			u, _ := url.Parse(r.URL)
			switch u.Path {
			case "/webcast/room/web/enter/":
				return jsonResponse(map[string]any{
					"status_code": 0,
					"data": map[string]any{"data": []any{map[string]any{
						"id_str":     "7687099550015753000",
						"owner":      map[string]any{"id_str": "1565319530819928", "nickname": "主播"},
						"linker_map": map[string]any{"1": "7687114086841259023"},
					}}},
				}), nil
			case "/webcast/linkmic/list/":
				return jsonResponse(map[string]any{
					"status_code": 0,
					"data": map[string]any{"battle_stats": map[string]any{
						"battle_settings": map[string]any{
							"battle_id_str":  "7687114318937084966",
							"channel_id_str": "7687114086841259023",
						},
					}},
				}), nil
			case "/webcast/linkmic/battle/ranklist_armies/":
				return jsonResponse(map[string]any{"status_code": rankStatus}), nil
			}
			return liveUnitOK(r)
		})
		res, err := c.GetLivePKRank(t.Context(), "403309276429", "current")
		if err != nil {
			t.Fatalf("GetLivePKRank: %v", err)
		}
		return res, st.requests()
	}

	t.Run("ok", func(t *testing.T) {
		res, reqs := run(t, 0)
		if res["state"] != "ok" {
			t.Fatalf("state = %v, want ok", res["state"])
		}
		ranks := liveObj(res["ranks"])
		if _, ok := ranks["1565319530819928"]; !ok {
			t.Fatalf("ranks not keyed by anchor id: %#v", ranks)
		}
		if got := len(reqs); got != 5 {
			t.Fatalf("issued %d requests, want 5 (context, rank, context)", got)
		}
	})

	t.Run("api_error", func(t *testing.T) {
		res, _ := run(t, 88)
		if res["state"] != "api_error" {
			t.Fatalf("state = %v, want api_error", res["state"])
		}
	})
}

// TestLiveProductionEndpoints covers the 小黄车 promotion/detail/comments
// requests and the empty-body promotion boundary.
func TestLiveProductionEndpoints(t *testing.T) {
	pageURL := "https://live.douyin.com/403309276429"

	t.Run("promotions", func(t *testing.T) {
		c, st := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0, "promotions": []any{}}), nil
		})
		if _, err := c.GetLiveProduction(t.Context(), pageURL, "881", "77", "0"); err != nil {
			t.Fatalf("GetLiveProduction: %v", err)
		}
		req := st.last(t)
		if req.Method != fhttp.MethodGet {
			t.Fatalf("method = %s", req.Method)
		}
		u, q := liveUnitParseURL(t, req.URL)
		if u.Host != liveSubHost || u.Path != "/live/promotions/pop/v3/" {
			t.Errorf("url = %s", req.URL)
		}
		if q.Get("room_id") != "881" || q.Get("author_id") != "77" || q.Get("aid") != "6383" {
			t.Errorf("query = %v", q)
		}
		if q.Get("webid") != "fix-web-id" {
			t.Errorf("webid = %q", q.Get("webid"))
		}
		entrance := q.Get("entrance_info")
		if !strings.Contains(entrance, `"room_id":"881"`) || !strings.Contains(entrance, `"carrier_type":"live_popup_card"`) {
			t.Errorf("entrance_info = %q", entrance)
		}
		if got := req.Header("referer"); got != pageURL {
			t.Errorf("referer = %q, want page url", got)
		}
		if got := req.Header("sec-ch-ua"); got != "" {
			t.Errorf("sec-ch-ua should be dropped, got %q", got)
		}
	})

	t.Run("promotions_empty_body", func(t *testing.T) {
		c, _ := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, ""), nil
		})
		res, err := c.GetLiveProduction(t.Context(), pageURL, "881", "77", "0")
		if err != nil {
			t.Fatalf("GetLiveProduction: %v", err)
		}
		if promotions, ok := res["promotions"].([]any); !ok || len(promotions) != 0 {
			t.Fatalf("promotions = %#v, want empty slice", res["promotions"])
		}
	})

	t.Run("production_detail", func(t *testing.T) {
		c, st := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0}), nil
		})
		if _, err := c.GetLiveProductionDetail(t.Context(), pageURL, "P1", "638303"); err != nil {
			t.Fatalf("GetLiveProductionDetail: %v", err)
		}
		reqs := st.requests()
		if len(reqs) != 2 || reqs[0].Method != fhttp.MethodHead {
			t.Fatalf("requests = %#v, want HEAD csrf probe then POST", reqs)
		}
		post := reqs[1]
		if post.Method != fhttp.MethodPost {
			t.Fatalf("method = %s, want POST", post.Method)
		}
		u, q := liveUnitParseURL(t, post.URL)
		if u.Path != "/aweme/v2/shop/promotion/pack/detail/" {
			t.Errorf("path = %q", u.Path)
		}
		if q.Get("origin_type") != "638303" || q.Get("is_h5") != "1" {
			t.Errorf("query = %v", q)
		}
		body := string(post.Body)
		if !strings.Contains(body, "promotion_id=P1") || !strings.Contains(body, "bff_type=2") {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("product_comments", func(t *testing.T) {
		c, st := liveUnitClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"status_code": 0, "data": map[string]any{}}), nil
		})
		if _, err := c.GetProductComments(t.Context(), "P1", "S1", "C1"); err != nil {
			t.Fatalf("GetProductComments: %v", err)
		}
		req := st.last(t)
		u, q := liveUnitParseURL(t, req.URL)
		if u.Host != "www.douyin.com" || u.Path != "/aweme/v1/web/ecom/product/comments/" {
			t.Errorf("url = %s", req.URL)
		}
		if q.Get("product_id") != "P1" || q.Get("shop_id") != "S1" || q.Get("cursor") != "C1" || q.Get("count") != "10" {
			t.Errorf("query = %v", q)
		}

		if _, err := c.GetProductCommentCounter(t.Context(), "P1", "S1", "T1"); err != nil {
			t.Fatalf("GetProductCommentCounter: %v", err)
		}
		u2, q2 := liveUnitParseURL(t, st.last(t).URL)
		if u2.Path != "/aweme/v1/web/ecom/product/comment/counter/" || q2.Get("stat_id") != "T1" {
			t.Errorf("counter url = %s", st.last(t).URL)
		}
	})
}

// TestLiveValueContracts locks the JSON-value coercion contracts.
func TestLiveValueContracts(t *testing.T) {
	truthy := []struct {
		name string
		in   any
		want bool
	}{
		{"nil", nil, false},
		{"bool_true", true, true},
		{"bool_false", false, false},
		{"empty_string", "", false},
		{"nonempty_string", "x", true},
		{"zero_string_is_truthy", "0", true},
		{"int64_zero", int64(0), false},
		{"int64_nonzero", int64(5), true},
		{"int_zero", int(0), false},
		{"float_zero", float64(0), false},
		{"float_nonzero", float64(0.5), true},
		{"json_number_zero", json.Number("0"), false},
		{"json_number_zero_dot", json.Number("0.0"), true},
		{"json_number_nonzero", json.Number("12"), true},
		{"json_number_empty", json.Number(""), false},
		{"other_empty_map", map[string]any{}, true},
	}
	for _, tc := range truthy {
		t.Run("truthy_"+tc.name, func(t *testing.T) {
			if got := liveTruthy(tc.in); got != tc.want {
				t.Errorf("liveTruthy(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}

	strs := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string", "abc", "abc"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"int64", int64(-5), "-5"},
		{"int", 7, "7"},
		{"float_integral", float64(3), "3"},
		{"float_fraction", float64(3.5), "3.5"},
		{"json_number", json.Number("123"), "123"},
		{"nan", math.NaN(), "NaN"},
		{"default", []any{1, 2}, "[1 2]"},
	}
	for _, tc := range strs {
		t.Run("string_"+tc.name, func(t *testing.T) {
			if got := liveString(tc.in); got != tc.want {
				t.Errorf("liveString(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	// liveConvertNumbers must keep integers as int64 and fractions as float64.
	converted := liveConvertNumbers(map[string]any{
		"int":   json.Number("42"),
		"float": json.Number("1.25"),
		"bad":   json.Number("abc"),
		"bool":  true,
		"list":  []any{json.Number("2.5"), "s"},
	}).(map[string]any)
	if converted["int"] != int64(42) {
		t.Errorf("int = %#v, want int64(42)", converted["int"])
	}
	if converted["float"] != float64(1.25) {
		t.Errorf("float = %#v, want float64(1.25)", converted["float"])
	}
	if converted["bad"] != "abc" {
		t.Errorf("bad = %#v, want raw string", converted["bad"])
	}
	list := converted["list"].([]any)
	if list[0] != float64(2.5) || list[1] != "s" {
		t.Errorf("list = %#v", list)
	}
}

func TestLiveFormEncodeAndSortedKeys(t *testing.T) {
	forms := []struct {
		name  string
		pairs [][2]string
		want  string
	}{
		{"empty", nil, ""},
		{"single", [][2]string{{"a", "b"}}, "a=b"},
		{"multi", [][2]string{{"a", "b"}, {"c", "d"}}, "a=b&c=d"},
		{"space_plus", [][2]string{{"k", "a b"}}, "k=a+b"},
		{"reserved_escaped", [][2]string{{"k", "中 文&x=1"}}, "k=%E4%B8%AD+%E6%96%87%26x%3D1"},
		{"key_escaped", [][2]string{{"a b", "c"}}, "a+b=c"},
	}
	for _, tc := range forms {
		t.Run(tc.name, func(t *testing.T) {
			if got := liveFormEncode(tc.pairs); got != tc.want {
				t.Errorf("liveFormEncode(%v) = %q, want %q", tc.pairs, got, tc.want)
			}
		})
	}

	got := sortedKeys(map[string]any{"b": 1, "a": 2, "c": 3})
	if strings.Join(got, ",") != "a,b,c" {
		t.Errorf("sortedKeys = %v, want ascending", got)
	}
	if keys := sortedKeys(map[string]any{}); len(keys) != 0 {
		t.Errorf("sortedKeys(empty) = %#v, want empty slice", keys)
	}
	if keys := sortedKeys(nil); len(keys) != 0 {
		t.Errorf("sortedKeys(nil) = %#v, want empty slice", keys)
	}
}

// TestPKDecodeMappingHelpers locks the scalar coercions used by the message
// decoder.
func TestPKDecodeMappingHelpers(t *testing.T) {
	nums := []struct {
		name string
		in   any
		want int64
	}{
		{"nil", nil, 0},
		{"decimal_string", "42", 42},
		{"bad_string", "abc", 0},
		{"empty_string", "", 0},
		{"float_truncates", float64(3.9), 3},
		{"int64", int64(9), 9},
		{"int", int(4), 4},
		{"true", true, 1},
		{"false", false, 0},
		{"json_int", json.Number("77"), 77},
		{"json_float", json.Number("2.7"), 2},
		{"json_bad", json.Number("x"), 0},
	}
	for _, tc := range nums {
		t.Run("num_"+tc.name, func(t *testing.T) {
			if got := pkNum(tc.in); got != tc.want {
				t.Errorf("pkNum(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}

	if pkText("x") != "x" || pkText(5) != "" || pkText(nil) != "" {
		t.Error("pkText should only accept string values")
	}

	if got := pkID(5, ""); got != "5" {
		t.Errorf("pkID number = %#v, want \"5\"", got)
	}
	if got := pkID(5, "explicit"); got != "explicit" {
		t.Errorf("pkID explicit = %#v", got)
	}
	if got := pkID(0, ""); got != nil {
		t.Errorf("pkID zero = %#v, want nil", got)
	}

	times := []struct {
		in   int64
		want int64
	}{
		{0, 0},
		{-5, -5},
		{1_000_000_000, 1_000_000_000_000},
		{99_999_999_999, 99_999_999_999_000},
		{100_000_000_000, 100_000_000_000},
		{1_700_000_000_000, 1_700_000_000_000},
	}
	for _, tc := range times {
		if got := pkTimeMS(tc.in); got != tc.want {
			t.Errorf("pkTimeMS(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}

	ranks := pkRanks([]any{
		map[string]any{"user_id": json.Number("111"), "nickname": "A", "score": json.Number("10")},
		map[string]any{"user_id": int64(0), "user_id_str": "222", "nickname": "B", "score": "20"},
	})
	if len(ranks) != 2 {
		t.Fatalf("pkRanks len = %d", len(ranks))
	}
	first := liveObj(ranks[0])
	if first["rank"] != 1 || first["uid"] != "111" || first["nickname"] != "A" || first["score"] != "10" {
		t.Errorf("rank[0] = %#v", first)
	}
	second := liveObj(ranks[1])
	if second["rank"] != 2 || second["uid"] != "222" || second["score"] != "20" {
		t.Errorf("rank[1] = %#v", second)
	}

	scores := pkScores([]any{map[string]any{
		"user_id": "5", "score": "100", "score_blur_text": "x",
		"score_relative_text": "y", "battle_rank": json.Number("3"),
	}})
	score := liveObj(scores[0])
	if score["anchor_id"] != "5" || score["score"] != "100" || score["battle_rank"] != int64(3) ||
		score["score_blur_text"] != "x" || score["score_relative_text"] != "y" {
		t.Errorf("score = %#v", score)
	}

	armies := pkArmyAnchors(map[string]any{
		"10": map[string]any{"user_armies": []any{map[string]any{"user_id": "1", "nickname": "a", "score": "5"}}},
		"2":  map[string]any{"user_armies": []any{}},
		"x":  map[string]any{},
	})
	if len(armies) != 2 {
		t.Fatalf("pkArmyAnchors len = %d, want 2 (non-numeric keys skipped)", len(armies))
	}
	if liveObj(armies[0])["anchor_id"] != "2" || liveObj(armies[1])["anchor_id"] != "10" {
		t.Errorf("armies not sorted numerically: %#v", armies)
	}
	if users := liveArr(liveObj(armies[1])["users"]); len(users) != 1 || liveObj(users[0])["uid"] != "1" {
		t.Errorf("army users = %#v", liveObj(armies[1])["users"])
	}
}

// TestPKDecodeMessageDispatch checks each PK method mapping, the message_type
// gate and the (nil, nil) non-PK case.
func TestPKDecodeMessageDispatch(t *testing.T) {
	if pkCanonical("WebcastLinkMicMethod") != "LinkMicMethod" || pkCanonical("LinkMicMethod") != "LinkMicMethod" {
		t.Fatal("pkCanonical should strip the Webcast prefix")
	}

	t.Run("non_pk_method", func(t *testing.T) {
		event, err := DecodePKMessage("WebcastChatMessage", []byte("opaque"), 1)
		if err != nil || event != nil {
			t.Fatalf("non-PK method = %#v, %v; want nil, nil", event, err)
		}
	})

	t.Run("linkmic_method_other_type", func(t *testing.T) {
		msg, err := ProtoNew("douyin.pk.LinkMicMethod")
		if err != nil {
			t.Fatal(err)
		}
		if err := SetProtoField(msg, "message_type", 200); err != nil {
			t.Fatal(err)
		}
		payload, err := ProtoMarshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		event, err := DecodePKMessage("WebcastLinkMicMethod", payload, 5)
		if err != nil || event != nil {
			t.Fatalf("message_type=200 = %#v, %v; want nil, nil", event, err)
		}
	})

	t.Run("start", func(t *testing.T) {
		event, err := DecodePKMessage("WebcastLinkMicBattleMethod", liveUnhex(t, fxPKStart), 4242)
		if err != nil {
			t.Fatal(err)
		}
		if event["type"] != "start" || event["battle_id"] != fxBattle || event["channel_id"] != fxChannel {
			t.Fatalf("start = %#v", event)
		}
		if event["msg_id"] != "4242" {
			t.Errorf("msg_id = %v, want the supplied message id", event["msg_id"])
		}
	})

	t.Run("scores", func(t *testing.T) {
		event, err := DecodePKMessage("WebcastLinkMicMethod", liveUnhex(t, fxPKScores), 0)
		if err != nil {
			t.Fatal(err)
		}
		if event["type"] != "scores" || event["battle_id"] != fxBattle {
			t.Fatalf("scores = %#v", event)
		}
		scores := liveArr(event["scores"])
		if len(scores) == 0 {
			t.Fatal("scores list is empty")
		}
		first := liveObj(scores[0])
		if _, ok := first["anchor_id"].(string); !ok {
			t.Errorf("score anchor_id should be a string: %#v", first["anchor_id"])
		}
		if _, ok := first["score"].(string); !ok {
			t.Errorf("score should be a string: %#v", first["score"])
		}
	})

	t.Run("armies", func(t *testing.T) {
		event, err := DecodePKMessage("WebcastLinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 0)
		if err != nil {
			t.Fatal(err)
		}
		if event["type"] != "armies" || event["is_complete"] != false {
			t.Fatalf("armies = %#v", event)
		}
		if len(liveArr(event["anchors"])) == 0 {
			t.Error("armies anchors empty")
		}
	})

	t.Run("finish", func(t *testing.T) {
		event, err := DecodePKMessage("WebcastLinkMicBattleFinishMethod", liveUnhex(t, fxPKFinish), 0)
		if err != nil {
			t.Fatal(err)
		}
		if event["type"] != "finish" || event["battle_id"] != fxBattle {
			t.Fatalf("finish = %#v", event)
		}
		if len(liveArr(event["scores"])) == 0 || len(liveArr(event["anchors"])) == 0 {
			t.Errorf("finish missing scores/anchors: %#v", event)
		}
	})
}

// TestPKKeyCacheLRU checks the bounded recency cache: hit/miss, capacity
// eviction and the move-to-end behaviour.
func TestPKKeyCacheLRU(t *testing.T) {
	c := newPKKeyCache(2)
	if c.remember("a") {
		t.Fatal("first insert reported as duplicate")
	}
	if !c.remember("a") {
		t.Fatal("re-insert reported as new")
	}
	if c.has("missing") {
		t.Fatal("has() claimed a missing key")
	}
	if !c.has("a") {
		t.Fatal("has() lost a present key")
	}

	c.remember("b")
	// Touch "a" so "b" becomes the eviction candidate.
	if !c.remember("a") {
		t.Fatal("recency hit reported as new")
	}
	c.remember("c") // evicts "b"
	if c.has("b") {
		t.Fatal("least recently used key was not evicted")
	}
	if !c.has("a") || !c.has("c") {
		t.Fatalf("recent keys missing: a=%v c=%v", c.has("a"), c.has("c"))
	}
	if len(c.keys) != 2 || len(c.index) != 2 {
		t.Fatalf("cache not bounded: keys=%v index=%v", c.keys, c.index)
	}
	for i, key := range c.keys {
		if c.index[key] != i {
			t.Fatalf("index out of sync: keys=%v index=%v", c.keys, c.index)
		}
	}

	one := newPKKeyCache(1)
	one.remember("x")
	one.remember("y")
	if one.has("x") || !one.has("y") {
		t.Fatalf("limit=1 eviction wrong: %v", one.keys)
	}
}
