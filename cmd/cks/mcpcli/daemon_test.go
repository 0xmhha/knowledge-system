package mcpcli

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/daemon"
)

// TestApplyUpOverrides covers the reachability flags on `daemon up`. The case
// that matters operationally is --port: it used to be dropped without a word,
// because the registry always resolves an address and that address won. An
// operator asking for a port then got the registry's auto-assigned one on the
// registry's default bind — loopback — and no output said so.
func TestApplyUpOverrides(t *testing.T) {
	t.Parallel()
	single := func() *daemon.Registry {
		return &daemon.Registry{Instances: []daemon.RegistryEntry{{Name: "one", Dataset: "/d"}}}
	}
	cases := []struct {
		name      string
		reg       *daemon.Registry
		port      string
		httpAddr  string
		lan       bool
		wantErr   string
		wantBind  string
		wantPort  int
		unchanged bool
	}{
		{
			name: "port pins the single instance's port",
			reg:  single(), port: "8930",
			wantPort: 8930,
		},
		{
			name: "lan widens the bind to every interface",
			reg:  single(), lan: true,
			wantBind: wildcardBind,
		},
		{
			name: "lan and port together — the subnet case this exists for",
			reg:  single(), lan: true, port: "8930",
			wantBind: wildcardBind, wantPort: 8930,
		},
		{
			name: "port against several instances is refused, not silently applied",
			reg: &daemon.Registry{Instances: []daemon.RegistryEntry{
				{Name: "a", Dataset: "/a"}, {Name: "b", Dataset: "/b"},
			}},
			port:    "8930",
			wantErr: "declares 2",
		},
		{
			name: "http-addr is refused rather than ignored",
			reg:  single(), httpAddr: "10.0.0.5:8930",
			wantErr: "does not apply to a registry",
		},
		{
			name: "a nonsense port is refused",
			reg:  single(), port: "not-a-port",
			wantErr: "not a valid port",
		},
		{
			name:      "no flags leaves the registry alone",
			reg:       single(),
			unchanged: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := applyUpOverrides(tc.reg, tc.port, tc.httpAddr, tc.lan)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyUpOverrides: %v", err)
			}
			if tc.unchanged {
				if tc.reg.Bind != "" || tc.reg.Instances[0].Port != 0 {
					t.Errorf("registry mutated without flags: bind=%q port=%d",
						tc.reg.Bind, tc.reg.Instances[0].Port)
				}
				return
			}
			if tc.wantBind != "" && tc.reg.Bind != tc.wantBind {
				t.Errorf("bind = %q, want %q", tc.reg.Bind, tc.wantBind)
			}
			if tc.wantPort != 0 && tc.reg.Instances[0].Port != tc.wantPort {
				t.Errorf("port = %d, want %d", tc.reg.Instances[0].Port, tc.wantPort)
			}
		})
	}
}

// TestAddrForPort pins that moving the port never moves the host — the
// property that keeps a LAN-bound or wildcard-bound instance reachable across
// a restart.
func TestAddrForPort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		configured string
		want       string
		wantErr    bool
	}{
		{configured: "192.168.1.20:8080", want: "192.168.1.20:8930"},
		{configured: ":8080", want: ":8930"}, // wildcard stays wildcard
		{configured: "0.0.0.0:8080", want: "0.0.0.0:8930"},
		{configured: "", want: "127.0.0.1:8930"},    // no address configured
		{configured: "192.168.1.20", wantErr: true}, // set but unparseable
	}
	for _, tc := range cases {
		t.Run(tc.configured, func(t *testing.T) {
			t.Parallel()
			got, err := addrForPort(tc.configured, "8930")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("addrForPort(%q) = %q, want an error", tc.configured, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("addrForPort(%q): %v", tc.configured, err)
			}
			if got != tc.want {
				t.Errorf("addrForPort(%q) = %q, want %q", tc.configured, got, tc.want)
			}
		})
	}
}

func TestLoopbackNotice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr     string
		wantWarn bool
	}{
		{addr: "127.0.0.1:8801", wantWarn: true},
		{addr: "localhost:8801", wantWarn: true},
		{addr: "[::1]:8801", wantWarn: true},
		{addr: "0.0.0.0:8801", wantWarn: false},
		{addr: ":8801", wantWarn: false}, // wildcard: every interface
		{addr: "192.168.1.20:8801", wantWarn: false},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			got := loopbackNotice(tc.addr, "instances.yaml")
			if (got != "") != tc.wantWarn {
				t.Errorf("loopbackNotice(%q) = %q, want warning: %v", tc.addr, got, tc.wantWarn)
			}
			if tc.wantWarn && !strings.Contains(got, "--lan") {
				t.Errorf("notice %q does not tell the operator how to fix it", got)
			}
		})
	}
}
