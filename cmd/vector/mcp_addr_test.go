package main

import "testing"

func TestLoopbackListenAddr(t *testing.T) {
	ok := map[string]string{":8080": "127.0.0.1:8080", "127.0.0.1:9": "127.0.0.1:9", "localhost:9": "localhost:9", "[::1]:9": "[::1]:9"}
	for in, want := range ok {
		got, err := loopbackListenAddr(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"0.0.0.0:8080", "[::]:8080", "192.168.0.2:80", "example.com:80", "8080"} {
		if got, err := loopbackListenAddr(in); err == nil {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}
