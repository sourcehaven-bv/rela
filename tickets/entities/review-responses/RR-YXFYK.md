---
id: RR-YXFYK
type: review-response
title: RenameEntity corrupts search_text for mixed-case IDs (unsearchable after rename)
finding: 'go-architect review (C1): RenameEntity spliced the raw mixed-case newID into search_text via SQL `$3 || substr(search_text, length($1)+1)`, but search_text is otherwise all-lowercase (entitySearchText lowercases everything). A renamed entity with a mixed-case new ID became unfindable by that ID. Reproduced against live Postgres: create ''Old-ID'', rename to ''New-MixedCase'', search ''new-mixedcase'' -> [] (empty). The conformance suite missed it because RunSearchTests + rename tests use same-case IDs. Root cause (S1): search_text had two writers that had to agree byte-for-byte (Go entitySearchText on create/update; SQL substr-splice on rename).'
severity: critical
resolution: 'Fixed in entity.go RenameEntity: instead of splicing, the rename now updates the ID then recomputes search_text in Go via the canonical entitySearchText(renamed) builder — a SINGLE writer of the column. Also avoids the latent assumption that lower() is byte-length-preserving (false for some Unicode the store permits). Verified: the mixed-case rename->search reproduction now passes; full conformance + fuzz still green with -race. Adding a permanent pgstore regression test (conformance uses same-case IDs so this gap stays covered locally).'
status: addressed
---
