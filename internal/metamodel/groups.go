package metamodel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// GroupsFileName is the conventional filename for the project-level
// groups config at the project root.
const GroupsFileName = "groups.yaml"

// Groups maps group names to ordered lists of recipient identities.
// The identities are filename stems that must match .pub files in the
// project's keys/ directory, though that cross-check belongs at the
// wiring site (not in the metamodel package).
//
// The YAML schema is intentionally nested under a top-level `groups:`
// key to leave room for sibling keys (e.g., `schema_version`,
// `metadata`) in future migrations:
//
//	groups:
//	  engineering:
//	    - alice
//	    - bob
//	  exec:
//	    - bob
//	    - charlie
type Groups struct {
	groups map[string][]string
}

// groupsFile is the YAML representation — kept separate from the
// exported Groups type so the in-memory shape can differ from the
// on-disk one if we migrate later.
type groupsFile struct {
	Groups map[string][]string `yaml:"groups"`
}

// LoadGroups reads <projectRoot>/groups.yaml via the provided FS and
// returns the parsed Groups.
//
// Missing file: returns (nil, &GroupError{Kind: GroupErrorNotFound}).
// Callers that tolerate absence check via errors.Is(err, ErrGroupsNotFound).
//
// Duplicate identities within a group return
// (nil, &GroupError{Kind: GroupErrorDuplicate, Group, Identity}).
//
// Unknown YAML fields are rejected (strict decoding) — groups.yaml is
// a greenfield format; typos like misspelled group names should fail loudly
// rather than silently produce an empty group.
func LoadGroups(projectRoot string, fs storage.FS) (*Groups, error) {
	path := filepath.Join(projectRoot, GroupsFileName)
	data, err := fs.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &GroupError{Kind: GroupErrorNotFound}
		}
		return nil, fmt.Errorf("metamodel: read %s: %w", path, err)
	}

	var raw groupsFile
	// Treat an empty file as "no groups defined" — same shape as a
	// file with `groups: {}`. Prevents spurious EOF from the decoder.
	if len(bytes.TrimSpace(data)) > 0 {
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("metamodel: parse %s: %w", path, err)
		}
	}

	for name, identities := range raw.Groups {
		if name == "" {
			return nil, &GroupError{Kind: GroupErrorInvalid, Group: name}
		}
		seen := make(map[string]struct{}, len(identities))
		for _, id := range identities {
			// Strict identity validation: empty, whitespace-only, or
			// surrounding whitespace all indicate a typo — catch them
			// at load time rather than surfacing as "unknown recipient"
			// surprises at the wiring site.
			if id == "" || strings.TrimSpace(id) != id || strings.TrimSpace(id) == "" {
				return nil, &GroupError{
					Kind:     GroupErrorInvalid,
					Group:    name,
					Identity: id,
				}
			}
			if _, dup := seen[id]; dup {
				return nil, &GroupError{
					Kind:     GroupErrorDuplicate,
					Group:    name,
					Identity: id,
				}
			}
			seen[id] = struct{}{}
		}
	}

	return &Groups{groups: raw.Groups}, nil
}

// Recipients returns the ordered identity list for the given group
// and whether the group is defined. The returned slice is backed by
// internal state — do not mutate.
func (g *Groups) Recipients(group string) ([]string, bool) {
	if g == nil {
		return nil, false
	}
	r, ok := g.groups[group]
	return r, ok
}

// Contains reports whether the given group name is defined.
func (g *Groups) Contains(group string) bool {
	if g == nil {
		return false
	}
	_, ok := g.groups[group]
	return ok
}
