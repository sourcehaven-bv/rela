---
id: RR-VLKV5
type: review-response
title: CI only runs just test on ubuntu-latest — cross-platform claim is unverified
finding: Plan says 'CI must exercise the service on Linux and macOS at minimum. Windows behavior verified via unit tests on GOOS=windows fakes.' .github/workflows/ci.yml runs just test on ubuntu-latest only. macOS/Windows run in release.yml. Cross-platform tests will be Linux-only in PR checks.
severity: significant
resolution: 'Add a cross-platform test matrix to CI for the userstate package (or selected cross-platform tests). Minimum: add macos-latest and windows-latest to the test job as a separate matrix entry that runs go test ./internal/userstate/... on each. Keep full-suite ubuntu-only to preserve build time. Plan explicitly includes the CI change as part of the PR.'
status: addressed
---
