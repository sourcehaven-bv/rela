---
id: RR-Q02K2Q
type: review-response
title: Underscore directories are scanned
finding: The go tool ignores directories starting with an underscore, but the walk descended into them.
severity: minor
resolution: The walk skips them; the synthetic tree test covers it.
status: addressed
---
