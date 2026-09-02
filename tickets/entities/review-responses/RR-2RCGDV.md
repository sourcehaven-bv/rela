---
id: RR-2RCGDV
type: review-response
title: write_handler.go imports internal/entity twice under two names
finding: The new bare entity import duplicates the file's existing entityPkg alias and golangci-lint fails on it
severity: minor
resolution: Dropped the duplicate bare import; entityMutator's signatures now use the file's existing entityPkg alias. just lint is clean.
status: addressed
---

`internal/dataentry/write_handler.go:18-19` now has both:

```go
"github.com/Sourcehaven-BV/rela/internal/entity"
entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
```

The bare import was added for the new `entityMutator` signatures while the file
already imported the package as `entityPkg` (used throughout). It compiles, but
the same type is now spelled two ways in one file. `just lint` fails on it
(gocritic `dupImport`, 2 issues).

Use the existing `entityPkg` alias in the interface and drop the new import.
