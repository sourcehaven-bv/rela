---
id: PLAN-U2LVAO
type: planning-checklist
title: 'Planning: Capability-gate mail.send like http and ai'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:

- `Mail bool` on `lua.Capabilities` and on `metamodel.Capabilities` (the YAML
face), carried through `metamodel.Capabilities.Fields` — the single translation
seam — to every runtime consumer.
- `luaMailSend` returns a typed `denied` error table when the grant is absent.
- The binding stays **registered unconditionally**. The two concerns are
separable and the existing `internal/lua/mail.go` package doc is right about the
first: registration is about *ergonomics* (feature detection, a useful error in
an unconfigured deployment), the gate is about *authorization*. Always present,
refuses to send.
- `Mail: true` hard-wired in `internal/mail/config.go`'s
`ScriptCapabilities.toLua()` — mail's own send-script runtime is the one place
the gate cannot apply, and the code must say why.
- Startup warning naming data-entry actions whose script calls `mail.send`
without the grant.

OUT:

- Backwards compatibility. Explicitly waived by the project owner: the gate
defaults closed and existing projects must add `mail: true`. No transition
period, no env-var escape hatch — either would reproduce the defect for anyone
who did not read the release note.
- Recipient constraints (an allowlist of destination addresses). Separate
ticket; a sibling branch `mail-recipient-allowlist` is doing it.
- Gating the *outbox* or the declarative `mail_templates:` path. Those are
operator-authored config, not script code; the asymmetry this ticket rests on
(config is reviewed, a line buried in a 500-line script is not) does not apply
to them.

**Acceptance Criteria:**

1. **AC1 — denied without the grant.** A runtime built with a wired sender but
`Capabilities{}` runs `mail.send{...}`; the call returns `(nil, err)` with
`err.kind == "denied"` and the sender records **zero** messages. *Test:*
`TestMailSend_DeniedWithoutCapability` in `internal/lua/mail_test.go`.
2. **AC2 — the exfiltration probe from the ticket is closed.** The exact
scenario in the issue — `secrets` granted, `http`/`ai` absent, `mail.send` to an
attacker address — must not deliver. *Test:*
`TestMailSend_SecretsExfiltrationIsDenied`, which asserts the secret *is*
readable in Lua (so the test fails for the right reason: the gate, not a missing
secret) and that nothing reached the sender.
3. **AC3 — sends with the grant.** `Capabilities{Mail: true}` delivers.
*Test:* `TestMailSend_Succeeds` and every other positive test in `mail_test.go`,
via the `newMailRuntime` helper.
4. **AC4 — the binding is still there without the grant.** `type(mail.send)`
is `"function"` with no grant; a script can feature-detect rather than crashing
on a nil index. *Test:* `TestMailSend_DeniedWithoutCapability` asserts this in
the same run.
5. **AC5 — `denied` outranks `not_configured`.** With neither grant nor sender,
the answer is `denied`. Reporting "mail is not configured" to a script that was
never authorized leaks a fact about the deployment and sends the operator to the
wrong file. *Test:* `TestMailSend_DeniedOutranksNotConfigured`.
6. **AC6 — the YAML grant reaches every surface.** `capabilities: {mail: true}`
in `data-entry.yaml`, `schema.yaml` automations and `schedules.yaml` tasks
arrives as `lua.Capabilities.Mail`. *Tests:* `TestLuaCapabilities_CarriesMail`
(dataentry), `TestCapabilitiesRoundTrip*` (scheduler payload), `Fields`
consumers are compile-time-checked.
7. **AC7 — the scheduler payload round-trips it.** A task enqueued with
`mail: true` still has it after the JSON hop, and an unknown/garbage payload
value fails closed. *Test:* extended `internal/scheduler/jobs_test.go` cases.
8. **AC8 — the send-script runtime can still send.** `ScriptCapabilities.toLua()`
returns `Mail: true` regardless of YAML. *Test:*
`TestScriptCapabilities_ToLua_HardWiresMail`.
9. **AC9 — startup warning.** A project with an action script containing
`mail.send` and no `mail:` grant logs one warning naming the action. *Test:*
`TestCollectUngatedMailActions`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — this is a small change to an existing, well-established
mechanism. The prior art is in-repo and complete.

**Existing Solutions:**

No library involved; this is an in-repo authorization mechanism.

- **TKT-YH52OM** built the whole capability system and is the direct precedent.
Its reasoning transfers unchanged: `http` + `secrets` is a two-call exfiltration
path, therefore the outbound primitive is gated. `mail` is an outbound primitive
that was missed.
- `internal/metamodel/capabilities.go:72` — `Capabilities.Fields()` is the
declared single translation seam, and its godoc says explicitly that adding a
capability *should* break the build at every consumer. That is a design
affordance left for exactly this change; using it is the whole plan.
- `internal/lua/runtime.go:806-818` — how `http`/`ai` gate: absence of the
global. Deliberately **not** copied, see Approach.
- `internal/appbuild/appbuild.go:1016` `warnUngatedMembership` — the shape for
a startup security warning: `slog.Warn` with a `fix` key naming the concrete
remedy and a `docs` key.
- `internal/mail/config.go:183` `ScriptCapabilities.toLua` — already hard-wires
`AI: false` with a stated reason. The comment style to follow for `Mail: true`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add `Mail` to both capability types and thread it through `Fields()`. The
compiler then names every consumer that must be updated; there is no searching
involved and no way to miss one.

The gate itself is a check at the top of `luaMailSend`, before the sender
lookup, returning `pushMailError(ls, "denied", ...)`.

*Why a runtime check rather than skipping registration:* the existing package
doc argues at length that the binding must not vanish, and it is right — but it
is arguing about a different question than this ticket asks. Its argument is
that an operator whose `mail.yaml` is missing should read "mail is not
configured", not "attempt to call a nil value". That argument is about the
*quality of the error* and it applies with *more* force to a denied capability,
not less: "attempt to call a nil value (field 'send')" tells an operator
nothing, whereas `denied` can name the exact YAML key to add. The doc is silent
on authorization only because authorization was never considered; the two
concerns compose rather than conflict.

*Why `denied` outranks `not_configured`:* order the checks grant-first. A script
that was never authorized should not learn whether the project has mail
configured — that is a fact about the deployment, and the answer also sends the
operator to the wrong file (`mail.yaml` instead of their `capabilities:` block).

**Files to modify:**

| File | Change |
| --- | --- |
| `internal/lua/capabilities.go` | `Mail bool` field + `Any()` |
| `internal/lua/mail.go` | gate in `luaMailSend`, `errMailDenied`, package doc |
| `internal/lua/runtime.go` | registration-site comment (`:831`) |
| `internal/metamodel/capabilities.go` | `Mail bool` yaml field, `Any()`, `Fields()` |
| `internal/dataentry/capabilities.go` | consumer |
| `internal/script/luascriptrunner.go` | consumer |
| `internal/scheduler/scheduler.go` | consumer |
| `internal/scheduler/jobs.go` | consumer + payload key + round-trip |
| `internal/autocascade/runner.go`, `scriptrunner.go` | consumer + mirror struct |
| `internal/automation/types.go` | `CapabilityFields` signature |
| `internal/mail/config.go` | hard-wire `Mail: true`, document circularity |
| `internal/script/action.go` | `ReadActionScript` (body-returning sibling) |
| `internal/dataentry/mailgate.go` (new) | `collectUngatedMailActions` + warning |
| `internal/dataentry/app.go` | call the collector at boot |
| `docs-project/.../GUIDE-lua-scripting.md`, `GUIDE-mail.md` | rewrite |

**Alternatives considered:**

- *Skip registration when ungated, like http/ai.* Rejected — the package doc's
argument against a vanishing binding is sound and the ticket is explicit that it
must be preserved. A `nil` global is a bad error precisely where a good one is
needed.
- *Grandfather existing projects (warn-and-allow for a release).* Rejected —
explicitly waived by the project owner. A gate that logs and then sends is not a
gate, and the log line is read after the exfiltration, not before.
- *Reuse `Capabilities.HTTP` for mail.* Rejected — different blast radius and
a script needing mail would get the whole network. The per-capability
granularity is the point of the type.
- *Scan for `mail.send` in automation/scheduler scripts too, not just
data-entry actions.* Deferred — see Risks. Data-entry actions are the surface
where the boot-time enumeration already exists and already reads bodies.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `capabilities.mail` | operator YAML (`data-entry.yaml`, `schema.yaml`, `schedules.yaml`) | `metamodel.Capabilities.UnmarshalYAML` already refuses a bare bool for the whole block; the field itself is a plain `bool` so YAML type errors are decode errors | load fails |
| `capability_mail` job payload | the scheduler's own enqueue, round-tripped through JSON | `payload[payloadMail].(bool)` — a failed assert yields `false` | fails **closed** (no grant) |
| `capabilities.mail` in `mail.yaml` | not accepted — `ScriptCapabilities` has no such field | n/a | n/a; `toLua` hard-wires it |

This is an **allowlist**: the zero value grants nothing and a capability is
present only where an operator wrote it down. There is no deny-list and no
wildcard spelling reachable from YAML.

**Security-Sensitive Operations:**

- `mail.send` — the operation being gated. It is an outbound transfer facility
(CONTROL-5-14) and, paired with `rela.secrets`, an exfiltration primitive.
- The gate is checked **before** the sender is consulted and before the opts
table is parsed, so a denied script cannot use argument-parsing side effects or
error text to probe whether mail is configured.
- The denial error text contains no secret, no recipient and no configuration
detail — only the name of the YAML key to add. It is a constant string.
- `ScriptCapabilities.toLua` is the single deliberate exception, and it is
reachable only from `.rela/mail.yaml`, which is operator-authored config at the
same audited tier as `acl.yaml`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the AC table above — each criterion names its test.

Integration (not just unit): AC6/AC7 exercise the config→runtime path
(`luaCapabilities`, the scheduler's JSON payload hop) rather than only the `lua`
package in isolation; AC9 exercises boot-time collection over a real temp-dir
project with a real `actions/*.lua` file on disk.

**Edge Cases:**

| Case | Expected |
| --- | --- |
| No grant, no sender | `denied` (not `not_configured`) — AC5 |
| Grant, no sender | `not_configured` — the pre-existing behaviour is unchanged |
| Grant present, malformed opts | still **raises** — an argument error is a script bug and the gate must not convert it into a soft error |
| Empty `Capabilities{}` on `WithCapabilities` | "no opinion", does not revoke a deps-carried grant (pre-existing rule, must not regress) |
| Scheduler payload missing `capability_mail` | `false` — fails closed |
| Scheduler payload `capability_mail: "true"` (string) | `false` — fails closed |
| Action script mentioning `mail.send` in a comment | warned anyway; a substring scan over-warns rather than under-warns, and a warning is not an error |
| Action with no `script:` (set-only) | not scanned, no warning |

**Negative Tests:**

- `TestMailSend_DeniedWithoutCapability` — the gate denies and records nothing.
- `TestMailSend_SecretsExfiltrationIsDenied` — the issue's own probe.
- `TestMailSend_DeniedOutranksNotConfigured` — ordering.
- `TestCapabilitiesFromPayload_*` — garbage payload values fail closed.
- Mutation testing in both directions (always-deny and always-allow) is
required by the ticket; recorded in the implementation checklist.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
| --- | --- |
| A consumer is missed and one surface silently keeps sending | Route through `metamodel.Capabilities.Fields()`, whose signature change is a compile error at all six consumers. The three non-`Fields` paths (`mail/config.go`, `autocascade/scriptrunner.go`, the scheduler payload) are enumerated in the ticket and each gets a test. |
| A gate that always denies passes the negative test and breaks every real send | Mutation-verify **both** directions and record the table. This is the specific failure the ticket calls out. |
| The circularity: mail's own send script cannot send | `Mail: true` hard-wired in `toLua`, with a comment stating why the gate cannot apply there, plus `TestScriptCapabilities_ToLua_HardWiresMail`. |
| Existing projects break on upgrade with no clue why | The `denied` error names the exact YAML to add, and the startup warning names the affected actions **before** anyone triggers one. Backwards compatibility itself is waived. |
| The startup scan is a substring match and can be fooled (`local m = mail; m.send{}`) | Accepted, and documented in the code. It is a *warning*, not the control — the runtime gate is the control, and it is exact. A best-effort hint that occasionally misses is worth having; one that produces false confidence is not, so the comment says what it does not catch. |
| Merge conflict with the sibling `mail-recipient-allowlist` branch | Both touch `internal/lua/mail.go` and `mail_test.go`. Reported in the PR. The changes are in different functions (gate at the top of `luaMailSend` vs. recipient checking) and should merge with hand resolution. |

**Effort:** s (as ticketed) — the change is mechanical once the seam is used;
the volume is in tests and docs.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-lua-scripting.md` — **contradicts what
is being built.** Line 990 states "unlike `http` and `ai`, whose absence *is*
the capability gate, `mail.send` is not a capability a script holds". Must be
rewritten, not merely amended. The `### Capabilities` heading also needs `mail`
added, which changes its anchor — one inbound link at :1157 must move with it.
- [x] `docs-project/entities/guides/GUIDE-mail.md` — the "Sending mail from any
script" section (:265) says the binding is always present and stops there; it
needs the grant. The troubleshooting entries at :56/:183/:421 concern
`mail.yaml`'s **send-script** `capabilities:` block, which is a *different
thing* from a script-side `mail:` grant — they are not wrong, but the two now
look confusingly alike and must be disambiguated explicitly.
- [x] ~~`docs/metamodel.md`~~ — N/A: no metamodel feature changes; the capability is declared in existing `capabilities:` blocks
- [x] ~~`docs/cli-reference.md`~~ — N/A: no command or flag changes
- [x] `docs/data-entry.md` — regenerated via `just docs` (guides are the source)
- [x] ~~`CLAUDE.md`~~ — N/A: applies the existing capability-gating pattern to a third binding rather than introducing one
- [x] ~~`README.md`~~ — N/A: no project-level change

`docs/` is generated; edits go in `docs-project/` followed by `just docs`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Reviewed against the ticket's own analysis and the `internal/lua/mail.go`
package doc, which is the strongest existing statement of the opposing view.
Three findings, all addressed above:

1. *The package doc's argument must be preserved, not overturned.* It defends
unconditional **registration** and it is correct. The gate is about
**authorization**. Resolved by separating the two in both the code and the doc
rather than deleting the existing reasoning — the rewritten doc keeps the
original argument and adds the axis it never considered.
2. *Check ordering is security-relevant, not arbitrary.* `denied` must precede
`not_configured`, or an unauthorized script probes deployment state and the
operator is pointed at the wrong file. Promoted to AC5 with its own test.
3. *The startup warning must not overstate itself.* A substring scan cannot see
through aliasing. Rather than build something fragile and imply it is complete,
it is documented as a best-effort hint whose backstop is the runtime gate.
