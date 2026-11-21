package dataentry

import (
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The display format reaches the SPA unchanged: currency for amounts
// (TKT-SVLQQ6) and link for URL strings (TKT-0101IO4).
func TestToV1PropertyDef_Display(t *testing.T) {
	meta := &metamodel.Metamodel{}
	tests := []struct {
		name    string
		display *metamodel.PropertyDisplay
		want    *v1.PropertyDisplay
	}{
		{"unset", nil, nil},
		{"currency", &metamodel.PropertyDisplay{Currency: "EUR"}, &v1.PropertyDisplay{Currency: "EUR"}},
		{"link", &metamodel.PropertyDisplay{Link: true}, &v1.PropertyDisplay{Link: true}},
		{
			"link text",
			&metamodel.PropertyDisplay{Link: true, LinkText: "Website"},
			&v1.PropertyDisplay{Link: true, LinkText: "Website"},
		},
		{"image", &metamodel.PropertyDisplay{Image: true}, &v1.PropertyDisplay{Image: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd := metamodel.PropertyDef{Type: metamodel.PropertyTypeString, Display: tt.display}
			got := toV1PropertyDef(meta, pd).Display
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("Display = %+v, want %+v", got, tt.want)
			}
		})
	}
}
