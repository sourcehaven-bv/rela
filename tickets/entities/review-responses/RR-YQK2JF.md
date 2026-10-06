---
id: RR-YQK2JF
type: review-response
title: Relation tab edge is described twice on the wire
finding: Relation/Direction and Links[0] describe the same edge for a relation tab and are computed twice. usePageTabScope reads one, the Create menu the other; they can drift.
severity: minor
resolution: usePageTabScope now reads the tab's single link, so the SPA uses Links as its one source for the edge. Relation/Direction stay on the wire as the tab's scope description.
reason: ""
status: addressed
---
