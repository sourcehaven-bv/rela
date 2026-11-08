---
id: RR-OK5DCXE
type: review-response
title: Explicit --token-env pointing at an unset variable is ignored
finding: An unset variable silently sent no credential.
severity: minor
resolution: --token-env has no default; a named file or variable that is empty or unset is an error.
status: addressed
---
