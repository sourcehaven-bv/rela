---
id: RR-5A5Y6G
type: review-response
title: --expect false greens and gaps
finding: No refusal in whole-graph mode; some vs all conflated; rename not gateable; empty file handling.
severity: significant
resolution: --expect requires --principal or --webhook and refuses --verb/--type. `verb` means type-wide and `verb:some` means exceptions only, so widening shows as a diff. rename is rejected with a message. An empty or comment-only file expects no access.
status: addressed
---
