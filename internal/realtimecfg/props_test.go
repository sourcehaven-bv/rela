package realtimecfg

import (
	"errors"
	"testing"
)

// TestNormalizeProps_PropError pins that a refusal of one named property
// is a *propError whose Error is the fire-time text and whose reason
// leaves out the name, and that a missing required property is not one.
func TestNormalizeProps_PropError(t *testing.T) {
	mt := &MessageType{Name: "note", Properties: map[string]Property{
		"body":  {Name: "body", Type: PropertyString, Required: true},
		"short": {Name: "short", Type: PropertyString, Max: 3},
	}}
	cases := []struct {
		name         string
		in           map[string]any
		prop, reason string
		text         string
	}{
		{"undeclared", map[string]any{"body": "hi", "x": "y"}, "x",
			"message type note has no property of this name", `message type note has no property "x"`},
		{"over max", map[string]any{"body": "hi", "short": "abcd"}, "short",
			"must be at most 3 characters", `property "short": must be at most 3 characters`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mt.NormalizeProps(tc.in, 1024)
			var pe *propError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v; want a *propError", err)
			}
			if pe.prop != tc.prop || pe.reason != tc.reason || err.Error() != tc.text {
				t.Errorf("propError = %+v, Error() = %q; want prop %q, reason %q, text %q",
					*pe, err.Error(), tc.prop, tc.reason, tc.text)
			}
		})
	}

	t.Run("missing required", func(t *testing.T) {
		_, err := mt.NormalizeProps(map[string]any{}, 1024)
		var pe *propError
		if err == nil || errors.As(err, &pe) {
			t.Fatalf("err = %v; want a plain error", err)
		}
		if err.Error() != `property "body" is required` {
			t.Errorf("err = %q", err)
		}
	})
}
