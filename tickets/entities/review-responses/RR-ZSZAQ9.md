---
id: RR-ZSZAQ9
type: review-response
title: Lua test adapters named GetAddress still read by raw id
finding: gatedReader and failingElevatedReader in acl_bypass_reads_test.go implement GetAddress with raw.GetEntity(ctx, id), so ID@face misses.
severity: minor
resolution: Both now read through store.GetEntityAt with an addr parameter, like rawAddressReader.
status: addressed
---
