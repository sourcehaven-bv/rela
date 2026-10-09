---
id: RR-MCEU1Y
type: review-response
title: 'Design: run_as is an unmarked elevation'
finding: Any saver can start a script as run_as, unaudited as elevation.
severity: significant
resolution: run_as validated at load (no surrounding space or control chars); enqueuing principal logged; docs call run_as an elevation and entity values untrusted.
status: addressed
---
