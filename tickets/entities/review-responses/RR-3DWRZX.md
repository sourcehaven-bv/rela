---
id: RR-3DWRZX
type: review-response
title: TestAudit_UnnamedAutomationKeepsGenericLabel's comment described a different test
finding: It claimed to drive the runner directly and asserted the engine always supplies a name - neither is true
severity: significant
resolution: Comment rewritten to describe the test that actually exists (full Manager.CreateEntity path) and to drop the false claim that the engine always supplies a name. Assertion tightened from cascaded == 0 to an exact count of 2.
status: addressed
---

The comment said *"The engine always supplies a name in production, so this
drives the runner directly with an untagged Result."* The test does neither: it
constructs an `automation.Automation{Name: ""}` and goes through the full
`Manager.CreateEntity` path — which is the better test — and the parenthetical
is false, since an empty name is reachable from a user-authored schema.

Comment rewritten to describe the test that exists. Also tightened the assertion
from `cascaded == 0` to an exact count of 2, so a future change that stopped one
of the two cascade writes firing fails loudly rather than quietly reducing
coverage.
