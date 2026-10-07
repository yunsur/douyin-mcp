package main

import (
	"testing"

	"github.com/yunsur/douyin-mcp/douyin"
)

// The IM list endpoint only returns conversations changed since the last sync,
// so the service must merge deltas with its accumulated cache: fresh entries
// win and cached-only entries come back flagged.
func TestMergeConversationsKeepsDeltaAndCache(t *testing.T) {
	s := &DouyinService{convCache: map[string]douyin.IMConversation{}}

	fresh := []douyin.IMConversation{
		{ConversationID: "a", Name: "A", Unread: 3},
		{ConversationID: "b", Name: "B", Unread: 10},
	}
	if got := s.mergeConversations(fresh); len(got) != 2 {
		t.Fatalf("first merge = %d conversations, want 2", len(got))
	}

	// Next delta only carries "a": "b" must survive from the cache.
	merged := s.mergeConversations([]douyin.IMConversation{{ConversationID: "a", Name: "A2", Unread: 0}})
	if len(merged) != 2 {
		t.Fatalf("second merge = %d conversations, want 2", len(merged))
	}
	byID := map[string]douyin.IMConversation{}
	for _, c := range merged {
		byID[c.ConversationID] = c
	}
	if a := byID["a"]; a.Cached || a.Name != "A2" || a.Unread != 0 {
		t.Fatalf("fresh entry not refreshed: %+v", a)
	}
	if b := byID["b"]; !b.Cached || b.Name != "B" || b.Unread != 10 {
		t.Fatalf("cached entry lost: %+v", b)
	}
}
