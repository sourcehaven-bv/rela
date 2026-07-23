---
id: RR-KNEF4K
type: review-response
title: 'Open `params: Record<string,string>` prop turns CommandModal into a generic query-param channel into a shell-exec endpoint'
finding: |-
    The plan generalizes CommandModal's prop from `entityId: string` to `params: Record<string,string>`, spread into URLSearchParams. Today (CommandModal.vue:42-45) the client can express exactly one key, hardcoded:

      const params = new URLSearchParams()
      params.set('entity_id', props.entityId)

    After the change, an arbitrary map is spread into the query string of an endpoint that runs `sh -c`. handleCommandExec reads entity_id, list_id, view_id AND exec_id from that same query string (commands.go:295).

    VERIFIED at commands.go:295-298:

      execID := r.URL.Query().Get("exec_id")
      if execID == "" {
          execID = fmt.Sprintf("cmd-%d", time.Now().UnixNano())
      }

    exec_id is client-supplied and used as the key into a package-level runningCommands sync.Map with no per-user or per-session namespacing. handleCommandCancel keys off the same map.

    FAILURE SCENARIO: a future call site passes exec_id through the params map (entirely plausible — the plan's own deferred item 2 is 'move exec into api/commands.ts', where an exec-id naturally lives). Two surfaces then collide on one key; the `defer runningCommands.Delete(execID)` on whichever finishes first removes the other's registration, and a subsequent cancel targets the wrong process. Because the key is also guessable (cmd-<UnixNano>), widening who can set it worsens an existing weakness.

    MITIGATION (costs nothing, do it): type the prop as a closed discriminated union matching the four backend contexts, not an open map:

      type CommandParams =
        | { entity_id: string }
        | { entity_id: string; view_id: string }
        | { list_id: string }
        | Record<string, never>

    This keeps exec_id structurally unreachable from any call site and makes the four contexts explicit in the type system. An open Record<string,string> is precisely the generic pass-through that internal/dataentry/CLAUDE.md's bridge rule warns against ('a closed method allow-list, never a URL proxy').
severity: significant
resolution: |-
    Addressed in PLAN-CNDJ78 Approach step 3. The modal prop is specified as a closed discriminated union rather than the originally-planned open Record<string,string>:

      type CommandParams =
        | { entity_id: string }
        | { entity_id: string; view_id: string }
        | { list_id: string }
        | Record<string, never>   // global

    This mirrors the four backend contexts exactly and keeps exec_id structurally unreachable from any call site, so the exec_id collision scenario in the finding cannot originate from the modal. The rejected alternative is recorded explicitly in the plan's Alternatives Considered so a future edit does not silently reopen it. Verification is an acceptance criterion: mount CommandModal with {} params and assert the request URL carries no entity_id.
status: addressed
---

## Why this matters more after the ticket than before

Today `CommandModal` is mounted once, by one parent, with one hardcoded param.
The open-map generalization is what converts it into a reusable channel — and
the plan simultaneously adds two more mount sites.

The fix is a type declaration, not a code change. Take it.
