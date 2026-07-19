---
id: RR-QWFGJ
type: review-response
title: Write-method contract docs live on the wrappers, cores undocumented
finding: The CreateEntity→createEntity renames leave the unexported cores without doc comments; the contract godoc lives on the exported wrappers in tx.go.
severity: nit
reason: 'Deliberate: the wrapper IS the public surface and carries the contract; duplicating it on the cores would drift. Reviewer verified no core lost an invariant-bearing comment and no rename changed behavior.'
status: wont-fix
---
