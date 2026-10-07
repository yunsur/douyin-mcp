package douyin

import (
	"os"
	"strings"
	"testing"
)

// --- 编解码回归（无网络） ---------------------------------------------------

// 断言 4 个管理类操作的 cmd / body 包装字段号与字段布局。
// 服务端按「cmd + body 包装字段号」分发，写错会得到 "body is nil / is empty"。
func TestIMAdminRequestBody(t *testing.T) {
	name := "群名"
	notice := "公告"
	cases := []struct {
		op        string
		cmd       int64
		bodyField int64
		body      *pbw
		wantStr   map[int]string
		wantInt   map[int]int64
		wantList  map[int][]int64
	}{
		{
			op:        "set_core_info",
			cmd:       902,
			bodyField: 902,
			body:      imCoreInfoBody("cid", 42, 2, IMCoreInfoOptions{Name: &name, Notice: &notice}),
			wantStr:   map[int]string{1: "cid", 4: "群名", 7: "公告"},
			wantInt:   map[int]int64{2: 42, 3: 2, 8: 1, 11: 1},
		},
		{
			op:        "remove_participants",
			cmd:       651,
			bodyField: 651,
			body:      imRemoveParticipantsBody("cid", 42, 2, []int64{11, 22}),
			wantStr:   map[int]string{1: "cid"},
			wantInt:   map[int]int64{2: 42, 3: 2},
			wantList:  map[int][]int64{4: {11, 22}}, // participants 是重复 int64
		},
		{
			op:        "recall_message",
			cmd:       702,
			bodyField: 702,
			body:      imRecallMessageBody("cid", 42, 2, 99887766),
			wantStr:   map[int]string{1: "cid"},
			wantInt:   map[int]int64{2: 42, 3: 2, 4: 99887766},
		},
		{
			op:        "delete_message",
			cmd:       701,
			bodyField: 701,
			body:      imDeleteMessageBody("cid", 42, 2, 1234),
			wantStr:   map[int]string{1: "cid"},
			wantInt:   map[int]int64{2: 42, 3: 2, 4: 1234},
		},
	}

	for _, tc := range cases {
		// body 必须包在 bodyField 里，cmd 必须写在信封的字段 1。
		wrapped := &pbw{}
		wrapped.Msg(int(tc.bodyField), tc.body)
		env := imEnvelope(tc.cmd, 1, wrapped)

		parsed, err := pbParse(env)
		if err != nil {
			t.Fatalf("%s: 解析信封失败: %v", tc.op, err)
		}
		if got := pbInt(parsed, 1); got != tc.cmd {
			t.Errorf("%s: cmd = %d want %d", tc.op, got, tc.cmd)
		}
		bodyFields, ok := pbMsg(parsed, 8)
		if !ok {
			t.Fatalf("%s: 信封里没有 body(8)", tc.op)
		}
		inner, ok := pbMsg(bodyFields, int(tc.bodyField))
		if !ok {
			t.Fatalf("%s: body 未被包在字段 %d 里（实际字段: %v）", tc.op, tc.bodyField, bodyFields)
		}
		for num, want := range tc.wantStr {
			if got := pbString(inner, num); got != want {
				t.Errorf("%s: 字段 %d = %q want %q", tc.op, num, got, want)
			}
		}
		for num, want := range tc.wantInt {
			if got := pbInt(inner, num); got != want {
				t.Errorf("%s: 字段 %d = %d want %d", tc.op, num, got, want)
			}
		}
		for num, want := range tc.wantList {
			got := pbList(inner, num)
			if len(got) != len(want) {
				t.Errorf("%s: 字段 %d = %v want %v", tc.op, num, got, want)
				continue
			}
			for i, v := range want {
				if toInt64(got[i]) != v {
					t.Errorf("%s: 字段 %d[%d] = %v want %d", tc.op, num, i, got[i], v)
				}
			}
		}
	}
}

// --- 路由实测（不碰任何真实会话） -------------------------------------------

// 用不存在的会话 id 调 4 个管理接口，验证「服务端认得这个 cmd/body 组合」：
// 正确编号会走到业务校验（权限/会话不存在），错误编号会直接报 body 为空。
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveIMAdminCmdRouting -v
func TestLiveIMAdminCmdRouting(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1")
	}
	c, err := NewClient(liveCookie(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	const bogus = "0" // 不存在的会话：服务端只会回业务错，不会改动任何会话

	missing := func(msg string) bool {
		for _, bad := range []string{"is nil", "is empty", "invalid reqBody", "Unsupported"} {
			if strings.Contains(msg, bad) {
				return true
			}
		}
		return false
	}

	check := func(op string, cmd, bodyField int64, body *pbw, path string) {
		t.Helper()
		_, err := c.imCall(ctx, path, cmd, 1, bodyField, body, bodyField)
		if err == nil {
			t.Logf("%-20s cmd=%d body=%d -> OK", op, cmd, bodyField)
			return
		}
		msg := err.Error()
		if missing(msg) {
			t.Errorf("%-20s cmd=%d body=%d -> body 未被识别: %v", op, cmd, bodyField, err)
			return
		}
		t.Logf("%-20s cmd=%d body=%d -> 服务端已解析（业务错误）: %v", op, cmd, bodyField, err)
	}

	check("set_core_info", 902, 902, imCoreInfoBody(bogus, 0, 2, IMCoreInfoOptions{Name: new("x")}), "/v1/conversation/set_core_info")
	check("remove_participants", 651, 651, imRemoveParticipantsBody(bogus, 0, 2, []int64{1}), "/v1/conversation/remove_participants")
	check("recall_message", 702, 702, imRecallMessageBody(bogus, 0, 2, 1), "/v1/message/recall")
	check("delete_message", 701, 701, imDeleteMessageBody(bogus, 0, 2, 1), "/v1/message/delete")
}
