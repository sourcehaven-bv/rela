---
id: TKT-4KQ7ZP
type: ticket
title: 'End-to-end demo test for scheduled mail fan-out over http + script transports'
kind: test
priority: low
effort: s
status: done
---

## Description

Add a build-tag-gated (`maildemo`) demo test that drives the whole declarative
mail chain end to end, and give the `maildemo` tag a compile check in CI so it
cannot silently rot.

The mail arc (TKT-332QZY, TKT-DS1CR6, TKT-U2R7GU, TKT-XWZIOB) is covered by unit
tests per package. What was missing is one artifact showing the pieces working
*together*, from a `schedules.yaml` declaration through to the bytes on the
wire, in a form a human can read.

`internal/mail/demo_test.go` already does this for the SMTP render/send half
against Mailpit. This ticket adds the other half: scheduler `for_each` fan-out
and per-recipient ACL scoping, over both non-SMTP transports.

## What the demo shows

`internal/appbuild/scheduled_mail_demo_test.go` boots a real project (metamodel,
`acl.yaml`, mail templates, `schedules.yaml`, entities), runs a real scheduler
over the real job queue, and asserts four properties:

1. `for_each` produces one message per selected recipient — not a broadcast —
   and the `where: [active = true]` filter excludes the inactive person.
2. The ACL changes the content: the same template rendered for a `manager` and
   for a `worker` differ, with `task.budget` redacted by `visible:` and the
   whole `salary_review` row absent for the worker.
3. The wire body is APIv2-shaped, with `account_id` in the path and the token
   from `.rela/secrets.yaml` as a bearer header.
4. Payloads are written to `/tmp/maildemo-*/` for inspection.

Two tests, one per transport:

- `TestDemo_ScheduledMailOverHTTP` uses `mail.WithHTTPBaseURL`, the Go-only seam,
  because `transport: http` has a compile-time endpoint on purpose.
- `TestDemo_ScheduledMailOverScript` uses no seam at all: the URL comes from the
  send script via `.rela/secrets.yaml`, exactly as an operator would deploy it.

Both run against an in-process `httptest` stub, so the demo is self-contained —
unlike the Mailpit demo, it needs no external server.

## CI decision

The tests stay OUT of the normal CI path. They print a narrated walkthrough and
dump payloads for a human to read; as a pass/fail gate that buys nothing over
the existing unit tests.

They are still compile-checked, because `go build ./...` does not compile
`_test.go` files, so a tagged test file is invisible to every existing CI step.
That is not hypothetical: `internal/dataentry/e2e_test.go` (tag `e2e`) has not
compiled since `dataentry.NewApp` gained parameters, and nothing caught it.

A `go vet -tags maildemo` step in the existing `demos` job keeps the demo's
references to `appbuild.Services.mail`, `mail.NewHTTPSender` and
`scheduler.NewWithQueue` honest, for a few seconds of CI time.

## Scope: IS NOT

- No production code changes. The demo uses existing test seams only.
- No fix for the rotted `internal/dataentry/e2e_test.go`; that is pre-existing
  and belongs in its own ticket.
- No CI coverage for the `mailmanual` or `e2e` tags.

## Acceptance criteria

1. `go test -tags maildemo ./internal/appbuild/ -run TestDemo_ScheduledMail`
   passes with no external services running.
2. The demo asserts fan-out cardinality, the `where` filter, per-recipient ACL
   redaction on both transports, and the APIv2 request shape.
3. CI compiles the `maildemo` tag and fails if it stops compiling.
4. No production code is modified.
