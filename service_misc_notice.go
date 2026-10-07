package main

// 点赞通知里的「谁赞了我」列表（notice/digg/list）。

import (
	"context"
	"fmt"
	"strings"
)

// NoticeDiggList returns the likers behind a like notification
// (`/aweme/v1/web/notice/digg/list/`). notice_id 必填：该接口只认这个参数名。
func (s *DouyinService) NoticeDiggList(ctx context.Context, noticeID, count, maxTime, minTime string) (map[string]any, error) {
	if strings.TrimSpace(noticeID) == "" {
		return nil, fmt.Errorf("缺少 notice_id（取 get_notices 结果里的 nid_str，作为 notice_id 传入）")
	}
	return s.Client().GetNoticeDiggList(ctx, noticeID, count, maxTime, minTime)
}
