---
id: FEAT-LB6O7P
type: feature
title: Account menu over a proxy-neutral identity contract
summary: A user menu in the SPA showing who you are (from rela's person entity), your org, and links to sign out, manage your account, switch org and administer, supplied by any upstream login proxy through a generic contract.
description: 'rela does not own login. Whoever fronts it (Pratique, oauth2-proxy, Pomerium) owns the session, so rela can only show identity and link to the proxy''s pages. The contract keeps rela ignorant of any one proxy: facts arrive as verified token claims, actions are configured links to pages the proxy owns. Profile data (display name, avatar) stays rela''s, on the person entity.'
priority: medium
status: proposed
---

## Summary

rela has no user menu and no sign-out today. The SPA cannot even ask who the
current user is. This feature adds an account menu that works behind any login
proxy, without rela calling the proxy's API.

## The contract

Each piece of information lives where it belongs.

| What | Source | Why |
|---|---|---|
| User id, email, org id and slug, roles | Verified assertion claims (already read) | Per-user facts, signed on every request |
| Org display name | Optional `org_name` claim | Orgs are the login provider's data |
| Display name, avatar, profile page | rela's person entity | Profile data is app-specific, not auth |
| Sign out, account, switch org, admin | Links in rela config | Same for every user of a deployment |

Rules:

- **Links, never API calls.** Each configured link opens a page the proxy owns.
CSRF and confirmation happen on that page, so rela never handles the proxy's
CSRF token. oauth2-proxy (`/oauth2/sign_out`) and Pomerium already work this
way; Pratique gains a GET confirmation page for `/auth/logout`.
- **All claims optional.** With no proxy (`RELA_DATAENTRY_USER`) or a proxy
that sends only `sub`, the menu shows what rela knows: the user id and the
person entity.
- **Display only.** The menu's data drives no access decision. Authorization
keeps using the verified principal.
- **Org lists stay out of the token.** They grow and go stale; "Switch org"
links to the proxy's picker instead.

## Out of scope

- Calling a proxy's session API from the SPA.
- OIDC `end_session_endpoint` discovery: it ends the IdP login, not the proxy
session that matters here.
- Profile editing beyond linking to the person entity.
