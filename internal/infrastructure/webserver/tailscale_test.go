package webserver

import "testing"

func TestIsTailscalePeer(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"100.64.0.1", true},
		{"100.100.50.25", true},
		{"100.127.255.254", true},
		{"100.63.255.255", false}, // just below the range
		{"100.128.0.0", false},    // just above the range
		{"192.168.1.10", false},
		{"10.0.0.1", false},
		{"127.0.0.1", false},
		{"not-an-ip", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isTailscalePeer(tc.host); got != tc.want {
			t.Errorf("isTailscalePeer(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestHostInAnyCIDR(t *testing.T) {
	nets := parseCIDRs([]string{"192.168.0.0/16", "10.0.0.0/8", "bogus"})
	if len(nets) != 2 {
		t.Fatalf("expected 2 valid CIDRs, got %d", len(nets))
	}
	if !hostInAnyCIDR("192.168.5.5", nets) {
		t.Error("192.168.5.5 should match")
	}
	if !hostInAnyCIDR("10.1.2.3", nets) {
		t.Error("10.1.2.3 should match")
	}
	if hostInAnyCIDR("172.16.0.1", nets) {
		t.Error("172.16.0.1 should not match")
	}
}
