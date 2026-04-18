package script

import (
	"fmt"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// NewReaderRuntime builds a read-only lua.Runtime wired with AI provider and
// per-script secrets loaded from cacheDir. scriptPath is used to locate
// per-script secrets; pass "" for inline code. The caller owns the returned
// runtime and must call Close.
func NewReaderRuntime(deps lua.ReadDeps, cacheDir, scriptPath string,
	stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error) {
	ctxOpts, err := lua.LoadContextOptions(cacheDir, scriptPath)
	if err != nil {
		return nil, fmt.Errorf("lua context: %w", err)
	}
	all := append([]lua.Option{}, ctxOpts...)
	all = append(all, opts...)
	return lua.NewReader(deps, stdout, all...), nil
}

// NewWriterRuntime builds a read-write lua.Runtime wired with AI provider and
// per-script secrets loaded from cacheDir. scriptPath is used to locate
// per-script secrets; pass "" for inline code. The caller owns the returned
// runtime and must call Close.
func NewWriterRuntime(deps lua.WriteDeps, cacheDir, scriptPath string,
	stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error) {
	ctxOpts, err := lua.LoadContextOptions(cacheDir, scriptPath)
	if err != nil {
		return nil, fmt.Errorf("lua context: %w", err)
	}
	all := append([]lua.Option{}, ctxOpts...)
	all = append(all, opts...)
	return lua.NewWriter(deps, stdout, all...), nil
}
