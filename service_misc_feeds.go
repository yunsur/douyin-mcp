package main

// Thin service wrappers for the feeds-slice endpoints (精选/推荐 tab).

import (
	"context"
)

// GetChannelModuleFeed returns the V2 channel module feed.
func (s *DouyinService) GetChannelModuleFeed(ctx context.Context, moduleID, count, refreshIndex, useLiteType, preItemIDs, preLogID, encodedPreItemIDs, encodedPreRoomIDs string) (map[string]any, error) {
	return s.Client().GetChannelModuleFeed(ctx, moduleID, count, refreshIndex, useLiteType, preItemIDs, preLogID, encodedPreItemIDs, encodedPreRoomIDs)
}

// GetCourseCategoryVideos returns the 公开课 category's video list.
func (s *DouyinService) GetCourseCategoryVideos(ctx context.Context, tabID, offset, size, tagIDList, idList string) (map[string]any, error) {
	return s.Client().GetCourseCategoryVideos(ctx, tabID, offset, size, tagIDList, idList)
}

// GetCourseCategoryTags returns the course category tags.
func (s *DouyinService) GetCourseCategoryTags(ctx context.Context, tabID string) (map[string]any, error) {
	return s.Client().GetCourseCategoryTags(ctx, tabID)
}

// GetSolutionResources returns the sub-tab solution resources.
func (s *DouyinService) GetSolutionResources(ctx context.Context, spotKeys, appID string) (map[string]any, error) {
	return s.Client().GetSolutionResources(ctx, spotKeys, appID)
}

// GetMulticastConfig returns the multicast config.
func (s *DouyinService) GetMulticastConfig(ctx context.Context) (map[string]any, error) {
	return s.Client().GetMulticastConfig(ctx)
}

// PageTurnOffline reports a page-turn breadcrumb.
func (s *DouyinService) PageTurnOffline(ctx context.Context) (map[string]any, error) {
	return s.Client().PageTurnOffline(ctx)
}

// GetEmojiList returns the emoji list.
func (s *DouyinService) GetEmojiList(ctx context.Context, needAll string) (map[string]any, error) {
	return s.Client().GetEmojiList(ctx, needAll)
}

// GetPublishHighlight returns the publish highlight config.
func (s *DouyinService) GetPublishHighlight(ctx context.Context, highlightType string) (map[string]any, error) {
	return s.Client().GetPublishHighlight(ctx, highlightType)
}

// GetMixListCollection returns the mix collection.
func (s *DouyinService) GetMixListCollection(ctx context.Context, cursor, count string) (map[string]any, error) {
	return s.Client().GetMixListCollection(ctx, cursor, count)
}

// GetSEOInnerLink returns the SEO inner-link block.
func (s *DouyinService) GetSEOInnerLink(ctx context.Context) (map[string]any, error) {
	return s.Client().GetSEOInnerLink(ctx)
}

// GetStudyNotes returns the study AI-assistant notes.
func (s *DouyinService) GetStudyNotes(ctx context.Context, offset, count, filterDraft string) (map[string]any, error) {
	return s.Client().GetStudyNotes(ctx, offset, count, filterDraft)
}
