package config

import (
	"strings"
	"testing"
)

func TestCollectionConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  CollectionConfig
		ok   bool
	}{
		{"disabled", CollectionConfig{}, true},
		{"explicit", CollectionConfig{CatalogPath: "/owned/catalog.json", MemberConfigs: []string{"/owned/a.yaml", "/owned/b.yaml"}}, true},
		{"missing catalog", CollectionConfig{MemberConfigs: []string{"/owned/a.yaml"}}, false},
		{"relative catalog", CollectionConfig{CatalogPath: "catalog.json", MemberConfigs: []string{"/owned/a.yaml"}}, false},
		{"missing profiles", CollectionConfig{CatalogPath: "/owned/catalog.json"}, false},
		{"relative profile", CollectionConfig{CatalogPath: "/owned/catalog.json", MemberConfigs: []string{"a.yaml"}}, false},
		{"duplicate profile", CollectionConfig{CatalogPath: "/owned/catalog.json", MemberConfigs: []string{"/owned/a.yaml", "/owned/a.yaml"}}, false},
		{"unclean profile", CollectionConfig{CatalogPath: "/owned/catalog.json", MemberConfigs: []string{"/owned/x/../a.yaml"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Collection = tc.cfg
			if (cfg.Validate() == nil) != tc.ok {
				t.Fatalf("collection validation differs: %+v", tc.cfg)
			}
		})
	}
}

func TestCollectionConfigKnownFieldsRemainStrict(t *testing.T) {
	raw := "version: 1\nlogging: {mode: dev}\ncollection:\n  catalog_path: /owned/catalog.json\n  member_configs: [/owned/a.yaml]\n"
	cfg, err := LoadBytes([]byte(raw))
	if err != nil || cfg.Collection.CatalogPath != "/owned/catalog.json" {
		t.Fatalf("explicit opt-in could not load: %v", err)
	}
	if _, err := LoadBytes([]byte(strings.Replace(raw, "member_configs:", "member_config:", 1))); err == nil {
		t.Fatal("collection typo accepted")
	}
}
