---
id: RR-0A3JYK
type: review-response
title: 'WITHDRAWN: claimed ACL-failure/all-hidden ambiguity does not exist in the code'
finding: 'FieldRedactor''s contract (visibility.go:63-72, RR-FJUQSF) requires an implementation that cannot compute verdicts to return the hide-everything set — every property name of e — rather than nil. So a total ACL evaluation failure and a policy that genuinely hides every property produce byte-identical output. The plan''s edge-case list treats ''ALL properties hidden'' and ''fail-closed redactor error path'' as two separate cases, but at the Redact seam they are the same case and cannot be distinguished. A script rendering ''[redacted]'' for every field cannot tell the reader is unauthorized from the ACL subsystem being broken, which is an operability problem: the failure is silent and looks like normal policy.'
severity: significant
resolution: 'Withdrawn as factually wrong. I read FieldRedactor''s fail-closed godoc (an implementer contract) as if it described an active runtime path, then constructed an ''ACL subsystem down'' incident scenario on top of it. Verification: PolicyResolver.FieldVerdicts (resolver.go:370) returns a VALUE with no error channel; a nil policy yields empty verdicts (hide nothing); malformed policy fails at predicate.Compile during CONSTRUCTION, not per-read; PolicyRedactor.HiddenProperties (adapters.go:104) merely maps Visible[name]==false into a set and has no failure path. So there is no runtime state in which a redactor failure masquerades as an all-fields-hidden policy, and nothing for a script to be confused by. The single genuine all-hidden case is the historical closed-world at resolver.go:394 (TKT-73C6B2), which is a deliberate strictness for history snapshots and does not reach the live Lua read path. No plan change, no follow-up ticket, no documentation note required.'
reason: 'Withdrawn as factually incorrect, not deferred. I read FieldRedactor''s fail-closed godoc (an implementer CONTRACT for future redactors that can fail) as if it described a live runtime path, then built an ''ACL subsystem down'' incident scenario on top of it. Verification of the actual code: PolicyResolver.FieldVerdicts (resolver.go:370) returns a VALUE with no error channel; a nil policy yields empty verdicts meaning hide-nothing; malformed policy fails at predicate.Compile during CONSTRUCTION rather than per-read; and PolicyRedactor.HiddenProperties (adapters.go:104) merely maps Visible[name]==false into a set with no failure path. There is therefore no runtime state in which a redactor failure is indistinguishable from an all-fields-hidden policy, so there is nothing for a script to misread and no operability gap to document. The only genuine all-hidden case is the historical closed-world at resolver.go:394 (TKT-73C6B2), a deliberate strictness for history snapshots that never reaches the live Lua read path. No code change, no doc note, and no follow-up ticket are warranted.'
status: wont-fix
---

## Finding

`FieldRedactor`'s godoc (`internal/visibility/visibility.go:63-72`) is explicit:

> FAIL-CLOSED CONTRACT (RR-FJUQSF): an implementation that cannot
> compute verdicts must return the hide-everything set (every property
> name of e), never nil — nil means "nothing hidden" and would fail
> open.

The plan's edge-case table lists these as two rows:

- "ALL properties hidden → fail-closed redactor returns every name"
- "Fail-closed redactor error path: must mark redacted, never silently
reveal"

At the `Redact` seam these are **the same observation**. `Redact` receives only
a `map[string]struct{}` of names; it has no error channel and no way to learn
whether that set means "policy says hide all" or "policy evaluation failed".
Writing them as separate test cases implies a distinction the design cannot
deliver.

## Why this matters beyond test bookkeeping

The ticket's motivation is operability: let a script show `[redacted]` instead
of a misleading blank. But under this design, a script that renders `[redacted]`
on every field cannot tell:

- the reader is legitimately unauthorized for everything, from
- the ACL subsystem failed and fail-closed swallowed it.

The second is an incident. The first is Tuesday. Making redaction visible to
scripts while leaving these fused means the new signal is actively misleading in
exactly the situation an operator most needs it.

Note this is a **pre-existing** property of the fail-closed contract, not one
this ticket introduces — today nobody can see either case from Lua, so nothing
is lost. But the ticket is the thing that makes the fused signal *observable*,
so it is the right moment to decide whether that fusion is acceptable.

## Options

1. **Accept and document.** Add an explicit note to the Lua binding docs
that an all-fields-redacted entity may indicate policy failure, and that
operators should check logs. Cheapest; keeps the seam simple. The fail-closed
path already logs (`slog.Warn` in `PolicyReader`), so the diagnostic exists — it
is just not reachable from the script.
2. **Widen `FieldRedactor` to distinguish the cases** (e.g. a second
return, or a sentinel reason). This is a change to a security- critical contract
with a documented rationale and a review-response ID behind it; doing it as a
side effect of a Lua ergonomics ticket is the wrong forcing function.
3. **Defer.** File a follow-up for redactor-failure observability and
ship this ticket with option 1's documentation.

Recommendation: option 1 now, with option 3's follow-up filed. Do NOT widen the
`FieldRedactor` contract inside this ticket — RR-FJUQSF exists because that
contract was already gotten wrong once.

## Required plan changes

- Merge the two edge-case rows into one, stating plainly that they are
indistinguishable by construction.
- Add the documentation note to the Documentation Impact section.
