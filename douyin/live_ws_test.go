package douyin

// Deterministic live-room tests: PK wire decoding, the PK dedup/context
// handler, the WebSocket frame dispatch/ack path and the handshake query.
// The wire captures and fixture ids come from a browser capture of the
// live-room WebSocket and PK HTTP APIs.

import (
	"cmp"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// liveTestBrowserVersion mirrors HeaderBuilder.ua.split("Mozilla/")[-1].
func liveTestBrowserVersion() string {
	return strings.Replace(GetProfile().UA, "Mozilla/", "", 1)
}

const (
	fxPKStart  = "0a1b108180daea81aad5a169188280daea81aad5a16920e4ceefc18b34124d108380daea81aad5a16918b3cbefc18b3420ac02308480daea81aad5a169c00201d2021337353835303030303030303030303030303033da021337353835303030303030303030303030303034"
	fxPKScores = "0a1b108580daea81aad5a169188280daea81aad5a16920e7d281c28b3410ca01408480daea81aad5a1698a012a08da01108680daea81aad5a1693a033231384001aa0113373538353030303030303030303030303030368a012a08d301108780daea81aad5a1693a033231314002aa011337353835303030303030303030303030303037c0068380daea81aad5a16992071337353835303030303030303030303030303033"
	fxPKArmies = "0a1b108880daea81aad5a169188280daea81aad5a16920b2d981c28b34124f088780daea81aad5a16912430a15088980daea81aad5a16910c6011a065669657765720a14088a80daea81aad5a16910051a065669657765720a14088b80daea81aad5a16910051a06566965776572124e088680daea81aad5a16912420a14088c80daea81aad5a16910631a065669657765720a14088d80daea81aad5a16910371a065669657765720a14088e80daea81aad5a16910071a06566965776572"
	fxPKFinish = "0a1b108f80daea81aad5a169188280daea81aad5a1692088f581c28b34124d108380daea81aad5a16918b3cbefc18b3420ac02308480daea81aad5a169c00202d2021337353835303030303030303030303030303033da0213373538353030303030303030303030303030341aa101088780daea81aad5a169122a088980daea81aad5a169120656696577657220c6012a13373538353030303030303030303030303030391229088a80daea81aad5a169120656696577657220052a13373538353030303030303030303030303031301229088b80daea81aad5a169120656696577657220052a13373538353030303030303030303030303031311a13373538353030303030303030303030303030371aa001088680daea81aad5a1691229088c80daea81aad5a169120656696577657220632a13373538353030303030303030303030303031321229088d80daea81aad5a169120656696577657220372a13373538353030303030303030303030303031331229088e80daea81aad5a169120656696577657220072a13373538353030303030303030303030303031341a1337353835303030303030303030303030303036222a08da01108680daea81aad5a1694213373538353030303030303030303030303030367a03323138800101222a08d301108780daea81aad5a1694213373538353030303030303030303030303030377a03323131800102"
	fxBattle   = "7585000000000000003"
	fxChannel  = "7585000000000000004"

	fxChat       = "12131a06546573746572f202085345432d434841541a0568656c6c6f"
	fxGift       = "30033a131a0653656e646572f202085345432d53454e4442151a085265636569766572f202085345432d524543567a07820104526f7365"
	fxMember     = "12131a064a6f696e6572f202085345432d4a4f494e182a"
	fxLike       = "100518642a121a054c696b6572f202085345432d4c494b45"
	fxSocial     = "12171a08466f6c6c6f776572f2020a5345432d464f4c4c4f5720013007"
	fxStats      = "220d31323334e4babae59ca8e7babf28d209"
	fxFramePlain = "1089808080808080103a036d736742ea020a340a1257656263617374436861744d657373616765121c12131a06546573746572f202085345432d434841541a0568656c6c6f18010a4f0a1257656263617374476966744d657373616765123730033a131a0653656e646572f202085345432d53454e4442151a085265636569766572f202085345432d524543567a07820104526f736518020a310a14576562636173744d656d6265724d657373616765121712131a064a6f696e6572f202085345432d4a4f494e182a18030a300a12576562636173744c696b654d6573736167651218100518642a121a054c696b6572f202085345432d4c494b4518040a370a1457656263617374536f6369616c4d657373616765121d12171a08466f6c6c6f776572f2020a5345432d464f4c4c4f572001300718050a2f0a1757656263617374526f6f6d53746174734d6573736167651212220d31323334e4babae59ca8e7babf28d20918061204435552312a0a6f70617175652d61636b4801"
	fxFrameGzip  = "1089808080808080103a036d7367429d021f8b08000000000002ffe332e1120a4f4d4a4e2c2e71ce482cf14d2d2e4e4c4f1592111296620b492d2e492dfac4c411eceaacebece11822c59a919a93932fc1c8e50fd7e59e9906d7656ec06c05d4179c9a9702d717eceae7e2242ac511949a9c9a5906170e72750eab626f626409ca2f4e9560e232e412811ae89b9a9b945a0433521ce410affccc3cb84e2f7f4f3f092d09662e03b81b7c32b353611a2404582552b484a4584182303d3e9edeae122c5ce6704b82f393331373607a6485c4a538dcf2815e2b0769e1026971f3f7f1f10f576034609760e5d2e712876a0ccacfcf0d2e492c2986e91552e23534323679b26bd7d3392b9eefdaaf718953824d88c53934c8508b2bbf20b1b03455373139db831100919734606a010000"
	fxDetail     = "1208435552534f522d582a054558542d58"

	fxExpectedQuery = `app_name=douyin_web&version_code=180800&webcast_sdk_version=1.0.15&update_version_code=1.0.15&compress=gzip&device_platform=web&cookie_enabled=true&screen_width=1707&screen_height=960&browser_language=zh-CN&browser_platform=Win32&browser_name=Mozilla&browser_version=__UA__&browser_online=true&tz_name=Etc%2FGMT-8&cursor=CUR1&internal_ext=EXT1&host=https%3A%2F%2Flive.douyin.com&aid=6383&live_id=1&did_rule=3&endpoint=live_pc&support_wrds=1&user_unique_id=1234567890&im_path=%2Fwebcast%2Fim%2Ffetch%2F&identity=audience&need_persist_msg_count=15&insert_task_id=&live_reason=&room_id=7654321098765432100&heartbeatDuration=0&signature=SIGPLACEHOLDER`
)

func liveUnhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return b
}

func liveTMap(t *testing.T, v any) map[string]any {
	t.Helper()
	o, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %#v", v)
	}
	return o
}

func liveTArr(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("not an array: %#v", v)
	}
	return l
}

func liveTReverse(list protoreflect.List) {
	n := list.Len()
	vals := make([]protoreflect.Value, n)
	for i := range n {
		vals[i] = list.Get(i)
	}
	for i := range n {
		list.Set(i, vals[n-1-i])
	}
}

func TestLivePKWireFixtures(t *testing.T) {
	start, err := DecodePKMessage("WebcastLinkMicBattleMethod", liveUnhex(t, fxPKStart), 0)
	if err != nil {
		t.Fatal(err)
	}
	scores, err := DecodePKMessage("LinkMicMethod", liveUnhex(t, fxPKScores), 0)
	if err != nil {
		t.Fatal(err)
	}
	armies, err := DecodePKMessage("WebcastLinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 0)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := DecodePKMessage("LinkMicBattleFinishMethod", liveUnhex(t, fxPKFinish), 0)
	if err != nil {
		t.Fatal(err)
	}

	if start["battle_id"] != fxBattle || start["channel_id"] != fxChannel {
		t.Fatalf("start ids: %v %v", start["battle_id"], start["channel_id"])
	}
	if start["type"] != "start" || toInt64(start["duration"]) != 300 {
		t.Fatalf("start type/duration: %v %v", start["type"], start["duration"])
	}
	scoreList := liveTArr(t, scores["scores"])
	got := []string{}
	for _, s := range scoreList {
		got = append(got, liveTMap(t, s)["score"].(string))
	}
	if strings.Join(got, ",") != "218,211" {
		t.Fatalf("scores: %v", got)
	}
	if liveTMap(t, scoreList[0])["anchor_id"] != "7585000000000000006" {
		t.Fatalf("score anchor: %v", liveTMap(t, scoreList[0])["anchor_id"])
	}
	if armies["battle_id"] != nil || armies["is_complete"] != false {
		t.Fatalf("armies flags: %v %v", armies["battle_id"], armies["is_complete"])
	}
	anchorUsers := liveTArr(t, liveTMap(t, liveTArr(t, armies["anchors"])[0])["users"])
	scoresStr := []string{}
	uids := []string{}
	for _, u := range anchorUsers {
		scoresStr = append(scoresStr, liveTMap(t, u)["score"].(string))
		uids = append(uids, liveTMap(t, u)["uid"].(string))
	}
	if strings.Join(scoresStr, ",") != "99,55,7" {
		t.Fatalf("army scores: %v", scoresStr)
	}
	if strings.Join(uids, ",") != "7585000000000000012,7585000000000000013,7585000000000000014" {
		t.Fatalf("army uids: %v", uids)
	}
	if liveTMap(t, anchorUsers[0])["nickname"] != "Viewer" {
		t.Fatalf("army nickname: %v", liveTMap(t, anchorUsers[0])["nickname"])
	}
	if finish["battle_id"] != fxBattle {
		t.Fatalf("finish battle: %v", finish["battle_id"])
	}
	a, _ := json.Marshal(finish["scores"])
	b, _ := json.Marshal(scores["scores"])
	if string(a) != string(b) {
		t.Fatalf("finish scores != score event scores:\n%s\n%s", a, b)
	}
	fa := liveTArr(t, finish["anchors"])
	slices.SortStableFunc(fa, func(a, b any) int {
		return cmp.Compare(liveTMap(t, a)["anchor_id"].(string), liveTMap(t, b)["anchor_id"].(string))
	})
	aa := liveTArr(t, armies["anchors"])
	faJSON, _ := json.Marshal(fa)
	aaJSON, _ := json.Marshal(aa)
	if string(faJSON) != string(aaJSON) {
		t.Fatalf("finish anchors != armies anchors:\n%s\n%s", faJSON, aaJSON)
	}

	// Unknown play mode is not an empty complete rank.
	other, err := DecodePKMessage("LinkMicArmiesMethod", []byte{0x22, 0x02, 0x08, 0x01}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if other["has_unsupported_rank_list_v2"] != true || other["is_complete"] != false {
		t.Fatalf("rank_list_v2: %v", other)
	}
	if skip, err := DecodePKMessage("LinkMicMethod", []byte{0x10, 0x64}, 0); err != nil || skip != nil {
		t.Fatalf("non-202 LinkMicMethod: %v %v", skip, err)
	}
	if skip, err := DecodePKMessage("WebcastChatMessage", []byte("anything"), 0); err != nil || skip != nil {
		t.Fatalf("chat is not PK: %v %v", skip, err)
	}

	// An explicit *_str id wins over the numeric field.
	msg, err := ProtoUnmarshal("douyin.pk.LinkMicBattleFinish", liveUnhex(t, fxPKFinish))
	if err != nil {
		t.Fatal(err)
	}
	armyList := msg.Mutable(msg.Descriptor().Fields().ByName("battle_armies")).List()
	rankList := armyList.Get(0).Message().Mutable(
		armyList.Get(0).Message().Descriptor().Fields().ByName("rank_list")).List()
	first := rankList.Get(0).Message()
	first.Set(first.Descriptor().Fields().ByName("user_id"), protoreflect.ValueOfInt64(1))
	raw, err := ProtoMarshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	forced, err := DecodePKMessage("LinkMicBattleFinishMethod", raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	uid := liveTMap(t, liveTArr(t, liveTMap(t, liveTArr(t, forced["anchors"])[0])["users"])[0])["uid"]
	if uid != "7585000000000000009" {
		t.Fatalf("explicit id lost: %v", uid)
	}
}

func TestPKHandlerDedupAndContext(t *testing.T) {
	h := NewPKMessageHandler()
	first, err := h.Handle("LinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 0)
	if err != nil || first["context_battle_id"] != nil {
		t.Fatalf("armies before battle: %v %v", first, err)
	}
	if _, err := h.Handle("LinkMicBattleMethod", liveUnhex(t, fxPKStart), 0); err != nil {
		t.Fatal(err)
	}
	event, err := h.Handle("WebcastLinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 99)
	if err != nil {
		t.Fatal(err)
	}
	if event["context_battle_id"] != fxBattle || event["battle_id"] != nil {
		t.Fatalf("armies context: %v %v", event["context_battle_id"], event["battle_id"])
	}
	if dup, err := h.Handle("LinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 99); err != nil || dup != nil {
		t.Fatalf("dup armies: %v %v", dup, err)
	}
	if _, err := h.Handle("WebcastLinkMicBattleFinishMethod", liveUnhex(t, fxPKFinish), 0); err != nil {
		t.Fatal(err)
	}
	if dup, err := h.Handle("LinkMicBattleFinishMethod", liveUnhex(t, fxPKFinish), 100); err != nil || dup != nil {
		t.Fatalf("dup finish: %v %v", dup, err)
	}
	reordered, err := ProtoUnmarshal("douyin.pk.LinkMicBattleFinish", liveUnhex(t, fxPKFinish))
	if err != nil {
		t.Fatal(err)
	}
	liveTReverse(reordered.Mutable(reordered.Descriptor().Fields().ByName("battle_armies")).List())
	liveTReverse(reordered.Mutable(reordered.Descriptor().Fields().ByName("battle_scores")).List())
	rawReordered, err := ProtoMarshal(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if dup, err := h.Handle("LinkMicBattleFinishMethod", rawReordered, 102); err != nil || dup != nil {
		t.Fatalf("reordered finish: %v %v", dup, err)
	}
	stale, err := h.Handle("LinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 101)
	if err != nil {
		t.Fatal(err)
	}
	if stale["context_battle_id"] != nil {
		t.Fatalf("armies after finish: %v", stale["context_battle_id"])
	}

	// A new battle on the same channel must not adopt stale armies.
	newer, err := ProtoUnmarshal("douyin.pk.LinkMicBattle", liveUnhex(t, fxPKStart))
	if err != nil {
		t.Fatal(err)
	}
	common := newer.Mutable(newer.Descriptor().Fields().ByName("common")).Message()
	common.Set(common.Descriptor().Fields().ByName("msg_id"), protoreflect.ValueOfInt64(200))
	settings := newer.Mutable(newer.Descriptor().Fields().ByName("battle_settings")).Message()
	bidFD := settings.Descriptor().Fields().ByName("battle_id")
	newBattle := settings.Get(bidFD).Int() + 100
	settings.Set(bidFD, protoreflect.ValueOfInt64(newBattle))
	settings.Set(settings.Descriptor().Fields().ByName("battle_id_str"),
		protoreflect.ValueOfString(strconv.FormatInt(newBattle, 10)))
	startFD := settings.Descriptor().Fields().ByName("start_time_ms")
	settings.Set(startFD, protoreflect.ValueOfInt64(settings.Get(startFD).Int()+400000))
	rawNewer, err := ProtoMarshal(newer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle("LinkMicBattleMethod", rawNewer, 0); err != nil {
		t.Fatal(err)
	}
	staleNew, err := h.Handle("LinkMicArmiesMethod", liveUnhex(t, fxPKArmies), 201)
	if err != nil {
		t.Fatal(err)
	}
	if staleNew["context_battle_id"] != nil {
		t.Fatalf("stale armies acquired context: %v", staleNew["context_battle_id"])
	}
	if _, err := h.Handle("LinkMicMethod", liveUnhex(t, fxPKScores), 202); err != nil {
		t.Fatal(err)
	}
	active := h.Active()
	if active == nil || active["battle_id"] != strconv.FormatInt(newBattle, 10) {
		t.Fatalf("active battle: %v", active)
	}
}

func TestLiveFrameDispatch(t *testing.T) {
	upgrader := websocket.Upgrader{}
	received := make(chan []byte, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received <- data
		}
	}))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	listener := NewLiveListener(nil, "7654321098765432100")
	writer := &liveWSWriter{conn: conn}

	expectEvents := func() {
		want := []string{"chat", "gift", "member", "like", "social", "room_stats"}
		for i, kind := range want {
			select {
			case event := <-listener.Events():
				if event.Kind != kind {
					t.Fatalf("event %d kind=%s want %s", i, event.Kind, kind)
				}
				if i == 0 {
					if event.Data["content"] != "hello" {
						t.Fatalf("chat content: %v", event.Data)
					}
					user := liveTMap(t, event.Data["user"])
					if user["nickname"] != "Tester" || user["sec_uid"] != "SEC-CHAT" {
						t.Fatalf("chat user: %v", user)
					}
				}
				if i == 1 {
					if event.Data["comboCount"].(string) != "3" || event.Data["toUser"] == nil {
						t.Fatalf("gift: %v", event.Data)
					}
					if liveTMap(t, event.Data["gift"])["name"] != "Rose" {
						t.Fatalf("gift name: %v", event.Data["gift"])
					}
				}
				if i == 3 {
					if event.Data["count"].(string) != "5" || event.Data["total"].(string) != "100" {
						t.Fatalf("like: %v", event.Data)
					}
				}
				if i == 4 {
					if event.Data["action"].(string) != "1" {
						t.Fatalf("social action: %v", event.Data)
					}
				}
				if i == 5 {
					if event.Data["displayLong"] != "1234人在线" {
						t.Fatalf("room stats: %v", event.Data)
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("missing event %d (%s)", i, kind)
			}
		}
	}

	expectAck := func() {
		select {
		case ackRaw := <-received:
			ack, err := ProtoUnmarshal("PushFrame", ackRaw)
			if err != nil {
				t.Fatal(err)
			}
			if ProtoFieldString(ack, "payloadType") != "ack" ||
				string(liveProtoBytes(ack, "payload")) != "opaque-ack" ||
				liveProtoUint64(ack, "logId") != 9007199254741001 {
				t.Fatalf("ack frame: %s %q %d", ProtoFieldString(ack, "payloadType"),
					liveProtoBytes(ack, "payload"), liveProtoUint64(ack, "logId"))
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no ack written")
		}
	}

	// Plain frame: all six message kinds must be decoded and emitted.
	listener.handleFrame(writer, liveUnhex(t, fxFramePlain))
	expectEvents()
	expectAck()

	// Replaying the identical messages (here as the gzip variant) must still
	// parse -- the ack proves it -- but must NOT re-emit them: upstream issue
	// #88 reports duplicated gift/chat events around reconnects.
	listener.handleFrame(writer, liveUnhex(t, fxFrameGzip))
	select {
	case event := <-listener.Events():
		t.Fatalf("replayed frame re-emitted %s despite msgId dedup", event.Kind)
	case <-time.After(500 * time.Millisecond):
	}
	expectAck()
}

func TestLiveWSHandshakeQuery(t *testing.T) {
	client, err := NewClient("ttwid=fake", Options{})
	if err != nil {
		t.Fatal(err)
	}
	listener := NewLiveListener(client, "7654321098765432100")
	query := listener.wsQuery("CUR1", "EXT1", "7654321098765432100", "1234567890")
	wantQuery := strings.Replace(fxExpectedQuery, "__UA__", url.QueryEscape(liveTestBrowserVersion()), 1)
	got := strings.Split(query, "&signature=")
	want := strings.Split(wantQuery, "&signature=")
	if len(got) != 2 || len(want) != 2 {
		t.Fatalf("query shape: %q", query)
	}
	if got[0] != want[0] {
		t.Fatalf("handshake query mismatch:\n got %s\nwant %s", got[0], want[0])
	}
	if got[1] == "" {
		t.Fatal("empty signature")
	}
	signature, err := url.QueryUnescape(got[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(signature) != 16 {
		t.Fatalf("x-bogus length %d (%q), the reference emits 16", len(signature), signature)
	}

	seed, err2 := ProtoUnmarshal("LiveResponse", liveUnhex(t, fxDetail))
	if err2 != nil {
		t.Fatal(err)
	}
	if ProtoFieldString(seed, "cursor") != "CURSOR-X" || ProtoFieldString(seed, "internalExt") != "EXT-X" {
		t.Fatalf("seed parse: %q %q", ProtoFieldString(seed, "cursor"), ProtoFieldString(seed, "internalExt"))
	}
}

func TestLiveInfoPageParse(t *testing.T) {
	page := `<html><head><script nonce="abc">window.__INITIAL_STATE__ = {\"a\":\"roomId\":\"7654321098765432100\",\"user_unique_id\":\"1234567890\",\"roomInfo\":{\"room\":{\"id_str\":\"7654321098765432100\",\"status\":2,\"status_str\":\"2\",\"title\":\"测试直播间\"}},\"anchor\":{\"id_str\":\"111222333\"},\"sec_uid\":\"MS4wLjABAAAA\"}</script></head></html>`
	info, ok := parseLiveInfoPage(page, "TTWID-X")
	if !ok {
		t.Fatal("nonce script not parsed")
	}
	for key, want := range map[string]string{
		"room_id": "7654321098765432100", "user_id": "1234567890",
		"anchor_id": "111222333", "sec_uid": "MS4wLjABAAAA", "ttwid": "TTWID-X",
		"room_status": "2", "room_title": "测试直播间",
	} {
		if info[key] != want {
			t.Errorf("%s = %v want %s", key, info[key], want)
		}
	}

	fallback := `x=\"roomId\":\"999\",\"user_unique_id\":\"888\",\"anchor\":{\"id_str\":\"777\"},\"sec_uid\":\"SEC\"y`
	info2, ok := parseLiveInfoPage(fallback, "T2")
	if !ok || info2["room_id"] != "999" || info2["anchor_id"] != "777" || info2["sec_uid"] != "SEC" {
		t.Fatalf("fallback parse: %v %v", info2, ok)
	}
}

func TestLiveDecodeJSONKeepsBigInts(t *testing.T) {
	decoded, err := liveDecodeJSON([]byte(`{"id_str":"7687099550015753000","linker_map":{"1":7687114086841259023},"n":1,"f":1.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if decoded["n"].(int64) != 1 || decoded["f"].(float64) != 1.5 {
		t.Fatalf("numbers: %#v", decoded)
	}
	if liveIDText(liveTMap(t, decoded["linker_map"]), "1") != "7687114086841259023" {
		t.Fatalf("linker map id: %v", decoded["linker_map"])
	}
	if rid, err := (&Client{}).LiveWebRID("https://live.douyin.com/403309276429?from=test"); err != nil || rid != "403309276429" {
		t.Fatalf("web rid: %q %v", rid, err)
	}
	for _, bad := range []string{"https://example.com/123", "1e20", "https://live.douyin.com/abc"} {
		if _, err := (&Client{}).LiveWebRID(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestLivePKContextFixtures(t *testing.T) {
	roomJSON := `{"status_code":0,"data":{"data":[{"id_str":"7687099550015753000","owner":{"id_str":"1565319530819928","nickname":"主播"},"linker_map":{"1":7687114086841259023}}]}}`
	snapshotJSON := `{"status_code":0,"data":{"battle_stats":{"battle_settings":{"battle_id_str":"7687114318937084966","channel_id_str":"7687114086841259023","finished":0},"user_infos":{"7585857326805287985":{"user":{"user_id_str":"7585857326805287985","nick_name":"对手"},"room_id":"7687000000000000001"}}}}}`
	room, err := liveDecodeJSON([]byte(roomJSON))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := liveDecodeJSON([]byte(snapshotJSON))
	if err != nil {
		t.Fatal(err)
	}

	state := newLivePKContext("403309276429", "", room)
	if !state.ready() {
		t.Fatalf("state not ready: %#v", state.context)
	}
	state.applySnapshot(snapshot)
	result := state.result()

	for key, want := range map[string]any{
		"web_rid": "403309276429", "room_id": "7687099550015753000",
		"anchor_id": "1565319530819928", "channel_id": "7687114086841259023",
		"battle_id": "7687114318937084966", "finished": int64(0),
	} {
		if result[key] != want {
			t.Errorf("%s = %#v want %#v", key, result[key], want)
		}
	}
	anchors, ok := result["anchors"].([]any)
	if !ok || len(anchors) != 2 {
		t.Fatalf("anchors: %#v", result["anchors"])
	}
	first, _ := anchors[0].(map[string]any)
	second, _ := anchors[1].(map[string]any)
	if first["nickname"] != "主播" || first["anchor_id"] != "1565319530819928" {
		t.Errorf("anchor 0: %#v", first)
	}
	if second["nickname"] != "对手" || second["anchor_id"] != "7585857326805287985" ||
		second["room_id"] != "7687000000000000001" {
		t.Errorf("anchor 1: %#v", second)
	}

	// A room without a linker_map must not query the snapshot (partial context).
	noChannel, err := liveDecodeJSON([]byte(`{"status_code":0,"data":{"data":[{"id_str":"7687099550015753000","owner":{"id_str":"1565319530819928","nickname":"主播"},"linker_map":{}}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	partial := newLivePKContext("403309276429", "", noChannel)
	if partial.ready() {
		t.Fatal("ready without channel")
	}
	out := partial.result()
	if out["channel_id"] != nil || out["battle_id"] != nil {
		t.Errorf("partial context: %#v", out)
	}
	if _, ok := out["battle_stats"].(map[string]any); !ok {
		t.Errorf("partial battle_stats: %#v", out["battle_stats"])
	}

}
