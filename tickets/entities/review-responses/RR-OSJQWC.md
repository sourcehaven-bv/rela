---
id: RR-OSJQWC
type: review-response
title: Handler-field assignment is a separate opt-in step test constructors must remember (nil-panic at request time if forgotten)
finding: Deps.handlers() centralizes the derivation, but ASSIGNING the result stays a separate step each deps-carrying test constructor must remember. A Server built without it does not fail loudly at construction — it nil-panics at request time on the first resolveType call. Both current call sites are correct, but this is the first slice of a multi-PR arc and every later extraction adds another field to the same forget-to-assign hazard. Reviewer suggested a test-only constructor so a half-wired Server cannot be built.
severity: minor
resolution: 'Closed structurally in the very next arc slice (TKT-MGNE5L / PR #1468) rather than deferred: the six handler groups were collapsed into a single named `handlerSet` struct EMBEDDED on Server, so wiring is one assignment (`s.handlerSet = deps.handlers()`) that cannot be partially forgotten — adding a future handler group adds a field to handlerSet and every call site picks it up automatically, with no per-field assignment to miss. Field promotion keeps every existing `s.trace.…` reference working. (That refactor was independently motivated: the 6-value return tripped gocritic''s tooManyResultsChecker.)'
status: addressed
---

Nit from the TKT-YUETL7 code review (cranky-code-reviewer, PR #1463). Reviewer's
verdict on the PR itself was a clean pass: normalized-receiver diff proved a
pure move; principalMiddleware still SDK-level with no principal in any handler
struct; handlers hold the narrow GraphReader (so a networked wiring substitutes
a gated reader without touching them); toolGetSchema/toolGetMetamodel aliasing
intact; directive matches the real count of 38; tests re-pointed only, with
TestACL_Trace_HiddenRootIsNotAnOracle surviving intact.
