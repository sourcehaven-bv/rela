---
id: RR-QAEM5Z
type: review-response
title: Extracting the shared predicate weakens the grep guard exactly where it was designed to bite
finding: |-
    The plan extracts permitsByPermission(ctx, aclImpl, permission string) so nav and dashboard share one ACL switch, then widens TestNavFilterStaysPresentational's allowlist to cover the new handler file. That is defensible, but the plan understates the cost and gets the guard's mechanics slightly wrong.

    The guard (lint_test.go:68-110) greps for the literal "permitsNavEntry(" and fails if it appears in any non-test .go file in internal/dataentry other than views_handler.go. Its doc is explicit that the risk is live, not hypothetical: "A prose rule in CLAUDE.md failed to prevent that; a grep test does not decay."

    Two problems with the plan as written:

    1. The value of the guard comes from the allowlist being ONE file. A predicate callable from two files is materially easier to justify calling from a third — the reviewer's question shifts from "why are you calling this at all?" to "why is your file different from the two that already do?". The plan treats widening as free ("the intended way to use it") and cites the test's own doc as saying so. The doc does not say that; it says the opposite — that needing the predicate elsewhere is "the moment to stop and ask whether you want an authorization check instead".

    2. Extracting to a permission-string signature strictly increases misuse surface. permitsNavEntry(entry) is self-evidently about a nav entry; permitsByPermission(ctx, acl, "some:perm") reads like a general-purpose authorization helper and is one import away from being called on a write path, which is precisely the drift TKT-M1AX6P was reverted for.
severity: significant
resolution: 'Kept the shared switch (one copy of the read-only arm is worth more than the naming risk) but hardened against misuse on all three axes the finding named. (1) Renamed permitsByPermission → permitsGatedUIElement, which reads wrong anywhere but a presentation path. (2) The grep guard now carries TWO needles rather than one widened allowlist: permitsNavEntry( stays pinned to views_handler.go alone, and permitsGatedUIElement( is allowed only in views_handler.go plus the new dashboard handler. (3) The full ''presentation only'' godoc moves onto the extracted function — not left on the thin wrapper — and states that a caller wanting authorization must use authorizeCommand. The plan''s misreading of the test''s own doc is corrected: widening is a deliberate, argued exception, not ''the intended way to use it''.'
status: addressed
---

## Recommended resolution

Keep the shared switch (avoiding a second copy of the RR-XYO03L read-only arm is
worth more than the naming risk), but make misuse harder rather than easier:

1. **Name it for what it is, not what it takes.** `permitsByPermission` invites
general use. Prefer something that reads wrong on a write path — e.g.
`showsGatedUIElement` / `permitsUIElement`. The name is the cheapest guard.
2. **Keep the grep guard's teeth.** Guard the *extracted* name with the
two-file allowlist, and keep `permitsNavEntry(` pinned to `views_handler.go`
alone. Both needles, not one widened rule.
3. **Carry the "presentation only" godoc onto the extracted function**, and
state in it that a caller wanting authorization must use `authorizeCommand`. The
doc currently lives on `permitsNavEntry`; if it stays only on the thin wrapper,
the function everyone actually calls is the undocumented one.
4. Correct the plan's claim that widening the allowlist is "the intended way to
use it" — the test doc says the opposite. The widening is a deliberate, argued
exception, and the plan should say so in those terms.
