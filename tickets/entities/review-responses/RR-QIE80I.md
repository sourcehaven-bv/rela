---
id: RR-QIE80I
type: review-response
title: pushedListPage re-reads config and metamodel
finding: a.Meta()/a.Cfg() were read again after the scope was resolved against an earlier snapshot; breaks capture-state-once.
severity: minor
resolution: listPage captures a.State() once and passes the snapshot to pushedListPage.
status: addressed
---
