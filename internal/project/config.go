package project

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the conventional filename for project configuration.
const ConfigFile = "config.yaml"

// Config holds project-level settings stored in .rela/config.yaml.
type Config struct {
	// Formatting configures how markdown content is formatted.
	Formatting FormattingConfig `yaml:"formatting,omitempty"`

	// Validation is the deployment's validation mode: "advisory" (the
	// default) or "strict", which refuses a write with a missing required
	// property, a type mismatch or an invalid value (DEC-OYV2AK). The
	// RELA_VALIDATION environment variable overrides it. Parsed by
	// internal/entitymanager, which owns the meaning.
	Validation string `yaml:"validation,omitempty"`

	// RemoteMCP controls rela-server's HTTP MCP endpoint. rela-server
	// combines it with the -mcp flag and $RELA_MCP; see docs/mcp-server.md.
	RemoteMCP RemoteMCPConfig `yaml:"remote_mcp,omitempty"`
}

// RemoteMCPConfig is the `remote_mcp:` block of .rela/config.yaml.
//
// It is decoded strictly: an unknown key fails the load, and so does a
// top-level key that is `remote_mcp` misspelled. The rest of the file is
// lenient, but a misspelled `enabled` or `deny` here would leave the endpoint
// or a tool on that the operator meant to turn off. Every reader of
// .rela/config.yaml, not only rela-server, then fails on the mistake.
type RemoteMCPConfig struct {
	// Enabled turns the endpoint on (true) or off (false). Nil leaves the
	// decision to the flag and the environment.
	Enabled *bool `yaml:"enabled,omitempty"`

	// Tools limits which tools the endpoint serves.
	Tools RemoteMCPTools `yaml:"tools,omitempty"`
}

// RemoteMCPTools names the tools the remote MCP endpoint serves. With a
// non-empty Allow, only those tools are served; Deny then removes tools.
// Deny wins over Allow.
type RemoteMCPTools struct {
	Allow []string `yaml:"allow,omitempty"`
	Deny  []string `yaml:"deny,omitempty"`
}

// UnmarshalYAML decodes the block and refuses an unknown key, a merge key
// and an empty `allow` list. It checks the keys on the node itself, so an
// alias resolves against the whole file and an error names the file's line.
func (c *RemoteMCPConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := checkKeys(node, "remote_mcp", map[string]func(*yaml.Node) error{
		"enabled": nil,
		"tools": func(n *yaml.Node) error {
			return checkKeys(n, "remote_mcp.tools", map[string]func(*yaml.Node) error{"allow": nil, "deny": nil})
		},
	}); err != nil {
		return err
	}
	type plain RemoteMCPConfig
	var out plain
	if err := node.Decode(&out); err != nil {
		return fmt.Errorf("remote_mcp: %w", err)
	}
	if out.Tools.Allow != nil && len(out.Tools.Allow) == 0 {
		return fmt.Errorf("line %d: remote_mcp.tools.allow is empty, which would serve "+
			"every tool; leave it out to mean that, or name the tools", node.Line)
	}
	*c = RemoteMCPConfig(out)
	return nil
}

// checkKeys refuses a key of the mapping node that keys does not name, and
// runs the check keys holds for each key that has one.
func checkKeys(node *yaml.Node, where string, keys map[string]func(*yaml.Node) error) error {
	for node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	if node.Kind != yaml.MappingNode {
		return nil // node.Decode reports the type error
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i], node.Content[i+1]
		check, ok := keys[k.Value]
		if !ok {
			return fmt.Errorf("line %d: %s has no key %q", k.Line, where, k.Value)
		}
		if check != nil {
			if err := check(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// refuseMisspelledRemoteMCP refuses a top-level key that reads as
// `remote_mcp` but is spelled differently, such as `remote-mcp` or
// `remoteMCP`. The rest of the file is lenient, and such a key would drop
// the operator's `enabled: false` without a word.
func refuseMisspelledRemoteMCP(data []byte) error {
	var top map[string]any
	if err := yaml.Unmarshal(data, &top); err != nil {
		return nil //nolint:nilerr // the main decode reports it
	}
	for key := range top {
		norm := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
		if norm == "remotemcp" && key != "remote_mcp" {
			return fmt.Errorf("unknown key %q; the remote MCP block is spelled remote_mcp", key)
		}
	}
	return nil
}

// FormattingConfig holds settings for markdown formatting.
type FormattingConfig struct {
	// LineWidth is the maximum line width for paragraph wrapping.
	// Default is 80 if not specified.
	LineWidth int `yaml:"line_width,omitempty"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Formatting: FormattingConfig{
			LineWidth: 80,
		},
	}
}

// LoadConfig loads the project configuration from .rela/config.yaml.
// If the file doesn't exist, returns default configuration.
func LoadConfig(ctx *Context) (*Config, error) {
	return LoadConfigFromPath(ctx.ConfigPath())
}

// LoadConfigFromPath loads project configuration from a specific path.
// If the file doesn't exist, returns default configuration.
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := refuseMisspelledRemoteMCP(data); err != nil {
		return nil, err
	}

	// Apply defaults for zero values
	if cfg.Formatting.LineWidth <= 0 {
		cfg.Formatting.LineWidth = 80
	}

	return cfg, nil
}

// ConfigPath returns the path to the project configuration file.
func (c *Context) ConfigPath() string {
	return c.CacheDir + "/" + ConfigFile
}
