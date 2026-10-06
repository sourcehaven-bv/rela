---
id: RR-WERY5D
type: review-response
title: An explicit false could not be stored
finding: setAt treated false as remove, so unticking a default-true setting fell back to the default.
severity: minor
resolution: false is stored; only undefined, empty string and null remove.
status: addressed
---
