---
id: DOCS-EBV2HD
type: docs-checklist
title: 'Docs: Audit history:read-redacted reveals'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`audit.OpHistoryReveal` carries the doc comment that does the real work here.
Following the house style of its neighbours (`OpACLBypass`, `OpACLBypassRead`),
it records not just what the op means but the four decisions a reader would
otherwise re-litigate:

- **why a NEW op** rather than a flag on an existing one — folding a read into
an existing op silently changes what every stored query for that op means;
- **why not `acl-bypass-read`**, despite the family resemblance — no
`bypass_acl` closure is involved, and reusing it would pollute a forensic query
about elevated automation with ordinary auditor traffic;
- **why this record has a Subject where `acl-bypass-read` deliberately has
none** — a bypass closure's read set is unbounded (one `admin.list_entities` can
walk the graph); a reveal is one entity at one version;
- **why only the reveal arm is recorded** — a row that appears for every
history read buries the privileged ones it exists to surface.

`revealIsPrivileged` documents the trap that produced RR-KBD2T2: taking the
reveal arm is not the same as having been granted a reveal, because under
NopACL/ReadOnlyACL the permit-all gate answers true to every permission probe.
It also records why the switch is on the ACL implementation rather than a
read-gate query (the gate is the thing that cannot answer, so asking it is the
fail-open mistake being avoided), why both value and pointer forms are matched,
and why its default arm AUDITS where `permitsGatedUIElement`'s default hides —
one decides disclosure, the other decides whether to write evidence, and for
evidence the conservative direction is to record.

`(*App).recordHistoryReveal` documents the one thing its body does not make
obvious: the entity type comes from the stored snapshot, never the
caller-supplied URL segment, because the recorded type is forensic evidence and
sourcing it from the request would let a caller write a type of their choosing
into the audit log.

The call site carries a short comment saying it records the *disclosure*, not
the read, and pointing at the op's doc for the rest.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: introduces no new
architectural pattern — it applies the existing `OpACLBypassRead` shape for
auditing a privileged read to a second call site)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency or
wiring-contract change; `arch-lint` clean. `dataentry` already imported both
`audit` and `principal`)

`docs/acl-security.md` gains a paragraph in the **`history:read-redacted`**
section — the place a reader already goes to understand the permission, so the
audit guarantee sits with the thing it guarantees rather than in a separate
log-format appendix that would go stale.

It states what an operator needs in order to *use* the row: that every reveal is
recorded *under a configured policy* — and explicitly that with no `acl.yaml`
these reads are not recorded, since nothing is redacted and so nothing is
revealed. That sentence exists because its absence is precisely what would make
an operator distrust the row: seeing `history-reveal` on a deployment with no
policy would read as a bug, or worse, as noise to filter out. It further states
that the row names the entity, version and principal, that it deliberately
records neither the revealed values nor the revealed field names (that list
being itself a map of what the policy hides), that ordinary redacted reads are
not recorded and why, and the `op == "history-reveal"` query that isolates them.
That last part matters: an audit row nobody knows how to query for is not much
better than no row.

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag; the
audit log is read as JSONL, not through a command this ticket touches)
- [x] ~~API docs updated~~ (N/A: no HTTP surface change — same routes, same
request and response shapes. The change is a side effect on the server, not a
change to the contract)

## Rationale for N/A

The response body is byte-identical before and after: a reveal still reveals, a
redacted read is still redacted, and no status code, header or field changes.
Nothing an API consumer can observe has moved, which is why the API docs need no
entry.

What changed is observable only to an operator reading the audit log, and that
is exactly where the documentation went. The one deliberate omission is the
audit log's own format reference: `history-reveal` uses the existing `Record`
shape with no new fields, so there is no format change to document — only a new
value in the `op` vocabulary, which is described where the permission is.
