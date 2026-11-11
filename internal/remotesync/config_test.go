package remotesync

import (
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
)

func TestParse_Valid(t *testing.T) {
	meta := testMeta(t)
	cfg, err := Parse([]byte(testRemotes), meta)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Compile("cat", cfg.Remotes["cat"], meta)
	if err != nil {
		t.Fatal(err)
	}
	if m.System != "catalog" || m.Entities["req"].Local != "requirement" || len(m.Entities["req"].Fields) != 3 ||
		m.Relations["depends"].Local != "depends" {

		t.Fatalf("mapping: %+v", m)
	}
	if p := cfg.Remotes["cat"]; p.onGone() != OnGoneDetach || p.maxGone() != DefaultMaxGone ||
		p.maxRows() != DefaultMaxRows || p.maxFetch() != DefaultMaxFetch {

		t.Fatalf("defaults: %+v", p)
	}
	other := testMapping(t, meta, func(p *Remote) { p.Entities["req"].Fields["note"] = "note" })
	if other.Hash == m.Hash {
		t.Fatal("a changed mapping must change the hash")
	}
	if _, err := Parse(nil, meta); err != nil {
		t.Fatalf("an empty file is valid: %v", err)
	}
	for _, cr := range []httpremote.Credential{
		{Env: "REMOTE_CREDENTIAL_CAT"}, {File: "/run/credentials/rela/remote_credential.cat"},
		{Secret: "remote_credential.cat"},
	} {
		testMapping(t, meta, func(p *Remote) { p.Credential = cr })
	}
}

func TestParse_Errors(t *testing.T) {
	meta := testMeta(t)
	base := `
remotes:
  %s:
    url: %s
    credential: %s
    %s
    entities:
      %s
`
	okEnt := `component: {ref: cat}`
	cases := []struct {
		name, remote, url, cred, extra, ents, want string
	}{
		{"bad name", "Bad", "https://x", "{secret: remote_credential.x}", "", okEnt, "invalid name"},
		{"bad url", "p", "ftp://x", "{secret: remote_credential.x}", "", okEnt, "url"},
		{"plain http", "p", "http://example.com", "{secret: remote_credential.x}", "", okEnt, "allow_insecure_http"},
		{"url credentials", "p", "https://u:p@x", "{secret: remote_credential.x}", "", okEnt, "credentials"},
		{"credential name", "p", "https://x", "{secret: token_key}", "", okEnt, "remote_credential."},
		{"no credential", "p", "https://x", "{}", "", okEnt, "name one source"},
		{"two credentials", "p", "https://x", "{env: REMOTE_CREDENTIAL_X, file: /x}", "", okEnt, "name one credential source"},
		{"env name", "p", "https://x", "{env: RELA_DATABASE_URL}", "", okEnt, "REMOTE_CREDENTIAL_"},
		{"env prefix only", "p", "https://x", "{env: REMOTE_CREDENTIAL_}", "", okEnt, "REMOTE_CREDENTIAL_"},
		{"relative file", "p", "https://x", "{file: .rela/remote_credential.x}", "", okEnt, "absolute"},
		{"file name", "p", "https://x", "{file: /run/credentials/rela/token_key}", "", okEnt, "remote_credential."},
		{"auth header", "p", "https://x", "{env: REMOTE_CREDENTIAL_X}", "auth_header: Cookie", okEnt, "auth_header"},
		{"run_as space", "p", "https://x", "{env: REMOTE_CREDENTIAL_X}", `run_as: " bot"`, okEnt, "run_as"},
		{"run_as flow", "p", "https://x", "{env: REMOTE_CREDENTIAL_X}", "run_as: system:flow", okEnt, "run_as"},
		{"on_gone", "p", "https://x", "{secret: remote_credential.x}", "on_gone: purge", okEnt, "on_gone"},
		{"max_gone", "p", "https://x", "{secret: remote_credential.x}", "max_gone: -1", okEnt, "max_gone"},
		{"max_rows", "p", "https://x", "{secret: remote_credential.x}", "max_rows: 99999999", okEnt, "max_rows"},
		{"max_fetch", "p", "https://x", "{secret: remote_credential.x}", "max_fetch: -2", okEnt, "max_fetch"},
		{"unknown key", "p", "https://x", "{secret: remote_credential.x}", "flavor: red", okEnt, "flavor"},
		{"no entities", "p", "https://x", "{secret: remote_credential.x}", "", "", "at least one"},
		{"unknown type", "p", "https://x", "{secret: remote_credential.x}", "", `x: {type: nope, ref: cat}`, "unknown local type"},
		{"ref not external", "p", "https://x", "{secret: remote_credential.x}", "", `component: {ref: name}`, "not an external_ref"},
		{"no ref", "p", "https://x", "{secret: remote_credential.x}", "", `component: {}`, "ref names no property"},
		{"undeclared field", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, fields: {nope: name}}`, "not declared"},
		{"ref as field", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, fields: {cat: cat}}`, "external ref"},
		{"bad source field", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, fields: {name: "a b"}}`, "not a valid name"},
		{"mixed systems", "p", "https://x", "{secret: remote_credential.x}", "",
			"component: {ref: cat}\n      other: {ref: ref}", "system"},
		{"same local type", "p", "https://x", "{secret: remote_credential.x}", "",
			"component: {ref: cat}\n      c2: {type: component, ref: cat}", "both map"},
		{"unknown rule", "p", "https://x", "{secret: remote_credential.x}", "conflict: newest", okEnt, "unknown rule"},
		{"unknown type rule", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, conflict: newest}`, "unknown rule"},
		{"unknown field rule", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, fields: {name: name}, field_conflicts: {name: newest}}`, "unknown rule"},
		{"bad skew", "p", "https://x", "{secret: remote_credential.x}", "max_clock_skew: soon", okEnt, "max_clock_skew"},
		{"zero skew", "p", "https://x", "{secret: remote_credential.x}", "max_clock_skew: 0s", okEnt, "max_clock_skew"},
		{"large skew", "p", "https://x", "{secret: remote_credential.x}", "max_clock_skew: 2h", okEnt, "max_clock_skew"},
		{"field rule unmapped", "p", "https://x", "{secret: remote_credential.x}", "push: true",
			`component: {ref: cat, field_conflicts: {name: hold}}`, "not a mapped field"},
		{"share without push", "p", "https://x", "{secret: remote_credential.x}", "",
			`component: {ref: cat, share: "entity.name == 'x'"}`, "share needs push"},
		{"share invalid", "p", "https://x", "{secret: remote_credential.x}", "push: true",
			`component: {ref: cat, share: "entity.nope =="}`, "share"},
		{"share related", "p", "https://x", "{secret: remote_credential.x}", "push: true",
			`component: {ref: cat, share: "related(entity, 'depends', { name = 'x' })"}`, "related(...) is not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yml := fmt.Sprintf(base, tc.remote, tc.url, tc.cred, tc.extra, tc.ents)
			_, err := Parse([]byte(yml), meta)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParse_PushAndHold(t *testing.T) {
	meta := testMeta(t)
	m := testMapping(t, meta, func(p *Remote) {
		p.Push, p.Conflict = true, ConflictHold
		em := p.Entities["req"]
		em.Share, em.Conflict = "entity.status == 'open'", ConflictHold
		em.FieldConflicts = map[string]string{"title": ConflictHold}
		p.Entities["req"] = em
	})
	if m.Entities["req"].Share == nil || m.Entities["component"].Share != nil {
		t.Fatalf("share: %+v", m.Entities)
	}
}

// The most specific rule applies: field, then type, then remote, then hold.
func TestParse_RulePrecedence(t *testing.T) {
	meta := testMeta(t)
	m := testMapping(t, meta, func(p *Remote) {
		p.Push, p.Conflict, p.MaxClockSkew = true, ConflictRemote, "90s"
		em := p.Entities["req"]
		em.Conflict = ConflictLatest
		em.FieldConflicts = map[string]string{"title": ConflictLocal}
		p.Entities["req"] = em
		rm := p.Relations["satisfies"]
		rm.FieldConflicts = map[string]string{"weight": ConflictHold}
		p.Relations["satisfies"] = rm
	})
	want := map[string]string{"title": ConflictLocal, "status": ConflictLatest}
	for _, f := range m.Entities["req"].Fields {
		if w, ok := want[f.Local.Name]; ok && f.Rule != w {
			t.Errorf("req.%s rule = %q, want %q", f.Local.Name, f.Rule, w)
		}
	}
	if f := m.Relations["satisfies"].Fields[0]; f.Rule != ConflictHold {
		t.Errorf("satisfies.%s rule = %q, want hold", f.Local.Name, f.Rule)
	}
	if got := m.Remote.maxClockSkew(); got != 90*time.Second {
		t.Errorf("max_clock_skew = %s", got)
	}
	plain := testMapping(t, meta, nil)
	for _, f := range plain.Entities["req"].Fields {
		if f.Rule != ConflictHold {
			t.Errorf("default rule of %s = %q, want hold", f.Local.Name, f.Rule)
		}
	}
	if got := plain.Remote.maxClockSkew(); got != DefaultMaxClockSkew {
		t.Errorf("default max_clock_skew = %s", got)
	}
}

func TestParse_RelationAndSystemErrors(t *testing.T) {
	meta := testMeta(t)
	cases := map[string]string{
		"unknown relation": `
remotes:
  p:
    url: https://x
    credential: {secret: remote_credential.x}
    entities: {component: {ref: cat}}
    relations: {nope: {}}`,
		"relation not connecting": `
remotes:
  p:
    url: https://x
    credential: {secret: remote_credential.x}
    entities: {component: {ref: cat}}
    relations: {satisfies: {}}`,
		"relation field": `
remotes:
  p:
    url: https://x
    credential: {secret: remote_credential.x}
    entities: {component: {ref: cat}}
    relations: {depends: {fields: {weight: weight}}}`,
		"shared system": `
remotes:
  p:
    url: https://x
    credential: {secret: remote_credential.x}
    entities: {component: {ref: cat}}
  q:
    url: https://y
    credential: {secret: remote_credential.y}
    entities: {requirement: {ref: cat}}`,
	}
	for name, yml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(yml), meta); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	if _, err := Parse([]byte("remotes: {}"), nil); err == nil {
		t.Fatal("want an error without a schema")
	}
}

// Relation conflict rules apply only to pushed meta.
func TestCompile_RelationRulesNeedPushedMeta(t *testing.T) {
	meta, err := metamodel.Parse([]byte(strings.Replace(testSchema,
		"    to: [component]\n    properties:", "    to: [component]\n    content: true\n    properties:", 1)))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	cfg, err := Parse([]byte(testRemotes), meta)
	if err != nil {
		t.Fatalf("parse remotes: %v", err)
	}
	for _, tc := range []struct {
		name string
		push bool
		edit func(rm *RelationMap)
		want string
	}{
		{"type rule without push", false, func(rm *RelationMap) { rm.Conflict = ConflictLocal }, "need push"},
		{"field rule without push", false,
			func(rm *RelationMap) { rm.FieldConflicts = map[string]string{"weight": ConflictLocal} }, "need push"},
		{"content rule", true, func(rm *RelationMap) {
			rm.Fields = map[string]string{"weight": "weight", "content": "content"}
			rm.FieldConflicts = map[string]string{"content": ConflictLocal}
		}, "only pulled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := cfg.Remotes["cat"]
			p.Push = tc.push
			p.Relations = maps.Clone(p.Relations)
			rm := p.Relations["satisfies"]
			tc.edit(&rm)
			p.Relations["satisfies"] = rm
			if _, err := Compile("cat", p, meta); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
