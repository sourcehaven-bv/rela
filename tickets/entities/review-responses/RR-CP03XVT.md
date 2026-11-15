---
id: RR-CP03XVT
type: review-response
title: Keeping the lowest ids drops new matches of a busy rule
finding: With a cap of 100 that kept the lowest ids, a rule with many standing matches missed most new matches (sequential ids sort last) and fired stale notifications for old matches moving into the window.
severity: significant
resolution: The cap no longer selects a window. A rule over the cap (raised to 1000) notifies nothing, the desktop logs a deduplicated warning naming it, and the guide documents it. Rules under the cap behave as before the change.
status: addressed
---
