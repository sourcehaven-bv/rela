package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

// UnmarshalYAML decodes the block and refuses an unknown key, a merge key,
// an `enabled` with no value, and an `allow` that is empty or has no value.
// It checks the keys on the node itself, so an alias resolves against the
// whole file and an error names the file's line.
func (c *RemoteMCPConfig) UnmarshalYAML(node *yaml.Node) error {
	if err := checkKeys(node, "remote_mcp", map[string]func(*yaml.Node) error{
		// With no value, enabled would leave the decision to the flag and
		// the environment, which may turn the endpoint on.
		"enabled": func(n *yaml.Node) error { return refuseNull(n, "remote_mcp.enabled", "") },
		"tools": func(n *yaml.Node) error {
			return checkKeys(n, "remote_mcp.tools", map[string]func(*yaml.Node) error{
				"allow": refuseEmptyAllow,
				"deny":  nil,
			})
		},
	}); err != nil {
		return err
	}
	type plain RemoteMCPConfig
	var out plain
	if err := node.Decode(&out); err != nil {
		return fmt.Errorf("remote_mcp: %w", err)
	}
	// The node check above cannot see every form that decodes to an empty
	// list, so check the decoded value too.
	if out.Tools.Allow != nil && len(out.Tools.Allow) == 0 {
		return fmt.Errorf("line %d: remote_mcp.tools.allow is empty%s", node.Line, serveEveryTool)
	}
	*c = RemoteMCPConfig(out)
	return nil
}

const serveEveryTool = ", which would serve every tool; leave it out to mean that, or name the tools"

// refuseEmptyAllow refuses `allow: []`, an `allow:` with no value, and an
// item with no value (`[~]`, a bare `-`, an item that is only a comment).
// Each decodes to a list with fewer names than the operator wrote, and an
// empty list would serve every tool.
func refuseEmptyAllow(n *yaml.Node) error {
	if err := refuseNull(n, "remote_mcp.tools.allow", serveEveryTool); err != nil {
		return err
	}
	v := resolveAlias(n)
	if v.Kind != yaml.SequenceNode {
		return nil // node.Decode reports the type error
	}
	if len(v.Content) == 0 {
		return fmt.Errorf("line %d: remote_mcp.tools.allow is empty%s", n.Line, serveEveryTool)
	}
	for _, item := range v.Content {
		if err := refuseNull(item, "an item of remote_mcp.tools.allow", "; name a tool or remove the item"); err != nil {
			return err
		}
	}
	return nil
}

// refuseNull refuses a value node that is YAML null (`key:`, `~`, `null`).
// The error names the line of n itself, also when n is an alias.
func refuseNull(n *yaml.Node, where, consequence string) error {
	if v := resolveAlias(n); v.Kind == yaml.ScalarNode && v.ShortTag() == "!!null" {
		return fmt.Errorf("line %d: %s has no value%s", n.Line, where, consequence)
	}
	return nil
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
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
// the operator's `enabled: false` without a word. For the same reason it
// refuses a second YAML document, which the decode would skip.
//
// It reads the keys from the parsed node, not from a decode into a map, so a
// duplicate key elsewhere cannot make the check skip. It follows key aliases
// and `<<` merges, which the decode also follows.
func refuseMisspelledRemoteMCP(data []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil // an empty file
		}
		return err
	}
	var next yaml.Node
	if err := dec.Decode(&next); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("line %d: a second YAML document; .rela/config.yaml holds one", next.Line)
	}
	if len(doc.Content) == 0 {
		return nil
	}
	return refuseMisspelledKeys(doc.Content[0])
}

var keyFolder = strings.NewReplacer("_", "", "-", "", " ", "", ".", "")

func refuseMisspelledKeys(m *yaml.Node) error {
	m = resolveAlias(m)
	switch m.Kind {
	case yaml.SequenceNode: // the value of `<<: [*a, *b]`
		for _, item := range m.Content {
			if err := refuseMisspelledKeys(item); err != nil {
				return err
			}
		}
		return nil
	case yaml.MappingNode:
	default:
		return nil // the main decode reports the type error
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		key := resolveAlias(k).Value
		if k.Kind == yaml.ScalarNode && k.ShortTag() == "!!merge" {
			if err := refuseMisspelledKeys(m.Content[i+1]); err != nil {
				return err
			}
			continue
		}
		if key == "remote_mcp" {
			// With no value the block counts as absent, and an indentation
			// slip would make its keys unknown top-level keys, which the
			// lenient decode drops.
			if err := refuseNull(m.Content[i+1], "remote_mcp", "; leave it out, or indent its keys under it"); err != nil {
				return err
			}
			continue
		}
		if keyFolder.Replace(strings.ToLower(key)) == "remotemcp" {
			return fmt.Errorf("line %d: unknown key %q; the remote MCP block is spelled remote_mcp",
				k.Line, key)
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
