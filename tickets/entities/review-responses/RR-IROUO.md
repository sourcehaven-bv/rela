---
id: RR-IROUO
type: review-response
title: 'ReDoS via =~ regex: catastrophic-backtracking pattern hangs the render thread'
finding: conditions.ts new RegExp(String(pattern)).test(value) compiles a config/binding-supplied pattern with no length cap or timeout and runs it on the main thread in a Vue computed. A catastrophic-backtracking pattern (e.g. '(a+)+$' against a long non-matching string) runs effectively forever (measured >120s at 35 chars, exponential). The try/catch only catches invalid-syntax regexes; a valid-but-pathological regex wedges the tab. Violates the fail-safe contract (doesn't throw, hangs). Empirically reproduced by the reviewer.
severity: critical
resolution: 'compareRegex now caps pattern length at MAX_REGEX_LENGTH (200 chars): an over-long pattern is rejected with a warn + false before new RegExp/test runs, so a catastrophic-backtracking pattern can''t wedge the render thread. Documented that =~ patterns are expected to be trusted config (defence-in-depth, not the boundary). Test ''rejects an over-long regex pattern instead of running it'' asserts a pathological binding-sourced pattern returns false in <100ms.'
status: addressed
---
