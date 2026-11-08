package peersync

import (
	"fmt"
	"strings"
	"testing"
)

func TestParse_Valid(t *testing.T) {
	meta := testMeta(t)
	cfg, err := Parse([]byte(testPeers), meta)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Compile("cat", cfg.Peers["cat"], meta)
	if err != nil {
		t.Fatal(err)
	}
	if m.System != "catalog" || m.Entities["req"].Local != "requirement" || len(m.Entities["req"].Fields) != 3 ||
		m.Relations["depends"].Local != "depends" {

		t.Fatalf("mapping: %+v", m)
	}
	if p := cfg.Peers["cat"]; p.onGone() != OnGoneDetach || p.maxGone() != DefaultMaxGone ||
		p.maxRows() != DefaultMaxRows || p.maxFetch() != DefaultMaxFetch {

		t.Fatalf("defaults: %+v", p)
	}
	other := testMapping(t, meta, func(p *Peer) { p.Entities["req"].Fields["note"] = "note" })
	if other.Hash == m.Hash {
		t.Fatal("a changed mapping must change the hash")
	}
	if _, err := Parse(nil, meta); err != nil {
		t.Fatalf("an empty file is valid: %v", err)
	}
}

func TestParse_Errors(t *testing.T) {
	meta := testMeta(t)
	base := `
peers:
  %s:
    url: %s
    credential: %s
    %s
    entities:
      %s
`
	okEnt := `component: {ref: cat}`
	cases := []struct {
		name, peer, url, cred, extra, ents, want string
	}{
		{"bad name", "Bad", "https://x", "peer_credential.x", "", okEnt, "invalid name"},
		{"bad url", "p", "ftp://x", "peer_credential.x", "", okEnt, "url"},
		{"plain http", "p", "http://example.com", "peer_credential.x", "", okEnt, "allow_insecure_http"},
		{"url credentials", "p", "https://u:p@x", "peer_credential.x", "", okEnt, "credentials"},
		{"credential name", "p", "https://x", "token_key", "", okEnt, "credential"},
		{"on_gone", "p", "https://x", "peer_credential.x", "on_gone: purge", okEnt, "on_gone"},
		{"max_gone", "p", "https://x", "peer_credential.x", "max_gone: -1", okEnt, "max_gone"},
		{"max_rows", "p", "https://x", "peer_credential.x", "max_rows: 99999999", okEnt, "max_rows"},
		{"max_fetch", "p", "https://x", "peer_credential.x", "max_fetch: -2", okEnt, "max_fetch"},
		{"unknown key", "p", "https://x", "peer_credential.x", "flavor: red", okEnt, "flavor"},
		{"no entities", "p", "https://x", "peer_credential.x", "", "", "at least one"},
		{"unknown type", "p", "https://x", "peer_credential.x", "", `x: {type: nope, ref: cat}`, "unknown local type"},
		{"ref not external", "p", "https://x", "peer_credential.x", "", `component: {ref: name}`, "not an external_ref"},
		{"no ref", "p", "https://x", "peer_credential.x", "", `component: {}`, "ref names no property"},
		{"undeclared field", "p", "https://x", "peer_credential.x", "",
			`component: {ref: cat, fields: {nope: name}}`, "not declared"},
		{"ref as field", "p", "https://x", "peer_credential.x", "",
			`component: {ref: cat, fields: {cat: cat}}`, "external ref"},
		{"bad source field", "p", "https://x", "peer_credential.x", "",
			`component: {ref: cat, fields: {name: "a b"}}`, "not a valid name"},
		{"mixed systems", "p", "https://x", "peer_credential.x", "",
			"component: {ref: cat}\n      other: {ref: ref}", "system"},
		{"same local type", "p", "https://x", "peer_credential.x", "",
			"component: {ref: cat}\n      c2: {type: component, ref: cat}", "both map"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yml := fmt.Sprintf(base, tc.peer, tc.url, tc.cred, tc.extra, tc.ents)
			_, err := Parse([]byte(yml), meta)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParse_RelationAndSystemErrors(t *testing.T) {
	meta := testMeta(t)
	cases := map[string]string{
		"unknown relation": `
peers:
  p:
    url: https://x
    credential: peer_credential.x
    entities: {component: {ref: cat}}
    relations: {nope: {}}`,
		"relation not connecting": `
peers:
  p:
    url: https://x
    credential: peer_credential.x
    entities: {component: {ref: cat}}
    relations: {satisfies: {}}`,
		"relation field": `
peers:
  p:
    url: https://x
    credential: peer_credential.x
    entities: {component: {ref: cat}}
    relations: {depends: {fields: {weight: weight}}}`,
		"shared system": `
peers:
  p:
    url: https://x
    credential: peer_credential.x
    entities: {component: {ref: cat}}
  q:
    url: https://y
    credential: peer_credential.y
    entities: {requirement: {ref: cat}}`,
	}
	for name, yml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(yml), meta); err == nil {
				t.Fatal("want an error")
			}
		})
	}
	if _, err := Parse([]byte("peers: {}"), nil); err == nil {
		t.Fatal("want an error without a schema")
	}
}
