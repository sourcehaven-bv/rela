package realtimecfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPropsSchema(t *testing.T) {
	t.Parallel()
	mt := &MessageType{Name: "note", Properties: map[string]Property{
		"body":  {Name: "body", Label: "Body", Type: PropertyMarkdown, Required: true, Max: 100},
		"n":     {Name: "n", Type: PropertyInteger, Required: true},
		"ok":    {Name: "ok", Type: PropertyBoolean},
		"on":    {Name: "on", Type: PropertyDate},
		"at":    {Name: "at", Type: PropertyDatetime},
		"pick":  {Name: "pick", Type: PropertyEnum, Values: []string{"low", "high"}},
		"about": {Name: "about", Label: "About", Type: PropertyRef, To: []string{"ticket", "doc"}},
		"any":   {Name: "any", Type: PropertyRef},
	}}
	got, err := json.Marshal(mt.PropsSchema())
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type": "object",
		"additionalProperties": false,
		"required": ["body", "n"],
		"properties": {
			"body":  {"type": "string", "maxLength": 100, "description": "Body"},
			"n":     {"type": "integer"},
			"ok":    {"type": "boolean"},
			"on":    {"type": "string", "format": "date"},
			"at":    {"type": "string", "format": "date-time"},
			"pick":  {"type": "string", "enum": ["low", "high"]},
			"about": {"type": "string", "description": "About. Entity ID of type ticket, doc"},
			"any":   {"type": "string", "description": "Entity ID"}
		}
	}`, string(got))

	empty, err := json.Marshal((&MessageType{Name: "typing"}).PropsSchema())
	require.NoError(t, err)
	require.JSONEq(t, `{"type": "object", "additionalProperties": false, "properties": {}}`, string(empty))
}
