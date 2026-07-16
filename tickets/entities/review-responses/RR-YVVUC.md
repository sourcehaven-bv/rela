---
id: RR-YVVUC
type: review-response
title: Collapse to one PurgeVersions(ctx, PurgeRequest) per capability; pgstore.Store at plimsoll ceiling (37)
finding: 'architect S1. *pgstore.Store is at EXACTLY max-exported-methods=37, zero headroom — ANY new exported method fails the plimsoll CI job immediately. Fix: keep the entity and relation capabilities SEPARATE (like HistoryReader/RelationHistoryReader, type-asserted independently) but collapse each to ONE method: `PurgeVersions(ctx, VersionPurgeRequest) (count int, err error)` where the request carries the target (id or triple), a vseq/content-hash selector, and an all-flag — so +2 exported methods, not +4. Bump the pgstore.Store plimsoll directive 37->39 with a comment noting these are interface-mandated optional-capability methods (Required-interface exception, TKT-N0IKN9 ratchet target), same rationale as the existing bumps. Also: address purge by vseq/content-hash in the request (NOT the fragile 1-based ordinal the ticket''s own ''stable handle'' requirement demands) — and surface vseq in `rela history` output so the operator can name it (architect M3 / cranky N1).'
severity: significant
resolution: 'Design revised: separate VersionPurger/RelationVersionPurger capabilities, ONE method each (PurgeVersions(ctx, VersionPurgeRequest)), +2 exported; plimsoll directive 37->39; stable vseq/content-hash selector; vseq surfaced in rela history. See revised design #6/#7.'
status: addressed
---
