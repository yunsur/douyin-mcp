package douyin

import (
	"os"
	"testing"
)

// acrawler VMP 的端到端检查：只要能解析到 Node（PATH 或 mise），就必须产出签名。
// 本机 node 由 mise 管理、不在 PATH 上，正是要覆盖的场景。
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveACSignatureRunner -v
func TestLiveACSignatureRunner(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the acrawler runner")
	}
	node := resolveNode()
	if node == "" {
		t.Skip("no Node.js resolvable (PATH / DY_NODE / mise)")
	}
	t.Logf("node = %s", node)

	res, err := GenerateACSignature(t.Context(), ACOptions{
		Nonce:    "0a1b2c3d4e5f6071",
		Cookie:   "ttwid=test; UIFID=abc",
		UA:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36",
		Referrer: "https://www.douyin.com/",
		Strict:   true,
	})
	if err != nil {
		t.Fatalf("GenerateACSignature: %v", err)
	}
	sig, _ := res["sig"].(string)
	if sig == "" {
		t.Fatalf("未产出签名: %v", res)
	}
	t.Logf("sig len=%d provenance=%v variant=%v", len(sig), res["provenance"], res["variant"])
}
