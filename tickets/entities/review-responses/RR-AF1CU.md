---
id: RR-AF1CU
type: review-response
title: filepath.Dir in FSKV.Put returns backslash on Windows; resolve then rejects it
finding: Old FSKV.Put called filepath.Dir(key) to decide whether to MkdirAll. On Windows this returns OS-native separators (backslash), which resolve() rejects. All nested-key writes would fail on Windows.
severity: significant
resolution: Moved parent-directory creation into RootedFS.WriteFile itself. WriteFile now calls r.fs.MkdirAll(filepath.Dir(full), 0o755) before the actual write — full is already an absolute path at that point, so OS-native separators are correct. FSKV.Put simplifies to just fs.WriteFile(key, data, 0o644). No filepath.Dir on keys anywhere.
status: addressed
---
