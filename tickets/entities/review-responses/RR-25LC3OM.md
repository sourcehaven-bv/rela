---
id: RR-25LC3OM
type: review-response
title: Configure has no control for primary
finding: The Configure list editor shows no control for a column's primary. It keeps the key because it writes per key.
severity: minor
reason: primary is tri-state (unset follows the default) which a switch cannot show. data-entry.yaml is the documented surface.
status: wont-fix
---
