package docs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// fixtureMeta builds a small ISMS-flavored metamodel: a `risico` entity with a
// required `kans`/`impact`, a `status` state machine (todo→doing→done) and a
// flat `behandeling` enum with per-value descriptions, plus a `maatregel` and a
// `wordt_gemitigeerd_door` relation.
func fixtureMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m := &metamodel.Metamodel{
		Description: "The Sourcehaven ISMS.",
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"todo", "doing", "done"},
				Initial: "todo",
				Transitions: []metamodel.TransitionDef{
					{From: "todo", To: "doing", Label: "start"},
					{From: "doing", To: "done", Label: "finish"},
				},
			},
			"behandeling": {
				Values:  []string{"mitigeren", "accepteren"},
				Default: "accepteren",
				Descriptions: map[string]string{
					"mitigeren":  "Reduce the risk with controls.",
					"accepteren": "Accept the residual risk.",
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"risico": {
				Label:         "risico",
				PropertyOrder: []string{"titel", "kans", "impact", "status", "behandeling"},
				Properties: map[string]metamodel.PropertyDef{
					"titel":       {Type: "string", Required: true},
					"kans":        {Type: "integer", Required: true},
					"impact":      {Type: "integer", Required: true},
					"status":      {Type: "status", Required: true},
					"behandeling": {Type: "behandeling"},
				},
			},
			"maatregel": {
				Label:         "maatregel",
				PropertyOrder: []string{"titel"},
				Properties:    map[string]metamodel.PropertyDef{"titel": {Type: "string", Required: true}},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"wordt_gemitigeerd_door": {
				Description: "A risk is mitigated by a control.",
				From:        []string{"risico"},
				To:          []string{"maatregel"},
			},
		},
	}
	m.InitAliases()
	return m
}

func fixturePolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"*"}, Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}},
			"viewer": {Read: []string{"*"}},
		},
	}
}

func build(t *testing.T, src string, opts Options) string {
	t.Helper()
	if opts.Meta == nil {
		opts.Meta = fixtureMeta(t)
	}
	out, err := Build(context.Background(), src, opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return out
}

// AC1: statement island emits and splices; surrounding markdown passes through.
func TestBuild_StatementIsland(t *testing.T) {
	t.Parallel()
	out := build(t, "intro\n\n```rela\nh2(\"Risico\")\nmd(\"body\")\n```\n\noutro\n", Options{})
	for _, want := range []string{"intro", "## Risico", "body", "outro"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// AC2: echo island substitutes a value inline.
func TestBuild_EchoCount(t *testing.T) {
	t.Parallel()
	src := "```rela\ncreate(\"risico\", {titel=\"a\", kans=2, impact=3})\ncreate(\"risico\", {titel=\"b\", kans=1, impact=1})\n```\nThere are `rela count{type=\"risico\"}` risks.\n"
	out := build(t, src, Options{})
	if !strings.Contains(out, "There are 2 risks.") {
		t.Errorf("echo count not substituted:\n%s", out)
	}
}

// AC3: typeref required-only table in property order.
func TestBuild_TyperefRequired(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\ntyperef{type=\"risico\", fields=\"required\"}\n```\n", Options{})
	if !strings.Contains(out, "| `titel` | string | yes |") {
		t.Errorf("missing titel row:\n%s", out)
	}
	// behandeling is optional → excluded from required-only.
	if strings.Contains(out, "`behandeling`") {
		t.Errorf("optional field leaked into required-only table:\n%s", out)
	}
}

// AC4: values with descriptions render a meaning table + default marker.
func TestBuild_ValuesWithDescriptions(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nvalues{type=\"risico\", field=\"behandeling\"}\n```\n", Options{})
	for _, want := range []string{"Reduce the risk with controls.", "Accept the residual risk.", "_(default)_"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

// AC5: lifecycle renders a mermaid state diagram for a machine; falls back to a
// flat list when there are no transitions.
func TestBuild_LifecycleDiagram(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nlifecycle{type=\"risico\", field=\"status\"}\n```\n", Options{})
	if !strings.Contains(out, "```mermaid") || !strings.Contains(out, "stateDiagram-v2") {
		t.Errorf("expected a mermaid state diagram:\n%s", out)
	}
	if !strings.Contains(out, ": start") {
		t.Errorf("expected the start transition label:\n%s", out)
	}
}

func TestBuild_LifecycleFlatFallback(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nlifecycle{type=\"risico\", field=\"behandeling\"}\n```\n", Options{})
	if strings.Contains(out, "stateDiagram") {
		t.Errorf("flat enum should not render a diagram:\n%s", out)
	}
	if !strings.Contains(out, "`mitigeren`") {
		t.Errorf("expected the flat value list:\n%s", out)
	}
}

// AC6: instance graph over a seeded diamond asserts the exact deduped edge set.
func TestBuild_GraphInstanceDiamond(t *testing.T) {
	t.Parallel()
	// r -mit-> m1, r -mit-> m2 (fan-out); both are maatregel. depth 1.
	src := `` +
		"```rela\n" +
		"local r = create(\"risico\", {titel=\"x\", kans=1, impact=1})\n" +
		"local m1 = create(\"maatregel\", {titel=\"c1\"})\n" +
		"local m2 = create(\"maatregel\", {titel=\"c2\"})\n" +
		"link(r, \"wordt_gemitigeerd_door\", m1)\n" +
		"link(r, \"wordt_gemitigeerd_door\", m2)\n" +
		"graph{from=r.id, depth=1}\n" +
		"```\n"
	out := build(t, src, Options{})
	if !strings.Contains(out, "```mermaid") || !strings.Contains(out, "graph LR") {
		t.Errorf("expected a mermaid flow graph:\n%s", out)
	}
	// Two distinct edges labeled with the relation, no duplicate.
	if c := strings.Count(out, "wordt_gemitigeerd_door"); c != 2 {
		t.Errorf("expected 2 mitigation edges, got %d:\n%s", c, out)
	}
}

// AC6: schema-grain graph from a type.
func TestBuild_GraphSchema(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\ngraph{from=\"risico\", depth=1}\n```\n", Options{})
	if !strings.Contains(out, "graph LR") {
		t.Errorf("expected schema graph:\n%s", out)
	}
	if !strings.Contains(out, "wordt_gemitigeerd_door") {
		t.Errorf("expected the schema edge:\n%s", out)
	}
}

// AC6: exclude prunes a relation type.
func TestBuild_GraphExclude(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\ngraph{from=\"risico\", depth=1, exclude={\"wordt_gemitigeerd_door\"}}\n```\n", Options{})
	if strings.Contains(out, "wordt_gemitigeerd_door") {
		t.Errorf("excluded relation should be pruned:\n%s", out)
	}
}

func TestBuild_GraphExcludeOnlyConflict(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(),
		"```rela\ngraph{from=\"risico\", exclude={\"a\"}, only={\"b\"}}\n```\n",
		Options{Meta: fixtureMeta(t)})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual-exclusion error, got %v", err)
	}
}

// AC7: roles_matrix renders a role × verb table.
func TestBuild_RolesMatrix(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nroles_matrix{type=\"risico\"}\n```\n", Options{Meta: fixtureMeta(t), Policy: fixturePolicy()})
	if !strings.Contains(out, "editor") || !strings.Contains(out, "viewer") {
		t.Errorf("expected both roles as columns:\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected at least one grant tick:\n%s", out)
	}
}

func TestBuild_RolesMatrixNoPolicy(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nroles_matrix{type=\"risico\"}\n```\n", Options{Meta: fixtureMeta(t)})
	if !strings.Contains(out, "No access policy") {
		t.Errorf("expected no-policy note:\n%s", out)
	}
}

// AC8: raw-store seed accepts any status (no state-machine gate).
func TestBuild_SeedRawStoreNoGate(t *testing.T) {
	t.Parallel()
	out := build(t, "```rela\nlocal r = create(\"risico\", {titel=\"x\", kans=1, impact=1, status=\"done\"})\nentity{id=r.id, fields={\"status\"}}\n```\n", Options{})
	if !strings.Contains(out, "status: done") {
		t.Errorf("raw-store seed should accept status=done directly:\n%s", out)
	}
}

// AC9: unknown type fails loud with a manual line. Also a regression guard that
// a resolver's typed BuildError round-trips cleanly (kind=resolve, clean Msg,
// correct line) rather than being stringified into a lua-kind error by
// RaiseError.
func TestBuild_FailLoudUnknownType(t *testing.T) {
	t.Parallel()
	// ```rela is line 2; the resolver call is on body line 3.
	_, err := Build(context.Background(), "line1\n```rela\ntyperef{type=\"nope\"}\n```\n", Options{Meta: fixtureMeta(t)})
	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("want *BuildError, got %T: %v", err, err)
	}
	if be.Kind != "resolve" {
		t.Errorf("kind = %q, want resolve (typed round-trip, not lua-stringified)", be.Kind)
	}
	if be.Line != 3 {
		t.Errorf("manual line = %d, want 3", be.Line)
	}
	if be.Msg != `typeref: unknown entity type "nope"` {
		t.Errorf("msg = %q, want the clean resolver message", be.Msg)
	}
}

// AC9 (M1): a Lua error on the SECOND line of a multi-line island reports the
// manual line of that line, not the island start.
func TestBuild_FailLoudLuaLineOffset(t *testing.T) {
	t.Parallel()
	// ```rela is line 1; body line 1 = "h2(...)" (manual 2); body line 2 =
	// "nosuchfunc()" (manual 3), which errors.
	_, err := Build(context.Background(), "```rela\nh2(\"ok\")\nnosuchfunc()\n```\n", Options{Meta: fixtureMeta(t)})
	var be *BuildError
	if !errors.As(err, &be) {
		t.Fatalf("want *BuildError, got %T: %v", err, err)
	}
	if be.Line != 3 {
		t.Errorf("manual line = %d, want 3 (the erroring line, not the island start)", be.Line)
	}
}

// AC2/echo: a table-returning resolver in an echo span is an author error.
func TestBuild_EchoRejectsBlockResolver(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(), "x `rela typeref{type=\"risico\"}` y\n", Options{Meta: fixtureMeta(t)})
	if err == nil {
		t.Fatal("expected an error for a block resolver in an echo span")
	}
}

// AC9 strict: empty resolve errors under strict, warns otherwise.
func TestBuild_StrictEmptyResolve(t *testing.T) {
	t.Parallel()
	m := fixtureMeta(t)
	m.Description = "" // description() now yields empty
	_, err := Build(context.Background(), "`rela description()`\n", Options{Meta: m, Strict: true})
	if err == nil {
		t.Fatal("strict mode should fail on empty resolve")
	}
	// Non-strict: builds fine (empty substituted).
	if _, err := Build(context.Background(), "`rela description()`\n", Options{Meta: m}); err != nil {
		t.Fatalf("non-strict empty resolve should not fail: %v", err)
	}
}

// AC12: an infinite loop is stopped by the build timeout (use a short one).
func TestBuild_InfiniteLoopTimesOut(t *testing.T) {
	t.Parallel()
	// A tiny timeout keeps the test fast; the real default is buildTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), 500_000_000) // 500ms
	defer cancel()
	_, err := Build(ctx, "```rela\nwhile true do end\n```\n", Options{Meta: fixtureMeta(t)})
	if err == nil {
		t.Fatal("infinite loop should abort via timeout")
	}
}
