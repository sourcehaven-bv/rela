---
id: RR-J5D5ZO
type: review-response
title: 'max_items error says ''add list: true'' when list: true is present'
finding: 'internal/realtimecfg/validate.go:541. For ''list: true'' on a non-string type, p.List stays false, so max_items also gets ''only a list property has max_items; add list: true'' although the author wrote list: true. The second problem only confuses. Fix: pass raw.List to maxItems for the presence check (or skip the max_items problem when a .list problem was added).'
severity: nit
resolution: 'The max_items presence check now reads raw.List, so ''list: true'' on a non-string type gets only the .list problem. Added a load_test case that asserts the single problem.'
status: addressed
---
