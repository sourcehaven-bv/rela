package script

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// NewWriterRuntime builds a read-write lua.Runtime wired with AI provider,
// per-script secrets and the mail transport, read from deps.Host or, when
// that is nil, from the project's .rela/ directory (derived from
// deps.ProjectRoot). After [RequireHostConfig] a nil deps.Host is an
// error. scriptPath is used to locate per-script secrets; pass "" for
// inline code. The caller owns the returned
// runtime and must call Close.
//
// Option precedence: caller opts are applied first, then context opts (AI
// provider + secrets). Context wins on conflict, so a caller cannot
// accidentally clobber wired AI/secrets by passing a nil-valued override.
// The deps' [lua.Seams] win over both: Seams.Mail replaces the mail.yaml
// transport loader, and Seams.AI replaces the ai.yaml provider, which is
// then not read.
func NewWriterRuntime(deps lua.WriteDeps, scriptPath string,
	stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error) {
	// mail.LoadLuaSender is supplied HERE rather than imported by internal/lua,
	// because internal/mail depends on internal/lua (transport: script runs a
	// Lua runtime) and the reverse import would be a cycle. This package
	// already depends on both, so it is the natural place for the one-line
	// adapter — and it keeps LoadContextOptions the single load point rather
	// than adding a parallel mail-loading call site.
	host, err := hostFor(deps)
	if err != nil {
		return nil, err
	}
	mailLoader := lua.MailSenderLoader(loadMailSender)
	if deps.Seams.Mail != nil {
		mailLoader = deps.Seams.Mail
	}
	if deps.Seams.AI != nil {
		// The injected provider wins, so ai.yaml is not read: a broken or
		// real one must not matter to a world that replaced it.
		host = withoutAIConfig{host}
	}
	ctxOpts, err := lua.LoadContextOptions(host, scriptPath, mailLoader)
	if err != nil {
		return nil, fmt.Errorf("lua context: %w", err)
	}
	all := append([]lua.Option{}, opts...)
	all = append(all, ctxOpts...)
	return lua.NewWriter(deps, stdout, all...), nil
}

// hostConfigRequired is set by [RequireHostConfig].
var hostConfigRequired atomic.Bool

// RequireHostConfig makes [NewWriterRuntime] and [AIProvider] refuse deps
// without a host config for the rest of the process, instead of reading the
// project's .rela directory. Tenant mode calls it at startup, through
// appbuild.TenantProfile.ApplyLuaPolicy, because there that directory is
// the operator's and is shared by every tenant.
func RequireHostConfig() { hostConfigRequired.Store(true) }

// errNoHostConfig is returned in tenant mode for deps without a host config.
var errNoHostConfig = errors.New("lua context: no host config in tenant mode")

// hostFor is the host config deps use. The .rela fallback serves a
// single-project process: the CLI, the desktop and tests, where that
// directory is the project's own. A tenant-mode process shares one .rela
// directory between tenants, so there a missing host config is a wiring bug
// that would hand a tenant the operator's secrets; it is refused instead.
// Every production caller sets Host (appbuild's Lua deps and the data-entry
// App through dataentry.SetLuaHost), so the refusal only catches a new
// surface that forgot it.
func hostFor(deps lua.WriteDeps) (lua.HostConfig, error) {
	if deps.Host != nil {
		return deps.Host, nil
	}
	if hostConfigRequired.Load() {
		return nil, errNoHostConfig
	}
	return hostconfig.Dir(cacheDirFor(deps.ProjectRoot)), nil
}

// AIProvider is the AI provider a writer runtime built from deps would use:
// the injected seam when there is one, else the one ai.yaml configures. It
// is the provider of a declarative llm action (TKT-4USH1HO), which calls the
// model without running Lua, and keeps the same single load point as
// [NewWriterRuntime]. ai.ErrConfigNotFound when neither is set.
func AIProvider(deps lua.WriteDeps) (ai.Provider, error) {
	if deps.Seams.AI != nil {
		return deps.Seams.AI, nil
	}
	host, err := hostFor(deps)
	if err != nil {
		return nil, err
	}
	return lua.AIProvider(host)
}

// withoutAIConfig is a host config that reports ai.yaml as absent.
type withoutAIConfig struct{ lua.HostConfig }

// File reports ai.yaml as absent and serves every other file from the
// wrapped host config.
func (h withoutAIConfig) File(name string) ([]byte, error) {
	if name == ai.ConfigFile {
		return nil, fs.ErrNotExist
	}
	return h.HostConfig.File(name)
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
