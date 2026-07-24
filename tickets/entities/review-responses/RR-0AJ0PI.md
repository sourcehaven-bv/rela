---
id: RR-0AJ0PI
type: review-response
title: Fail-loud-on-screenshot-without-browser untested; capturer tests host-dependent
finding: docscapture.New() succeeds or fails depending on whether Chrome is on PATH, so NewCapturer's coverage differed by environment and the tests silently exercised different branches. The core 'no graceful degradation' contract (screenshot{} manual + no browser -> hard error) was not tested at all.
severity: significant
resolution: Introduced an injectable `newCapturer` package-var seam (defaults to the build-tagged NewCapturer). Added stubOKCapturer/stubNoCapturer test helpers so all capturer-touching tests are host-independent, plus TestBuild_ScreenshotNoBrowser_FailsLoud asserting the CapturerErr path errors with the no-browser reason.
status: addressed
---
