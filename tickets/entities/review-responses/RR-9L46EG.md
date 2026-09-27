---
id: RR-9L46EG
type: review-response
title: Reconcile duplicates the Tx code and accepts a Tx view
finding: Reconcile pins its own connection with BEGIN IMMEDIATE and does not reject a Tx view, whose own open transaction holds the write lock.
severity: minor
resolution: 'Reconcile refuses a Tx view with an error. Its doc explains why it does not use Store.Tx: a dry run must roll back a successful transaction, and DDL emits no store events.'
status: addressed
---
