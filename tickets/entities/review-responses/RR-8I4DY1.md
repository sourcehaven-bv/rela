---
id: RR-8I4DY1
type: review-response
title: .gitignore check accepts a leading-space pattern
finding: TrimSpace treated '  .rela/' as covering .rela/, but git keeps leading spaces, so the pattern ignores nothing.
severity: minor
resolution: Only trailing spaces and CR are trimmed. TestRun_GitignoreWithLeadingSpaces.
status: addressed
---
