package douyin

import (
	"os"
	"testing"
)

// Live checks for the personal-profile tabs. Opt-in like the other live tests:
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveProfileTabs -v
func TestLiveProfileTabs(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live profile-tab tests")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	// 稍后再看: the response carries list_num + items.
	later, err := c.GetWatchLater(ctx, "0")
	if err != nil {
		t.Fatalf("稍后再看: %v", err)
	}
	if toInt64(later["status_code"]) != 0 {
		t.Fatalf("稍后再看 status_code=%v", later["status_code"])
	}
	t.Logf("稍后再看 list_num=%v items=%d", later["list_num"], len(duSlice(later["items"])))

	// 观看历史: paged list shape (aweme_list + max_cursor + has_more).
	hist, err := c.GetWatchHistory(ctx, "0", "20")
	if err != nil {
		t.Fatalf("观看历史: %v", err)
	}
	if toInt64(hist["status_code"]) != 0 {
		t.Fatalf("观看历史 status_code=%v", hist["status_code"])
	}
	for _, key := range []string{"aweme_list", "max_cursor", "has_more"} {
		if _, ok := hist[key]; !ok {
			t.Fatalf("观看历史 缺少字段 %s (keys=%v)", key, keysOf(hist))
		}
	}
	t.Logf("观看历史 items=%d max_cursor=%v has_more=%v", len(duSlice(hist["aweme_list"])), hist["max_cursor"], hist["has_more"])

	// 我的预约.
	appt, err := c.GetAppointments(ctx, "100", 0)
	if err != nil {
		t.Fatalf("我的预约: %v", err)
	}
	if toInt64(appt["status_code"]) != 0 {
		t.Fatalf("我的预约 status_code=%v", appt["status_code"])
	}
	t.Logf("我的预约 count=%d", len(duSlice(appt["appointment_list"])))

	// 喜欢: self account should expose liked works when the session allows it.
	sec, err := c.SecUID(ctx)
	if err != nil || sec == "" {
		t.Skipf("no sec_uid: %v", err)
	}
	fav, err := c.GetUserFavorite(ctx, sec, "0", "5")
	if err != nil {
		t.Fatalf("喜欢: %v", err)
	}
	t.Logf("喜欢 status_code=%v items=%d", fav["status_code"], len(duSlice(fav["aweme_list"])))
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
