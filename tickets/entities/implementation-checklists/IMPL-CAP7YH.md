---
id: IMPL-CAP7YH
type: implementation-checklist
title: 'Implementation: Lua capability gating (http, ai, secrets, write_file)'
status: done
---

<!-- @managed: claude-workflow v1 -->
## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Before writing any code, a probe against a plain `lua.NewReader` confirmed the
ticket's premise on the current tree — a **read-only** runtime (validation rule,
document render) held every capability:

```text
http = table   ai = table   db_dsn = postgres://SUPER-SECRET
write_file = nil   bypass_acl = nil
```

After the change, the same probe with a partial grant
(`{HTTP: true, Secrets: ["slack"]}`):

```text
http=table  ai=nil  slack=SLACK-TOK  db_dsn=nil
```

`db_dsn` is withheld while `slack` is granted from the *same* secrets file —
the named-list behaviour, which a boolean could not express.

**Mutation testing.** Each gate was broken to confirm the tests fail:

| Mutation | Result |
|---|---|
| `http`/`ai` back to always-registered | `ZeroValueDeniesEverything`, `GrantsAreIndependent` FAIL |
| secrets filter bypassed (`range r.secrets`) | `ZeroValueDeniesEverything`, `SecretsAreNamedNotAllOrNothing` FAIL |
| automation grant dropped (`caps := lua.Capabilities{}`) | `CapabilitiesFlowFromAction/declared_grant...` FAIL |

**CI regression check.** `scripts/generate-docs.sh` (run by the `docs` CI job)
uses `rela.write_file` and would have broken under fail-closed. Ran it against
the built binary: completes, and `git status docs/ README.md` is clean — output
byte-identical. This is why `rela script` / `rela flow` / the docs runtime get
`TrustedCapabilities()`.

**Full suite**: `go test ./...` → 0 failures. `just arch-lint` → OK (the new
`scheduler` → `metamodel` import is permitted).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the three config→runtime translations
      (data-entry, automation, scheduler) are deliberately NOT collapsed into
      one helper: they cross package boundaries that arch-lint enforces
      (`autocascade` may not import `metamodel` or `lua`), which is the same
      reason `AllowACLBypass` is already carried as a plain string there.
      Within `dataentry` the repeated conversion IS extracted (`luaCapabilities`).
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Deviations from the plan

- **`ExecuteCode`/`ExecuteFile` are NOT the CLI.** The ticket described them as
  a CLI surface and the plan was to exempt them. They have three callers, none
  of which is the CLI: the **automation engine** (`luascriptrunner.go:184,186`)
  and the **scheduler**. Exempting them would have left the network-triggered
  automation path fully ungated — the exact hole the ticket exists to close.
  The exemption was applied to the genuine operator-shell entry points instead
  (`cli/script.go`, `cli/flow.go`, `docs/runtime.go`). Recorded in the ticket.

- **`AllSecrets` added** as a Go-only field. `TrustedCapabilities()` needs
  "every secret", but a `"*"` sentinel inside `Secrets` would be writable from
  YAML and would silently mean "everything" on a network-reachable surface. A
  separate field the YAML decoder never populates keeps the broad grant
  unreachable from config; pinned by `AllSecretsIsNotConfigReachable`.

- **`capabilities: true` is refused at parse time**, mirroring what TKT-Y3JVFK
  established for `allow_acl_bypass`: for a privilege field, mapping a loose
  value to the broadest setting is the wrong default.

## Not done (follow-up)

- No `rela migrate` step and no validation warning for a `capabilities:` block
  naming a secret absent from `.rela/secrets.yaml` — that is the TKT-E5EM3N
  class of "config names something no one grants" check.
- Operators with existing scripts that use `http`/`ai`/`secrets` on a
  config-declared surface must add a `capabilities:` block. This is the
  intended breaking change of a fail-closed default, and is documented, but it
  is a release-note item.
