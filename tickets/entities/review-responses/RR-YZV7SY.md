---
id: RR-YZV7SY
type: review-response
title: '/api/command-cancel/ is ungated: any caller can kill any principal''s running command, including under --read-only'
finding: |-
    handleCommandExec is now gated, but its sibling route handleCommandCancel (internal/dataentry/commands.go:511-543, mounted at command_handler.go:59) consults no ACL whatsoever — no authorizeCommand call, no h.currentACL(). It looks up execID in the package-level runningCommands sync.Map and signals the process.

    VERIFIED BY PoC (temporary test, since removed): under acl.ReadOnlyACL, exec returns 403 (the new guard works) while cancel with an arbitrary execID reaches the map lookup and returns 404 — proving no authorization runs before the lookup. A principal denied STARTING every command can still ABORT one.

    Two compounding factors:

    1. execID is CLIENT-SUPPLIED (commands.go:376-380): `execID := r.URL.Query().Get("exec_id")`, falling back to `fmt.Sprintf("cmd-%d", time.Now().UnixNano())`. The map is keyed only by execID with no principal binding, so there is nothing tying an entry to its owner.
    2. The handler distinguishes unknown (404) from found (200), giving a clean enumeration oracle; the nanosecond-timestamp fallback is enumerable and a client-chosen id is outright guessable.

    FAILURE SCENARIO: Bob holds no command permissions, so every exec 403s. He POSTs /api/command-cancel/<guessed-id> in a loop. On a hit, Alice's nightly-export takes SIGINT then an unconditional Kill() after 3s (cancelGrace). Aborting a half-run migration or export is frequently more damaging than not starting it.

    SECOND ORDER: because runningCommands.Store(execID, ...) at commands.go:469 accepts a client-chosen key, a caller can pre-register a colliding execID so a legitimate command's registration is overwritten — making the victim's command uncancellable by its actual owner.

    This is pre-existing (the route predates this ticket) but this ticket is what establishes the expectation that command execution is authorized, and it leaves the abort half of the same lifecycle open. Scoping it out silently would be worse than fixing it: the ticket's own docs now claim commands are gated under --read-only.

    RECOMMENDED: bind the entry to its principal at registration (store principal.From(ctx) in runningCommand) and compare on cancel, returning 404 rather than 403 on mismatch so existence is not confirmed. Additionally stop honoring a client-supplied exec_id — mint it server-side and return it on the SSE stream.
severity: significant
resolution: |-
    FIXED. Bound running commands to the principal that started them.

    - runningCommand gained an `owner principal.Principal` field, documented with the reason (the registry is process-global and keyed only by a client-supplied execID).
    - handleCommandExec populates it via principal.From(r.Context()) at registration.
    - handleCommandCancel compares rc.owner against the requesting principal and returns 404 — byte-identical to the unknown-id response — on mismatch. 404 rather than 403 deliberately: a 403 would confirm that a command with that execID is currently running under another principal, turning cancel into an enumeration oracle.

    Pinned by TestCommandCancelOwnerBound: bob cancelling alice's command gets 404, an unknown id also gets 404 (asserted in the same test so the indistinguishability is a pinned property, not an accident), the victim process is asserted un-signaled, and the owner can still cancel their own command (200).

    NOT fixed here, deliberately: the finding also noted that exec_id is client-supplied and could be pre-registered to collide with a legitimate run. Owner-binding neutralizes the cross-principal impact (a colliding entry from another principal cannot be cancelled by them, and cannot cancel theirs). Server-minting the id is a protocol change affecting the SSE contract and the frontend, which is out of scope for this ticket — worth a follow-up if the SSE stream is revisited.
status: addressed
---

## Evidence

`internal/dataentry/commands.go:511-543` — the entire handler; note the absence
of any ACL consultation between the method check and the signal:

```go
func (h *commandHandler) handleCommandCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { ... }

	execID := strings.TrimPrefix(r.URL.Path, "/api/command-cancel/")
	val, ok := runningCommands.Load(execID)
	if !ok {
		http.Error(w, "No running command: "+execID, http.StatusNotFound)
		return
	}
	...
	_ = rc.cmd.Process.Signal(syscall.SIGINT)
	go func() {
		time.Sleep(cancelGrace)
		_ = rc.cmd.Process.Kill()
	}()
```

## PoC output (test written, run, then deleted)

```
exec under read-only   => 403   (new guard works)
cancel under read-only => 404   (reached lookup — no ACL gate)
```

## Fix sketch

```go
type runningCommand struct {
	cmd  *exec.Cmd
	prin principal.Principal // NEW
}

// in handleCommandCancel, after the Load:
if rc.prin != principal.From(r.Context()) {
	http.Error(w, "No running command: "+execID, http.StatusNotFound)
	return
}
```
