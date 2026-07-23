---
id: RR-PG8HR2
type: review-response
title: /api/open-file is ungated and spawns the OS file handler; unaffected by --read-only
finding: |-
    handleOpenFile (internal/dataentry/commands.go:546-589, mounted at command_handler.go:60) performs no ACL check. Under acl.ReadOnlyACL a principal denied every command can still make the server spawn `open` / `xdg-open` / `explorer` on any file inside the project root.

    What it DOES validate is solid, and worth stating so this is not over-rated: containedProjectPath (commands.go:685-733) confines the path to the project root, handling traversal, absolute paths, NUL bytes and symlink escape; the path is passed as a discrete exec.Command argv element by openFileCommand (commands.go:593-616), never through a shell, so unlike handleCommandExec there is no `sh -c` and no argument injection. It is POST-only and sits behind requireSameOrigin + requireLocalHost (router.go:105-109), so it is not cross-origin reachable.

    Residual risk: it is still a process spawn dispatched by MIME association. xdg-open on a .desktop file, or `cmd /c start` on Windows, launches by handler association rather than merely displaying content. Severity depends entirely on deployment shape — on the intended local-desktop deployment this is benign (the user could open the file themselves); on a shared or remote rela-server it is an ACL-unaware process spawn on the server host.

    Rated MINOR rather than significant because: (a) it is pre-existing and untouched by this ticket, (b) path containment is genuinely enforced, (c) it cannot execute arbitrary attacker-supplied content — the file must already be in the project, (d) the primary deployment is local. It is filed because this ticket establishes 'command surfaces are ACL-gated' as an expectation, and this route sits in the same registerCommandRoutes block while not meeting it.

    SUGGESTED: deny under ReadOnlyACL at minimum (cheap, consistent with the exec guard, and matches what an operator running --read-only expects). A named permission under Declarative would be the fuller treatment but is arguably its own ticket.
severity: minor
reason: |-
    Deferred as MINOR severity, which the workflow permits to remain open. Justification for not fixing in this PR:

    1. UNCHANGED BY THIS TICKET. handleOpenFile predates the change and this PR does not touch it, widen it, or make it more reachable. Unlike RR-YZV7SY (cancel), it is not the other half of a lifecycle this ticket authorizes — it is a separate launcher route that happens to be registered in the same block.
    2. GENUINELY DEFENDED WHERE IT MATTERS. containedProjectPath enforces project-root containment against traversal, absolute paths, NUL bytes and symlink escape; openFileCommand passes the path as a discrete argv element with no shell; POST-only; behind requireSameOrigin + requireLocalHost. It cannot execute attacker-supplied content — the file must already exist inside the project.
    3. THE FIX NEEDS A DESIGN DECISION THIS TICKET DOES NOT FORCE. Should open-file have its own named permission, ride on the command permission of whatever produced the file, or simply be denied under ReadOnlyACL? Each has different implications for the SSE `file` message flow, where auto_open triggers this route programmatically after a command completes — gating it naively would break the auto-open affordance for legitimately-authorized commands. Deciding that inside a PR about command authorization would be scope creep on a security-sensitive path.
    4. RISK IS DEPLOYMENT-SHAPED. On the primary local-desktop deployment the user could open the file themselves; the exposure is meaningful only on a shared/remote rela-server, which is not the documented deployment model today.

    FOLLOW-UP: worth a ticket covering the launcher routes (/api/open-file and the unmounted handleOpenURL) as a group, including the ReadOnlyACL denial, so the auto_open interaction is designed once rather than patched twice.
status: deferred
---

Deliberately not bundled into this PR: fixing it properly means deciding whether
`open-file` deserves its own permission name or should ride on the command
permission of whatever produced the file, and that decision is not forced by
this ticket.

Read-only denial alone would be a two-line change and could reasonably be folded
in if you want the `--read-only` story airtight.
