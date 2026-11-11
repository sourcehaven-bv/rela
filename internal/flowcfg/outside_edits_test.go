package flowcfg

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// withEdits returns the sample with block added to the onboarding flow as
// its `outside_edits:`.
func withEdits(block string) [2]string {
	const at = "    initial: collect\n"
	return [2]string{at, at + "    outside_edits:\n" + block}
}

func TestParse_OutsideEdits(t *testing.T) {
	tests := []struct {
		name string
		// block is the `outside_edits:` body, indented by six spaces.
		block string
		key   string
		want  string
		warn  string
		// none marks a block that compiles to no rules.
		none bool
	}{
		{name: "absent", block: "", none: true},
		{name: "no rules", block: "      states: {}\n", warn: "flows.onboarding.outside_edits", none: true},
		{name: "flow default and properties", block: "      default: deny\n" +
			"      properties:\n        documents: allow\n        salary: { permission: onboarding.start }\n"},
		{name: "state override", block: "      states:\n        collect:\n          default: deny\n" +
			"          properties:\n            name: allow\n"},
		{name: "bad default", block: "      default: maybe\n",
			key: "flows.onboarding.outside_edits.default", want: "allow, deny or {permission"},
		{name: "bad property value", block: "      properties:\n        name: [deny]\n",
			key: "flows.onboarding.outside_edits.properties.name", want: "allow, deny or {permission"},
		{name: "unknown rule key", block: "      properties:\n        name: { role: hr }\n",
			key: "flows.onboarding.outside_edits.properties.name", want: "allow, deny or {permission"},
		{name: "empty permission", block: "      properties:\n        name: { permission: \"\" }\n",
			key: "flows.onboarding.outside_edits.properties.name.permission", want: "is required"},
		{name: "permission nobody holds", block: "      properties:\n        name: { permission: nobody.has }\n",
			warn: "flows.onboarding.outside_edits.properties.name.permission"},
		{name: "unknown property", block: "      properties:\n        namez: deny\n",
			key: "flows.onboarding.outside_edits.properties.namez", want: "no property"},
		{name: "derived property", block: "      properties:\n        task_count: deny\n",
			key: "flows.onboarding.outside_edits.properties.task_count", want: "derived"},
		{name: "reserved property", block: "      properties:\n        _x: deny\n",
			key: "flows.onboarding.outside_edits.properties._x", want: "no property"},
		{name: "unknown key", block: "      defaults: deny\n",
			key: "flows.onboarding.outside_edits", want: "defaults"},
		{name: "unknown state", block: "      states:\n        nowhere: { default: deny }\n",
			key: "flows.onboarding.outside_edits.states.nowhere", want: "no state"},
		{name: "final state", block: "      states:\n        finished: { default: deny }\n",
			key: "flows.onboarding.outside_edits.states.finished", want: "final"},
		{name: "nested states", block: "      states:\n        collect:\n          states: {}\n",
			key: "flows.onboarding.outside_edits.states.collect.states", want: "only the flow level"},
		{name: "not a mapping", block: "      - deny\n",
			key: "flows.onboarding.outside_edits", want: "cannot unmarshal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edit := withEdits(tc.block)
			if tc.block == "" {
				edit = [2]string{"initial: collect", "initial: collect"}
			}
			c, err := parseSample(t, edit)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				f, _ := c.Flow("onboarding")
				if (f.OutsideEdits() == nil) != tc.none {
					t.Errorf("OutsideEdits() = %v for block %q", f.OutsideEdits(), tc.block)
				}
				gotWarn := ""
				for _, w := range c.Warnings() {
					if strings.Contains(w.Key, "outside_edits") {
						gotWarn = w.Key
					}
				}
				if gotWarn != tc.warn {
					t.Errorf("warning at %q, want %q", gotWarn, tc.warn)
				}
				return
			}
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want a load error", err)
			}
			if !strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q at %s", err, tc.want, tc.key)
			}
		})
	}
}

func TestParse_OutsideEditsTooMany(t *testing.T) {
	var b strings.Builder
	b.WriteString("      properties:\n")
	for i := range MaxOutsideEditRules + 1 {
		fmt.Fprintf(&b, "        p%d: deny\n", i)
	}
	_, err := parseSample(t, withEdits(b.String()))
	if err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v, want at most", err)
	}
}

// roleless is a flow that names no role, so it loads without acl.yaml.
const roleless = `flows:
  simple:
    subject: person
    start: [{on: create}]
    initial: open
    states:
      open:
        on: { change: { to: done, when: "entity.documents == true" } }
      done: { final: true }
`

func TestParse_OutsideEditsNeedsACL(t *testing.T) {
	if _, err := Parse([]byte(roleless), testMeta(t), nil); err != nil {
		t.Fatalf("Parse without outside_edits: %v", err)
	}
	src := roleless + "    outside_edits: { default: deny }\n"
	_, err := Parse([]byte(src), testMeta(t), nil)
	if err == nil || !strings.Contains(err.Error(), "flows.simple.outside_edits: needs acl.yaml") {
		t.Fatalf("err = %v, want needs acl.yaml", err)
	}
	if _, err := Parse([]byte(src), testMeta(t), testPolicy(t)); err != nil {
		t.Fatalf("Parse with acl.yaml: %v", err)
	}
}

// TestParseFlow_IgnoresStoredOutsideEdits: a run's stored declaration
// loads whatever its `outside_edits:` says, because the rules in force come
// from the current flows.yaml. A rule that today's schema rejects must not
// strand the run.
func TestParseFlow_IgnoresStoredOutsideEdits(t *testing.T) {
	src := roleless + "    outside_edits: { default: deny }\n"
	c, err := Parse([]byte(src), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f, _ := c.Flow("simple")
	stored := []byte(strings.Replace(string(f.Source()), "default: deny", "properties: { gone: deny }", 1))
	if bytes.Equal(stored, f.Source()) {
		t.Fatalf("source has no outside_edits to replace: %s", f.Source())
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(stored, &doc); err != nil {
		t.Fatalf("unmarshal stored: %v", err)
	}
	want, _ := version(doc.Content[0])
	got, err := ParseFlow("simple", stored, want, testMeta(t), nil)
	if err != nil {
		t.Fatalf("ParseFlow: %v", err)
	}
	if got.OutsideEdits() != nil {
		t.Errorf("ParseFlow compiled the stored rules")
	}
}

func TestOutsideEdits_Rule(t *testing.T) {
	block := "      default: deny\n" +
		"      properties:\n" +
		"        documents: allow\n" +
		"        salary: { permission: onboarding.start }\n" +
		"      states:\n" +
		"        collect:\n" +
		"          default: allow\n" +
		"          properties:\n" +
		"            stage: deny\n" +
		"        review:\n" +
		"          properties:\n" +
		"            salary: deny\n"
	c, err := parseSample(t, withEdits(block))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f, _ := c.Flow("onboarding")
	o := f.OutsideEdits()
	perm := EditRule{Kind: EditPermission, Permission: "onboarding.start"}
	tests := []struct {
		state, property string
		want            EditRule
	}{
		{"collect", "stage", EditRule{Kind: EditDeny}},   // state property rule
		{"collect", "salary", EditRule{Kind: EditAllow}}, // state default beats flow property rule
		{"collect", "name", EditRule{Kind: EditAllow}},   // state default
		{"review", "salary", EditRule{Kind: EditDeny}},   // state property rule beats flow rule
		{"review", "documents", EditRule{Kind: EditAllow}},
		{"review", "name", EditRule{Kind: EditDeny}},         // no state default: flow default
		{"escalated", "salary", perm},                        // state without rules: flow property rule
		{"escalated", "name", EditRule{Kind: EditDeny}},      // flow default
		{"finished", "documents", EditRule{Kind: EditAllow}}, // final state: flow rules
		{"gone", "name", EditRule{Kind: EditDeny}},           // a state no longer declared: flow rules
	}
	for _, tc := range tests {
		t.Run(tc.state+"/"+tc.property, func(t *testing.T) {
			if got := o.Rule(tc.state, tc.property); got != tc.want {
				t.Errorf("Rule = %+v, want %+v", got, tc.want)
			}
		})
	}

	if got := o.Permissions(); !slices.Equal(got, []string{"onboarding.start"}) {
		t.Errorf("Permissions = %v, want [onboarding.start]", got)
	}

	var none *OutsideEdits
	if got := none.Rule("collect", "name"); got.Kind != EditAllow {
		t.Errorf("nil Rule = %+v, want allow", got)
	}
	if got := none.Permissions(); got != nil {
		t.Errorf("nil Permissions = %v, want none", got)
	}
	// A block with no default and no rule for the property allows it.
	c, err = parseSample(t, withEdits("      properties:\n        stage: deny\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f, _ = c.Flow("onboarding")
	if got := f.OutsideEdits().Rule("collect", "name"); got.Kind != EditAllow {
		t.Errorf("Rule without default = %+v, want allow", got)
	}
}
