---
id: RR-7ICXWT
type: review-response
title: Guard evaluation errors other than ErrNilOperand must refuse the button
finding: The plan only says that an ErrNilOperand does not hold. At runtime, any other evaluation error (a bad regex pattern, a non-boolean result, rrule_next on bad input) stops the decision and nothing moves (internal/realtime/session.go:509-513, 556-567). The load check must refuse the button when its own guard, or any earlier guard that a person poster of the entry admits, returns such an error for the preset. Treating that error as 'does not hold' would serve a button that never moves anything. Name this in AC 4 and add a case to TestButtons_Guards.
severity: significant
resolution: AC 4 now refuses the button when its own guard, or an earlier guard that admits a person class of the entry, returns an error other than ErrNilOperand for the preset. TestButtons_Guards gains cases for both.
status: addressed
---
