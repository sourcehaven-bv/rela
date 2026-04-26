---
id: RR-DSB22
type: review-response
title: 'F1: readSourceSlice path-cleaning is belt-and-braces over os.DirFS'
finding: internal/lua/scripterror.go:269-274 cleans + rejects '../' prefixes before fs.Stat/fs.ReadFile. Since Go 1.20 os.DirFS already rejects escaping reads, so the in-builder check is redundant for the production caller. It is load-bearing for any future caller passing an FS rooted higher (e.g. os.DirFS("/")). The synthetic MapFS test is the only thing that exercises the prefix branch. No real bypass.
severity: nit
resolution: Added a comment in readSourceSlice explaining the prefix check is belt-and-braces over os.DirFS's Go 1.20+ root enforcement, so a future maintainer doesn't 'simplify' it away when the production caller is the only one exercised.
status: addressed
---
