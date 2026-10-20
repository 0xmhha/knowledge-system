package config

import "github.com/0xmhha/knowledge-system/internal/system/maintenance"

// MaintenanceConfig only registers operator-owned setup profiles. Tool callers
// receive IDs and pins, never the setup paths or an arbitrary execution surface.
type MaintenanceConfig struct {
	Profiles []maintenance.Profile `yaml:"profiles,omitempty"`
}

func (c MaintenanceConfig) Validate() error {
	if len(c.Profiles) > 32 {
		return &maintenance.Error{Code: "unsupported_capability"}
	}
	seen := map[string]bool{}
	for _, profile := range c.Profiles {
		if err := profile.Validate(); err != nil {
			return err
		}
		if seen[profile.ID] {
			return &maintenance.Error{Code: "profile_unknown"}
		}
		seen[profile.ID] = true
	}
	return nil
}
