---
id: RR-IWZ16L
type: review-response
title: CI never exercised the editor serving path (skip = coverage hole)
finding: The handler subtests for _rela-editor.js/.woff2 serving, CORS, and shadowing all gated on editorBundleBuilt(), which is false in the `go test ./...` CI job (no frontend build). So the new reserved-entry branches + the security-relevant shadowing test never ran in CI.
severity: significant
resolution: Made appEditorSource/appEditorFontSource overridable vars and added withTestEditorAssets(t) which injects fixed fake bytes. The serving/CORS/ETag/shadowing subtests now run unconditionally (no skip). The 'is the real built bundle valid' test (TestAppEditorBundleEmbedded) still skips when unbuilt, which is correct.
status: addressed
---
