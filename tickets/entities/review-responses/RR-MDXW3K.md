---
id: RR-MDXW3K
type: review-response
title: Required-fields path drops atomic-path guarantees
finding: InlineCreateFormModal loses the recipe face, needs a form id the recipe lacks, turns the mapping into editable prefill, skips the update-on-source check, and can leave an unlinked entity.
severity: significant
resolution: 'Plan: preflight returns the required fields; a small dialog shows only those and posts them as properties to the atomic endpoint. InlineCreateFormModal is not used.'
status: addressed
---
