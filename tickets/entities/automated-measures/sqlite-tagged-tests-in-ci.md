---
id: sqlite-tagged-tests-in-ci
type: automated-measure
title: 'CI: sqlite-tagged tests must actually run'
description: 'ci.yml exercises the sqlite build tag only through go build and go list -deps, so no sqlite-tagged test has ever run on a PR — including internal/store/sqlitestore''s storetest.RunAll conformance suite, its fuzz functions, and the migration ladder. That is how BUG-LL3C07 survived on develop. The measure is a CI job running `go test -tags sqlite` over the packages the tag actually changes (internal/store/..., internal/appbuild/..., internal/cli/...), scoped rather than the whole tree twice.'
kind: ci
location: .github/workflows/ci.yml (job to be added)
status: proposed
---

`ci.yml` exercises the `sqlite` build tag only through `go build` and
`go list -deps`. No job runs `go test -tags sqlite`, so the entire
sqlite-tagged test surface — including `internal/store/sqlitestore`'s
conformance suite (`storetest.RunAll` plus its fuzz functions) and the
migration ladder — is unverified on every PR.

That is how BUG-LL3C07 survived: a test whose fixture only works on fsstore
has been failing under the sqlite tag on `develop`, and nothing looked.

## The measure

A CI job running `go test -tags sqlite` over the sqlite build's own packages
plus the shared ones whose behaviour it changes (`internal/store/...`,
`internal/appbuild/...`, `internal/cli/...`).

Deliberately scoped rather than the whole tree twice: the value is in the
packages the tag actually changes, and a full second run would double CI time
to re-verify code the default job already covers.

Expect the first run to surface further pre-existing failures. Triage them in
that pass rather than filing one ticket per assertion.
