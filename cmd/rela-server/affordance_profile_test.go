//go:build !postgres && !memorybackend && !sqlite

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// RELA_AFFORDANCE_PROFILE=none|demo fails startup when acl.yaml declares
// field rules (BUG-UO72G2), and still starts without them.
func TestNewServer_AffordanceProfileWithFieldRules(t *testing.T) {
	const fieldRuleACL = cfgACL + `client_baselines:
  apps:
    applies_to: [app]
    redact:
      task: [notes]
`
	cases := []struct {
		name    string
		profile string
		acl     string
		refused bool
	}{
		{"none with ceiling", "none", fieldRuleACL, true},
		{"demo with ceiling", "demo", fieldRuleACL, true},
		{"none without field rules", "none", cfgACL, false},
		{"unset with ceiling", "", fieldRuleACL, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RELA_AFFORDANCE_PROFILE", tc.profile)
			dir := newConfigProject(t)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "acl.yaml"), []byte(tc.acl), 0o644))
			f := &serverFlags{
				projectDir: dir, bind: "127.0.0.1", port: "8080",
				principalHeader: "X-User", jwtHeader: "X-Auth-Assertion",
			}
			svc, err := discoverProject(f)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })
			srv, err := newServer(f, svc, nil)
			if tc.refused {
				require.ErrorIs(t, err, dataentry.ErrProfileOverridesFieldRules)
				require.Contains(t, err.Error(), "RELA_AFFORDANCE_PROFILE="+tc.profile)
				return
			}
			require.NoError(t, err)
			g := srv.cur.Load()
			g.haltScheduler()
			g.app.StopWatching()
		})
	}
}
