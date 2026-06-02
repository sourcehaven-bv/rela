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
func (c *ScriptCmd) Run(ctx context.Context, svc *cliServices) error {
	opts := []lua.Option{
		lua.WithContext(ctx),
		lua.WithCache(svc.LuaCache()),
	}
	if c.OutputDir != "" {
		opts = append(opts, lua.WithOutputDir(c.OutputDir))
	}
	runtime, err := script.NewWriterRuntime(svc.LuaWriteDeps(), c.File,
		os.Stdout, opts...)
	if err != nil {
		return err
	}
	defer runtime.Close()
	//nolint:contextcheck // ctx is threaded through lua.WithContext(ctx) above
	return runtime.RunFile(c.File, c.Args)
}
