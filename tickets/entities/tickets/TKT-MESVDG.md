---
id: TKT-MESVDG
type: ticket
title: ACL test coverage for per-recipient scheduled mail rendering
kind: test
priority: medium
effort: s
status: done
---

## Description

The core claim of scheduled-mail `for_each` fan-out — that each message is
rendered under its OWN recipient's ACL principal, with row denial and field
redaction applied — was never verified against a real policy.

The only test touching `RunScheduledTemplate`
(`TestRunScheduledTemplateSendsRenderedRecipientMessage`) sets `fieldRedactor:
visibility.NopRedactor{}` and supplies no `aclDeclarative`, so it exercises the
rendering plumbing with access control switched off.

The project's own planning document promised this test. PLAN-XMWT23 AC7: *"Deny
one row and redact one field... assert both are absent."* It was not in the
diff.

GitHub issue #1474. Source: IB-review rela#1455. Severity: moderate.

**Violated requirement**: CONTROL-5-15 — rules for controlling access to
information shall be established and implemented. A new fan-out mail feature
that may show per-recipient data lacked automated verification of that access
mechanism.

## Why it matters

The regression this would catch is the worst one this feature has: a break in
recipient scoping that mails one person another person's data. It is silent —
the mail still sends, still looks correct, and only the wrong recipient can
tell.

## Scope

IN: tests that drive `RunScheduledTemplate` under a real `acl.Declarative` and
`visibility.PolicyRedactor`, asserting a denied ROW and a redacted FIELD are
both absent from the rendered content.

OUT: changing the implementation. This is a test-coverage ticket — the behaviour
was verified correct, it simply had nothing pinning it.
