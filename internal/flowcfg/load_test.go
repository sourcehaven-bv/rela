package flowcfg

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse(readFile(t, "schema.yaml"))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

func testPolicy(t *testing.T) *acl.Policy {
	t.Helper()
	p, err := acl.ParsePolicy(readFile(t, "acl.yaml"), "acl.yaml")
	if err != nil {
		t.Fatalf("parse acl: %v", err)
	}
	return p
}

func parseSample(t *testing.T, edits ...[2]string) (*Compiled, error) {
	t.Helper()
	src := string(readFile(t, "flows.yaml"))
	for _, e := range edits {
		if !strings.Contains(src, e[0]) {
			t.Fatalf("sample has no %q", e[0])
		}
		src = strings.Replace(src, e[0], e[1], 1)
	}
	return Parse([]byte(src), testMeta(t), testPolicy(t))
}

func TestParse_Sample(t *testing.T) {
	c, err := parseSample(t)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v; want none", w)
	}
	f, ok := c.Flow("onboarding")
	if !ok {
		t.Fatal("no onboarding flow")
	}
	if f.Subject() != "person" || f.RunAs() != "system:onboarding-bot" || f.Initial() != "collect" ||
		f.PerSubject() != PerSubjectOne || f.Label() != "Onboarding" {

		t.Errorf("flow header = %s %s %s %s %s", f.Subject(), f.RunAs(), f.Initial(), f.PerSubject(), f.Label())
	}
	if f.Deadline().String() != "30d" || f.Retention().Action != RetentionDelete || f.Retention().After.String() != "90d" {
		t.Errorf("deadline %s, retention %+v", f.Deadline(), f.Retention())
	}
	starts := f.Starts()
	if len(starts) != 4 || starts[0].On != StartCreate || starts[2].Permission != "onboarding.start" ||
		starts[3].Every.String() != "1d" || starts[1].When == nil {

		t.Errorf("starts = %+v", starts)
	}
	collect, _ := f.State("collect")
	if collect.Status != flow.StatusInputRequired || collect.Label != "Collect documents" {
		t.Errorf("collect = %+v", collect)
	}
	if outs := collect.Outputs(); len(outs) != 2 || outs[1].Relate != "about" || outs[0].Kind != OutputSet {
		t.Errorf("outputs = %+v", outs)
	}
	if bs := f.Branches("collect", EventChange); len(bs) != 1 || bs[0].To != "review" || bs[0].Roles[0] != "hr" {
		t.Errorf("branches = %+v", bs)
	}
	review, _ := f.State("review")
	if review.Status != flow.StatusWorking || len(review.Events()) != 1 || review.Events()[0] != EventDone {
		t.Errorf("review = %+v", review)
	}
	failed, _ := f.State("failed")
	if !failed.Final || failed.Status != flow.StatusFailed {
		t.Errorf("failed = %+v", failed)
	}
	finished, _ := f.State("finished")
	if finished.Status != flow.StatusCompleted {
		t.Errorf("finished status = %s", finished.Status)
	}
	d := f.Definition()
	st, ok := d.State("escalated")
	if !ok || len(st.Timers()) != 1 || st.Timers()[0].Since != flow.SinceSession {
		t.Errorf("escalated timers = %+v", st.Timers())
	}
	if d.Initial() != "collect" {
		t.Errorf("definition initial = %s", d.Initial())
	}
	if len(f.Version()) != 16 || len(f.Source()) == 0 {
		t.Errorf("version %q, source %d bytes", f.Version(), len(f.Source()))
	}
}

func TestParse_Version(t *testing.T) {
	base, err := parseSample(t)
	if err != nil {
		t.Fatal(err)
	}
	v := func(c *Compiled) string {
		f, _ := c.Flow("onboarding")
		return f.Version()
	}
	commented, err := parseSample(t, [2]string{"label: Onboarding", "label: Onboarding # the HR flow"})
	if err != nil {
		t.Fatal(err)
	}
	if v(commented) != v(base) {
		t.Error("a comment changed the version")
	}
	edited, err := parseSample(t, [2]string{"deadline: 30d", "deadline: 31d"})
	if err != nil {
		t.Fatal(err)
	}
	if v(edited) == v(base) {
		t.Error("an edit kept the version")
	}
}

func TestParse_Empty(t *testing.T) {
	for _, src := range []string{"", "flows: {}\n", "# nothing\n"} {
		c, err := Parse([]byte(src), testMeta(t), nil)
		if err != nil {
			t.Fatalf("Parse(%q): %v", src, err)
		}
		if len(c.Flows()) != 0 {
			t.Errorf("Parse(%q) declared flows", src)
		}
	}
}

func TestParse_NilMeta(t *testing.T) {
	if _, err := Parse(nil, nil, nil); err == nil {
		t.Fatal("Parse accepted a nil metamodel")
	}
}

func TestParse_Problems(t *testing.T) {
	tests := []struct {
		name string
		edit [2]string
		key  string
		want string
	}{
		{"unknown key", [2]string{"per_subject: one", "per_subjects: one"}, "flows.onboarding", "per_subjects"},
		{"unknown subject", [2]string{"subject: person", "subject: human"}, "flows.onboarding.subject", "does not declare"},
		{"bad per_subject", [2]string{"per_subject: one", "per_subject: two"}, "flows.onboarding.per_subject", "one or many"},
		{"short deadline", [2]string{"deadline: 30d", "deadline: 10s"}, "flows.onboarding.deadline", "shorter than"},
		{"bad retention action", [2]string{"action: delete", "action: shred"}, "flows.onboarding.retention.action", "archive or delete"},
		{"unknown start", [2]string{"on: create", "on: birth"}, "flows.onboarding.start[0].on", "create, predicate"},
		{"predicate without when", [2]string{"      - on: predicate\n        when: \"entity.status == 'hired'\"", "      - on: predicate"},
			"flows.onboarding.start[1].when", "required"},
		{"manual without permission", [2]string{"permission: onboarding.start", ""}, "flows.onboarding.start[2].permission", "required"},
		{"schedule with permission", [2]string{"every: 1d", "every: 1d\n        permission: x"}, "flows.onboarding.start[3].permission", "does not apply"},
		{"guard does not compile", [2]string{"entity.documents == true\"\n        after", "entity.docs == true\"\n        after"},
			"flows.onboarding.states.collect.on.change[0].when", "does not compile"},
		{"guard on hidden field", [2]string{"entity.documents == true\"\n        after", "entity.salary > 3\"\n        after"},
			"flows.onboarding.states.collect.on.change[0].when", "visible: grant"},
		{"guard traversal", [2]string{"entity.status == 'new'", "related(entity, 'mentions')"},
			"flows.onboarding.start[3].when", "related"},
		{"guard function", [2]string{"entity.status == 'new'", "match(entity.name, 'x')"},
			"flows.onboarding.start[3].when", "may not call"},
		{"unknown target", [2]string{"done: { to: finished }", "done: { to: finish }"},
			"flows.onboarding.states.review.on.done.to", "does not declare"},
		{"unknown event", [2]string{"done: { to: finished }", "click: { to: finished }"},
			"flows.onboarding.states.review.on.click", "change or done"},
		{"done with roles", [2]string{"done: { to: finished }", "done: { to: finished, roles: [hr] }"},
			"flows.onboarding.states.review.on.done.roles", "nobody causes it"},
		{"unknown role", [2]string{"roles: [hr]", "roles: [boss]"},
			"flows.onboarding.states.collect.on.change[0].roles[0]", "does not declare"},
		{"waiting role cannot answer", [2]string{"waiting_for: [hr]", "waiting_for: [manager]"},
			"flows.onboarding.states.collect.waiting_for[0]", "no on.change entry"},
		{"unreachable state", [2]string{"to: escalated, label", "to: review, label"},
			"flows.onboarding.states.escalated", "no run reaches it"},
		{"initial final", [2]string{"initial: collect", "initial: finished"}, "flows.onboarding.initial", "final state"},
		{"initial missing", [2]string{"initial: collect", "initial: start"}, "flows.onboarding.initial", "does not declare"},
		{"final with on", [2]string{"      finished:\n        final: true", "      finished:\n        final: true\n        on: { done: { to: review } }"},
			"flows.onboarding.states.finished", "a final state has no on:"},
		{"no way out", [2]string{"        on:\n          done: { to: finished }", "        label: Review"},
			"flows.onboarding.states.review", "no way out"},
		{"bad final status", [2]string{"status: failed", "status: working"}, "flows.onboarding.states.failed.status", "completed, cancelled"},
		{"bad timer since", [2]string{"since: run", "since: session"}, "flows.onboarding.states.escalated.after.deadline.since", "state or run"},
		{"timer without period", [2]string{"period: 7d, ", ""}, "flows.onboarding.states.collect.after.stalled.period", "required"},
		{"unknown output property", [2]string{"stage: collecting", "phase: collecting"},
			"flows.onboarding.states.collect.outputs[0].set.properties.phase", "no property"},
		{"unknown create type", [2]string{"type: task", "type: chore"}, "flows.onboarding.states.collect.outputs[1].create.type", "does not declare"},
		{"manual create type", [2]string{"type: task", "type: code"}, "flows.onboarding.states.collect.outputs[1].create.type", "id_type: manual"},
		{"relate wrong way", [2]string{"relate: { type: about }", "relate: { type: mentions }"},
			"flows.onboarding.states.collect.outputs[1].create.relate.type", "does not link"},
		{"template unknown ref", [2]string{"{{subject.name}}", "{{subject.nickname}}"},
			"flows.onboarding.states.collect.outputs[1].create.properties.title", "no property"},
		{"template hidden ref", [2]string{"{{subject.name}}", "{{subject.salary}}"},
			"flows.onboarding.states.collect.outputs[1].create.properties.title", "visible: grant"},
		{"template wrong type", [2]string{"due: \"{{today}}\"", "due: \"{{now}}\""},
			"flows.onboarding.states.collect.outputs[1].create.properties.due", "only a whole placeholder"},
		{"template unclosed", [2]string{"{{subject.name}}", "{{subject.name"},
			"flows.onboarding.states.collect.outputs[1].create.properties.title", "unclosed"},
		{"template unknown root", [2]string{"{{subject.name}}", "{{user.name}}"},
			"flows.onboarding.states.collect.outputs[1].create.properties.title", "may read"},
		{"no run_as", [2]string{"    run_as: system:onboarding-bot\n", ""}, "flows.onboarding.run_as", "required"},
		{"bad run_as", [2]string{"run_as: system:onboarding-bot", "run_as: bob"}, "flows.onboarding.run_as", "bob"},
		{"run_as without grant", [2]string{"run_as: system:onboarding-bot", "run_as: system:nobody"},
			"flows.onboarding.states.collect.outputs[0].set", "cannot update person"},
		{"bad flow name", [2]string{"  onboarding:", "  Onboarding:"}, "flows.Onboarding", "a flow name"},
		{"duplicate flow", [2]string{"flows:\n", "flows:\n  onboarding: {}\n"}, "flows.onboarding", "declared twice"},
		{"duplicate state", [2]string{"      finished:\n        final: true", "      finished:\n        final: true\n      finished:\n        final: true"},
			"flows.onboarding.states.finished", "declared twice"},
		{"duplicate timer", [2]string{"          stalled: { period: 7d", "          stalled: { period: 1d, to: escalated }\n          stalled: { period: 7d"},
			"flows.onboarding.states.collect.after.stalled", "declared twice"},
		{"done cycle", [2]string{"done: { to: finished }", "done: { to: review }"},
			"flows.onboarding.states.review.on.done", "cycle of done branches"},
		{"empty change list", [2]string{"        on:\n          done: { to: finished }", "        on:\n          change: []"},
			"flows.onboarding.states.review", "no way out"},
		{"id into date", [2]string{"due: \"{{today}}\"", "due: \"{{subject.id}}\""},
			"flows.onboarding.states.collect.outputs[1].create.properties.due", "only a whole placeholder"},
		{"template rollup", [2]string{"{{subject.name}}", "{{subject.task_count}}"},
			"flows.onboarding.states.collect.outputs[1].create.properties.title", "roll-up"},
		{"guard rollup", [2]string{"entity.documents == true\"\n        after", "entity.task_count > 3\"\n        after"},
			"flows.onboarding.states.collect.on.change[0].when", "does not compile"},
		{"literal wrong type", [2]string{"stage: collecting", "documents: maybe"},
			"flows.onboarding.states.collect.outputs[0].set.properties.documents", "true or false"},
		{"literal not an option", [2]string{"stage: collecting", "status: fired"},
			"flows.onboarding.states.collect.outputs[0].set.properties.status", "allowed"},
		{"create leaves required unset", [2]string{"                title: \"Collect documents for {{subject.name}}\"\n", ""},
			"flows.onboarding.states.collect.outputs[1].create.properties", "required"},
		{"schedule with many", [2]string{"per_subject: one", "per_subject: many"},
			"flows.onboarding.start[3]", "per_subject: one"},
		{"two sets", [2]string{"          - create:", "          - set: { properties: { stage: x } }\n          - create:"},
			"flows.onboarding.states.collect.outputs[1]", "at most one set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSample(t, tt.edit)
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("Parse = %v; want a load error", err)
			}
			for _, p := range e.Problems {
				if (tt.key == "" || p.Key == tt.key) && strings.Contains(p.Message, tt.want) {
					return
				}
			}
			t.Errorf("problems %v; want %s: ...%s...", e.Problems, tt.key, tt.want)
		})
	}
}

func TestParse_Warnings(t *testing.T) {
	c, err := parseSample(t, [2]string{"permission: onboarding.start", "permission: nobody.has"})
	if err != nil {
		t.Fatal(err)
	}
	if w := c.Warnings(); len(w) != 1 || w[0].Key != "flows.onboarding.start[2].permission" {
		t.Errorf("warnings = %v", w)
	}
}

func TestParse_NoStartWarns(t *testing.T) {
	src := string(readFile(t, "flows.yaml"))
	head, rest, _ := strings.Cut(src, "    start:\n")
	_, tail, _ := strings.Cut(rest, "    initial:")
	c, err := Parse([]byte(head+"    initial:"+tail), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if w := c.Warnings(); len(w) != 1 || w[0].Key != "flows.onboarding.start" {
		t.Errorf("warnings = %v", w)
	}
}

func TestParse_NullAndAlias(t *testing.T) {
	c, err := Parse([]byte("flows:\n"), testMeta(t), nil)
	if err != nil || len(c.Flows()) != 0 {
		t.Errorf("flows: null = %v, %v; want no flows", c.Flows(), err)
	}
	src := "flows:\n  a: &x\n    subject: person\n  b: *x\n"
	_, err = Parse([]byte(src), testMeta(t), nil)
	var e *Error
	if !errors.As(err, &e) || !strings.Contains(e.Error(), "anchor or alias") {
		t.Errorf("alias = %v; want refused", err)
	}
}

func TestTemplate_Eval(t *testing.T) {
	c, err := parseSample(t)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := c.Flow("onboarding")
	st, _ := f.State("collect")
	create := st.Outputs()[1]
	props := create.Properties()
	now := time.Date(2026, 11, 6, 9, 30, 0, 0, time.UTC)
	vars := Vars{SubjectID: "PER-1", RunID: "FR-1", Now: now, Subject: map[string]any{"name": "Ada"}}
	if v, ok := props["title"].Eval(vars); !ok || v != "Collect documents for Ada" {
		t.Errorf("title = %v, %v", v, ok)
	}
	if v, _ := props["due"].Eval(vars); v != "2026-11-06" {
		t.Errorf("due = %v", v)
	}
	if v, _ := props["opened"].Eval(vars); v != "2026-11-06T09:30:00Z" {
		t.Errorf("opened = %v", v)
	}
	if refs := props["title"].Refs(); len(refs) != 1 || refs[0] != "subject.name" {
		t.Errorf("refs = %v", refs)
	}
	set := st.Outputs()[0].Properties()["stage"]
	if v, ok := set.Eval(vars); !set.Literal() || !ok || v != "collecting" {
		t.Errorf("stage = %v", v)
	}
	whole := Template{parts: []templatePart{{ref: "subject.started"}}}
	if _, ok := whole.Eval(vars); ok {
		t.Error("a whole placeholder of an unset property evaluated")
	}
	if v, _ := (Template{parts: []templatePart{{ref: RefRunID}}}).Eval(vars); v != "FR-1" {
		t.Errorf("run id = %v", v)
	}
}

func TestFlow_Mermaid(t *testing.T) {
	c, err := parseSample(t)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := c.Flow("onboarding")
	m := f.Mermaid()
	for _, want := range []string{"stateDiagram-v2", "change by hr when entity.documents == true", "after 7d", "done"} {
		if !strings.Contains(m, want) {
			t.Errorf("mermaid lacks %q:\n%s", want, m)
		}
	}
}
