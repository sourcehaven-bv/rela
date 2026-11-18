---
id: RR-ES28L79
type: review-response
title: directOnly ignores Any branches
finding: Any branches keep their inheritance expansion; a later foldHop that handles Any would hit the same refusal.
severity: minor
resolution: Documented in directOnly that only HasInbound is narrowed because foldHop refuses Any.
status: addressed
---
