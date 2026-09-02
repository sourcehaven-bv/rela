---
id: RR-ZYVERL
type: review-response
title: triggeredByCtx clobbered an enclosing label - scheduler cascades lost schedule:<task>
finding: audit.WithTriggeredBy is an unconditional WithValue so the new per-entry tag overwrote an outer label
severity: critical
resolution: 'Fixed. triggeredByCtx now declines when audit.TriggeredByFrom(ctx) != "" - mirroring recordCascade''s own guard - so an enclosing label survives. Policy chosen and documented in the helper and the audit-log guide: the outermost cause wins; triggered_by is a single string and the enclosing cause is the operator-facing one. Pinned by TestAudit_OuterLabelSurvivesCascade; verified it fails with got "automation:spawn-checklist" when the guard is removed. Confirmed the regression by running the probe against the pre-change tree.'
status: addressed
---

**A regression this ticket introduced, verified by running the tree before and
after the change.**

`audit.WithTriggeredBy` is an unconditional `context.WithValue` — it overwrites.
Before this ticket the cascade paths passed ctx through untouched and
`recordCascade` stamped its generic label only when the ctx label was empty;
that `if` was load-bearing, preserving an enclosing label. The new per-entry tag
overwrote it before `recordCascade` ever saw it.

Measured on the production path (scheduler ctx pre-stamped `schedule:nightly`,
create trips an `on: created` automation):

```
BEFORE                                    AFTER (the regression)
create-entity   requirement  schedule:nightly    schedule:nightly
create-entity   checklist    schedule:nightly    automation:spawn-checklist
create-relation has-checklist schedule:nightly   automation:spawn-checklist
```

"What did last night's task write?" is the most obvious audit query for a
scheduler, and it silently returned fewer rows. Nothing errored; no test failed.

**The negative test I claimed covered this does not.** PLAN-VKUSB7 listed
"pre-wrapped ctx → preserved label" and IMPL-HLR62T credited
`TestAudit_IfExistsReplaceUsesCascadeLabel`. That test asserts a label
`cascadeHost.DeleteEntity` stamps *internally*, downstream of the runner's wrap,
so it never exercises an enclosing label at all. False confidence, which is
worse than no test.

Fixed by making the helper compose rather than clobber — it now declines when
`audit.TriggeredByFrom(ctx) != ""`, mirroring `recordCascade`'s own `if`. Policy
chosen and written down: **the outermost cause wins**, because `triggered_by` is
a single string and the enclosing cause is the operator-facing one. Pinned by
`TestAudit_OuterLabelSurvivesCascade`, verified to fail with `got
"automation:spawn-checklist"` when the guard is removed.

This also fixes the same clobbering on the scripted path (RR-KPTYPY), which had
it since day one.
