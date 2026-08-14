package service

import (
	"fmt"
	"net"
	"strings"
)

// URLFor renders the MCP endpoint a client on the same network should use.
// An IPv6 host is bracketed, without which the port cannot be parsed.
func URLFor(host, port string) string {
	if host == "" {
		return ""
	}
	return "http://" + net.JoinHostPort(host, port) + "/mcp"
}

// PickExternal returns the first address a client on the same network could
// use to reach this host, or "" when there is none. Loopback is not reachable
// from anywhere else; link-local (169.254/16, fe80::/10) means the interface
// never got a lease, which is a failed connection rather than a working one.
// IPv6 is accepted only after every IPv4 candidate is exhausted, because this
// address is typed into client configs and an IPv4 literal is what those
// consumers handle without brackets.
//
// Input order is the caller's — interface order — so repeated polls on an
// unchanged host pick the same address instead of flapping between two
// equally valid ones.
func PickExternal(addrs []string) string {
	var v6 string
	for _, a := range addrs {
		ip := parseHost(a)
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			continue
		}
		if ip.To4() != nil {
			return ip.String()
		}
		if v6 == "" {
			v6 = ip.String()
		}
	}
	return v6
}

// parseHost accepts both a bare IP and the "ip/prefix" form net.Interface
// addresses carry.
func parseHost(s string) net.IP {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if ip, _, err := net.ParseCIDR(s); err == nil {
		return ip
	}
	return net.ParseIP(s)
}

// HostAddrs lists the host's addresses in interface order, skipping interfaces
// that are down. This is the production Addrs for LinkWatcher.
func HostAddrs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("service: list interfaces: %w", err)
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue // an interface that vanished mid-walk is not fatal
		}
		for _, a := range addrs {
			out = append(out, a.String())
		}
	}
	return out, nil
}
