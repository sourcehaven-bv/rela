package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// RemoteSchema is the slice of the primary's /api/v1/_schema the sync client
// needs: the type → plural routing map and, per type, each declared property's
// SHAPE (type + list-ness) for the compatibility handshake. It decodes only
// these fields rather than importing the server's apiwire/v1 types — the CLI
// must not depend on the server's wire package (package-boundary rule).
type RemoteSchema struct {
	Entities  map[string]remoteEntityType   `json:"entities"`
	Relations map[string]remoteRelationType `json:"relations"`
}

type remoteEntityType struct {
	Plural     string                   `json:"plural"`
	Properties map[string]remotePropDef `json:"properties"`
}

type remoteRelationType struct {
	Properties map[string]remotePropDef `json:"properties"`
}

// remotePropDef is the shape-relevant slice of a wire PropertyDef: the value
// type and whether it is a list. A drift in either is what silently mangles data
// (e.g. the replica stores a string where the primary expects a number, or a
// scalar where the primary expects a list), so the handshake compares them.
type remotePropDef struct {
	Type string `json:"type"`
	List bool   `json:"list"`
}

// Plural returns the URL plural for an entity type, and whether the type exists
// in the remote schema. The plural is the /api/v1/{plural}/{id} path segment.
func (s *RemoteSchema) Plural(typeName string) (string, bool) {
	et, ok := s.Entities[typeName]
	if !ok || et.Plural == "" {
		return "", false
	}
	return et.Plural, true
}

// Schema fetches the primary's public metamodel once per sync run. It doubles as
// the source for type→plural routing and the compatibility handshake. Config is
// not secret (root CLAUDE.md), so this is an ordinary authorized GET.
func (c *Client) Schema(ctx context.Context) (*RemoteSchema, error) {
	resp, err := c.get(ctx, []string{"api", "v1", "_schema"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp, "fetch schema")
	}
	var s RemoteSchema
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	return &s, nil
}

// CheckSchemaCompatible verifies the LOCAL metamodel is compatible with the
// primary's before any record is synced, failing fast on divergence rather than
// discovering it mid-splice (a real corruption vector: an unknown type, a
// missing property, or a property whose SHAPE drifted can silently mangle data).
// "Compatible" here is deliberately narrow — the two schemas need not be
// identical (the primary may declare types/properties the replica ignores), but
// every type and property the REPLICA knows must exist on the primary with a
// matching plural AND a matching property shape (value type + list-ness), so the
// replica's reads/writes address the right routes and never push a value the
// primary will store wrong.
//
// local is the replica's own type→(plural, property-shapes) view, built from its
// metamodel by the caller (kept as an input so this package does not depend on
// the metamodel package's shape).
func (s *RemoteSchema) CheckSchemaCompatible(local LocalSchema) error {
	var problems []string
	for typeName, lt := range local.Entities {
		rt, ok := s.Entities[typeName]
		if !ok {
			problems = append(problems, fmt.Sprintf("entity type %q exists locally but not on the remote", typeName))
			continue
		}
		if lt.Plural != "" && rt.Plural != "" && lt.Plural != rt.Plural {
			problems = append(problems, fmt.Sprintf(
				"entity type %q has plural %q locally but %q on the remote", typeName, lt.Plural, rt.Plural))
		}
		problems = append(problems, comparePropShapes("entity", typeName, lt.Properties, rt.Properties)...)
	}
	for typeName, lt := range local.Relations {
		rt, ok := s.Relations[typeName]
		if !ok {
			problems = append(problems, fmt.Sprintf("relation type %q exists locally but not on the remote", typeName))
			continue
		}
		problems = append(problems, comparePropShapes("relation", typeName, lt.Properties, rt.Properties)...)
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("sync: local and remote schemas are incompatible; refusing to sync to avoid corruption:\n  - %s",
		strings.Join(problems, "\n  - "))
}

// comparePropShapes reports the compatibility problems for one type's properties:
// each local property must EXIST on the remote and match its shape (value type +
// list-ness). A local-only property (absent remotely) or a drifted type/list is
// a problem; a remote-only property is fine (the replica just ignores it).
func comparePropShapes(kind, typeName string, local map[string]LocalProp, remote map[string]remotePropDef) []string {
	var problems []string
	for name, lp := range local {
		rp, ok := remote[name]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s type %q property %q exists locally but not on the remote", kind, typeName, name))
			continue
		}
		// A non-empty local type that disagrees with the remote is a drift. (Empty
		// local type = the replica declared no type; don't flag, nothing to mangle.)
		if lp.Type != "" && rp.Type != "" && lp.Type != rp.Type {
			problems = append(problems, fmt.Sprintf(
				"%s type %q property %q is type %q locally but %q on the remote", kind, typeName, name, lp.Type, rp.Type))
		}
		if lp.List != rp.List {
			problems = append(problems, fmt.Sprintf(
				"%s type %q property %q is list=%t locally but list=%t on the remote", kind, typeName, name, lp.List, rp.List))
		}
	}
	return problems
}

// LocalSchema is the replica's own metamodel reduced to what the compatibility
// handshake compares: per entity/relation type, the plural and the declared
// property shapes. The CLI wiring builds it from the local metamodel and passes
// it in, so this package stays independent of the metamodel package.
type LocalSchema struct {
	Entities  map[string]LocalType
	Relations map[string]LocalType
}

// LocalType is one type's compatibility-relevant shape: its URL plural (entities
// only) and its declared properties keyed by name to their shape.
type LocalType struct {
	Plural     string
	Properties map[string]LocalProp
}

// LocalProp is a local property's compatibility-relevant shape: value type and
// list-ness, mirroring remotePropDef.
type LocalProp struct {
	Type string
	List bool
}
