---
id: RR-607JNU
type: review-response
title: SetWorlds panics on a caller-controlled condition
finding: defaultWorldScope could return an unset scope from a WorldLookup.
severity: minor
resolution: defaultWorldScope falls back to the default handle's scope when the lookup returns an unset scope, so the panic is unreachable.
status: addressed
---
