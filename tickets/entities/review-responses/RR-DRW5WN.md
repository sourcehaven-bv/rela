---
id: RR-DRW5WN
type: review-response
title: Three prototype schemas carry the same defect and do not load
finding: 'Verified with `rela migrate --check`: prototypes/data-entry/schema.yaml (ticket), prototypes/data-entry/catalog/schema.yaml (product) and prototypes/data-entry/catalog-metamodel.yaml (product) each declare id_prefix without id_type, beside siblings that explicitly say id_type: manual — an incomplete edit rather than a deliberate choice. tickets/, docs-project/, prototypes/perf/project and prototypes/worlds/project are clean, so nothing CLAUDE.md names for perf work is blocked. catalog-metamodel.yaml additionally looks like an orphan of the schema.yaml rename, duplicating catalog/schema.yaml.'
severity: significant
status: open
---
