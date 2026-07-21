package appcore

import "testing"

func TestSameAddrSet(t *testing.T) {
	cases := []struct {
		name string
		x, y []string
		want bool
	}{
		{"both empty", nil, nil, true},
		{"equal same order", []string{"127.0.0.1:9301", "10.0.0.5:9301"}, []string{"127.0.0.1:9301", "10.0.0.5:9301"}, true},
		{"equal different order", []string{"127.0.0.1:9301", "10.0.0.5:9301"}, []string{"10.0.0.5:9301", "127.0.0.1:9301"}, true},
		{"different length", []string{"127.0.0.1:9301"}, []string{"127.0.0.1:9301", "10.0.0.5:9301"}, false},
		{"same length different member", []string{"127.0.0.1:9301"}, []string{"10.0.0.5:9301"}, false},
		{"duplicate vs distinct", []string{"127.0.0.1:9301", "127.0.0.1:9301"}, []string{"127.0.0.1:9301", "10.0.0.5:9301"}, false},
	}
	for _, c := range cases {
		if got := sameAddrSet(c.x, c.y); got != c.want {
			t.Errorf("%s: sameAddrSet(%v, %v) = %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}
