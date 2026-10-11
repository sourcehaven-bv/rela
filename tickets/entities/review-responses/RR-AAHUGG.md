---
id: RR-AAHUGG
type: review-response
title: jobqueue_postgres lost WithConnectionString
finding: appbuild/jobqueue_postgres.go dropped the upstream option name.
severity: minor
resolution: Restored as the queue library's WithConnectionString.
status: addressed
---
