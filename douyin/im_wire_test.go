package douyin

import (
	"os"
	"testing"
)

// --- wire-format regression tests (no network) -----------------------------

// The cmd-specific body must be wrapped in the body-type field; getting this
// wrong produced "unsupported wire type"/HTTP 400 against the live API.
func TestIMEnvelopeWrapping(t *testing.T) {
	pin := true
	mute := false

	cases := []struct {
		name      string
		envelope  []byte
		cmd       int64
		bodyField int
		want      map[int]int64
	}{
		{
			name: "init",
			envelope: imEnvelope(2043, 1, func() *pbw {
				w := &pbw{}
				w.Msg(2043, func() *pbw { b := &pbw{}; b.IntAlways(2, 0); return b }())
				return w
			}()),
			cmd:       2043,
			bodyField: 2043,
			want:      map[int]int64{2: 0},
		},
		{
			name:      "set_setting_info",
			envelope:  imEnvelope(921, 1, func() *pbw { w := &pbw{}; w.Msg(921, imSettingBody("123", 123, 2, &pin, nil)); return w }()),
			cmd:       921,
			bodyField: 921,
			want:      map[int]int64{2: 123, 3: 2, 4: 1},
		},
		{
			name:      "leave_conversation",
			envelope:  imEnvelope(652, 1, func() *pbw { w := &pbw{}; w.Msg(652, imConversationRefBody("123", 123, 2)); return w }()),
			cmd:       652,
			bodyField: 652,
			want:      map[int]int64{2: 123, 3: 2},
		},
		{
			name:      "mute_only_writes_field_5",
			envelope:  imEnvelope(921, 1, func() *pbw { w := &pbw{}; w.Msg(921, imSettingBody("123", 123, 2, nil, &mute)); return w }()),
			cmd:       921,
			bodyField: 921,
			want:      map[int]int64{2: 123, 3: 2, 5: 0},
		},
	}

	for _, tc := range cases {
		env, err := pbParse(tc.envelope)
		if err != nil {
			t.Fatalf("%s: envelope parse: %v", tc.name, err)
		}
		if got := pbInt(env, 1); got != tc.cmd {
			t.Errorf("%s: cmd=%d want %d", tc.name, got, tc.cmd)
		}
		// The build number is 24 chars; the trailing "B" in older notes was the
		// next field's tag byte (0x42).
		if got := pbString(env, 7); got != "0d50935:feat/pc-im-group" {
			t.Errorf("%s: build_number=%q", tc.name, got)
		}
		if got := len(pbList(env, 15)); got != 17 {
			t.Errorf("%s: header entries=%d want 17", tc.name, got)
		}
		body, ok := pbMsg(env, 8)
		if !ok {
			t.Fatalf("%s: no body", tc.name)
		}
		inner, ok := pbMsg(body, tc.bodyField)
		if !ok {
			t.Fatalf("%s: body not wrapped in field %d (fields: %v)", tc.name, tc.bodyField, body)
		}
		for field, want := range tc.want {
			if got := pbInt(inner, field); got != want {
				t.Errorf("%s: body field %d = %d want %d", tc.name, field, got, want)
			}
		}
	}
}

// --- live integration tests (opt-in: DOUYIN_LIVE_TEST=1) -------------------

func imLiveClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run live IM tests")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLiveIMConversationsAndHistory(t *testing.T) {
	c := imLiveClient(t)
	ctx := t.Context()
	convs, err := c.IMListConversations(ctx)
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if len(convs) == 0 {
		t.Fatal("no conversations returned")
	}
	named := 0
	for _, conv := range convs {
		if conv.Name != "" {
			named++
		}
	}
	if named == 0 {
		t.Fatal("no conversation carried a display name")
	}
	t.Logf("%d conversations, %d named (first: %s / %s)", len(convs), named, convs[0].ConversationID, convs[0].Name)

	for _, conv := range convs {
		if conv.ConversationType != 2 || conv.Name == "" {
			continue
		}
		msgs, _, err := c.IMConversationHistory(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, 0, 5)
		if err != nil {
			t.Fatalf("history of %s: %v", conv.Name, err)
		}
		t.Logf("group %q: %d messages, newest=%q", conv.Name, len(msgs), firstText(msgs))
		break
	}
}

func TestLiveIMParticipants(t *testing.T) {
	c := imLiveClient(t)
	ctx := t.Context()
	convs, err := c.IMListConversations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, conv := range convs {
		if conv.ConversationType != 2 {
			continue
		}
		members, err := c.IMParticipants(ctx, conv.ConversationID, conv.ConversationShortID, conv.ConversationType, 0, 20)
		if err != nil {
			t.Fatalf("participants of %s: %v", conv.Name, err)
		}
		if len(members) == 0 {
			t.Fatalf("group %s returned no participants", conv.Name)
		}
		t.Logf("group %q: %d participants (first uid=%d nick=%q)", conv.Name, len(members), members[0].UID, members[0].Nickname)
		return
	}
	t.Skip("no group conversation")
}

func firstText(msgs []IMChatMessage) string {
	for _, m := range msgs {
		if t := m.Text(); t != "" {
			return t
		}
	}
	return ""
}
