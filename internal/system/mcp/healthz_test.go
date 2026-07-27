package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
)

func TestHealthzHandler(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		setup    func(*fixture)
		wantCode int
		wantOK   bool
	}{
		{
			name:     "serviceable → 200",
			setup:    nil, // fixture defaults are both-reachable
			wantCode: http.StatusOK,
			wantOK:   true,
		},
		{
			name: "ckv model down → 503",
			setup: func(f *fixture) {
				f.ckv.HealthVal = ckvclient.Health{Reachable: true, ModelReachable: false}
			},
			wantCode: http.StatusServiceUnavailable,
			wantOK:   false,
		},
		{
			name: "ckg down → 503",
			setup: func(f *fixture) {
				f.ckg.HealthVal = ckgclient.Health{Reachable: false}
			},
			wantCode: http.StatusServiceUnavailable,
			wantOK:   false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, tc.setup)
			f.deps.InstanceName = "probe-me"

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			healthzHandler(f.deps)(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			var body struct {
				Status      string `json:"status"`
				Serviceable bool   `json:"serviceable"`
				Name        string `json:"name"`
				Reason      string `json:"reason"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Serviceable != tc.wantOK || body.Name != "probe-me" {
				t.Errorf("body = %+v, want serviceable=%v name=probe-me", body, tc.wantOK)
			}
			if !tc.wantOK && body.Reason == "" {
				t.Errorf("expected a reason when unavailable")
			}
		})
	}
}
