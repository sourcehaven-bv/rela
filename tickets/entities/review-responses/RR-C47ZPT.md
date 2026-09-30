---
id: RR-C47ZPT
type: review-response
title: EnforceRestore checked only the last edge into the value
finding: The first EnforceRestore admitted a value when any single edge into it was unguarded or held. A principal without approve could restore established when approved->established was unguarded, and an unguarded edge out of an unreachable state admitted its target.
severity: significant
resolution: 'EnforceRestore now computes reachability from the entry value: a path of declared edges with every guard held must reach the value (403 naming the first unheld guard otherwise), and a value no path reaches is 422. Pinned by TestEnforceRestore and TestEnforceRestore_PathNotLastHop.'
status: addressed
---
