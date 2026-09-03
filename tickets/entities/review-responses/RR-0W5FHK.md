---
id: RR-0W5FHK
type: review-response
title: 'AC 4 over-claims: suppression is configured, non-actionable behaviour and does not warrant Warn'
finding: 'Original finding argued suppression should log at Warn to match the sibling skipBadAddress path, because AC 4 makes the log line the mitigation for the feature''s main risk. That reasoning was inverted: it picked a log level to make an acceptance criterion true rather than from what the event is. Log level tracks actionability, not non-delivery. skipBadAddress warns because a malformed/missing address is a DATA DEFECT the operator must fix; suppression is the feature doing exactly what require_visible_content was set to do - configured, correct, non-actionable. Info is right (Debug defensible). The actual defect is in AC 4, which claims the log makes suppression ''observable to the operator'' and thereby mitigates the silent-non-delivery risk; a below-default-threshold Info line does not carry that weight, and escalating a non-event to Warn to make the claim true would trade correct logging for a satisfied checkbox.'
severity: minor
resolution: 'Keep slog.InfoContext as originally planned. Rewrite AC 4 to stop over-claiming: the log line is a diagnostic available when the operator raises the level, NOT a mitigation that makes suppression visible by default. Correspondingly soften the ''silent non-delivery is harder to diagnose'' risk entry - it is a real, accepted cost of the feature rather than one neutralised by logging. Retain the non-disclosure requirement (template + recipient fields only, never entity titles or property values), which was correct and is the part of AC 4 worth testing.'
status: addressed
---

## Finding

The original form of this finding argued that suppression should log at `Warn`
to match the sibling `skipBadAddress` path
(`internal/appbuild/scheduled_mail.go:79`), on the grounds that AC 4 makes the
log line the mitigation for the feature's primary risk.

That reasoning was inverted. It selected a log level to make an acceptance
criterion come out true, rather than from what the event actually is.

**Log level tracks actionability, not non-delivery.** The two paths only look
alike if "a mail was not delivered" is taken as the salient property:

- `skipBadAddress` warns because a malformed or missing address is a **data
defect**. The operator must fix it, and the mail is failing to arrive for a
reason nobody intended.
- Suppression is the feature doing **exactly what `require_visible_content: true`
was configured to do**. Nothing is broken, nothing is degraded, and there is no
action to take.

Warning about correct, configured behaviour is how logs become noise that
operators filter wholesale — which then buries the warnings that do matter.
`Info` is right; `Debug` would be defensible.

## The real defect

Not the log level — **AC 4**, which reads:

> Suppression is observable to the operator (log line naming template and
> recipient) without disclosing what was filtered.

paired with the risk entry claiming AC 4's log line "gives the operator the
answer." An `Info` line below the default threshold does not carry that weight,
and escalating a non-event to `Warn` to make the claim true would trade correct
logging for a satisfied checkbox.

The honest position: silent non-delivery **is** harder to diagnose, and that is
an accepted cost of the feature, not one neutralised by logging. The log line is
a diagnostic available to an operator who raises the level — useful, but not a
mitigation that operates by default.

## Resolution

- Keep `slog.InfoContext` as originally planned.
- Rewrite AC 4 so it no longer claims default-visibility. It should assert the
diagnostic exists and, critically, that it **does not disclose** what was
filtered.
- Soften the corresponding risk entry: accepted cost, not mitigated.
- Retain the non-disclosure requirement — `template` + `recipient` fields only,
never entity titles or property values. That part was correct and is the part of
AC 4 genuinely worth a test.
