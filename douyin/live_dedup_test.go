package douyin

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

// Regression for upstream issue #88: Douyin re-pushes buffered messages around
// reconnects, so the listener must drop repeated (method, msgId) pairs instead
// of emitting the same gift/chat event twice.
func TestLiveDispatchDeduplicatesRepeatedMsgID(t *testing.T) {
	l := NewLiveListener(nil, "room")

	gift, err := ProtoNew("GiftMessage")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := ProtoMarshal(gift)
	if err != nil {
		t.Fatal(err)
	}

	newItem := func(msgID int64) proto.Message {
		item, err := ProtoNew("Message")
		if err != nil {
			t.Fatal(err)
		}
		if err := SetProtoField(item, "method", "WebcastGiftMessage"); err != nil {
			t.Fatal(err)
		}
		if err := SetProtoField(item, "payload", payload); err != nil {
			t.Fatal(err)
		}
		if err := SetProtoField(item, "msgId", msgID); err != nil {
			t.Fatal(err)
		}
		return item
	}

	l.dispatch(newItem(12345))
	l.dispatch(newItem(12345)) // replay of the same push
	if got := len(l.events); got != 1 {
		t.Fatalf("duplicate msgId emitted %d events, want 1", got)
	}

	l.dispatch(newItem(12346)) // a new message must pass
	if got := len(l.events); got != 2 {
		t.Fatalf("new msgId emitted %d events, want 2", got)
	}

	// Messages without a msgId are never deduplicated.
	l.dispatch(newItem(0))
	l.dispatch(newItem(0))
	if got := len(l.events); got != 4 {
		t.Fatalf("msgId=0 emitted %d events, want 4", got)
	}
}

func TestLiveDedupCacheIsBounded(t *testing.T) {
	l := NewLiveListener(nil, "room")
	for i := int64(1); i <= liveDedupLimit+50; i++ {
		if l.duplicate("WebcastChatMessage", i) {
			t.Fatalf("fresh msgId %d reported as duplicate", i)
		}
	}
	if len(l.seen) > liveDedupLimit {
		t.Fatalf("dedup cache grew to %d entries, limit %d", len(l.seen), liveDedupLimit)
	}
	if len(l.seenQ) != len(l.seen) {
		t.Fatalf("dedup queue %d != map %d", len(l.seenQ), len(l.seen))
	}
	// An evicted id is no longer deduplicated (bounded memory, not correctness).
	if l.duplicate("WebcastChatMessage", 1) {
		t.Fatal("evicted msgId should not be reported as duplicate")
	}
}

// Backoff must grow exponentially and cap (upstream PR #78 intent) so a
// flapping room cannot produce a permanent 1s reconnect storm.
func TestLiveBackoffGrowsAndCaps(t *testing.T) {
	if got := liveBackoff(0); got != time.Second {
		t.Fatalf("liveBackoff(0) = %v, want 1s", got)
	}
	prev := time.Duration(0)
	for attempt := 1; attempt <= 6; attempt++ {
		got := liveBackoff(attempt)
		if got < prev {
			t.Fatalf("liveBackoff(%d) = %v shrank from %v", attempt, got, prev)
		}
		prev = got
	}
	if got := liveBackoff(20); got != liveMaxBackoff {
		t.Fatalf("liveBackoff(20) = %v, want cap %v", got, liveMaxBackoff)
	}
}
