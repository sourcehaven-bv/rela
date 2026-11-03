---
id: RR-4K85IL
type: review-response
title: Changefeed is an ungated base source
finding: A later consumer forwarding bus events without a gate would leak per-id existence.
severity: minor
resolution: Bus holds no entity ids, is documented as an ungated base source like store, and arch-lint allows only dataentry to import it.
status: addressed
---
