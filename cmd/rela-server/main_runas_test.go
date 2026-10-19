package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// serverMainEnv, when set to 1, makes TestServerMainSubprocess run main.
const serverMainEnv = "RELA_SERVER_TEST_MAIN"

// TestServerMainSubprocess runs rela-server's main for
// TestServer_RefusesRunAsEnv, in a process of its own because main exits.
func TestServerMainSubprocess(t *testing.T) {
	if os.Getenv(serverMainEnv) != "1" {
		t.Skip("run by TestServer_RefusesRunAsEnv")
	}
	os.Args = []string{"rela-server", "-project", t.TempDir()}
	main()
}

// TestServer_RefusesRunAsEnv pins that rela-server exits non-zero at startup
// when RELA_RUN_AS is set, before it opens the project.
func TestServer_RefusesRunAsEnv(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerMainSubprocess$")
	cmd.Env = append(os.Environ(), serverMainEnv+"=1", principal.RunAsEnv+"=system:ci")
	got, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, string(got))
	assert.Equal(t, 1, exit.ExitCode())
	assert.Contains(t, string(got), principal.RunAsEnv)
	assert.Contains(t, string(got), "refusing to start")
}
