---
id: RR-SA5FB
type: review-response
title: 'Deferred: cross-platform CI matrix only runs userstate+project, not encryption'
finding: 'cranky-code-reviewer #18: the new matrix job runs internal/userstate/ and internal/project/ on macOS+Windows, but internal/encryption consumes userstate too and isn''t covered cross-platform.'
severity: significant
reason: Extending the matrix command to include ./internal/encryption/... roughly doubles the macOS/Windows runtime. The encryption package is already covered on ubuntu-latest; the matrix is specifically a cross-platform gate for the new userstate primitives. Tracked as a CI tuning follow-up.
status: deferred
---
