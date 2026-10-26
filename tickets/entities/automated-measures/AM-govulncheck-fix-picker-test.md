---
id: AM-govulncheck-fix-picker-test
type: automated-measure
title: 'Test: govulncheck fix picker chooses the fix for the version in use'
description: 'scripts/govulncheck-fixable-test.sh feeds govulncheck-fixable.sh fixture findings (multi-branch stdlib OSV; competing module fixes; uncalled; ignored; truncated JSON) and runs in the CI Demos job. Catches the BUG-NVNPD8 class: a wrong go get target that only shows up in the scheduled security scan.'
kind: test
location: scripts/govulncheck-fixable-test.sh (ci.yml Demos job)
status: active
---
