---
id: TKT-L0CACN
type: ticket
title: Dismiss individual prose-check findings
kind: enhancement
priority: low
effort: m
status: backlog
description: Let users dismiss a single prose-check finding without editing content or checker config; needs a stable finding hash.
---

## Problem

Prose checkers (TKT-EKY2TV) produce false positives. The only way to silence one
today is an inline marker in the content (`<!-- vale off -->`) or a change to
the checker's config. The first pollutes the text; the second is not something
rela should manage.

## Proposal

Let a user dismiss a single finding in the editor or via analyze, stored by rela
next to the entity, without touching content or checker config.

This needs a stable finding identity that survives unrelated edits. A line
number does not: any edit above the finding shifts it. A candidate is a hash of
checker name, rule, matched text and the normalized sentence around it. A
dismissal then stays valid until that sentence changes.

## Open questions

- Storage: an entity property, a side table like comments, or analysis
dismissals if those exist by then.
- Who may dismiss, and is a dismissal audited?
- How do dismissed findings show in analyze output (hidden, or counted)?
