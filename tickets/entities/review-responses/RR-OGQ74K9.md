---
id: RR-OGQ74K9
type: review-response
title: Single-step and details-only step layouts untested
finding: No test covers a single-step form's step heading or a step with details but no description.
severity: nit
reason: Both paths reuse the multi-step markup and differ only in a margin rule; a layout margin is not observable in happy-dom.
status: wont-fix
---
