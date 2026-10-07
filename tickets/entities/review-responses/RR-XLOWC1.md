---
id: RR-XLOWC1
type: review-response
title: Describe() claims restricted reads on macOS
finding: 'sandbox_options.go Describe reports ''reads: system directories only'' for sandbox-exec, which does not restrict reads at all.'
severity: minor
resolution: 'Describe() now reports ''reads: unrestricted, unix-socket connects: <list|none>'' on non-Linux backends; Linux keeps ''reads: system directories only'' / ''+ <list>''.'
status: addressed
---
