package budget

import (
	"testing"
)

func TestOptionsFromEnv(t *testing.T) {
	cases := []struct {
		name         string
		value        string
		wantOpts     bool
		wantErr      bool
		wantMax      int
		wantNeighbor int
	}{
		{name: "unset keeps defaults", value: ""},
		{name: "default size", value: "12", wantOpts: true, wantMax: 12, wantNeighbor: 2},
		{name: "small pack drops the neighbor reserve", value: "3", wantOpts: true, wantMax: 3, wantNeighbor: 0},
		{name: "half pack keeps one neighbor slot", value: "6", wantOpts: true, wantMax: 6, wantNeighbor: 1},
		{name: "larger pack scales the reserve", value: "24", wantOpts: true, wantMax: 24, wantNeighbor: 4},
		{name: "zero is rejected", value: "0", wantErr: true},
		{name: "above the limit is rejected", value: "65", wantErr: true},
		{name: "not a number is rejected", value: "three", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvMaxBodies, tc.value)
			opts, err := OptionsFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("OptionsFromEnv(%q): want error, got nil", tc.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("OptionsFromEnv(%q): %v", tc.value, err)
			}
			if !tc.wantOpts {
				if len(opts) != 0 {
					t.Fatalf("OptionsFromEnv(%q): want no options, got %d", tc.value, len(opts))
				}
				return
			}
			a, err := New(&FakeFetcher{}, opts...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if a.config.MaxCitations != tc.wantMax || a.config.NeighborReserve != tc.wantNeighbor {
				t.Fatalf("config = max %d neighbor %d, want max %d neighbor %d",
					a.config.MaxCitations, a.config.NeighborReserve, tc.wantMax, tc.wantNeighbor)
			}
			def := DefaultConfig()
			if a.config.MaxTokens != def.MaxTokens || a.config.KnowledgeReserve != def.KnowledgeReserve {
				t.Fatalf("other fields changed: %+v", a.config)
			}
		})
	}
}
