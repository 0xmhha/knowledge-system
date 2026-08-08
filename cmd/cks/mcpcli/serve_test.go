package mcpcli

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

// TestOverrideListen covers the launch-time address overrides. The case that
// matters operationally is --port against a config bound to a routable
// address: moving the port must not silently drop the instance back onto
// loopback, or a LAN-reachable deployment becomes unreachable on restart.
func TestOverrideListen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		configured    string
		port          string
		httpAddr      string
		wantAddr      string
		wantTransport string
	}{
		{
			name:       "port keeps the configured host",
			configured: "192.168.1.20:8930", port: "8991",
			wantAddr: "192.168.1.20:8991", wantTransport: "http",
		},
		{
			name:     "port on an address-less config lands on loopback",
			port:     "8991",
			wantAddr: "127.0.0.1:8991", wantTransport: "http",
		},
		{
			name:       "http-addr names the whole address",
			configured: "192.168.1.20:8930", httpAddr: "10.0.0.5:9000",
			wantAddr: "10.0.0.5:9000", wantTransport: "http",
		},
		{
			name:       "no override leaves the config alone",
			configured: "127.0.0.1:8080",
			wantAddr:   "127.0.0.1:8080", wantTransport: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Listen.HTTPAddr = tc.configured
			overrideListen(cfg, tc.port, tc.httpAddr)
			if cfg.Listen.HTTPAddr != tc.wantAddr {
				t.Errorf("HTTPAddr = %q, want %q", cfg.Listen.HTTPAddr, tc.wantAddr)
			}
			if cfg.Listen.Transport != tc.wantTransport {
				t.Errorf("Transport = %q, want %q", cfg.Listen.Transport, tc.wantTransport)
			}
		})
	}
}
