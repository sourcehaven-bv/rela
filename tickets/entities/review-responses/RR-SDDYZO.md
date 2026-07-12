---
id: RR-SDDYZO
type: review-response
title: FROM-only history gating is a TO-endpoint existence/content oracle via to_vseq
finding: 'The design gates relation-history read on the FROM entity''s read verdict only, then renders from_vseq/to_vseq endpoint references. If a caller can read FROM entity A but is ACL-denied entity Z (TO), a relation (A,t,Z) lets them read its history, and each version''s to_vseq confirms Z exists, reveals its version count/lineage, and (if the UI JOINs to render ''TO@v7'') potentially Z''s historical content. This re-opens exactly the hidden-entity existence/content oracle the entity work closed with 404-on-type-mismatch. Fix: gate relation-history read on BOTH endpoints (FROM ∧ TO), or redact the to_vseq reference and any JOINed TO content when the caller lacks read on TO. The ''FROM entity owns the history'' decision governs UI PLACEMENT only — it must not become the AUTHORIZATION boundary for TO-side data. Combine with L1 (resolve endpoint versions at read time) to make dual-endpoint redaction natural (cranky C4).'
severity: critical
resolution: 'Design revised: relation-history read now requires read verdict on BOTH endpoints (FROM ∧ TO). Endpoint versions resolved at read time with per-endpoint ACL (''endpoint hidden'' when denied), so no TO-side existence/content oracle. FROM-owns-history is UI placement only.'
status: addressed
---
