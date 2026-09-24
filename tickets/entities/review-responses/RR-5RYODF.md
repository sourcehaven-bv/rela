---
id: RR-5RYODF
type: review-response
title: EXPLAIN test hand-builds its hop and misses relations scans
finding: The hop hard-coded the target type and direction despite a docstring claiming no drift; only Seq Scan on entities was checked.
severity: minor
resolution: Hop comes from predicatefns.ResolveTraversal; docstring states the ungated lowering; the test also rejects Seq Scan on relations.
status: addressed
---
