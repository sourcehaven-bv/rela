---
id: app-lock-contention-regression-test
type: automated-measure
title: 'Regression test: page-load latency under file-watcher pressure'
description: Spins up an App, fires file watcher events repeatedly while concurrently calling /api/v1/_schema, and asserts every request completes in <100ms. Reproduces BUG-FMS1 — fails on the current writer-priority RWMutex design, passes after the atomic-snapshot refactor.
kind: test
location: internal/dataentry/lock_contention_test.go
status: proposed
---
