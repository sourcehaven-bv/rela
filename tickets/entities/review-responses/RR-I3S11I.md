---
id: RR-I3S11I
type: review-response
title: Two edited guide lines overflow the wrap width
finding: 'GUIDE-realtime.md:766 ("`default_in: keyed`. Nothing is stored. The store decides under its lock, so of two posts that arrive") and :2687 ("may post` in `rela channel read` change as the session moves. A post outside them answers") are about 100 characters wide, while the paragraphs around them wrap at about 74. Fix: rewrap both paragraphs and run just docs.'
severity: nit
resolution: Rewrapped both paragraphs to the surrounding width and ran just docs.
status: addressed
---
