package dataentry

import (
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// PropertyHelp holds documentation for a single property.
type PropertyHelp struct {
	Name        string
	Type        string
	Required    bool
	Description htmltemplate.HTML
}

// RelationHelp holds documentation for a single relation.
type RelationHelp struct {
	Name        string
	Label       string
	TargetType  string // target type for outgoing, source type for incoming
	Cardinality string
	Required    bool // true if min cardinality >= 1
	Description htmltemplate.HTML
}

// EnumHelp documents an enum/state-machine property's allowed values and (when
// it is a state machine) its lifecycle. Property is the property name, TypeName
// the custom-type name (empty for an inline enum). Values / Transitions are only
// populated when present; both may be empty for a plain field, in which case the
// property contributes no help sections (TKT-DUQBD0).
type EnumHelp struct {
	Property    string
	TypeName    string
	Initial     string // entry state for the diagram; "" when unknown
	Values      []ValueHelp
	Transitions []TransitionHelp
}

// ValueHelp documents one allowed value of an enum: the raw value, its optional
// display Label, and its optional prose Description (CustomType.Descriptions).
type ValueHelp struct {
	Value       string
	Label       string
	Description htmltemplate.HTML
}

// TransitionHelp documents one lifecycle move: the target label (the verb), the
// From→To states, and the optional Help prose (why/when to make the move).
type TransitionHelp struct {
	Move string // the move label, falling back to the To value
	From string
	To   string
	Help htmltemplate.HTML
}

// handleEntityHelp returns HTML fragment with documentation for an entity type.
// GET /api/help/{entityType}
func (a *App) handleEntityHelp(w http.ResponseWriter, r *http.Request) {
	entityType := strings.TrimPrefix(r.URL.Path, "/api/help/")
	if entityType == "" {
		http.Error(w, "entity type required", http.StatusBadRequest)
		return
	}

	s := a.State()
	entDef, ok := s.Meta.GetEntityDef(entityType)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Gather property documentation
	props := make([]PropertyHelp, 0, len(entDef.Properties))
	for name, prop := range entDef.Properties {
		ph := PropertyHelp{
			Name:     name,
			Type:     prop.Type,
			Required: prop.Required,
		}
		if prop.Description != "" {
			ph.Description = simpleMarkdownToHTML(prop.Description)
		}
		props = append(props, ph)
	}
	// Sort properties alphabetically
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })

	// Gather outgoing and incoming relations
	outgoingRels := a.gatherRelations(s.Meta, entityType, true)
	incomingRels := a.gatherRelations(s.Meta, entityType, false)

	// Gather enum value + lifecycle help for enum/state-machine properties.
	enums := gatherEnumHelp(s.Meta, entDef)

	// Render entity description
	var entityDesc htmltemplate.HTML
	if entDef.Description != "" {
		entityDesc = simpleMarkdownToHTML(entDef.Description)
	}

	// Generate inline HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	a.renderHelpContent(w, entityDesc, props, outgoingRels, incomingRels, enums)
}

// gatherEnumHelp collects per-value descriptions and lifecycle transitions for
// every enum / state-machine property of entDef, in property-name order
// (TKT-DUQBD0). A property contributes an [EnumHelp] only when its type has
// declared values (a named custom type with Values, or an inline enum); the
// Values / Transitions slices are populated from the resolved CustomType. Plain
// (non-enum) properties are skipped entirely, so they add no help sections.
func gatherEnumHelp(meta *metamodel.Metamodel, entDef *metamodel.EntityDef) []EnumHelp {
	var out []EnumHelp
	names := make([]string, 0, len(entDef.Properties))
	for name := range entDef.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		prop := entDef.Properties[name]
		// Resolve the value set: a named custom type, else an inline enum.
		ct, named := meta.Types[prop.Type]
		values := ct.Values
		labels := ct.Labels
		descriptions := ct.Descriptions
		transitions := ct.Transitions
		if !named {
			// Inline enum: values live on the property; no custom-type-level
			// descriptions or transitions exist for inline enums.
			values = prop.Values
			labels = prop.Labels
			descriptions = nil
			transitions = nil
		}
		if len(values) == 0 {
			continue // not an enum — no help to add
		}

		eh := EnumHelp{Property: name}
		if named {
			eh.TypeName = prop.Type
			// Entry state for the diagram: Initial, else Default.
			eh.Initial = ct.Initial
			if eh.Initial == "" {
				eh.Initial = ct.Default
			}
		}
		for _, v := range values {
			vh := ValueHelp{Value: v, Label: labels[v]}
			if d := descriptions[v]; d != "" {
				vh.Description = simpleMarkdownToHTML(d)
			}
			eh.Values = append(eh.Values, vh)
		}
		for _, tr := range transitions {
			move := tr.Label
			if move == "" {
				move = tr.To
			}
			th := TransitionHelp{Move: move, From: tr.From, To: tr.To}
			if tr.Help != "" {
				th.Help = simpleMarkdownToHTML(tr.Help)
			}
			eh.Transitions = append(eh.Transitions, th)
		}
		out = append(out, eh)
	}
	return out
}

// renderHelpContent generates HTML for entity help content.
func (a *App) renderHelpContent(
	w http.ResponseWriter, entityDesc htmltemplate.HTML, props []PropertyHelp,
	outgoingRels, incomingRels []RelationHelp, enums []EnumHelp,
) {
	fmt.Fprint(w, `<div class="help-content">`)
	if entityDesc != "" {
		fmt.Fprintf(w, `<div class="entity-description">%s</div>`, entityDesc)
	}

	// Properties section
	if len(props) > 0 {
		fmt.Fprint(w, `<h4>Properties</h4><table class="help-table"><thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>`)
		for _, p := range props {
			required := ""
			if p.Required {
				required = "Yes"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(p.Name),
				htmltemplate.HTMLEscapeString(p.Type),
				required,
				p.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	// Outgoing relations section
	if len(outgoingRels) > 0 {
		fmt.Fprint(w, `<h4>Outgoing Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Target</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>`)
		for _, r := range outgoingRels {
			name := r.Name
			if r.Label != "" {
				name = r.Label + " (" + r.Name + ")"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(name),
				htmltemplate.HTMLEscapeString(r.TargetType),
				htmltemplate.HTMLEscapeString(r.Cardinality),
				r.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	// Incoming relations section
	if len(incomingRels) > 0 {
		fmt.Fprint(w, `<h4>Incoming Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Source</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>`)
		for _, r := range incomingRels {
			name := r.Name
			if r.Label != "" {
				name = r.Label + " (" + r.Name + ")"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(name),
				htmltemplate.HTMLEscapeString(r.TargetType),
				htmltemplate.HTMLEscapeString(r.Cardinality),
				r.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	// Values + Lifecycle sections per enum / state-machine property (TKT-DUQBD0).
	for _, e := range enums {
		renderEnumHelp(w, e)
	}

	fmt.Fprint(w, `</div>`)
}

// renderEnumHelp emits the Values table (and, for a state machine, the Lifecycle
// table) for one enum property. Sections with no rows are skipped, so a plain
// enum shows only Values and a value with no description shows just its
// value/label.
func renderEnumHelp(w http.ResponseWriter, e EnumHelp) {
	if len(e.Values) > 0 {
		fmt.Fprintf(w, `<h4>Values: <code>%s</code></h4>`, htmltemplate.HTMLEscapeString(e.Property))
		fmt.Fprint(w, `<table class="help-table"><thead><tr><th>Value</th><th>Description</th></tr></thead><tbody>`)
		for _, v := range e.Values {
			shown := v.Value
			if v.Label != "" {
				shown = v.Label + " (" + v.Value + ")"
			}
			fmt.Fprintf(w, `<tr><td><code>%s</code></td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(shown), v.Description)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}

	if len(e.Transitions) > 0 {
		fmt.Fprintf(w, `<h4>Lifecycle: <code>%s</code></h4>`, htmltemplate.HTMLEscapeString(e.Property))
		// A mermaid state diagram of this field's machine, rendered client-side
		// (the help modal runs renderMermaidDiagrams over the injected HTML). One
		// diagram per state-machine field.
		fmt.Fprintf(w, `<pre class="mermaid">%s</pre>`,
			htmltemplate.HTMLEscapeString(mermaidStateDiagram(e)))
		fmt.Fprint(w, `<table class="help-table"><thead><tr><th>Move</th><th>From &rarr; To</th><th>When to use</th></tr></thead><tbody>`)
		for _, tr := range e.Transitions {
			fmt.Fprintf(w, `<tr><td>%s</td><td><code>%s</code> &rarr; <code>%s</code></td><td>%s</td></tr>`,
				htmltemplate.HTMLEscapeString(tr.Move),
				htmltemplate.HTMLEscapeString(tr.From),
				htmltemplate.HTMLEscapeString(tr.To),
				tr.Help)
		}
		fmt.Fprint(w, `</tbody></table>`)
	}
}

// mermaidStateDiagram builds a stateDiagram-v2 source for one state-machine
// field: an entry arrow to the initial state (when known) plus one edge per
// transition, labeled with the move (verb). The text is HTML-escaped by the
// caller before injection; mermaid identifiers here are the raw enum values,
// which are simple tokens in practice.
func mermaidStateDiagram(e EnumHelp) string {
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	if e.Initial != "" {
		fmt.Fprintf(&b, "    [*] --> %s\n", e.Initial)
	}
	for _, tr := range e.Transitions {
		if tr.Move != "" && tr.Move != tr.To {
			fmt.Fprintf(&b, "    %s --> %s: %s\n", tr.From, tr.To, tr.Move)
		} else {
			fmt.Fprintf(&b, "    %s --> %s\n", tr.From, tr.To)
		}
	}
	return b.String()
}

// gatherRelations collects relation documentation for an entity type.
// If outgoing is true, gathers relations where entityType is in "from";
// otherwise gathers relations where entityType is in "to".
func (a *App) gatherRelations(meta *metamodel.Metamodel, entityType string, outgoing bool) []RelationHelp {
	rels := make([]RelationHelp, 0, len(meta.Relations))
	for name, rel := range meta.Relations {
		var matchTypes, targetTypes []string
		var minCard, maxCard *int
		if outgoing {
			matchTypes, targetTypes = rel.From, rel.To
			minCard, maxCard = rel.MinOutgoing, rel.MaxOutgoing
		} else {
			matchTypes, targetTypes = rel.To, rel.From
			minCard, maxCard = rel.MinIncoming, rel.MaxIncoming
		}
		if !containsString(matchTypes, entityType) {
			continue
		}
		rh := RelationHelp{
			Name:        name,
			Label:       rel.Label,
			TargetType:  strings.Join(targetTypes, ", "),
			Cardinality: formatCardinality(minCard, maxCard),
			Required:    minCard != nil && *minCard >= 1,
		}
		if rel.Description != "" {
			rh.Description = simpleMarkdownToHTML(rel.Description)
		}
		rels = append(rels, rh)
	}
	sort.Slice(rels, func(i, j int) bool { return rels[i].Name < rels[j].Name })
	return rels
}

// formatCardinality formats min/max constraints as a human-readable string.
func formatCardinality(minC, maxC *int) string {
	if minC == nil && maxC == nil {
		return ""
	}
	minVal := 0
	if minC != nil {
		minVal = *minC
	}
	if maxC == nil {
		if minVal == 0 {
			return ""
		}
		return fmt.Sprintf("min %d", minVal)
	}
	maxVal := *maxC
	if minVal == maxVal {
		return fmt.Sprintf("exactly %d", minVal)
	}
	if minVal == 0 {
		return fmt.Sprintf("max %d", maxVal)
	}
	return fmt.Sprintf("%d-%d", minVal, maxVal)
}
