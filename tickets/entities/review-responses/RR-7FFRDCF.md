---
id: RR-7FFRDCF
type: review-response
title: 'Nits: column headers, mentions boundary, entity view i18n, version hash'
finding: Column headers built once; mentions() matched renew.x; EntityTypeView text is English; versionOf now hashes an extra separator so stored drafts report a conflict once after upgrade.
severity: nit
resolution: Columns are computed; mentions() splits on a word boundary. EntityTypeView keeps English like the rest of that confirm text. The one-time version change is noted in the PR.
status: addressed
---
