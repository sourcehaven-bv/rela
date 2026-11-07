package websocket_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	websocket "github.com/Sourcehaven-BV/rela/third_party/ygows"
)

// TestUpstreamVersion fails when go.mod requires a different ygo version
// than the fork was taken from. A Dependabot bump of ygo then fails CI, so
// it is not auto-merged until someone re-syncs the fork.
func TestUpstreamVersion(t *testing.T) {
	got, err := requiredVersion("../../go.mod", websocket.UpstreamModule)
	if err != nil {
		t.Fatal(err)
	}
	if got != websocket.UpstreamVersion {
		t.Fatalf("go.mod requires %s %s, but third_party/ygows is forked from %s.\n"+
			"Re-sync the fork with upstream provider/websocket %s (see third_party/ygows/README.md),\n"+
			"then set UpstreamVersion in third_party/ygows/upstream.go to %s.",
			websocket.UpstreamModule, got, websocket.UpstreamVersion, got, got)
	}
}

func TestRequiredVersion(t *testing.T) {
	const mod = "github.com/reearth/ygo"
	tests := []struct {
		name    string
		gomod   string
		want    string
		wantErr string
	}{
		{
			name:  "require block",
			gomod: "module x\n\nrequire (\n\ta v1.0.0\n\tgithub.com/reearth/ygo v1.55.0 // indirect\n)\n",
			want:  "v1.55.0",
		},
		{
			name:  "single require",
			gomod: "module x\nrequire github.com/reearth/ygo v1.56.0\n",
			want:  "v1.56.0",
		},
		{
			name:    "missing",
			gomod:   "module x\nrequire a v1.0.0\n",
			wantErr: "does not require",
		},
		{
			name:    "single replace",
			gomod:   "module x\nrequire github.com/reearth/ygo v1.55.0\nreplace github.com/reearth/ygo => ../ygo\n",
			wantErr: "replace directive",
		},
		{
			name:    "replace block",
			gomod:   "module x\nrequire github.com/reearth/ygo v1.55.0\nreplace (\n\tgithub.com/reearth/ygo v1.55.0 => ../ygo\n)\n",
			wantErr: "replace directive",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go.mod")
			if err := os.WriteFile(path, []byte(tc.gomod), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := requiredVersion(path, mod)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// requiredVersion returns the version go.mod requires for module. A replace
// directive for the module is an error, because it would hide which upstream
// code is in the build. A go.work file could hide it too; rela has none.
func requiredVersion(path, module string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var version, block string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "//")
		fields := strings.Fields(line)
		kind := block
		switch {
		case len(fields) == 0:
			continue
		case len(fields) == 2 && fields[1] == "(":
			block = fields[0]
			continue
		case fields[0] == ")":
			block = ""
			continue
		case fields[0] == "require" || fields[0] == "replace":
			kind, fields = fields[0], fields[1:]
		}
		if len(fields) < 2 || fields[0] != module {
			continue
		}
		switch kind {
		case "replace":
			return "", fmt.Errorf("%s has a replace directive for %s; the fork check cannot vouch for it", path, module)
		case "require":
			version = fields[1]
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if version == "" {
		return "", fmt.Errorf("%s does not require %s", path, module)
	}
	return version, nil
}
