---
id: govulncheck-filtered-scan
type: automated-measure
title: govulncheck CI scan with explicit ignore list
description: Release and Security workflows run govulncheck via scripts/govulncheck-filtered.sh, which fails the build on any new vulnerability while explicitly ignoring documented OSVs that have no upstream fix (currently GO-2026-4923 in bbolt, reached transitively via blevesearch).
kind: ci
location: scripts/govulncheck-filtered.sh, .github/workflows/release.yml, .github/workflows/security.yml
status: active
---
