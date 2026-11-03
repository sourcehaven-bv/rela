---
id: RR-U7GAKX
type: review-response
title: Migration 0030 leaves gap at 29 under a version-only migrator; header says 29
finding: migrate.go skips any m.version <= current, so a later 0029 is silently never applied on databases already at 30. The 0030 header comment still says "version 29". The stack's 0027_channel_type_refs also collides with develop's 0027_version_face_lineage; renumber contiguously on rebase.
severity: significant
resolution: The header now says 30 and documents that 29 is reserved for another stack branch and that the file is renumbered on rebase if 29 has not landed.
status: addressed
---
