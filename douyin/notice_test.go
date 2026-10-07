package douyin

import (
	"strings"
	"testing"
)

// The notice tabs are addressed by notice_group; these are the ids the current
// PC client uses (they also appear in /notice/count/'s interactive_group list).
func TestNoticeGroupConstants(t *testing.T) {
	want := map[string]string{
		"all": NoticeGroupAll, "fans": NoticeGroupFans, "mentions": NoticeGroupMentions,
		"comments": NoticeGroupComments, "diggs": NoticeGroupDiggs, "danmaku": NoticeGroupDanmaku,
	}
	expect := map[string]string{"all": "700", "fans": "401", "mentions": "601", "comments": "2", "diggs": "3", "danmaku": "520"}
	for name, got := range want {
		if got != expect[name] {
			t.Errorf("notice group %s = %q want %q", name, got, expect[name])
		}
	}
}

func TestNoticeStatusError(t *testing.T) {
	if err := noticeStatusError("op", map[string]any{"status_code": int64(0)}); err != nil {
		t.Fatalf("status 0 must not error: %v", err)
	}
	err := noticeStatusError("获取通知详情", map[string]any{"status_code": int64(5), "status_msg": "参数不合法"})
	if err == nil {
		t.Fatal("non-zero status must surface an error")
	}
	if !strings.Contains(err.Error(), "参数不合法") || !strings.Contains(err.Error(), "5") {
		t.Fatalf("error should carry code and message, got %v", err)
	}
}

func TestNoticeDeleteRequiresID(t *testing.T) {
	c, err := NewClient("a=b", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.NoticeDelete(t.Context(), "0", ""); err == nil {
		t.Fatal("empty notice id must be rejected before any request")
	}
	if _, err := c.NoticeDetail(t.Context(), ""); err == nil {
		t.Fatal("empty notice id must be rejected for detail")
	}
}
