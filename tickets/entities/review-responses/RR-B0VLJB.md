---
id: RR-B0VLJB
type: review-response
title: T4 and T5 disagree on the auto-transition path, author and triggered_by
finding: 'T5 §0/§3 say `auto.after` (T4''s sweep) and `auto.when` both go through a shared `applyTransition`. T4 A3 calls `SetLifecycle` directly, uses author `{system:realtime, Tool: scheduler, PrincipalType: system}` instead of T5''s `{system:realtime, realtime}` and `principal.UserRealtime`, and adds no `Transition.TriggeredBy`. Agree on one `systemAuthor` constant, one `applyTransition` (with the From check reusing T4''s `naturalFrom`), and `triggered_by` for auto.after too, and write it into both plans before either starts.'
severity: significant
resolution: One systemAuthor {system:realtime, realtime, principal_type system} and one applyTransition (From check via naturalFrom) are shared. auto.after also carries triggered_by. Both plans are updated.
status: addressed
---
