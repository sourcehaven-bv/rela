---
id: RR-IHWEB0
type: review-response
title: Scheduled tasks get row gating only, so is_redacted() always reports false there (pre-existing RR-7408F5)
finding: 'Found during manual end-to-end verification, not by reading the plan. appbuild.ScheduledLuaWriteDeps calls luaWriteDepsFor(nil) — a nil redactor, which scriptEntityReader substitutes with visibility.NopRedactor. So scheduled Lua tasks get ROW gating but NO field-level visible: redaction (the documented RR-7408F5 limitation). Consequence for this ticket: on the scheduler path entity:is_redacted() always returns false and entity.redacted is always empty, even when acl.yaml declares a visible: block that would redact the same property for the same principal in the UI. Confirmed empirically: a scheduled task running as a principal whose role granted visible: [title] on person still read salary in full with redacted_set=[]. This is pre-existing and NOT a regression — the field was simply never redacted on that path — but it makes the new accessor silently unhelpful on the one runtime most likely to feed an LLM prompt.'
severity: minor
status: open
---

## Finding

Manual verification of the feature was run three ways. Results:

| Runtime | Row gate | Field redaction | `is_redacted` |
|---------|----------|-----------------|---------------|
| CLI (`rela script`) | none | none | always false — correct, documented |
| Scheduler (`run_as: alice`) | YES | **no** | always false — **misleading** |
| Data-entry (documents/views) | YES | YES | works |

The middle row is the problem. A scheduled task is *row*-gated, so an operator
reasonably concludes ACL is in force — but field policy is not applied, and the
new accessor reports "nothing redacted" rather than "redaction was not evaluated
here". That is the same evaluated-vs-unevaluated conflation discussed on
RR-Q2ZRSP, except here it lands on a path that IS partially gated, so it is more
likely to mislead.

Reproduced with a policy granting `visible: [title]` on `person`: the scheduled
task read `salary` in full and reported `redacted_set=[]`.

## Why this is out of scope to fix here

Closing it means wiring an affordance resolver into `appbuild`, which
`ScheduledLuaWriteDeps`' own godoc names as the fix. That is a change to ACL
enforcement scope, not to a Lua accessor, and it would silently change what
existing scheduled jobs can read — exactly the kind of security-behavior change
that deserves its own ticket and review rather than riding along on an
ergonomics change.

## Recommended action

1. Document the runtime table above in `docs/lua-scripting.md` alongside
the existing "ungated runtimes report false" note, so the scheduler case is
explicit rather than inferred.
2. File a follow-up to wire an affordance resolver into appbuild
(closing RR-7408F5), and note that it would make `is_redacted` meaningful on the
scheduler path.

The docs already warn operators about RR-7408F5 in `docs/scheduled-tasks.md`;
this adds the Lua-visible consequence.
