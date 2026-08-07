---
id: RR-2KZEXF
type: review-response
title: 'permission: values are never validated against acl.yaml'
finding: 'validateNavEntry checks list:/kanban:/action: references against the config but accepts any non-empty string as a permission:. A typo like admin:raed yields an entry permanently invisible to everyone, with no error and no warning — and since the failure mode is a silently missing menu item, nobody notices until a user complains.'
severity: significant
reason: |-
    Real, but not fixable within this ticket's boundary. internal/dataentryconfig does not see acl.yaml at all — the same reason command permissions (config.go, viewCommandPermissionWarnings) and document permissions are equally unvalidated today. Fixing it means plumbing the policy into config validation, which is a change to the validation architecture affecting three features, not one. Doing it here would be scope creep into shared machinery on a UX ticket.

    Mitigated where it costs nothing: the docs now name this explicitly, tell the reader the failure mode (an entry nobody can see, no error), and point at the roles' permissions: list as the first thing to check when a menu item has vanished. That converts a silent puzzle into a documented gotcha.

    Worth its own ticket covering all three consumers together — a warning listing unknown permission names, in the shape of the existing viewCommandPermissionWarnings precedent.
status: deferred
---
