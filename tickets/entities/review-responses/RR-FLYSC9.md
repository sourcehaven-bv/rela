---
id: RR-FLYSC9
type: review-response
title: cascadeHost.DeleteEntity family delete is not face-aware
finding: internal/entitymanager/cascadehost.go:182 reads the entity with the zero-face GetEntity (a faced type is not found) and writes one audit record with no per-face version capture. Automation-driven deletes of a faced family therefore do not follow the per-face contract.
severity: significant
reason: 'Out of scope for BUG-1YN750: automation cascades are system writes that do not authorize per principal. The per-face capture and audit there belongs to the faces-intrinsic programme and is reported to the coordinator for a follow-up ticket.'
status: deferred
---
