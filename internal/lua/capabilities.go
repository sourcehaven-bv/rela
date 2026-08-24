package lua

import (
	"maps"
	"slices"
)

// Capabilities declares which ambient, non-graph capabilities a Lua runtime
// may reach: outbound HTTP, the AI provider, named secrets, and file writes
// under output/.
//
// # Fail-closed
//
// The zero value grants NOTHING (TKT-YH52OM). A runtime built without a grant
// from EITHER source — [ReadDeps.Capabilities] or [WithCapabilities] — has no
// `http` global, no `ai` global, an empty `rela.secrets`, and no
// `rela.write_file`; calling any of them raises "attempt to call a nil value"
// rather than succeeding quietly.
//
// Because the zero value means "nothing", it is also treated as "no opinion":
// an empty [WithCapabilities] does not revoke a grant carried on the deps. See
// that function's godoc for the defect this prevents.
//
// This is the same shape [WriteDeps.ElevatedManager] already uses for
// rela.bypass_acl, and the same rule ReadDeps.VisibleReader states for a nil
// reader (RR-X9NVHI): a forgotten wiring must DENY, never fall back to a
// permissive default. A capability that appears when an operator forgot to
// name it is indistinguishable, at the call site, from one they granted.
//
// Why fail-closed here despite the usual "don't break existing scripts"
// instinct: the capabilities gated here are the exfiltration primitives. A
// script holding `secrets` and `http` can read every secret and POST it
// anywhere in two calls, and before this type existed EVERY runtime held both
// — including read-only ones (validation rules, document renders) that have no
// business with either. Failing open would preserve exactly the property the
// ticket exists to remove.
//
// # Secrets is a list, not a bool
//
// [Secrets] names the individual keys exposed, because a boolean grants the
// whole of .rela/secrets.yaml: an action needing one Slack webhook would also
// receive the database DSN and every API key. Per-script `overrides:` in
// secrets.yaml do NOT already provide this — they substitute a VALUE for a key
// the script would receive anyway and never remove a global key.
//
// # Not a confidentiality boundary for the graph
//
// These are ambient capabilities, not read-ACL. What a script may READ from the
// graph is decided by ReadDeps.VisibleReader; Capabilities decides what it may
// reach OUTSIDE the graph. Do not conflate them — granting `http` does not
// widen what a script can see, and denying it does not narrow it.
type Capabilities struct {
	// HTTP registers the `http` global (get/post/put/patch/delete/request).
	HTTP bool

	// AI registers the `ai` global (chat/complete/embed). Note that ai.* is
	// billable, so this gate is a cost control as well as a security one.
	AI bool

	// WriteFile registers rela.write_file. Writes are already confined to
	// output/ by luaWriteFile, so this is the narrowest of the four; it is
	// gated for consistency and because "may this script touch the disk at
	// all" is a question an operator should be able to answer from config.
	//
	// Only meaningful on a writer runtime — readers never register
	// write_file regardless of this field.
	WriteFile bool

	// Secrets names the keys from .rela/secrets.yaml this runtime may read
	// via rela.secrets. A key not listed is absent from the table, not
	// empty-string — so a typo in a name surfaces as a nil index at the use
	// site rather than as a silently-wrong credential.
	//
	// nil or empty means no secrets at all.
	Secrets []string

	// AllSecrets exposes every key in .rela/secrets.yaml, ignoring [Secrets].
	//
	// This exists ONLY for the operator-shell boundary (see
	// [TrustedCapabilities]), where the caller can already cat the file. It is
	// deliberately a separate field rather than a sentinel value in Secrets
	// (such as "*"): a sentinel would be reachable from YAML by an operator
	// writing `secrets: ["*"]` and would silently mean "everything" at a
	// surface where that is exactly wrong. This field is not settable from
	// config — the YAML decoder never populates it — so the broad grant can
	// only come from a Go wiring site that names it.
	AllSecrets bool
}

// Any reports whether this grant carries anything at all. The zero value
// carries nothing, which is what makes it safe for [WithCapabilities] to treat
// an empty grant as "no opinion" rather than as a revocation.
func (c Capabilities) Any() bool {
	return c.HTTP || c.AI || c.WriteFile || c.AllSecrets || len(c.Secrets) > 0
}

// AllowsSecret reports whether name is exposed to the runtime.
func (c Capabilities) AllowsSecret(name string) bool {
	return c.AllSecrets || slices.Contains(c.Secrets, name)
}

// filterSecrets returns the subset of all whose keys this Capabilities grants.
// Returns an empty (non-nil) map when nothing is granted, so callers can range
// over it unconditionally.
func (c Capabilities) filterSecrets(all map[string]string) map[string]string {
	if c.AllSecrets {
		out := make(map[string]string, len(all))
		maps.Copy(out, all)
		return out
	}
	out := make(map[string]string, len(c.Secrets))
	for _, name := range c.Secrets {
		if v, ok := all[name]; ok {
			out[name] = v
		}
	}
	return out
}

// TrustedCapabilities grants everything. It is for the OPERATOR-SHELL trust
// boundary only — `rela script`, `rela flow`, and the docs build — where the
// caller already has the shell, the project directory, and .rela/secrets.yaml
// itself, so withholding a capability from them protects nothing and only
// breaks working scripts. This is the same boundary `rela db migrate` and
// `rela history-purge` run at.
//
// Do NOT use this for a surface reachable over the network or by an agent.
// In particular it must not be wired into the data-entry app, the scheduler,
// the automation engine, or the MCP lua_eval / lua_run tools: those take input
// from someone other than the person who owns the shell.
func TrustedCapabilities() Capabilities {
	return Capabilities{HTTP: true, AI: true, WriteFile: true, AllSecrets: true}
}
