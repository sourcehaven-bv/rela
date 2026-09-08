---
id: RR-0FNW41
type: review-response
title: Hoist the one-event-per-write contract into the storetest conformance harness and add a package-level guard
finding: The 'a write through the store produces exactly one event' contract is backend-agnostic but is pinned only in internal/app, where a store author will not look; and the class 'capability that must be installed exactly once per FS' has no package-wide guard.
severity: nit
reason: Out of scope for the bug fix. With fan-out observers the install-exactly-once class no longer exists (nothing evicts). The conformance-harness hoist is a good follow-up for the fsstore arc (TKT-9XDEY0 onward) where storetest is already being exercised.
status: deferred
---
