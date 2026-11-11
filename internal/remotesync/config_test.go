package remotesync

import (
	"fmt"
	"strings"
	"testing"

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
		{"rule not yet", "p", "https://x", "{secret: remote_credential.x}", "push: true\n    conflict: latest", okEnt,
			`rule "latest" is not supported yet; use hold`},
		{"unknown rule", "p", "https://x", "{secret: remote_credential.x}", "conflict: newest", okEnt, "unknown rule"},
		{"type rule", "p", "https://x", "{secret: remote_credential.x}", "push: true", `component: {ref: cat, conflict: remote}`,
			"not supported yet"},
		{"field rule", "p", "https://x", "{secret: remote_credential.x}", "push: true",
			`component: {ref: cat, fields: {name: name}, field_conflicts: {name: local}}`, "not supported yet"},
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
