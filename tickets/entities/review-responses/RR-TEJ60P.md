---
id: RR-TEJ60P
type: review-response
title: EXIF parser never fuzzed directly; GIF bomb and timeout paths untested
finding: FuzzNormalize seeds only PNG/JPEG-magic fragments, so mutation will essentially never synthesise a valid JPEG+EXIF+TIFF-IFD — orientationFromTIFF (the riskiest function) gets zero fuzz coverage. No test for the multi-frame GIF resource bomb, no test for oversized animated GIF rejection, and the goleak test only uses fast inputs (never exercises the timeout-leaves-worker-running path). The 78% coverage floor is met without exercising the actual threat model.
severity: significant
resolution: Added FuzzExifOrientation and FuzzOrientationFromTIFF (seeded with real EXIF segments so mutation explores IFD offset/count/endianness) — the risky parser now has direct fuzz coverage (1.6M+ combined execs clean). Added TestGIFBomb_RejectedWithoutDecode (many-frame bomb rejected cheaply), TestGIFFrameCount unit + malformed/truncation tests, TestGIF_SingleFrameStillReencodes. imgproc coverage rose to 88.1%.
status: addressed
---
