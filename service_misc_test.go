package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunsur/douyin-mcp/douyin"
)

// --- imConvID / imConvType coercion ---------------------------------------

// The IM event buffer is decoded from JSON, so the same logical id/type can be
// a string, a json.Number, or a Go numeric depending on the code path that
// filled it. Both helpers must normalise all of those and fall back to ""/0.
func TestServiceMiscIMConvCoercion(t *testing.T) {
	idCases := []struct {
		name string
		in   any
		want string
	}{
		{"string", "abc", "abc"},
		{"json.Number integer", json.Number("42"), "42"},
		{"json.Number huge", json.Number("9007199254740993"), "9007199254740993"},
		{"int64", int64(8), "8"},
		{"int", int(7), "7"},
		{"float64 truncates", float64(3.9), "3"},
		{"nil", nil, ""},
		{"bool wrong type", true, ""},
		{"slice wrong type", []any{1}, ""},
	}
	for _, tc := range idCases {
		t.Run("id/"+tc.name, func(t *testing.T) {
			if got := imConvID(tc.in); got != tc.want {
				t.Fatalf("imConvID(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	typeCases := []struct {
		name string
		in   any
		want int64
	}{
		{"int", int(2), 2},
		{"int32", int32(3), 3},
		{"int64", int64(4), 4},
		{"float64 truncates", float64(5.9), 5},
		{"json.Number integer", json.Number("6"), 6},
		{"json.Number non-integer", json.Number("x"), 0},
		{"string wrong type", "2", 0},
		{"nil", nil, 0},
		{"bool wrong type", true, 0},
	}
	for _, tc := range typeCases {
		t.Run("type/"+tc.name, func(t *testing.T) {
			if got := imConvType(tc.in); got != tc.want {
				t.Fatalf("imConvType(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// --- conversation cache ----------------------------------------------------

func TestServiceMiscConvCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversations_cache.json")

	s := &DouyinService{convCache: map[string]douyin.IMConversation{}, convCachePath: path}
	s.mergeConversations([]douyin.IMConversation{
		{ConversationID: "a", ConversationShortID: 11, ConversationType: 2, Name: "A", Unread: 4,
			LastMessages: []douyin.IMChatMessage{{}}},
		{ConversationID: "b", Name: "B"},
	})

	restored := &DouyinService{convCache: map[string]douyin.IMConversation{}, convCachePath: path}
	restored.loadConversationCache()
	if len(restored.convCache) != 2 {
		t.Fatalf("restored cache = %d conversations, want 2", len(restored.convCache))
	}
	a := restored.convCache["a"]
	if a.ConversationShortID != 11 || a.ConversationType != 2 || a.Name != "A" || a.Unread != 4 {
		t.Fatalf("restored entry corrupted: %+v", a)
	}
	// Message bodies are deliberately not persisted (they are the bulk of the file).
	if len(a.LastMessages) != 0 {
		t.Fatalf("LastMessages must not be persisted, got %d", len(a.LastMessages))
	}
}

func TestServiceMiscConvCacheDeltaUnionPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversations_cache.json")

	// First sync carries "a"; the next delta only carries "b".
	first := &DouyinService{convCache: map[string]douyin.IMConversation{}, convCachePath: path}
	first.mergeConversations([]douyin.IMConversation{{ConversationID: "a", Name: "A", Unread: 9}})
	first.mergeConversations([]douyin.IMConversation{{ConversationID: "b", Name: "B"}})

	restored := &DouyinService{convCache: map[string]douyin.IMConversation{}, convCachePath: path}
	restored.loadConversationCache()
	if len(restored.convCache) != 2 {
		t.Fatalf("persisted union = %d conversations, want 2", len(restored.convCache))
	}
	if a := restored.convCache["a"]; a.Name != "A" || a.Unread != 9 {
		t.Fatalf("earlier conversation lost from persisted union: %+v", a)
	}
	if b := restored.convCache["b"]; b.Name != "B" {
		t.Fatalf("delta conversation missing from persisted union: %+v", b)
	}
}

func TestServiceMiscConvCacheToleratedWhenCorruptOrMissing(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"corrupt", corrupt},
		{"missing", filepath.Join(dir, "absent.json")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &DouyinService{convCache: map[string]douyin.IMConversation{}, convCachePath: tc.path}
			s.loadConversationCache() // must not panic
			if len(s.convCache) != 0 {
				t.Fatalf("cache = %d entries, want 0", len(s.convCache))
			}
		})
	}
}

func TestServiceMiscConvCacheNoPathIsNoop(t *testing.T) {
	s := &DouyinService{convCache: map[string]douyin.IMConversation{"x": {ConversationID: "x"}}}
	s.loadConversationCache() // empty path: read must be skipped harmlessly
	if len(s.convCache) != 1 {
		t.Fatalf("cache mutated by load with empty path: %d", len(s.convCache))
	}
	// Saving with no path must also be a silent no-op.
	s.saveConversationCacheLocked()
}

// --- live listener ---------------------------------------------------------

func TestServiceMiscLiveListenNoopAndEventsCopy(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})

	// Nothing running: not starting a socket is still a successful stop.
	res := svc.StopLiveListen()
	if res["success"] != true || !strings.Contains(res["message"].(string), "已停止") {
		t.Fatalf("StopLiveListen no-op = %#v", res)
	}

	svc.liveEvents = []map[string]any{{"kind": "chat"}}
	out := svc.LiveEvents()
	if len(out) != 1 || out[0]["kind"] != "chat" {
		t.Fatalf("LiveEvents = %#v", out)
	}
	// The returned slice is a copy: replacing an element must not touch state.
	out[0] = map[string]any{"kind": "mutated"}
	if got := svc.LiveEvents(); len(got) != 1 || got[0]["kind"] != "chat" {
		t.Fatalf("LiveEvents leaked internal slice: %#v", got)
	}
}

// A second StartLiveListen while one is running must fail before any socket is
// opened (no HTTP request, no dial).
func TestServiceMiscLiveListenAlreadyRunning(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{})
	svc.live = douyin.NewLiveListener(svc.Client(), "1")
	t.Cleanup(func() { svc.StopLiveListen() })

	before := len(st.requests())
	if _, err := svc.StartLiveListen(t.Context(), "1"); err == nil {
		t.Fatal("StartLiveListen while running: want error, got nil")
	} else if !strings.Contains(err.Error(), "已在运行") {
		t.Fatalf("unexpected error: %v", err)
	}
	if after := len(st.requests()); after != before {
		t.Fatalf("rejected start issued %d requests", after-before)
	}
}

// --- IM listener -----------------------------------------------------------

func TestServiceMiscIMListenNoop(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})

	res := svc.StopIMListen()
	if res["success"] != true || !strings.Contains(res["message"].(string), "已停止") {
		t.Fatalf("StopIMListen no-op = %#v", res)
	}
}

func TestServiceMiscIMMessagesCopyAndFilter(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	svc.imMessages = []map[string]any{
		{"conversation_id": json.Number("100"), "conversation_type": float64(2), "n": 1},
		{"conversation_id": "100", "conversation_type": int64(1), "n": 2},
		{"conversation_id": "200", "conversation_type": json.Number("2"), "n": 3},
	}

	if got := svc.IMMessages("", 0); len(got) != 3 {
		t.Fatalf("unfiltered = %d messages, want 3", len(got))
	}
	if got := svc.IMMessages("100", 0); len(got) != 2 {
		t.Fatalf("by id = %d messages, want 2", len(got))
	}
	// Type 2 matches both the float64 and the json.Number spellings.
	byType := svc.IMMessages("", 2)
	if len(byType) != 2 {
		t.Fatalf("by type = %d messages, want 2", len(byType))
	}
	if got := svc.IMMessages("100", 2); len(got) != 1 || imConvID(got[0]["conversation_id"]) != "100" {
		t.Fatalf("id+type filter = %#v", got)
	}

	// Copy semantics: replacing an element must not mutate the buffer.
	out := svc.IMMessages("", 0)
	out[0] = nil
	if got := svc.IMMessages("", 0); got[0] == nil {
		t.Fatal("IMMessages leaked its internal slice")
	}
}

// The receiver is built with an HTTP-stub-backed client and never started, so
// this exercises the guard without dialing the real frontier-im socket.
func TestServiceMiscIMListenAlreadyRunning(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{})
	receiver, err := douyin.NewIMReceiver(svc.Client())
	if err != nil {
		t.Fatalf("NewIMReceiver: %v", err)
	}
	svc.im = receiver
	svc.imStop = make(chan struct{})
	t.Cleanup(func() { svc.StopIMListen() })

	before := len(st.requests())
	if _, err := svc.StartIMListen(t.Context()); err == nil {
		t.Fatal("StartIMListen while running: want error, got nil")
	} else if !strings.Contains(err.Error(), "已在运行") {
		t.Fatalf("unexpected error: %v", err)
	}
	if after := len(st.requests()); after != before {
		t.Fatalf("rejected start issued %d requests", after-before)
	}
}

// --- session / cookies -----------------------------------------------------

func TestServiceMiscPersistCookies(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	svc.cookiePath = filepath.Join(t.TempDir(), "cookies.txt")

	if err := svc.persistCookies(); err != nil {
		t.Fatalf("persistCookies: %v", err)
	}
	raw, err := os.ReadFile(svc.cookiePath)
	if err != nil {
		t.Fatalf("read cookie file: %v", err)
	}
	want := svc.Client().CookieStr()
	if string(raw) != want {
		t.Fatalf("cookie file = %q, want the session cookie %q", raw, want)
	}
	if !strings.Contains(want, "sessionid=abc") || !strings.Contains(want, "UIFID=uif") {
		t.Fatalf("cookie string missing stub values: %q", want)
	}
}

func TestServiceMiscReloadSwapsClient(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	cfgPath := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cfgPath, []byte("sessionid=fromfile; UIFID=ff\n"), 0o600); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}
	svc.cookiePath = cfgPath

	old := svc.Client()
	if err := svc.Reload(""); err != nil {
		t.Fatalf("Reload from file: %v", err)
	}
	fileClient := svc.Client()
	if fileClient == old {
		t.Fatal("Reload did not swap the client")
	}
	if cs := fileClient.CookieStr(); !strings.Contains(cs, "sessionid=fromfile") {
		t.Fatalf("Reload did not read the file cookie: %q", cs)
	}

	// A non-empty argument overrides the on-disk file.
	if err := svc.Reload("sessionid=override"); err != nil {
		t.Fatalf("Reload override: %v", err)
	}
	if cs := svc.Client().CookieStr(); !strings.Contains(cs, "sessionid=override") || strings.Contains(cs, "fromfile") {
		t.Fatalf("Reload override cookie = %q", cs)
	}
}

func TestServiceMiscDeleteCookies(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	path := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(path, []byte("sessionid=abc; UIFID=uif"), 0o600); err != nil {
		t.Fatalf("seed cookie file: %v", err)
	}
	svc.cookiePath = path
	old := svc.Client()

	res, err := svc.DeleteCookies(t.Context())
	if err != nil {
		t.Fatalf("DeleteCookies: %v", err)
	}
	if res["cookie_path"] != path {
		t.Fatalf("cookie_path = %v, want %q", res["cookie_path"], path)
	}
	if msg, _ := res["message"].(string); !strings.Contains(msg, "已删除") {
		t.Fatalf("message = %q", msg)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cookie file still present (err=%v)", err)
	}

	reset := svc.Client()
	if reset == old {
		t.Fatal("DeleteCookies did not reset the client")
	}
	if reset.Cookie.Has("sessionid") {
		t.Fatal("reset client still carries the old sessionid cookie")
	}
}

// --- input validation before any I/O ---------------------------------------

// An empty room id used to start a listener that connects nowhere: no events,
// no error, and a "running" listener the caller cannot distinguish from a
// healthy one. It must be rejected up front, without opening anything.
func TestServiceMiscLiveListenRejectsEmptyRoomID(t *testing.T) {
	for _, webRID := range []string{"", "   ", "\t"} {
		svc, st := newTestServiceJSON(t, map[string]any{})

		res, err := svc.StartLiveListen(t.Context(), webRID)
		if err == nil {
			t.Fatalf("StartLiveListen(%q): want error, got %#v", webRID, res)
		}
		if !strings.Contains(err.Error(), "web_rid") {
			t.Fatalf("StartLiveListen(%q) error = %v, want it to mention web_rid", webRID, err)
		}
		if got := len(st.requests()); got != 0 {
			t.Fatalf("StartLiveListen(%q) issued %d requests", webRID, got)
		}

		svc.liveMu.Lock()
		listener := svc.live
		svc.liveMu.Unlock()
		if listener != nil {
			t.Fatalf("StartLiveListen(%q) left a listener installed", webRID)
		}
	}
}

// An unsupported attachment kind must be rejected before the conversation is
// created: otherwise a typo opens an empty conversation with the recipient.
func TestServiceMiscSendDMMediaRejectsUnknownKind(t *testing.T) {
	for _, kind := range []string{"", "sticker", "Image", "text"} {
		svc, st := newTestServiceJSON(t, map[string]any{})

		res, err := svc.SendDMMedia(t.Context(), 123456, kind, "/tmp/whatever.jpg")
		if err == nil {
			t.Fatalf("SendDMMedia(kind=%q): want error, got %#v", kind, res)
		}
		if !strings.Contains(err.Error(), "不支持的私信媒体类型") {
			t.Fatalf("SendDMMedia(kind=%q) error = %v", kind, err)
		}
		for _, want := range douyin.IMMediaKinds {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("SendDMMedia(kind=%q) error %v should list %q", kind, err, want)
			}
		}
		if got := len(st.requests()); got != 0 {
			t.Fatalf("SendDMMedia(kind=%q) issued %d requests; validation must precede CreateConversation", kind, got)
		}
	}
}
