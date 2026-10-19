package realtimecfg

import (
	"slices"
	"strings"
)

// PropsSchema is the JSON Schema of a message type's props object. The
// OpenAPI document and the MCP tools both describe a message type with it,
// so the two cannot drift.
//
// It states what JSON Schema can express: the declared properties, their
// JSON types and formats, enum values, the max length of a string and the
// required ones. The service checks more than this when a message is
// posted: the type's `when:`, the number of properties, the encoded size
// and that a ref names a readable entity of an allowed type.
type PropsSchema struct {
	Type                 string                `json:"type"`
	Properties           map[string]PropSchema `json:"properties"`
	Required             []string              `json:"required,omitempty"`
	AdditionalProperties bool                  `json:"additionalProperties"`
}

// PropSchema is the JSON Schema of one message property.
type PropSchema struct {
	Type        string   `json:"type"`
	Format      string   `json:"format,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	MaxLength   int      `json:"maxLength,omitempty"`
	Description string   `json:"description,omitempty"`
}

// JSON Schema formats of the date and datetime property types.
const (
	FormatDate     = "date"
	FormatDateTime = "date-time"
)

// PropsSchema returns the JSON Schema of mt's props. Labels are operator
// configuration, so they go into the descriptions as they are.
func (mt *MessageType) PropsSchema() PropsSchema {
	out := PropsSchema{Type: "object", Properties: make(map[string]PropSchema, len(mt.Properties))}
	for name, p := range mt.Properties {
		out.Properties[name] = p.schema()
		if p.Required {
			out.Required = append(out.Required, name)
		}
	}
	slices.Sort(out.Required)
	return out
}

// schema is the JSON Schema of p. The service limits only string and
// markdown values by max, so only those carry maxLength.
func (p Property) schema() PropSchema {
	s := PropSchema{Type: "string", Description: p.Label}
	switch p.Type {
	case PropertyInteger:
		s.Type = "integer"
	case PropertyBoolean:
		s.Type = "boolean"
	case PropertyDate:
		s.Format = FormatDate
	case PropertyDatetime:
		s.Format = FormatDateTime
	case PropertyEnum:
		s.Enum = slices.Clone(p.Values)
	case PropertyRef:
		ref := "Entity ID"
		if len(p.To) > 0 {
			ref += " of type " + strings.Join(p.To, ", ")
		}
		s.Description = joinDescription(p.Label, ref)
	case PropertyString, PropertyMarkdown:
		s.MaxLength = p.Max
	}
	return s
}

func joinDescription(label, rest string) string {
	if label == "" {
		return rest
	}
	return label + ". " + rest
}
