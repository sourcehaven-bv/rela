---
id: RR-2ITK58
type: review-response
title: ErrTimeout maps to 4xx rejection; image.quality unvalidated at load
finding: '(a) command.go maps ErrTimeout to Rejectedf (4xx), telling the user their file is bad when the truth is the server gave up — a 5xx internal fault is more honest. (b) metamodel image.quality is documented 1..100 but never validated at load; jpeg.Encode clamps internally so no panic, but a fat-fingered quality: 850 is silently clamped with no load-time diagnostic, unlike every other constrained field in this loader.'
severity: minor
resolution: (a) ErrTimeout now maps to a plain internal-fault error (5xx), not Rejectedf, so a slow decode isn't reported as a bad file. (b) image.quality is now validated 1..100 (0=default) at metamodel load, with a test covering -3/0/850.
status: addressed
---
