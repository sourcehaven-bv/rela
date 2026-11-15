package projectsetup_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const atfRoot = "/proj"

// atfProject returns a memfs project whose schema.yaml is schema, plus the
// given extra files (paths relative to the project root).
func atfProject(t *testing.T, schema string, extra map[string]string) storage.FS {
	t.Helper()
	fs := storage.NewMemFS()
	if _, err := projectsetup.InitializeWithFS(atfRoot, fs); err != nil {
		t.Fatalf("init: %v", err)
	}
	atfWrite(t, fs, "schema.yaml", schema)
	for p, c := range extra {
		atfWrite(t, fs, p, c)
	}
	return fs
}

func atfWrite(t *testing.T, fs storage.FS, rel, content string) {
	t.Helper()
	p := rel
	if !filepath.IsAbs(rel) {
		p = filepath.Join(atfRoot, rel)
	}
	if err := fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := fs.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func atfRead(t *testing.T, fs storage.FS, rel string) string {
	t.Helper()
	p := rel
	if !filepath.IsAbs(rel) {
		p = filepath.Join(atfRoot, rel)
	}
	b, err := fs.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

// assertSameYAML compares two YAML documents by decoded value.
func assertSameYAML(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := yaml.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("got does not parse: %v\n%s", err, got)
	}
	if err := yaml.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want does not parse: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("YAML differs\n--- got\n%s\n--- want\n%s", got, want)
	}
}

const atfPrefix = `version: "1"
# entity types
entities:
  ticket:
    id_type: sequential
    properties:
      status: {type: string}

`

const atfSuffix = `# rules
validations:
  - name: v1
    entity: ticket
    rule: "status != ''"
`

func TestConvertAutomationsYAML_EveryKey(t *testing.T) {
	schema := `automations:
  - name: all-keys
    description: every key
    on:
      entity: [ticket, bug]
      property: status
      becomes: done
      from: open
      when: ["kind=bug"]
      condition: "entity.priority == 'high'"
      faces: [draft]
      copy: promote
    do:
      - set: closed_by
        value: "{{user}}"
      - create_relation: {relation: blocks, to: "{{new.parent}}"}
      - create_entity:
          type: checklist
          template: "{{new.kind}}"
          properties: {title: "T {{new.title}}"}
          relation: has-checklist
          if_exists: skip
      - add_to_pile: {pile: inbox, owner: "{{new.assignee}}", create: false}
      - notify: {pile: watch, to: {via: [watches], type: person, property: login}, reason: changed}
      - lua: "rela.log('x')"
        allow_acl_bypass: read
        capabilities: {http: true}
      - lua_file: hooks/x.lua
        allow_acl_bypass: read+write
        capabilities: {secrets: [k]}
        background: true
        run_as: system:bot
        retry: persistent
      - notify: {pile: watch, to: {property: owner}}
        set: touched
        value: "yes"
        add_to_pile: {pile: p}
        create_relation: {relation: r, to: t}
    validate:
      - check: "status=done"
        severity: warning
        message: must be done
  - name: other
    on: {created: true, updated: true, deleted: true, relation_created: a, relation_removed: b}
    do:
      - set: x
`
	want := `triggers:
  all-keys:
    description: every key
    on:
      entity: [ticket, bug]
      property: status
      becomes: done
      from: open
      where: ["kind=bug"]
      when: "entity.priority == 'high'"
      faces: [draft]
      copy: promote
    outputs:
      - set: {properties: {closed_by: "{{user}}"}}
      - relate: {type: blocks, to: "{{new.parent}}"}
      - create:
          type: checklist
          template: "{{new.kind}}"
          properties: {title: "T {{new.title}}"}
          relate: {type: has-checklist, from: subject}
          if_exists: skip
      - add_to_pile: {pile: inbox, owner: "{{new.assignee}}", create: false}
      - notify: {pile: watch, to: {via: [watches], type: person, property: login}, reason: changed}
      - script: {lua: "rela.log('x')", allow_acl_bypass: read, capabilities: {http: true}}
      - script:
          file: hooks/x.lua
          allow_acl_bypass: read+write
          capabilities: {secrets: [k]}
          background: true
          run_as: system:bot
          retry: persistent
      - set: {properties: {touched: "yes"}}
      - relate: {type: r, to: t}
      - add_to_pile: {pile: p}
      - notify: {pile: watch, to: {property: owner}}
      - check: {where: "status=done", severity: warning, message: must be done}
  other:
    on: {created: true, updated: true, deleted: true, relation_created: a, relation_removed: b}
    outputs:
      - set: {properties: {x: ""}}
`
	got, err := projectsetup.ConvertAutomationsYAML([]byte(schema))
	if err != nil {
		t.Fatalf("ConvertAutomationsYAML: %v", err)
	}
	assertSameYAML(t, string(got), want)
}

func TestConvertAutomationsYAML_NoAutomations(t *testing.T) {
	got, err := projectsetup.ConvertAutomationsYAML([]byte("entities: {}\n"))
	if err != nil {
		t.Fatalf("ConvertAutomationsYAML: %v", err)
	}
	assertSameYAML(t, string(got), "triggers: {}\n")
}

const atfBlock = `# Automations
automations:
  # Close the loop.
  - name: close # trailing name
    on:
      entity: ticket
      property: status
      becomes: done
    do:
      # stamp it
      - set: closed
        value: "{{today}}"
`

func TestMigrate_MovesAutomationsAndKeepsSchemaRest(t *testing.T) {
	fs := atfProject(t, atfPrefix+atfBlock+atfSuffix, nil)

	res, err := projectsetup.MigrateWithFS(atfRoot, fs)
	if err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	if res.AutomationsMoved != 1 {
		t.Errorf("AutomationsMoved = %d, want 1", res.AutomationsMoved)
	}
	if got := atfRead(t, fs, "schema.yaml"); got != atfPrefix+atfSuffix {
		t.Errorf("schema rest changed:\n%q\nwant\n%q", got, atfPrefix+atfSuffix)
	}
	flows := atfRead(t, fs, "flows.yaml")
	for _, c := range []string{"# Close the loop.", "# trailing name", "# stamp it"} {
		if !strings.Contains(flows, c) {
			t.Errorf("comment %q lost:\n%s", c, flows)
		}
	}
	assertSameYAML(t, flows, `triggers:
  close:
    on: {entity: ticket, property: status, becomes: done}
    outputs:
      - set: {properties: {closed: "{{today}}"}}
`)

	// Second run: nothing to do, no byte changes.
	schemaBefore, flowsBefore := atfRead(t, fs, "schema.yaml"), flows
	res, err = projectsetup.MigrateWithFS(atfRoot, fs)
	if err != nil {
		t.Fatalf("second MigrateWithFS: %v", err)
	}
	if res.AutomationsMoved != 0 {
		t.Errorf("second run moved %d", res.AutomationsMoved)
	}
	if atfRead(t, fs, "schema.yaml") != schemaBefore || atfRead(t, fs, "flows.yaml") != flowsBefore {
		t.Error("second run changed files")
	}
	if d, err := projectsetup.DetectMigrationsWithFS(atfRoot, fs); err != nil || len(d) != 0 {
		t.Errorf("pending after migrate: %v %v", d, err)
	}
}

func TestMigrate_AutomationsLastInFile(t *testing.T) {
	fs := atfProject(t, atfPrefix+atfBlock, nil)
	if _, err := projectsetup.MigrateWithFS(atfRoot, fs); err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	if got := atfRead(t, fs, "schema.yaml"); got != atfPrefix {
		t.Errorf("schema = %q, want %q", got, atfPrefix)
	}
}

func TestMigrate_DetectsPending(t *testing.T) {
	fs := atfProject(t, atfPrefix+atfBlock, nil)
	d, err := projectsetup.DetectMigrationsWithFS(atfRoot, fs)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(d) != 1 || !strings.Contains(d[0].Migrations[0].Description, "flows.yaml under triggers:") {
		t.Errorf("detections = %+v", d)
	}
}

func TestMigrate_FlowsYAMLExisting(t *testing.T) {
	tests := []struct {
		name, flows, want string
		keepPrefix        string
	}{
		{
			name:       "flows only",
			flows:      "# my flows\nflows:\n  onboarding:\n    steps: []\n",
			keepPrefix: "# my flows\nflows:\n  onboarding:\n    steps: []\n",
			want: `flows: {onboarding: {steps: []}}
triggers:
  close:
    on: {entity: ticket, property: status, becomes: done}
    outputs: [{set: {properties: {closed: "{{today}}"}}}]
`,
		},
		{
			name:       "existing triggers",
			flows:      "triggers:\n    keep: # mine\n        on: {created: true}\n\n# flows\nflows: {}\n",
			keepPrefix: "triggers:\n    keep: # mine\n        on: {created: true}\n",
			want: `triggers:
  keep: {on: {created: true}}
  close:
    on: {entity: ticket, property: status, becomes: done}
    outputs: [{set: {properties: {closed: "{{today}}"}}}]
flows: {}
`,
		},
		{
			name:       "empty triggers",
			flows:      "flows: {}\ntriggers:\n",
			keepPrefix: "flows: {}\n",
			want: `flows: {}
triggers:
  close:
    on: {entity: ticket, property: status, becomes: done}
    outputs: [{set: {properties: {closed: "{{today}}"}}}]
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := atfProject(t, atfPrefix+atfBlock, map[string]string{"flows.yaml": tc.flows})
			if _, err := projectsetup.MigrateWithFS(atfRoot, fs); err != nil {
				t.Fatalf("MigrateWithFS: %v", err)
			}
			got := atfRead(t, fs, "flows.yaml")
			if !strings.HasPrefix(got, tc.keepPrefix) {
				t.Errorf("existing bytes not kept:\n%s", got)
			}
			assertSameYAML(t, got, tc.want)
		})
	}
}

func TestMigrate_CrashRecovery(t *testing.T) {
	converted, err := projectsetup.ConvertAutomationsYAML([]byte(atfBlock))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	t.Run("equal trigger is only cut", func(t *testing.T) {
		fs := atfProject(t, atfPrefix+atfBlock, map[string]string{"flows.yaml": string(converted)})
		res, err := projectsetup.MigrateWithFS(atfRoot, fs)
		if err != nil {
			t.Fatalf("MigrateWithFS: %v", err)
		}
		if res.AutomationsMoved != 1 {
			t.Errorf("AutomationsMoved = %d", res.AutomationsMoved)
		}
		if atfRead(t, fs, "flows.yaml") != string(converted) {
			t.Error("flows.yaml changed")
		}
		if atfRead(t, fs, "schema.yaml") != atfPrefix {
			t.Error("schema not cut")
		}
	})

	t.Run("different trigger refuses", func(t *testing.T) {
		flows := "triggers:\n  close:\n    on: {created: true}\n"
		fs := atfProject(t, atfPrefix+atfBlock, map[string]string{"flows.yaml": flows})
		_, err := projectsetup.MigrateWithFS(atfRoot, fs)
		if err == nil || !strings.Contains(err.Error(), `"close"`) {
			t.Fatalf("err = %v, want conflict naming close", err)
		}
		if atfRead(t, fs, "flows.yaml") != flows || atfRead(t, fs, "schema.yaml") != atfPrefix+atfBlock {
			t.Error("refusal wrote files")
		}
	})

	// Unnamed automations are named by their position across the root and
	// the includes. A crash after flows.yaml is written and some files are
	// cut must not rename the uncut ones on the re-run, or the re-run would
	// append a second copy of each under a new name.
	t.Run("unnamed in includes after a partial cut", func(t *testing.T) {
		const (
			rootAuto = "automations:\n  - name: first\n    on: {entity: ticket, updated: true}\n" +
				"  - on: {entity: ticket, created: true}\n"
			aBody = "entities:\n  bug: {}\n"
			aAuto = "automations:\n  - on: {entity: bug, created: true}\n"
			bBody = "entities:\n  task: {}\n"
			bAuto = "automations:\n  - on: {entity: task, created: true}\n  - on: {entity: task, deleted: true}\n"
		)
		root := atfPrefix + "includes: [a.yaml, b.yaml]\n"
		full := map[string]string{"a.yaml": aBody + aAuto, "b.yaml": bBody + bAuto}

		clean := atfProject(t, root+rootAuto, full)
		if _, err := projectsetup.MigrateWithFS(atfRoot, clean); err != nil {
			t.Fatalf("clean MigrateWithFS: %v", err)
		}
		wantFlows := atfRead(t, clean, "flows.yaml")

		// Crash on each schema file write in turn: flows.yaml is written by
		// then, and the files cut before the crash stay cut.
		for _, crashAt := range []string{"schema.yaml", "a.yaml", "b.yaml"} {
			t.Run("crash writing "+crashAt, func(t *testing.T) {
				fs := atfProject(t, root+rootAuto, full)
				crashing := &failWriteFS{FS: fs, path: filepath.Join(atfRoot, crashAt)}
				if _, err := projectsetup.MigrateWithFS(atfRoot, crashing); err == nil {
					t.Fatal("first run did not fail")
				}
				if atfRead(t, fs, "flows.yaml") != wantFlows {
					t.Fatal("flows.yaml not written before the crash")
				}
				res, err := projectsetup.MigrateWithFS(atfRoot, fs)
				if err != nil {
					t.Fatalf("re-run MigrateWithFS: %v", err)
				}
				if got := atfRead(t, fs, "flows.yaml"); got != wantFlows {
					t.Errorf("re-run changed flows.yaml\n--- got\n%s\n--- want\n%s", got, wantFlows)
				}
				for _, p := range []string{"schema.yaml", "a.yaml", "b.yaml"} {
					if got, want := atfRead(t, fs, p), atfRead(t, clean, p); got != want {
						t.Errorf("%s = %q, want %q", p, got, want)
					}
				}
				if res.AutomationsMoved == 0 {
					t.Error("AutomationsMoved = 0, want the recovered automations counted")
				}
			})
		}
	})
}

// failWriteFS fails every write to path.
type failWriteFS struct {
	storage.FS
	path string
}

func (f *failWriteFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if path == f.path {
		return errors.New("simulated crash")
	}
	return f.FS.WriteFile(path, data, perm)
}

func TestMigrate_Refusals(t *testing.T) {
	tests := []struct {
		name, automations, want string
	}{
		{"unknown key", "  - name: a\n    on: {created: true}\n    oops: 1\n", `automation "a": unknown key "oops"`},
		{"unknown nested key", "  - name: a\n    on: {created: true, bogus: x}\n", `unknown key "bogus"`},
		{"anchor", "  - name: a\n    on: &t {created: true}\n", `key "on" declares a YAML anchor`},
		{"alias", "  - name: a\n    on: &t {created: true}\n  - name: b\n    on: *t\n", "YAML"},
		{
			"create_entity with lua",
			"  - name: a\n    on: {created: true}\n    do:\n      - create_entity: {type: x}\n        lua: \"x()\"\n",
			"split them into two do: items",
		},
		{"dot in name", "  - name: a.b\n    on: {created: true}\n", "may not contain '.' or ':'"},
		{"colon in name", "  - name: \"a:b\"\n    on: {created: true}\n", "may not contain '.' or ':'"},
		{"duplicate name", "  - name: a\n    on: {created: true}\n  - name: a\n    on: {updated: true}\n", "already used"},
		{"mapping instead of list", "  name: a\n  on: {created: true}\n", "automations: is not a list"},
		{"scalar instead of list", "  a\n", "automations: is not a list"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := atfPrefix + "automations:\n" + tc.automations
			fs := atfProject(t, schema, nil)
			_, err := projectsetup.MigrateWithFS(atfRoot, fs)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "schema.yaml") {
				t.Errorf("error does not name the file: %v", err)
			}
			if atfRead(t, fs, "schema.yaml") != schema {
				t.Error("schema changed on refusal")
			}
			if atfRead(t, fs, "flows.yaml") != "" {
				t.Error("flows.yaml written on refusal")
			}
		})
	}
}

func TestMigrate_UnnamedNaming(t *testing.T) {
	schema := atfPrefix + `automations:
  - on: {created: true}
  - name: automation-3
    on: {updated: true}
  - on: {deleted: true}
`
	fs := atfProject(t, schema, nil)
	if _, err := projectsetup.MigrateWithFS(atfRoot, fs); err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}
	assertSameYAML(t, atfRead(t, fs, "flows.yaml"), `triggers:
  automation-1: {on: {created: true}}
  automation-3: {on: {updated: true}}
  automation-4: {on: {deleted: true}}
`)
}

func TestMigrate_Includes(t *testing.T) {
	t.Run("in project", func(t *testing.T) {
		schema := atfPrefix + "includes: [lib/extra.yaml]\n" + atfBlock
		inc := "entities:\n  bug: {}\nautomations:\n  - name: bug-hook\n    on: {entity: bug, created: true}\n" +
			"    do:\n      - lua_file: b.lua\n        allow_acl_bypass: true\n"
		fs := atfProject(t, schema, map[string]string{"lib/extra.yaml": inc})

		d, err := projectsetup.DetectMigrationsWithFS(atfRoot, fs)
		if err != nil || len(d) != 2 || d[1].File.Name != "lib/extra.yaml" {
			t.Fatalf("detections = %+v, %v", d, err)
		}

		res, err := projectsetup.MigrateWithFS(atfRoot, fs)
		if err != nil {
			t.Fatalf("MigrateWithFS: %v", err)
		}
		if res.AutomationsMoved != 2 {
			t.Errorf("AutomationsMoved = %d", res.AutomationsMoved)
		}
		if got := atfRead(t, fs, "lib/extra.yaml"); strings.Contains(got, "automations") {
			t.Errorf("include not cut:\n%s", got)
		}
		assertSameYAML(t, atfRead(t, fs, "flows.yaml"), `triggers:
  close:
    on: {entity: ticket, property: status, becomes: done}
    outputs: [{set: {properties: {closed: "{{today}}"}}}]
  bug-hook:
    on: {entity: bug, created: true}
    outputs: [{script: {file: b.lua, allow_acl_bypass: read+write}}]
`)
	})

	t.Run("namespace refused", func(t *testing.T) {
		schema := atfPrefix + "includes: [lib.yaml]\n"
		inc := "namespace: lib\nautomations:\n  - name: x\n    on: {created: true}\n"
		fs := atfProject(t, schema, map[string]string{"lib.yaml": inc})
		_, err := projectsetup.MigrateWithFS(atfRoot, fs)
		if err == nil || !strings.Contains(err.Error(), "lib.yaml") || !strings.Contains(err.Error(), "namespace:") {
			t.Fatalf("err = %v", err)
		}
		if atfRead(t, fs, "lib.yaml") != inc || atfRead(t, fs, "flows.yaml") != "" {
			t.Error("refusal wrote files")
		}
	})

	t.Run("outside project refused", func(t *testing.T) {
		schema := atfPrefix + "includes: [../shared.yaml]\n"
		inc := "automations:\n  - name: x\n    on: {created: true}\n"
		fs := atfProject(t, schema, map[string]string{"/shared.yaml": inc})
		_, err := projectsetup.MigrateWithFS(atfRoot, fs)
		if err == nil || !strings.Contains(err.Error(), "outside the project directory") {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestMigrate_InRepoSchemas converts frozen copies of the in-repo schemas as
// they were before the move (tickets, examples/basecamp and the test
// fixtures), then checks a second run is a no-op. The copies live in
// testdata/automations because the originals have been migrated.
func TestMigrate_InRepoSchemas(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "automations", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no schemas in testdata/automations")
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !strings.Contains(string(data), "\nautomations:") {
				t.Fatalf("%s has no automations", p)
			}
			if _, convErr := projectsetup.ConvertAutomationsYAML(data); convErr != nil {
				t.Fatalf("ConvertAutomationsYAML: %v", convErr)
			}
			fs := atfProject(t, string(data), nil)
			res, err := projectsetup.MigrateWithFS(atfRoot, fs)
			if err != nil {
				t.Fatalf("MigrateWithFS: %v", err)
			}
			if res.AutomationsMoved == 0 {
				t.Error("nothing moved")
			}
			schema, flows := atfRead(t, fs, "schema.yaml"), atfRead(t, fs, "flows.yaml")
			if strings.Contains(schema, "\nautomations:") {
				t.Error("schema.yaml still declares automations:")
			}
			meta, err := metamodel.Parse([]byte(schema))
			if err != nil {
				t.Fatalf("parse migrated schema: %v", err)
			}
			rules, err := triggers.Parse([]byte(flows), meta)
			if err != nil {
				t.Fatalf("parse migrated triggers: %v", err)
			}
			if len(rules) != res.AutomationsMoved {
				t.Errorf("%d triggers parsed, %d automations moved", len(rules), res.AutomationsMoved)
			}
			if _, err := projectsetup.MigrateWithFS(atfRoot, fs); err != nil {
				t.Fatalf("second MigrateWithFS: %v", err)
			}
			if atfRead(t, fs, "schema.yaml") != schema || atfRead(t, fs, "flows.yaml") != flows {
				t.Error("second run changed files")
			}
		})
	}
}
