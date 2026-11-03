---
id: RR-ENXFX5Z
type: review-response
title: Sender goroutine leaked on a failed first assembly
finding: NewLocalWithHints starts the sender; when assembly failed after buildRefHolders nothing stopped it, so each failed Configure save leaked one goroutine.
severity: minor
resolution: assemble now runs abandonFailedAssembly on error, which closes the job queue and stops a Local the failed assembly made itself (not a predecessor's). buildRefHolders does the same on its own late error.
status: addressed
---
