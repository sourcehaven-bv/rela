---
id: PLAN-XNSDSC
type: planning-checklist
title: 'Planning: Trigger and reminder literals through MessageType.NormalizeProps'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** A trigger's `post.props` and a reminder's `post.props` are literal
strings from `realtime.yaml`. At load they are checked by two functions in
`internal/realtimecfg/validate.go`:

- `validator.literalProps` (validate.go:1000) checks against the *raw*
message type: undeclared property, `{{` in the value, list property, property
kind (only string, markdown, or an enum value in `v.meta.Types[...].Values`),
and missing required properties.
- `validator.literalMessage` (validate.go:953) checks each value's `max`,
the JSON size against `message_bytes`, the `today()` refusal, and the type's
`when:` (binding every field as `predicate.NewString`).

At fire time the same literals go through `realtime.validateProps` →
`MessageType.NormalizeProps` (`internal/realtime/system.go:39` for triggers,
`internal/realtime/reminders.go:200` `reminderProps` for reminders), then
`whenHolds`, which binds fields with `mt.PredicateFields`. So the prop rules
exist twice: once in `NormalizeProps` (`internal/realtimecfg/props.go:51`) and
once in the load-time copy. They can drift. The compose-button preset
(`buttonChecker.preset`, buttons.go:180) already shows the target shape:
literal-specific policy first, then `mt.normalize`, then `when:`.

**Answers to the semantic questions.**

| NormalizeProps rule | Right for a load-time literal? | Decision |
| --- | --- | --- |
| Undeclared property refused | Yes. | Use it. |
| Required property must be set (`requireAll` true) | Yes. A literal is the whole message; no person fills the rest. The fire-time check also uses `requireAll` true. | Use `NormalizeProps`, not `normalize(..., false)`. |
| Defaults | Not applicable: message properties have no defaults (`Property` has no default field). | Nothing to do. |
| Enum values, `max`, UTF-8, date/datetime/ref format | Yes, identical to fire time. | Use it. |
| Strict boolean, whole-number integer | A literal is always a YAML string (`map[string]string`), so `NormalizeProps` would say "must be a boolean" for `b: true`. That message is misleading. | Keep a literal kind rule (below) that refuses integer and boolean first, with a clear reason. |
| List values | `NormalizeProps` would say "must be a list of strings". | Keep the existing literal reason "cannot set a list property; it posts one literal value" (pinned by `load_test.go` "list in a trigger literal" and documented at GUIDE-realtime.md:320). |
| Count limit, `message_bytes` | Yes. | Use it; map `ErrTooManyProps` and `ErrTooLarge` to load reasons. |
| Template placeholders | `NormalizeProps` does not know them. A trigger or reminder never substitutes, so `{{entity.title}}` would be posted verbatim. | Keep the `{{` refusal as literal policy. |
| Values computed at fire time | None exist. Literals are fixed text; the trigger adds only the entity reference outside `props`. | Nothing to do. |

**Behaviour only the old copy has.**

1. `{{` refusal: kept, as literal policy.
2. Kind restriction (string, markdown, enum only): kept, as literal policy,
now read from the compiled `Property.Type` and `Property.List` instead of the
raw type and `v.meta.Types`. Enum *value* membership moves to `NormalizeProps`.
Date, datetime and ref stay refused, so no configuration that is refused today
starts to load.
3. Reporting every problem at once: the old copy reported every bad
property in one pass. That covered undeclared properties, bad values, every
missing required property, and undeclared properties even when `max` or `when:`
also failed. `NormalizeProps` stops at its first refusal of any kind, in sorted
property order, and the required check runs after the per-property checks. So
after this change the operator sees one `NormalizeProps` problem per load: the
first undeclared or bad value, else the first missing required property, else
the size. The literal policy (`{{` and kind) still reports every policy problem
in one pass, and when it reports any, `NormalizeProps` is not run. Accepted:
this is how a button preset and a person's post are checked, and a fix and
reload shows the next problem.
4. `when:` binding: the old copy bound every field as a string. The new
code uses `mt.PredicateFields(np.Props)`, as `whenHolds` does at fire time. For
the kinds a literal may set the result is the same.
5. Copying the literal into `Trigger.Props` / `SessionReminder.props`: the
old `literalProps` did this as a side effect. The callers now copy with
`maps.Copy` before the check, so a refused file still gets the same partial
compiled value as today.

**Scope.**

In scope:

- Replace `validator.literalProps` and `validator.literalMessage` with one
function, `validator.literalPost`, that applies the literal policy, then
`MessageType.NormalizeProps`, then the `today()` rule and the `when:` check.
- An unexported typed per-property refusal (`propError`) from
`normalize`, so load errors can name the property key without repeating the
property name in the reason.
- Update the tests that pin old refusal reasons, add parity tests.
- One paragraph in `docs-project/entities/guides/GUIDE-realtime.md`.

Out of scope:

- Allowing date, datetime or ref literals (a behaviour widening; can be a
follow-up ticket).
- Changing fire-time checks in `internal/realtime` (`validateProps`,
`reminderProps`, `systemWriter.post`). Their text stays byte-identical.
- Changing `buttonChecker.preset` beyond what the shared helper needs.
- Changing the raw YAML shape (`rawTriggerPost.Props` stays
`map[string]string`).

**Acceptance Criteria:**

1. `literalProps` and `literalMessage` no longer exist; no code in
`internal/realtimecfg` other than `MessageType.normalize` and
`Property.Normalize` checks a message property's max, enum values, required
flag, declared name or byte size. Test: `grep` in review.
2. A literal that loads never fails at fire time. For a table of literal
maps over a message type with string, enum, markdown, required and `max`
properties, a `when:`, and a small `message_bytes`, every reminder file and
trigger file that `realtimecfg.Parse` accepts passes the fire-time checks:
`reminderProps` for the reminder, and `validateProps` then `whenHolds` (the
first two steps of `systemWriter.post`) for the trigger. The table also pins
which cases load, so the test cannot pass by refusing everything. Test:
`TestLiteralLoadPassesFireTime` in the new file
`internal/realtime/literal_parity_test.go` (package `realtime`, so it reaches
the unexported fire-time functions; realtimecfg cannot import realtime).
3. A per-property refusal from `NormalizeProps` (undeclared property or
bad value) is reported at `<trigger or reminder key>.post.props.<prop>` with the
bare reason, without the property name, which the key already carries:
   - undeclared `x` on type `nudge`: `message type nudge has no property
of this name`;
   - enum `nope`: `must be one of [...]`;
   - `abcd` over `max: 3`: `must be at most 3 characters`.

A missing required property is reported at `...post.props` (the key the operator
wrote, as today and as `buttonChecker.checkLeftOut` does) with `property "body"
is required`.

Test, reminders (reminders_test.go around line 155): "undeclared prop" becomes
`...r.post.props.x` / `has no property of this name`; "literal on an enum not in
the values" becomes `...r.post.props.level` / `must be one of`; "required prop
not set" becomes `...r.post.props` / `property "body" is required`; "prop over
max" becomes `...r.post.props.short` / `must be at most 3 characters`.

Test, triggers (load_test.go `TestParse_Invalid`): "trigger prop unknown" stays
`...post.props.color` / `no property` (still a substring); "trigger prop enum
value" (line 441) becomes `...post.props.severity` / `must be one of`; "trigger
required prop" (line 445) keeps key `triggers.breach-escalated.post.props` and
its reason becomes `property "summary" is required`; "trigger prop over max"
becomes `must be at most 5 characters`.
4. A size refusal is reported at `...post.props` as
`the message is over message_bytes (N bytes)` and a count refusal as `at most 20
properties`. Test: existing "props over message_bytes" and "trigger prop over
message_bytes" cases keep passing; new reminder case with 21 literals.
5. `{{` in a literal of a declared property, a list property, and an
integer, boolean, ref, date or datetime property are refused at
`...post.props.<prop>` with a literal-policy reason, and `NormalizeProps` is
then not run, so each problem is reported exactly once. The kind reason is
exactly: `a trigger or reminder sets a property to literal text, so it can set
only string, markdown or enum properties; <prop> has type <TypeName>` (for
example `n has type integer`). A list keeps its existing reason. Test: new
reminder cases for integer, boolean, ref and date, asserted with
`assertOnlyProblemAt` (see Test Plan), which fails on a second problem at the
same key; existing "substitution", "trigger prop ref" and "list in a trigger
literal" cases keep passing.
6. The `when:` checks keep their reasons: `calls today()` and
`the message does not meet the when: condition of message type X`, at
`...post.props`. Test: existing `whenCases` in reminders_test.go and "trigger
props fail when" / "trigger message when on today" in load_test.go pass
unchanged.
7. `internal/realtime` behaviour and HTTP refusal text are unchanged.
Test: `go test ./internal/realtime/...` passes without edits.
8. A valid file loads as before: `testdata/valid.yaml`, the docs
examples (`docs_example_test.go`) and the repo's own `realtime.yaml` files load
with the same compiled `Trigger.Props` and reminder props. Test: `go test
./internal/realtimecfg/...`, `go test ./...`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small refactor inside one package)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo deduplication, no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the reference is in this package)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `buttonChecker.preset` (internal/realtimecfg/buttons.go:180 to 240) is
the pattern: preset-specific kind policy (refuses ref and markdown), then
`mt.normalize`, mapping `ErrTooManyProps` and `ErrTooLarge` to load reasons and
any other error to `key`, then the `today()` rule, then `mt.Holds(ctx,
mt.PredicateFields(np.Props), time.Time{})`.
- `realtime.validateProps` (internal/realtime/validate.go:30) and
`realtime.whenHolds` are the fire-time checks the load check must match.
- `realtime.reminderProps` (internal/realtime/reminders.go:187) and
`systemWriter.post` (internal/realtime/system.go:34) are the two fire-time
callers.
- TKT-S9TLJJ introduced `NormalizeProps` for this purpose; this ticket
finishes it for the remaining copy.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/realtimecfg/props.go`: add a typed refusal for one property.

   ```go
   // propError is a refusal by [MessageType.NormalizeProps] of one named
   // property. Error is the fire-time text, property name included;
   // reason is the same refusal without the name, for a load problem
   // whose key already ends in the name.
   type propError struct {
       prop, reason, text string
   }
   func (e *propError) Error() string { return e.text }
   ```

It is unexported: only `literalPost` reads it, and `realtime.validateProps` uses
`Error()` text only.

In `normalize`, return `*propError` for two refusals, with `text` byte-identical
to today:
- undeclared: `text` `message type <mt> has no property "<name>"`,
`reason` `message type <mt> has no property of this name`;
- `p.Normalize` failure: `text` `property "<name>": <err>`, `reason`
`<err>`.

The missing-required refusal stays a plain error (`property "<name>" is
required`), reported at the props key (see AC 3). `ErrTooManyProps` and
`ErrTooLarge` stay as they are. The `NormalizeProps` doc comment is unchanged in
meaning; `normalize`'s comment names `*propError`.

2. `internal/realtimecfg/validate.go`: delete `literalProps` and
`literalMessage`; add

   ```go
   func (v *validator) literalPost(key string, mt *MessageType, lit map[string]string, limits Limits)
   ```

Steps:
- If any literal names a declared property whose compiled `Type` is
empty, return without a problem. An empty `Type` means the raw type was unknown;
`validator.property` already refused the file at `...properties.<p>.type`, and a
kind reason would read "has type " with nothing after it.
- For each literal in sorted order, skip it if the property is not
declared (`NormalizeProps` reports it, so an undeclared `colour: '{{x}}'` is
reported as undeclared, as the old switch did). For a declared property: if the
value contains `{{`, add `key.<prop>`: "a trigger or reminder posts literal
text; {{...}} substitution is not supported". Else, if its kind is not string,
markdown or enum, add `key.<prop>` with `literalKindReason(p)`: the existing
list reason for a list, else the exact AC 5 text with `p.TypeName()`. One
problem per property at most.
- If any policy problem was added, return.
- `np, err := mt.NormalizeProps(in, limits.MessageBytes)` with `in`
the literals as `map[string]any`. Map `ErrTooManyProps` to `key` "at most N
properties", `ErrTooLarge` to `key` "the message is over message_bytes (N
bytes)", `*propError` to `key.<prop>` with its `reason`, and any other error
(the missing-required refusal) to `key` with `err.Error()`.
- Keep the `today()` refusal and the `when:` check, evaluated over
`mt.PredicateFields(np.Props)`.
- Update the doc comment: it checks the message as
`realtime.validateProps` and `whenHolds` will at fire time.

3. Callers:
   - `validator` trigger compile (validate.go:925 to 928): replace the two
calls with `maps.Copy(t.Props, raw.Post.Props)` and
`v.literalPost(key+".post.props", cmt, t.Props, c.limits)` where `cmt :=
compiledMessageType(...)`. Drop the `&mt` raw-type argument; the raw `mt` stays
in use for the ephemeral check above.
   - `reminderCompiler.post` (reminders.go:244 to 248): same change with
`maps.Copy(r.props, raw.Props)` and `c.ct.MessageTypes[raw.Type]`. The `before
:= len(v.problems)` ok check is unchanged.
   - The compiled message type is never nil at these two call sites:
`compile` builds every channel type (and `validator.messageType` always returns
a non-nil type) before triggers, and the reminder compiler runs after the
message-type loop; both callers return earlier when the raw type is undeclared.
Both keep a `!= nil` guard as a defensive check, with a one-line comment stating
this invariant and `// coverage-ignore: unreachable, see comment`. No test is
written for it.

4. Remove imports that become unused in validate.go (`encoding/json`,
`unicode/utf8` if no other use; check with the compiler).

5. Docs: in `docs-project/entities/guides/GUIDE-realtime.md` (Triggers
section, lines 1733 to 1737), replace the load-check sentence with a paragraph
that says: rela checks a trigger's or reminder's `props` when the file loads,
with the same rules the service applies to a post (declared properties, required
properties, enum values, `max`, `message_bytes` and the type's `when`); a
literal is text, so it may set only string, markdown and enum properties;
`{{...}}` is refused; the load reports the first such problem, so fix and reload
to see the next; and the service checks the message again each time it posts,
because the metamodel may have changed since the load. In the reminder table
(line 1435), change "as for a trigger" to "checked as a trigger's are, see
Triggers". Read the summary list (around lines 1859 and 1884) and adjust it if
it no longer matches; today it says a list literal is refused, which stays true.
Do not edit `docs/realtime.md` directly; it is generated (`just docs`).

**Files to modify:**

- `internal/realtimecfg/props.go` (propError, normalize, doc comment)
- `internal/realtimecfg/validate.go` (literalPost replaces literalProps and
literalMessage; trigger caller)
- `internal/realtimecfg/reminders.go` (reminderCompiler.post caller)
- `internal/realtimecfg/reminders_test.go` (updated reasons and keys, new
kind, count, unknown-type and undeclared-substitution cases)
- `internal/realtimecfg/load_test.go` (trigger cases "prop enum value",
"required prop" and "prop over max"; new `assertOnlyProblemAt` helper)
- `internal/realtimecfg/props_test.go` (new; package `realtimecfg`:
`NormalizeProps` refusals return `*propError` with `prop`, `reason` and
unchanged `Error()` text; missing required is not a `*propError`)
- `internal/realtime/literal_parity_test.go` (new; AC 2)
- `docs-project/entities/guides/GUIDE-realtime.md` (one paragraph)
- `docs/realtime.md` (regenerated by `just docs`, not hand-edited)

**Alternatives considered:**

- *Report all value problems at once by calling `Property.Normalize` per
literal in the load check.* Rejected: it re-implements the loop of `normalize`
(undeclared, required, size), which is the second copy this ticket removes.
- *Report every `NormalizeProps` error at `...post.props` with no typed
error, as `preset` does.* Rejected: the key would no longer point at the
property, which today's load errors do. The typed error is small and keeps the
fire-time text unchanged.
- *Export the typed error (`PropError`).* Rejected: no consumer outside
realtimecfg needs it.
- *Report a missing required property at `...post.props.<name>`.*
Rejected: the operator never wrote that key; today's load error and
`buttonChecker.checkLeftOut` both use the props key.
- *Prove parity with a realtimecfg-only test that calls
`NormalizeProps`.* Rejected: `literalPost` calls the same function, so such a
test passes by construction. The parity test lives in `internal/realtime`, where
the fire-time path is reachable.
- *Drop the kind restriction and let `NormalizeProps` decide.* Rejected:
integer and boolean literals would get "must be a whole number" for `n: 3`,
which is wrong-looking, and date, datetime and ref literals would start to load,
which is a behaviour change outside this ticket.
- *Use `normalize(..., false)` like a compose preset.* Rejected: a literal
is the whole message, and fire time uses `requireAll` true.
- *Change `Trigger.Props` to the normalized `map[string]any`.* Rejected:
for the kinds a literal may set, the stored form equals the input, and the
runtime re-normalizes anyway; it would widen the change into
`internal/realtime`.

**Dependencies:** `MessageType.NormalizeProps`, `ErrTooManyProps`,
`ErrTooLarge`, `realtimelimits.MaxMessageProperties`,
`MessageType.PredicateFields`, `MessageType.Holds`, `predicatefns.FuncToday`,
standard `maps` and `errors`. No new packages; no arch-lint change.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `realtime.yaml` `triggers.<name>.post.props` and
`...reminders.<name>.post.props`: operator-authored configuration. Validation is
an allowlist: declared properties only, allowed kinds only (string, markdown,
enum), enum values from the declared list, then the same `NormalizeProps` rules
the service applies to a post. Invalid input refuses the whole file with
`Problem{Key, Reason}`, as today.

**Security-Sensitive Operations:**

- None new. The service still re-checks every system post at fire time
(`systemWriter.post`, `reminderProps`), so a load-check gap cannot store an
invalid message. Refusal reasons name config keys and literal values, which are
configuration, not secrets (CLAUDE.md "The configuration is not a secret").

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | Review: `grep -n "literalProps\|literalMessage" internal/` is empty; the only size and required checks are in props.go. |
| 2 | New `TestLiteralLoadPassesFireTime` (internal/realtime/literal_parity_test.go, package `realtime`). Table columns: name, literal map, `loads bool`. Cases: valid; empty string on a required string; enum in values; undeclared; enum outside values; over max; missing required; over a small `message_bytes`; `when:` false. For each case, build a reminder file and a trigger file with the literal and call `realtimecfg.Parse` with the package's `testMeta`. Assert the load outcome equals `loads` for both. When it loads: for the reminder, call `reminderProps(ctx, cfg, ct, r, now)` and require no error; for the trigger, copy `Trigger.Props` into `map[string]any` as `trigger_rules.go` does, then require `validateProps(mt, props, cfg.Limits().MessageBytes)` to succeed and `whenHolds(ctx, mt, vp.props, now)` to be true. |
| 3 | Update the reminders_test.go and load_test.go cases listed in AC 3 to the new keys and reasons. |
| 4 | Existing "props over message_bytes" (reminders_test.go:175) and "trigger prop over message_bytes" (load_test.go:506) unchanged; new reminder case with 21 literals expecting "at most 20 properties" at `...post.props`. |
| 5 | New reminder cases: integer, boolean, ref and date property literal, each at `...post.props.<prop>` with the exact kind reason. They use a new helper `assertOnlyProblemAt(t, err, key, reason)` in load_test.go, which requires exactly one problem at `key` and that it contains `reason`. Existing "substitution", "trigger prop ref" and "list in a trigger literal" pass unchanged. New case: undeclared `colour: '{{x}}'` is reported once at `...post.props.colour` with `has no property of this name`, not as a substitution. New case: a message property with an unknown type and a literal for it gives the `...properties.<p>.type` problem and no problem at `...post.props.<p>`. |
| 6 | Existing `whenCases` (reminders_test.go:183) and load_test.go "trigger props fail when" / "trigger message when on today" pass unchanged. |
| 7 | `go test -race ./internal/realtime/...` with no edits to existing tests. New `internal/realtimecfg/props_test.go`: an undeclared prop and an over-max value each return a `*propError` (via `errors.As`) with the expected `prop` and `reason`, and `Error()` equal to today's text; a missing required prop is not a `*propError`. |
| 8 | `go test -race ./internal/realtimecfg/...` (includes `testdata/valid.yaml` and `docs_example_test.go`), then `just ci` subset: `go test ./...`, `just arch-lint`, `just comment-lint`, golangci-lint. |

**Edge Cases:**

- Empty `props` with no required property: loads, as today.
- Empty string literal on a required string property: `NormalizeProps`
accepts `""` as set (only nil and empty lists are absent); same as fire time.
Pinned in the parity table.
- Literal set to an enum's value with a `when:` comparing that enum:
`PredicateFields` binds it as a string, the same as before.
- Two problems in one file (an integer literal and an undeclared one): the
policy problem is reported, `NormalizeProps` is skipped; after fixing, the
undeclared one is reported. Two missing required properties: only the first in
sorted order is reported. Both documented as accepted (Understanding, item 3).
- Compiled message type is nil: cannot happen (see Approach step 3); the
guard is defensive only and has no test.
- A message property with an unknown type: `literalPost` returns without
a problem; the `.type` problem refuses the file. Pinned (AC 5).
- A raw `list: true` on an integer property compiles as a plain integer;
the literal gets the integer kind reason next to the `.list` problem. The reason
is true of the compiled property, so it is accepted.
- Non-ASCII literal near `max`: `max` counts code points in both old and
new code.

**Negative Tests:** every case in AC 3, 4 and 5 is a refused file with a pinned
key and reason. A fire-time negative test is not needed: the runtime path is
unchanged and already tested.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Refusal text changes break an operator's script or a doc example.*
Grep found the old texts only in realtimecfg code and tests; the guide does not
quote them. Mitigation: AC 3 updates the tests; the guide paragraph is
rewritten.
- *`propError` changes the realtime HTTP refusal text.* Mitigation:
`Error()` returns the unchanged text; AC 7 runs `internal/realtime` tests
without edits.
- *A file that loaded before is now refused, or the reverse.* The kind
policy is unchanged and `NormalizeProps` applies the same value rules the old
copy did (unknown, required, enum, max, size). Mitigation: the fire-time parity
test (AC 2) and AC 8.
- *Losing the "report all problems" behaviour.* Accepted (see
Understanding, item 3).
- *Commentlint duplication between `literalPost` and `preset` doc
comments.* Mitigation: point to each other rather than repeat the `today()`
reasoning.
- Security risk: none beyond the above; the runtime re-check stays.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-realtime.md`, Triggers section
(lines 1735 to 1739): literals are checked as a post is; a literal sets only
string, markdown and enum properties; `{{...}}` is refused. `docs/realtime.md`
regenerated with `just docs`.
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no command change)
- [x] ~~docs/data-entry.md~~ (N/A: no UI change)
- [x] ~~CLAUDE.md~~ (N/A: no new convention)
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 11 findings (2 significant, 7 minor, 2 nits) were
folded into this plan: a parity test on the fire-time path in
`internal/realtime` (AC 2); the trigger cases whose key or reason change (AC 3);
the missing-required key kept at `...post.props`; skipping the kind rule for an
unknown property type; the declared-name check before `{{`; an unexported
`propError` with a bare reason; an `assertOnlyProblemAt` helper; the nil guard
marked as defensive; the full reporting regression (Understanding, item 3); the
exact kind reason; and a fuller docs paragraph.
