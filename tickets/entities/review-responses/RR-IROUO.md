---
id: RR-IROUO
type: review-response
title: 'ReDoS via =~ regex: catastrophic-backtracking pattern hangs the render thread'
finding: conditions.ts new RegExp(String(pattern)).test(value) compiles a config/binding-supplied pattern with no length cap or timeout and runs it on the main thread in a Vue computed. A catastrophic-backtracking pattern (e.g. '(a+)+$' against a long non-matching string) runs effectively forever (measured >120s at 35 chars, exponential). The try/catch only catches invalid-syntax regexes; a valid-but-pathological regex wedges the tab. Violates the fail-safe contract (doesn't throw, hangs). Empirically reproduced by the reviewer.
severity: critical
resolution: |-
    SUPERSEDED by TKT-CCFUQ / issue #1139 — see RR-HPQV2. The original resolution below was WRONG on its central claim and is retained for the record, not as guidance.

    ORIGINAL (incorrect): "compareRegex now caps pattern length at MAX_REGEX_LENGTH (200 chars): an over-long pattern is rejected with a warn + false before new RegExp/test runs, so a catastrophic-backtracking pattern can't wedge the render thread. Test 'rejects an over-long regex pattern instead of running it' asserts a pathological binding-sourced pattern returns false in <100ms."

    WHY IT WAS WRONG: a length cap cannot bound catastrophic backtracking, because a hostile pattern is SHORT and blows up on TINY inputs. Measured: `(a+)+$` is 6 chars (far under the 200 cap) and hangs >60s against a 41-char value. The cap only ever bounded LINEAR work. The regression test was also hollow — `(a+)+$` vs all-`a`s MATCHES in 0.11ms, so it asserted a fast match, not a prevented hang (RR-P1QZK).

    WHAT THE ORIGINAL GOT RIGHT: "=~ patterns are expected to be trusted config (defence-in-depth, not the boundary)" — that observation was the actual fix, but it was recorded as a footnote to a cap rather than enforced. TKT-CCFUQ enforces it: the parser now REJECTS any non-literal `=~` pattern, removing the untrusted-pattern path (`form.v =~ form.pat`) instead of mitigating it. The value cap remains only as hygiene, explicitly relabelled as not-a-ReDoS-control.
status: addressed
---
