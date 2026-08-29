package cli

// JSON serialization for the `rela schema` subcommands. This is schema
// serialization, not output formatting — none of it branches on the output
// format — so it lives next to its only caller (schema.go) instead of on
// output.Writer (TKT-NS3XPE). The interfaces are consumer-side: the metamodel
// package satisfies them without importing this one.

import (
	"encoding/json"
	"io"
)

// schemaMetamodel is the metamodel view the schema JSON output needs.
// Satisfied by *metamodel.Metamodel.
type schemaMetamodel interface {
	GetVersion() string
	GetNamespace() string
	GetEntities() any
	GetRelations() any
	GetTypes() any
}

// schemaEntityDef is the entity-definition view the schema JSON output needs.
// Satisfied by *metamodel.EntityDef.
type schemaEntityDef interface {
	GetLabel() string
	GetAliases() []string
	GetIDPatterns() []string
	GetProperties() any
	GetRDFType() string
	GetColor() string
	GetBorderColor() string
}

// schemaRelationDef is the relation-definition view the schema JSON output
// needs. Satisfied by *metamodel.RelationDef.
type schemaRelationDef interface {
	GetLabel() string
	GetFrom() []string
	GetTo() []string
	GetDescription() string
	GetInverse() any
	IsSymmetric() bool
	GetMinOutgoing() *int
	GetMaxOutgoing() *int
	GetMinIncoming() *int
	GetMaxIncoming() *int
}

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
func (w schemaJSONWriter) writeOverview(m schemaMetamodel) error {
	data := map[string]any{
		"version":   m.GetVersion(),
		"namespace": m.GetNamespace(),
		"entities":  m.GetEntities(),
		"relations": m.GetRelations(),
		"types":     m.GetTypes(),
	}
	return w.writeJSON(data)
}

// writeEntities outputs entity types as JSON
func (w schemaJSONWriter) writeEntities(m schemaMetamodel) error {
	return w.writeJSON(m.GetEntities())
}

// writeRelations outputs relation types as JSON
func (w schemaJSONWriter) writeRelations(m schemaMetamodel) error {
	return w.writeJSON(m.GetRelations())
}

// writeTypes outputs custom types as JSON
func (w schemaJSONWriter) writeTypes(m schemaMetamodel) error {
	return w.writeJSON(m.GetTypes())
}

// writeEntityDetail outputs a single entity type as JSON
func (w schemaJSONWriter) writeEntityDetail(name string, def schemaEntityDef) error {
	data := map[string]any{
		"name":        name,
		"label":       def.GetLabel(),
		"aliases":     def.GetAliases(),
		"id_patterns": def.GetIDPatterns(),
		"properties":  def.GetProperties(),
	}
	if rdfType := def.GetRDFType(); rdfType != "" {
		data["rdf_type"] = rdfType
	}
	if entityColor := def.GetColor(); entityColor != "" {
		data["color"] = entityColor
	}
	if borderColor := def.GetBorderColor(); borderColor != "" {
		data["border_color"] = borderColor
	}
	return w.writeJSON(data)
}

// writeRelationDetail outputs a single relation type as JSON
func (w schemaJSONWriter) writeRelationDetail(name string, def schemaRelationDef) error {
	data := map[string]any{
		"name":  name,
		"label": def.GetLabel(),
		"from":  def.GetFrom(),
		"to":    def.GetTo(),
	}
	if desc := def.GetDescription(); desc != "" {
		data["description"] = desc
	}
	if inv := def.GetInverse(); inv != nil {
		data["inverse"] = inv
	}
	if def.IsSymmetric() {
		data["symmetric"] = true
	}
	if minOut := def.GetMinOutgoing(); minOut != nil {
		data["min_outgoing"] = *minOut
	}
	if maxOut := def.GetMaxOutgoing(); maxOut != nil {
		data["max_outgoing"] = *maxOut
	}
	if minIn := def.GetMinIncoming(); minIn != nil {
		data["min_incoming"] = *minIn
	}
	if maxIn := def.GetMaxIncoming(); maxIn != nil {
		data["max_incoming"] = *maxIn
	}
	return w.writeJSON(data)
}
