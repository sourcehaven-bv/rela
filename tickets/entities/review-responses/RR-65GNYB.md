---
id: RR-65GNYB
type: review-response
title: 'The grep promise was half-delivered: the NopACL path stayed a bare store'
finding: 'The godoc claimed the grep ''enumerates every ungated script read path in the tree, in one command'', but appbuild.scriptEntityReader''s NopACL branch still did `return st` — a bare store, invisible to the grep. That branch is reached from luaReadDepsFor, whose own godoc says it serves ''every identity-bearing path: data-entry requests, automations/cascades, scheduled tasks''. So the single largest ungated surface, on exactly the identity-bearing paths, was the one the new grep did not find. dataentry.App.scriptReader had the same bare-store NopACL branch. Two of the four originally converted sites were a test fixture and a throwaway memstore.'
severity: significant
resolution: Wrapped both NopACL branches in visibility.Unrestricted(st). The grep now returns six sites including both NopACL paths, so the godoc claim is true. Behavior is unchanged (pass-through wrapper). The existing TestScriptEntityReader_NoPolicyIsPassThrough asserted identity with the raw store, which the wrapper breaks; rewrote it to assert the BEHAVIOR its own comment described (with no policy, every entity including the otherwise-hidden SEC-1 must be readable) and mutation-verified it catches a DenyReader substitution.
status: addressed
---
