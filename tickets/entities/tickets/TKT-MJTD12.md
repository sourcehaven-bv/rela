---
id: TKT-MJTD12
type: ticket
title: 'Account menu v1: GET /_me, account: links config, org_name claim, sidebar account menu'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

First slice of FEAT-LB6O7P.

### Server

- **Read `org_name`** from the verified assertion onto the principal, next to
`org_id` and `org_slug`. Optional; absent means "show the slug".
- **`GET /api/v1/_me`** returns, for the request principal:
  - `user`: the principal id;
  - `email`, `org` (`{id, slug, name}`), `roles`, when the assertion carries them;
  - `person`: `{type, id, title, avatar?}` when the principal resolves to a
readable person entity. Read through the visibility wrapper like any other read;
an unreadable entity is omitted, not redacted in place;
  - `links`: the configured account links the principal may see.
- **`account:` config** in `data-entry.yaml`:

  ```yaml
  account:
    sign_out: /pratique/auth/logout
    account: /pratique/admin/account
    switch_org: /pratique/auth/select-tenant
    admin: { url: /pratique/admin/, role: org-admin }
    avatar_property: photo   # optional, on the person type
  ```

Each link is a URL to a page the proxy owns. Validate at load: relative path or
`https` URL only (no `javascript:`), known keys only. `admin.role` is a UX
filter on the asserted roles, not a security boundary. All keys optional.

### SPA

Composed from library parts, following rela-components' recipe (no
`RlAccountMenu`; the row set depends on the deployment):

- **Placement:** `RlSidebar`'s `#footer` slot, beside `RlThemeToggle`, which
stays there. The theme is a browser setting, not an account one.
- **Menu:** `RlMenu align="start" placement="top"`. `RlMenuSection` (being added
to the library) holds name, email and org as non-interactive text.
- **Rows:** `RlMenuItem` with `href` for Profile (the person entity, when
present), Account, Switch org, Admin and Settings; `RlMenuSeparator`; then Sign
out in the default tone, not `danger`. No row calls a proxy API.
- **Trigger:** a small rela component spreading the menu's `attrs`: `RlAvatar`
(initials when there is no photo) with name and org. At rail width (`@container
rl-sidebar (max-width: 120px)`) the text is visually hidden, not removed, so the
button keeps its accessible name; `RlAvatar` is `decorative` only in the wide
case.
- Icons: `user`, `settings`, `organization`, `shield`, `sign-out`.
- Without any configured links the menu still shows identity and Profile.

### Tests

- `_me` with a full assertion, with `sub` only, with `RELA_DATAENTRY_USER`,
and with an unreadable person entity (omitted).
- Config validation: rejected schemes, unknown keys.
- e2e: the menu renders the person title and a configured sign-out link.

### Docs

Guide sections on the contract for proxy operators: the claims rela reads, the
link keys, and the rule that every link must be a page (with Pratique,
oauth2-proxy and Pomerium examples).

### Depends on

- Pratique: GET confirmation page for `/auth/logout` and the optional
`org_name` claim (separate PR in the Pratique repo).
- rela-components: `RlMenuSection` and the account-menu recipe.
