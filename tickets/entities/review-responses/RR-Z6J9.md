---
id: RR-Z6J9
type: review-response
title: Hard-break test does not pin <br> position
finding: querySelectorAll('br').length === 1 proves a <br> exists somewhere in the paragraph, but not that it sits between 'foo' and 'bar'. Test name claims position-aware semantics without enforcing them.
severity: minor
resolution: Added an `innerHTML` regex `/foo\s*<br[^>]*>\s*bar/` assertion plus child-node-tag inspection to pin the <br> position between the two text fragments. The structural count check is preserved as well.
status: addressed
---
