---
id: RR-3ZNTB0
type: review-response
title: sqlite derived query indexes lack trailing id until reconcile
finding: Between the v8 migration and the next Reconcile an upgraded database still has derived query indexes without id so the planner may pick entities_type_id_face_idx.
severity: minor
reason: appbuild reconcileDerivedSchemaIfSupported runs Reconcile on every sqlite assembly right after open and migration. Reconcile compares stored DDL and recreates the index. The window is the startup itself and costs speed only.
status: wont-fix
---
