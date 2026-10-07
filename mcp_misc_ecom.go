package main

// 商品/电商（misc）MCP 工具注册。由 registerTools 统一调用。

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool argument structs -------------------------------------------------

type productSKUListArgs struct {
	ProductID string `json:"product_id" jsonschema:"商品 id（必填）"`
	ShopID    string `json:"shop_id" jsonschema:"店铺 id（必填）"`
}

// registerMiscEcomTools 注册商品/电商相关的 MCP 工具。
func registerMiscEcomTools(server *mcp.Server, appServer *AppServer) {
	svc := appServer.service

	addTool(server, "get_product_sku_list", "获取商品规格/SKU 列表（商品详情页「选择规格」，POST /aweme/v1/web/ecom/product/sku/list/）", true, appServer,
		func(ctx context.Context, args productSKUListArgs) (*MCPToolResult, error) {
			res, err := svc.GetProductSKUList(ctx, args.ProductID, args.ShopID)
			if err != nil {
				return nil, err
			}
			return textResult(res), nil
		})
}
