package triggers

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestCapabilitiesSurviveActionConversion is a REGRESSION test (TKT-YH52OM).
//
// The decoder turns a `script:` output into the internal Action, and the
// conversion is a hand-written field list. The old metamodel conversion
// copied AllowACLBypass but NOT Capabilities, so every automation's
// `capabilities:` block was silently dropped one hop before it could reach
// the runtime. The script then failed with "attempt to index a nil value
// (global 'http')" despite the operator having declared it.
//
// This pins both hops: flows.yaml to Action, and Action to LuaToExecute.
func TestCapabilitiesSurviveActionConversion(t *testing.T) {
	t.Parallel()

	meta := mustMeta(t, `
entities:
  note: {label: Note, id_prefix: "NOTE-", properties: {title: {type: string}}}
`)
	rules := mustParse(t, meta, `
triggers:
  notify:
    on: {entity: note, created: true}
    outputs:
      - script:
          lua: "return 1"
          capabilities: {http: true, secrets: [slack]}
`)
	if len(rules) != 1 || len(rules[0].Do) != 1 {
		t.Fatalf("rules = %+v, want one rule with one action", rules)
	}
	caps := rules[0].Do[0].Capabilities
	if !caps.HTTP {
		t.Error("http grant was dropped decoding the script output")
	}
	if len(caps.Secrets) != 1 || caps.Secrets[0] != "slack" {
		t.Errorf("secrets grant was dropped: got %v, want [slack]", caps.Secrets)
	}

	engine, err := New(meta, rules)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: entity.New("NOTE-1", "note")})
	if len(res.LuaToExecute) != 1 {
		t.Fatalf("LuaToExecute = %+v, want one script", res.LuaToExecute)
	}
	got := res.LuaToExecute[0].Capabilities
	if !got.HTTP || len(got.Secrets) != 1 || got.Secrets[0] != "slack" {
		t.Errorf("grant dropped on the way to LuaToExecute: http=%v secrets=%v", got.HTTP, got.Secrets)
	}
}
