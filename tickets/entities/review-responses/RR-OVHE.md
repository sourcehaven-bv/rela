---
id: RR-OVHE
type: review-response
title: Bold/italic dropping pinned as 'limitation'
finding: TestMdTaskListLimitations subtests assert that bold/italic markers vanish — this is documented as a feature but is actually a bug. Scripts mutating text will silently lose user's formatting. Either fix to match strikethrough decision or rename test and file follow-up.
severity: nit
resolution: Replaced TestMdTaskListLimitations with TestMdInlineTextPolicy, which presents bold/italic/link dropping as the documented inline marker preservation POLICY (not as bugs). The policy is also documented in extractInlineText's doc comment. The asymmetry vs. strikethrough/code-spans is intentional and justified in the comment.
status: addressed
---
