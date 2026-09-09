---
id: RR-DRW5WN
type: review-response
title: Three prototype schemas carry the same defect and do not load
finding: 'Verified with `rela migrate --check`: prototypes/data-entry/schema.yaml (ticket), prototypes/data-entry/catalog/schema.yaml (product) and prototypes/data-entry/catalog-metamodel.yaml (product) each declare id_prefix without id_type, beside siblings that explicitly say id_type: manual — an incomplete edit rather than a deliberate choice. tickets/, docs-project/, prototypes/perf/project and prototypes/worlds/project are clean, so nothing CLAUDE.md names for perf work is blocked. catalog-metamodel.yaml additionally looks like an orphan of the schema.yaml rename, duplicating catalog/schema.yaml.'
severity: significant
resolution: 'Added id_type: short to the three flagged entities (ticket in prototypes/data-entry/schema.yaml, product in catalog/schema.yaml and catalog-metamodel.yaml). Chose short rather than manual because each declares an id_prefix, unlike their manual siblings which have none. Also converted the four `link: true` occurrences in prototypes/data-entry/catalog/data-entry.yaml to `link: detail`, since that was the remaining migration blocking that prototype from loading. `rela migrate --check` now reports ''No migrations needed'' for both prototype projects. The suspected catalog-metamodel.yaml orphan is left in place: deleting an operator-style file is out of scope here.'
status: addressed
---
