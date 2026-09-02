---
id: RR-EE4DIU
type: review-response
title: 'writeHandler lost its doc comment: the interface was inserted between the comment and the type'
finding: Go attaches a comment to the declaration that follows it - writeHandler's block now documents entityMutator instead
severity: significant
resolution: Confirmed - writeHandler had lost its doc comment entirely. Moved entityMutator above writeHandler's doc block so the comment reattaches to the type it documents.
status: addressed
---

In `internal/dataentry/write_handler.go`, `entityMutator`'s declaration was
inserted between `writeHandler`'s doc comment and `type writeHandler struct`,
with no blank line separating the two comment blocks. The previous block ends
`// NOT part of this refactor.)` and the next line is `// entityMutator is the
write surface...`.

Go attaches a doc comment to the declaration immediately following it, so
`writeHandler`'s carefully-written explanation of its collaborator shape and the
`writeMu` pointer now documents `entityMutator`, and `writeHandler` has **no doc
comment at all**. A silent documentation regression on the type that most needed
the explanation.

Neither blocking comment gate catches this — `doclink` and `commented-code`
check links and commented-out code, not comment *attachment*.
