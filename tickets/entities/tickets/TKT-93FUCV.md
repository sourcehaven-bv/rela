---
id: TKT-93FUCV
type: ticket
title: Replace command open/reveal launcher with an ACL-gated HTTP download
kind: enhancement
priority: high
effort: m
status: review
---

## Problem

When a `commands:` script emits `::rela::{"type":"file", ...}`, the browser's
**Open** / **Reveal** buttons `POST /api/open-file`, and the server executes an
OS launcher on the machine where `rela-server` runs — `open` / `xdg-open` /
`explorer`, selected by `runtime.GOOS`
(`internal/dataentry/commands.go:513-533`, `handleOpenFile` at `:468-508`).

This is coherent for the desktop/local model. On a remote deployment (observed
on `f25fa238`, `rela-server-postgres` behind oauth2-proxy) it breaks:

- The server is a headless Linux VM with no display, so `xdg-open` has nowhere to open and silently no-ops. The UI shows the command **Completed** with Open/Reveal buttons that do nothing.
- Even if it worked, it would open on the *server*, not in the user's browser — never what a remote user wants.
- It lets an authenticated remote user drive arbitrary server-side process launch (constrained to the project root by `containedProjectPath`, but still).

## Decision: delete the launcher, don't abstract over it

An earlier draft of this ticket proposed a strategy pattern (desktop keeps
open/reveal, daemon gets download) behind a self-describing affordance tuple.
**Rejected as overkill** — there are no real users of the desktop launcher
today, so the second implementation would be vestigial and the abstraction would
exist to serve it.

Download becomes the only path, on every deployment including `rela-desktop`.
One code path, no `runtime.GOOS` switch, no mode discriminator to misconfigure.
The `cmd/rela-desktop` **binary stays** — only the launcher goes.

Auto-download on file completion was also considered and **rejected**: browsers
block or prompt on multiple programmatic downloads, so a script emitting several
files would reproduce exactly the silent-no-op failure this ticket exists to
fix. An explicit button always works.

## Scope

**In scope** (line numbers per develop @ dd0fe649)

- Delete `openFileCommand` (`commands.go:634`) and its GOOS switch. Confirmed a bare `exec.Command`, not routed through `internal/cmdexec`.
- Delete `handleOpenFile` (`commands.go:589`) and the `/api/open-file` mount (`command_handler.go:60`).
- Delete `handleOpenURL` (`commands.go:659`) / `openURLCommand` (`commands.go:692`) / `validateOpenURL` (`commands.go:775`) — already unreachable; `/api/open-url` is never mounted, and `CommandModal.vue`'s SSE switch has no `open` case, so `type:"open"` is dropped client-side too. Removing it now prevents someone wiring it up later without the gating.
- Add a token-scoped download route and per-run action table.
- `CommandModal.vue`: one **Download** button per file item; delete `openFile` / `revealFile` and the hardcoded Open/Reveal button pair.

**Not in scope**

- `containedProjectPath` (`commands.go:725`) **stays** — `resolveConflictPath` (`api_v1.go`) still uses it, and minting a token still needs it for the mint-time containment check.
- Deleting the `cmd/rela-desktop` binary. Separate decision if ever wanted.
- The command ACL gate — **landed** in #1180; see Dependency (resolved).

## Approach

> **Revised after #1180 landed the command ACL gate.** The original approach
> assumed a per-*entity* read gate at `handleCommandExec` and had the download
> "re-authorize read on the recorded entity." That gate does not exist —
> `authorizeCommand` (`commands.go:84`) is a per-*command* permission check
> (commands have no entity Subject), so the download re-checks the **command**,
> not an entity. This simplifies the design; see below.

1. **Per-run action table.** When the SSE emit loop (`commands.go:~446` onward, where each parsed `file` message is written) hits a `file` message, containment-check the path once and register an entry under an opaque token, holding the resolved absolute path plus the **`CommandConfig` the run executed** (in scope as `cmd` at the emit point — it carries `cmd.Permission` and `cmd.Context`). Entries live in memory, evicted on run completion plus a TTL.
2. **Payload carries the token, not the path.** The `file` SSE payload ships the token in place of the raw server path.
3. **`GET /api/command-file/{token}`** looks up the entry, **re-runs `authorizeCommand(ctx, currentACL, storedCmd)`** against the *live* ACL, and on success streams the bytes through the shared hardened-download header helper (`export.go:266-270` — `X-Content-Type-Options: nosniff`, sandbox CSP, `Cache-Control: no-store`, sanitized `Content-Disposition: attachment`). Reuse that helper; do not hand-roll the headers.

**Why re-check on every download (not just mint-time validation).** The token is
a **capability**. Re-running `authorizeCommand` at download time is the whole
point: it re-evaluates the live ACL, so a leaked token stops working after a
`--read-only` restart or a policy change that revokes the command's permission —
exactly the bimodal policy the gate enforces (DEC-EIHQSU: no `acl.yaml` ⇒ runs
as before; policy present ⇒ `permission` must be set and held; `--read-only`
denies all). Mint-time-only validation would let a token outlive the grant that
produced it. Run-scoped TTL on top bounds the window further.

**Why token-scoped rather than path-addressed.** A route that accepts a
caller-supplied path reproduces the original arbitrary-launch hole in a nicer
wrapper. This mirrors the principle the transform-export work established
(CLAUDE.md, "View export & transforms"): *"Export is downstream of an
already-authorized [action], never a new capability"* — a request may reference
a token minted by an authorized run, never name a path. Command output has no
store key (scripts write arbitrary paths), so containment stays load-bearing at
mint time; binding the token to the command's authorization is what replaces the
missing store key.

## Dependency — RESOLVED

The command ACL gate landed on develop in **#1180** (`feat(dataentry):
per-command ACL guard for command execution`, TKT-MJ02AO). `authorizeCommand`
(`commands.go:84`) is the single decision point, re-consulted at exec time. The
download re-check calls the **same function** with the stored `CommandConfig`,
so it cannot drift from the exec boundary. No longer blocked.

## Acceptance criteria

1. `/api/open-file` returns 404 — the route is not registered, on any build.
2. `grep -r "xdg-open\|openFileCommand\|handleOpenURL" internal/` returns nothing.
3. A `context: entity` command emitting a `file` message renders exactly one **Download** button; clicking it downloads the file in the browser with `Content-Disposition: attachment`.
4. A caller who may not run command C (permission not held, or `--read-only`) cannot download a file C produced, even holding a valid token — `authorizeCommand` is re-run per download against the live ACL.
5. A token is rejected after its run completes + TTL elapses.
6. The raw server-side filesystem path no longer appears in the `file` SSE payload.

## Test plan

- **Unit** — token mint/lookup/expiry; containment rejection at mint time; the download handler's 404-on-unknown-token and 403/404-on-unauthorized paths (matching the attachment handler's convention that hidden and nonexistent are indistinguishable).
- **Integration** — full command run → SSE `file` event carries a token and no path → `GET /api/command-file/{token}` streams expected bytes with expected headers.
- **Authorization** — the AC-4 case as an explicit test: authorized run mints a token, then re-download under a revoked/`--read-only` ACL is refused (re-check per download, not mint-time-only).
- **Manual** — verify on the actual headless deployment, since the reported symptom (silent no-op) is only observable there.

## Risk

- **Behavior change:** the server-side path becomes invisible in the UI on all deployments. Correct — a server path is meaningless to a remote user — but worth naming rather than discovering.
- **Desktop regression:** `rela-desktop` users lose open/reveal and get a browser download instead. Accepted explicitly: no real users today.
- **Concurrency with the ACL fix:** touches adjacent code in `commands.go`. Sequence after the ACL change lands to avoid a conflict, or coordinate on the emit loop.
