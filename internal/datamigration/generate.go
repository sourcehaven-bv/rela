package datamigration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Draft is a generated migration file, ready to be written to
// migrations/<FileName> and REVIEWED by the operator. The guesses are the
// value; the review is the safety — a draft is never applied unseen.
type Draft struct {
	FileName string
	Content  []byte
	// Report is the classified diff the draft was generated from.
	Report metamodel.ShapeReport
}

// Generate drafts a migration from the store's current shape (the marker's
// projection) to the live schema's shape. Only needs-migration deltas
// produce active steps; drift deltas produce commented-out optional
// cleanups (drops, backfills) the operator may enable. Returns nil when the
// shapes are identical or the change is purely additive with no drift — in
// both cases there is nothing worth a file.
func Generate(current, live metamodel.ShapeProjection, existing []*File, description string) (*Draft, error) {
	report := metamodel.CompareShapes(current, live)
	if len(report.Deltas) == 0 {
		return nil, nil //nolint:nilnil // nil draft = shapes identical, nothing to generate (documented)
	}
	if report.Tier() < metamodel.TierDrift {
		return nil, nil //nolint:nilnil // nil draft = purely additive change, nothing worth a file (documented)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Data migration drafted by `rela migrate gen` — REVIEW BEFORE APPLYING.\n")
	fmt.Fprintf(&b, "# Steps marked GUESS were inferred from the schema diff and may be wrong;\n")
	fmt.Fprintf(&b, "# steps marked TODO need values filled in; commented steps are optional\n")
	fmt.Fprintf(&b, "# cleanups that DELETE data when uncommented.\n")
	fmt.Fprintf(&b, "from: %s\n", current.Hash())
	fmt.Fprintf(&b, "to: %s\n", live.Hash())
	if description == "" {
		description = "schema change"
	}
	fmt.Fprintf(&b, "description: %s\n", quoteYAML(description))
	b.WriteString("steps:\n")

	steps, comments := draftSteps(report, live)
	if steps == "" && comments == "" {
		b.WriteString("  []\n")
	} else {
		b.WriteString(steps)
		b.WriteString(comments)
	}

	// The embedded projections make the file self-contained (amendment A2):
	// integrity-checked against the hashes above at parse time.
	fromYAML, err := marshalProjectionYAML("from_projection", current)
	if err != nil {
		return nil, err
	}
	toYAML, err := marshalProjectionYAML("to_projection", live)
	if err != nil {
		return nil, err
	}
	b.WriteString(fromYAML)
	b.WriteString(toYAML)

	name := fmt.Sprintf("%04d-schema-change.yaml", nextIndex(existing))
	content := []byte(b.String())
	// A draft must round-trip through the parser — a generator bug that
	// emits an unparsable file should fail HERE, not when the operator runs
	// `migrate data`.
	if _, err := ParseFile(name, content); err != nil {
		return nil, fmt.Errorf("datamigration: generated draft does not parse (generator bug): %w", err)
	}
	return &Draft{FileName: name, Content: content, Report: report}, nil
}

// draftSteps renders the active steps and the commented optional cleanups.
func draftSteps(report metamodel.ShapeReport, live metamodel.ShapeProjection) (active, commented string) {
	// Subjects consumed by a rename guess must not ALSO get a drop comment.
	renamed := map[string]bool{}
	for _, d := range report.Deltas {
		if d.Kind == "possible_property_rename" || d.Kind == "possible_entity_type_rename" {
			renamed[d.Counterpart] = true
		}
	}
	var act, com strings.Builder
	for _, d := range report.Deltas {
		draftActiveStep(&act, d, live)
		draftCleanupComment(&com, d, live, renamed)
	}
	return act.String(), com.String()
}

// draftActiveStep emits the uncommented (GUESS/TODO) step for one delta,
// if its kind produces one.
func draftActiveStep(w *strings.Builder, d metamodel.ShapeDelta, live metamodel.ShapeProjection) {
	switch d.Kind {
	case "possible_property_rename":
		owner, newProp, ok := splitPropertyKey(d.Subject)
		_, oldProp, ok2 := splitPropertyKey(d.Counterpart)
		if !ok || !ok2 || strings.HasPrefix(d.Subject, "rel:") {
			return // relation property renames: lua territory in v1
		}
		fmt.Fprintf(w, "  # GUESS — confirm this is a rename, not an unrelated remove+add\n")
		fmt.Fprintf(w, "  - rename_property: {entity: %s, from: %s, to: %s}\n", owner, oldProp, newProp)
	case "possible_entity_type_rename":
		fmt.Fprintf(w, "  # GUESS — confirm this is a rename, not an unrelated remove+add\n")
		fmt.Fprintf(w, "  - rename_entity_type: {from: %s, to: %s}\n", d.Counterpart, d.Subject)
	case "enum_values_replaced":
		for _, target := range enumTargets(d.Subject, live) {
			fmt.Fprintf(w, "  # TODO — map each removed value to its replacement\n")
			fmt.Fprintf(w, "  - map_values:\n      entity: %s\n      property: %s\n      mapping:\n",
				target.entity, target.property)
			for i, removedVal := range d.Removed {
				guess := "CHANGEME"
				if i < len(d.Added) {
					guess = d.Added[i]
				}
				fmt.Fprintf(w, "        %s: %s\n", quoteYAML(removedVal), quoteYAML(guess))
			}
		}
	case "property_type_changed", "property_format_changed", "property_list_changed":
		owner, prop, ok := splitPropertyKey(d.Subject)
		if !ok || strings.HasPrefix(d.Subject, "rel:") {
			return
		}
		ps, found := live.Entities[owner].Properties[prop]
		if !found {
			return
		}
		if isCoercible(ps.Type) {
			fmt.Fprintf(w, "  # GUESS — verify the coercion handles your stored values\n")
			fmt.Fprintf(w, "  - convert: {entity: %s, property: %s, to_type: %s}\n", owner, prop, ps.Type)
		} else {
			fmt.Fprintf(w, "  # TODO — no built-in coercion to %q: write migrations/%s-%s.lua\n", ps.Type, owner, prop)
			fmt.Fprintf(w, "  # - lua: {entity: %s, script: migrations/%s-%s.lua}\n", owner, owner, prop)
		}
	case "relation_endpoint_narrowed", "relation_cardinality_tightened", "relation_symmetry_changed":
		fmt.Fprintf(w, "  # TODO — %s: no declarative step can fix this; write a lua step or adjust the data by hand\n", d.Detail)
	}
}

// draftCleanupComment emits the commented-out optional cleanup for one
// deletion delta. Deletions are NEVER emitted live — the operator uncomments.
func draftCleanupComment(
	w *strings.Builder, d metamodel.ShapeDelta, live metamodel.ShapeProjection, renamed map[string]bool,
) {
	switch d.Kind {
	case "property_removed":
		if renamed[d.Subject] || strings.HasPrefix(d.Subject, "rel:") {
			return
		}
		owner, prop, ok := splitPropertyKey(d.Subject)
		if !ok {
			return
		}
		fmt.Fprintf(w, "  # optional cleanup — uncomment to DELETE the orphaned values now (otherwise GC removes them after the grace period)\n")
		fmt.Fprintf(w, "  # - drop_property: {entity: %s, property: %s}\n", owner, prop)
	case "entity_type_removed":
		if renamed[d.Subject] {
			return
		}
		fmt.Fprintf(w, "  # optional cleanup — uncomment to DELETE all %q entities now\n", d.Subject)
		fmt.Fprintf(w, "  # - drop_entities: {type: %s}\n", d.Subject)
	case "relation_type_removed":
		fmt.Fprintf(w, "  # optional cleanup — uncomment to DELETE all %q relations now\n", trimRelPrefix(d.Subject))
		fmt.Fprintf(w, "  # - drop_relations: {type: %s}\n", trimRelPrefix(d.Subject))
	case "required_property_added":
		owner, prop, ok := splitPropertyKey(d.Subject)
		if !ok || strings.HasPrefix(d.Subject, "rel:") {
			return
		}
		def := live.Entities[owner].Properties[prop].Default
		if def == "" {
			def = "CHANGEME"
		}
		fmt.Fprintf(w, "  # optional backfill for the new required property\n")
		fmt.Fprintf(w, "  # - set_default: {entity: %s, property: %s, value: %s}\n", owner, prop, quoteYAML(def))
	}
}

// enumTarget is one (entity, property) pair affected by an enum value change.
type enumTarget struct{ entity, property string }

// enumTargets resolves an enum delta subject to concrete entity properties:
// an inline enum subject ("task.state") is itself the target; a named-type
// subject ("type:status") fans out to every entity property of that type.
func enumTargets(subject string, live metamodel.ShapeProjection) []enumTarget {
	if strings.HasPrefix(subject, "rel:") {
		return nil // relation property enums: lua territory in v1
	}
	if typeName, isNamed := strings.CutPrefix(subject, "type:"); isNamed {
		var targets []enumTarget
		for _, entName := range sortedShapeKeys(live.Entities) {
			for _, propName := range sortedPropKeys(live.Entities[entName].Properties) {
				if live.Entities[entName].Properties[propName].Type == typeName {
					targets = append(targets, enumTarget{entity: entName, property: propName})
				}
			}
		}
		return targets
	}
	owner, prop, ok := splitPropertyKey(subject)
	if !ok {
		return nil
	}
	return []enumTarget{{entity: owner, property: prop}}
}

func isCoercible(toType string) bool {
	switch toType {
	case "string", "integer", "boolean", "date", "datetime":
		return true
	}
	return false
}

// marshalProjectionYAML renders one embedded projection as a top-level YAML
// key, indented under it.
func marshalProjectionYAML(key string, p metamodel.ShapeProjection) (string, error) {
	m, err := projectionToYAML(p)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", key)
	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// quoteYAML renders a scalar safely for the hand-built parts of the draft.
func quoteYAML(s string) string {
	var n yaml.Node
	n.SetString(s)
	out, err := yaml.Marshal(&n)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimRight(string(out), "\n")
}

// nextIndex picks the next chain index: one past the highest numeric prefix
// among the existing files (count-based numbering would collide after a file
// is deleted from the middle of the chain).
func nextIndex(existing []*File) int {
	highest := 0
	for _, f := range existing {
		digits := 0
		for digits < len(f.Name) && f.Name[digits] >= '0' && f.Name[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			continue
		}
		if n, err := strconv.Atoi(f.Name[:digits]); err == nil && n > highest {
			highest = n
		}
	}
	return highest + 1
}

// sortedShapeKeys/sortedPropKeys keep draft output deterministic.
func sortedShapeKeys(m map[string]metamodel.EntityShape) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedPropKeys(m map[string]metamodel.PropertyShape) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
