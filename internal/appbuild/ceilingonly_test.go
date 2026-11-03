//go:build !postgres && !memorybackend && !sqlite

// Same build constraint as fieldredaction_test.go: the fixture is written as
// markdown files, which only fsstore reads back.

package appbuild_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestCeilingOnlyPolicy_RedactsField pins BUG-KJ5D18 (GitHub #1884): a policy
// whose only field restriction is a client ceiling must still apply it. No role
// declares a field block, so before the fix appbuild chose NopRedactor and the
// app client read the hidden field. The unattenuated client of the same user
// still sees it, which proves the redaction comes from the ceiling and not from
// a role.
func TestCeilingOnlyPolicy_RedactsField(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseline string
	}{
		{"redact", "    redact:\n      person: [salary]\n"},
		{"visible", "    visible:\n      person: [name]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMetamodelBody(t, root, redactionMetamodel)
			writePolicy(t, root, "roles:\n  viewer:\n    read: [\"*\"]\nassignments:\n  bob: viewer\n"+
				"client_baselines:\n  apps:\n    applies_to: [app]\n"+tc.baseline)
			dir := filepath.Join(root, "entities", "people")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			body := "---\nid: PERS-1\ntype: person\nname: Alice\nsalary: \"99000\"\n---\n"
			if err := os.WriteFile(filepath.Join(dir, "PERS-1.md"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}

			svc, err := appbuildOnDisk(t, root)
			if err != nil {
				t.Fatalf("appbuild.New: %v", err)
			}
			defer svc.Close()

			app := boundAs(t, svc, principal.VerifiedFrom("bob", principal.ToolMCP,
				principal.Claims{PrincipalType: "app"}))
			got, err := svc.GatedReads().Reader.Resolve(app, "PERS-1")
			if err != nil || got == nil {
				t.Fatalf("Resolve as app client: %v, %v", got, err)
			}
			if got.GetString("name") != "Alice" {
				t.Errorf("name = %q, want \"Alice\": the ceiling leaves name visible", got.GetString("name"))
			}
			if v := got.GetString("salary"); v != "" {
				t.Errorf("salary = %q for the app client, want it redacted by the client ceiling", v)
			}
			if ids := searchIDs(app, t, svc.GatedReads().Searcher, "99000"); len(ids) != 0 {
				t.Errorf("search on the ceiling-hidden salary = %v, want no hits", ids)
			}

			full, err := svc.GatedReads().Reader.Resolve(bobCtx(t, svc, principal.ToolMCP), "PERS-1")
			if err != nil || full == nil {
				t.Fatalf("Resolve as unattenuated client: %v, %v", full, err)
			}
			if full.GetString("salary") != "99000" {
				t.Errorf("salary = %q for the unattenuated client, want \"99000\"", full.GetString("salary"))
			}
		})
	}
}
