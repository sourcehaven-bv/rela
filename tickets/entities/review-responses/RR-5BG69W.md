---
id: RR-5BG69W
type: review-response
title: Exit code 0 always accepted contrary to docs
finding: docs said any code not in exit_codes is an error, but 0 always succeeds.
severity: minor
resolution: docs/checkers.md and the CheckerDef comment say 0 is always accepted.
status: addressed
---
