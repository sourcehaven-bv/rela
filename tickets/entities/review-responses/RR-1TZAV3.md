---
id: RR-1TZAV3
type: review-response
title: Type named out collides with the out file
finding: validInputName refused only the whole name out, not a directory segment, and compared case-sensitively.
severity: minor
resolution: Every segment is refused when it equals out without regard to case; cases added to TestRunWith_InvalidInputName.
status: addressed
---
