---
id: RR-YLYWUZ
type: review-response
title: Animated-GIF check via gif.DecodeAll is a memory bomb bypassing all DoS controls
finding: 'isAnimatedGIF calls gif.DecodeAll(data) BEFORE decodeBounded, so it runs outside the semaphore AND the timeout. DecodeConfig only checked the logical-screen dims (frame 0), not frames × screen. A ~0.5 MiB, 300-frame 1000×1000 GIF passes the 64 Mpx cap but DecodeAll materialises all frames: measured 3.41s and large allocation before the ErrAnimated verdict; the reviewer measured ~7.6 GiB with 500 frames at 4000×4000. N concurrent uploads OOM the host with no gating — the exact DoS the package claims to bound. The doc comment ''without a full decode'' is false. Remotely triggerable.'
severity: critical
resolution: 'Replaced gif.DecodeAll with a new allocation-free GIF header walker (internal/imgproc/gif.go, gifFrameCount) that counts image-descriptor blocks without decoding any pixels. Normalize now rejects animated GIFs from the block structure. Verified: the 300-frame 1000x1000 bomb (~0.5 MiB) that previously took 3.41s + large allocation now rejects in ~350us. Regression test TestGIFBomb_RejectedWithoutDecode asserts <100ms. The misleading ''without a full decode'' comment is now true.'
status: addressed
---
