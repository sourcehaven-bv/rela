---
id: RR-L5GWLU
type: review-response
title: Blank-line seam fix only covered the island side (literal-side double blank still emitted MD012)
finding: 'cranky: the first seam fix trimmed only island-emitted trailing blanks; a literal segment ending in 2+ blank lines before an island (an author writing two blanks before a fence) still produced a double blank — the exact failure the ticket set out to kill, relocated to the literal side. Also: no test pinned the load-bearing island-trailing-blank trim with a resolver that actually emits one (md/h1/roles_matrix), nor the literal-side seam.'
severity: significant
resolution: 'Replaced the per-case trimming with a seamWriter that caps the blank run STRADDLING any segment boundary (buffer trailing newlines + next segment leading newlines) to a single blank line: 2+ -> one blank, exactly 1 -> line break, 0 -> no newline (so a mid-line echo never gains one). Segment interiors are never touched, so fenced ```markdown samples with intentional double blanks are preserved. Added tests: TestBuild_LiteralDoubleBlankBeforeIsland, TestBuild_ResolverTrailingBlankAtSeam (md()), TestBuild_MidLineEchoNoNewline, plus the existing literal-preservation/echo-trim/single-trailing-newline tests. Handbook output byte-identical; markdownlint 0 issues; docs coverage 78.2%.'
status: addressed
---
