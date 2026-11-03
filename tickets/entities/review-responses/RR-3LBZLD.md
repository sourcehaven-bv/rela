---
id: RR-3LBZLD
type: review-response
title: contextcheck exclusion now covers the whole of lifecycle.go
finding: .golangci.yml:367 widens a file-level suppression. It matches service/ops, but the file is new, so a `//nolint:contextcheck // reason` on the two store calls would keep the rest of the file checked.
severity: nit
resolution: Removed lifecycle.go from the .golangci.yml path exclusion. Added `//nolint:contextcheck` with a reason on the two store calls. The rest of the file is checked, and lint is clean on every tag set.
status: addressed
---
