---
id: RR-6G3KI7
type: review-response
title: WithWorld has no production caller
finding: ScriptReader.WithWorld and UnrestrictedReader.WithWorld are not called by any wiring yet.
severity: minor
reason: The design keeps the world zero in Stage 1 (ruling 7.1); the wiring that sets a non-default world is TKT-7IZHP0. The seam is tested (TestScriptReader_Resolves).
status: deferred
---
