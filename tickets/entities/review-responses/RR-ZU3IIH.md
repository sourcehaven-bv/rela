---
id: RR-ZU3IIH
type: review-response
title: Pre-create checks are not shared
finding: Face, OpCreate authorization, field write gate, file and external_ref rejection and the manual-id check live in CreateEntity, not createCore; a third copy would drift.
severity: significant
resolution: 'Plan: extract prepareEntityCreate (counterpart of prepareRelationCreate) and call it from CreateEntity, Converter where it fits, and TaskItemConverter.'
status: addressed
---
