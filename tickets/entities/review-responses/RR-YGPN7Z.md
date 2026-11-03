---
id: RR-YGPN7Z
type: review-response
title: PolicyChannelService duplicates prepare's ACL compile sequence
finding: realtime_service.go:53-69 repeats loadACLPolicy, ValidateACLPolicy, buildACL and buildFieldRedactor, which prepare/resolveACL already do (appbuild.go:2129-2236). Any new compile gate added there will be missing here. Extract one compileDeclarative(cfg, meta, store) helper that both paths use.
severity: significant
resolution: Extracted appbuild.compileDeclarative (validate, buildACL, buildFieldPolicy). Assembly (resolveACLAndRedactor) and PolicyChannelService now both use it.
status: addressed
---
