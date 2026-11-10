package automation

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestWithClock pins that one option reaches both clocks of the engine:
// {{today}} in a set value and today() in a condition.
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
      stamp: {type: string}
relations: {}
automations:
  - name: stamp-started
    on:
      entity: [item]
      created: true
      condition: "entity.start <= today()"
    do:
      - set: stamp
        value: "{{today}}"
`))
	if err != nil {
		t.Fatal(err)
	}
	pinned := func() time.Time { return time.Date(2020, 1, 1, 9, 0, 0, 0, time.UTC) }
	tests := []struct {
		name  string
		start string
		want  string // empty: the automation does not fire
	}{
		{"started on the pinned day", "2019-12-31", "2020-01-01"},
		// Long past on the wall clock, but after the pinned day.
		{"not yet started on the pinned day", "2020-06-01", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngineFromMetamodel(meta, meta.Automations, WithClock(pinned))
			if err != nil {
				t.Fatal(err)
			}
			res := engine.Process(context.Background(), Event{
				Type:   EventEntityCreated,
				Entity: &entity.Entity{ID: "I-1", Type: "item", Properties: map[string]any{"start": tc.start}},
			})
			if got := res.PropertiesSet["stamp"]; got != tc.want {
				t.Errorf("stamp = %v, want %v", got, tc.want)
			}
		})
	}
}
