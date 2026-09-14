---
id: RR-ZHXSDI
type: review-response
title: HasConfiguredScan iterates only Entities, not relation properties
finding: HasConfiguredScan iterates p.m.Entities only. Relations also have Properties (types.go:531, validated by validateRelationProperties), so if a relation type could carry a file property, a project scanning only via relations would get no warning while every relation upload failed closed. Noted as mirroring a pre-existing gap in HasUnconfiguredScan and probeAttachmentCommands — i.e. adding a third copy of the same blind spot.
severity: minor
reason: 'Not applicable: attachments are entity-only. Verified by grep — no PropertyTypeFile reference anywhere in the codebase is coupled to relations, and internal/attachment contains no Relation handling at all (zero non-test matches). There is therefore no relation upload path that could fail closed unwarned. Matching the existing HasUnconfiguredScan / probeAttachmentCommands entity-only walk is correct rather than a third copy of a blind spot. Should relation file properties ever be introduced, all three walks would need updating together and a shared helper would be justified then.'
status: wont-fix
---
