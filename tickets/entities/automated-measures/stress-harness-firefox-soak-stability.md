---
id: stress-harness-firefox-soak-stability
type: automated-measure
title: Stress harness completes 30-min Firefox soak without runner-side wedge
description: After fixing BUG-K570 (likely via per-user Firefox process), a 30-minute watcher-pressure run with 4 Firefox users must show canary rate within 10% of target (5/s) for the entire duration with no >2-minute gaps in progress checkpoints. Currently the rate drops to 0.03-0.08/s after ~8 minutes.
kind: test
location: frontend/stress/ + manual run gate
status: proposed
---
