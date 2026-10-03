---
id: RR-TGZZ70
type: review-response
title: A per-file count cannot see a swapped read
finding: Removing one allowed read and adding another in the same file keeps the count and passes.
severity: minor
resolution: Documented in the allowlist godoc as a limit that review covers.
status: addressed
---
