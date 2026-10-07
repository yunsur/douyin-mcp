package douyin

import (
	"os"
	"strings"
	"testing"
)

// Live check that a bare sec_user_id is expanded into the profile URL before
// it reaches the Referer header. Douyin answers a non-URL Referer with
// {"status_msg":"blocked","user":{}}, which is what the MCP tool used to get.
//
//	DOUYIN_LIVE_TEST=1 DOUYIN_SEC_UID=MS4wLjABAAAA... go test ./douyin/ -run TestLiveUserInfoBareSecUID -v
func TestLiveUserInfoBareSecUID(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the live user-info test")
	}
	secUID := strings.TrimSpace(os.Getenv("DOUYIN_SEC_UID"))
	if secUID == "" {
		t.Skip("set DOUYIN_SEC_UID to a target sec_user_id")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.GetUserInfo(t.Context(), secUID)
	if err != nil {
		t.Fatalf("GetUserInfo(%q): %v", secUID, err)
	}
	if msg, _ := res["status_msg"].(string); msg != "" {
		t.Fatalf("GetUserInfo(%q) 被拒: status_msg=%q", secUID, msg)
	}
	user, _ := res["user"].(map[string]any)
	nick, _ := user["nickname"].(string)
	if nick == "" {
		t.Fatalf("GetUserInfo(%q) 未返回昵称: %v", secUID, res)
	}
	t.Logf("nickname=%q follower_count=%v", nick, user["follower_count"])
}
