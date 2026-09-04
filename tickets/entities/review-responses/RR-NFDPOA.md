---
id: RR-NFDPOA
type: review-response
title: worldreader guard_test's signature pin is a no-op; plan cites it as structural
finding: |-
    VERIFIED. dn37j2-plan.md §3.1 and PLAN-BW4X7W's Security Considerations both lean on worldreader/guard_test.go as making it 'structurally impossible' for the grant check to live in worldreader.

    The import-scan half (guard_test.go:41-48, forbidding acl/visibility/principal/affordances/aclmap with a fail-closed exemption list) is REAL and does work.

    The signature half does NOT. guard_test.go:98 is:

        var newResolver = worldreader.NewResolver

    No type annotation — Go infers whatever NewResolver's type currently is. Adding a gate or principal parameter COMPILES CLEANLY and this test still passes. The comment above it (:92-97) claims 'Adding a gate parameter would have to change this call, which is the point' — the comment asserts a guarantee the code does not provide.

    Also: the import ban would not catch a gate passed as a LOCALLY-DECLARED narrow interface (`type gate interface{ PermitsRead(...) bool }` inside worldreader imports nothing forbidden) — which is this codebase's own mandated consumer-side-interface convention.

    So 'the grant check structurally cannot live in worldreader' is an overstatement: the import ban makes it awkward, nothing makes it impossible.

    FIX (cheap, do it in PR-A): spell the type explicitly —
        var newResolver func(worldreader.StateReader, store.WorldScope, worldreader.TypeCanonicalizer) (*worldreader.Resolver, error) = worldreader.NewResolver
    That makes the claim the plan already relies on actually true. Note this is a pre-existing defect in Step 2's deliverable, not introduced here.
severity: significant
status: open
---
