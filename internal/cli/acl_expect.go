package cli

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// accessExpectation is an `rela acl map --expect` file: the access one
// principal is meant to have on the axes acl map reports: read, create,
// update and delete per type, named permissions and relation writes. Rename
// (TKT-4APGVF), field-level access, entity-local face grants and per-face
// relation writes are not compared; docs/webhooks.md lists them. Anything the principal
// holds on those axes that the file does not list is a difference, and so is
// anything listed that it does not hold. An empty file expects no access.
//
//	types:
//	  incident: [read, create, update]  # held on every incident
//	  ticket: [read:some]               # held on some tickets only
//	  beleid@concept: [update]          # held on that face only
//	permissions: [comment.add]
//	relations:
//	  affects:
//	    create: [incident]              # source types
type accessExpectation struct {
	Types       map[string][]string            `yaml:"types"`
	Permissions []string                       `yaml:"permissions"`
	Relations   map[string]map[string][]string `yaml:"relations"`
}

// someSuffix marks a verb held on some entities of a type, not type-wide.
const someSuffix = ":some"

// Capability sections, in report order.
const (
	sectionTypes       = "types"
	sectionPermissions = "permissions"
	sectionRelations   = "relations"
)

var sectionOrder = map[string]int{sectionTypes: 0, sectionPermissions: 1, sectionRelations: 2}

// loadExpectation reads and validates an expectations file. Names are checked
// against the schema, so a typo is an error rather than a permanent
// difference that reads like a missing grant.
func loadExpectation(path string, meta *metamodel.Metamodel) (*accessExpectation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read expectations file: %w", err)
	}
	var exp accessExpectation
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&exp); err != nil && !stderrors.Is(err, io.EOF) {
		// io.EOF is a file that is empty or only comments: it expects no
		// access, which is a legitimate thing to assert.
		return nil, fmt.Errorf("parse expectations file %s: %w", path, err)
	}
	// A second YAML document would otherwise be ignored without a word.
	var extra any
	if err := dec.Decode(&extra); !stderrors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse expectations file %s: it must hold one YAML document", path)
	}
	if err := exp.validate(meta); err != nil {
		return nil, fmt.Errorf("expectations file %s: %w", path, err)
	}
	return &exp, nil
}

// validate checks every name in the file against the schema.
func (x *accessExpectation) validate(meta *metamodel.Metamodel) error {
	var errs []error
	for key, verbs := range x.Types {
		errs = append(errs, validateTypeKey(meta, key, verbs)...)
	}
	for _, perm := range x.Permissions {
		if strings.TrimSpace(perm) == "" || perm == "*" {
			errs = append(errs, fmt.Errorf("permissions: %q is not a permission name", perm))
		}
	}
	for rel, byVerb := range x.Relations {
		def, ok := meta.GetRelationDef(rel)
		if !ok {
			errs = append(errs, fmt.Errorf("relations: unknown relation type %q", rel))
			continue
		}
		for verb, froms := range byVerb {
			if !slices.Contains([]string{"create", "update", "delete"}, verb) {
				errs = append(errs, fmt.Errorf("relations.%s: verb %q is not one of create, update, delete", rel, verb))
			}
			for _, from := range froms {
				if !slices.Contains(def.From, from) {
					errs = append(errs, fmt.Errorf("relations.%s.%s: %q is not a source type of %s", rel, verb, from, rel))
				}
			}
		}
	}
	return stderrors.Join(errs...)
}

// validateTypeKey checks one `types:` entry: `type` or `type@face`, with verbs
// from the map's verb set.
func validateTypeKey(meta *metamodel.Metamodel, key string, verbs []string) []error {
	typ, face, faced := strings.Cut(key, "@")
	if !meta.HasEntityType(typ) {
		return []error{fmt.Errorf("types: unknown entity type %q", typ)}
	}
	var errs []error
	if faced && !slices.Contains(metamodel.FaceOrderOf(meta, typ), face) {
		errs = append(errs, fmt.Errorf("types: %s declares no face %q", typ, face))
	}
	for _, v := range verbs {
		verb, some := strings.CutSuffix(v, someSuffix)
		if verb == string(acl.VerbRename) {
			errs = append(errs, fmt.Errorf("types.%s: rename is not reported by acl map yet (TKT-4APGVF); leave it out", key))
			continue
		}
		if !slices.Contains(aclmap.Verbs(), acl.Verb(verb)) {
			errs = append(errs, fmt.Errorf("types.%s: verb %q is not one of read, create, update, delete", key, v))
			continue
		}
		if some && faced {
			errs = append(errs, fmt.Errorf("types.%s: %q cannot be combined with a face", key, v))
		}
	}
	return errs
}

// capability is one unit of access in the comparison: a verb on a type, a
// permission, or a relation write from one source type.
type capability struct {
	Section string `json:"section"`
	Key     string `json:"key,omitempty"`
	Value   string `json:"value"`
	// Why says what grants it, on a capability the principal holds.
	Why string `json:"why,omitempty"`
}

func (c capability) id() string { return c.Section + "\x00" + c.Key + "\x00" + c.Value }

// String renders the capability as a diff line body.
func (c capability) String() string {
	s := c.Section
	if c.Key != "" {
		s += "." + c.Key
	}
	s += ": " + c.Value
	if c.Why != "" {
		s += " (" + c.Why + ")"
	}
	return s
}

// expectationResult is the outcome of comparing a map with an expectations
// file. Unexpected is held but not listed; Missing is listed but not held.
type expectationResult struct {
	File       string       `json:"file"`
	Match      bool         `json:"match"`
	Unexpected []capability `json:"unexpected"`
	Missing    []capability `json:"missing"`
}

// compare diffs the map against the expectation.
func (x *accessExpectation) compare(file string, res *aclmap.MapPrincipalResult) *expectationResult {
	actual := actualCapabilities(res)
	expected := x.capabilities()
	has := func(set []capability, c capability) bool {
		return slices.ContainsFunc(set, func(o capability) bool { return o.id() == c.id() })
	}
	out := &expectationResult{File: file, Unexpected: []capability{}, Missing: []capability{}}
	for _, c := range actual {
		if !has(expected, c) {
			out.Unexpected = append(out.Unexpected, c)
		}
	}
	for _, c := range expected {
		if !has(actual, c) {
			out.Missing = append(out.Missing, c)
		}
	}
	sortCapabilities(out.Unexpected)
	sortCapabilities(out.Missing)
	out.Match = len(out.Unexpected) == 0 && len(out.Missing) == 0
	return out
}

// capabilities flattens the file into comparable units.
func (x *accessExpectation) capabilities() []capability {
	var out []capability
	for key, verbs := range x.Types {
		for _, v := range verbs {
			out = append(out, capability{Section: sectionTypes, Key: key, Value: v})
		}
	}
	for _, perm := range x.Permissions {
		out = append(out, capability{Section: sectionPermissions, Value: perm})
	}
	for rel, byVerb := range x.Relations {
		for verb, froms := range byVerb {
			for _, from := range froms {
				out = append(out, capability{Section: sectionRelations, Key: rel, Value: verb + " from " + from})
			}
		}
	}
	return dedupeCapabilities(out)
}

// actualCapabilities flattens a map result into comparable units. A verb held
// type-wide is `verb`; one held only through per-entity exceptions is
// `verb:some`, so a grant that widens from some entities to the whole type
// shows as a difference.
func actualCapabilities(res *aclmap.MapPrincipalResult) []capability {
	var out []capability
	for _, ta := range res.Types {
		for verb, routes := range ta.Baseline {
			out = append(out, capability{Section: sectionTypes, Key: ta.Type, Value: verb, Why: joinRoutes(routes)})
		}
		some := map[string]int{}
		for _, ex := range ta.Exceptions {
			for verb := range ex.Extra {
				if _, typeWide := ta.Baseline[verb]; !typeWide {
					some[verb]++
				}
			}
		}
		for verb, n := range some {
			out = append(out, capability{
				Section: sectionTypes, Key: ta.Type, Value: verb + someSuffix,
				Why: fmt.Sprintf("on %d %s entit%s", n, ta.Type, plural(n, "y", "ies")),
			})
		}
		for face, verbs := range ta.Faces {
			for _, verb := range verbs {
				out = append(out, capability{Section: sectionTypes, Key: ta.Type + "@" + face, Value: verb})
			}
		}
	}
	for _, perm := range res.Permissions {
		out = append(out, capability{Section: sectionPermissions, Value: perm})
	}
	for _, ra := range res.Relations {
		for verb, froms := range ra.Verbs {
			for _, from := range froms {
				out = append(out, capability{Section: sectionRelations, Key: ra.Relation, Value: verb + " from " + from})
			}
		}
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func dedupeCapabilities(in []capability) []capability {
	sortCapabilities(in)
	return slices.CompactFunc(in, func(a, b capability) bool { return a.id() == b.id() })
}

// sortCapabilities orders by section, then key, then value, so a diff is
// stable from run to run.
func sortCapabilities(cs []capability) {
	sort.Slice(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.Section != b.Section {
			return sectionOrder[a.Section] < sectionOrder[b.Section]
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Value < b.Value
	})
}

// writeExpectationText prints the comparison: `+` for access held but not
// expected, `-` for access expected but not held.
func writeExpectationText(r *expectationResult) {
	if r.Match {
		out.WriteMessage("Expectations %s: access matches.", r.File)
		return
	}
	out.WriteMessage("Expectations %s: %d difference(s). + held but not expected, - expected but not held.",
		r.File, len(r.Unexpected)+len(r.Missing))
	for _, c := range r.Unexpected {
		out.WriteMessage("  + %s", c)
	}
	for _, c := range r.Missing {
		out.WriteMessage("  - %s", c)
	}
}
