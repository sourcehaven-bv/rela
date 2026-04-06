---
id: RR-B00A
type: review-response
title: Hue distance calculation must handle circular wrap-around
finding: 'Hue is circular (0° = 360°). Red at 0° and a pinkish-red at 350° should be very close, but naive |h1-h2| gives 350°. The algorithm needs circular distance: min(|h1-h2|, 1-|h1-h2|) in normalized [0,1] space. Without this, colors near red will be poorly matched.'
severity: significant
resolution: 'Implemented circular hue distance: min(|h1-h2|, 1-|h1-h2|) in normalized [0,1] space'
status: addressed
---
