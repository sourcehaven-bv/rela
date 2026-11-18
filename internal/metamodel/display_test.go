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
		// wantLink marks a valid link format; otherwise a valid case is EUR.
		wantLink bool
	}{
		{name: "currency on integer", yaml: prop("        type: integer\n        display:\n          currency: EUR\n")},
		{name: "lower-case code", yaml: prop("        type: integer\n        display:\n          currency: eur\n"),
			wantErr: `property "value": display: currency "eur" is not an ISO 4217 code`},
		{name: "four letters", yaml: prop("        type: integer\n        display:\n          currency: EURO\n"),
			wantErr: `currency "EURO" is not an ISO 4217 code`},
		{name: "symbol", yaml: prop("        type: integer\n        display:\n          currency: \"€\"\n"),
			wantErr: `is not an ISO 4217 code`},
		{name: "on a string", yaml: prop("        type: string\n        display:\n          currency: EUR\n"),
			wantErr: `display: currency needs an integer or decimal property, not "string"`},
		{name: "on a list", yaml: prop("        type: integer\n        list: true\n        display:\n          currency: EUR\n"),
			wantErr: "display: currency is not supported on a list property"},
		{name: "empty display", yaml: prop("        type: integer\n        display: {}\n"),
			wantErr: "display names no format; use currency or link"},
		{name: "format not yet supported", yaml: prop("        type: integer\n        display:\n          percent: true\n"),
			wantErr: "display names no format; use currency or link"},
		{name: "link on a string", yaml: prop("        type: string\n        display:\n          link: true\n"), wantLink: true},
		{name: "link false", yaml: prop("        type: string\n        display:\n          link: false\n"),
			wantErr: "display names no format; use currency or link"},
		{name: "link on an integer", yaml: prop("        type: integer\n        display:\n          link: true\n"),
			wantErr: `display: link needs a string property, not "integer"`},
		{name: "link on a list", yaml: prop("        type: string\n        list: true\n        display:\n          link: true\n"),
			wantErr: "display: link is not supported on a list property"},
		{name: "link on an inline enum", yaml: prop("        type: string\n        values: [a, b]\n        display:\n          link: true\n"),
			wantErr: "display: link is not supported on a property with values"},
		{name: "link and currency", yaml: prop("        type: integer\n        display:\n          link: true\n          currency: EUR\n"),
			wantErr: "display names two formats; use currency or link, not both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := Parse([]byte(tt.yaml))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got := meta.Entities["ticket"].Properties["value"].Display
				want := PropertyDisplay{Currency: "EUR"}
				if tt.wantLink {
					want = PropertyDisplay{Link: true}
				}
				if got == nil || *got != want {
					t.Fatalf("display = %+v, want %+v", got, want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
