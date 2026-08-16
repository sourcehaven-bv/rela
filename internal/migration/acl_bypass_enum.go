package migration

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&ACLBypassEnumMigration{})
}

// ACLBypassEnumMigration rewrites the boolean `allow_acl_bypass: true` on an
// automation action to the enum spelling `allow_acl_bypass: read+write`
// (TKT-Y3JVFK), and drops `allow_acl_bypass: false` entirely.
//
// The field became `read` | `write` | `read+write` so a document render can
// unlock elevated READS without writes. `true` meant both, so `read+write`
// preserves the exact prior capability.
//
// The parser REFUSES the boolean rather than accepting both spellings, which
// is what makes this migration necessary rather than optional. That was
// deliberate: allow_acl_bypass grants ACL bypass, and a compatibility shim
// mapping a legacy value to the broadest setting is the wrong default for a
// privilege field — it resolves toward more access if the spellings ever
// drift, and nothing ever forces its removal.
//
// `false` is deleted instead of rewritten to `""`: it always meant "no
// elevation", which is the absent-key default, so keeping a key that grants
// nothing is noise.
type ACLBypassEnumMigration struct{}

func (m *ACLBypassEnumMigration) Name() string {
	return "acl-bypass-enum"
}

func (m *ACLBypassEnumMigration) Description() string {
	return "Rewrite boolean allow_acl_bypass to the read/write/read+write enum"
}

// FileTypes covers BOTH files the key can appear in. The schema file is where
// the legacy boolean actually exists (automation actions, TKT-D8T148); the
// data-entry file is where the key is new (documents, TKT-Y3JVFK) and no
// legacy value can exist yet.
//
// data-entry.yaml is listed anyway because an operator who writes
// `allow_acl_bypass: true` there — the natural mistake, since that was the
// spelling for the whole life of TKT-D8T148 — gets a hard config-load error
// whose message says to run `rela migrate`. A migration that skipped the file
// would make that instruction a lie.
func (m *ACLBypassEnumMigration) FileTypes() []FileType {
	return []FileType{FileTypeMetamodel, FileTypeDataEntry}
}

// legacyBools are the YAML 1.1 spellings that decode as booleans. The parser
// rejects every one of them, so every one has to migrate — an operator who
// wrote `yes` is in exactly the same position as one who wrote `true`.
var legacyBools = map[string]bool{
	"true": true, "yes": true, "on": true, "y": true,
	"false": false, "no": false, "off": false, "n": false,
}

// isLegacyBool reports whether v is a boolean spelling, and which one.
// Case-insensitive, matching how a YAML parser reads these.
func isLegacyBool(v string) (truthy, ok bool) {
	t, found := legacyBools[strings.ToLower(strings.TrimSpace(v))]
	return t, found
}

func (m *ACLBypassEnumMigration) Detect(doc *yaml.Node) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}
	for _, entry := range FindMapEntriesByKey(root, "allow_acl_bypass") {
		value := entry[1]
		if value.Kind != yaml.ScalarNode {
			continue
		}
		if _, ok := isLegacyBool(value.Value); ok {
			return true
		}
	}
	return false
}

func (m *ACLBypassEnumMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}

	// Collect the parents that need a key deleted, rather than mutating the
	// tree while walking it.
	var falsyParents []*yaml.Node

	for _, entry := range FindMapEntriesByKey(root, "allow_acl_bypass") {
		value := entry[1]
		if value.Kind != yaml.ScalarNode {
			continue
		}
		truthy, ok := isLegacyBool(value.Value)
		if !ok {
			continue
		}
		if !truthy {
			// Handled below; deleting here would invalidate the walk.
			continue
		}
		value.Value = "read+write"
		value.Tag = "!!str"
		value.Style = 0
	}

	// A falsy value carried no capability, so the key is removed rather than
	// rewritten. Walk mappings to find the owners.
	WalkMappings(root, func(n *yaml.Node) bool {
		v := GetMapValue(n, "allow_acl_bypass")
		if v == nil || v.Kind != yaml.ScalarNode {
			return true
		}
		if truthy, ok := isLegacyBool(v.Value); ok && !truthy {
			falsyParents = append(falsyParents, n)
		}
		return true
	})
	for _, parent := range falsyParents {
		DeleteMapKey(parent, "allow_acl_bypass")
	}

	return nil
}
