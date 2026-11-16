package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// withheldSchema is the probe type. withheldFlows runs one trigger on it
// that reports what a script sees of the host config: a secret, ai.* and
// mail.send.
const withheldSchema = `version: "1.0"
entities:
  probe:
    label: Probe
    plural: probes
    id_prefix: "PRB-"
    id_type: sequential
    properties:
      title: {type: string}
      note: {type: string}
relations: {}
`

const withheldFlows = `triggers:
  probe:
    on: {entity: [probe], created: true}
    outputs:
      - script:
          file: probe.lua
          capabilities: {ai: true, mail: true, secrets: [token]}
`

// withheldScript is the probe. It is a file because only a script file is
// given secrets; inline Lua has no script path to scope them by.
const withheldScript = `local t = entity.properties.title
local out
if t == "secret" then
  local _, v = pcall(function() return rela.secrets.token end)
  out = tostring(v)
elseif t == "ai" then
  local _, err = ai.complete("hi")
  out = err and err.message or "answered"
elseif t == "mail" then
  local _, err = mail.send{to = "ok@example.com", subject = "s", text = "b"}
  out = err and err.kind or "sent"
end
rela.update_entity(entity.id, {note = out})
`

// writeWithheldProject writes a project whose .rela directory holds a
// secret, an ai.yaml pointing at a closed port, and a memory mail transport.
func writeWithheldProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"schema.yaml":        withheldSchema,
		"flows.yaml":         withheldFlows,
		"scripts/probe.lua":  withheldScript,
		".rela/secrets.yaml": "token: operator-token\n",
		".rela/ai.yaml":      "base_url: http://127.0.0.1:9/v1\nmodel: m\n",
		".rela/mail.yaml":    "transport: memory\nfrom: rela@example.com\nrecipients:\n  also_allow: [ok@example.com]\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))
	return root
}

func probeNote(t *testing.T, svc *appbuild.Services, title string) string {
	t.Helper()
	res, err := svc.EntityManager().CreateEntity(asAlice(t, svc),
		&entity.Entity{Type: "probe", Properties: map[string]any{"title": title}}, entity.CreateOptions{})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)
	got, err := svc.Store().GetEntity(context.Background(), res.Entity.Ref())
	require.NoError(t, err)
	note, _ := got.Properties["note"].(string)
	return note
}

// TestWithWithheldHostConfig pins both settings of tenant mode's
// -tenant-shared-host-config: withheld, a script's secret read and ai.*
// fail with the reason while mail.send still uses the host's transport;
// shared, the script reads the operator's secret.
func TestWithWithheldHostConfig(t *testing.T) {
	const reason = "withheld by the test host"

	t.Run("withheld", func(t *testing.T) {
		opts := append(backendtest.Options(t), appbuild.WithWithheldHostConfig(reason))
		svc, err := appbuild.Discover(writeWithheldProject(t), script.NewEngine(), opts...)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })

		secret := probeNote(t, svc, "secret")
		require.Contains(t, secret, reason)
		require.NotContains(t, secret, "operator-token")
		require.Contains(t, probeNote(t, svc, "ai"), "AI is not available: "+reason)
		require.Equal(t, "sent", probeNote(t, svc, "mail"), "mail stays the host's transport")
	})

	t.Run("shared", func(t *testing.T) {
		svc, err := appbuild.Discover(writeWithheldProject(t), script.NewEngine(), backendtest.Options(t)...)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })

		require.Equal(t, "operator-token", probeNote(t, svc, "secret"))
		require.NotContains(t, probeNote(t, svc, "ai"), reason)
		require.Equal(t, "sent", probeNote(t, svc, "mail"))
	})
}

// TestWithWithheldHostConfig_SurvivesReassembly pins that a schema reload
// keeps the host config withheld although its new base is built from the
// config alone, as stdio MCP's reload does.
func TestWithWithheldHostConfig_SurvivesReassembly(t *testing.T) {
	const reason = "withheld by the test host"
	opts := append(backendtest.Options(t), appbuild.WithWithheldHostConfig(reason))
	origin, err := appbuild.Discover(writeWithheldProject(t), script.NewEngine(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = origin.Close() })

	base, err := appbuild.NewSharedBase(origin.Base().Config())
	require.NoError(t, err)
	successor, err := base.ForReassembly(origin).Assemble(
		origin.Store(), origin.Searcher(), origin.VisibleSearcher(), nil)
	require.NoError(t, err)
	t.Cleanup(successor.CloseAssembly)

	secret := probeNote(t, successor, "secret")
	require.Contains(t, secret, reason)
	require.NotContains(t, secret, "operator-token")
}
