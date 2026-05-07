---
id: RR-6B5GM
type: review-response
title: PR must commit migrated in-tree configs and add CI guard
finding: Without committing migrated tickets/data-entry.yaml + prototypes/data-entry/*/data-entry.yaml, CI breaks (server bails on unmigrated config). PR must include the migrated YAML diffs. Add a CI check (or a Go test) that runs migration in --check mode against in-tree configs and asserts zero detections.
severity: significant
resolution: PR explicitly migrates and commits tickets/data-entry.yaml, prototypes/data-entry/project/data-entry.yaml, and prototypes/data-entry/catalog/data-entry.yaml. Added CI guard test in internal/migration/ that walks repo for data-entry.yaml files and asserts Detect()=false against each.
status: addressed
---
