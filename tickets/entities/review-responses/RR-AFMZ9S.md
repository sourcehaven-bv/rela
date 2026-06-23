---
id: RR-AFMZ9S
type: review-response
title: computeAttachments silently swallows ListAttachments backend error
finding: 'computeAttachments returns an empty map on a ListAttachments error with no log, while the sibling attachmentFileName (handler) correctly logs a non-ErrNotFound failure. Inconsistent: on the serialize path a backend fault silently reports ''no attachments'' with zero operator signal. Fix: log a non-ErrNotFound error before degrading to empty.'
severity: significant
resolution: computeAttachments now logs a non-ErrNotFound ListAttachments error via slog.Warn before degrading to an empty map, consistent with the sibling attachmentFileName. A backend fault on the serialize path is no longer silent.
status: addressed
---
