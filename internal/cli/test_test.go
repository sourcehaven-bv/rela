package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCmdSchema = `version: "1.0"
entities:
  note:
    label: Note
    plural: notes
    id_prefix: "N-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
relations: {}
`

// newTestCmdProject writes a project with one test file holding code.
func newTestCmdProject(t *testing.T, code string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"schema.yaml":        testCmdSchema,
		"tests/a.test.lua":   code,
		"relations/.gitkeep": "",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return root
}

const passingTest = `local t = require("rela.test")
t.case("creates", function()
  local e = t.as("anyone"):create("note", {})
  t.expect(e.properties.title).not_to_be_nil()
end)
`

func TestTestCmd_ExitCodes(t *testing.T) { // AC8
	setBackendEnv(t)
	root := newTestCmdProject(t, passingTest)
	stdout, stderr, code := runCLIProcess(t, nil, "--project", root, "test", "--seed", "5")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "ok      tests/a.test.lua: creates")
	assert.Contains(t, stdout, "1 passed, 0 failed (seed 5)")

	failing := newTestCmdProject(t, `local t = require("rela.test")
t.case("fails", function()
  t.expect(1).to_be(2)
end)
`)
	stdout, stderr, code = runCLIProcess(t, nil, "--project", failing, "-o", "json", "test")
	require.Equal(t, 1, code, stderr)
	var rep struct {
		Seed  uint64 `json:"seed"`
		Cases []struct {
			Location string `json:"location"`
			Message  string `json:"message"`
		} `json:"cases"`
		Failed int `json:"failed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &rep), stdout)
	require.Equal(t, 1, rep.Failed)
	assert.Equal(t, "tests/a.test.lua:3", rep.Cases[0].Location)
	assert.NotZero(t, rep.Seed)
}

func TestTestCmd_NoTests(t *testing.T) {
	setBackendEnv(t)
	root := newTestCmdProject(t, passingTest)
	require.NoError(t, os.RemoveAll(filepath.Join(root, "tests")))
	stdout, stderr, code := runCLIProcess(t, nil, "--project", root, "test")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "no test files found")
	_, _, code = runCLIProcess(t, nil, "--project", root, "test", "--require-tests")
	assert.Equal(t, 1, code)
	stdout, stderr, code = runCLIProcess(t, nil, "--project", root, "-o", "json", "test")
	require.Equal(t, 0, code, stderr)
	assert.JSONEq(t, `{"seed": 0, "cases": [], "passed": 0, "failed": 0}`, stdout)
}

// A relative path names a file from the working directory when one is
// there, and from the project root otherwise.
func TestResolveTestPaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tests", "sub"), 0o755))
	t.Chdir(filepath.Join(root, "tests"))
	got, err := resolveTestPaths(root, []string{"sub", "tests/other.test.lua"})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join("tests", "sub"), "tests/other.test.lua"}, got)
}

func TestTestCmd_MissingConfig(t *testing.T) { // AC15
	setBackendEnv(t)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	_, stderr, code := runCLIProcess(t, nil, "--project", root, "test")
	require.Equal(t, 1, code)
	assert.Contains(t, stderr, "schema.yaml")
	assert.Contains(t, stderr, "TKT-DAUS290")
}
