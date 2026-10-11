package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// loopbackHostOnly rejects requests whose Host header is not a loopback
// name or address. The server binds 127.0.0.1, so a foreign Host can only
// come from a browser page that rebound its own name to 127.0.0.1 (DNS
// rebinding); such a page must not read the graph or its source text.
func loopbackHostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback Host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = strings.Trim(hostport, "[]")
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}
