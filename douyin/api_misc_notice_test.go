package douyin

import (
	"os"
	"testing"
)

// 点赞通知「谁赞了我」接口的 live 测试。
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscNotice -v
func TestLiveMiscNotice(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1")
	}
	c, err := NewClient(liveCookie(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	list, err := c.GetNoticeList(ctx, "0", "0", "10", "3")
	if err != nil {
		t.Fatalf("notice list: %v", err)
	}
	notices := duSlice(list["notice_list"])
	if len(notices) == 0 {
		t.Skip("no like notices for this account")
	}
	nid := duStr(duMap(notices[0])["nid_str"])
	if nid == "" {
		t.Fatalf("notice has no nid_str: %v", notices[0])
	}

	res, err := c.GetNoticeDiggList(ctx, nid, "", "", "")
	if err != nil {
		t.Fatalf("notice digg list(%s): %v", nid, err)
	}
	if code := toInt64(res["status_code"]); code != 0 {
		t.Fatalf("notice digg list(%s) status_code=%v msg=%v", nid, res["status_code"], res["status_msg"])
	}
	t.Logf("notice digg list ok: notice_id=%s total=%v has_more=%v digg_list=%d user_list=%d",
		nid, res["total"], res["has_more"], len(duSlice(res["digg_list"])), len(duSlice(res["user_list"])))

	// notice_id 是唯一被接受的键：nid_str 会返回 5（参数不合法）。
	if _, err := c.GetNoticeDiggList(ctx, "", "", "", ""); err == nil {
		t.Log("空 notice_id 未被拒绝（服务端容忍）")
	}
}
