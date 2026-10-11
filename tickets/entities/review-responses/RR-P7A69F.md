---
id: RR-P7A69F
type: review-response
title: NewValidator panic vs constructor rule undocumented; zero value unusable
finding: NewValidator panics instead of returning error without saying why; Validator{} dies with a bare nil dereference.
severity: minor
resolution: Godoc now says it is a value constructor called inline, not a wiring constructor; type doc says the zero value is not usable.
status: addressed
---
