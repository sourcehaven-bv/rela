---
id: RR-CWEDAQ
type: review-response
title: rela-docs --version flag was dead (kong.Vars wired but no consumer)
finding: main.go passed kong.Vars{version} and goreleaser injects -X main.Version, but nothing consumed it — no VersionCmd or kong.VersionFlag field. `rela-docs --version` would error as unknown flag, and Version+kong.Vars were dead code.
severity: significant
resolution: 'Added a `Version kong.VersionFlag` field to the root cli struct. Kong now prints ${version} and exits before dispatch. Verified: `rela-docs --version` prints the injected version.'
status: addressed
---
