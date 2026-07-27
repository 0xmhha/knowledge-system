// Package netutil resolves the address remote agents should use to reach a
// fused-server (system-mcp) instance. It ports the shell `detect_lan_ip` used
// when an operator serves on all interfaces without pinning an explicit host:
// the server needs to advertise a routable IP so a remote Claude Code / agent
// can connect over MCP.
package netutil

import "net"

// AdvertiseHost returns the host a client should connect to for a server bound
// to bindHost. When bindHost is unset or a wildcard (""/"0.0.0.0"/"::"), it
// resolves the machine's default-route source IPv4 (the address a client reaches
// it on), falling back to the first usable interface address and then loopback.
// A concrete bindHost is returned unchanged.
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
	// Prefer the source IP of the default route — the interface that reaches the
	// LAN gateway. On a multi-homed host this picks the NIC an agent actually
	// connects through, matching the shell detect_lan_ip (route -n get default),
	// instead of whichever private address net.InterfaceAddrs happens to list
	// first. No packet is sent: a UDP "connect" only resolves routing locally.
	if ip := defaultRouteIP(); ip != "" {
		return ip
	}
	// Fallback (no default route, e.g. an offline host): first usable IPv4 among
	// the interface addresses.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	return pickLANIP(addrs)
}

// defaultRouteIP returns the local source IPv4 the OS would use to reach an
// off-link destination, or "" when there is no route. The UDP socket is never
// used to send; Dial just binds the local address per the routing table.
func defaultRouteIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	ua, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	ip4 := ua.IP.To4()
	if ip4 == nil || ip4.IsLoopback() {
		return ""
	}
	return ip4.String()
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
