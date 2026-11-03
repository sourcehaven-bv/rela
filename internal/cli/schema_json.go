package cli

// JSON serialization for the `rela schema` subcommands. This is schema
// serialization, not output formatting — none of it branches on the output
// format — so it lives next to its only caller (schema.go) instead of on
// output.Writer (TKT-NS3XPE).

import (
	"encoding/json"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// schemaJSONWriter serializes metamodel schema views as indented JSON.
type schemaJSONWriter struct {
	out io.Writer
}

func (w schemaJSONWriter) writeJSON(data any) error {
	encoder := json.NewEncoder(w.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeOverview outputs the metamodel overview as JSON
func (w schemaJSONWriter) writeOverview(m *metamodel.Metamodel) error {
	data := map[string]any{
		"version":   m.Version,
		"namespace": m.Namespace,
		"entities":  m.Entities,
		"relations": m.Relations,
		"types":     m.Types,
	}
	return w.writeJSON(data)
}

// writeEntities outputs entity types as JSON
func (w schemaJSONWriter) writeEntities(m *metamodel.Metamodel) error {
	return w.writeJSON(m.Entities)
}

// writeRelations outputs relation types as JSON
func (w schemaJSONWriter) writeRelations(m *metamodel.Metamodel) error {
	return w.writeJSON(m.Relations)
}

// writeTypes outputs custom types as JSON
func (w schemaJSONWriter) writeTypes(m *metamodel.Metamodel) error {
	return w.writeJSON(m.Types)
}

// writeEntityDetail outputs a single entity type as JSON
func (w schemaJSONWriter) writeEntityDetail(name string, def *metamodel.EntityDef) error {
	data := map[string]any{
		"name":        name,
		"label":       def.Label,
		"aliases":     def.Aliases,
		"id_patterns": def.GetIDPrefixes(),
		"properties":  def.Properties,
	}
	if rdfType := def.RDFType; rdfType != "" {
		data["rdf_type"] = rdfType
	}
	if entityColor := def.Color; entityColor != "" {
		data["color"] = entityColor
	}
	if borderColor := def.BorderColor; borderColor != "" {
		data["border_color"] = borderColor
	}
	return w.writeJSON(data)
}

// writeRelationDetail outputs a single relation type as JSON
func (w schemaJSONWriter) writeRelationDetail(name string, def *metamodel.RelationDef) error {
	data := map[string]any{
		"name":  name,
		"label": def.Label,
		"from":  def.From,
		"to":    def.To,
	}
	if desc := def.Description; desc != "" {
		data["description"] = desc
	}
	if inv := def.Inverse; inv != nil {
		data["inverse"] = inv
	}
	if def.Symmetric {
		data["symmetric"] = true
	}
	if def.Owning {
		data["owning"] = true
	}
	if minOut := def.MinOutgoing; minOut != nil {
		data["min_outgoing"] = *minOut
	}
	if maxOut := def.MaxOutgoing; maxOut != nil {
		data["max_outgoing"] = *maxOut
	}
	if minIn := def.MinIncoming; minIn != nil {
		data["min_incoming"] = *minIn
	}
	if maxIn := def.MaxIncoming; maxIn != nil {
		data["max_incoming"] = *maxIn
	}
	return w.writeJSON(data)
}
