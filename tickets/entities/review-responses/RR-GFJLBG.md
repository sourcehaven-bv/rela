---
id: RR-GFJLBG
type: review-response
title: Create-error classes fall through to 500
finding: writeEntityMakingError has no case for ErrIllegalEntry, ErrOwningRule, ErrRelationAlreadyExists, FileWriteError, face errors or ErrCustomIDNotAllowed; a recipe that copies a non-entry status answers 500 while the offer says allowed.
severity: significant
resolution: 'Unnamed create errors now map through ClassifyCreateError: 403, 409, 422, and 500 only for unknown errors. ErrOwningRule joined the invalid class. Pinned by TestTaskItemHandler_CreateErrors.'
status: addressed
---
