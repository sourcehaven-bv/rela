---
id: RR-LP9XAU
type: review-response
title: Gate role-bounds/sanitization not exercised end-to-end through the gate
finding: The 32-role count cap and 256-rune/control-char sanitization live in jwtauth.stringSliceClaim and the shared verifiedPrincipal helper, and are unit-tested in internal/jwtauth. But no test drove an over-cap or control-char roles array THROUGH the gate specifically, so the systemic guard stopped short of proving sanitization holds on the production gate path. Defense-in-depth gap, not a hole — the caps do apply in production because VerifyAssertion runs upstream of the gate and verifiedPrincipal re-sanitizes per element.
severity: minor
resolution: 'Added TestRequireVerifiedJWT_SanitizesRolesThroughGate: drives roles ["ad\x00min", "\x00\x00", "ok"] through the real requireVerifiedJWT and asserts the stamped Principal.Roles() carry no control chars, no empty entries, and the control-only role is dropped (2 survivors). Fault-injected: replacing verifiedPrincipal''s role-sanitize loop with a raw passthrough fails the test with ''role "ad\x00min" reached the Principal with a control char'' and ''want 2 surviving entries''. The count cap stays covered by internal/jwtauth (upstream of the stub used here); this closes the dataentry-side per-element sanitization on the gate path.'
status: addressed
---

## Finding

The per-element role sanitization (control-char strip, empty-drop) and the
32-role / 256-rune caps are real on the production gate path — `VerifyAssertion`
runs upstream, `verifiedPrincipal` re-sanitizes per element — but no **gate
test** drove a hostile roles array through to prove it. The systemic guard
covered subject-and-roles-reach-ACL but not sanitization-on-the-gate-path.

## Resolution

`TestRequireVerifiedJWT_SanitizesRolesThroughGate` drives `["ad\x00min",
"\x00\x00", "ok"]` through the real `requireVerifiedJWT` and asserts the stamped
`Roles()` are clean, the control-only role is dropped, and exactly two survive.

Fault-injected: replacing `verifiedPrincipal`'s sanitize loop with `roles :=
id.Roles` fails with *"role \"ad\x00min\" reached the Principal with a control
char"* and *"want 2 surviving entries"*. The count cap stays covered by
`internal/jwtauth` (it runs before the stub used here).
