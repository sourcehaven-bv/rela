---
id: RR-5C37DB
type: review-response
title: Tracer node read errors vanish silently
finding: A header read error in node() makes the node disappear without a log.
severity: nit
reason: Matches the previous GetEntity behaviour; the tracer is a pure reader with no logger, and the visibility decorator already logs its failed reads.
status: wont-fix
---
