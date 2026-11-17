---
id: RR-1ULY95T
type: review-response
title: Includes and future trigger migrations
finding: Per-file migrations touch only the root schema; flows.yaml has no FileType; external includes have no documented path.
severity: minor
resolution: 'Plan: run metamodel per-file migrations on in-project includes before the move; add FileTypeFlows; document manual steps for external and namespaced includes.'
status: addressed
---
