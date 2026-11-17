package main

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// mcpServerVersion is the version rela reports in the MCP `initialize` /
// `server/discover` response. It is a distinct var from the CLI's Version
// (which rela-server does not import — it has no CLI command surface) and is
// overridable at build time via
// -ldflags "-X main.mcpServerVersion=$(git describe --tags)".
//
// It is informational: MCP clients log it, they do not gate on it. "dev" in
// an unstamped build is honest rather than a wrong number.
var mcpServerVersion = "dev"

// remoteMCPEnv is the environment variable that turns the remote MCP
// endpoint on ("1") or off ("0").
const remoteMCPEnv = "RELA_MCP"

// toggle is one source's say on a setting: unset, on or off.
type toggle int8

const (
	toggleUnset toggle = iota
	toggleOn
	toggleOff
)

// toggleFlag is a boolean command-line flag that remembers whether it was
// given. `-mcp` and `-mcp=true` vote on, `-mcp=false` votes off, and no flag
// leaves the decision to the other sources.
type toggleFlag struct{ t *toggle }

func (f toggleFlag) String() string {
	if f.t == nil || *f.t == toggleUnset {
		return ""
	}
	return strconv.FormatBool(*f.t == toggleOn)
}

func (f toggleFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*f.t = toggleOf(v)
	return nil
}

// IsBoolFlag lets the flag be given without a value, as `-mcp`.
func (toggleFlag) IsBoolFlag() bool { return true }

func toggleOf(on bool) toggle {
	if on {
		return toggleOn
	}
	return toggleOff
}

// envToggle reads $RELA_MCP. Any value other than "1", "0" or empty is
// refused: before the config setting existed, `RELA_MCP=true` meant off, and
// guessing either way could expose an endpoint or hide one.
func envToggle(v string) (toggle, error) {
	switch v {
	case "":
		return toggleUnset, nil
	case "1":
		return toggleOn, nil
	case "0":
		return toggleOff, nil
	}
	return toggleUnset, fmt.Errorf("$%s=%q: want 1 or 0", remoteMCPEnv, v)
}

// mcpSource is one place an operator can turn the endpoint on or off.
type mcpSource struct {
	name string
	say  toggle
}

// resolveRemoteMCP decides whether the endpoint is served. It is served only
// when no source turns it off and at least one turns it on. An off anywhere
// wins, so an operator who writes `remote_mcp.enabled: false` in
// .rela/config.yaml cannot be overridden by a stray flag or variable in the
// process environment, and the reverse holds too.
//
// It returns the source that turned the endpoint off while another turned
// it on, so the caller can say why a requested endpoint is absent; empty
// when there was no such conflict.
func resolveRemoteMCP(sources ...mcpSource) (on bool, overruledBy string) {
	var off string
	for _, s := range sources {
		switch s.say {
		case toggleOn:
			on = true
		case toggleOff:
			if off == "" {
				off = s.name
			}
		case toggleUnset:
		}
	}
	if off != "" {
		if on {
			return false, off
		}
		return false, ""
	}
	return on, ""
}

// remoteMCPConfig reads `remote_mcp:` from the project's .rela/config.yaml.
// A server without a cache directory has no such file and gets the zero
// block.
func remoteMCPConfig(paths *project.Context) (project.RemoteMCPConfig, error) {
	if paths == nil || paths.CacheDir == "" {
		return project.RemoteMCPConfig{}, nil
	}
	path := paths.ConfigPath()
	cfg, err := project.LoadConfigFromPath(path)
	if err != nil {
		return project.RemoteMCPConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	return cfg.RemoteMCP, nil
}

// remoteMCPPlan is what wireRemoteMCP does: whether to serve the endpoint,
// and the server options it is built with.
type remoteMCPPlan struct {
	serve bool
	opts  []relamcp.Option
}

// planRemoteMCP combines the -mcp flag, $RELA_MCP and `remote_mcp:` in
// paths' .rela/config.yaml through [resolveRemoteMCP], and turns
// `remote_mcp.tools` into a [relamcp.WithToolFilter]. An unreadable config
// file or an unknown $RELA_MCP value is an error, because either could hold
// the operator's "off".
func planRemoteMCP(f *serverFlags, paths *project.Context) (remoteMCPPlan, error) {
	env, err := envToggle(f.remoteMCPEnv)
	if err != nil {
		return remoteMCPPlan{}, err
	}
	cfg, err := remoteMCPConfig(paths)
	if err != nil {
		return remoteMCPPlan{}, err
	}
	var fromConfig toggle
	if cfg.Enabled != nil {
		fromConfig = toggleOf(*cfg.Enabled)
	}
	on, overruledBy := resolveRemoteMCP(
		mcpSource{"remote_mcp.enabled in .rela/config.yaml", fromConfig},
		mcpSource{"$" + remoteMCPEnv, env},
		mcpSource{"-mcp", f.remoteMCP},
	)
	if !on {
		if overruledBy != "" {
			slog.Warn("remote MCP endpoint not served: a source turned it off",
				"source", overruledBy, "path", dataentry.MCPPath)
		}
		return remoteMCPPlan{}, nil
	}
	plan := remoteMCPPlan{serve: true}
	if len(cfg.Tools.Allow) > 0 || len(cfg.Tools.Deny) > 0 {
		plan.opts = append(plan.opts, relamcp.WithToolFilter(cfg.Tools.Allow, cfg.Tools.Deny))
	}
	return plan, nil
}

// wireRemoteMCP enables the HTTP MCP endpoint when [planRemoteMCP] says so.
// The server it mounts is built by [dataentrywire.RemoteMCP], which says how
// identity, reads and tools are wired.
//
// It returns an error rather than exiting so the caller owns the exit, the
// same shape as validateIdentityFlags. A failure here MUST stop startup: the
// operator asked for MCP, and booting without it would serve a server that
// silently lacks the feature they enabled.
func wireRemoteMCP(app *dataentry.App, svc *appbuild.Services, f *serverFlags) error {
	var paths *project.Context
	if svc != nil {
		paths = svc.Paths()
	}
	plan, err := planRemoteMCP(f, paths)
	if err != nil || !plan.serve {
		return err
	}
	return dataentrywire.RemoteMCP(app, svc, mcpServerVersion, plan.opts...)
}
