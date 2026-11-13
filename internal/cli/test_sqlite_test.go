//go:build sqlite

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TestTestCmd_RunsWhileDatabaseLocked runs `rela test` while another
// process holds the project's database lock: the test world never opens
// the database. AC14.
func TestTestCmd_RunsWhileDatabaseLocked(t *testing.T) {
	setBackendEnv(t)
	root := newTestCmdProject(t, passingTest)
	// A file under relations/ is markdown data, which opens on the file
	// store; without it the project opens on rela.db and takes its lock.
	require.NoError(t, os.Remove(filepath.Join(root, "relations", ".gitkeep")))
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	_, stderr, code := runCLIProcess(t, nil, "--project", root, "list", "note")
	require.NotEqual(t, 0, code, "the database lock must be held for this test to mean anything: %s", stderr)

	stdout, stderr, code := runCLIProcess(t, nil, "--project", root, "test")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "1 passed, 0 failed")
}
