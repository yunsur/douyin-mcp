package douyin

import (
	"os"
	"testing"
)

// Live check for the mssdk msToken exchange (/web/r/token). The real token is
// longer than the 107-char random fallback, which is the observable proof that
// the exchange produced it.
//
//	DOUYIN_LIVE_TEST=1 go test ./douyin/ -run TestLiveMsTokenExchange -v
func TestLiveMsTokenExchange(t *testing.T) {
	if os.Getenv("DOUYIN_LIVE_TEST") != "1" {
		t.Skip("set DOUYIN_LIVE_TEST=1 to run the live msToken test")
	}
	cookie := liveCookie()
	if cookie == "" {
		t.Skip("no cookie available")
	}
	c, err := NewClient(cookie, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	tok, err := c.PrimeMsToken(ctx)
	if err != nil {
		t.Fatalf("mssdk /web/r/token: %v", err)
	}
	if len(tok) < 120 {
		t.Fatalf("token 太短 (%d)：仍像随机回退值 %q", len(tok), tok[:20])
	}
	if got := c.MsToken(); got != tok {
		t.Fatalf("MsToken() 未复用已换取的真实 token: %q vs %q", got[:12], tok[:12])
	}
	t.Logf("msToken 长度 %d（随机回退为 107）", len(tok))
}
