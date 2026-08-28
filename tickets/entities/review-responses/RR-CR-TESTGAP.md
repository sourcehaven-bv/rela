---
id: RR-CR-TESTGAP
type: review-response
title: 'No test asserted wrapCss output was valid CSS, which is why the regex bugs were invisible'
finding: 'Every test in relaCssLayer.test.ts asserted `toContain(...)` on a substring. None asserted the result was structurally valid. That is precisely why the two critical regex bugs shipped with a green suite — the tests verified that expected fragments were present, never that the surrounding output was well-formed.'
severity: significant
status: addressed
resolution: 'Added a 14-case adversarial table (comment-with-brace, string-with-brace, url-with-brace, nesting, @media, @font-face, @keyframes, @supports, @import, @charset, consecutive tokens, empty, ...). Each case asserts (a) postcss.parse() does not throw, (b) re-stringifying a parsed tree is stable (catches text spliced into a comment), and (c) the set of declarations is preserved exactly — catching content dropped or hoisted out of its rule, the failure mode where output stays balanced but means something different. Note the reviewer suggested a raw brace-count assertion; that was tried and is WRONG, because braces legitimately appear inside comments, strings and url(). Parser-based validity is the correct oracle.'
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
