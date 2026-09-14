---
id: REV-4KQ7
type: review-checklist
title: 'Review: scheduled-mail fan-out demo test + maildemo compile gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Both demo tests pass: `go test -tags maildemo ./internal/appbuild/ -run TestDemo_ScheduledMail` → ok, 2.896s
- [x] Demo compiles against current develop with no source changes needed (`go vet -tags maildemo ./internal/appbuild/` clean)
- [x] The new CI step's exact command passes locally (`go vet -tags maildemo ./internal/mail/ ./internal/appbuild/`)
- [x] `.github/workflows/ci.yml` parses as YAML; the step lands in the existing `demos` job
- [x] `just ci` green
- [x] ~~Coverage~~ (N/A: test-only change behind a build tag, no production code touched — floors cannot drop)

## Manual Review

- [x] Verified the demo is not redundant with the pre-existing `internal/mail/demo_test.go`: that one covers SMTP render/delivery to Mailpit, this one covers scheduler fan-out + ACL scoping over the http and script transports. Complementary, not duplicate.
- [x] Verified the demo bites rather than merely passing: bob's rendered text omits `EUR 42000` (budget, redacted by `visible:`) and `EUR 71000` (salary_review, row-denied) while alice's contains both, and the run reports `children=2 dropped=0` with the inactive third person excluded.
- [x] Confirmed the test is self-contained — an in-process `httptest` stub, no Mailpit or other external service, so it is safe to compile and run anywhere.
- [x] Confirmed no production code is modified: the diff is one new `_test.go` file, one CI step, plus ticket entities.
- [x] CI decision recorded and justified in the ticket: compile-check the tag, do not run the tests. Rationale is that `go build ./...` skips `_test.go`, so tagged tests have no CI coverage today.
- [x] Rot risk verified as real, not theoretical: `go vet -tags e2e ./internal/dataentry/` currently FAILS (`not enough arguments in call to NewApp`). That file is untouched since PR #817 while `NewApp` gained `store.VersionService`, `search.VisibleSearcher`, and `state.KV`. Left unfixed deliberately — out of scope, noted in the ticket and PR body.
- [x] The `maildemo` tag choice follows the existing convention rather than inventing one; the pre-existing `internal/mail/demo_test.go` already uses it, so one step covers both packages.
- [x] Test-only code lives in `_test.go` inside package `appbuild` (not `appbuild_test`) because `Services.mail` is unexported; this mirrors the trade `internal/mail/export_test.go` already makes and cannot reach a binary.
