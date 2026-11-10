---
id: RR-G0MYYIH
type: review-response
title: invalidOrgIDs is a shared mutable test variable
finding: Parallel tests share the slice.
severity: nit
reason: No test mutates it; a package-level table of inputs is common in this repo.
status: wont-fix
---
