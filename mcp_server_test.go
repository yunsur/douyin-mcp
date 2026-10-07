package main

// Tests for the MCP tool registry in mcp_server.go and the slice registration
// functions (mcp_misc_*.go, mcp_im_admin.go).
//
// Everything here goes through the official SDK client connected to the real
// server over an in-memory transport, so the assertions cover what an MCP
// client actually observes (tools/list payload, generated input schema,
// annotations and tools/call results) rather than internal wiring.

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yunsur/douyin-mcp/douyin"
)

// --- helpers ---------------------------------------------------------------

// mcpServerTestConnect wires an official SDK client to server over an
// in-memory transport and returns the connected client session.
func mcpServerTestConnect(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "douyin-mcp-test", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

// mcpServerTestTools issues tools/list and returns the advertised tools, keyed
// by name. A duplicate name or a paginated response fails the test.
func mcpServerTestTools(t *testing.T, server *mcp.Server) map[string]*mcp.Tool {
	t.Helper()
	session := mcpServerTestConnect(t, server)
	result, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if result.NextCursor != "" {
		t.Fatalf("tools/list: unexpected pagination cursor %q", result.NextCursor)
	}
	byName := make(map[string]*mcp.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		if _, dup := byName[tool.Name]; dup {
			t.Fatalf("tools/list: tool %q registered twice", tool.Name)
		}
		byName[tool.Name] = tool
	}
	return byName
}

// mcpServerTestTool returns the named tool, failing when it is absent.
func mcpServerTestTool(t *testing.T, tools map[string]*mcp.Tool, name string) *mcp.Tool {
	t.Helper()
	tool, ok := tools[name]
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	return tool
}

// mcpServerTestSchema decodes a tool's generated JSON input schema.
func mcpServerTestSchema(t *testing.T, tool *mcp.Tool) map[string]any {
	t.Helper()
	switch schema := tool.InputSchema.(type) {
	case map[string]any:
		return schema
	case json.RawMessage:
		var decoded map[string]any
		if err := json.Unmarshal(schema, &decoded); err != nil {
			t.Fatalf("tool %s: decode input schema: %v", tool.Name, err)
		}
		return decoded
	default:
		t.Fatalf("tool %s: unexpected input schema type %T", tool.Name, tool.InputSchema)
		return nil
	}
}

// mcpServerTestProperties returns the schema's "properties" object.
func mcpServerTestProperties(t *testing.T, tool *mcp.Tool) map[string]any {
	t.Helper()
	schema := mcpServerTestSchema(t, tool)
	raw, ok := schema["properties"]
	if !ok {
		return map[string]any{}
	}
	props, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("tool %s: properties is %T, want object", tool.Name, raw)
	}
	return props
}

// mcpServerTestProperty returns the JSON schema of one input property.
func mcpServerTestProperty(t *testing.T, tool *mcp.Tool, name string) map[string]any {
	t.Helper()
	props := mcpServerTestProperties(t, tool)
	raw, ok := props[name]
	if !ok {
		t.Fatalf("tool %s: input schema has no property %q (have %v)", tool.Name, name, mcpServerTestSortedKeys(props))
	}
	prop, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("tool %s: property %q is %T, want object", tool.Name, name, raw)
	}
	return prop
}

// mcpServerTestRequired returns the schema's required field set.
func mcpServerTestRequired(t *testing.T, tool *mcp.Tool) map[string]bool {
	t.Helper()
	schema := mcpServerTestSchema(t, tool)
	raw, ok := schema["required"]
	if !ok {
		return map[string]bool{}
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("tool %s: required is %T, want array", tool.Name, raw)
	}
	required := make(map[string]bool, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			t.Fatalf("tool %s: required entry is %T, want string", tool.Name, item)
		}
		required[name] = true
	}
	return required
}

// mcpServerTestTypeContains asserts a property's "type" covers want; the SDK
// emits either "boolean" or ["null","boolean"] for nullable Go fields.
func mcpServerTestTypeContains(t *testing.T, tool *mcp.Tool, prop string, schema map[string]any, want string) {
	t.Helper()
	raw, ok := schema["type"]
	if !ok {
		t.Fatalf("tool %s: property %q has no type", tool.Name, prop)
	}
	switch v := raw.(type) {
	case string:
		if v != want {
			t.Fatalf("tool %s: property %q type = %q, want %q", tool.Name, prop, v, want)
		}
	case []any:
		found := false
		for _, item := range v {
			if item == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("tool %s: property %q type = %v, want it to contain %q", tool.Name, prop, v, want)
		}
	default:
		t.Fatalf("tool %s: property %q type is %T", tool.Name, prop, raw)
	}
}

// mcpServerTestSortedKeys renders a property map's keys for failure messages.
func mcpServerTestSortedKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}

// mcpServerTestTextContent asserts a result carries exactly one text block and
// returns it.
func mcpServerTestTextContent(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("result has %d content blocks, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("result content is %T, want *mcp.TextContent", res.Content[0])
	}
	return text.Text
}

// mcpServerTestJSONText decodes a tool's text payload, preserving numbers.
func mcpServerTestJSONText(t *testing.T, text string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatalf("decode tool text %q: %v", text, err)
	}
	return decoded
}

// --- registry --------------------------------------------------------------

// TestMCPServerToolRegistry pins the total tool count and the presence of the
// documented tool names on the real app server.
func TestMCPServerToolRegistry(t *testing.T) {
	svc, _ := newTestService(t, nil)
	app := NewAppServer(svc, "")
	tools := mcpServerTestTools(t, app.mcpServer)

	if len(tools) != 133 {
		t.Fatalf("registered %d tools, want 133", len(tools))
	}

	wantNames := []string{
		// Original core surface.
		"check_login_status",
		"get_login_qrcode",
		"search_videos",
		"publish_content",
		"publish_with_video",
		"send_dm",
		"list_conversations",
		"start_live_listen",
		// One representative per misc slice registration function.
		"get_channel_module_feed",
		"get_follow_feed",
		"get_my_profile",
		"get_im_spotlight_relation",
		"get_hot_search_videos",
		"get_product_sku_list",
		"get_notice_digg_list",
		"update_conversation_name",
	}
	for _, name := range wantNames {
		tool := mcpServerTestTool(t, tools, name)
		if tool.Description == "" {
			t.Errorf("tool %q has no description", name)
		}
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", name)
		}
	}
}

// TestMCPServerMiscSliceToolCount checks miscSliceToolCount against the tools
// actually registered by the slice registration functions, and that the
// remaining tools plus the slices add up to the advertised total (133).
func TestMCPServerMiscSliceToolCount(t *testing.T) {
	svc, _ := newTestService(t, nil)
	app := NewAppServer(svc, "")

	slices := []struct {
		label    string
		want     int
		register func(*mcp.Server, *AppServer)
	}{
		{"feeds", 11, registerMiscFeedsTools},
		{"social", 11, registerMiscSocialTools},
		{"profile", 8, registerMiscProfileTools},
		{"im", 9, registerMiscImTools},
		{"hotmusic", 5, registerMiscHotMusicTools},
		{"ecom", 1, registerMiscEcomTools},
		{"notice", 1, registerMiscNoticeTools},
		{"im_admin", 4, registerIMAdminTools},
	}

	miscNames := map[string]bool{}
	total := 0
	for _, slice := range slices {
		server := mcp.NewServer(&mcp.Implementation{Name: "douyin-mcp-" + slice.label, Version: "1.0.0"}, nil)
		slice.register(server, app)
		tools := mcpServerTestTools(t, server)
		if len(tools) != slice.want {
			t.Errorf("slice %s registered %d tools, want %d", slice.label, len(tools), slice.want)
		}
		for name := range tools {
			if miscNames[name] {
				t.Errorf("tool %q registered by more than one slice", name)
			}
			miscNames[name] = true
		}
		total += len(tools)
	}

	if total != miscSliceToolCount {
		t.Errorf("slices registered %d tools, miscSliceToolCount is %d", total, miscSliceToolCount)
	}

	// The startup log reports 83+miscSliceToolCount; verify that the base tools
	// really are everything outside the slices.
	all := mcpServerTestTools(t, app.mcpServer)
	const baseToolCount = 83
	if got := len(all) - len(miscNames); got != baseToolCount {
		t.Errorf("non-slice tools = %d, want %d", got, baseToolCount)
	}
	if len(all) != baseToolCount+miscSliceToolCount {
		t.Errorf("total tools = %d, want %d", len(all), baseToolCount+miscSliceToolCount)
	}
	for name := range miscNames {
		if _, ok := all[name]; !ok {
			t.Errorf("slice tool %q missing from the full registry", name)
		}
	}
}

// --- input schemas ---------------------------------------------------------

// TestMCPServerToolInputSchemas checks the generated JSON schema of a
// representative sample: required fields, property types, and no-argument
// tools exposing an empty object schema.
func TestMCPServerToolInputSchemas(t *testing.T) {
	svc, _ := newTestService(t, nil)
	app := NewAppServer(svc, "")
	tools := mcpServerTestTools(t, app.mcpServer)

	t.Run("no-argument tools", func(t *testing.T) {
		for _, name := range []string{"check_login_status", "get_login_qrcode", "get_notice_count", "get_live_events"} {
			tool := mcpServerTestTool(t, tools, name)
			if props := mcpServerTestProperties(t, tool); len(props) != 0 {
				t.Errorf("tool %q exposes properties %v, want none", name, mcpServerTestSortedKeys(props))
			}
			if req := mcpServerTestRequired(t, tool); len(req) != 0 {
				t.Errorf("tool %q requires %v, want nothing", name, req)
			}
		}
	})

	t.Run("search_videos", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "search_videos")
		schema := mcpServerTestSchema(t, tool)
		if got := schema["type"]; got != "object" {
			t.Errorf("schema type = %v, want object", got)
		}
		req := mcpServerTestRequired(t, tool)
		if !req["keyword"] {
			t.Errorf("required = %v, want keyword", req)
		}
		for _, optional := range []string{"offset", "channel", "sort_type", "publish_time", "filter_duration", "search_range", "content_type", "count"} {
			if req[optional] {
				t.Errorf("required = %v, want %q optional", req, optional)
			}
			mcpServerTestProperty(t, tool, optional)
		}
		mcpServerTestTypeContains(t, tool, "keyword", mcpServerTestProperty(t, tool, "keyword"), "string")
		// count is the only typed-numeric knob: it triggers auto-pagination.
		mcpServerTestTypeContains(t, tool, "count", mcpServerTestProperty(t, tool, "count"), "integer")
		mcpServerTestTypeContains(t, tool, "sort_type", mcpServerTestProperty(t, tool, "sort_type"), "string")
	})

	t.Run("list_conversations", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "list_conversations")
		if req := mcpServerTestRequired(t, tool); len(req) != 0 {
			t.Errorf("required = %v, want nothing (filters are optional)", req)
		}
		mcpServerTestTypeContains(t, tool, "name", mcpServerTestProperty(t, tool, "name"), "string")
		mcpServerTestTypeContains(t, tool, "type", mcpServerTestProperty(t, tool, "type"), "integer")
	})

	t.Run("send_dm", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "send_dm")
		req := mcpServerTestRequired(t, tool)
		for _, name := range []string{"to_user_id", "content"} {
			if !req[name] {
				t.Errorf("required = %v, want %q", req, name)
			}
		}
		mcpServerTestTypeContains(t, tool, "to_user_id", mcpServerTestProperty(t, tool, "to_user_id"), "integer")
		mcpServerTestTypeContains(t, tool, "content", mcpServerTestProperty(t, tool, "content"), "string")
	})

	t.Run("get_all_video_comments", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "get_all_video_comments")
		req := mcpServerTestRequired(t, tool)
		if !req["video"] {
			t.Errorf("required = %v, want video", req)
		}
		if req["include_replies"] || req["limit"] {
			t.Errorf("required = %v, want include_replies/limit optional", req)
		}
		mcpServerTestTypeContains(t, tool, "include_replies", mcpServerTestProperty(t, tool, "include_replies"), "boolean")
		mcpServerTestTypeContains(t, tool, "limit", mcpServerTestProperty(t, tool, "limit"), "integer")
	})

	t.Run("set_conversation_setting", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "set_conversation_setting")
		req := mcpServerTestRequired(t, tool)
		if !req["conversation"] {
			t.Errorf("required = %v, want conversation", req)
		}
		if req["pin"] || req["mute"] {
			t.Errorf("required = %v, want pin/mute optional", req)
		}
		// Nullable *bool fields stay nullable so "omitted" is distinguishable
		// from "false".
		pin := mcpServerTestProperty(t, tool, "pin")
		mcpServerTestTypeContains(t, tool, "pin", pin, "boolean")
		types, ok := pin["type"].([]any)
		if !ok || len(types) != 2 || types[0] != "null" {
			t.Errorf("pin type = %v, want [null, boolean]", pin["type"])
		}
	})

	t.Run("get_im_user_info", func(t *testing.T) {
		tool := mcpServerTestTool(t, tools, "get_im_user_info")
		if !mcpServerTestRequired(t, tool)["sec_uids"] {
			t.Errorf("sec_uids must be required")
		}
		prop := mcpServerTestProperty(t, tool, "sec_uids")
		mcpServerTestTypeContains(t, tool, "sec_uids", prop, "array")
		items, ok := prop["items"].(map[string]any)
		if !ok {
			t.Fatalf("sec_uids items = %v, want object", prop["items"])
		}
		if items["type"] != "string" {
			t.Errorf("sec_uids items type = %v, want string", items["type"])
		}
	})

	t.Run("unknown properties are rejected", func(t *testing.T) {
		for _, name := range []string{"search_videos", "send_dm", "digg_video", "set_conversation_setting"} {
			tool := mcpServerTestTool(t, tools, name)
			schema := mcpServerTestSchema(t, tool)
			if schema["additionalProperties"] != false {
				t.Errorf("tool %q: additionalProperties = %v, want false", name, schema["additionalProperties"])
			}
		}
	})
}

// --- annotations -----------------------------------------------------------

// TestMCPServerToolAnnotations checks the read-only / destructive split is
// complete: every tool is one of the two, and the well-known names land in the
// expected class.
func TestMCPServerToolAnnotations(t *testing.T) {
	svc, _ := newTestService(t, nil)
	app := NewAppServer(svc, "")
	tools := mcpServerTestTools(t, app.mcpServer)

	readOnlyNames := []string{
		"check_login_status",
		"get_login_qrcode",
		"search_videos",
		"list_conversations",
		"get_conversation_history",
		"get_live_events",
		"get_user_info",
	}
	destructiveNames := []string{
		"digg_video",
		"publish_content",
		"publish_with_video",
		"send_dm",
		"start_live_listen",
		"delete_cookies",
		"set_conversation_setting",
		"recall_message",
	}
	isNamed := func(names []string, name string) bool {
		for _, candidate := range names {
			if candidate == name {
				return true
			}
		}
		return false
	}

	readOnlyCount, destructiveCount := 0, 0
	for name, tool := range tools {
		ann := tool.Annotations
		if ann == nil {
			t.Errorf("tool %q: no annotations", name)
			continue
		}
		if ann.Title != name {
			t.Errorf("tool %q: annotation title = %q, want the tool name", name, ann.Title)
		}
		switch {
		case ann.ReadOnlyHint:
			readOnlyCount++
			if ann.DestructiveHint != nil && *ann.DestructiveHint {
				t.Errorf("tool %q: read-only but marked destructive", name)
			}
			if !isNamed(readOnlyNames, name) && isNamed(destructiveNames, name) {
				t.Errorf("tool %q: read-only, want destructive", name)
			}
		default:
			destructiveCount++
			if ann.DestructiveHint == nil {
				t.Errorf("tool %q: write tool has no destructiveHint", name)
			} else if !*ann.DestructiveHint {
				t.Errorf("tool %q: write tool is not marked destructive", name)
			}
			if isNamed(readOnlyNames, name) {
				t.Errorf("tool %q: destructive, want read-only", name)
			}
		}
	}

	for _, name := range readOnlyNames {
		tool := mcpServerTestTool(t, tools, name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q: readOnlyHint = %v, want true", name, tool.Annotations)
		}
	}
	for _, name := range destructiveNames {
		tool := mcpServerTestTool(t, tools, name)
		ann := tool.Annotations
		if ann == nil {
			t.Errorf("tool %q: no annotations", name)
			continue
		}
		if ann.ReadOnlyHint {
			t.Errorf("tool %q: readOnlyHint = true, want false", name)
		}
		if ann.DestructiveHint == nil || !*ann.DestructiveHint {
			t.Errorf("tool %q: destructiveHint = %v, want true", name, ann.DestructiveHint)
		}
	}

	if readOnlyCount != 92 {
		t.Errorf("read-only tools = %d, want 92", readOnlyCount)
	}
	if destructiveCount != 41 {
		t.Errorf("destructive tools = %d, want 41", destructiveCount)
	}
	if readOnlyCount+destructiveCount != len(tools) {
		t.Errorf("read-only + destructive = %d, want %d", readOnlyCount+destructiveCount, len(tools))
	}
}

// --- execution -------------------------------------------------------------

// TestMCPServerToolExecution calls tools end-to-end through the in-memory
// client: a text result mirrors the stub's JSON, and handler validation
// failures surface as tool errors without touching the transport.
func TestMCPServerToolExecution(t *testing.T) {
	t.Run("check_login_status mirrors the stub payload", func(t *testing.T) {
		svc, stub := newTestService(t, func(req mainStubRequest) (*douyin.Response, error) {
			switch {
			case strings.Contains(req.URL, "/aweme/v1/web/query/user/"):
				return mainJSONResponse(map[string]any{"user_uid": 987654321}), nil
			case strings.Contains(req.URL, "/web/api/media/user/info/"):
				return mainJSONResponse(map[string]any{"user": map[string]any{"sec_uid": "SEC_STUB_UID"}}), nil
			}
			return mainJSONResponse(map[string]any{}), nil
		})
		app := NewAppServer(svc, "")
		session := mcpServerTestConnect(t, app.mcpServer)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "check_login_status"})
		if err != nil {
			t.Fatalf("tools/call check_login_status: %v", err)
		}
		if res.IsError {
			t.Fatalf("check_login_status returned an error result: %s", mcpServerTestTextContent(t, res))
		}
		data := mcpServerTestJSONText(t, mcpServerTestTextContent(t, res))
		if loggedIn, ok := data["logged_in"].(bool); !ok || !loggedIn {
			t.Errorf("logged_in = %v, want true", data["logged_in"])
		}
		if uid, ok := data["uid"].(json.Number); !ok || uid.String() != "987654321" {
			t.Errorf("uid = %v, want 987654321", data["uid"])
		}
		if data["sec_uid"] != "SEC_STUB_UID" {
			t.Errorf("sec_uid = %v, want SEC_STUB_UID", data["sec_uid"])
		}

		var sawUID, sawSecUID bool
		for _, req := range stub.requests() {
			if strings.Contains(req.URL, "/aweme/v1/web/query/user/") {
				sawUID = true
			}
			if strings.Contains(req.URL, "/web/api/media/user/info/") {
				sawSecUID = true
			}
		}
		if !sawUID {
			t.Error("check_login_status never probed /aweme/v1/web/query/user/")
		}
		if !sawSecUID {
			t.Error("check_login_status never probed the creator user/info endpoint")
		}
	})

	t.Run("arguments reach the transport", func(t *testing.T) {
		svc, stub := newTestService(t, func(req mainStubRequest) (*douyin.Response, error) {
			if strings.Contains(req.URL, "/aweme/v1/web/search/sug/") {
				return mainJSONResponse(map[string]any{
					"sug_list": []any{map[string]any{"content": "火锅 附近"}},
				}), nil
			}
			return mainJSONResponse(map[string]any{}), nil
		})
		app := NewAppServer(svc, "")
		session := mcpServerTestConnect(t, app.mcpServer)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "search_suggest",
			Arguments: map[string]any{"keyword": "火锅"},
		})
		if err != nil {
			t.Fatalf("tools/call search_suggest: %v", err)
		}
		if res.IsError {
			t.Fatalf("search_suggest returned an error result: %s", mcpServerTestTextContent(t, res))
		}
		data := mcpServerTestJSONText(t, mcpServerTestTextContent(t, res))
		sugList, ok := data["sug_list"].([]any)
		if !ok || len(sugList) != 1 {
			t.Fatalf("sug_list = %v, want one entry", data["sug_list"])
		}
		entry, ok := sugList[0].(map[string]any)
		if !ok || entry["content"] != "火锅 附近" {
			t.Errorf("sug_list[0] = %v, want content 火锅 附近", sugList[0])
		}

		var found bool
		for _, req := range stub.requests() {
			if !strings.Contains(req.URL, "/aweme/v1/web/search/sug/") {
				continue
			}
			found = true
			parsed, err := url.Parse(req.URL)
			if err != nil {
				t.Fatalf("parse recorded URL %q: %v", req.URL, err)
			}
			if got := parsed.Query().Get("keyword"); got != "火锅" {
				t.Errorf("keyword = %q, want 火锅", got)
			}
		}
		if !found {
			t.Error("search_suggest never called /aweme/v1/web/search/sug/")
		}
	})

	t.Run("missing required argument is rejected before the handler", func(t *testing.T) {
		svc, stub := newTestService(t, nil)
		app := NewAppServer(svc, "")
		session := mcpServerTestConnect(t, app.mcpServer)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "search_videos"})
		if err != nil {
			t.Fatalf("tools/call search_videos: %v", err)
		}
		if !res.IsError {
			t.Fatalf("search_videos without keyword succeeded: %s", mcpServerTestTextContent(t, res))
		}
		if text := mcpServerTestTextContent(t, res); !strings.Contains(text, "keyword") {
			t.Errorf("rejection text %q does not mention keyword", text)
		}
		if reqs := stub.requests(); len(reqs) != 0 {
			t.Errorf("invalid call issued %d HTTP requests, want 0", len(reqs))
		}
	})

	t.Run("unknown argument is rejected before the handler", func(t *testing.T) {
		svc, stub := newTestService(t, nil)
		app := NewAppServer(svc, "")
		session := mcpServerTestConnect(t, app.mcpServer)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "search_suggest",
			Arguments: map[string]any{"keyword": "火锅", "bogus": 1},
		})
		if err != nil {
			t.Fatalf("tools/call search_suggest: %v", err)
		}
		if !res.IsError {
			t.Fatalf("search_suggest accepted an unknown argument: %s", mcpServerTestTextContent(t, res))
		}
		if reqs := stub.requests(); len(reqs) != 0 {
			t.Errorf("rejected call issued %d HTTP requests, want 0", len(reqs))
		}
	})

	t.Run("handler error becomes a tool error result", func(t *testing.T) {
		svc, stub := newTestService(t, nil)
		app := NewAppServer(svc, "")
		session := mcpServerTestConnect(t, app.mcpServer)

		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "set_conversation_setting",
			Arguments: map[string]any{"conversation": "九号mz5闲聊群"},
		})
		if err != nil {
			t.Fatalf("tools/call set_conversation_setting: %v", err)
		}
		if !res.IsError {
			t.Fatal("set_conversation_setting without pin/mute did not fail")
		}
		text := mcpServerTestTextContent(t, res)
		if !strings.Contains(text, "pin") || !strings.Contains(text, "mute") {
			t.Errorf("error text %q does not mention pin/mute", text)
		}
		if reqs := stub.requests(); len(reqs) != 0 {
			t.Errorf("rejected call issued %d HTTP requests, want 0", len(reqs))
		}
	})
}
