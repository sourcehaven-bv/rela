---
id: RR-IL05NDW
type: review-response
title: Per-side field verdicts were untested
finding: The fake resolver returned the same verdicts for every row so judging each side was never exercised. Unset and enum-filter paths and a positive relation case were missing.
severity: significant
resolution: Added TestV1ConflictResolve_FieldWriteCases with a row-sensitive resolver where only one side is locked and the resolved row is not. Added the unset and enum cases and TestV1ConflictResolve_RelationMetaAllowed.
status: addressed
---
