package config

import (
	"errors"
	"path/filepath"
)

// CollectionConfig opts into additional, read-only scoped tools. Member
// configs are explicit local cks configurations, each for one pinned version.
// No path or endpoint may be selected by a tool caller.
type CollectionConfig struct {
	CatalogPath   string   `yaml:"catalog_path,omitempty"`
	MemberConfigs []string `yaml:"member_configs,omitempty"`
}

func (c CollectionConfig) Validate() error {
	if c.CatalogPath == "" && len(c.MemberConfigs) == 0 {
		return nil
	}
	if !filepath.IsAbs(c.CatalogPath) || len(c.MemberConfigs) == 0 || len(c.MemberConfigs) > 128 {
		return errors.New("config: collection requires an absolute catalog_path and 1..128 member_configs")
	}
	seen := map[string]bool{}
	for _, name := range c.MemberConfigs {
		if !filepath.IsAbs(name) || filepath.Clean(name) != name || seen[name] {
			return errors.New("config: collection member_configs must be unique absolute paths")
		}
		seen[name] = true
	}
	return nil
}
