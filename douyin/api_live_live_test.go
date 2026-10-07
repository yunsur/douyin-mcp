package douyin

import (
	"os"
	"strings"
	"testing"
)

// Live check that the webcast endpoints send the query they signed. a_bogus is
// computed over p.SpliceURL(); sending p.ToString() instead makes the webcast
// gateway answer HTTP 200 with an empty body, which the callers used to report
// as "no data".
//
//	DOUYIN_LIVE_TEST=1 DOUYIN_LIVE_ROOM=<room_id> go test ./douyin/ -run TestLiveLiveRankSignedQuery -v
func TestLiveLiveRankSignedQuery(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the live rank test")
	}
	roomID := strings.TrimSpace(os.Getenv("DOUYIN_LIVE_ROOM"))
	if roomID == "" {
		t.Skip("set DOUYIN_LIVE_ROOM to a live room_id")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.GetRankList(t.Context(), roomID, "", "")
	if err != nil {
		t.Fatalf("GetRankList(%q): %v", roomID, err)
	}
	if _, ok := res["data"]; !ok {
		t.Fatalf("rank list 返回体缺少 data（多为签名与 wire query 不一致导致的空响应）: %v", res)
	}
	t.Logf("rank list ok: status_code=%v", res["status_code"])
}
