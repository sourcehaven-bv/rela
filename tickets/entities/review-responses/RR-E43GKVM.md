---
id: RR-E43GKVM
type: review-response
title: Placeholder check accepts spaced placeholders
finding: 'parse.go placeholders() trims spaces, so {{ new.title }} passes the load check, but Interpolate only replaces the exact {{new. form and writes the text literally. Fix: treat a spaced placeholder as unknown.'
severity: minor
resolution: placeholders() no longer trims and knownPlaceholder refuses whitespace; a parse test covers {{ new.title }} as refused.
status: addressed
---
