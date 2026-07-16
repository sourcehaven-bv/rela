---
id: RR-N16SDV
type: review-response
title: Non-existent entity-id silently returns global+everyone readers (misleading attestation)
finding: 'ForEntity/computeForEntity never fetch the entity; ancestors() for a non-existent ID returns just the seed ID and finds no edges, so a typo''d or deleted entity-id silently yields only global + everyone attributions with no error (resolver.go ancestors, request.go ForEntity). A security officer running `who-can read INC-04X` (typo) would see a short, reassuring reader list and wrongly attest a nonexistent entity as safe. Fix: gate on an explicit store.GetEntity existence check (returns ErrNotFound) before reporting; error clearly on missing entity.'
severity: significant
resolution: Plan adds an explicit store.GetEntity existence gate before any reporting; missing entity errors (ErrNotFound) rather than returning global+everyone. Acceptance criterion + test added.
status: addressed
---
