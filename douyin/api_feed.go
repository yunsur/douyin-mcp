package douyin

// Home recommendation feed (抖音 Web 端).

import (
	"context"
)

// GetFeed returns the home recommendation feed through the V2 endpoint used
// by the current PC Web client.
func (c *Client) GetFeed(ctx context.Context, count, refreshIndex string) (map[string]any, error) {
	// 当前 Web 客户端已切换到 V2 POST；复用统一实现，独立调用使用
	// use_lite_type=1 以返回 JSON 视频流。
	return c.GetChannelModuleFeed(ctx, "", count, refreshIndex, "1", "", "", "", "")
}
