---
id: RR-EGU7HTA
type: review-response
title: -h exits with code 2
finding: Asking for help is not a usage error.
severity: nit
resolution: flag.ErrHelp returns 0. Pinned by TestEraseTenant_HelpExitsZero.
status: addressed
---
