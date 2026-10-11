---
id: RR-0JABKB
type: review-response
title: Auto-update cannot bump the toolchain
finding: 'security.yml runs go get stdlib@v1.26.9 for a stdlib finding, which fails, so the scan files an issue instead of a PR. #1817 came from this and would recur on every Go patch release.'
severity: significant
resolution: 'The auto-update maps module stdlib to go get toolchain@go<ver> and runs with GOTOOLCHAIN=auto so the re-scan uses the new toolchain. All workflows use go-version-file: go.mod, so the bot PR only touches go.mod and go.sum.'
status: addressed
---
