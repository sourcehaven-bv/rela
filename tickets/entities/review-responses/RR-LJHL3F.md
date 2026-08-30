---
id: RR-LJHL3F
type: review-response
title: Reviewer's claim that plimsoll is inert on App is incorrect — gate verified firing
finding: 'The review closed with: ''just plimsoll is inert on this receiver — I set max-methods=1 and it still exited 0, so the god-object gate is not actually enforcing the App directive.'' If true this would invalidate every count in the TKT-N0IKN9 arc, since the ratchet would be unenforced and the numbers would rest purely on hand-verification.'
severity: significant
reason: 'NOT REPRODUCIBLE — the claim is wrong. Tested twice: in the main checkout (directive 104 → 1) plimsoll failed with ''type App has 104 methods, over the load line of 1'', exit status 3; and independently in the PR worktree (directive 86 → 1), also exit status 3, recipe failed. The gate enforces the App directive correctly in both. Most likely cause is the very worktree hazard the reviewer flagged one paragraph earlier: `cd` resets between Bash calls, so the edit and the linter run can land in different checkouts. No action needed; recorded so the false ''gate is inert'' belief is not carried into later arc steps.'
status: wont-fix
---

Claim from the TKT-SJ0LRS code review (cranky-code-reviewer, PR #1470),
investigated and refuted by the coordinator.

The rest of that review was excellent and its central finding was verified
empirically by the reviewer rather than argued: they implemented the REJECTED
by-value design for `visibleSearcher`/`affordances` and ran the suite, producing
four ACL test failures — most damningly `TestACLSearch_ScopeErrorMapping`
returning 200 with entity data where a 500 ACL failure was expected. That is
direct proof the closures are load-bearing and that a by-value capture would
have made those tests pass vacuously.

The reviewer also confirmed the gate tests are not vacuous in the other
direction: `TestACLSearch_DenyAllShortCircuit` carries a positive control
asserting the backend runs exactly once for a granted search, so it cannot pass
by never reaching the searcher.
