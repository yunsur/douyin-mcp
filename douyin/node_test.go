package douyin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeExecutable(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// resolveNode 的优先级：DY_NODE > PATH 上的 node > `mise which node`。
func TestResolveNodePrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell shims only")
	}
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit-node")
	writeExecutable(t, explicit, "#!/bin/sh\nexit 0\n")
	onPath := filepath.Join(dir, "node")
	writeExecutable(t, onPath, "#!/bin/sh\nexit 0\n")
	fromMise := filepath.Join(dir, "mise-node")
	writeExecutable(t, fromMise, "#!/bin/sh\nexit 0\n")

	// 1) DY_NODE 优先于 PATH 与 mise。
	miseDir := t.TempDir()
	writeExecutable(t, filepath.Join(miseDir, "mise"), "#!/bin/sh\necho "+fromMise+"\n")
	t.Setenv("PATH", miseDir)
	t.Setenv("DY_NODE", explicit)
	if got := resolveNode(); got != explicit {
		t.Fatalf("DY_NODE 未生效: got=%q want=%q", got, explicit)
	}

	// 2) DY_NODE 指向不存在的文件时视为没有 Node，不再回退。
	t.Setenv("DY_NODE", filepath.Join(dir, "missing"))
	if got := resolveNode(); got != "" {
		t.Fatalf("DY_NODE 无效时不应回退: got=%q", got)
	}

	// 3) PATH 上的 node 优先于 mise。
	t.Setenv("DY_NODE", "")
	both := t.TempDir()
	writeExecutable(t, filepath.Join(both, "node"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(both, "mise"), "#!/bin/sh\necho "+fromMise+"\n")
	t.Setenv("PATH", both)
	if got := resolveNode(); got != filepath.Join(both, "node") {
		t.Fatalf("PATH 上的 node 应为次优先: got=%q", got)
	}

	// 4) node 不在 PATH、只有 mise 时，回退到 `mise which node`。
	onlyMise := t.TempDir()
	writeExecutable(t, filepath.Join(onlyMise, "mise"), "#!/bin/sh\necho "+fromMise+"\n")
	t.Setenv("PATH", onlyMise)
	if got := resolveNode(); got != fromMise {
		t.Fatalf("mise 回退失败: got=%q want=%q", got, fromMise)
	}
	if !NodeAvailable() {
		t.Fatal("NodeAvailable 应为 true（mise 提供 node）")
	}

	// 5) mise 存在但返回空 → 视为没有 Node。
	writeExecutable(t, filepath.Join(onlyMise, "mise"), "#!/bin/sh\nexit 1\n")
	if got := resolveNode(); got != "" {
		t.Fatalf("mise 无 node 时应返回空: got=%q", got)
	}
}
