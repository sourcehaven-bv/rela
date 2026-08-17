---
id: RR-CWI8HG
type: review-response
title: Update path is a second 23505 source (automation-set unique props) the plan ignores
finding: 'Manager.CreateEntity isn''t in one tx: after the committed+audited+broadcast create, automation may issue a second UpdateEntity that also raises 23505. UpdateEntity has NO isUniqueViolation handling today. Step-5 mapping must cover the UPDATE path; document the create-then-failed-automation-update outcome (row persists with pre-automation values audited as created).'
severity: significant
status: open
---

`Manager.CreateEntity` (manager.go:467) is NOT wrapped in store.Tx. Flow:
createCore (scan→CreateEntity, own tx, commits+emits) → recordEntityAudit →
automation → possibly a SECOND UpdateEntity for automation-set properties. That
UpdateEntity (pgstore/entity.go) is its own tx and will ALSO raise 23505 if an
automation sets a unique property that collides. Consequences the plan misses:
(1) by then the entity is already committed, audited (OpCreateEntity), and
broadcast (pg_notify+emit); if the derived index rejects the UpdateEntity,
CreateEntity returns an error to the caller but the row persists with
pre-automation values audited as "created" — undocumented behaviour to call out.
(2) Step-5 ConstraintName mapping must handle 23505 on the UPDATE path, not just
INSERT — UpdateEntity today maps ErrNoRows→ErrNotFound and returns raw error
otherwise, with NO isUniqueViolation handling at all.

REQUIRED: add isUniqueViolation + ConstraintName branch to the UpdateEntity
path; document the create-then-failed-automation-update outcome.
