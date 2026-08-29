---
id: RR-L03E7L
type: review-response
title: bleveindex_shared.go's tag did not state its own requirement; noopSQLiteCloser was a third copy
finding: Two related points. (1) The shared bleve file was tagged `!postgres && !memorybackend`, which the comment described as 'the union of the recipes that use it'. It is not -- it is 'everything except two', which coincides with the union today only by accident. Under sqlite+memorybackend the file drops out and produces undefined-symbol noise on top of the redeclaration error. (2) noopSQLiteCloser was declared per-recipe, defended as matching the other builds. But fs and memory duplicate theirs only because their tags are mutually exclusive so sharing was impossible; bleveindex_shared.go is already compiled into both fs and sqlite builds and is exactly where a shared closer belongs. The shared file was created in this PR and then not used for it.
severity: minor
resolution: Tag is now the explicit union `(!postgres && !memorybackend) || sqlite`, with a comment saying why stating the requirement matters -- a tag that does not express its own need is one someone narrows without noticing. Moved the closer into the shared file and deleted the third copy. Verified the widened tag does not leak bleve into the postgres build (the CI criterion), and that the sqlite build still links it.
status: addressed
---
