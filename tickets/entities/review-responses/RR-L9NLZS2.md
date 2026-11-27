---
id: RR-L9NLZS2
type: review-response
title: Lint and gofmt still run repo-wide
finding: golangci-lint lints the whole repo and gofmt -w . rewrites unstaged files without restaging.
severity: nit
reason: Pre-existing behaviour outside this bug; the lint cache keeps it fast and scoping lint is a separate policy choice.
status: wont-fix
---
