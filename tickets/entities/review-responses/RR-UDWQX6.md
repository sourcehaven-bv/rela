---
id: RR-UDWQX6
type: review-response
title: Entity-like text decoded on the next pass
finding: '&amp;copy; became &copy; and then ©; &amp;nbsp; emptied the body.'
severity: significant
resolution: An & that starts an entity-like run is escaped in the markdown; table tests cover it.
status: addressed
---
