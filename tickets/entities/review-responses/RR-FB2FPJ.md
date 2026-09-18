---
id: RR-FB2FPJ
type: review-response
title: Tool version duplicated between go install and the cache key; a bump would silently serve the old binary
finding: 'Each of the four pinned tool versions appeared twice — once in the `go install ...@vX.Y.Z` and once in the cache key — on top of the existing ''keep in sync with the justfile'' note, so three places each. Bumping the install without the key means the cache hits on the old key, the install step is skipped as a cache hit, and the old binary is served forever: a green run that lies about which version it checked.'
severity: minor
resolution: Hoisted each version to a job-level env var (GOLANGCI_LINT_VERSION, GO_ARCH_LINT_VERSION, PLIMSOLL_VERSION, COMMENTLINT_VERSION) referenced by both the install and the key, so the version is declared once per job and a bump necessarily rotates the cache.
status: addressed
---
