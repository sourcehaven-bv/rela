---
id: RR-GZ6ZW4
type: review-response
title: Tall full-page capture rendered blank below the viewport
finding: Viewport height is emulated at 1600 but maxFullHeight allows a clip up to 4000px. page.CaptureScreenshot().WithClip(...) defaults CaptureBeyondViewport to false, so a clip taller than the emulated viewport renders the below-fold region blank — exactly the 1600<H≤4000 case the cap was designed to permit.
severity: significant
resolution: Added .WithCaptureBeyondViewport(true) on the full-page capture path so the whole clipped height renders.
status: addressed
---
