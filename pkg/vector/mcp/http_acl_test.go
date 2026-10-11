package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The HTTP transport has no authentication, so only loopback clients may
// reach it.
func TestHTTPHandlerRejectsNonLoopbackClients(t *testing.T) {
	h := NewServer(buildSample(t)).httpHandler()
	for remote, forbidden := range map[string]bool{"10.0.0.5:4321": true, "192.168.1.9:80": true, "127.0.0.1:4321": false, "[::1]:4321": false} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:8080/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if (rec.Code == http.StatusForbidden) != forbidden {
			t.Errorf("client %s: status %d, forbidden=%v", remote, rec.Code, forbidden)
		}
	}
}
