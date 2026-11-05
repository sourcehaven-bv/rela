---
id: RR-EQT49R7
type: review-response
title: Relation meta update gate fails open on store error and has a check-then-write race
finding: checkRelationMetaUpdate returned nil on any GetRelation error, skipping the gate. It also diffed changed keys against a row read outside the write Tx, so a gated key sent unchanged could overwrite a concurrent change without a check.
severity: significant
resolution: Only not-found passes, and then every key is gated; other errors refuse the update. Keys unchanged at check time are dropped from the written options, so only keys that passed the gate are written. Pinned by TestRelationMeta_UpdateGateFailsClosed.
status: addressed
---
