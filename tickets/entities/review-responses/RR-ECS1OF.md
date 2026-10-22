---
id: RR-ECS1OF
type: review-response
title: The general bool check refuses configs that load today (yes/on/no/off)
finding: 'Step 3 refuses any bool scalar whose ShortTag is not !!bool, and AC 2 adds TestParse_Invalid rows for "final: yes". But yaml.v3 decodes y/yes/on/n/no/off (all cases) into a typed bool for YAML 1.1 compatibility (gopkg.in/yaml.v3 decode.go:678-693), so "final: yes" and "required: on" load as true today. After this change, such a realtime.yaml fails at boot and on reload, and a reload refusal keeps the old config without the operator noticing. Risks only mentions "changes the error text". Plan change: either (a) flag only scalars the decoder would reject (not !!bool and not one of the 1.1 words), so behaviour is unchanged and the message gains the key path, with AC 2 rows "typing: 1" and "typing: \"true\"" and a row proving "final: yes" still loads; or (b) make the refusal deliberate: list it under Risks as breaking, give the migration in the guide, and add a reload test that the old config is refused with the key path.'
severity: significant
resolution: 'Plan updated: the bool check decodes with yaml.v3 itself (node.Decode), so yes/no/on/off still load; only already-refused values gain a key path; TestParse_BoolKeysUnchanged added (D12, AC 2).'
status: addressed
---
