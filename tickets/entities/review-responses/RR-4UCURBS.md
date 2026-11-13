---
id: RR-4UCURBS
type: review-response
title: Bounded reason for rela.list_entities was too broad
finding: The allowlist reason claimed every store pages by keyset, but the load-then-filter fallback and memstore read the whole type.
severity: minor
resolution: Reworded the reason to name where the bound holds and where it does not.
status: addressed
---
