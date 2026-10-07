package main

// Thin service wrappers for the hot-search-video + music-page endpoints
// (热搜词视频 / 音乐页).

import (
	"context"
)

// GetHotSearchVideos returns the video list of one hot-search word.
func (s *DouyinService) GetHotSearchVideos(ctx context.Context, hotword, sentenceID, offset, count, entryName string) (map[string]any, error) {
	return s.Client().HotSearchVideos(ctx, hotword, sentenceID, offset, count, entryName)
}

// GetMusicAweme returns the videos under one music.
func (s *DouyinService) GetMusicAweme(ctx context.Context, musicID, cursor, count string) (map[string]any, error) {
	return s.Client().MusicAweme(ctx, musicID, cursor, count)
}

// GetMusicDetail returns the music detail block.
func (s *DouyinService) GetMusicDetail(ctx context.Context, musicID, scene string) (map[string]any, error) {
	return s.Client().MusicDetail(ctx, musicID, scene)
}

// GetMusicCollection returns the logged-in user's collected music.
func (s *DouyinService) GetMusicCollection(ctx context.Context, cursor, count string) (map[string]any, error) {
	return s.Client().MusicListCollection(ctx, cursor, count)
}

// CollectMusic collects or uncollects a music (write API).
func (s *DouyinService) CollectMusic(ctx context.Context, musicID, typ, action string) (map[string]any, error) {
	return s.Client().CollectMusic(ctx, musicID, typ, action)
}
