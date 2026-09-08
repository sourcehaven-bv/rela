---
id: RR-SAXKLP
type: review-response
title: AllowAllCopyVisibility with a nil Store passes New and nil-panics at copy time
finding: The opt-out shipped alongside the guard reproduces the failure mode the guard exists to prevent. AllowAllCopyVisibility had an exported Store field, so a bare AllowAllCopyVisibility{} — a plausible copy-paste from the AllowAllCopyReadGate{} literal beside it — satisfies CopyReader, passes requireCopyGates, and nil-panics on the first cross-entity copy, a path that may not run until long after deploy. This is the exact 'never substitute a no-op or sentinel implementation silently, that defers the failure to a downstream symptom' rule from CLAUDE.md that the change itself invokes.
severity: significant
resolution: Made the field unexported and added NewAllowAllCopyVisibility(st) (AllowAllCopyVisibility, error), which rejects a nil store. The broken zero value is now unconstructible outside the package rather than merely checked for. Validated at construction rather than inside requireCopyGates because a nil store is broken under ANY ACL, not only a policy-backed one, as the reviewer noted. The error hint in requireCopyGates now names the constructor. Pinned by TestNewAllowAllCopyVisibility_RejectsNilStore; the 12 test call sites go through a t.Helper wrapper so they exercise the real constructor.
status: addressed
---
