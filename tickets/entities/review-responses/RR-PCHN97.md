---
id: RR-PCHN97
type: review-response
title: justCreated notice leaked across add cycles in RelationCards
finding: justCreated was cleared in addRelation() and cancelAdd() but not in selectTarget(). After inline-creating an entity and then clicking a different pre-existing search result, the UI showed "Created <inline entity> — link it below" pinned next to an unrelated target. The widget test suite mounted this component but never re-selected, so it passed.
severity: significant
resolution: selectTarget() now clears the notice first; handleInlineCreated sets it AFTER calling selectTarget (order matters, and is commented). Added a regression test that inline-creates, then picks a different target, and asserts the notice is gone.
status: addressed
---
