---
id: RR-2OP5SX
type: review-response
title: catalog prototype fixture still set the deleted allow_create/create_form knobs
finding: 'prototypes/data-entry/catalog/data-entry.yaml lines 44/49/74/79 still set allow_create: true and create_form: add_brand|add_tag. Only prototypes/data-entry/project/ was cleaned. Because nested unmarshal is non-strict these now parse to nothing, so the file documents behaviour it no longer has.'
severity: significant
resolution: Removed all four knob pairs from the catalog prototype. A repo-wide grep now finds allow_create only in the docs changelog note that announces its removal.
status: addressed
---
