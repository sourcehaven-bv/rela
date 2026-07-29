---
id: release-test-gate-sandbox-parity
type: automated-measure
title: 'Measure: release test gate installs and verifies bubblewrap like CI'
description: 'Control for BUG-2J30F3. The test job in release.yml now pins ubuntu-26.04 and installs bubblewrap, mirroring ci.yml. It keeps CI''s `bwrap --unshare-all --ro-bind / / /bin/true` verification step so a runner without a working sandbox fails hard and visibly, instead of every confined-command test failing closed with an exec-not-found error that reads like a code defect. Scope is honest: this is a configuration mirror, not a test pin. The two gates remain duplicated and only the release one runs on tag push, so a future divergence is still possible — the structural control (a workflow_call gate shared by both workflows) is deferred.'
kind: ci
location: '.github/workflows/release.yml (test job: runs-on ubuntu-26.04; Install bubblewrap; Verify the sandbox actually works on this runner)'
status: active
---
