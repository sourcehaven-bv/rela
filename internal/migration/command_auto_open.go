package migration

import (
	"gopkg.in/yaml.v3"
)

func init() {
	Register(&CommandAutoOpenMigration{})
}

// CommandAutoOpenMigration removes `auto_open` from every entry in the
// `commands:` map of data-entry.yaml.
//
// TKT-93FUCV deleted the server-side file launcher the key controlled. A
// command that reports a file now renders a Download button; there is no
// "open it automatically on completion" behavior left for the key to select,
// and nothing anywhere reads it.
//
// This is a removal of a DEAD key, which is the only kind this migration is
// entitled to make. It is not the re-derivable-value removal
// [DataEntryCleanupMigration] performs — there is no value to re-derive,
// because there is no consumer. Removing it cannot change how the project
// behaves; leaving it only preserves a setting whose name promises something
// the server no longer does.
//
// The parser still accepts the key (dataentryconfig.CommandConfig keeps the
// field) so an unmigrated project keeps loading. This migration is the tidy-up,
// not a precondition for starting.
type CommandAutoOpenMigration struct{}

func (m *CommandAutoOpenMigration) Name() string {
	return "command-auto-open"
}

func (m *CommandAutoOpenMigration) Description() string {
	return "Remove the inert auto_open key from data-entry.yaml commands"
}

func (m *CommandAutoOpenMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *CommandAutoOpenMigration) Detect(doc *yaml.Node) bool {
	return m.eachCommand(doc, func(cmd *yaml.Node) bool {
		return GetMapValue(cmd, "auto_open") != nil
	})
}

func (m *CommandAutoOpenMigration) Apply(doc *yaml.Node) error {
	m.eachCommand(doc, func(cmd *yaml.Node) bool {
		DeleteMapKey(cmd, "auto_open")
		return false // visit every command, don't stop at the first
	})
	return nil
}

// eachCommand runs fn over each command definition, stopping early (and
// reporting true) as soon as fn returns true. Apply passes a fn that always
// returns false so every entry is visited.
func (m *CommandAutoOpenMigration) eachCommand(doc *yaml.Node, fn func(cmd *yaml.Node) bool) bool {
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}
	commands := GetMapValue(root, "commands")
	if commands == nil || commands.Kind != yaml.MappingNode {
		return false
	}
	// A mapping node interleaves keys and values, so the definitions are the
	// odd indices.
	for i := 1; i < len(commands.Content); i += 2 {
		cmd := commands.Content[i]
		if cmd.Kind != yaml.MappingNode {
			continue
		}
		if fn(cmd) {
			return true
		}
	}
	return false
}
