package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func pngPayload() []byte {
	raw := []byte("\x89PNG\r\n\x1a\n")
	raw = append(raw, []byte("fake-png-body")...)
	return raw
}

func TestWriteQRCodeImage(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		badDir  bool // 目标路径的父目录不存在
		wantOK  bool
		wantPNG []byte
	}{
		{
			name:    "有效 base64 PNG 写入",
			data:    map[string]any{"qrcode": base64.StdEncoding.EncodeToString(pngPayload())},
			wantOK:  true,
			wantPNG: pngPayload(),
		},
		{
			name: "混杂非字符串与无效项时仍能识别",
			data: map[string]any{
				"count":  1,
				"nil":    nil,
				"broken": "not-base64!!!",
				"png":    base64.StdEncoding.EncodeToString(pngPayload()),
			},
			wantOK:  true,
			wantPNG: pngPayload(),
		},
		{
			name:   "非 PNG 的 base64 被拒绝",
			data:   map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("hello world!"))},
			wantOK: false,
		},
		{
			name:   "非 base64 字符串被拒绝",
			data:   map[string]any{"data": "这不是 base64 @@@@"},
			wantOK: false,
		},
		{
			name:   "空字符串被拒绝",
			data:   map[string]any{"data": ""},
			wantOK: false,
		},
		{
			name:   "空 map 被拒绝",
			data:   map[string]any{},
			wantOK: false,
		},
		{
			name:   "目标目录不存在时不写入",
			data:   map[string]any{"png": base64.StdEncoding.EncodeToString(pngPayload())},
			badDir: true,
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "qr.png")
			if tc.badDir {
				path = filepath.Join(dir, "missing", "qr.png")
			}

			gotPath, ok := writeQRCodeImage(tc.data, path)
			if ok != tc.wantOK {
				t.Fatalf("writeQRCodeImage() ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if gotPath != "" {
					t.Fatalf("失败时返回路径 = %q, want empty", gotPath)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("失败时不应写出文件，Stat err = %v", err)
				}
				return
			}

			if gotPath != path {
				t.Fatalf("返回路径 = %q, want %q", gotPath, path)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != string(tc.wantPNG) {
				t.Fatalf("写出内容 = %q, want %q", got, tc.wantPNG)
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			if perm := fi.Mode().Perm(); perm != 0o644 {
				t.Fatalf("文件权限 = %o, want 644", perm)
			}
		})
	}
}
