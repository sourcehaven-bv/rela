package configedit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// schedulesFile holds the capture tasks. It is operator-only: the Configure
// space never writes it.
const schedulesFile = "schedules.yaml"

// A capture task in schedules.yaml (TKT-6OYQO79) computes a dashboard card
// as its run_as identity and stores the figure where every reader of the
// owner entity reads it. The operator chooses that owner's read rule for the
// card as it stands. So the card is a data-flow definition with the trust of
// schedules.yaml, and a Configure save may not change it: not the cards with
// that title on that dashboard, not the list they read, and not the parts of
// the schema the card reads (see capturedCard). Without this, an editor could point a captured card at data only
// run_as reads and read the stored figure. The operator changes a captured
// card in the files directly.

// captureRef is the part of a capture task this check needs.
type captureRef struct {
	Name    string `yaml:"name"`
	Capture *struct {
		Dashboard string `yaml:"dashboard"`
		Card      string `yaml:"card"`
	} `yaml:"capture"`
}

// captureProblems refuses a save that changes what a capture task computes.
// When schedules.yaml cannot be read or parsed the save is refused, because
// a running scheduler may still hold capture tasks from an earlier version.
func (s *Service) captureProblems(cur *current, p *prepared) []Problem {
	raw, err := s.deps.Files.ReadFile(schedulesFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	var file struct {
		Tasks []captureRef `yaml:"tasks"`
	}
	if err == nil {
		err = yaml.Unmarshal(raw, &file)
	}
	if err != nil {
		return []Problem{{
			Code: "captured_card", File: DataEntryFile,
			Message: fmt.Sprintf("%s could not be read (%v), so it is not known which dashboard cards a "+
				"capture task stores. Fix %s, or make this change in the files directly.", schedulesFile, err,
				schedulesFile),
		}}
	}
	var out []Problem
	for _, t := range file.Tasks {
		if t.Capture == nil {
			continue
		}
		before := capturedCard(cur.schema, cur.dataEntry, t.Capture.Dashboard, t.Capture.Card)
		after := capturedCard(p.schema, p.dataEntry, t.Capture.Dashboard, t.Capture.Card)
		if before == after {
			continue
		}
		out = append(out, Problem{
			Code: "captured_card", File: DataEntryFile, Value: t.Capture.Card,
			Message: fmt.Sprintf("The card %q is stored by the capture task %q in %s, so this card, its list "+
				"and the schema it reads are changed in the files directly.", t.Capture.Card, t.Name, schedulesFile),
		})
	}
	return out
}

// capturedCard returns the definition of the card titled title on dashboard,
// as one canonical JSON text: every card with that title, the list each one
// reads, and from the schema the property each card folds, every query
// scope and the faces of the list's type and of each type it extends (a
// list that names no scope reads the type's default one), the worlds and default world a capture resolves
// faces in, and every property and relation definition whose name appears
// in the card, its list, the folded property or those scopes (a filter, a
// condition, a related(...) traversal), followed transitively. Two files that give the same text compute the same figure.
// Parts that do not exist are left out, so a definition that disappears also
// counts as a change. Matching names as words over-approximates what the
// expressions read, which can only refuse more saves, never fewer.
func capturedCard(schema, dataEntry []byte, dashboard, title string) string {
	var de, sc map[string]any
	_ = yaml.Unmarshal(dataEntry, &de) // the Validator has accepted both files
	_ = yaml.Unmarshal(schema, &sc)

	// A named dashboard is under dashboards:; the default one (an empty
	// name, or the reserved name no dashboards: key may take) is not.
	dash := mapAt(de, "dashboard")
	if named, ok := mapAt(de, "dashboards")[dashboard]; ok {
		dash, _ = named.(map[string]any)
	}
	type part struct {
		Card     any            `json:"card"`
		List     any            `json:"list,omitempty"`
		Property any            `json:"property,omitempty"`
		Lineage  map[string]any `json:"lineage,omitempty"`
		Named    map[string]any `json:"named,omitempty"`
	}
	var parts []part
	cards, _ := dash["cards"].([]any)
	for _, c := range cards {
		card, _ := c.(map[string]any)
		if t, _ := card["title"].(string); t != title {
			continue
		}
		pt := part{Card: card}
		listName, _ := card["list"].(string)
		list := mapAt(mapAt(de, "lists"), listName)
		if list != nil {
			pt.List = list
		}
		typeName, _ := list["entity_type"].(string)
		def := mapAt(mapAt(sc, "entities"), typeName)
		if prop, _ := card["property"].(string); prop != "" {
			pt.Property = mapAt(mapAt(def, "properties"), prop)
		}
		pt.Lineage = lineage(sc, typeName)
		pt.Named = namedDefinitions(sc, pt.Card, pt.List, pt.Property, pt.Lineage)
		parts = append(parts, pt)
	}
	if len(parts) == 0 {
		return ""
	}
	text, _ := json.Marshal(struct {
		Parts        []part `json:"parts"`
		Worlds       any    `json:"worlds,omitempty"`
		DefaultWorld any    `json:"default_world,omitempty"`
	}{parts, sc["worlds"], sc["default_world"]}) // maps marshal with sorted keys
	return string(text)
}

// namedDefinitions returns the schema property definitions (of any entity
// type, keyed "type.property") and relation definitions (keyed
// "relation:name") whose names appear as a word in the given parts.
//
// It follows names transitively: a definition it adds (a computed
// expression, a rollup's relation and property) is searched in turn, until
// no new name appears.
func namedDefinitions(sc map[string]any, parts ...any) map[string]any {
	out := map[string]any{}
	for pending := leafText(parts); pending != ""; {
		var added []any
		for typeName, d := range mapAt(sc, "entities") {
			def, _ := d.(map[string]any)
			for prop, pd := range mapAt(def, "properties") {
				key := typeName + "." + prop
				if _, seen := out[key]; !seen && containsWord(pending, prop) {
					out[key] = pd
					added = append(added, pd)
				}
			}
		}
		for rel, rd := range mapAt(sc, "relations") {
			key := "relation:" + rel
			if _, seen := out[key]; !seen && containsWord(pending, rel) {
				out[key] = rd
				added = append(added, rd)
			}
		}
		pending = leafText(added)
	}
	return out
}

// lineage returns, for typeName and every type it extends, the keys of its
// raw definition that decide which rows a list over it reads: its query
// scopes and faces (a variant inherits both from its parent), and the
// extends and abstract keys that make it a variant. Keys are type names.
func lineage(sc map[string]any, typeName string) map[string]any {
	out := map[string]any{}
	for name := typeName; name != ""; {
		if _, seen := out[name]; seen {
			break // a cycle; the Validator refuses it anyway
		}
		def := mapAt(mapAt(sc, "entities"), name)
		out[name] = map[string]any{
			"query_scopes": def["query_scopes"],
			"faces":        def["faces"],
			"extends":      def["extends"],
			"abstract":     def["abstract"],
		}
		name, _ = def["extends"].(string)
	}
	return out
}

// leafText joins every string, map key and scalar in v with spaces, so a
// name is searched in the raw text an expression holds, not in an encoding
// of it that could glue an escape such as \n to the name.
func leafText(v any) string {
	var b strings.Builder
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				b.WriteString(k)
				b.WriteByte(' ')
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case nil:
		default:
			fmt.Fprint(&b, x)
			b.WriteByte(' ')
		}
	}
	walk(v)
	return b.String()
}

// containsWord reports whether word occurs in text with no name character
// (letter, digit, '_' or '-') directly before or after it.
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(text[i:], word)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(word)
		if (start == 0 || !isNameByte(text[start-1])) && (end == len(text) || !isNameByte(text[end])) {
			return true
		}
		i = start + 1
	}
}

func isNameByte(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// mapAt returns m[key] as a mapping, or nil.
func mapAt(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}
