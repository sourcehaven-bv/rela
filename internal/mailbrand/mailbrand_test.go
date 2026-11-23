package mailbrand_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/mailbrand"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// files is a ConfigReader and KVReader over a map; an absent name is
// fs.ErrNotExist, and a name in fail returns that error.
type files struct {
	data map[string]string
	fail map[string]error
}

func (f files) Load(_ context.Context, name string) ([]byte, error) { return f.get(name) }
func (f files) Get(_ context.Context, key string) ([]byte, error)   { return f.get(key) }

func (f files) get(name string) ([]byte, error) {
	if err := f.fail[name]; err != nil {
		return nil, err
	}
	v, ok := f.data[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(v), nil
}

const base = "https://app.example.test"

func TestRenderOptions_NoInputsGiveDefaults(t *testing.T) {
	t.Parallel()
	for _, src := range []mailbrand.Source{{}, {Config: files{}, KV: files{}}} {
		opts, err := src.RenderOptions(context.Background(), base)
		require.NoError(t, err)
		assert.Equal(t, mailrender.Options{BaseURL: base}, opts)
	}
}

func TestRenderOptions_ReadsAppAccentLogoAndName(t *testing.T) {
	t.Parallel()
	cfg := files{data: map[string]string{dataentryconfig.ConfigFile: `
app:
  name: Acme Workspace
  logo: /_custom/logo.png
palette:
  accent: "#e11d48"
  dark:
    accent: "#fb7185"
`}}
	opts, err := mailbrand.Source{Config: cfg}.RenderOptions(context.Background(), base)
	require.NoError(t, err)
	assert.Equal(t, base, opts.BaseURL)
	assert.Equal(t, "/_custom/logo.png", opts.LogoURL)
	assert.Equal(t, "Acme Workspace", opts.LogoAlt)
	assert.Equal(t, map[string]string{"--accent-color": "#e11d48", "--dark-accent-color": "#fb7185"}, opts.Palette)

	_, err = mailrender.New(&opts)
	require.NoError(t, err, "the options must be accepted by the renderer")
}

func TestRenderOptions_SettingsPaletteWinsAndDarkOffUsesLight(t *testing.T) {
	t.Parallel()
	cfg := files{data: map[string]string{dataentryconfig.ConfigFile: "palette:\n  accent: \"#e11d48\"\n"}}
	kv := files{data: map[string]string{dataentryconfig.UserPaletteKey: "accent: \"#0d9488\"\ndark: false\n"}}
	opts, err := mailbrand.Source{Config: cfg, KV: kv}.RenderOptions(context.Background(), base)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"--accent-color": "#0d9488", "--dark-accent-color": "#0d9488"}, opts.Palette)
}

func TestRenderOptions_SkipsLogoMailCannotShow(t *testing.T) {
	t.Parallel()
	for _, logo := range []string{"/_custom/logo.svg", "logo.png", "javascript:alert(1)"} {
		cfg := files{data: map[string]string{dataentryconfig.ConfigFile: "app:\n  logo: " + logo + "\n"}}
		opts, err := mailbrand.Source{Config: cfg}.RenderOptions(context.Background(), base)
		require.NoError(t, err)
		assert.Empty(t, opts.LogoURL, logo)
	}
}

// TestRenderOptions_ErrorsKeepWhatWasRead pins the fallback: a read error is
// returned, and the options still carry the parts that could be read.
func TestRenderOptions_ErrorsKeepWhatWasRead(t *testing.T) {
	t.Parallel()
	cfg := files{data: map[string]string{dataentryconfig.ConfigFile: "app:\n  logo: /logo.png\n"}}
	kv := files{fail: map[string]error{dataentryconfig.UserPaletteKey: errors.New("disk on fire")}}
	opts, err := mailbrand.Source{Config: cfg, KV: kv}.RenderOptions(context.Background(), base)
	require.ErrorContains(t, err, "disk on fire")
	assert.Equal(t, "/logo.png", opts.LogoURL)
	assert.Nil(t, opts.Palette)

	bad := files{data: map[string]string{dataentryconfig.ConfigFile: "palette:\n  accent: red;}\n"}}
	opts, err = mailbrand.Source{Config: bad}.RenderOptions(context.Background(), base)
	require.Error(t, err)
	assert.Nil(t, opts.Palette)
	_, err = mailrender.New(&opts)
	require.NoError(t, err)
}
