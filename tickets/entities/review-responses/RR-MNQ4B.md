---
id: RR-MNQ4B
type: review-response
title: Phase 1 commit-failure test relies on count-based injection that's fragile
finding: |-
    api_v1_relations_test.go:885-890 resets failFS.count = 0 after fixture pre-write. Fragile assumption that pre-fixture WriteEntity calls take exactly N renames. If WriteEntity grows another rename (metamodel-driven side effect, etc), the wrong call gets failed.

    Fix: switch to path-based injection — fail when newpath contains a specific marker, e.g. CAT-001's relation file path:
    ```go
    type failOnPathFS struct {
        storage.FS
        failPath string
    }
    func (f *failOnPathFS) Rename(old, new string) error {
        if strings.Contains(new, f.failPath) { return errors.New("injected") }
        return f.FS.Rename(old, new)
    }
    ```
severity: nit
resolution: 'Replaced failOnNthRenameFS (count-based) with failOnPathRenameFS (path-marker-based). AC #20 test now uses failPathMarker: ''belongs-to'' so it fails on the first relation file rename regardless of how many entity writes preceded it. Robust to fixture changes.'
status: addressed
---
