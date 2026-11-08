---
id: RR-A44DAAX
type: review-response
title: Swapping only the store still writes under .rela and migrations/applied.json
finding: buildStateAndAliases falls back to state.NewFSKV; selectMigState opens filemigstate on the root; filecomments and lock files under .rela; piles/leases file KV on default build.
severity: critical
resolution: NewInMemory sets Config.FS = storage.NewMemFS() and projectConfig = rootfs.New(root) so config is read from disk and every write lands in memory. AC6 tree hash stays as backstop.
status: addressed
---
