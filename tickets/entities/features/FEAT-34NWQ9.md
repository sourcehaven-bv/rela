---
id: FEAT-34NWQ9
type: feature
title: Sequence diagrams of traced request flows
summary: Instrumented build that records rela's own calls per request and draws Mermaid sequence diagrams
description: tools/seqtrace rewrites a copy of the source through a go build -overlay to log every call with argument and result summaries, links parents across goroutines, and draws one numbered sequence diagram per request. just seqtrace-demo traces a postgres + ACL deployment from a copy of the checkout.
priority: low
status: implemented
---
