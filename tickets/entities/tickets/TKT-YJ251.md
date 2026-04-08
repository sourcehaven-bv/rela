---
id: TKT-YJ251
type: ticket
title: Allow cli to depend on rename in arch config
kind: chore
priority: low
effort: xs
status: done
---

After PR #353 migrated rename orchestration into workspace, the rename package
became a types-only DTO package. `internal/cli/rename.go` imports rename for
`rename.Options` when calling `ws.Rename`, but the `.go-arch-lint.yml` allowlist
was missed in that PR. This adds `rename` to `cli.mayDependOn`, matching the
existing `mcp→rename` dependency.
