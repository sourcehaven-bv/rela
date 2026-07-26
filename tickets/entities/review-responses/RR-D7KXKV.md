---
id: RR-D7KXKV
type: review-response
title: Mistyped get_relations filter silently became a full ungated scan
finding: 'elevatedRelationQuery used RawGetString(...).(lua.LString) with the ok result discarded into a silent skip, so a non-string value dropped the filter entirely. Verified: admin.get_relations({from = 12345}) returned ALL relations rather than zero. Triggered by `from = entity.id` where the id came back as a number, or by a typo''d key. This mirrors the gated luaGetRelations so it is not a regression, but the blast radius differs: on the gated path an over-broad result is still peer-gated by the reader, whereas on the elevated path nothing gates it — the script receives a whole-graph edge dump and treats it as filtered. Also internally inconsistent, since the two list methods validate their arguments and this one did not.'
severity: minor
resolution: 'elevatedRelationQuery now returns an error and rejects any non-string, non-nil from/type/to, naming the offending option and its actual type. Absent keys and an absent options table still mean "no constraint". Added TestElevatedRead_GetRelationsRejectsNonStringFilter (3 subtests: number, boolean, table) and TestElevatedRead_GetRelationsAcceptsAbsentFilters so the rejection cannot swing too far; mutation-verified. The gated luaGetRelations was left as-is — changing it is a behavior change on a widely-used binding and belongs in its own ticket.'
status: addressed
---

Raised by the cranky-code-reviewer against commit `31813351`.

Fixed on the elevated path only. The identical shape in the gated
`luaGetRelations` (`runtime.go:938-950`) is left alone deliberately: tightening
it would be a breaking behavior change on a binding scripts already use, and the
risk there is bounded by the peer-gating the reader applies afterwards. Worth a
follow-up ticket, not a drive-by change inside this one.
