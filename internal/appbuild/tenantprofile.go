package appbuild

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TenantProfile is the part of tenant mode's SaaS profile (TKT-3RWQ1PK)
// that every entry point serving a tenant applies the same way:
// rela-server -tenant-mode, and the CLI, the scheduler and stdio MCP with
// RELA_TENANT_MODE=1. It sets a strict Lua HTTP policy and withholds the
// .rela directory's secrets.yaml and ai.yaml unless the operator shares
// them.
//
// The rest of the profile (commands, git sync, the in-process scheduler)
// belongs to the HTTP server and stays there.
type TenantProfile struct {
	// LuaHTTPAllow is the comma-separated list of CIDRs, IPs and host:port
	// destinations the strict Lua HTTP policy reopens.
	//
	// Empty: accepted — only public addresses are reachable.
	LuaHTTPAllow string

	// SharedHostConfig shares secrets.yaml and ai.yaml with every tenant.
	SharedHostConfig bool

	// WithheldReason is the text a script sees when it reads a withheld
	// secret or calls ai.*. It names the switch that shares them.
	WithheldReason string
}

// LuaHTTPPolicy is the profile's Lua HTTP policy: strict, with the
// LuaHTTPAllow destinations reopened. An entry that does not parse is an
// error, so a mistyped allowlist never starts a process.
func (p TenantProfile) LuaHTTPPolicy() (lua.HTTPPolicy, error) {
	var entries []string
	for e := range strings.SplitSeq(p.LuaHTTPAllow, ",") {
		if e = strings.TrimSpace(e); e != "" {
			entries = append(entries, e)
		}
	}
	prefixes, hosts, err := lua.ParseHTTPAllowList(entries)
	if err != nil {
		return lua.HTTPPolicy{}, err
	}
	return lua.HTTPPolicy{Strict: true, Allow: prefixes, AllowHosts: hosts}, nil
}

// Options are the build options of the profile: [WithWithheldHostConfig]
// unless SharedHostConfig is set.
func (p TenantProfile) Options() []Option {
	if p.SharedHostConfig {
		return nil
	}
	return []Option{WithWithheldHostConfig(p.WithheldReason)}
}

// ApplyLuaPolicy sets the profile's process-wide Lua policy and returns its
// HTTP policy. It installs the HTTP policy with [lua.SetHTTPPolicy], and it
// calls [script.RequireHostConfig], so a script runtime built without a host
// config fails instead of reading the shared .rela directory. It must run
// before any Lua runtime is built, so no script sees the single-tenant
// default. It fails closed: it returns an error unless
// [lua.CurrentHTTPPolicy] reports a strict policy afterwards.
func (p TenantProfile) ApplyLuaPolicy() (lua.HTTPPolicy, error) {
	policy, err := p.applyLuaHTTPPolicy(lua.SetHTTPPolicy, lua.CurrentHTTPPolicy)
	if err != nil {
		return lua.HTTPPolicy{}, err
	}
	script.RequireHostConfig()
	return policy, nil
}

// applyLuaHTTPPolicy is the HTTP half of [TenantProfile.ApplyLuaPolicy] over an
// injected setter and query, so a test does not swap the process-wide
// client.
func (p TenantProfile) applyLuaHTTPPolicy(
	set func(lua.HTTPPolicy) error, current func() (lua.HTTPPolicy, bool),
) (lua.HTTPPolicy, error) {
	policy, err := p.LuaHTTPPolicy()
	if err != nil {
		return lua.HTTPPolicy{}, err
	}
	if err := set(policy); err != nil {
		return lua.HTTPPolicy{}, fmt.Errorf("set the Lua HTTP policy: %w", err)
	}
	if got, ok := current(); !ok || !got.Strict {
		return lua.HTTPPolicy{}, errors.New("tenant mode: the strict Lua HTTP policy is not installed")
	}
	return policy, nil
}
