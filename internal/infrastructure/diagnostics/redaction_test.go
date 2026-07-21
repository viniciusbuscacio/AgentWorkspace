package diagnostics

import (
	"strings"
	"testing"
)

func TestRedactTextStripsIdentifiers(t *testing.T) {
	cases := []struct {
		in      string
		absent  []string
		present []string
	}{
		{
			in:     "Adapter 00:1A:2B:3C:4D:5E went down at 192.168.1.42",
			absent: []string{"00:1A:2B:3C:4D:5E", "192.168.1.42"},
		},
		{
			in:     "Profile loaded from C:\\Users\\Vinicius\\AppData and /home/vinicius/.config",
			absent: []string{"Vinicius", "vinicius"},
		},
		{
			in:     "Device {12345678-1234-1234-1234-123456789abc} signed by user@example.com",
			absent: []string{"12345678-1234-1234-1234-123456789abc", "user@example.com"},
		},
		{
			in:      "Token blob deadbeefdeadbeefcafe fetched from https://x.test/p?token=secret",
			absent:  []string{"deadbeefdeadbeefcafe", "token=secret"},
			present: []string{"https://x.test/p"},
		},
	}
	for _, c := range cases {
		got := redactText(c.in)
		for _, a := range c.absent {
			if strings.Contains(got, a) {
				t.Errorf("redactText(%q) still contains %q: %q", c.in, a, got)
			}
		}
		for _, p := range c.present {
			if !strings.Contains(got, p) {
				t.Errorf("redactText(%q) dropped %q: %q", c.in, p, got)
			}
		}
	}
}

func TestRedactMessageTruncates(t *testing.T) {
	long := strings.Repeat("x", eventMessageCap+500)
	got := redactMessage(long)
	if len(got) <= eventMessageCap {
		// truncated form is cap + suffix; ensure it did not exceed wildly
		t.Fatalf("expected truncation marker, got len %d", len(got))
	}
	if !strings.HasSuffix(got, "[truncated]") {
		t.Errorf("expected truncated suffix, got tail %q", got[len(got)-20:])
	}
}

func TestRedactDeviceNameGeneralizesPersonal(t *testing.T) {
	if got := redactDeviceName("John's iPhone"); got != "[user]'s iPhone" {
		t.Errorf("redactDeviceName personal = %q", got)
	}
	if got := redactDeviceName("Intel(R) Wireless-AC 9560"); got != "Intel(R) Wireless-AC 9560" {
		t.Errorf("redactDeviceName generic should be unchanged, got %q", got)
	}
}
