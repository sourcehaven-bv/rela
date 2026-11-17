package appbuild

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// serverAIConfig is the server-wide ai.yaml of [WithServerAIConfig]: its
// path, and its contents once [prepare] has read and checked them.
type serverAIConfig struct {
	path string
	data []byte
}

// WithServerAIConfig makes the ai.yaml at path the AI provider of every
// project that has no ai.yaml of its own (TKT-WQ925LL). A project's own
// file wins. The file is read and checked once, when the services are
// built, and a missing or invalid file fails the build.
//
// The file is configuration, not a secret: name the API key with
// api_key_env, or let each project set ai_api_key in its secrets. Under
// [WithWithheldHostConfig] the server-wide file is withheld like the
// project's own.
//
// Empty: ignored.
func WithServerAIConfig(path string) Option {
	return func(o *options) { o.serverAI.path = path }
}

// load reads and checks the file at c.path.
func (c *serverAIConfig) load() error {
	if c.path == "" {
		return nil
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return fmt.Errorf("server AI config: %w", err)
	}
	if _, err := ai.ParseConfig(data, c.path); err != nil {
		return fmt.Errorf("server AI config: %w", err)
	}
	c.data = data
	return nil
}

// serverAIHost serves the server-wide ai.yaml when inner has none, and
// everything else from inner.
type serverAIHost struct {
	inner lua.HostConfig
	data  []byte
}

// File returns inner's file, or the server-wide ai.yaml when inner has no
// ai.yaml.
func (h serverAIHost) File(name string) ([]byte, error) {
	data, err := h.inner.File(name)
	if name == ai.ConfigFile && errors.Is(err, fs.ErrNotExist) {
		return h.data, nil
	}
	return data, err
}

// Secrets returns inner's secrets.
func (h serverAIHost) Secrets(scriptPath string) (map[string]string, error) {
	return h.inner.Secrets(scriptPath)
}

// Path is inner's .rela directory.
func (h serverAIHost) Path() string { return h.inner.Path() }
