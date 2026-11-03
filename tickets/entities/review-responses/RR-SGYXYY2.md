---
id: RR-SGYXYY2
type: review-response
title: Navigating away silently drops held values
finding: commitImmediately always settled without error because each runPatch catches its own error, so confirmLeave never prompted for a field showing a conflict or failure.
severity: significant
resolution: commitImmediately reports an error while any field is held, so the leave guard prompts. Unit test 'reports a held field to commitImmediately'.
status: addressed
---
