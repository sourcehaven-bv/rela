---
id: RR-CAFF
type: review-response
title: TestACLMiddleware_StampedPrincipalAttachesGate tests type, not behaviour
finding: 'The test asserts readGateFromContext returns aclReadGate (not nopReadGate). A future change that wires a broken aclReadGate (always returns true, wraps wrong Request) passes this test. Type-equality assertions pin implementation, not behaviour. Fix: have the next handler call g.Visible(ctx, type, id) under a known-denied policy and assert the boolean. The test already constructs a real policy/declarative; use it.'
severity: significant
resolution: TestACLMiddleware_StampedPrincipalAttachesGate now invokes gate.Visible against ticket and document under a policy granting read:[ticket]; asserts the booleans. A broken aclReadGate would fail the test.
status: addressed
---
