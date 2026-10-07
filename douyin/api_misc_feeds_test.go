package douyin

import (
	"os"
	"testing"
)

// Live checks for the feeds-slice endpoints (精选/推荐 tab).
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscFeeds -v
func TestLiveMiscFeeds(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live feeds tests")
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

	// 默认（不带 use_lite_type）走 1：JSON 视频流，必须真的有内容。
	res, err := c.GetChannelModuleFeed(ctx, "", "", "", "", "", "", "", "")
	report("get_channel_module_feed(默认 lite=1)", res, err, "aweme_list 条数="+itoa(len(duSlice(res["aweme_list"]))))
	if err == nil && len(duSlice(res["aweme_list"])) == 0 {
		t.Errorf("默认参数未返回 aweme_list：%v", keysOf(res))
	}

	// 显式 2 与浏览器一致：返回 protobuf，已被解码成 field_* 结构。
	res, err = c.GetChannelModuleFeed(ctx, "", "", "", "2", "", "", "", "")
	report("get_channel_module_feed(lite=2 protobuf)", res, err, "field_18 条数="+itoa(len(duSlice(res["field_18"]))))

	// 精选「公开课」是唯一有独立接口的分类。
	res, err = c.GetCourseCategoryVideos(ctx, "", "", "", "", "")
	report("get_course_category_videos", res, err, "")

	res, err = c.GetCourseCategoryTags(ctx, "")
	report("get_course_category_tags", res, err, "")

	res, err = c.GetSolutionResources(ctx, "", "")
	report("get_solution_resources", res, err, "")

	res, err = c.GetMulticastConfig(ctx)
	report("get_multicast_config", res, err, "")

	res, err = c.PageTurnOffline(ctx)
	report("page_turn_offline", res, err, "")

	res, err = c.GetEmojiList(ctx, "")
	report("get_emoji_list", res, err, "emoji 条数="+itoa(len(duSlice(res["emoji_list"]))))

	res, err = c.GetPublishHighlight(ctx, "")
	report("get_publish_highlight", res, err, "")

	res, err = c.GetMixListCollection(ctx, "", "")
	report("get_mix_list_collection", res, err, "data 条数="+itoa(len(duSlice(res["data"]))))

	res, err = c.GetSEOInnerLink(ctx)
	report("get_seo_inner_link", res, err, "")

	res, err = c.GetStudyNotes(ctx, "", "", "")
	report("get_study_notes", res, err, "")
}
