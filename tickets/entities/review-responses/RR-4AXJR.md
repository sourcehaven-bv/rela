---
id: RR-4AXJR
type: review-response
title: Bare 'edit:' YAML block silently bypasses validation
finding: 'yaml.v3 deserializes a bare ''edit:'' (no subkeys) line to Edit == nil, NOT &DocumentEdit{}. Authors who write a stub ''edit:'' line they intended to fill in get no button and no error. validateDocuments only fires when Edit != nil, so misconfiguration is silent. Two fixes: (a) document the YAML semantics so authors know to write ''edit: {}'' if they want validation to catch missing fields, or (b) add a custom UnmarshalYAML / post-parse normalization that treats key-present-but-null as field-present-with-empty-subkeys. (a) is cheaper; (b) is the right fix.'
severity: significant
resolution: 'Documented the YAML semantic in two places: the DocumentEdit struct comment (config.go) explaining bare `edit:` deserializes to nil, and the docs paragraph in data-entry.md telling authors to write `edit: {}` if they want validation to fire on a stub block. Did not add custom UnmarshalYAML — disproportionate for one config field.'
status: addressed
---
