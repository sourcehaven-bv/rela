---
id: RR-G7IYZRS
type: review-response
title: Focus after a row click has no target
finding: After a post the row usually leaves, so focusRow drops focus to body.
severity: minor
resolution: focusRow and focusHeader emit focusNext; the inbox focuses the next waiting row or the section heading (AC 8).
status: addressed
---
