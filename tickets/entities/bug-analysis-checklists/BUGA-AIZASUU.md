---
id: BUGA-AIZASUU
type: bug-analysis-checklist
title: 'Analysis: Tenant DSN derivation drops sslmode and other base_dsn settings, downgrading TLS to prefer'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Reproduction

- [x] Bug reproduced locally: probe in the bug body shows `sslmode=verify-full` becoming unverified TLS with plaintext fallback after `dsnForSchema`.
- [x] Minimal reproduction steps documented (base and derived DSN in the bug body).
- [x] Environment/conditions noted: shared-tier tenants derived from `base_dsn` on the postgres build.

## Root Cause

- [x] Immediate cause identified (why1): `dsnForSchema` rebuilt the DSN from a fixed set of fields and dropped the rest.
- [x] Contributing factors found (why2-3): the code rebuilt the DSN to change one setting; the same rebuild was copied to docscapture, backendtest and a listener test helper.
- [x] Systemic cause explored (why4-5): no single owner for DSN edits and no round-trip test of TLS and pool settings.

## Fix Planning

- [x] Fix approach determined: `pgstore.PinSearchPath` and `pgstore.WithDatabase` change only the one setting and keep every other setting of the operator's DSN.
- [x] Regression test planned: `TestDSNHelpers_KeepBaseSettings` compares parsed TLS and pool config before and after; `TestPinSearchPath_RefusesOtherSpellings`, `TestPinSearchPath_OverridesBasePath`.
- [x] Related areas checked for similar issues: docscapture `dsnWithSearchPath`, backendtest `dsnWithSearchPath` and the listener test helper all moved to the helpers.
