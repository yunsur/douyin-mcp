package douyin

import "testing"

// Reference values captured from the 浏览器抓包.
func TestSecsdkWebSignParity(t *testing.T) {
	cases := []struct {
		url        string
		ts         int64
		uifid      string
		wantQuery  string
		wantSig    string
		wantSigned string
	}{
		{
			url:        "https://www.douyin.com/aweme/v1/web/aweme/post/?device_platform=webapp&aid=6383&sec_user_id=MS4wLjABAAAAtest&whale_cut_token=",
			ts:         1720000000,
			uifid:      "UIFID-ABC",
			wantQuery:  "device_platform=webapp&aid=6383&sec_user_id=MS4wLjABAAAAtest&whale_cut_token=&uifid=UIFID-ABC&timestamp=1720000000",
			wantSig:    "45dd7924a045b5507675713c4620fba2",
			wantSigned: "https://www.douyin.com/aweme/v1/web/aweme/post/?device_platform=webapp&aid=6383&sec_user_id=MS4wLjABAAAAtest&whale_cut_token=&uifid=UIFID-ABC&timestamp=1720000000&x-secsdk-web-signature=45dd7924a045b5507675713c4620fba2",
		},
		{
			url:        "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=7445533736877264178&a+b=x%2By&kw=%E6%A6%B4%E8%8E%B2",
			ts:         1720000001,
			uifid:      "",
			wantQuery:  "aweme_id=7445533736877264178&a b=x%2By&kw=%E6%A6%B4%E8%8E%B2&timestamp=1720000001",
			wantSig:    "d55025710ae9352b0ecb02e2ece98282",
			wantSigned: "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=7445533736877264178&a b=x%2By&kw=%E6%A6%B4%E8%8E%B2&timestamp=1720000001&x-secsdk-web-signature=d55025710ae9352b0ecb02e2ece98282",
		},
	}

	for i, tc := range cases {
		ts, sig, query := SignWeb(tc.url, tc.ts, tc.uifid)
		if ts != tc.ts || sig != tc.wantSig || query != tc.wantQuery {
			t.Errorf("case %d:\n ts=%d sig=%s\nquery=%s", i, ts, sig, query)
		}
		if got := SignWebURL(tc.url, tc.ts, tc.uifid); got != tc.wantSigned {
			t.Errorf("case %d signed url:\n got %s\nwant %s", i, got, tc.wantSigned)
		}
		// Signing must be idempotent over an already signed URL.
		if got := SignWebURL(tc.wantSigned, tc.ts, tc.uifid); got != tc.wantSigned {
			t.Errorf("case %d not idempotent:\n got %s\nwant %s", i, got, tc.wantSigned)
		}
	}
}

func TestIsProtectedPath(t *testing.T) {
	if !IsProtectedPath("/aweme/v1/web/aweme/post/", "GET") {
		t.Error("aweme/post should be protected")
	}
	if IsProtectedPath("/aweme/v1/web/comment/list/", "GET") {
		t.Error("comment/list must not be signed")
	}
	if !IsProtectedPath("/aweme/v1/web/aweme/post/", "POST") {
		t.Error("aweme/post POST should be protected")
	}
}
