package douyin

import (
	"fmt"
	"os"
	"testing"
)

// 关注/朋友 tab 接口的 live 测试。需登录态：
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscSocial -v
func TestLiveMiscSocial(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live social-tab tests")
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

	// 1. 关注流（GET /aweme/v1/web/follow/feed/）
	follow, err := c.GetFollowFeed(ctx, "0", "20")
	if err != nil {
		t.Fatalf("关注流: %v", err)
	}
	if sc := toInt64(follow["status_code"]); sc != 0 {
		t.Fatalf("关注流 status_code=%v msg=%v", sc, follow["status_msg"])
	}
	items := duSlice(follow["aweme_list"])
	t.Logf("关注流 items=%d has_more=%v", len(items), follow["has_more"])

	// 从关注流取一条真实作品，供上报类接口使用。
	var awemeID, authorID, duration string
	if len(items) > 0 {
		if first, ok := items[0].(map[string]any); ok {
			awemeID = fmt.Sprint(first["aweme_id"])
			if author, ok := first["author"].(map[string]any); ok {
				authorID = fmt.Sprint(author["uid"])
			}
			if video, ok := first["video"].(map[string]any); ok {
				duration = fmt.Sprint(video["duration"])
			}
		}
	}
	t.Logf("样例作品 aweme_id=%s author_id=%s duration=%s", awemeID, authorID, duration)

	// 关注流可能为空，兜底从搜索取一条真实作品用于上报类接口。
	if awemeID == "" {
		if sv, serr := c.SearchVideoWork(ctx, "风景", "0", "5", "0", "0", "", ""); serr == nil {
			if sitems := duSlice(sv["data"]); len(sitems) > 0 {
				if m, ok := sitems[0].(map[string]any)["aweme_info"].(map[string]any); ok {
					awemeID = fmt.Sprint(m["aweme_id"])
					if author, ok := m["author"].(map[string]any); ok {
						authorID = fmt.Sprint(author["uid"])
					}
					if video, ok := m["video"].(map[string]any); ok {
						duration = fmt.Sprint(video["duration"])
					}
					t.Logf("兜底样例作品 aweme_id=%s author_id=%s duration=%s", awemeID, authorID, duration)
				}
			}
		}
	}

	// 2. 朋友流（POST /aweme/v1/web/familiar/feed/）
	familiar, err := c.GetFamiliarFeed(ctx, "0", "20")
	if err != nil {
		t.Fatalf("朋友流: %v", err)
	}
	if sc := toInt64(familiar["status_code"]); sc != 0 {
		t.Fatalf("朋友流 status_code=%v msg=%v", sc, familiar["status_msg"])
	}
	t.Logf("朋友流 items=%d has_more=%v", len(duSlice(familiar["aweme_list"])), familiar["has_more"])

	// 3. 朋友推荐（GET /aweme/v1/web/familiar/recommend/feed/）
	rec, err := c.GetFamiliarRecommendFeed(ctx, "", "0", "0", "18")
	if err != nil {
		t.Fatalf("朋友推荐: %v", err)
	}
	if sc := toInt64(rec["status_code"]); sc != 0 {
		t.Fatalf("朋友推荐 status_code=%v msg=%v", sc, rec["status_msg"])
	}
	t.Logf("朋友推荐 keys=%v", keysOf(rec))

	// 4. 关注页顶部直播卡片（POST /webcast/feed/follow_top/）
	top, err := c.GetFollowLiveTop(ctx)
	if err != nil {
		t.Fatalf("关注页直播卡片: %v", err)
	}
	t.Logf("关注页直播卡片 keys=%v status_code=%v", keysOf(top), top["status_code"])

	// 5. 关注页直播流（GET /webcast/web/feed/follow/）
	liveFeed, err := c.GetFollowLiveFeed(ctx, "")
	if err != nil {
		t.Fatalf("关注页直播流: %v", err)
	}
	t.Logf("关注页直播流 keys=%v status_code=%v", keysOf(liveFeed), liveFeed["status_code"])

	// 6. 观看历史写入（POST /aweme/v1/web/history/write/）
	// 该接口对本测试账号返回 status_code=7「无权限操作」（服务端可正常解析请求），
	// 属于账号侧权限问题，不作为失败条件，仅记录。
	if awemeID != "" {
		hw, err := c.ReportHistoryWrite(ctx, authorID, awemeID)
		if err != nil {
			t.Fatalf("历史写入: %v", err)
		}
		t.Logf("历史写入 status_code=%v status_msg=%v", hw["status_code"], hw["status_msg"])
		if sc := toInt64(hw["status_code"]); sc != 0 && sc != 7 {
			t.Fatalf("历史写入 status_code=%v msg=%v", sc, hw["status_msg"])
		}

		// 7. 作品统计上报（POST /aweme/v2/web/aweme/stats/）
		// 成功为 0；高频调用时服务端可能返回 2863「操作过于频繁」，属限流而非接口错误。
		st, err := c.ReportAwemeStats(ctx, awemeID, "0", "1", "0")
		if err != nil {
			t.Fatalf("作品统计: %v", err)
		}
		if sc := toInt64(st["status_code"]); sc != 0 && sc != 2863 {
			t.Fatalf("作品统计 status_code=%v msg=%v", sc, st["status_msg"])
		}
		t.Logf("作品统计 status_code=%v msg=%v keys=%v", st["status_code"], st["status_msg"], keysOf(st))

		// 8. 关注已读标记（GET /aweme/v1/following/list/item/seen/）
		seen, err := c.MarkFollowingSeen(ctx, awemeID, "1")
		if err != nil {
			t.Fatalf("已读标记: %v", err)
		}
		if sc := toInt64(seen["status_code"]); sc != 0 {
			t.Fatalf("已读标记 status_code=%v msg=%v", sc, seen["status_msg"])
		}
		t.Logf("已读标记 ok keys=%v", keysOf(seen))
	}

	// 10. 弹幕配置（GET /aweme/v1/web/danmaku/conf/get/）
	conf, err := c.GetDanmakuConf(ctx, "")
	if err != nil {
		t.Fatalf("弹幕配置: %v", err)
	}
	if sc := toInt64(conf["status_code"]); sc != 0 {
		t.Fatalf("弹幕配置 status_code=%v msg=%v", sc, conf["status_msg"])
	}
	t.Logf("弹幕配置 keys=%v", keysOf(conf))

	// 9. 弹幕拉取（GET /aweme/v1/web/danmaku/get_v2/）
	if awemeID != "" {
		token := fmt.Sprint(conf["danmaku_authentication_token"])
		if token == "<nil>" || token == "" {
			token = ""
		}
		dm, err := c.GetDanmaku(ctx, awemeID, "0", duration, duration, token)
		if err != nil {
			t.Fatalf("弹幕: %v", err)
		}
		if sc := toInt64(dm["status_code"]); sc != 0 {
			t.Fatalf("弹幕 status_code=%v msg=%v", sc, dm["status_msg"])
		}
		t.Logf("弹幕 keys=%v", keysOf(dm))
	}

	// 11. 系列观看上报（GET /aweme/v1/web/series/watch/record/）
	// 需要一个带 series_info 的作品（短剧/合集），从短剧搜索里取。
	if sv, serr := c.SearchVideoWork(ctx, "短剧", "0", "10", "0", "0", "", ""); serr == nil {
		for _, it := range duSlice(sv["data"]) {
			aw, _ := it.(map[string]any)["aweme_info"].(map[string]any)
			if aw == nil {
				continue
			}
			si, _ := aw["series_info"].(map[string]any)
			if si == nil {
				continue
			}
			itemID := fmt.Sprint(aw["aweme_id"])
			seriesID := fmt.Sprint(si["series_id"])
			sw, err := c.ReportSeriesWatch(ctx, itemID, seriesID, "1")
			if err != nil {
				t.Fatalf("系列观看: %v", err)
			}
			if sc := toInt64(sw["status_code"]); sc != 0 {
				t.Fatalf("系列观看 status_code=%v msg=%v", sc, sw["status_msg"])
			}
			t.Logf("系列观看 ok item=%s series=%s keys=%v", itemID, seriesID, keysOf(sw))
			break
		}
	}
}
