---
id: RR-4RWHHZ
type: review-response
title: STATUS and TRIGGER bypass escaping; injection test only covers already-safe fields
finding: |-
    ical.go:149 (STATUS) and :171 (TRIGGER) build their lines by raw string concatenation into writeLine, unlike every other field which goes through writeProp. Verified: Status=TodoStatus("NEEDS-ACTION\r\nX-EVIL:1") renders STATUS:NEEDS-ACTIONX-EVIL:1, and the same for Trigger. `;` and `,` also pass through raw, which is genuinely wrong for TRIGGER where `;` is the parameter separator.

    NOT currently a line-forging vulnerability: writeLine calls stripLineBreaks, so no forged property line is reachable (independently probed on UID/STATUS/TRIGGER/alarm-description — all safe). But the value lands unescaped and semantically corrupted, and the mitigation rests entirely on a defence-in-depth layer in a different function — the coupling that breaks when someone later optimises writeLine.

    TestRenderTodo_NoLineBreakInjection (vtodo_test.go:203) only exercises Summary and Description, i.e. the two fields already safe via writeProp. It never touches Status, Trigger, UID or the alarm Description, so it does not actually test what it claims to.

    Severity is significant rather than critical because no forged line is currently reachable — but this becomes the foundation for a CalDAV WRITE path where these values are client-supplied.

    Fix: validate rather than escape (neither is a TEXT value). status() should fall back to NEEDS-ACTION for any value outside the known set; Trigger should be rejected or sanitised unless it is a valid RFC 5545 duration / DATE-TIME. Note the same defect pre-exists on RRULE (ical.go:96), so it is a pattern, not a one-off. Extend the injection test to cover every field that reaches output.
severity: significant
resolution: |-
    Fixed in commit 5a0cac4e by VALIDATING rather than escaping, since neither is a TEXT value — escaping a typed value would corrupt a legal one.

    - status() now switches over the four legal VTODO values (RFC 5545 §3.8.1.11) and falls back to NEEDS-ACTION for anything else. TodoInProcess was added, so the constant set is now complete for VTODO and load-bearing rather than advisory.
    - validTrigger() checks the trigger against an RFC 5545 duration regexp; an unparseable trigger drops its whole VALARM, on the grounds that a silently mis-timed reminder is worse than an absent one. This also closes the `;` concern — a parameter separator can no longer alter the property's meaning.

    Pinned by TestRenderTodo_RejectsInvalidTypedValues, which covers an unknown status, an injected `;PARAM=x`, a non-duration trigger, and asserts all four legal statuses still render.

    DEFERRED, tracked separately: the same raw-concatenation pattern on RRULE (ical.go:96) is pre-existing on the VEVENT path and out of scope for this ticket — it needs its own change so the VEVENT fixtures and callers can be re-verified.
status: addressed
---
