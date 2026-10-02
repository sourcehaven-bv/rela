---
id: RR-YDX4G8
type: review-response
title: Commands would open the store; read sources inconsistent
finding: readServices opens the full store (sqlite exclusive lock); validate reads disk only; acl.yaml read from disk.
severity: significant
resolution: Commands use a validate-style light setup with disk reads only; refuse if the file exists only in DB-backed config; AC7 added.
status: addressed
---
