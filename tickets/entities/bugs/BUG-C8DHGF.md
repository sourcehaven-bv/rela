---
id: BUG-C8DHGF
type: bug
title: Data API request without identity gets 500 instead of 401
description: A data API request with no identity is answered 500 instead of 401.
priority: low
status: backlog
---

## Description

Under a Declarative ACL with header (or env) identity, a data API request that
carries no identity is answered 500 `acl_unstamped_principal`
(internal/dataentry/router.go:482) instead of 401. `wirePrincipalResolvers`
chains env and header; when both miss, nothing stamps a principal and the gate
reports an internal error. The request is refused (fail-closed), but a missing
credential is a client error, and a 500 hides it among real faults in logs and
monitoring.

Recorded by the TKT-0M4H0S golden parity corpus for all 98 anonymous cases.

## Fix

Return 401 with a problem body naming the missing identity when no resolver
yields a principal; keep 500 for a genuine stamping fault. Pin with a test per
identity mode; update the corpus anonymous recordings.
