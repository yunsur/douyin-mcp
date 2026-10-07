package main

// 商品/电商（misc）HTTP 路由注册。由 setupRoutes 在 /api/v1 分组内调用。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// --- Request payloads ------------------------------------------------------

type miscEcomProductSKURequest struct {
	ProductID string `json:"product_id" binding:"required"`
	ShopID    string `json:"shop_id" binding:"required"`
}

// --- Handlers --------------------------------------------------------------

func (s *AppServer) miscEcomProductSKUHandler(c *gin.Context) {
	var req miscEcomProductSKURequest
	if !bindOrFail(c, &req) {
		return
	}
	res, err := s.service.GetProductSKUList(c.Request.Context(), req.ProductID, req.ShopID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PRODUCT_SKU_LIST_FAILED", "获取商品规格失败", err.Error())
		return
	}
	respondSuccess(c, res, "获取商品规格成功")
}

// registerMiscEcomRoutes 注册商品/电商相关路由。
func registerMiscEcomRoutes(api *gin.RouterGroup, appServer *AppServer) {
	api.POST("/ecom/product/sku/list", appServer.miscEcomProductSKUHandler)
}
