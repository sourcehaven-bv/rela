---
id: BUG-E19G7O1
type: bug
title: idp-sync.lua keeps roles of a deactivated user and calls a missing Pratique route
description: examples/idp-sync.lua ignores the member status and clears asserted roles only on a 404. Pratique deactivates a member with status inactive and keeps its roles, so the user keeps the mapped rela role. The action also calls GET /api/v1/orgs/{org}/members/{user}, which Pratique does not serve.
priority: high
effort: s
why1: The action stores user.roles whenever the operator call returns 200 and clears roles only on a 404. It never reads user.status, so a member Pratique set to inactive keeps its roles. It also calls GET /api/v1/orgs/{org}/members/{user}, which Pratique does not route (405).
why2: The script was written against an assumed per-member endpoint and an assumed hard delete on offboarding. Pratique offboards by status and only exposes the member list and DELETE on the operator API.
why3: The test stub answered whatever path the script requested and always returned an active member, so it confirmed the script's own assumptions instead of Pratique's routes and lifecycle.
why4: Nothing ties the reference action to Pratique's operator-API contract. The routes and membership states live in another repository and were inferred when the action was written, not checked.
why5: 'The reference action had no fail-closed rule for roles: keeping roles was the default and clearing was the exception for one known case, so any unknown state (inactive, pending, missing status) kept access.'
prevention: The stub now serves only Pratique's real operator routes (list + DELETE) via a method-aware mux, and a table test pins that every non-active status, a missing status, an absent subject and a 404 clear the roles. The action keeps roles only for status active.
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Reproduction

GitHub issue #2117 (finding 2 of the security review of PR #2113).

1. Pratique offboards a member softly: it sends `membership.updated` and sets
the membership `status` to `inactive`. The roles stay on the membership.
2. `examples/idp-sync.lua` fetches the member and stores `user.roles` as the
subject's asserted roles without reading `status`.
3. The deactivated user keeps the mapped rela role, visible through
`rela.principals_with_role` and in background jobs that run as the person.

The action also calls `GET /api/v1/orgs/{org}/members/{user}`. Pratique's
operator API (`internal/web/operator_handlers.go` on Pratique `develop`) has
only `GET /api/v1/orgs/{id}/members` (list, with `user_id`, `email`, `roles`,
`status`) and `DELETE /api/v1/orgs/{id}/members/{mid}`. Go's mux answers the GET
with 405, so the action fails every webhook (502, retried) against a real
Pratique and never provisions anyone.

## Fix

Fetch the member list, select the member whose `user_id` matches the subject,
and keep roles only when `status` is `active`. Any other status, or no matching
member, clears the stored roles.

From code review: any non-200 (404 included) is an error, because Pratique
answers 200 with an empty list for an unknown org. A new required secret
`idp_org_id` pins the deployment's org: Pratique sends every org's events to
every endpoint and rela stores roles per subject, so an event for another org is
ignored. For `membership.deleted` a failed fetch clears instead of erroring.
