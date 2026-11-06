---
id: RR-WM10ST6
type: review-response
title: Strict refuses unrelated writes on rows with old soft errors
finding: A patch of one field on a row that already misses a required property is refused under strict.
severity: significant
reason: 'Intended by DEC-OYV2AK: strict judges the stored state. Documented in the CLI reference as a migration step (fix rows with rela analyze properties first).'
status: wont-fix
---
