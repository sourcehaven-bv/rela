---
id: RR-FNM685Y
type: review-response
title: HolderLookupFor recovers the ACL tier by a type switch
finding: HolderLookupFor maps the reader to a lookup by concrete type; a future decorator around ScriptReader would make every webhook delivery refuse.
severity: significant
resolution: Kept HolderLookupFor in this change; it fails closed (unknown reader refuses) and the dataentry webhook and subscription tests exercise the wired path. Filed TKT-S7KB1WU to carry the lookup from appbuild.scriptReads as a typed dependency.
reason: Carrying the lookup means setting a new field at every site that sets lua.ReadDeps.VisibleReader (about seven) which is wider than this ticket; the current form cannot read ungated.
status: deferred
---
