package douyin

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestProtoDescriptorsCompile(t *testing.T) {
	for _, name := range []string{
		"PushFrame", "LiveResponse", "Message", "ChatMessage", "GiftMessage",
		"MemberMessage", "LikeMessage", "SocialMessage", "RoomStatsMessage",
		"Request", "Response", "douyin.pk.LinkMicMethod",
		"douyin.pk.LinkMicArmies", "douyin.pk.LinkMicBattle",
		"douyin.pk.LinkMicBattleFinish",
	} {
		if _, err := ProtoNew(name); err != nil {
			t.Fatalf("ProtoNew(%s): %v", name, err)
		}
	}
}

func TestProtoRoundTrip(t *testing.T) {
	frame, err := ProtoNew("PushFrame")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetProtoField(frame, "payloadType", "hb"); err != nil {
		t.Fatal(err)
	}
	data, err := ProtoMarshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProtoUnmarshal("PushFrame", data)
	if err != nil {
		t.Fatal(err)
	}
	if v := ProtoFieldString(got, "payloadType"); v != "hb" {
		t.Fatalf("payloadType = %q", v)
	}
	if !proto.Equal(frame, got) {
		t.Fatalf("round trip mismatch")
	}
}
