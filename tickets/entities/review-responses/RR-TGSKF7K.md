---
id: RR-TGSKF7K
type: review-response
title: Role-holder narrowing had no batch bound
finding: One Narrowing branch per holder subject in one query could exceed backend bind-parameter limits for a role with many holders.
severity: significant
resolution: Chunk subjects at 500 per query.
status: addressed
---
