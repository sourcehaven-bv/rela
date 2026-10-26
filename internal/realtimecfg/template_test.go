package realtimecfg

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestTemplate_Eval pins AC 6: templates evaluate to typed values in one
// pass, from the notice time rather than the clock.
func TestTemplate_Eval(t *testing.T) {
	v := &validator{meta: testMeta(t)}
	mt := &MessageType{Name: "assessed", Properties: map[string]Property{
		"risk":  {Name: "risk", Type: PropertyEnum, Enum: "risk-level"},
		"note":  {Name: "note", Type: PropertyMarkdown},
		"count": {Name: "count", Type: PropertyInteger},
	}}
	at := time.Date(2026, 10, 26, 23, 30, 0, 0, time.FixedZone("CET", -3600))
	vars := OutputVars{
		Message: map[string]any{"risk": "high", "note": "see {{message.risk}}", "count": float64(3)},
		Author:  "u-ann", From: "contained", To: "assessed", Generation: 4,
		ChannelKey: "datalek/DL-1/breach-response", ChannelType: "breach-response", ChannelTitle: "Breach",
		At: at,
	}
	str := metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	cases := []struct {
		name string
		raw  any
		def  metamodel.PropertyDef
		vars OutputVars
		want any
		ok   bool
	}{
		{"string literal", "notified", str, vars, "notified", true},
		{"integer literal", 7, metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}, vars, 7, true},
		{"whole enum reference", "{{message.risk}}", metamodel.PropertyDef{Type: "risk-level"}, vars, "high", true},
		{"whole integer reference", "{{ message.count }}", metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			vars, int64(3), true},
		{"absent whole reference", "{{message.risk}}", str, OutputVars{}, nil, false},
		{"absent reference in text", "risk: {{message.risk}}.", str, OutputVars{}, "risk: .", true},
		{"no rescan", "note: {{message.note}}", str, vars, "note: see {{message.risk}}", true},
		{"now into datetime", "{{now}}", metamodel.PropertyDef{Type: metamodel.PropertyTypeDatetime}, vars,
			"2026-10-27T00:30:00Z", true},
		{"now into string", "at {{now}}", str, vars, "at 2026-10-27T00:30:00Z", true},
		{"today", "{{today}}", metamodel.PropertyDef{Type: metamodel.PropertyTypeDate}, vars, "2026-10-27", true},
		{"session and channel names",
			"{{session.from}}>{{session.to}}#{{session.generation}} {{channel.key}} {{channel.type}} " +
				"{{channel.title}} {{message.author}}", str, vars,
			"contained>assessed#4 datalek/DL-1/breach-response breach-response Breach u-ann", true},
		{"generation into integer", "{{session.generation}}", metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			vars, int64(4), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, reason := writeChecker{v}.compileTemplate(tc.raw, templateTarget{entityType: "datalek", name: "p", def: tc.def}, mt)
			if reason != "" {
				t.Fatalf("compile: %s", reason)
			}
			for range 2 { // the same inputs give the same output
				got, ok := tpl.Eval(tc.vars)
				if ok != tc.ok || got != tc.want {
					t.Fatalf("Eval = %#v, %v; want %#v, %v", got, ok, tc.want, tc.ok)
				}
			}
		})
	}
}
