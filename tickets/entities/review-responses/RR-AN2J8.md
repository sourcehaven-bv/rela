---
id: RR-AN2J8
type: review-response
title: Rewriter behaviour for returnPath=='' on non-form paths unspecified
finding: 'Plan says ''always inject return_to on any internal path'' but doesn''t say which branch wins when returnPath=='''' and the path is a non-form internal route with a pre-existing return_to. Two defensible interpretations: (a) strip any pre-existing return_to (rewriter owns it as single source of truth) or (b) pass through unchanged (conservative, but lets author-planted return_to slip through). Decide and document.'
severity: significant
resolution: 'Decision: always strip pre-existing return_to on every internal path, regardless of form-vs-non-form and regardless of whether we''re injecting a replacement. Documented in the decision table under Approach. Author-planted return_to values never reach the user.'
status: addressed
---
