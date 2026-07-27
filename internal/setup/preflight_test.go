package setup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreflightOllama(t *testing.T) {
	tagsSrv := func(models ...string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/tags" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var b strings.Builder
			b.WriteString(`{"models":[`)
			for i, m := range models {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(`{"name":"` + m + `"}`)
			}
			b.WriteString(`]}`)
			_, _ = w.Write([]byte(b.String()))
		}))
	}

	t.Run("model present (implicit latest)", func(t *testing.T) {
		s := tagsSrv("bge-m3:latest", "llama3:latest")
		defer s.Close()
		if err := PreflightOllama(s.URL, "bge-m3", nil); err != nil {
			t.Errorf("want ok, got %v", err)
		}
	})

	t.Run("model absent", func(t *testing.T) {
		s := tagsSrv("llama3:latest")
		defer s.Close()
		err := PreflightOllama(s.URL, "bge-m3", nil)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("want not-found error, got %v", err)
		}
	})

	t.Run("empty model = reachability only", func(t *testing.T) {
		s := tagsSrv("anything:latest")
		defer s.Close()
		if err := PreflightOllama(s.URL, "", nil); err != nil {
			t.Errorf("want ok (reachability), got %v", err)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		s := tagsSrv("bge-m3:latest")
		s.Close() // close so the endpoint refuses connections
		err := PreflightOllama(s.URL, "bge-m3", nil)
		if err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Errorf("want unreachable error, got %v", err)
		}
	})
}
