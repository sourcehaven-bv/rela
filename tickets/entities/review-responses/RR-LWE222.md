---
id: RR-LWE222
type: review-response
title: normalizeWorldGrants in Policy.Validate is bypassable via NewDeclarative
finding: |-
    VERIFIED. dn37j2-plan.md §2.1 argues the split 'cannot be bypassed — the same argument Validate already makes for normalizeAssertedRoles'. That argument does not hold here.

    acl.NewDeclarative (declarative.go:83) — the constructor that turns a *Policy into a thing that SERVES READS — does not call Validate (grep confirms zero Validate() calls in declarative.go), and its godoc does not require a validated policy.

    Bypass paths:
    - internal/visibility/visibilitytest/suite.go:150-154 — a NON-test file in the normal build — does `var p acl.Policy; yaml.Unmarshal(...); acl.NewDeclarative(&p, ...)` and exercises real read gating.
    - internal/appbuild/appbuild.go:653-656 (NewFromCollaborators) lifts a policy out of a caller-supplied *acl.Declarative; never validated.
    - policy.go:559-561: LoadPolicyBytes returns &Policy{} on empty input without calling Validate.
    - ~175 test-constructed acl.Policy{...} literals reaching NewDeclarative.

    Failure mode: RoleDef.Worlds stays empty and "world:published" remains inline in Read, where roleGrantsRead (readquery.go:117-123) never matches it. SILENT, and it lands in exactly the state §1.1 identifies as hazardous under filterTypes.

    Same concern applies to the load REFUSAL: a refusal that only fires in Validate is not a refusal for any path that skips Validate.

    FIX: make normalizeWorldGrants idempotent and ALSO call it from NewDeclarative — Validate is the LOAD chokepoint, NewDeclarative is the SERVE chokepoint, and they are different. Consider a `validated bool` set by Validate with NewDeclarative rejecting an unvalidated policy, converting silent bypasses into one loud fix-up. Note ValidateAgainstMetamodel (policy.go:836) already has an unenforced dependency on Validate having run (comment at :875).
severity: critical
status: open
---
