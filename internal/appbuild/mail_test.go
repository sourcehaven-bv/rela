package appbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// writeMailConfig writes .rela/mail.yaml into a temp project and returns the
// project context pointing at it.
func writeMailConfig(t *testing.T, body string) *project.Context {
	t.Helper()

	root := t.TempDir()
	cacheDir := filepath.Join(root, project.CacheDir)
	require.NoError(t, os.MkdirAll(cacheDir, 0o750))
	if body != "" {
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, mail.ConfigFile), []byte(body), 0o600))
	}
	return &project.Context{Root: root, CacheDir: cacheDir}
}

// TestStartMail_NotConfigured covers AC 1 at the wiring layer: no mail.yaml
// means mail is off and nothing else is disturbed.
func TestStartMail_NotConfigured(t *testing.T) {
	t.Parallel()

	ob, stop := startMail(writeMailConfig(t, ""))
	require.Nil(t, ob, "mail must be off when unconfigured")
	require.NotNil(t, stop, "stop must always be callable")
	stop()
	stop() // idempotent
}

// TestStartMail_NilPaths guards the degenerate wiring case rather than panicking
// deep inside a config load.
func TestStartMail_NilPaths(t *testing.T) {
	t.Parallel()

	ob, stop := startMail(nil)
	require.Nil(t, ob)
	require.NotNil(t, stop)
	stop()
}

// TestStartMail_InvalidConfigDoesNotFailBoot pins the degrade-with-a-warning
// contract: a broken mail.yaml disables mail, it does not stop the process.
//
// The distinction from "not configured" matters — both leave mail off, but only
// this one logs, because an operator who wrote a config and got silence would
// have nothing to debug with.
func TestStartMail_InvalidConfigDoesNotFailBoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
	}{
		{"unknown transport", "transport: carrier-pigeon\nfrom: f@e.com\n"},
		{"literal password", "transport: smtp\nhost: h\nfrom: f@e.com\npassword: hunter2\n"},
		{"malformed yaml", "transport: [smtp\n"},
		{"missing from", "transport: smtp\nhost: h\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ob, stop := startMail(writeMailConfig(t, tc.yaml))
			require.Nil(t, ob, "invalid config must leave mail off")
			require.NotNil(t, stop)
			stop()
		})
	}
}

// TestStartMail_Memory pins the local-dev path: transport: memory needs no
// server and yields a working outbox.
func TestStartMail_Memory(t *testing.T) {
	t.Parallel()

	ob, stop := startMail(writeMailConfig(t, "transport: memory\nfrom: rela@example.com\n"))
	require.NotNil(t, ob, "memory transport must produce a working outbox")
	t.Cleanup(stop)

	require.NoError(t, ob.Enqueue(mail.Message{
		To:      []mail.Address{{Email: "to@example.com"}},
		Subject: "hello",
		Text:    []byte("body"),
	}))
	stop()
}

// TestStartMail_SMTPBuildsWithoutDialing pins that wiring does not connect: a
// mail server that is down at boot must not delay or fail startup, since
// delivery is the worker's job and is retried.
func TestStartMail_SMTPBuildsWithoutDialing(t *testing.T) {
	t.Parallel()

	ob, stop := startMail(writeMailConfig(t,
		"transport: smtp\nhost: unreachable.invalid\nport: 1\nfrom: rela@example.com\n"))
	require.NotNil(t, ob, "an unreachable server must still wire cleanly")
	t.Cleanup(stop)
	stop()
}
