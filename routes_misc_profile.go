package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 「我的」tab 的 HTTP 路由。

type profileUserDashboardRequest struct {
	UserID string `json:"user_id"`
}

type profileUserSettingsRequest struct {
	Source                  string `json:"source"`
	IsFetchFrequencyControl string `json:"is_fetch_frequency_control"`
	HasLocalCache           string `json:"has_local_cache"`
	RequestSource           string `json:"request_source"`
}

type profileCustomSettingsRequest struct {
	SettingTypes string `json:"setting_types"`
}

type profileCollectedAwemesRequest struct {
	Count  string `json:"count"`
	Cursor string `json:"cursor"`
}

type profileBindingSubjectRequest struct {
	AccountUID string `json:"account_uid"`
}

// registerMiscProfile_Routes 注册「我的」tab 相关 HTTP 接口。
func registerMiscProfile_Routes(api *gin.RouterGroup, appServer *AppServer) {
	svc := appServer.service

	api.POST("/user/my-profile", func(c *gin.Context) {
		res, err := svc.GetMyProfile(c.Request.Context())
		if err != nil {
			respondError(c, http.StatusInternalServerError, "MY_PROFILE_FAILED", "获取我的资料失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取我的资料成功")
	})

	api.POST("/user/dashboard", func(c *gin.Context) {
		var req profileUserDashboardRequest
		if !bindOrFail(c, &req) {
			return
		}
		res, err := svc.GetUserDashboard(c.Request.Context(), req.UserID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "USER_DASHBOARD_FAILED", "获取用户数据面板失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取用户数据面板成功")
	})

	api.POST("/user/social-count", func(c *gin.Context) {
		res, err := svc.GetSocialCount(c.Request.Context())
		if err != nil {
			respondError(c, http.StatusInternalServerError, "SOCIAL_COUNT_FAILED", "获取社交计数失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取社交计数成功")
	})

	api.POST("/user/settings", func(c *gin.Context) {
		var req profileUserSettingsRequest
		if !bindOrFail(c, &req) {
			return
		}
		res, err := svc.GetUserSettings(c.Request.Context(), req.Source, req.IsFetchFrequencyControl, req.HasLocalCache, req.RequestSource)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "USER_SETTINGS_FAILED", "获取用户设置失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取用户设置成功")
	})

	api.POST("/user/custom-settings", func(c *gin.Context) {
		var req profileCustomSettingsRequest
		if !bindOrFail(c, &req) {
			return
		}
		res, err := svc.GetCustomSettings(c.Request.Context(), req.SettingTypes)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "CUSTOM_SETTINGS_FAILED", "获取自定义设置失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取自定义设置成功")
	})

	api.POST("/user/collected-awemes", func(c *gin.Context) {
		var req profileCollectedAwemesRequest
		if !bindOrFail(c, &req) {
			return
		}
		res, err := svc.GetCollectedAwemes(c.Request.Context(), req.Count, req.Cursor)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "COLLECTED_AWEMES_FAILED", "获取收藏作品失败", err.Error())
			return
		}
		respondSuccess(c, res, "获取收藏作品成功")
	})

	api.POST("/baike/check-worldbook", func(c *gin.Context) {
		res, err := svc.BaikeCheckWorldbook(c.Request.Context())
		if err != nil {
			respondError(c, http.StatusInternalServerError, "BAIKE_WORLDBOOK_FAILED", "查询百科词条权限失败", err.Error())
			return
		}
		respondSuccess(c, res, "查询百科词条权限成功")
	})

	api.POST("/baike/binding-subject", func(c *gin.Context) {
		var req profileBindingSubjectRequest
		if !bindOrFail(c, &req) {
			return
		}
		res, err := svc.BaikeBindingSubject(c.Request.Context(), req.AccountUID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "BAIKE_BINDING_FAILED", "查询百科绑定主体失败", err.Error())
			return
		}
		respondSuccess(c, res, "查询百科绑定主体成功")
	})
}
