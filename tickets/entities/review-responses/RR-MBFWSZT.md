---
id: RR-MBFWSZT
type: review-response
title: Automation-created edges cannot satisfy an update
finding: Automation edges are written after the update commits, so an update whose own automation links the required edge is refused. The guide did not say this.
severity: minor
resolution: Documented under the not-checked writes in the metamodel guide.
status: addressed
---
