package douyin

import (
	"os"
	"testing"
)

// Live 校验 IM 消息面板（misc）新增接口。
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscIM -v
func TestLiveMiscIM(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the IM misc live tests")
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

	secUID := ""
	if id, err := c.SecUID(ctx); err == nil {
		secUID = id
	}
	t.Logf("当前登录 sec_uid=%q", secUID)

	// 1. 好友/关系列表
	rel, err := c.IMGetSpotlightRelation(ctx, "", "")
	if err != nil {
		t.Fatalf("get_im_spotlight_relation: %v", err)
	}
	if code := toInt64(rel["status_code"]); code != 0 {
		t.Fatalf("get_im_spotlight_relation status_code=%v (%v)", rel["status_code"], rel["status_msg"])
	}
	t.Logf("get_im_spotlight_relation status_code=0 keys=%d", len(rel))

	// 2. 批量在线状态
	st, err := c.IMGetActiveStatus(ctx, nil, []string{secUID})
	if err != nil {
		t.Fatalf("get_im_active_status: %v", err)
	}
	if code := toInt64(st["status_code"]); code != 0 {
		t.Fatalf("get_im_active_status status_code=%v (%v)", st["status_code"], st["status_msg"])
	}
	t.Logf("get_im_active_status status_code=0 keys=%d", len(st))

	// 3. 在线心跳
	hb, err := c.IMActiveHeartbeat(ctx, "")
	if err != nil {
		t.Fatalf("im_active_heartbeat: %v", err)
	}
	if code := toInt64(hb["status_code"]); code != 0 {
		t.Fatalf("im_active_heartbeat status_code=%v (%v)", hb["status_code"], hb["status_msg"])
	}
	t.Logf("im_active_heartbeat status_code=0 keys=%d", len(hb))

	// 4. 在线状态配置
	cfg, err := c.IMGetActiveConfig(ctx)
	if err != nil {
		t.Fatalf("get_im_active_config: %v", err)
	}
	if code := toInt64(cfg["status_code"]); code != 0 {
		t.Fatalf("get_im_active_config status_code=%v (%v)", cfg["status_code"], cfg["status_msg"])
	}
	t.Logf("get_im_active_config status_code=0 keys=%d", len(cfg))

	// 5. 策略配置
	sc, err := c.IMGetStrategyConfig(ctx, "")
	if err != nil {
		t.Fatalf("get_im_strategy_config: %v", err)
	}
	if code := toInt64(sc["status_code"]); code != 0 {
		t.Fatalf("get_im_strategy_config status_code=%v (%v)", sc["status_code"], sc["status_msg"])
	}
	t.Logf("get_im_strategy_config status_code=0 keys=%d", len(sc))

	// 6. 资源聚合
	res, err := c.IMGetResources(ctx, "", "", "")
	if err != nil {
		t.Fatalf("get_im_resources: %v", err)
	}
	if code := toInt64(res["status_code"]); code != 0 {
		t.Fatalf("get_im_resources status_code=%v (%v)", res["status_code"], res["status_msg"])
	}
	t.Logf("get_im_resources status_code=0 keys=%d", len(res))

	// 7. 热门表情
	emo, err := c.IMGetEmoticonTrending(ctx, "", "", "")
	if err != nil {
		t.Fatalf("get_im_emoticon_trending: %v", err)
	}
	if code := toInt64(emo["status_code"]); code != 0 {
		t.Fatalf("get_im_emoticon_trending status_code=%v (%v)", emo["status_code"], emo["status_msg"])
	}
	t.Logf("get_im_emoticon_trending status_code=0 keys=%d", len(emo))

	// 8. 在线反馈入口
	fb, err := c.IMGetFeedbackEntrance(ctx, "")
	if err != nil {
		t.Fatalf("get_im_feedback_entrance: %v", err)
	}
	if code := toInt64(fb["status_code"]); code != 0 {
		t.Fatalf("get_im_feedback_entrance status_code=%v (%v)", fb["status_code"], fb["status_msg"])
	}
	t.Logf("get_im_feedback_entrance status_code=0 keys=%d", len(fb))

	// 9. protobuf 增量拉消息（cmd 2048）
	pull, err := c.IMPullMessages(ctx, 0, 0)
	if err != nil {
		t.Fatalf("pull_im_messages: %v", err)
	}
	if len(pull) == 0 {
		t.Fatalf("pull_im_messages 返回空响应")
	}
	for _, k := range []string{"1", "2", "4"} {
		if v, ok := pull[k]; ok {
			t.Logf("pull_im_messages field %s type=%T value=%v", k, v, v)
		}
	}
	t.Logf("pull_im_messages keys=%d fields=%v", len(pull), keysOf(pull))
}
