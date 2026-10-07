package douyin

import (
	"strconv"
	"testing"
	"time"
)

// Regression for upstream PR #86 / issue #85: the follower/following list
// endpoints must not send max_time=0, which makes the server take its
// "recommend" branch and return an empty list with status_code=0.
func TestResolveRelationMaxTime(t *testing.T) {
	for _, in := range []string{"", "0"} {
		got := resolveRelationMaxTime(in)
		n, err := strconv.ParseInt(got, 10, 64)
		if err != nil {
			t.Fatalf("resolveRelationMaxTime(%q) = %q, not a timestamp", in, got)
		}
		if n < time.Now().Unix()-60 {
			t.Fatalf("resolveRelationMaxTime(%q) = %d, not current epoch seconds", in, n)
		}
	}
	if got := resolveRelationMaxTime("1700000000"); got != "1700000000" {
		t.Fatalf("explicit timestamp must pass through, got %q", got)
	}
	if got := resolveRelationMaxTime("1700000001"); got == "0" {
		t.Fatal("non-zero timestamp must not be replaced")
	}
}
