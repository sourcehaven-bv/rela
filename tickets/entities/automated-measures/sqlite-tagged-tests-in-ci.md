---
id: sqlite-tagged-tests-in-ci
type: automated-measure
title: 'CI: sqlite-tagged tests must actually run'
description: 'ci.yml exercises the sqlite build tag only through go build and go list -deps, so no sqlite-tagged test has ever run on a PR — including internal/store/sqlitestore''s storetest.RunAll conformance suite, its fuzz functions, and the migration ladder. That is how BUG-LL3C07 survived on develop. The measure is a CI job running `go test -tags sqlite` over the packages the tag actually changes (internal/store/..., internal/appbuild/..., internal/cli/...), scoped rather than the whole tree twice.'
kind: ci
location: .github/workflows/ci.yml (SQLite Backend job)
status: active
---

`ci.yml` exercises the `sqlite` build tag only through `go build` and
`go list -deps`. No job runs `go test -tags sqlite`, so the entire
sqlite-tagged test surface — including `internal/store/sqlitestore`'s
conformance suite (`storetest.RunAll` plus its fuzz functions) and the
migration ladder — is unverified on every PR.

That is how BUG-LL3C07 survived: a test whose fixture only works on fsstore
has been failing under the sqlite tag on `develop`, and nothing looked.

## The measure

A dedicated `SQLite Backend` job running `go test -tags sqlite -shuffle=on`
over `internal/store/...`, `internal/appbuild/...` and `internal/cli/...`.

Its own job rather than a step in the postgres one, which is where it was
first written and where CI caught the mistake. That job sets
`RELA_TEST_DATABASE_REQUIRED=1` so a missing DSN hard-fails instead of
skipping — correct for proving backend parity, and exactly wrong here: the
sqlite build has no DSN, so ~30 DB-gated pgstore tests that would otherwise
skip failed instead. A separate job also keeps this off the critical path of
one that waits on a service container.

Deliberately scoped rather than the whole tree twice: the value is in the
packages the tag actually changes, and a full second run would spend minutes
re-verifying code byte-identical to what the Test job already ran.

No `-race`: the Test job races the same shared packages on the default
backend, and sqlitestore's own concurrency is covered by the conformance
suite. Worth adding if this job ever grows a genuinely sqlite-specific
concurrency test.

The job runs on `ubuntu-26.04` and installs bubblewrap, matching the Test job
on both counts. 24.04 restricts unprivileged user namespaces via AppArmor, so
the sandbox fails there with `bwrap: Failed RTM_NEWADDR: Operation not
permitted` even when bubblewrap is installed. `internal/cli` is in
scope and its render/export tests shell out through `internal/cmdexec`, which
fails closed with no sandbox available — so without it they do not skip, they
fail. CI caught that too, on the second run.

## Verified, not assumed

Reverting the build-tag fix and running the job's exact command reproduces
`--- FAIL: TestBuild_StateRows_WarnAtStartup`. The measure catches the defect
it was written for.

A full `go test -tags sqlite ./...` sweep found exactly one failing test, so
the anticipated tail of further pre-existing failures does not exist.
