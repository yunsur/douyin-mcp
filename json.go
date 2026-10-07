package main

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

// jsonText renders a value as indented JSON for MCP text content.
func jsonText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		logrus.WithError(err).Warn("序列化工具结果失败")
		return "{}"
	}
	return string(b)
}
