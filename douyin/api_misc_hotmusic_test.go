package douyin

import (
	"fmt"
	"os"
	"testing"
)

// hotMusicStr mirrors fmt.Sprint but keeps <nil> out of fingerprints.
func hotMusicStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// hotMusicTrunc clips long values for test logs.
func hotMusicTrunc(s string) string {
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

// collectMusicIDs walks a decoded response and returns the music ids it finds
// (video search results and the hot-word feed both embed a music object).
func collectMusicIDs(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		if m, ok := t["music"].(map[string]any); ok {
			id := hotMusicStr(m["id_str"])
			if id == "" || id == "<nil>" {
				id = hotMusicStr(m["id"])
			}
			if id != "" && id != "<nil>" {
				*out = append(*out, id)
			}
		}
		for _, val := range t {
			collectMusicIDs(val, out)
		}
	case []any:
		for _, item := range t {
			collectMusicIDs(item, out)
		}
	}
}

// Live checks for the hot-search-video + music-page endpoints
// (热搜词视频 / 音乐页).
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscHotMusic -v
func TestLiveMiscHotMusic(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live hot-music tests")
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

	report := func(name string, res map[string]any, err error, fingerprint string) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		t.Logf("%s status_code=%v %s keys=%v", name, res["status_code"], fingerprint, keysOf(res))
	}

	// 热搜榜里取一个真实的热词 + sentence_id（热搜词视频的必填参数）。
	hotword, sentenceID := "", ""
	if board, err := c.HotSearchBoard(ctx); err != nil {
		t.Logf("热搜榜拉取失败（将用兜底热词）: %v", err)
	} else if wl := duSlice(duMap(board["data"])["word_list"]); len(wl) > 0 {
		if m := duMap(wl[0]); m != nil {
			hotword = hotMusicStr(m["word"])
			sentenceID = hotMusicStr(m["sentence_id"])
		}
	}
	if hotword == "" || sentenceID == "" {
		hotword, sentenceID = "长假期兴趣班", "2682366"
	}
	t.Logf("热搜词 hotword=%q sentence_id=%s", hotword, sentenceID)

	// 该接口此前在 www.douyin.com 上每次都被 bdturing 拦截；hj 域 + 不带 uifid 实测通过。
	res, err := c.HotSearchVideos(ctx, hotword, sentenceID, "", "", "")
	report("get_hot_search_videos", res, err, "aweme_list 条数="+itoa(len(duSlice(res["aweme_list"]))))
	if err == nil && len(duSlice(res["aweme_list"])) == 0 {
		t.Logf("热搜词视频返回空列表，全部 keys=%v data=%v", keysOf(res), hotMusicTrunc(hotMusicStr(res["data"])))
	}

	// 从搜索结果里取真实 music_id 用于音乐接口。
	var musicIDs []string
	if sv, serr := c.SearchVideoWork(ctx, "风景", "0", "10", "0", "0", "", ""); serr == nil {
		collectMusicIDs(sv, &musicIDs)
	} else {
		t.Logf("搜索取 music_id 失败: %v", serr)
	}
	if len(musicIDs) == 0 {
		t.Fatalf("未能从搜索视频里取到 music_id，音乐接口无法实测")
	}
	musicID := musicIDs[0]
	t.Logf("候选 music_id=%v", musicIDs)

	// 搜索结果里排第一的常是「原声」且只有一条作品，用后续候选兜底。
	res, err = c.MusicAweme(ctx, musicID, "", "")
	report("get_music_aweme", res, err, "aweme_list 条数="+itoa(len(duSlice(res["aweme_list"])))+
		" has_more="+hotMusicStr(res["has_more"])+" cursor="+hotMusicStr(res["cursor"]))
	for i := 1; err == nil && len(duSlice(res["aweme_list"])) == 0 && i < len(musicIDs) && i < 3; i++ {
		musicID = musicIDs[i]
		res, err = c.MusicAweme(ctx, musicID, "", "")
		report("get_music_aweme(候选 "+itoa(i)+")", res, err, "aweme_list 条数="+itoa(len(duSlice(res["aweme_list"]))))
	}

	res, err = c.MusicDetail(ctx, musicID, "")
	report("get_music_detail", res, err, "music_info="+hotMusicStr(duMap(res["music_info"])["id_str"]))

	res, err = c.MusicListCollection(ctx, "", "")
	report("get_music_collection", res, err, "mc_list 条数="+itoa(len(duSlice(res["mc_list"]))))

	// collect_music 是写操作（会改账号收藏），不在 live 测试里调用。
}
