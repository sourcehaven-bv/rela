package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpanRootsACL(t *testing.T) {
	dir := t.TempDir()
	trace := func(span string) string {
		return `{"e":"c","id":1,"g":1,"pkg":"internal/dataentry","fn":"requestStats.func1","t":1,` +
			`"a":["w=*http.response","r=*http.Request{Span:` + span + `}"]}` + "\n" +
			`{"e":"r","id":1,"t":2}` + "\n"
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("server-1-1.jsonl", trace("a-1"))
	write("server-1-1.json", `{"acl":false}`)
	write("server-1-2.jsonl", trace("a-2"))
	write("server-1-2.json", `{"acl":true}`)
	write("server-1-3.jsonl", trace("a-3")) // no sidecar: ACL unknown

	roots, err := spanRoots(dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		span  string
		noACL bool
	}{
		{"a-1", true},
		{"a-2", false},
		{"a-3", false},
	}
	for _, tc := range tests {
		r, ok := roots[tc.span]
		if !ok {
			t.Errorf("span %s: no root", tc.span)
			continue
		}
		if r.noACL != tc.noACL {
			t.Errorf("span %s: noACL = %v, want %v", tc.span, r.noACL, tc.noACL)
		}
	}
}
