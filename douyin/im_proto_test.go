package douyin

// Deterministic, no-network tests for the PC IM protobuf envelope and the
// WebSocket frame decoder.

import (
	"fmt"
	randv2 "math/rand/v2"
	"regexp"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// RFC 6979 P-256 test private key (valid scalar), used only to exercise
// ECDSA request signing without any session material.
const testIMPrivateKeyHex = "c9afa9d845ba75166b5c215767b1d6934e50c3db36e89b127b8a622b120f6721"

// buildIMSendRequest mirrors the message/send body construction in
// imSendMessageRaw without touching the network.
func buildIMSendRequest(t *testing.T) (content string, clientMessageID string) {
	t.Helper()
	req, err := imNewNormalRequest(100)
	if err != nil {
		t.Fatalf("imNewNormalRequest: %v", err)
	}
	body, err := pbSub(req, "body")
	if err != nil {
		t.Fatalf("pbSub body: %v", err)
	}
	send, err := pbSub(body, "send_message_body")
	if err != nil {
		t.Fatalf("pbSub send_message_body: %v", err)
	}
	content = imEncodeContent(map[string]any{"text": "hi"})
	clientMessageID = imUUID4()
	for _, set := range []struct {
		name  string
		value any
	}{
		{"conversation_id", "0:1:1:2"},
		{"conversation_type", int64(1)},
		{"conversation_short_id", int64(99)},
		{"content", content},
		{"message_type", int64(imText)},
		{"ticket", "ticket-abc"},
		{"client_message_id", clientMessageID},
	} {
		if err := SetProtoField(send, set.name, set.value); err != nil {
			t.Fatalf("SetProtoField %s: %v", set.name, err)
		}
	}
	if err := pbAppendExt(send, "s:mentioned_users", ""); err != nil {
		t.Fatalf("append ext: %v", err)
	}
	if err := pbAppendExt(send, "s:client_message_id", clientMessageID); err != nil {
		t.Fatalf("append ext: %v", err)
	}
	stime := fmt.Sprintf("%d.%05d", GenerateMillisecond(), randv2.IntN(100000))
	if err := pbAppendExt(send, "s:stime", stime); err != nil {
		t.Fatalf("append ext: %v", err)
	}
	raw, err := ProtoMarshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	re, err := ProtoUnmarshal("Request", raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := ProtoToMap(re)
	if err != nil {
		t.Fatalf("ProtoToMap: %v", err)
	}

	if v := toInt64(got["cmd"]); v != 100 {
		t.Errorf("cmd = %d, want 100", v)
	}
	for key, want := range map[string]string{
		"sdk_version":     "0.1.8",
		"build_number":    "0d50935:feat/pc-im-group",
		"device_platform": "douyin_pc",
		"version_code":    "360000",
		"biz":             "douyin_web",
		"access":          "web_sdk",
	} {
		if v, _ := got[key].(string); v != want {
			t.Errorf("%s = %q, want %q", key, v, want)
		}
	}
	if v := toInt64(got["auth_type"]); v != 4 {
		t.Errorf("auth_type = %d, want 4", v)
	}

	bodyMap, _ := got["body"].(map[string]any)
	sendMap, _ := bodyMap["send_message_body"].(map[string]any)
	if sendMap == nil {
		t.Fatalf("body.send_message_body missing: %v", got)
	}
	if v, _ := sendMap["content"].(string); v != content {
		t.Errorf("content = %q, want %q", v, content)
	}
	if v, _ := sendMap["ticket"].(string); v != "ticket-abc" {
		t.Errorf("ticket = %q, want %q", v, "ticket-abc")
	}
	if v, _ := sendMap["client_message_id"].(string); v != clientMessageID {
		t.Errorf("client_message_id = %q, want %q", v, clientMessageID)
	}

	ext, _ := sendMap["ext"].([]any)
	if len(ext) != 3 {
		t.Fatalf("ext length = %d, want 3: %v", len(ext), ext)
	}
	wantKeys := []string{"s:mentioned_users", "s:client_message_id", "s:stime"}
	for i, want := range wantKeys {
		entry, _ := ext[i].(map[string]any)
		if entry == nil {
			t.Fatalf("ext[%d] not an object: %v", i, ext[i])
		}
		if k, _ := entry["key"].(string); k != want {
			t.Errorf("ext[%d].key = %q, want %q", i, k, want)
		}
	}
	stimeEntry, _ := ext[2].(map[string]any)
	stimeValue, _ := stimeEntry["value"].(string)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]{5}$`).MatchString(stimeValue) {
		t.Errorf("s:stime = %q, want <ms>.<5 digits>", stimeValue)
	}
	return content, clientMessageID
}

func TestIMSendEnvelopeShape(t *testing.T) {
	buildIMSendRequest(t)
}

func TestIMCreateConversationEnvelope(t *testing.T) {
	const toUserID, myUserID = int64(2222), int64(1111)

	signInput := fmt.Sprintf(
		`{"sign_data":"avatar_url=&idempotent_id=&name=&participants=%d,%d","certType":"cookie","scene":"web_protect"}`,
		toUserID, myUserID,
	)
	signature, err := GetReqSign(signInput, testIMPrivateKeyHex)
	if err != nil {
		t.Skipf("no usable private key material: %v", err)
	}
	if signature == "" {
		t.Skip("empty request signature")
	}

	req, err := imNewNormalRequest(609)
	if err != nil {
		t.Fatalf("imNewNormalRequest: %v", err)
	}
	body, err := pbSub(req, "body")
	if err != nil {
		t.Fatalf("pbSub body: %v", err)
	}
	conv, err := pbSub(body, "create_conversation_v2_body")
	if err != nil {
		t.Fatalf("pbSub create_conversation_v2_body: %v", err)
	}
	if err := SetProtoField(conv, "conversation_type", int64(1)); err != nil {
		t.Fatalf("conversation_type: %v", err)
	}
	partsFD := pbFD(conv, "participants")
	parts := conv.Mutable(partsFD).List()
	parts.Append(protoreflect.ValueOfInt64(toUserID))
	parts.Append(protoreflect.ValueOfInt64(myUserID))
	if err := SetProtoField(req, "reuqest_sign", signature); err != nil {
		t.Fatalf("reuqest_sign: %v", err)
	}

	raw, err := ProtoMarshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	re, err := ProtoUnmarshal("Request", raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := ProtoToMap(re)
	if err != nil {
		t.Fatalf("ProtoToMap: %v", err)
	}
	if v := toInt64(got["cmd"]); v != 609 {
		t.Errorf("cmd = %d, want 609", v)
	}
	if v, _ := got["reuqest_sign"].(string); v == "" {
		t.Errorf("reuqest_sign empty, want non-empty ECDSA signature")
	}
	bodyMap, _ := got["body"].(map[string]any)
	convMap, _ := bodyMap["create_conversation_v2_body"].(map[string]any)
	if convMap == nil {
		t.Fatalf("body.create_conversation_v2_body missing: %v", got)
	}
	if v := toInt64(convMap["conversation_type"]); v != 1 {
		t.Errorf("conversation_type = %d, want 1", v)
	}
	list, _ := convMap["participants"].([]any)
	if len(list) != 2 {
		t.Fatalf("participants = %v, want 2 entries", list)
	}
	if a, b := toInt64(list[0]), toInt64(list[1]); a != toUserID || b != myUserID {
		t.Errorf("participants = [%d %d], want [%d %d]", a, b, toUserID, myUserID)
	}
}

func TestIMFrameDecode(t *testing.T) {
	resp, err := ProtoNew("Response")
	if err != nil {
		t.Fatalf("ProtoNew Response: %v", err)
	}
	body, err := pbSub(resp, "body")
	if err != nil {
		t.Fatalf("pbSub body: %v", err)
	}
	notify, err := pbSub(body, "new_message_notify")
	if err != nil {
		t.Fatalf("pbSub new_message_notify: %v", err)
	}
	message, err := pbSub(notify, "message")
	if err != nil {
		t.Fatalf("pbSub message: %v", err)
	}
	for _, set := range []struct {
		name  string
		value any
	}{
		{"conversation_id", "0:1:1111:2222"},
		{"index_in_conversation", int64(7)},
		{"sender", int64(123456789)},
		{"message_type", int64(imText)},
		{"content", `{"text":"hello"}`},
	} {
		if err := SetProtoField(message, set.name, set.value); err != nil {
			t.Fatalf("SetProtoField %s: %v", set.name, err)
		}
	}
	respBytes, err := ProtoMarshal(resp)
	if err != nil {
		t.Fatalf("marshal Response: %v", err)
	}

	frame, err := ProtoNew("PushFrame")
	if err != nil {
		t.Fatalf("ProtoNew PushFrame: %v", err)
	}
	if err := SetProtoField(frame, "payloadType", "pb"); err != nil {
		t.Fatalf("payloadType: %v", err)
	}
	if err := SetProtoField(frame, "payload", respBytes); err != nil {
		t.Fatalf("payload: %v", err)
	}
	frameBytes, err := ProtoMarshal(frame)
	if err != nil {
		t.Fatalf("marshal PushFrame: %v", err)
	}

	r := &IMReceiver{ch: make(chan IMMessage, 4), stop: make(chan struct{})}
	r.initStop()
	r.handleFrame(frameBytes)

	select {
	case msg := <-r.ch:
		if msg.MessageIndex != 7 {
			t.Errorf("MessageIndex = %d, want 7", msg.MessageIndex)
		}
		if msg.ConversationID != "0:1:1111:2222" {
			t.Errorf("ConversationID = %q", msg.ConversationID)
		}
		if msg.Sender != 123456789 {
			t.Errorf("Sender = %d, want 123456789", msg.Sender)
		}
		if msg.MessageType != imText {
			t.Errorf("MessageType = %d, want %d", msg.MessageType, imText)
		}
		if got, _ := msg.Content["text"].(string); got != "hello" {
			t.Errorf("Content[text] = %v, want hello", msg.Content["text"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no IMMessage emitted by handleFrame")
	}
}
