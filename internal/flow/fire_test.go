package flow

import (
	"errors"
	"testing"
)

func TestFire(t *testing.T) {
	boom := errors.New("boom")
	type trig struct {
		name string
		ok   bool
		err  error
	}
	holds := func(tr trig) (bool, error) { return tr.ok, tr.err }
	tests := []struct {
		name     string
		triggers []trig
		want     []Fired[trig]
	}{
		{"none", nil, nil},
		{"all hold, order kept",
			[]trig{{name: "a", ok: true}, {name: "b", ok: true}},
			[]Fired[trig]{{Trigger: trig{name: "a", ok: true}}, {Trigger: trig{name: "b", ok: true}}}},
		{"false guard skipped",
			[]trig{{name: "a"}, {name: "b", ok: true}},
			[]Fired[trig]{{Trigger: trig{name: "b", ok: true}}}},
		{"failed guard reported in place, later ones still tried",
			[]trig{{name: "a", ok: true}, {name: "b", err: boom}, {name: "c", ok: true}},
			[]Fired[trig]{
				{Trigger: trig{name: "a", ok: true}},
				{Trigger: trig{name: "b", err: boom}, Err: boom},
				{Trigger: trig{name: "c", ok: true}},
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Fire(tc.triggers, holds)
			if len(got) != len(tc.want) {
				t.Fatalf("Fire = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i].Trigger.name != tc.want[i].Trigger.name || !errors.Is(got[i].Err, tc.want[i].Err) {
					t.Errorf("Fire[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
