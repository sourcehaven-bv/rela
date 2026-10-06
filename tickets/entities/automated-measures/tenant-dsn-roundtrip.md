---
id: tenant-dsn-roundtrip
type: automated-measure
title: Tenant DSN derivation preserves every base_dsn setting except search_path
description: A round-trip test parses base_dsn and the derived tenant DSN and asserts identical TLS config (verify mode, root CA, fallbacks), pool settings and runtime parameters other than search_path.
kind: test
location: internal/tenant (dsnForSchema round-trip test)
status: proposed
---

Fails when a derivation drops or changes any operator-set connection setting.
