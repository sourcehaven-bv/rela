package metamodel

import (
	"strings"
	"testing"
)

func TestParse_PropertyDisplay(t *testing.T) {
	prop := func(body string) string {
		return refSchemaHead + "      value:\n" + body
	}
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "currency on integer", yaml: prop("        type: integer\n        display:\n          currency: EUR\n")},
		{name: "lower-case code", yaml: prop("        type: integer\n        display:\n          currency: eur\n"),
			wantErr: `property "value": display: currency "eur" is not an ISO 4217 code`},
		{name: "four letters", yaml: prop("        type: integer\n        display:\n          currency: EURO\n"),
			wantErr: `currency "EURO" is not an ISO 4217 code`},
		{name: "symbol", yaml: prop("        type: integer\n        display:\n          currency: \"€\"\n"),
			wantErr: `is not an ISO 4217 code`},
		{name: "on a string", yaml: prop("        type: string\n        display:\n          currency: EUR\n"),
			wantErr: `display: currency needs an integer property, not "string"`},
		{name: "on a list", yaml: prop("        type: integer\n        list: true\n        display:\n          currency: EUR\n"),
			wantErr: "display: currency is not supported on a list property"},
		{name: "empty display", yaml: prop("        type: integer\n        display: {}\n"),
			wantErr: "display names no format; use currency"},
		{name: "format not yet supported", yaml: prop("        type: integer\n        display:\n          percent: true\n"),
			wantErr: "display names no format; use currency"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := Parse([]byte(tt.yaml))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got := meta.Entities["ticket"].Properties["value"].Display; got == nil || got.Currency != "EUR" {
					t.Fatalf("display = %+v, want currency EUR", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
