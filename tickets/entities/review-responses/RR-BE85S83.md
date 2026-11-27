---
id: RR-BE85S83
type: review-response
title: Configure writer must stay strict
finding: If the config writer used the lenient loader, operators could write new deprecated syntax through the API.
severity: nit
resolution: 'Verified: cmd/rela-server/configure.go uses metamodel.Parse and projectsetup.CheckDataEntry, which stay strict.'
status: addressed
---
