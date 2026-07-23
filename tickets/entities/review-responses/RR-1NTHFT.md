---
id: RR-1NTHFT
type: review-response
title: wrapDiscoverError duplicated verbatim across rela and rela-docs mains
finding: The no-project-hint policy was copied verbatim into both cmd/rela-docs/main.go and internal/cli/kong.go; a dependency-free func(error) error that must not drift between binaries.
severity: minor
resolution: Extracted to internal/errors.WrapDiscoverError (a commonComponent, importable by all; already owns ErrNoProject). Both mains call it; the TestWrapDiscoverError test moved alongside. errors package now 100% covered.
status: addressed
---
