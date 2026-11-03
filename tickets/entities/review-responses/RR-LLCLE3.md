---
id: RR-LLCLE3
type: review-response
title: Allowlist entries themselves are not deduplicated
finding: Only role entries already in the output are skipped; duplicate types in the allowlist (e.g. from scope reopen) are kept.
severity: nit
reason: 'Harmless: duplicates do not change any grant decision, and the code makes no claim to deduplicate the allowlist.'
status: wont-fix
---
