package douyin

import (
	"os"
	"testing"
)

// Live check for the product SKU endpoint.
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMiscEcom -v
func TestLiveMiscEcom(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live ecom tests")
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

	// 商品来自直播小黄车：product_id=3753424245829533915, shop_id=179437132。
	res, err := c.GetProductSKUList(ctx, "3753424245829533915", "179437132")
	if err != nil {
		t.Fatalf("获取商品 SKU 失败: %v", err)
	}
	if toInt64(res["status_code"]) != 0 {
		t.Fatalf("status_code=%v msg=%v keys=%v", res["status_code"], res["status_msg"], keysOf(res))
	}
	data, _ := res["data"].(map[string]any)
	specs := duSlice(data["specs"])
	if len(specs) == 0 {
		t.Fatalf("未返回 specs (data keys=%v)", keysOf(data))
	}
	first, _ := specs[0].(map[string]any)
	items := duSlice(first["spec_items"])
	firstItem := ""
	if len(items) > 0 {
		if it, ok := items[0].(map[string]any); ok {
			firstItem = "id=" + liveString(it["id"]) + " name=" + liveString(it["name"])
		}
	}
	t.Logf("status_code=%v specs=%d first_spec=%v spec_items=%d first_item=%s",
		res["status_code"], len(specs), first["name"], len(items), firstItem)
}
