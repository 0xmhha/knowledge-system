// Package netutil resolves the address remote agents should use to reach a
// fused-server (system-mcp) instance. It ports the shell `detect_lan_ip` used
// when an operator serves on all interfaces without pinning an explicit host:
// the server needs to advertise a routable IP so a remote Claude Code / agent
// can connect over MCP.
package netutil

import "net"

// AdvertiseHost returns the host a client should connect to for a server bound
// to bindHost. When bindHost is unset or a wildcard (""/"0.0.0.0"/"::"), it
// resolves the machine's primary private IPv4; if none is found it falls back
// to loopback. A concrete bindHost is returned unchanged.
func AdvertiseHost(bindHost string) string {
	switch bindHost {
	case "", "0.0.0.0", "::", "[::]":
		if ip := primaryLANIP(); ip != "" {
			return ip
		}
		return "127.0.0.1"
	default:
		return bindHost
	}
}

func primaryLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	return pickLANIP(addrs)
}

// pickLANIP chooses an advertisable IPv4 from addrs: a private (RFC1918/ULA)
// address first, else any non-loopback/non-link-local IPv4. Pure and testable.
func pickLANIP(addrs []net.Addr) string {
	var fallback string
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		ip4 := ip.To4()
		if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
			continue
		}
		if ip4.IsPrivate() {
			return ip4.String()
		}
		if fallback == "" {
			fallback = ip4.String()
		}
	}
	return fallback
}

// AdvertiseHostPort applies AdvertiseHost to the host of a "host:port" bind
// address, preserving the port. Returns bindAddr unchanged when it has no port.
func AdvertiseHostPort(bindAddr string) string {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return bindAddr
	}
	return net.JoinHostPort(AdvertiseHost(host), port)
}
