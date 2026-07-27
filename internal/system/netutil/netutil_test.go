package netutil

import (
	"net"
	"testing"
)

func TestPickLANIP(t *testing.T) {
	ipnet := func(s string) *net.IPNet { ip, _, _ := net.ParseCIDR(s); return &net.IPNet{IP: ip} }
	cases := []struct {
		name  string
		addrs []net.Addr
		want  string
	}{
		{"prefers private over public", []net.Addr{ipnet("8.8.8.8/32"), ipnet("192.168.0.5/24")}, "192.168.0.5"},
		{"skips loopback and link-local", []net.Addr{ipnet("127.0.0.1/8"), ipnet("169.254.1.1/16"), ipnet("10.0.0.7/8")}, "10.0.0.7"},
		{"falls back to public when no private", []net.Addr{ipnet("203.0.113.9/32")}, "203.0.113.9"},
		{"none", []net.Addr{ipnet("127.0.0.1/8")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickLANIP(c.addrs); got != c.want {
				t.Errorf("pickLANIP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAdvertiseHost(t *testing.T) {
	// concrete host returned unchanged
	if got := AdvertiseHost("example.internal"); got != "example.internal" {
		t.Errorf("concrete host: got %q", got)
	}
	// wildcard resolves to something non-empty (loopback fallback at worst)
	for _, w := range []string{"", "0.0.0.0", "::"} {
		if got := AdvertiseHost(w); got == "" || got == w {
			t.Errorf("wildcard %q resolved to %q, want a concrete host", w, got)
		}
	}
}

func TestAdvertiseHostPort(t *testing.T) {
	if got := AdvertiseHostPort("example.internal:8080"); got != "example.internal:8080" {
		t.Errorf("concrete host:port: got %q", got)
	}
	if got := AdvertiseHostPort("nonsense"); got != "nonsense" {
		t.Errorf("no-port passthrough: got %q", got)
	}
}
