---
id: RR-IGRH61
type: review-response
title: Audit triggered_by omits the channel type, unlike auto:<type>/<op>
finding: 'The lifecycle sweep writes auto:<type>/<op> (sweep.go autoLabel). The timer label timer:<state>/<name> drops the type, so an audit consumer grepping across types cannot tell timers of two types apart without joining on the key. Plan change: use timer:<type>/<state>/<name>.'
severity: nit
resolution: 'Plan updated: audit triggered_by is timer:<type>/<state>/<name>, matching auto:<type>/<op>; AC 11 asserts timer:breach-response/detected/no-containment.'
status: addressed
---
