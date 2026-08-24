package cli

import (
	"context"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// ScriptCmd executes a Lua script against the graph.
type ScriptCmd struct {
	OutputDir string   `name:"output-dir" help:"Directory for write_file output (default: {project}/output)."`
	File      string   `arg:"" help:"Path to Lua script file."`
	Args      []string `arg:"" optional:"" help:"Arguments passed to the script."`
}

// Run dispatches `rela script <file.lua> [args...]`.
func (c *ScriptCmd) Run(ctx context.Context, svc *writeServices) error {
	opts := []lua.Option{
		lua.WithContext(ctx),
		lua.WithCache(svc.LuaCache),
		// `rela script` is the OPERATOR-SHELL trust boundary (TKT-YH52OM):
		// the caller already has the shell, the project directory and
		// .rela/secrets.yaml, so withholding http/ai/secrets/write_file here
		// protects nothing and would only break working scripts. Every
		// network- or agent-reachable surface gets a narrow grant instead.
		lua.WithCapabilities(lua.TrustedCapabilities()),
	}
	if c.OutputDir != "" {
		opts = append(opts, lua.WithOutputDir(c.OutputDir))
	}
	runtime, err := script.NewWriterRuntime(svc.LuaWriteDeps, c.File,
		os.Stdout, opts...)
	if err != nil {
		return err
	}
	defer runtime.Close()
	//nolint:contextcheck // ctx is threaded through lua.WithContext(ctx) above
	return runtime.RunFile(c.File, c.Args)
}
