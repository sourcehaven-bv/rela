---
id: BUG-QSCRFRK
type: bug
title: Docs on relation write rules and import disagree with the code
description: 'Three review findings: POST relations are checked, the rule 422 detail named an entity, and rela import --mapping writes through the entity manager.'
priority: low
status: backlog
---

## Summary

The reviews of #2276 and #2302 found three gaps between the docs and the code:

- #2300: the metamodel guide said a create with `relations` is not checked
against relation rules. A data-entry POST writes those edges as relation
creates, which are checked.
- #2301: the guides said the write-rule 422 never names an entity. Its
`detail` named the first create target of the request.
- #2304: three guides said `rela import` writes to the store directly.
`rela import --mapping` writes through the entity manager.

## Fix

The rule refusal answers with the unwrapped `ValidationError`, so `detail` names
no entity; `TestRelationRule_PatchJudgesTheStateItLeaves` asserts it. The guides
name linked creates and `rela import` without `--mapping`.

GitHub: #2300, #2301, #2304
