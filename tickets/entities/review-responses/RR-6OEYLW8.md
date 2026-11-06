---
id: RR-6OEYLW8
type: review-response
title: Mermaid diagram does not mark final states
finding: Final states get no --> [*] edge.
severity: nit
reason: Final states are listed by name and have no outgoing edge; mermaid.StateDiagram is shared with schema transitions, which do not mark them either.
status: wont-fix
---
