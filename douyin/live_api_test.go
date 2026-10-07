package douyin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// liveCookie reads the cookie header from DOUYIN_COOKIES or the usual
// cookies.txt, without depending on the application's cookies package.
//
// Live network material is only handed out when DOUYIN_LIVE_TEST=1: otherwise
// a stray cookies.txt in the working tree would make `go test ./...` call the
// real Douyin API, which must never happen in the default/hermetic run.
func liveCookie() string {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		return ""
	}
	if v := strings.TrimSpace(os.Getenv("DOUYIN_COOKIES")); v != "" {
		return v
	}
	path := strings.TrimSpace(os.Getenv("DOUYIN_COOKIES_FILE"))
	if path == "" {
		path = "cookies.txt"
	}
	raw, err := os.ReadFile(filepath.Join("..", path))
	if err != nil {
		if raw, err = os.ReadFile(path); err != nil {
			return ""
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\n", ""))
}

// Live network checks against the real Douyin API. They are skipped unless
// DOUYIN_LIVE_TEST=1 and a cookie/uid/sec_uid are provided, so `go test ./...`
// stays hermetic.
//
//	DOUYIN_LIVE_TEST=1 DOUYIN_TEST_UID=<uid> DOUYIN_TEST_SEC_UID=<sec_uid> \
//	  go test ./douyin/ -run TestLiveFollowingListReturnsItems -v
func liveClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live network tests")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available (DOUYIN_COOKIES or cookies.txt)")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Regression for upstream PR #86 / issue #85: with max_time=0 the following
// list endpoint answers status_code=0 but an empty list; a current timestamp
// (what GetUserFollowingList now substitutes) must return the real items.
func TestLiveFollowingListReturnsItems(t *testing.T) {
	c := liveClient(t)
	uid := os.Getenv("DOUYIN_TEST_UID")
	sec := os.Getenv("DOUYIN_TEST_SEC_UID")
	if uid == "" || sec == "" {
		t.Skip("set DOUYIN_TEST_UID and DOUYIN_TEST_SEC_UID")
	}

	ctx := t.Context()
	fixed, err := c.GetUserFollowingList(ctx, uid, sec, "", "20")
	if err != nil {
		t.Fatalf("following list (max_time auto): %v", err)
	}
	items := len(duSlice(fixed["followings"]))
	switch duStr(fixed["status_code"]) {
	case "0":
		// The regression: status_code=0 with an empty list means the request
		// took the "recommend" branch (upstream PR #86 / issue #85).
		if items == 0 {
			t.Fatalf("status_code=0 but no followings: max_time substitution is not in effect (total=%v has_more=%v)",
				fixed["total"], fixed["has_more"])
		}
		if duStr(fixed["total"]) == "0" {
			t.Fatalf("status_code=0 but total=0: %v", fixed["total"])
		}
	case "2096":
		// Privacy-restricted: the payload omits total/has_more and returns a
		// null list; the client must return empty without erroring (PR #86b).
		if items != 0 {
			t.Fatalf("status_code=2096 but %d followings returned", items)
		}
		t.Logf("target restricts its following list (status_code=2096); handled as an empty list")
	default:
		t.Logf("upstream status_code=%v, items=%d (no assertion)", fixed["status_code"], items)
	}
}
