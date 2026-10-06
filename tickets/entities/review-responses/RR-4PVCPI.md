---
id: RR-4PVCPI
type: review-response
title: Loader and EntityGetter overlap without saying so
finding: Two near-identical interfaces invite drift.
severity: nit
resolution: The Loader godoc now states it is EntityGetter plus ListEntities and why EntityGetter remains (tracer and ScriptReader never query a world).
status: addressed
---
