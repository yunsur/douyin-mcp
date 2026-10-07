package main

// HTTP API and MCP response types.

// ErrorResponse is the JSON error envelope for the HTTP API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

// SuccessResponse is the JSON success envelope for the HTTP API.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Message string `json:"message,omitempty"`
}

// MCPToolResult is the internal tool result shape.
type MCPToolResult struct {
	Content []MCPContent
	IsError bool
}

// MCPContent is an internal tool content block.
type MCPContent struct {
	Type     string
	Text     string
	MimeType string
	Data     string
}

// textResult builds a text-only tool result.
func textResult(v any) *MCPToolResult {
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: jsonText(v)}}}
}

// errorResult builds an error tool result.
func errorResult(msg string) *MCPToolResult {
	return &MCPToolResult{IsError: true, Content: []MCPContent{{Type: "text", Text: msg}}}
}

// imageResult builds a base64 image tool result.
func imageResult(mimeType, data string) *MCPToolResult {
	return &MCPToolResult{Content: []MCPContent{{Type: "image", MimeType: mimeType, Data: data}}}
}
