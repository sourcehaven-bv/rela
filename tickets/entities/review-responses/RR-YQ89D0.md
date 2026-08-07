---
id: RR-YQ89D0
type: review-response
title: Cross-boundary contract test validates Go against Go and never opens scales.css
finding: |-
    TestAppCSSSource (internal/dataentry/apps_test.go:141-156) was strengthened to assert --font-size-* name/value pairs, and both its comment and the commit message claim it catches a silent revalue on EITHER side. It does not.

    The test compares appCSSSource(nil) against string literals hardcoded in the test file. Both sides are Go. frontend/src/styles/scales.css is never read. There are now THREE copies of the value: apps_css.go:90, apps_test.go:151, scales.css:62.

    VERIFIED EMPIRICALLY: changed scales.css --font-size-lg to 20px, left Go at 18px, ran `go test ./internal/dataentry/ -run TestAppCSSSource` -> PASSED. The SPA would render 20px while every embedded custom app renders 18px, and the mismatch ships green.

    This is the exact failure mode the test was written to prevent, and it is the entire justification for the test existing.

    The correct pattern already exists twelve lines up in the same file: TestAppTokensCSSInSyncWithFrontend reads frontend/src/styles/tokens.css off disk and byte-compares. Byte comparison will not work for scales.css (it legitimately holds much more than the four steps), but a parse-and-compare of the four declarations will -- and it removes the third copy entirely, because the test then asserts equality between the two REAL sources rather than against its own literals.

    Secondary defects in the same assertion: strings.Contains(css, name+": "+value) is whitespace-sensitive (reformatting apps_css.go to `--font-size-sm:12px` fails with a bogus 'contract violated' message) and substring-based (also matches a hypothetical --x--font-size-sm). Parse the declaration, do not grep it.
severity: critical
resolution: |-
    Fixed in 27bc6ded. Replaced the self-referential assertion with TestFrozenTypographyContractMatchesSPA, which reads BOTH internal/dataentry/apps_css.go (via appCSSSource) and frontend/src/styles/scales.css off disk and compares the four frozen declarations to EACH OTHER — following the existing TestAppTokensCSSInSyncWithFrontend pattern. This removes the third copy: the expected values are no longer hardcoded in the test at all.

    Added a cssDeclValue() helper using an anchored regexp instead of strings.Contains, fixing both secondary defects: it is whitespace-tolerant and cannot match a longer property name.

    Verified all three behaviours empirically: (a) CSS-side drift (scales.css lg 18->20px) now FAILS with 'Go="18px" scales.css="20px"' — this is the exact case that passed green before; (b) Go-side drift (apps_css.go xl 22->24px) FAILS; (c) whitespace reformat (`--font-size-sm:12px`) PASSES, no spurious failure.
status: addressed
---
