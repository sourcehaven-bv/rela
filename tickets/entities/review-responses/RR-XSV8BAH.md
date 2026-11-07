---
id: RR-XSV8BAH
type: review-response
title: 'Code: pg heal can deadlock with multi-row writers'
finding: The heal's batch UPDATE ... FROM unnest locked rows in join order. A cascading delete, rename or restore locking the same rows in another order could deadlock with it.
severity: significant
resolution: elementBatch.write now selects the target rows in key order with FOR UPDATE OF t SKIP LOCKED in a CTE and updates by ctid. A held row is skipped and stays NULL for the next pass. TestElementHealSkipsLockedRow holds a row lock in another transaction; it fails without SKIP LOCKED.
status: addressed
---
