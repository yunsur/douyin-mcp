package main

// 商品/电商（misc）服务层封装：仅透传到 douyin.Client 对应方法。

import (
	"context"
)

// GetProductSKUList 获取商品规格/SKU 列表。
func (s *DouyinService) GetProductSKUList(ctx context.Context, productID, shopID string) (map[string]any, error) {
	return s.Client().GetProductSKUList(ctx, productID, shopID)
}
