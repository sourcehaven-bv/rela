---
id: TKT-IRLEA1
type: ticket
title: Opaque access.Row so field redaction runs at one structural point
kind: refactor
priority: low
effort: m
status: backlog
description: Make the gated reader return an opaque row type that can only become a wire value through the single redaction point.
---

## Description

Follow-up from the TKT-WFS778 design (question 4). The access reader returns
rows that are row-gated but not field-redacted, because verdicts and `when:`
grants are computed on raw rows and `visibility.Redact` must run exactly once.
Today that is a convention.

## Scope

- Introduce an opaque `access.Row` whose fields cannot be read except through the verdict computation and the single redaction step.
- Out of scope: changing what is redacted.
