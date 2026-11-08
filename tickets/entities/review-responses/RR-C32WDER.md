---
id: RR-C32WDER
type: review-response
title: RelationCards description fallback untested
finding: RelationCards now shows the relation type description; no test covered the fallback or the precedence of authored help.
severity: minor
resolution: 'Two tests in RelationCards.test.ts: the fallback renders as Markdown, and authored help wins.'
status: addressed
---
