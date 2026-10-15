---
id: PLAN-TY1OLC
type: planning-checklist
title: 'Planning: Named dashboards: dashboards: in data-entry.yaml and dashboard: <name>'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a top-level `dashboards:` map of named dashboards with the shape of
`dashboard:`; `dashboard: <name>` wherever `dashboard: true` is accepted
(navigation entries, page tabs, command scopes, space `home:`);
`/api/v1/_dashboard?name=`; SPA routes `/dashboard/<name>` and
`/s/<space>/dashboard/<name>` with sidebar highlighting; validation; the
Configure space (edit an existing named dashboard, pick one as a navigation
target); `_config`, OpenAPI, ACL audit and query planner mirrors; docs.

Out of scope: new card kinds (TKT-AIEGHU); creating or renaming named dashboards
in the Configure space; a `rela.url` Lua helper for named dashboards.

**Acceptance Criteria:**
1. A config with only `dashboard:` behaves as before. Test: existing Go, Vitest and e2e dashboard tests pass unchanged except for type changes.
2. `dashboard: crm` on a navigation entry, page tab and space home resolves to `/dashboard/crm`. Test: sidebar/pages/spaces handler tests; Vitest for the route and nav highlighting; e2e.
3. `/_dashboard?name=crm` returns the crm dashboard with permission-filtered cards; unknown name returns 404 `dashboard_not_found`. Test: handler tests incl. ACL principal.
4. Unknown name in any reference, an invalid name, or `dashboards.default` fails config load naming the location. Test: validate tests.

Decisions (approved by the user): `dashboard:` and `dashboards:` coexist;
`default` is reserved as a `dashboards:` key and is an alias for `true` in a
reference; `dashboard: true` without a `dashboard:` block keeps rendering the
empty dashboard.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extends an existing config shape)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `ActionConfirm` (config.go) is a bool-or-string YAML
union with the YAML 1.1 yes/no guard; `navTargetErrors` is shared by nav
entries, space homes and page tabs (via `PageTab.Destination`), so one check
covers three surfaces. TKT-GNKR5H sketched `home: dashboard_projects`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `dataentryconfig.DashboardRef` (string type): YAML `true` -> `default`, `false` -> unset, string -> name; JSON serves `true` for the default and the name otherwise, so old clients keep working. Replaces the `bool` on `NavigationEntry`, `PageTab`, `CommandScope`.
- `Config.Dashboards map[string]DashboardConfig`; `Config.DashboardByName`, `Config.AllDashboards`.
- Validation: `dashboardRefError` in `navTargetErrors` and in command `available_on`; reserved/invalid names; card checks run per dashboard with a `dashboards[<name>]:` prefix.
- Handler: `?name=`; unknown -> 404 `dashboard_not_found`; same card permission filter.
- Sidebar href `/dashboard/<name>`; page tab target = name; command scope matches page_type `dashboard` with qualifier = name.
- SPA: route `/dashboard/:name` (space alias automatic), `DashboardView` takes a `name` prop and fetches `/_dashboard?name=`; PageView passes `name`.
- Configure: dashboard screen gets `?name=`; navigation editor lists named dashboards as targets; allowlist mirrors editable keys under `dashboards.*`.

Alternatives: a separate `dashboard_name:` key (rejected: two keys for one
destination, and the user asked for `dashboard: <name>`); turning `dashboard:`
into a map (rejected: breaks every existing config).

**Files to modify:**
internal/dataentryconfig/{dashboard.go,config.go,page.go,space.go,naventities.go,validate.go},
internal/dataentry/{dashboard_handler.go,views_handler.go,commands.go,api_v1.go},
internal/apiwire/v1/responses.go, internal/openapi/schemas.go,
internal/configedit/allowlist.go, internal/cli/acl.go,
internal/queryplan/queryplan.go, frontend router/views/api/types/configure, e2e
test, docs/data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config (validated at load); the `name`
query parameter is only a map lookup key.

**Security-Sensitive Operations:** card `permission:` filtering is reused
unchanged per dashboard; card queries still run through the ACL-scoped search.
Dashboard names are config, not data, so a distinct 404 code is fine (root
CLAUDE.md "The configuration is not a secret").

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria. Go: YAML/JSON round trip of
`DashboardRef`, validation, handler, sidebar, commands. Vitest: DashboardView
with a name, nav highlighting. E2E: navigate to a named dashboard from the
sidebar, inside a space.

**Edge Cases:** `dashboard: false`; `dashboard: ""`; unquoted `yes`; `dashboard:
default`; `?name=default`; `?name=` empty; named dashboard with no cards; all
cards filtered.

**Negative Tests:** unknown name in nav/tab/home/command; `dashboards.default`;
name with uppercase; `?name=nope` -> 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** the bool -> string type change touches many tests and the wire shape;
JSON keeps `true` for the default, pinned by the golden corpus. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md (dashboard section, navigation,
pages, spaces, commands).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (plan approved by the user directly)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
