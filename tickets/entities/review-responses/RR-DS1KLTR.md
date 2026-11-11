---
id: RR-DS1KLTR
type: review-response
title: Client ceiling bypass through elevated update
finding: A client under client_baselines or scope_grants with deny_write keeps its permissions. The elevated handle skipped AuthorizeWrite (the only place the ceiling's write verbs are enforced) so a read-only app token could make an elevated write through MCP run_action or HTTP _action.
severity: significant
resolution: ScopedUpdates now asks acl.CeilingPermitsVerb(OpUpdate) on the bound request next to the read-only check. It consults only the ceiling and never a role grant. A refusal is a client-ceiling 403 with a denied-write audit row. TestScopedUpdates_ClientCeiling covers deny_write (refused and audited) and a scope grant and an interactive caller (both elevate).
status: addressed
---
