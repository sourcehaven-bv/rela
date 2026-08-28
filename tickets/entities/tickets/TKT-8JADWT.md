---
id: TKT-8JADWT
type: ticket
title: 'Tie the slog escaping assertion to the installed handler, not TextHandler directly'
kind: test
priority: low
effort: s
status: backlog
---

## Description

Raised as a known limitation by the G706 (log injection) enablement work in
TKT-7U8C8R, and accepted there rather than fixed.

That ticket path-excludes `internal/dataentry` from gosec's G706 check, and the
exclusion is sound only because two invariants are tested rather than assumed:
every slog message in the package is a compile-time constant, and the slog handler
escapes newlines so user data in a structured attribute cannot forge a log line.

The second test — `TestSlogTextHandlerEscapesNewlines` — constructs a
`slog.NewTextHandler` and asserts on its escaping. That is true of `TextHandler`,
and every current entry point installs one, but the test does not assert that the
*wiring* installs it. Swapping in a custom handler that does not escape would
leave the G706 exclusion in place while removing the property it depends on, and
nothing would fail.

This is a low-likelihood, high-blast-radius gap: no one is likely to replace the
handler, but if they do, the failure is silent and the exclusion becomes wrong.

## Solution

Assert the property against the handler the application actually installs rather
than one the test constructs. Options:

- Have the entry points build their handler through a single shared constructor,
and point the test at that constructor.
- Or drive a log line through the real initialization path and assert on the
captured output, so the test covers wiring and escaping together.

Either way the test should fail if a future handler swap removes the escaping
guarantee that the `internal/dataentry` G706 exclusion rests on.
