---
id: RR-MS72Z6
type: review-response
title: Store the resolved canonical id, not the span text
finding: For a user candidate, record `ref.ID` from the Resolve answer as the target, not the raw span text. Otherwise a resolver that matches without regard to case (fsstore on a case-insensitive macOS volume) stores `usr-7`, which is not the person's key. The MentionView `text` used for SPA matching should stay the span text. Add a TestPost row with a lower-case span.
severity: minor
resolution: User target stored as resolved ref.ID; Mention.Span keeps the written text for SPA matching; TestPost row with lower-case span.
status: addressed
---
