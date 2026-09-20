---
id: RR-1LSKDO
type: review-response
title: Regex terminator-exclusion deserves a property test
finding: '`((?:(?!-->)[\s\S])*)` is correct but subtle enough that someone will ''simplify'' it to a lazy `.*?`, at which point anchored backtracking silently reunites two comments into one. A test asserting rejection of any string with an interior terminator would make that regression loud.'
severity: nit
resolution: 'Added a table-driven test rejecting three interior-terminator shapes: `<!-- a --> b <!-- c -->`, `<!-- x --><!-- y -->`, and a nested `<!-- outer <!-- inner --> tail -->`. The test names the backtracking failure mode so the reason survives.'
status: addressed
---

Worth doing because this exact mistake was already made once during
implementation: the first version used a lazy quantifier and the doc comment
still records the `a --> text <!-- b` body it produced. A test is the durable
form of that note.
