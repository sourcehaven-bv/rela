---
id: RR-NR7C07D
type: review-response
title: Prompt character check is too strict and misses line separators
finding: unicode.IsControl plus Cf rejected ZWJ in emoji sequences and accepted U+2028/U+2029.
severity: minor
resolution: validatePagePrompt uses the shared texthazard.Find; table cases for an emoji ZWJ sequence (accepted) and U+2028 (rejected).
status: addressed
---
