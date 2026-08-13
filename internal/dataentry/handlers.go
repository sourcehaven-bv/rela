package dataentry

import (
	"fmt"
	htmltemplate "html/template"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/mermaid"
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

// helpContentData is the template model for the entity-help fragment. It exists
// so the fragment is rendered by html/template (contextual auto-escaping) rather
// than assembled with fmt.Fprintf, which made every field's escaping a manual,
// individually-auditable decision.
type helpContentData struct {
	EntityDesc   htmltemplate.HTML
	Props        []PropertyHelp
	OutgoingRels []RelationHelp
	IncomingRels []RelationHelp
	Enums        []EnumHelp
}

// helpContentTemplate renders the entity-help fragment.
//
// Escaping contract: every plain-string field (names, types, cardinalities,
// enum values, state names, and the mermaid diagram source) is auto-escaped by
// html/template in its element context. The only fields interpolated as raw
// markup are the htmltemplate.HTML ones — Description / Help — which hold
// goldmark output produced from OPERATOR-authored schema.yaml prose by
// simpleMarkdownToHTML (internal/dataentry/helpers.go:345). That converter runs
// goldmark with html.WithUnsafe() (helpers.go:341), so it passes raw HTML
// through unsanitized; it is safe HERE only because schema.yaml is on-disk
// operator configuration that no HTTP/MCP/Lua write path can modify — NOT
// because the markdown is sanitized. Never route user-authored entity content
// (entity bodies or property values) into these fields.
var helpContentTemplate = htmltemplate.Must(htmltemplate.New("help").
	Funcs(htmltemplate.FuncMap{"stateDiagram": enumStateDiagram}).
	Parse(`
<div class="help-content">
{{- with .EntityDesc}}<div class="entity-description">{{.}}</div>{{end}}
{{- if .Props}}
<h4>Properties</h4><table class="help-table"><thead><tr><th>Name</th><th>Type</th><th>Required</th><th>Description</th></tr></thead><tbody>
{{- range .Props}}
<tr><td><code>{{.Name}}</code></td><td>{{.Type}}</td><td>{{if .Required}}Yes{{end}}</td><td>{{.Description}}</td></tr>
{{- end}}
</tbody></table>
{{- end}}
{{- if .OutgoingRels}}
<h4>Outgoing Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Target</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>
{{- range .OutgoingRels}}
<tr><td><code>{{template "relName" .}}</code></td><td>{{.TargetType}}</td><td>{{.Cardinality}}</td><td>{{.Description}}</td></tr>
{{- end}}
</tbody></table>
{{- end}}
{{- if .IncomingRels}}
<h4>Incoming Relations</h4><table class="help-table"><thead><tr><th>Name</th><th>Source</th><th>Cardinality</th><th>Description</th></tr></thead><tbody>
{{- range .IncomingRels}}
<tr><td><code>{{template "relName" .}}</code></td><td>{{.TargetType}}</td><td>{{.Cardinality}}</td><td>{{.Description}}</td></tr>
{{- end}}
</tbody></table>
{{- end}}
{{- range .Enums}}
{{- if .Values}}
<h4>Values: <code>{{.Property}}</code></h4><table class="help-table"><thead><tr><th>Value</th><th>Description</th></tr></thead><tbody>
{{- range .Values}}
<tr><td><code>{{if .Label}}{{.Label}} ({{.Value}}){{else}}{{.Value}}{{end}}</code></td><td>{{.Description}}</td></tr>
{{- end}}
</tbody></table>
{{- end}}
{{- if .Transitions}}
<h4>Lifecycle: <code>{{.Property}}</code></h4>
<pre class="mermaid">{{stateDiagram .}}</pre>
<table class="help-table"><thead><tr><th>Move</th><th>From &rarr; To</th><th>When to use</th></tr></thead><tbody>
{{- range .Transitions}}
<tr><td>{{.Move}}</td><td><code>{{.From}}</code> &rarr; <code>{{.To}}</code></td><td>{{.Help}}</td></tr>
{{- end}}
</tbody></table>
{{- end}}
{{- end}}
</div>`))

// relName renders a relation's display name (Label with the raw name in
// parentheses, else the bare name). Defined as an associated template so both
// relation tables share one definition.
var _ = htmltemplate.Must(helpContentTemplate.New("relName").
	Parse(`{{if .Label}}{{.Label}} ({{.Name}}){{else}}{{.Name}}{{end}}`))

// renderHelpContent generates HTML for entity help content. It takes an
// io.Writer (not http.ResponseWriter) because it only ever writes the body —
// which also lets the escaping contract be tested against a buffer.
func (a *App) renderHelpContent(
	w io.Writer, entityDesc htmltemplate.HTML, props []PropertyHelp,
	outgoingRels, incomingRels []RelationHelp, enums []EnumHelp,
) {
	// The template is static and parsed at init, so Execute can only fail if the
	// writer fails (e.g. the client hung up). Headers are already sent by then,
	// so there is nothing to report — matching the previous fmt.Fprintf behavior.
	_ = helpContentTemplate.Execute(w, helpContentData{
		EntityDesc:   entityDesc,
		Props:        props,
		OutgoingRels: outgoingRels,
		IncomingRels: incomingRels,
		Enums:        enums,
	})
}

// enumStateDiagram builds a stateDiagram-v2 source for one state-machine field
// by mapping the EnumHelp DTO onto the shared injection-safe renderer in
// internal/mermaid. The move label is EnumHelp.Transitions[].Move, which already
// carries the Label→To fallback applied when the EnumHelp was built.
func enumStateDiagram(e EnumHelp) string {
	ts := make([]mermaid.Transition, 0, len(e.Transitions))
	for _, tr := range e.Transitions {
		ts = append(ts, mermaid.Transition{From: tr.From, To: tr.To, Label: tr.Move})
	}
	return mermaid.StateDiagram(e.Initial, ts)
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
