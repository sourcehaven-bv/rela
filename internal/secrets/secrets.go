// Package secrets loads per-script secret values from .rela/secrets.yaml.
//
// The file has a flat global section (available to all scripts) and an
// optional "overrides" map keyed by script path. When a script is loaded
// its effective secrets are: global values merged with any per-script
// overrides (overrides win).
//
// Example .rela/secrets.yaml:
//
//	jira_api_key: sk-abc123
//	base_url: https://jira.example.com
//
//	overrides:
//	  reports/sync.lua:
//	    jira_api_key: sk-different-key
//
// The file lives in .rela/ which is gitignored by convention.
package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the name of the secrets file inside .rela/.
const ConfigFile = "secrets.yaml"

// ErrNotFound indicates that .rela/secrets.yaml does not exist.
var ErrNotFound = errors.New("secrets: no .rela/secrets.yaml")

// Load reads .rela/secrets.yaml and returns the resolved secrets for
// the given script path. Global values are merged with per-script
// overrides (overrides take precedence).
//
// Returns ErrNotFound (wrapped) when the file does not exist — callers
// should treat this as "no secrets configured" and pass an empty map.
func Load(relaDir, scriptPath string) (map[string]string, error) {
	raw, err := readFile(relaDir)
	if err != nil {
		return nil, err
	}
	return resolve(raw, scriptPath), nil
}

// rawConfig is the on-disk YAML structure. Top-level keys (except
// "overrides") are global secrets. The "overrides" key maps script
// paths to per-script secret maps.
type rawConfig struct {
	Global    map[string]string            `yaml:",inline"`
	Overrides map[string]map[string]string `yaml:"overrides"`
}

func readFile(relaDir string) (*rawConfig, error) {
	path := filepath.Join(relaDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w", ErrNotFound)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &raw, nil
}

// resolve merges global secrets with per-script overrides.
func resolve(raw *rawConfig, scriptPath string) map[string]string {
	result := make(map[string]string, len(raw.Global))
	for k, v := range raw.Global {
		result[k] = v
	}

	if overrides, ok := raw.Overrides[scriptPath]; ok {
		for k, v := range overrides {
			result[k] = v
		}
	}

	return result
}
