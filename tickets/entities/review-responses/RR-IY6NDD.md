---
id: RR-IY6NDD
type: review-response
title: The no-effect reload row does not check that it has a warning
finding: 'internal/realtimecfg/default_in_test.go:329-331,335: the "default_in with no effect" row is meant to show that a warning does not fail a reload, but it never asserts that next has the warning. If the warning stops firing, the row still passes. The three replace edits are also copied from TestDefaultIn_Checks "every keyed type has in: warns" (default_in_test.go:291-294). Fix: hoist the edits into one package-level []replace (for example allKeyedAuthored) used by both tests, and in the reload row require len(next.Warnings()) == 1 at defaultInKey before calling CheckReload.'
severity: minor
resolution: Hoisted the three edits into package-level allKeyedAuthored, used by TestDefaultIn_Checks and TestCheckReload_DefaultIn; the no-effect reload row now requires exactly one warning at session.default_in before calling CheckReload.
status: addressed
---
