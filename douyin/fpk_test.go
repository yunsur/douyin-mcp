package douyin

import "testing"

// The component string is built from the same captured challenge_profile.json,
// so the digest is a byte-for-byte comparison; fpk1 uses a fixed salt for
// determinism. The profile ships neutralized (generic GPU / 1920x1080 / 8 cores),
// so these literals change whenever the profile's component values change.
func TestFingerprintParity(t *testing.T) {
	if got := MurmurX64Hash128("abc", 0); got != "b4963f3f3fad78673ba2744126ca2d52" {
		t.Fatalf("murmurX64Hash128(abc) = %s", got)
	}

	digest, err := BuildFingerprintDigest()
	if err != nil {
		t.Fatal(err)
	}
	if digest != "0d21d32a3222d109fae3f27d062753e8" {
		t.Fatalf("fingerprint digest = %s, want 0d21d32a3222d109fae3f27d062753e8", digest)
	}

	salt := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	fpk1, err := BuildFpk1(digest, salt)
	if err != nil {
		t.Fatal(err)
	}
	want := "U2FsdGVkX18AAQIDBAUGB84DAb84oZd7BnHoRN4hvdH8wXBdxSfj4dsKZMu/EipHwqJQM2xaZ1JgT6kR+IKwaA=="
	if fpk1 != want {
		t.Fatalf("fpk1 mismatch:\n got %s\nwant %s", fpk1, want)
	}
}

func TestFpk2IsMD5OfUA(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/151.0.0.0"
	if got, want := BuildFpk2(ua), MD5Hex(ua); got != want {
		t.Fatalf("fpk2 = %s want %s", got, want)
	}
	if len(BuildFpk2(ua)) != 32 {
		t.Fatal("fpk2 must be 32 lowercase hex chars")
	}
}

func TestBuildFpk1RejectsBadInput(t *testing.T) {
	if _, err := BuildFpk1("nothex", make([]byte, 8)); err == nil {
		t.Fatal("非 32 位十六进制应被拒绝")
	}
	if _, err := BuildFpk1("0d21d32a3222d109fae3f27d062753e8", []byte{1, 2, 3}); err == nil {
		t.Fatal("salt 长度不是 8 应被拒绝")
	}
	got, err := BuildFpk1Random()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 40 {
		t.Fatalf("fpk1 输出过短: %q", got)
	}
}
