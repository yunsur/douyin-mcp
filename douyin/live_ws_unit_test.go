package douyin

// Unit coverage for the live WebSocket layer's pure logic: handshake query
// construction, PushFrame/gzip decoding, per-kind event normalisation, the
// bounded (method, msgId) replay cache and the reconnect backoff curve.
//
// These tests are hermetic: they feed canned protobuf bytes straight into
// handleFrame/dispatch and never dial. Helpers shared with live_ws_test.go
// (liveUnhex, liveTMap, liveTArr, liveTestBrowserVersion, fxExpectedQuery,
// fxPKStart, fxBattle) are reused rather than redefined.

import (
	"bytes"
	"compress/gzip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func liveUnitSet(t *testing.T, msg *dynamicpb.Message, name string, value any) {
	t.Helper()
	if err := SetProtoField(msg, name, value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

func liveUnitUser(t *testing.T, secUID, nickname string) *dynamicpb.Message {
	t.Helper()
	user, err := ProtoNew("User")
	if err != nil {
		t.Fatal(err)
	}
	liveUnitSet(t, user, "sec_uid", secUID)
	liveUnitSet(t, user, "nickname", nickname)
	return user
}

// liveUnitSetMessage wires a sub-message field (SetProtoField only handles
// scalar/list values, so nested messages go through the reflect API).
func liveUnitSetMessage(t *testing.T, parent *dynamicpb.Message, name string, child *dynamicpb.Message) {
	t.Helper()
	fd := parent.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		t.Fatalf("no field %s on %s", name, parent.Descriptor().FullName())
	}
	parent.Set(fd, protoreflect.ValueOfMessage(child))
}

func liveUnitNew(t *testing.T, name string) *dynamicpb.Message {
	t.Helper()
	msg, err := ProtoNew(name)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func liveUnitMarshal(t *testing.T, msg *dynamicpb.Message) []byte {
	t.Helper()
	raw, err := ProtoMarshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// liveUnitItem builds one Webcast Message list item.
func liveUnitItem(t *testing.T, method string, msgID int64, payload []byte) *dynamicpb.Message {
	t.Helper()
	item := liveUnitNew(t, "Message")
	liveUnitSet(t, item, "method", method)
	liveUnitSet(t, item, "payload", payload)
	liveUnitSet(t, item, "msgId", msgID)
	return item
}

// liveUnitResponse builds a LiveResponse carrying the given items.
func liveUnitResponse(t *testing.T, items ...*dynamicpb.Message) []byte {
	t.Helper()
	resp := liveUnitNew(t, "LiveResponse")
	list := resp.Mutable(resp.Descriptor().Fields().ByName("messagesList")).List()
	for _, item := range items {
		list.Append(protoreflect.ValueOfMessage(item))
	}
	return liveUnitMarshal(t, resp)
}

// liveUnitFrame wraps a payload in a PushFrame (payloadType empty = data frame).
func liveUnitFrame(t *testing.T, payload []byte, payloadType string) []byte {
	t.Helper()
	frame := liveUnitNew(t, "PushFrame")
	if payloadType != "" {
		liveUnitSet(t, frame, "payloadType", payloadType)
	}
	liveUnitSet(t, frame, "payload", payload)
	return liveUnitMarshal(t, frame)
}

func liveUnitGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, []byte{0x1f, 0x8b}) {
		t.Fatalf("gzip output has no magic prefix: %x", out[:2])
	}
	return out
}

// liveUnitDrain returns every buffered event without blocking.
func liveUnitDrain(l *LiveListener) []LiveEvent {
	events := []LiveEvent{}
	for {
		select {
		case event := <-l.events:
			events = append(events, event)
		default:
			return events
		}
	}
}

func liveUnitKinds(events []LiveEvent) []string {
	kinds := make([]string, len(events))
	for i, event := range events {
		kinds[i] = event.Kind
	}
	return kinds
}

// The handshake query is the browser wire order verbatim; only the random
// signature may differ, and it must be present exactly once and non-empty.
func TestLiveHandshakeQueryStablePrefix(t *testing.T) {
	client, err := NewClient("ttwid=fake", Options{})
	if err != nil {
		t.Fatal(err)
	}
	listener := NewLiveListener(client, "7654321098765432100")

	query := listener.wsQuery("CUR1", "EXT1", "7654321098765432100", "1234567890")

	if got := strings.Count(query, "&signature="); got != 1 {
		t.Fatalf("signature param appears %d times in %q", got, query)
	}
	parts := strings.SplitN(query, "&signature=", 2)
	if len(parts) != 2 {
		t.Fatalf("query has no signature: %q", query)
	}
	want := strings.Replace(fxExpectedQuery, "__UA__", url.QueryEscape(liveTestBrowserVersion()), 1)
	wantParts := strings.SplitN(want, "&signature=", 2)
	if parts[0] != wantParts[0] {
		t.Fatalf("query prefix mismatch:\n got %s\nwant %s", parts[0], wantParts[0])
	}
	if parts[1] == "" {
		t.Fatal("signature is empty")
	}

	// Everything except the signature is deterministic across calls.
	again := listener.wsQuery("CUR1", "EXT1", "7654321098765432100", "1234567890")
	if gotAgain := strings.SplitN(again, "&signature=", 2)[0]; gotAgain != parts[0] {
		t.Fatalf("prefix changed between calls:\n%s\n%s", parts[0], gotAgain)
	}

	// Room-specific values must flow into the query unchanged.
	other := listener.wsQuery("CUR2", "", "42", "99")
	if !strings.Contains(other, "cursor=CUR2") || !strings.Contains(other, "room_id=42") ||
		!strings.Contains(other, "user_unique_id=99") || !strings.Contains(other, "internal_ext=&") {
		t.Fatalf("cursor/room/user values not reflected: %q", other)
	}
}

// A plain PushFrame and its gzip-wrapped twin must decode to the same events,
// and framing metadata (hb/ack/empty payload) must be skipped silently.
func TestLiveFrameDecodePlainGzipAndSkips(t *testing.T) {
	chat := liveUnitNew(t, "ChatMessage")
	liveUnitSetMessage(t, chat, "user", liveUnitUser(t, "SEC-CHAT", "Tester"))
	liveUnitSet(t, chat, "content", "hello")

	gift := liveUnitNew(t, "GiftMessage")
	liveUnitSet(t, gift, "comboCount", 3)
	liveUnitSetMessage(t, gift, "toUser", liveUnitUser(t, "SEC-RECV", "Receiver"))
	giftStruct := liveUnitNew(t, "GiftStruct")
	liveUnitSet(t, giftStruct, "name", "Rose")
	liveUnitSetMessage(t, gift, "gift", giftStruct)

	response := liveUnitResponse(t,
		liveUnitItem(t, "WebcastChatMessage", 1, liveUnitMarshal(t, chat)),
		liveUnitItem(t, "WebcastGiftMessage", 2, liveUnitMarshal(t, gift)),
	)

	plain := liveUnitFrame(t, response, "")
	plainListener := NewLiveListener(nil, "room")
	plainListener.handleFrame(nil, plain)
	got := liveUnitDrain(plainListener)
	if kinds := liveUnitKinds(got); strings.Join(kinds, ",") != "chat,gift" {
		t.Fatalf("plain frame kinds = %v", kinds)
	}
	if got[0].Data["content"] != "hello" {
		t.Fatalf("chat content: %v", got[0].Data)
	}
	if got[1].Data["comboCount"] != "3" {
		t.Fatalf("gift comboCount: %v", got[1].Data)
	}

	gzipListener := NewLiveListener(nil, "room")
	gzipListener.handleFrame(nil, liveUnitFrame(t, liveUnitGzip(t, response), ""))
	gotGzip := liveUnitDrain(gzipListener)
	if kinds := liveUnitKinds(gotGzip); strings.Join(kinds, ",") != "chat,gift" {
		t.Fatalf("gzip frame kinds = %v", kinds)
	}

	// hb/ack frames and empty payloads carry no messages.
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"hb", liveUnitFrame(t, []byte("ignored"), "hb")},
		{"ack", liveUnitFrame(t, []byte("ignored"), "ack")},
		{"empty-payload", liveUnitFrame(t, nil, "")},
	} {
		l := NewLiveListener(nil, "room")
		l.handleFrame(nil, tc.raw)
		if events := liveUnitDrain(l); len(events) != 0 {
			t.Fatalf("%s frame emitted %d events", tc.name, len(events))
		}
	}
}

// Malformed frames and payloads are dropped, never panicking, and a bad item
// does not discard the valid items around it.
func TestLiveFrameMalformedDropsWithoutPanic(t *testing.T) {
	truncated := liveUnitFrame(t, liveUnitResponse(t, liveUnitItem(t, "WebcastChatMessage", 1, nil)), "")
	truncated = truncated[:len(truncated)/2]

	validChat := liveUnitNew(t, "ChatMessage")
	liveUnitSetMessage(t, validChat, "user", liveUnitUser(t, "SEC-CHAT", "Tester"))
	liveUnitSet(t, validChat, "content", "survivor")
	mixed := liveUnitResponse(t,
		liveUnitItem(t, "WebcastChatMessage", 1, liveUnitMarshal(t, validChat)),
		liveUnitItem(t, "WebcastChatMessage", 2, []byte{0xff, 0xfe}), // undecodable payload
		liveUnitItem(t, "WebcastUnknownMethod", 3, []byte{0x01}),     // unknown kind
		liveUnitItem(t, "WebcastChatMessage", 4, liveUnitMarshal(t, validChat)),
	)

	cases := []struct {
		name string
		raw  []byte
	}{
		{"not-protobuf", []byte("definitely not a PushFrame")},
		{"truncated-frame", truncated},
		{"garbage-payload", liveUnitFrame(t, []byte{0xff, 0xfe, 0xfd}, "")},
		{"truncated-gzip", liveUnitFrame(t, []byte{0x1f, 0x8b, 0x08, 0x00}, "")},
		{"gzip-of-garbage", liveUnitFrame(t, liveUnitGzip(t, []byte{0xff, 0xff}), "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := NewLiveListener(nil, "room")
			l.handleFrame(nil, tc.raw) // must not panic
			if events := liveUnitDrain(l); len(events) != 0 {
				t.Fatalf("malformed frame emitted %d events", len(events))
			}
		})
	}

	l := NewLiveListener(nil, "room")
	l.handleFrame(nil, liveUnitFrame(t, mixed, ""))
	got := liveUnitDrain(l)
	if kinds := liveUnitKinds(got); strings.Join(kinds, ",") != "chat,chat" {
		t.Fatalf("mixed frame kinds = %v", kinds)
	}
	if got[0].Data["content"] != "survivor" || got[1].Data["content"] != "survivor" {
		t.Fatalf("valid items around a bad one were lost: %v", got)
	}
	if got[0].Data["user"] == nil || got[1].Data["user"] == nil {
		t.Fatalf("surviving chat items lost their user: %v", got)
	}
}

// Every documented LiveEvent.Kind maps its canned protobuf payload to the
// proto field names (ProtoToMap keeps the declared names verbatim).
func TestLiveEventKindsNormalizeFields(t *testing.T) {
	l := NewLiveListener(nil, "room")

	chat := liveUnitNew(t, "ChatMessage")
	liveUnitSetMessage(t, chat, "user", liveUnitUser(t, "SEC-CHAT", "Tester"))
	liveUnitSet(t, chat, "content", "hello")

	gift := liveUnitNew(t, "GiftMessage")
	liveUnitSet(t, gift, "comboCount", 3)
	liveUnitSetMessage(t, gift, "user", liveUnitUser(t, "SEC-SEND", "Sender"))
	liveUnitSetMessage(t, gift, "toUser", liveUnitUser(t, "SEC-RECV", "Receiver"))
	giftStruct := liveUnitNew(t, "GiftStruct")
	liveUnitSet(t, giftStruct, "name", "Rose")
	liveUnitSetMessage(t, gift, "gift", giftStruct)

	member := liveUnitNew(t, "MemberMessage")
	liveUnitSetMessage(t, member, "user", liveUnitUser(t, "SEC-JOIN", "Joiner"))
	liveUnitSet(t, member, "memberCount", 42)

	like := liveUnitNew(t, "LikeMessage")
	liveUnitSet(t, like, "count", 5)
	liveUnitSet(t, like, "total", 100)
	liveUnitSetMessage(t, like, "user", liveUnitUser(t, "SEC-LIKE", "Liker"))

	social := liveUnitNew(t, "SocialMessage")
	liveUnitSetMessage(t, social, "user", liveUnitUser(t, "SEC-FOLLOW", "Follower"))
	liveUnitSet(t, social, "action", 7)
	liveUnitSet(t, social, "shareType", 3)

	stats := liveUnitNew(t, "RoomStatsMessage")
	liveUnitSet(t, stats, "displayLong", "1234人在线")

	items := []*dynamicpb.Message{
		liveUnitItem(t, "WebcastChatMessage", 1, liveUnitMarshal(t, chat)),
		liveUnitItem(t, "WebcastGiftMessage", 2, liveUnitMarshal(t, gift)),
		liveUnitItem(t, "WebcastMemberMessage", 3, liveUnitMarshal(t, member)),
		liveUnitItem(t, "WebcastLikeMessage", 4, liveUnitMarshal(t, like)),
		liveUnitItem(t, "WebcastSocialMessage", 5, liveUnitMarshal(t, social)),
		liveUnitItem(t, "WebcastRoomStatsMessage", 6, liveUnitMarshal(t, stats)),
		liveUnitItem(t, "WebcastLinkMicBattleMethod", 7, liveUnhex(t, fxPKStart)),
		liveUnitItem(t, "WebcastUnsupportedMessage", 8, []byte{0x01}), // unknown -> dropped
	}
	for _, item := range items {
		l.dispatch(item)
	}

	got := liveUnitDrain(l)
	want := "chat,gift,member,like,social,room_stats,pk"
	if kinds := liveUnitKinds(got); strings.Join(kinds, ",") != want {
		t.Fatalf("kinds = %v, want %s", kinds, want)
	}

	chatData := got[0].Data
	if chatData["content"] != "hello" {
		t.Fatalf("chat content: %v", chatData)
	}
	chatUser := liveTMap(t, chatData["user"])
	if chatUser["sec_uid"] != "SEC-CHAT" || chatUser["nickname"] != "Tester" {
		t.Fatalf("chat user: %v", chatUser)
	}

	giftData := got[1].Data
	if giftData["comboCount"] != "3" {
		t.Fatalf("gift comboCount: %v", giftData)
	}
	toUser := liveTMap(t, giftData["toUser"])
	if toUser["sec_uid"] != "SEC-RECV" || toUser["nickname"] != "Receiver" {
		t.Fatalf("gift toUser: %v", toUser)
	}
	if liveTMap(t, giftData["gift"])["name"] != "Rose" {
		t.Fatalf("gift name: %v", giftData["gift"])
	}

	memberData := got[2].Data
	if memberData["memberCount"] != "42" {
		t.Fatalf("member count: %v", memberData)
	}
	if liveTMap(t, memberData["user"])["sec_uid"] != "SEC-JOIN" {
		t.Fatalf("member user: %v", memberData["user"])
	}

	likeData := got[3].Data
	if likeData["count"] != "5" || likeData["total"] != "100" {
		t.Fatalf("like counts: %v", likeData)
	}
	if liveTMap(t, likeData["user"])["sec_uid"] != "SEC-LIKE" {
		t.Fatalf("like user: %v", likeData["user"])
	}

	socialData := got[4].Data
	if socialData["action"] != "7" || socialData["shareType"] != "3" {
		t.Fatalf("social fields: %v", socialData)
	}
	if liveTMap(t, socialData["user"])["sec_uid"] != "SEC-FOLLOW" {
		t.Fatalf("social user: %v", socialData["user"])
	}

	statsData := got[5].Data
	if statsData["displayLong"] != "1234人在线" {
		t.Fatalf("room stats displayLong: %v", statsData)
	}

	pkData := got[6].Data
	if pkData["type"] != "start" || pkData["battle_id"] != fxBattle {
		t.Fatalf("pk event: %v", pkData)
	}
}

// The dedup cache is an exact LRU-of-one-out: at capacity the oldest id still
// dedups, inserting one more evicts precisely that oldest id, and (method,id)
// is namespaced per method with msgId=0 never tracked.
func TestLiveDedupBoundaryEvictsOldestOnly(t *testing.T) {
	l := NewLiveListener(nil, "room")

	for i := int64(1); i <= liveDedupLimit; i++ {
		if l.duplicate("m", i) {
			t.Fatalf("fresh id %d reported duplicate", i)
		}
	}
	if len(l.seen) != liveDedupLimit || len(l.seenQ) != liveDedupLimit {
		t.Fatalf("cache sizes: map=%d queue=%d limit=%d", len(l.seen), len(l.seenQ), liveDedupLimit)
	}
	// At exactly the limit nothing has been evicted yet.
	if !l.duplicate("m", 1) {
		t.Fatal("oldest id evicted before the cache reached its bound")
	}
	if !l.duplicate("m", liveDedupLimit) {
		t.Fatal("newest id not retained at capacity")
	}

	// One insertion past the bound evicts exactly the oldest id.
	over := int64(liveDedupLimit + 1)
	overKey := "m:" + strconv.FormatInt(over, 10)
	if l.duplicate("m", over) {
		t.Fatalf("fresh id %d reported duplicate", over)
	}
	if len(l.seen) != liveDedupLimit {
		t.Fatalf("cache grew past limit: %d", len(l.seen))
	}
	if _, ok := l.seen["m:1"]; ok {
		t.Fatal("oldest id still cached after crossing the bound")
	}
	if _, ok := l.seen["m:2"]; !ok {
		t.Fatal("second-oldest id wrongly evicted")
	}
	if _, ok := l.seen[overKey]; !ok {
		t.Fatal("newly inserted id not cached")
	}

	// A replayed evicted id is emitted again (observable dedup miss) and in
	// turn pushes the next-oldest id out.
	if l.duplicate("m", 1) {
		t.Fatal("evicted oldest id must not deduplicate")
	}
	if _, ok := l.seen["m:1"]; !ok {
		t.Fatal("replayed id should have been re-cached")
	}
	if _, ok := l.seen["m:2"]; ok {
		t.Fatal("re-insert should have evicted the next-oldest id")
	}
	if !l.duplicate("m", over) {
		t.Fatal("newly inserted id must remain cached")
	}

	// Keys are namespaced by method.
	if l.duplicate("n", 2) {
		t.Fatal("same id under a different method must not deduplicate")
	}
	// msgId 0 is never deduplicated.
	if l.duplicate("m", 0) || l.duplicate("m", 0) {
		t.Fatal("msgId=0 must never be reported as duplicate")
	}
}

// Backoff follows an exact attempt-indexed curve and saturates at the cap.
func TestLiveBackoffExactSequenceAndCap(t *testing.T) {
	want := []time.Duration{
		time.Second,     // attempt 0
		time.Second,     // 1
		2 * time.Second, // 2
		4 * time.Second, // 3
		8 * time.Second, // 4
		16 * time.Second,
		liveMaxBackoff, // 32s would exceed the 30s cap
		liveMaxBackoff,
		liveMaxBackoff,
	}
	for attempt, expected := range want {
		if got := liveBackoff(attempt); got != expected {
			t.Fatalf("liveBackoff(%d) = %v, want %v", attempt, got, expected)
		}
	}
	if got := liveBackoff(1000); got != liveMaxBackoff {
		t.Fatalf("liveBackoff(1000) = %v, want cap %v", got, liveMaxBackoff)
	}
	// Never shrinks and never exceeds the cap.
	prev := time.Duration(0)
	for attempt := 0; attempt <= 40; attempt++ {
		got := liveBackoff(attempt)
		if got < prev || got > liveMaxBackoff {
			t.Fatalf("liveBackoff(%d) = %v (prev %v, cap %v)", attempt, got, prev, liveMaxBackoff)
		}
		prev = got
	}
}
