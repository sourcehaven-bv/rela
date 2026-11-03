---
id: RR-WLOUCA
type: review-response
title: The value check runs as a side effect inside an argument list
finding: 'internal/realtimecfg/session.go:368-369 calls checkDefaultIn, which records a problem, inside the argument list of withSession. A reader skimming the call misses that a validation runs there. funlen forced the move out of validator.session (impl notes). Fix: pass raw.DefaultIn and its key to messageInCheck.withSession and call checkDefaultIn at its top. That keeps all default_in logic in message_in.go and adds no line to validator.session.'
severity: nit
resolution: withSession now takes the raw default_in and calls checkDefaultIn at its top with the key it builds; session.go passes raw.DefaultIn, so all default_in logic is in message_in.go.
status: addressed
---
