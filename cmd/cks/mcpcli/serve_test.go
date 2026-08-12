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
		{
			// The wildcard form binds every interface. Reading its empty host
			// as "no address" and substituting loopback is the same silent
			// loss of reachability --port exists to prevent.
			name:       "port keeps a wildcard bind on every interface",
			configured: ":8930", port: "8991",
			wantAddr: ":8991", wantTransport: "http",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Listen.HTTPAddr = tc.configured
			if err := overrideListen(cfg, tc.port, tc.httpAddr); err != nil {
				t.Fatalf("overrideListen: %v", err)
			}
			if cfg.Listen.HTTPAddr != tc.wantAddr {
				t.Errorf("HTTPAddr = %q, want %q", cfg.Listen.HTTPAddr, tc.wantAddr)
			}
			if cfg.Listen.Transport != tc.wantTransport {
				t.Errorf("Transport = %q, want %q", cfg.Listen.Transport, tc.wantTransport)
			}
		})
	}
}

// TestOverrideListenRejectsUnparseableAddress pins the failure direction: an
// address that is set but not host:port must stop the launch, because the
// alternative is serving on loopback while the operator believes the
// configured interface is still bound.
func TestOverrideListenRejectsUnparseableAddress(t *testing.T) {
	t.Parallel()
	for _, configured := range []string{"192.168.1.20", "not:a:port"} {
		t.Run(configured, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Listen.HTTPAddr = configured
			err := overrideListen(cfg, "8991", "")
			if err == nil {
				t.Fatalf("overrideListen(%q) = nil error, want a refusal; got addr %q",
					configured, cfg.Listen.HTTPAddr)
			}
			if cfg.Listen.HTTPAddr != configured {
				t.Errorf("config mutated on the error path: HTTPAddr = %q, want %q",
					cfg.Listen.HTTPAddr, configured)
			}
		})
	}
}
