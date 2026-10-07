package douyin

import (
	"os"
	"testing"
)

// Live checks for the extra search surfaces.
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveSearchSurfaces -v
func TestLiveSearchSurfaces(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live search tests")
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

	sug, err := c.SearchSuggest(ctx, "榴莲")
	if err != nil {
		t.Fatalf("搜索联想: %v", err)
	}
	if toInt64(sug["status_code"]) != 0 {
		t.Fatalf("搜索联想 status_code=%v", sug["status_code"])
	}
	if got := len(duSlice(sug["sug_list"])); got == 0 {
		t.Fatalf("搜索联想未返回建议 (keys=%v)", keysOf(sug))
	} else {
		t.Logf("搜索联想 %d 条", got)
	}

	hot, err := c.HotSearchBoard(ctx)
	if err != nil {
		t.Fatalf("热搜榜: %v", err)
	}
	data, _ := hot["data"].(map[string]any)
	words := duSlice(data["word_list"])
	if len(words) == 0 {
		t.Fatalf("热搜榜未返回词条 (data keys=%v)", keysOf(data))
	}
	t.Logf("热搜榜 %d 条，trending %d 条", len(words), len(duSlice(data["trending_list"])))

	ch, err := c.SearchChallenges(ctx, "榴莲", "0", "10")
	if err != nil {
		t.Fatalf("话题搜索: %v", err)
	}
	if toInt64(ch["status_code"]) != 0 {
		t.Fatalf("话题搜索 status_code=%v", ch["status_code"])
	}
	t.Logf("话题搜索 %d 条", len(duSlice(ch["challenge_list"])))
}
