package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TestWithServerAIConfig pins the server-wide ai.yaml: a project without
// one gets it, a project's own file wins, tenant withholding still applies,
// and a bad file fails the build.
func TestWithServerAIConfig(t *testing.T) {
	serverAI := filepath.Join(t.TempDir(), "ai.yaml")
	require.NoError(t, os.WriteFile(serverAI, []byte("base_url: http://127.0.0.1:9/v1\nmodel: server-model\n"), 0o600))

	noAI := func(t *testing.T) string {
		t.Helper()
		root := writeWithheldProject(t)
		require.NoError(t, os.Remove(filepath.Join(root, ".rela", "ai.yaml")))
		return root
	}
	build := func(t *testing.T, root string, opts ...appbuild.Option) *appbuild.Services {
		t.Helper()
		svc, err := appbuild.Discover(root, script.NewEngine(), append(backendtest.Options(t), opts...)...)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		return svc
	}
	aiFile := func(t *testing.T, svc *appbuild.Services) string {
		t.Helper()
		host, _ := appbuild.LuaHost(svc)
		data, err := host.File("ai.yaml")
		if err != nil {
			return err.Error()
		}
		return string(data)
	}

	t.Run("fallback", func(t *testing.T) {
		svc := build(t, noAI(t), appbuild.WithServerAIConfig(serverAI))
		require.Contains(t, aiFile(t, svc), "server-model")
		require.Equal(t, "answered-or-failed", classify(probeNote(t, svc, "ai")))
	})
	t.Run("project wins", func(t *testing.T) {
		svc := build(t, writeWithheldProject(t), appbuild.WithServerAIConfig(serverAI))
		require.NotContains(t, aiFile(t, svc), "server-model")
	})
	t.Run("without it", func(t *testing.T) {
		svc := build(t, noAI(t))
		require.Contains(t, aiFile(t, svc), "no such file")
	})
	t.Run("withheld", func(t *testing.T) {
		svc := build(t, noAI(t), appbuild.WithServerAIConfig(serverAI), appbuild.WithWithheldHostConfig("tenant"))
		require.Contains(t, aiFile(t, svc), "tenant")
	})
	t.Run("invalid", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "ai.yaml")
		require.NoError(t, os.WriteFile(bad, []byte("model: [\n"), 0o600))
		for _, path := range []string{bad, filepath.Join(t.TempDir(), "missing.yaml")} {
			_, err := appbuild.Discover(noAI(t), script.NewEngine(),
				append(backendtest.Options(t), appbuild.WithServerAIConfig(path))...)
			require.ErrorContains(t, err, "server AI config")
		}
	})
}

// classify maps a probe's ai note to whether a provider was configured: a
// configured provider at a closed port fails with a connection error, an
// unconfigured one says so.
func classify(note string) string {
	if note == "" || strings.Contains(note, "not configured") || strings.Contains(note, "not available") {
		return "unconfigured: " + note
	}
	return "answered-or-failed"
}
