---
id: RR-GW1DSJ
type: review-response
title: plimsoll load lines exceeded
finding: Manager (42/37 methods), v1.Entity (21/20 fields) and App (92/91 methods) exceeded their plimsoll caps; CI would fail.
severity: significant
resolution: Owning helpers became package functions and CheckOwningEdge a package function (Manager back to 37); Self and Owner grouped in embedded v1.EntityLinks; App.owners became appOwners(a).
status: addressed
---
