---
id: RR-39HAJUL
type: review-response
title: Other answer type stays in the picker when the state has no decisions
finding: buttonTypesOf only read the Other choice at decisionSeq, which is undefined when the state has no decisions, while the question still shows Other.
severity: minor
resolution: buttonTypesOf reads the Other key of every choices entry in a session (only the waiting question has choices there) and no longer takes decisionSeq. Unit test for a state with no decisions.
status: addressed
---
