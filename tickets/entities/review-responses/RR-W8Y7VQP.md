---
id: RR-W8Y7VQP
type: review-response
title: Resolver.MentionedBy does not gate its target
finding: '[security] With retired ids, MentionedBy(Y) on a hidden Y returns readable bodies that mention X when X->Y is retired. All callers gate the target first today, but the precondition is unwritten and untested.'
severity: minor
resolution: MentionedBy gates the target through ResolveIDsErr after cursor parsing; a hidden target returns an empty page. TestResolver_MentionedByGatesTheTarget pins it; query budgets and goldens updated by one fixed read.
status: addressed
---
