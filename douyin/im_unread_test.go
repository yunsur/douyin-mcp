package douyin

import "testing"

// The conversation core carries this account's unread counters as a repeated
// {class, count} list (field 11). Classes 1+2 are the ordinary unread messages
// the web UI renders as the conversation badge (verified live: BOBING group
// classes {1:4, 2:24} == badge 28).
func TestIMConversationUnreadClasses(t *testing.T) {
	w := &pbw{}
	w.Str(1, "7677204082955682353")
	w.IntAlways(3, 2)
	meta := &pbw{}
	meta.Str(5, "测试群")
	w.Msg(50, meta)
	for _, pair := range []struct{ class, count int64 }{{1, 4}, {2, 24}, {3, 16}} {
		p := &pbw{}
		p.IntAlways(1, pair.class)
		p.IntAlways(2, pair.count)
		w.Msg(11, p)
	}

	core, err := pbParse(w.b)
	if err != nil {
		t.Fatalf("parse core: %v", err)
	}
	conv := imParseConversationCore(core)
	if conv.Name != "测试群" {
		t.Fatalf("name = %q", conv.Name)
	}
	if conv.Unread != 28 {
		t.Fatalf("unread = %d, want 28 (classes 1+2)", conv.Unread)
	}
	want := map[int64]int64{1: 4, 2: 24, 3: 16}
	if len(conv.UnreadClasses) != len(want) {
		t.Fatalf("classes = %v", conv.UnreadClasses)
	}
	for class, count := range want {
		if conv.UnreadClasses[class] != count {
			t.Fatalf("class %d = %d, want %d", class, conv.UnreadClasses[class], count)
		}
	}
}

func TestIMConversationNoUnread(t *testing.T) {
	w := &pbw{}
	w.Str(1, "0:1:1:2")
	w.IntAlways(3, 1)
	core, err := pbParse(w.b)
	if err != nil {
		t.Fatalf("parse core: %v", err)
	}
	conv := imParseConversationCore(core)
	if conv.Unread != 0 || len(conv.UnreadClasses) != 0 {
		t.Fatalf("unexpected unread: %d %v", conv.Unread, conv.UnreadClasses)
	}
}
