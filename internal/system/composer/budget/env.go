package budget

import (
	"fmt"
	"os"
	"strconv"
)

// EnvMaxBodies names the variable that overrides DefaultMaxCitations for one
// cks process. Evaluation runs use it to compare packs of different sizes on
// the same index; it is not a tuning knob for normal use.
const EnvMaxBodies = "CKS_PACK_MAX_BODIES"

// maxBodiesLimit bounds EnvMaxBodies. Larger packs exceed the token budget
// long before they reach this count.
const maxBodiesLimit = 64

// OptionsFromEnv returns the allocator options set by the environment.
//
// An unset EnvMaxBodies returns no options. A set value n in [1, 64] sets
// MaxCitations to n and scales NeighborReserve with it
// (DefaultNeighborReserve * n / DefaultMaxCitations, rounded down), so a
// smaller pack keeps the default share of neighbor slots instead of giving
// most of its slots to neighbors. Any other value is an error.
func OptionsFromEnv() ([]Option, error) {
	v := os.Getenv(EnvMaxBodies)
	if v == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > maxBodiesLimit {
		return nil, fmt.Errorf("budget: invalid %s %q (want an integer in [1, %d])", EnvMaxBodies, v, maxBodiesLimit)
	}
	cfg := DefaultConfig()
	cfg.MaxCitations = n
	cfg.NeighborReserve = DefaultNeighborReserve * n / DefaultMaxCitations
	return []Option{WithConfig(cfg)}, nil
}
