package writerule_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

// TestWithClock pins that today() in a write rule reads the injected clock.
func TestWithClock(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
version: "1.0"
entities:
  item:
    label: Item
    id_prefix: "I-"
    id_type: sequential
    properties:
      start: {type: date}
      title: {type: string}
relations: {}
validations:
  - name: no-future-start
    entity_type: item
    enforce: write
    when_condition: "entity.start > today()"
    then: ["title=never"]
`))
	if err != nil {
		t.Fatal(err)
	}
	e := &entity.Entity{ID: "I-1", Type: "item", Properties: map[string]any{"start": "2020-06-01"}}
	tests := []struct {
		name  string
		opts  []writerule.Option
		fails bool
	}{
		{"pinned before start", []writerule.Option{writerule.WithClock(func() time.Time {
			return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		})}, true},
		{"wall clock is after start", nil, false},
		{"nil clock is the wall clock", []writerule.Option{writerule.WithClock(nil)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, err := writerule.NewSet(meta, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(set.Check(context.Background(), e)) > 0; got != tc.fails {
				t.Errorf("rule broken = %v, want %v", got, tc.fails)
			}
		})
	}
}
