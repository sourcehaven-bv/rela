---
id: RR-NQQ70Z
type: review-response
title: Synthesized entity records violated the documented total-_title contract
finding: The resolve minted {id, type, properties:{}} records with no _title. entityDisplay documents _title as total — 'never empty for an API-sourced entity' — and these records rely on a downstream fallback rather than satisfying that contract. They also lack _actions/_world, so any future consumer assuming those breaks on exactly the rarest rows.
severity: significant
resolution: Set _title explicitly to the id, so the record satisfies the contract rather than depending on a fallback. Documented in outOfPageLinks.ts that these are synthesized rather than fetched, and that the id is the honest value since the relations endpoint carries no title. Pinned by 'sets _title to the id so the record satisfies the total-_title contract'.
status: addressed
---
