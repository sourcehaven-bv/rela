---
id: RR-S1GT3AN
type: review-response
title: Doc cap counting rules and Go tool placement
finding: Cap counts which lines? Is commentlint (external release cycle) the right home given a repo-specific baseline?
severity: minor
resolution: Count prose lines only, excluding directive lines and bare // separators. Put the Go check in-repo under tools/ with the baseline; no commentlint release dependency.
status: addressed
---

Cap counts which lines? Is commentlint (external release cycle) the right home
given a repo-specific baseline?
