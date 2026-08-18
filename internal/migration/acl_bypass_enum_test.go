package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseDoc(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &doc
}

func renderDoc(t *testing.T, doc *yaml.Node) string {
	t.Helper()
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

const bypassTrueYAML = `entities:
  ticket:
    properties:
      status:
        type: string
automations:
  - name: stamp-author
    trigger: on_create
    actions:
      - lua_file: stamp.lua
        allow_acl_bypass: true
`

// TestACLBypassEnum_RewritesTrue pins the capability-preserving rewrite:
// `true` meant reads AND writes, so it must become read+write, not read.
func TestACLBypassEnum_RewritesTrue(t *testing.T) {
	t.Parallel()

	doc := parseDoc(t, bypassTrueYAML)
	m := &ACLBypassEnumMigration{}
	if !m.Detect(doc) {
		t.Fatal("Detect() = false on a document containing allow_acl_bypass: true")
	}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := renderDoc(t, doc)
	if !strings.Contains(got, "allow_acl_bypass: read+write") {
		t.Errorf("migrated document does not contain read+write:\n%s", got)
	}
	if strings.Contains(got, "allow_acl_bypass: true") {
		t.Errorf("boolean survived the migration:\n%s", got)
	}
}

// TestACLBypassEnum_MigratedOutputParses is the round trip that matters: the
// rewritten file must load through the real enum parser. A migration whose
// output the parser still rejects would leave the project unloadable.
func TestACLBypassEnum_MigratedOutputParses(t *testing.T) {
	t.Parallel()

	doc := parseDoc(t, bypassTrueYAML)
	m := &ACLBypassEnumMigration{}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Decode the action through a struct carrying the real ACLBypass type.
	// Declared locally rather than importing metamodel: migration must not
	// depend on it (that is the package-boundary rule), so this mirrors the
	// shape and the aclbypass_test.go suite pins the parser itself.
	var probe struct {
		Automations []struct {
			Actions []struct {
				AllowACLBypass string `yaml:"allow_acl_bypass"`
			} `yaml:"actions"`
		} `yaml:"automations"`
	}
	if err := doc.Decode(&probe); err != nil {
		t.Fatalf("decode migrated doc: %v", err)
	}
	got := probe.Automations[0].Actions[0].AllowACLBypass
	if got != "read+write" {
		t.Errorf("migrated value = %q, want %q", got, "read+write")
	}
}

// TestACLBypassEnum_DropsFalse pins that a falsy value is removed rather than
// rewritten: it granted nothing, which is exactly the absent-key default, so
// keeping a key that grants nothing is noise.
func TestACLBypassEnum_DropsFalse(t *testing.T) {
	t.Parallel()

	doc := parseDoc(t, strings.Replace(bypassTrueYAML, "allow_acl_bypass: true", "allow_acl_bypass: false", 1))
	m := &ACLBypassEnumMigration{}
	if !m.Detect(doc) {
		t.Fatal("Detect() = false on allow_acl_bypass: false")
	}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := renderDoc(t, doc); strings.Contains(got, "allow_acl_bypass") {
		t.Errorf("falsy key survived:\n%s", got)
	}
}

// TestACLBypassEnum_YAMLBoolSpellings covers the YAML 1.1 spellings that also
// decode as booleans. The parser rejects every one, so the migration has to
// catch every one — an operator who wrote `yes` is stuck exactly as badly as
// one who wrote `true`.
func TestACLBypassEnum_YAMLBoolSpellings(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want string // "" means the key should be dropped
	}{
		{in: "true", want: "read+write"},
		{in: "yes", want: "read+write"},
		{in: "on", want: "read+write"},
		{in: "True", want: "read+write"},
		{in: "false"},
		{in: "no"},
		{in: "off"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(bypassTrueYAML, "allow_acl_bypass: true",
				"allow_acl_bypass: "+tc.in, 1)
			doc := parseDoc(t, src)
			m := &ACLBypassEnumMigration{}
			if !m.Detect(doc) {
				t.Fatalf("Detect() = false for %q", tc.in)
			}
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got := renderDoc(t, doc)
			if tc.want == "" {
				if strings.Contains(got, "allow_acl_bypass") {
					t.Errorf("%q should have been dropped:\n%s", tc.in, got)
				}
				return
			}
			if !strings.Contains(got, "allow_acl_bypass: "+tc.want) {
				t.Errorf("%q did not become %q:\n%s", tc.in, tc.want, got)
			}
		})
	}
}

// TestACLBypassEnum_LeavesEnumValuesAlone pins idempotency: running the
// migration over an already-migrated file must be a no-op, and must not
// mistake the valid value `read` for something needing rewriting.
func TestACLBypassEnum_LeavesEnumValuesAlone(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"read", "write", "read+write"} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(bypassTrueYAML, "allow_acl_bypass: true",
				"allow_acl_bypass: "+v, 1)
			doc := parseDoc(t, src)
			m := &ACLBypassEnumMigration{}
			if m.Detect(doc) {
				t.Errorf("Detect() = true for already-valid value %q", v)
			}
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if got := renderDoc(t, doc); !strings.Contains(got, "allow_acl_bypass: "+v) {
				t.Errorf("value %q was altered:\n%s", v, got)
			}
		})
	}
}

// TestACLBypassEnum_NoKeyIsNoOp pins that the overwhelmingly common file —
// one with no elevation anywhere — is untouched.
func TestACLBypassEnum_NoKeyIsNoOp(t *testing.T) {
	t.Parallel()

	src := "entities:\n  ticket:\n    properties:\n      status:\n        type: string\n"
	doc := parseDoc(t, src)
	m := &ACLBypassEnumMigration{}
	if m.Detect(doc) {
		t.Error("Detect() = true on a document with no allow_acl_bypass")
	}
}

// TestACLBypassEnum_MultipleActions pins that every occurrence migrates, not
// just the first — a file with one migrated and one legacy action would fail
// to load with a confusing error pointing at the wrong place.
func TestACLBypassEnum_MultipleActions(t *testing.T) {
	t.Parallel()

	src := `automations:
  - name: one
    actions:
      - lua_file: a.lua
        allow_acl_bypass: true
  - name: two
    actions:
      - lua_file: b.lua
        allow_acl_bypass: yes
      - lua_file: c.lua
        allow_acl_bypass: read
`
	doc := parseDoc(t, src)
	m := &ACLBypassEnumMigration{}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := renderDoc(t, doc)
	if n := strings.Count(got, "allow_acl_bypass: read+write"); n != 2 {
		t.Errorf("read+write count = %d, want 2:\n%s", n, got)
	}
	if !strings.Contains(got, "allow_acl_bypass: read\n") {
		t.Errorf("pre-migrated `read` was altered:\n%s", got)
	}
}
