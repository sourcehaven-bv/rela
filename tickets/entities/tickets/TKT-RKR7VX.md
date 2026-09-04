---
id: TKT-RKR7VX
type: ticket
title: 'plimsoll housekeeping: drop two dead directives, fix two stale prose references, document the store/Config exceptions and the #1452 ratchet bumps'
kind: chore
priority: low
effort: xs
tags:
    - tech-debt
    - good-first
status: ready
---

Sub-ticket of [[TKT-N0IKN9]]. One small PR; everything below is verified on
develop (e0187047).

## Dead directives (both under the 40 line; deleting them changes nothing)

- `internal/docs/runtime.go:87` `//plimsoll:max-methods=29` — docRuntime is
at 29 methods.
- `internal/mcp/server.go:214` `//plimsoll:max-methods=25` — mcp.Server is
at 25 methods.

A directive at or below the default reads as "grandfathered offender" to the
next person — a false signal. Remove them; if the ratchet intent matters, say it
in a plain comment ("29 methods; keep shrinking").

## Stale prose (the commentlint `duplication` failure mode)

- `internal/entitymanager/copylist.go:91` claims `Manager` carries a
`//plimsoll:max-methods=40` load line. **There is no directive on Manager** (37
methods / 13 exported). The decision — `CopiesForSource` as a package function —
is right; reword the justification to the true one (Manager is at 37 of 40 and
the headroom is not a budget).
- `internal/dataentry/worldneighbors.go:85` cites `max-methods=104`; the
directive is 87 (app.go:172). Replace with a `[App]` doc link rather than
restating a number that lives elsewhere. (If the configAPI ticket [[TKT-XDJTDC]]
lands first it fixes this one — check.)

## Document two settled exceptions so nobody opens a ticket for them

- **Store backends.** memstore's exported surplus over `store.Store` +
optional capabilities is **0**; sqlitestore's is 1 (`JournalMode`); fsstore's is
3; pgstore's is 7, every one already consumed through a narrow consumer-side
interface, and versioning is ALREADY behind `VersionServiceProvider`
(store.go:1164), contributing one method. There is nothing to ratchet on the
exported line; CLAUDE.md's exception is correct. Note the pgstore free-function
shuffle at pgstore/entity.go:872-875 as the anti-pattern (method moved off the
receiver purely to duck the counter).
- **`dataentryconfig.Config`** (22 fields). A read-only format mirror of
`data-entry.yaml` — never marshalled back, never JSON-encoded wholesale.
Grouping via `yaml:",inline"` works (verified) but costs 131 non-test call sites
for an arbitrary grouping (`Dashboard` under Views or Behaviour?). Leave the
directive; say so in its doc comment. Separately, the `json:",inline"` tags at
theme.go:70 and config.go:1740 are no-ops (`encoding/json` has no inline option)
— drop them.

## Record in the epic (TKT-N0IKN9)

The worlds feature (#1452, e0187047) RAISED directives rather than holding them:
fsstore 81→92 / 33→36, memstore 43→50 / 29→32, sqlitestore 43→53 / 29→32,
pgstore 49→52 / 39→42, app 86→87. The exported bumps are legitimate
(`store.Store` grew); the total-method bumps are the ratchet being defeated by a
large feature PR and are what the fsstore arc now pays back.
