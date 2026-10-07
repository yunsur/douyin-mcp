package main

import "context"

// 「我的」tab 服务的薄封装，只透传参数与默认值。

// GetMyProfile 获取登录用户自己的资料。
func (s *DouyinService) GetMyProfile(ctx context.Context) (map[string]any, error) {
	return s.Client().GetMyProfile(ctx)
}

// GetUserDashboard 获取用户数据面板；userID 为空时查自己。
func (s *DouyinService) GetUserDashboard(ctx context.Context, userID string) (map[string]any, error) {
	return s.Client().GetUserDashboard(ctx, userID)
}

// GetSocialCount 获取社交计数（关注/粉丝/获赞等）。
func (s *DouyinService) GetSocialCount(ctx context.Context) (map[string]any, error) {
	return s.Client().GetSocialCount(ctx)
}

// GetUserSettings 获取用户设置（source: www | hj）。
func (s *DouyinService) GetUserSettings(ctx context.Context, source, isFetchFrequencyControl, hasLocalCache, requestSource string) (map[string]any, error) {
	return s.Client().GetUserSettings(ctx, source, isFetchFrequencyControl, hasLocalCache, requestSource)
}

// GetCustomSettings 获取自定义设置项；settingTypes 为空时默认 "1"。
func (s *DouyinService) GetCustomSettings(ctx context.Context, settingTypes string) (map[string]any, error) {
	return s.Client().GetCustomSettings(ctx, settingTypes)
}

// GetCollectedAwemes 获取「收藏」tab 的收藏作品列表。
func (s *DouyinService) GetCollectedAwemes(ctx context.Context, count, cursor string) (map[string]any, error) {
	return s.Client().GetCollectedAwemes(ctx, count, cursor)
}

// BaikeCheckWorldbook 查询账号是否有 IP 百科词条访问权限。
func (s *DouyinService) BaikeCheckWorldbook(ctx context.Context) (map[string]any, error) {
	return s.Client().BaikeCheckWorldbook(ctx)
}

// BaikeBindingSubject 查询账号绑定的百科主体；accountUID 为空时查自己。
func (s *DouyinService) BaikeBindingSubject(ctx context.Context, accountUID string) (map[string]any, error) {
	return s.Client().BaikeBindingSubject(ctx, accountUID)
}
