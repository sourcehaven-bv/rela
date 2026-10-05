---
id: RR-L92DY8
type: review-response
title: Command renders do not tell the renderer which face it got
finding: renderEntityMarkdown writes id and type but not face, so a command renderer cannot tell which face it received.
severity: minor
reason: The command input carries the addressed face's content, which is the security property; naming the face in the frontmatter is a feature change to the command contract and belongs with the Stage 1 resolver.
status: deferred
---
