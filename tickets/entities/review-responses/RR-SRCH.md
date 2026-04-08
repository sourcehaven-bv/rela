---
id: RR-SRCH
type: review-response
title: 'F12: errBodySnippetBytes comment references a phantom constant'
finding: The comment claimed errBodySnippetBytes had to be 'kept in sync with the openai.go snippetBytes constant' and that 'both exist so the package compiles even if files are reorganized'. There was no snippetBytes constant in openai.go (it was deleted in an earlier refactor) and the import-cycle rationale was nonsense — files in the same Go package cannot have an import cycle.
severity: nit
resolution: Comment cleaned up to a single sentence describing what the constant does. The phantom reference and the false language-constraint claim are gone.
status: addressed
---
