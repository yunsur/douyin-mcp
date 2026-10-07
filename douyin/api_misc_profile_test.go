package douyin

import (
	"os"
	"testing"
)

// Live checks for the「我的」tab endpoints (profile / settings / collect / baike).
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscProfile -v
func TestLiveMiscProfile(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the misc-profile live test")
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

	assertStatus := func(name string, fn func() (map[string]any, error)) map[string]any {
		t.Helper()
		res, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if msg, _ := res["status_msg"].(string); msg != "" && msg != "success" {
			t.Fatalf("%s 被拒: status_msg=%q", name, msg)
		}
		if code := toInt64(res["status_code"]); code != 0 {
			t.Fatalf("%s status_code=%d res=%v", name, code, res)
		}
		return res
	}

	t.Run("GetMyProfile", func(t *testing.T) {
		res := assertStatus("GetMyProfile", func() (map[string]any, error) { return c.GetMyProfile(ctx) })
		user := duMap(res["user"])
		t.Logf("GetMyProfile keys=%d nickname=%q follower_count=%v",
			len(user), duStr(user["nickname"]), user["follower_count"])
	})

	t.Run("GetUserDashboard", func(t *testing.T) {
		res := assertStatus("GetUserDashboard", func() (map[string]any, error) { return c.GetUserDashboard(ctx, "") })
		t.Logf("GetUserDashboard keys=%d sample=%v", len(res), miscProfileFirstKey(res))
	})

	t.Run("GetSocialCount", func(t *testing.T) {
		res := assertStatus("GetSocialCount", func() (map[string]any, error) { return c.GetSocialCount(ctx) })
		t.Logf("GetSocialCount keys=%d following=%v follower=%v",
			len(res), res["following_count"], res["follower_count"])
	})

	t.Run("GetUserSettingsWWW", func(t *testing.T) {
		res := assertStatus("GetUserSettings(www)", func() (map[string]any, error) {
			return c.GetUserSettings(ctx, "www", "", "", "")
		})
		t.Logf("GetUserSettings(www) keys=%d sample=%v", len(res), miscProfileFirstKey(res))
	})

	t.Run("GetUserSettingsHJ", func(t *testing.T) {
		res := assertStatus("GetUserSettings(hj)", func() (map[string]any, error) {
			return c.GetUserSettings(ctx, "hj", "", "", "")
		})
		t.Logf("GetUserSettings(hj) keys=%d sample=%v", len(res), miscProfileFirstKey(res))
	})

	t.Run("GetCustomSettings", func(t *testing.T) {
		res := assertStatus("GetCustomSettings", func() (map[string]any, error) { return c.GetCustomSettings(ctx, "") })
		t.Logf("GetCustomSettings keys=%d", len(res))
	})

	t.Run("GetCollectedAwemes", func(t *testing.T) {
		res := assertStatus("GetCollectedAwemes", func() (map[string]any, error) {
			return c.GetCollectedAwemes(ctx, "", "")
		})
		items := duSlice(res["aweme_list"])
		fp := ""
		if len(items) > 0 {
			w := duMap(items[0])
			fp = duStr(w["aweme_id"])
		}
		t.Logf("GetCollectedAwemes count=%d has_more=%v first_aweme_id=%s", len(items), res["has_more"], fp)
	})

	t.Run("BaikeCheckWorldbook", func(t *testing.T) {
		res := assertStatus("BaikeCheckWorldbook", func() (map[string]any, error) { return c.BaikeCheckWorldbook(ctx) })
		data := duMap(res["data"])
		t.Logf("BaikeCheckWorldbook is_hit=%v", data["is_hit"])
	})

	t.Run("BaikeBindingSubject", func(t *testing.T) {
		res := assertStatus("BaikeBindingSubject", func() (map[string]any, error) {
			return c.BaikeBindingSubject(ctx, "")
		})
		t.Logf("BaikeBindingSubject keys=%d data=%v", len(res), res["data"])
	})
}

// miscProfileFirstKey returns one representative key/value for a fingerprint log.
func miscProfileFirstKey(m map[string]any) string {
	for k, v := range m {
		return k + "=" + duStr(v)
	}
	return ""
}
