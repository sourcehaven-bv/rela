---
id: RR-DNSDW4
type: review-response
title: An optional Due lets a move zero the hint, which means the timer never fires
finding: 'AC 6 specifies that a move by a step without Due clears NextDueAt. Zero is read as ''no timer'', not ''earlier'', so it breaks the never-late invariant. The plan says every production step sets Due anyway. Plan change: make Due required in PrepareSessionStep (ErrInvalidMessage when nil) and drop the no-Due branch from testSessionDueOnAppend. That is one less state to reason about.'
severity: minor
resolution: 'Plan updated: Due is required on every SessionStep (PrepareSessionStep refuses nil with ErrInvalidMessage); slice-1 conformance steps gain a zero Due; the no-Due branch is gone.'
status: addressed
---
