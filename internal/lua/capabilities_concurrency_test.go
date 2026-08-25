package lua_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// TestCapabilitiesConcurrentRuntimes pins the multi-request shape (TKT-YH52OM):
// ONE parsed config value is shared by many concurrently-built runtimes, some
// granted and some not.
//
// Two properties, both of which a single-threaded test cannot show:
//   - no bleed in EITHER direction — an ungranted runtime must not pick up a
//     neighbour's grant, and a granted one must not lose its own;
//   - the config-owned Secrets slice is never mutated. Capabilities is copied
//     by value but Secrets is a slice header, so the backing array IS shared
//     across every runtime built from one config. Both consumers
//     (filterSecrets, AllowsSecret) only read it; this fails if that changes.
//
// Run under -race (CI has it on) so a future mutable capability field is caught
// as a data race rather than as flaky output.
func TestCapabilitiesConcurrentRuntimes(t *testing.T) {
	shared := lua.Capabilities{HTTP: true, Secrets: []string{"slack"}}
	secrets := map[string]string{"slack": "TOK", "db_dsn": "DSN"}

	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var buf bytes.Buffer
			caps := shared
			// Half the goroutines run ungranted, to catch bleed in either direction.
			if i%2 == 0 {
				caps = lua.Capabilities{}
			}
			rt := lua.NewReader(lua.ReadDeps{Capabilities: caps}, &buf,
				lua.WithSecrets(secrets))
			defer rt.Close()
			if err := rt.RunString(
				`rela.output("http=" .. type(http) .. " slack=" .. tostring(rela.secrets.slack) ..
				 " dsn=" .. tostring(rela.secrets.db_dsn))`); err != nil {
				t.Errorf("run: %v", err)
				return
			}
			out := buf.String()
			if i%2 == 0 {
				if !strings.Contains(out, "http=nil") || !strings.Contains(out, "slack=nil") {
					t.Errorf("ungranted runtime leaked a capability: %s", out)
				}
			} else {
				if !strings.Contains(out, "http=table") || !strings.Contains(out, "slack=TOK") {
					t.Errorf("granted runtime missing capability: %s", out)
				}
			}
			// Neither may ever see the ungranted secret.
			if !strings.Contains(out, "dsn=nil") {
				t.Errorf("ungranted secret leaked: %s", out)
			}
		}(i)
	}
	wg.Wait()

	if len(shared.Secrets) != 1 || shared.Secrets[0] != "slack" {
		t.Errorf("shared config slice was mutated: %v", shared.Secrets)
	}
}
