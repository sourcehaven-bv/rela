---
id: PLAN-CAP7YH
type: planning-checklist
title: 'Planning: Lua capability gating (http, ai, secrets, write_file)'
status: done
---

<!-- @managed: claude-workflow v1 -->
## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — fail-closed gating of four ambient capabilities (`http`, `ai`,
`rela.secrets`, `rela.write_file`) at Lua runtime construction, plus a
`capabilities:` declaration on every config-declared script surface:
data-entry actions (HTTP + webhook), documents (entity / standalone / list),
automation actions, and scheduler tasks.

OUT — `permission:`-style "who may run this" gating (that was TKT-X06LA2's
original proposal and is a separate, UX-shaped question); a `rela migrate` step;
warning on a `capabilities:` block naming an undefined secret (TKT-E5EM3N class).

**Acceptance Criteria:**

1. A runtime built with no grant exposes none of the four → probe both a reader
   and a writer, assert `http=nil, ai=nil, write_file=nil`, secrets absent.
2. Granting one secret does not expose the others → grant `slack_token`, assert
   `db_dsn` is `nil` from the same file.
3. Grants are independent → `{HTTP:true}` must not turn on `ai`.
4. A config-declared grant reaches the runtime → automation action carrying
   `capabilities:` arrives at the executor verbatim; an action without one
   arrives empty.
5. Operator-shell paths keep working → `scripts/generate-docs.sh` (a CI job)
   still produces byte-identical output.

## Research

- [x] ~~run `/research`~~ (N/A: approach was settled by the precedent below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design question was "which existing pattern applies",
not "which approach exists".

**Existing Solutions:**

The pattern was already in the tree twice, and both were reused rather than
reinvented:

- `rela.bypass_acl` (`internal/lua/runtime.go:713-725`, TKT-D8T148/TKT-Y3JVFK) —
  a binding registered ONLY when a handle is wired, so absence is structural.
  This is the object-capability shape the whole ticket copies.
- `metamodel.ACLBypass` (`internal/metamodel/aclbypass.go`, TKT-Y3JVFK) — a
  privilege field migrated from bool to enum whose `UnmarshalYAML` REFUSES the
  legacy bool. Its godoc supplies the argument for `secrets` being a list: for a
  privilege field, "a parser that maps a legacy value to the BROADEST setting is
  the wrong default", and a compat shim "has no forcing function that ever
  removes it".
- `ReadDeps.VisibleReader` (RR-X9NVHI) — "a nil field DENIES; it never falls
  back to a raw handle" is the same fail-closed rule, applied to reads.
- `autocascade.ScriptAction.AllowACLBypass` — the precedent for carrying a
  config value across a package boundary arch-lint enforces (as a plain string,
  converted once at the consumer). `ScriptCapabilities` copies it exactly.

**Verification of the ticket's own claims** (it predated TKT-Y3JVFK, so it was
re-checked rather than trusted): all still true, and the exposure is WIDER than
written — `secrets`/`http`/`ai` are registered in `registerContextBindings` /
`registerBindings`, which run on READER runtimes too. A validation rule or a
document render could exfiltrate. Only `write_file` and `bypass_acl` were
writer-only. Evidence recorded in the ticket and IMPL.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Gate at **runtime construction**, not per `Execute*` method. The ticket framed
scope as "six entry points"; the actual chokepoint is the 7 construction sites,
which fall into 3 trust tiers (config-declared / operator-shell / agent-facing).
Gating at construction means a new Execute* variant cannot forget to gate,
because it has nothing to gate with unless someone hands it a grant.

- `lua.Capabilities` — zero value denies. `filterSecrets` builds the exposed
  subset by name.
- Carried BOTH on `ReadDeps.Capabilities` (wiring-site default) and via
  `WithCapabilities` (per-execution override); deps seed `r.caps` before options
  are applied, so the option wins. On ReadDeps rather than WriteDeps because the
  read-only surfaces had the same exposure.
- `metamodel.Capabilities` is the YAML face, with no "all" spelling.
- `lua.TrustedCapabilities()` is the operator-shell grant, and `AllSecrets` (a
  Go-only field) is how it says "every secret" without a config-writable
  sentinel.

**Alternatives rejected:**

- *Fail-open with opt-in gating* — my initial recommendation; overruled in favour
  of fail-closed. Fail-open preserves the exact property the ticket exists to
  remove, and leaves the capability present wherever an operator forgot.
- *`secrets: true`* — rejected by the ticket itself; a boolean hands over the
  whole file.
- *`"*"` sentinel in `Secrets` for the trusted grant* — rejected: writable from
  YAML, and would silently mean "everything" on a network-reachable surface.
- *A parameter on each Execute\* method* — rejected: 4 typed document/action
  seams would each grow a parameter, and the deps/option seam already exists.

**Files to modify:** `internal/lua/{capabilities,runtime,deps}.go`,
`internal/metamodel/capabilities.go`, `internal/script/{executor,luascriptrunner}.go`,
`internal/autocascade/{scriptrunner,runner}.go`, `internal/automation/{types,engine}.go`,
`internal/dataentry/{capabilities,actions,webhook,document,handlers_document}.go`,
`internal/dataentryconfig/config.go`, `internal/scheduler/{config,scheduler}.go`,
`internal/cli/{script,flow}.go`, `internal/docs/runtime.go`, `internal/mcp/tools_lua.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `capabilities:` in operator-authored YAML — an **allowlist** by construction:
  the runtime iterates the named keys, so an unnamed key cannot appear. A bare
  bool is refused at parse time with a message naming the mapping form.
- `.rela/secrets.yaml` values — never echoed; an ungranted key is ABSENT from
  `rela.secrets` rather than empty-string, so a typo fails at the use site
  instead of sending an empty credential upstream.

**Security-Sensitive Operations:**

- Secret exposure — narrowed from "whole file, every script" to "named keys,
  declared scripts".
- Outbound HTTP / AI — now absent unless declared; `ai` is also billable, so the
  gate doubles as a cost control.
- The trust-tier split is the load-bearing decision: `TrustedCapabilities()` is
  restricted to surfaces where the caller already owns the shell and could read
  `secrets.yaml` directly. MCP `lua_eval`/`lua_run` deliberately get NOTHING and
  cannot be granted from config — arbitrary client-chosen code is the worst
  possible holder of a credential.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 `ZeroValueDeniesEverything` (reader+writer subtests);
AC2 `SecretsAreNamedNotAllOrNothing`; AC3 `GrantsAreIndependent`;
AC4 `TestLuaScriptRunner_CapabilitiesFlowFromAction` (table: undeclared→empty,
declared→verbatim); AC5 executed `scripts/generate-docs.sh` and diffed.

**Edge Cases:** reader granted `WriteFile` (must still have no binding — gated by
runtime kind AND capability); `"*"` as a secret name (ordinary key, not a
wildcard); a granted name absent from secrets.yaml (silently skipped, no empty
string); `AllSecrets` with an empty file.

**Negative Tests:** `capabilities: true` / `yes` → parse error naming the mapping
form; `AllSecretsIsNotConfigReachable` → `Secrets: ["*"]` must not grant `db_dsn`.

**Integration approach:** the automation flow test asserts across the
config→autocascade→script→lua boundary, and the docs-generation run is a real
end-to-end exercise of the operator-shell tier.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — `l`, as filed

**Risks:**

- **Breaking existing operator scripts** (the accepted cost of fail-closed).
  Mitigated by: the operator-shell exemption (so `rela script` and CI docs keep
  working), a loud failure mode (`attempt to index a nil value (global 'http')`
  rather than a silent nil credential), and docs. NOT mitigated for
  config-declared surfaces — that is the intended behaviour change and is a
  release-note item.
- **A surface silently left ungranted.** Mitigated by enumerating construction
  sites rather than Execute* methods, and by `go test ./...` catching every
  in-tree script that used a capability (61 failures → each triaged, not
  blanket-granted).
- **Someone "fixes" MCP by granting it.** Mitigated by an explicit comment at
  both call sites saying not to.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/lua-scripting.md` (via `docs-project/.../GUIDE-lua-scripting.md`) —
      new "Capabilities" section. DONE.
- [x] `docs/idp-webhook-provisioning.md` — the shipped example would otherwise
      break; its `capabilities:` block is now shown and explained. DONE.
- [x] N/A for `docs/metamodel.md` / `cli-reference.md` — no new command, and the
      `capabilities:` key is documented where scripts are.

## Design Review

- [x] ~~Run `/design-review`~~ (N/A: the design question — default posture and
      surface scope — was put to the user directly and decided by them:
      fail-closed everywhere, all surfaces, CLI exempt. The one place their
      instruction rested on a wrong premise, `ExecuteCode`/`ExecuteFile` being
      "the CLI", was verified against the tree, corrected in the ticket, and
      flagged rather than silently applied.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
