---
id: RR-J1DXK3
type: review-response
title: Writes accepted ?world= and silently mutated the default state
finding: |-
    VERIFIED. worldCapablePath was METHOD-BLIND, so `/api/v1/tickets/{id}` was world-capable for every verb:

      PATCH  /api/v1/tickets/TKT-1?world=published -> 200, wrote the DEFAULT state
      DELETE /api/v1/tickets/TKT-1?world=published -> 200, deleted the entity
      POST   /api/v1/tickets?world=published       -> 201, created in the DEFAULT world

    All three reached the handler with a non-default worldHandle bound. handleV1UpdateEntity reads via the world-blind reader and writes through the entitymanager with no world in sight.

    An operator editing 'the published copy' silently edits the draft. Worse for DELETE: a caller who believes they are unpublishing deletes the underlying entity. Both return success, which is what makes them dangerous — the parameter is accepted, so the caller has no reason to doubt it was honored.

    This also directly contradicts design §9.4 ('writes never pass through worlds', a hard rule with its own pinning test) and my own file header calling this 'the read API' — a claim nothing enforced.
severity: critical
resolution: attachWorld now refuses a non-default world on anything but GET/HEAD/OPTIONS with a 422 (`world_read_only`). Pinned by TestAttachWorld_WritesRefuseAWorld across POST/PATCH/PUT/DELETE, which also asserts the same writes WITHOUT ?world= pass through untouched. Documented in the data-entry guide.
status: addressed
---
