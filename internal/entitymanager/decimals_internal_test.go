package entitymanager

import (
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A decimal that arrives as a number is stored as text, so the stored type
// does not depend on the client (TKT-DTUAG3U).
func TestStoreDecimalsAsText(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  invoice:
    label: Invoice
    plural: invoices
    id_type: manual
    properties:
      price:
        type: decimal
      rates:
        type: decimal
        list: true
      qty:
        type: integer
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}

	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{"float", map[string]any{"price": 12.5}, map[string]any{"price": "12.5"}},
		{"int", map[string]any{"price": 3}, map[string]any{"price": "3"}},
		{"text kept", map[string]any{"price": "12.50"}, map[string]any{"price": "12.50"}},
		{"list", map[string]any{"rates": []any{0.1, "0.20"}}, map[string]any{"rates": []any{"0.1", "0.20"}}},
		{"invalid left for validation", map[string]any{"price": true}, map[string]any{"price": true}},
		{"other types untouched", map[string]any{"qty": 4.0}, map[string]any{"qty": 4.0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storeDecimalsAsText(meta, "invoice", tc.in)
			if !reflect.DeepEqual(tc.in, tc.want) {
				t.Errorf("got %#v, want %#v", tc.in, tc.want)
			}
		})
	}
}
