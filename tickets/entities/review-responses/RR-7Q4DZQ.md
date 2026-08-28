---
id: RR-7Q4DZQ
type: review-response
title: M2-test-only-api
finding: predicatefns.RruleNext and the re-exported ErrRruleExhausted had no non-test callers - grep found only the declarations. They existed solely so one test could assert a sentinel that is already tested at its home in internal/metamodel.
severity: minor
resolution: Deleted both. The through-Eval test keeps the operator-visible assertion; internal/metamodel owns the sentinel contract.
status: addressed
---
