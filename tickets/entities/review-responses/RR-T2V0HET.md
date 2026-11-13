---
id: RR-T2V0HET
type: review-response
title: Tenant-free root differed from single-tenant headless root
finding: StaticShell returned a bare 404 at / while single-tenant headless answered 200 with the Link header.
severity: minor
resolution: StaticShell in headless serves withSpecLink(headlessRoot()).
status: addressed
---
