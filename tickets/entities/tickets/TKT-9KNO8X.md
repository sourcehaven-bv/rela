---
id: TKT-9KNO8X
type: ticket
title: Resolve the stdio and CLI principal to the person entity id
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Over stdio and the CLI, `principal.User` stays the raw login, so a person there
misses mentions of their user id, and replies to their stdio messages target the
login and never show in the web app.

When `principal_property` is set, re-stamp the stdio principal at startup
through `acl.IdentityResolver`, as `access.Restamp` does for the web app.

This changes authorship, read markers, pile owners and ownership checks for
every CLI write, so it needs its own design and migration notes; agents with job
identities are unaffected.

Follows TKT-6RXAC0.

Also decide the key for a web login that matches no person while person mapping
is on. Today `internal/access/opener.go` keeps the raw login as its key, so a
login spelled like a person id (`USR-7`) shares that person's read markers and
mention rows (RR-0RKY5W). Options: refuse such logins, or prefix unmatched
logins so they cannot collide with an entity id.
