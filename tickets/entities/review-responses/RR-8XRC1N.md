---
id: RR-8XRC1N
type: review-response
title: '--run-as refusal list misses other rela-owned system: identities and accepts invisible format characters'
finding: 'ownJobIdentity refuses only system:realtime/scheduler/automation/provisioner and system:webhook:*. rela also stamps system:subscription:<name> (scheduler/subscriptions.go:177), system:mail (mail.SendScriptPrincipal), system:fs-import and system:perf-seed, and --run-as accepts all of them, so a channel post or graph write can carry a subscription''s or the mail sender''s name. principal.ValidateRunAs rejects only unicode.IsControl, so Cf runes pass: --run-as $''system:realtime\u200b'' passes the exact-match refusal and renders as system:realtime in the SPA author field (the CLI shows U+FFFD). Fix: in validateRunAs, require system: names to match a strict charset like ValidIntegrationName (which rules out Cf and look-alike tricks), and refuse the subscription:, mail, fs-import and perf-seed names from one shared list exported next to the principal constants.'
severity: minor
resolution: 'Covered by the registry and charset fixes: system:subscription:*, system:mail, system:fs-import, system:perf-seed and system:gc-sweep are refused, and Cf characters fail the charset. Tests added.'
reason: 'Covered by the registry and charset fixes: system:subscription:*, system:mail, system:fs-import, system:perf-seed and system:gc-sweep are refused, and Cf characters fail the charset. Tests added.'
status: addressed
---
