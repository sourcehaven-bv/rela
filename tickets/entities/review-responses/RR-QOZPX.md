---
id: RR-QOZPX
type: review-response
title: 'Read/write drift: Performable evaluated when: against pre-transition entity, write uses post-transition'
finding: 'The shared evalEdge does not by itself guarantee AC4 parity - the invariant lives in the *entity fed to it. EnforceUpdate passes the post-write `updated` entity (value==to); Performable passed the current `e` (value==from). A when: referencing entity.value (the binding RR-NODYR introduced) therefore evaluates against the target on write but the source on read -> read verdict disagrees with what the write enforces. The parity test used a value-independent when: (count_relations) and so structurally could not catch it.'
severity: critical
resolution: 'Performable now clones e and sets prop=key.to before evalEdge, evaluating each edge exactly as the write path will when the move is made (value==to). resolve.go updated with a WHY comment. Parity test hardened (see RR for finding #3) with a driftMeta whose a->b edge has `when: entity.value == "b"`, proving read==write. Also skips self-loops (finding #2).'
status: addressed
---

## Finding

`evalEdge` is shared, but the invariant depends on WHICH entity it's handed.
Write (`enforce.go` `EnforceUpdate`) passes `updated` (already at `to`); read
(`resolve.go` `Performable`) passed the current `e` (at `from`). A `when:`
referencing `entity.value` (RR-NODYR's binding) thus sees `to` on write and
`from` on read → the read verdict disagrees with the write. The parity test's
only `when:` was value-independent (`count_relations`), so it could never expose
this — the AC4 drift guard was asserting on a fixture incapable of drifting.

## Fix

`Performable` clones `e`, sets `prop = key.to`, and evaluates the edge against
that post-transition entity — "can I move to `to`" is answered as the gates
*will* be evaluated when the move happens. Parity test now includes a
value-dependent `when: entity.value == "b"` edge (driftMeta), which fails loudly
if read/write ever diverge again.
