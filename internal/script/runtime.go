package script

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// NewWriterRuntime builds a read-write lua.Runtime wired with AI provider,
// per-script secrets and the mail transport, read from deps.Host or, when
// that is nil, from the project's .rela/ directory (derived from
// deps.ProjectRoot). scriptPath is used to locate
// per-script secrets; pass "" for inline code. The caller owns the returned
// runtime and must call Close.
//
// Option precedence: caller opts are applied first, then context opts (AI
// provider + secrets). Context wins on conflict, so a caller cannot
// accidentally clobber wired AI/secrets by passing a nil-valued override.
func NewWriterRuntime(deps lua.WriteDeps, scriptPath string,
	stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error) {
	// mail.LoadLuaSender is supplied HERE rather than imported by internal/lua,
	// because internal/mail depends on internal/lua (transport: script runs a
	// Lua runtime) and the reverse import would be a cycle. This package
	// already depends on both, so it is the natural place for the one-line
	// adapter — and it keeps LoadContextOptions the single load point rather
	// than adding a parallel mail-loading call site.
	host := deps.Host
	if host == nil {
		host = hostconfig.Dir(cacheDirFor(deps.ProjectRoot))
	}
	ctxOpts, err := lua.LoadContextOptions(host, scriptPath, loadMailSender)
	if err != nil {
		return nil, fmt.Errorf("lua context: %w", err)
	}
	all := append([]lua.Option{}, opts...)
	all = append(all, ctxOpts...)
	return lua.NewWriter(deps, stdout, all...), nil
}

// loadMailSender adapts mail.LoadLuaSender to lua.MailSenderLoader.
func loadMailSender(host lua.HostConfig) (lua.MailSender, error) {
	return mail.LoadLuaSender(host)
}

// cacheDirFor returns the absolute path to the project's .rela directory.
// Returns "" when projectRoot is empty so tests with a zero-value deps are
// unaffected (LoadContextOptions treats an empty cacheDir as "no config").
func cacheDirFor(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	return filepath.Join(projectRoot, project.CacheDir)
}
