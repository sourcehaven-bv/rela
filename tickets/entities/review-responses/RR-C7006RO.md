---
id: RR-C7006RO
type: review-response
title: Engine load errors use the old key names
finding: 'engine.go and notify.go errors say automation %q / condition / set: / create_entity while Check says trigger %q. ''condition requires entity:'' is misleading now that when: is the predicate. Fix: reword to trigger / when / set / create.'
severity: minor
resolution: Engine and notify errors now say trigger %q / when / set / create; the synthetic golden jsonl was regenerated (message texts only) and the appbuild boot test expects the new text.
status: addressed
---
