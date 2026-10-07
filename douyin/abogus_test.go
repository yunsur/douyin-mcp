package douyin

import "testing"

// Reference values captured with the deterministic fixture environment:
//
//	DY_ABOGUS_FIXED_RAND=0.4142135623730951
//	DY_ABOGUS_FIXED_NOW=1720000000000
//	DY_ABOGUS_FIXED_COUNTER=2
//
// a_bogus embeds the window geometry, so the capture-time screen (2560x1440)
// is pinned here: the shipped defaults are neutralized (1920x1080), and the
// vectors below stay byte-for-byte comparable with the capture.
func TestAbogusParityWithReference(t *testing.T) {
	t.Setenv("DY_ABOGUS_FIXED_RAND", "0.4142135623730951")
	t.Setenv("DY_ABOGUS_FIXED_NOW", "1720000000000")
	t.Setenv("DY_ABOGUS_FIXED_COUNTER", "2")
	t.Setenv("DY_FP_SCREEN_WIDTH", "2560")
	t.Setenv("DY_FP_SCREEN_HEIGHT", "1440")
	profileCache = nil
	t.Cleanup(func() { profileCache = nil })

	cases := []struct {
		query, body, host, want string
	}{
		{"device_platform=webapp&aid=6383", "", "www.douyin.com",
			"df0fgq6idxW5cdMSuOTiH1nlrHnMNsWyGMi/bCol9PL7bwUTXbYeYOcWaxo8bMdfpWpwiFV7ZDTMYnncF07TZCHkLmpDSmwWkUA5V66oZ1wXbMiQLNfBCw8LeJtbWOvEmAojJ1UlWtmO2dC4LpaTUBlytApismipQHabdc4aE9ef6zT9Bqq2uxSdO7zqwj=="},
		{"aweme_id=7445533736877264178&request_source=600&msToken=abc", "", "www.douyin.com",
			"df0fgq6idxW5cdMSuOTiH1nlrHnMNsWyNMi/WcAl9NE7bwUbXbYeYOcWaxo8bMdfpWpwiFV7ZDTMYnncF07TZCHkLmpDSmwWkUA5V66oZ1wXbMiQLNfBCw8LeJtbWOvEmAojJ1UlWtmO2dC4LpaTUBlytApismipQHabdc4aE9ef6zT9Bqq2uxSdO7zquf=="},
		{"device_platform=webapp&aid=6383&keyword=%E6%A6%B4%E8%8E%B2", "a=1&b=%2F", "www.douyin.com",
			"df0fgq6idxW5cdMSuOTiH1nlrHnMNsWyYMi/bbAl9xL1bwUYXbYeYunWaPo8bMdfpWpwiFV7ZDYqYnnOF07a8CHkLmpDSmwWkUA5V66oZ1wXbMiQLNfBCw8LeJtbWOvEmAojJ1UlWtmO2dC4LpaTUBlytApismipQHabdc4aE9ef6zT9Bqq2uxSdO7zqKE=="},
		{"room_id=123&user_unique_id=456", "", "live.douyin.com",
			"df0fgq6idxW5cdMSuOcEC1nlrHnMNsWyYMi/WSoK9PY3bwlGXbYeYOcWaxqK4MdkpWpwiFV7HDUMYxncF2XT1AHkLmpDSmwWkUA5V66oZ1wXbMiQLNfBCw8LeJtbWOvEmAojJ1UlWtmO2dC4LpaTUBlytApismipQHabdc4aE9ef6zT9Bqq2uxSdO7zqJj=="},
		{"aid=1128&cookie_enabled=true", "", "creator.douyin.com",
			"df0fgq6idxW5cdMSuOcEC1nlrHnMNsWybFi/WSLn9PLHbwlbXrSeYOcWaxqK4MdkpWp9iFV7HDUMYxncF0kTZAHkLmpDSmwWkUA5V66oZ1wXbMiQLNfBCw8LeJtbWOvEmAojJ1UlWtmO2dC4LpaTUBlytApismipQHabdc4aE9ef6zT9Bqq2uxSdO7zq-D=="},
	}

	signer := NewAbogusSigner()
	for i, tc := range cases {
		got := signer.SignQuery(tc.query, tc.body, tc.host)
		if got != tc.want {
			t.Errorf("case %d a_bogus mismatch:\n got %s\nwant %s", i, got, tc.want)
		}
	}
}

func TestSM3Vectors(t *testing.T) {
	if got := hexSM3([]byte("abc")); got != "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0" {
		t.Errorf("SM3(abc) = %s", got)
	}
	abcd := make([]byte, 0, 64)
	for range 16 {
		abcd = append(abcd, 'a', 'b', 'c', 'd')
	}
	if got := hexSM3(abcd); got != "debe9ff92275b8a138604889c18e5a4d6fdb70e5387e5765293dcba39c0c5732" {
		t.Errorf("SM3(abcd*16) = %s", got)
	}
}

// Regression: the live (non-fixture) random path must terminate and produce a
// well-formed token; a self-recursive randFloat previously blew the stack.
func TestAbogusLiveRandomPath(t *testing.T) {
	signer := NewAbogusSigner()
	got := signer.SignQuery("device_platform=webapp&aid=6383", "", "www.douyin.com")
	if len(got) < 180 || len(got)%4 != 0 {
		t.Fatalf("malformed a_bogus (len %d): %s", len(got), got)
	}
	second := signer.SignQuery("device_platform=webapp&aid=6383", "", "www.douyin.com")
	if got == second {
		t.Fatal("signer counter should advance between signatures")
	}
}

func hexSM3(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 0, 64)
	for _, c := range sm3Hash(b) {
		out = append(out, hexdigits[c>>4], hexdigits[c&15])
	}
	return string(out)
}
